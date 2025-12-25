package cluster

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	flaggerclientset "github.com/fluxcd/flagger/pkg/client/clientset/versioned"
	flaggerinformers "github.com/fluxcd/flagger/pkg/client/informers/externalversions"
	monitoringinformers "github.com/prometheus-operator/prometheus-operator/pkg/client/informers/externalversions"
	monitoringclient "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
	vpaclientset "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/client/clientset/versioned"
	vpainformers "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/client/informers/externalversions"

	"github.com/yanshicheng/kube-nova/common/k8smanager/informer"
	"github.com/zeromicro/go-zero/core/logx"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// clusterClient 集群客户端实现
type clusterClient struct {
	// 配置信息
	config Config

	// 日志 通过 WithContext 设置
	l   logx.Logger
	ctx context.Context

	// 原始客户端
	kubeClient    kubernetes.Interface
	dynamicClient dynamic.Interface
	restConfig    *rest.Config
	clientSet     *kubernetes.Clientset
	// informer 相关
	// Informer 管理器
	informerManager *informer.Manager
	// ========== VPA 和 Flagger 专用 clientset ==========
	vpaClientset     vpaclientset.Interface
	flaggerClientset flaggerclientset.Interface
	resourceOperator
	vpaInformerFactory     vpainformers.SharedInformerFactory
	flaggerInformerFactory flaggerinformers.SharedInformerFactory

	// ========== 监控专用 clientset ==========
	monitoringClientset       monitoringclient.Interface
	monitoringInformerFactory monitoringinformers.SharedInformerFactory

	// 集群生命周期管理
	wg     sync.WaitGroup
	closed int32
	ready  int32

	// 集群健康
	healthy              int32
	lastHealthCheckTime  time.Time
	lastHealthCheckError error
	healthMutex          sync.RWMutex
}

func (c *clusterClient) GetClientSet() *kubernetes.Clientset {
	return c.clientSet
}

// NewClient  创建集群客户端
func NewClient(config Config) (Client, error) {
	// 验证集群配置
	if err := validateConfig(config); err != nil {
		logx.Errorf("集群配置文件验证失败: %w", err)
		return nil, fmt.Errorf("集群配置文件验证失败: %w", err)
	}
	// 创建 REST 配置
	restConfig, err := buildRESTConfig(config)
	if err != nil {
		logx.Errorf("创建 REST 配置失败: %w", err)
		return nil, fmt.Errorf("创建 REST 配置失败: %w", err)
	}

	// 应用客户端选项
	restConfig.QPS = config.Options.QPS
	restConfig.Burst = config.Options.Burst
	restConfig.Timeout = config.Options.Timeout

	// 创建 Kubernetes 客户端
	kubeClient, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		logx.Errorf("创建 Kubernetes 客户端失败: %w", err)
		return nil, fmt.Errorf("创建 Kubernetes 客户端失败: %w", err)
	}

	// 创建 Dynamic Client
	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		logx.Errorf("创建 Dynamic Client 失败: %w", err)
		return nil, fmt.Errorf("创建 Dynamic Client 失败: %w", err)
	}
	clientSet, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		logx.Errorf("创建 Dynamic Client 失败: %w", err)
		return nil, fmt.Errorf("创建 Dynamic Client 失败: %w", err)
	}
	// ========== 创建 VPA 和 Flagger 专用 clientset ==========
	vpaClientset, err := vpaclientset.NewForConfig(restConfig)
	if err != nil {
		logx.Errorf("创建 VPA 客户端失败: %w", err)
		return nil, fmt.Errorf("创建 VPA 客户端失败: %w", err)
	}

	flaggerClientset, err := flaggerclientset.NewForConfig(restConfig)
	if err != nil {
		logx.Errorf("创建 Flagger 客户端失败: %w", err)
		return nil, fmt.Errorf("创建 Flagger 客户端失败: %w", err)
	}
	// ========== 创建监控专用 clientset ==========
	monitoringClientset, err := monitoringclient.NewForConfig(restConfig)
	if err != nil {
		logx.Errorf("创建 Monitoring 客户端失败: %w", err)
		return nil, fmt.Errorf("创建 Monitoring 客户端失败: %w", err)
	}

	//初始化集群客户端

	c := &clusterClient{
		config:              config,
		ctx:                 context.Background(),
		l:                   logx.WithContext(context.Background()),
		restConfig:          restConfig,
		kubeClient:          kubeClient,
		dynamicClient:       dynamicClient,
		clientSet:           clientSet,
		vpaClientset:        vpaClientset,
		flaggerClientset:    flaggerClientset,
		monitoringClientset: monitoringClientset,
	}

	// 设置初始化状态
	atomic.StoreInt32(&c.healthy, 1)
	atomic.StoreInt32(&c.closed, 0)
	atomic.StoreInt32(&c.ready, 0)

	// 如果启用 informer 创建 informerFactory
	// 如果启用 Informer，创建并启动 Informer 管理器
	if config.Options.EnableInformer {
		// 创建 Informer 管理器
		c.informerManager = informer.NewManager(
			c.kubeClient,
			c.dynamicClient,
			config.Options.ResyncPeriod,
		).WithLogger(c.l)

		// 注册所有 Informer
		if err := c.informerManager.RegisterInformers(); err != nil {
			c.l.Errorf("注册 Informer 失败: %v", err)
			// 允许降级运行
		}
		// ========== 创建 VPA 和 Flagger 的 InformerFactory ==========
		c.vpaInformerFactory = vpainformers.NewSharedInformerFactory(
			vpaClientset,
			config.Options.ResyncPeriod,
		)

		c.flaggerInformerFactory = flaggerinformers.NewSharedInformerFactory(
			flaggerClientset,
			config.Options.ResyncPeriod,
		)
		// ========== 创建 Monitoring 的 InformerFactory ==========
		c.monitoringInformerFactory = monitoringinformers.NewSharedInformerFactory(
			monitoringClientset,
			config.Options.ResyncPeriod,
		)
		// 启动 Informer
		c.wg.Add(1)
		go func() {
			defer c.wg.Done()

			if err := c.informerManager.Start(); err != nil {
				c.l.Errorf("启动 Informer 失败: %v", err)
			} else {
				atomic.StoreInt32(&c.ready, 1)
				c.l.Infof("集群 %s Informer 启动成功", config.ID)
			}
		}()
	} else {
		// 不使用 Informer，直接标记为就绪
		atomic.StoreInt32(&c.ready, 1)
	}

	// 初始化操作器（创建时使用默认 context）
	c.initOperators()
	// 执行初始健康检查
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := c.HealthCheck(ctx); err != nil {
		c.l.Errorf("集群 %s 初始健康检查失败: %v", config.ID, err)
		atomic.StoreInt32(&c.healthy, 0)
	}

	c.l.Infof("成功创建集群客户端: %s (%s), Informer: %v",
		config.ID, config.Name, config.Options.EnableInformer)

	return c, nil
}

// WithContext 设置日志上下文，返回带有新上下文的客户端
func (c *clusterClient) WithContext(ctx context.Context) Client {
	// 更新日志器和上下文
	c.ctx = ctx
	c.l = logx.WithContext(ctx)

	// 重新初始化操作器，使其使用新的上下文
	c.initOperators()

	return c
}

// HealthCheck 集群健康检查
func (c *clusterClient) HealthCheck(ctx context.Context) error {
	// 判断集群是否已关闭
	if atomic.LoadInt32(&c.closed) == 1 {
		return fmt.Errorf("集群已关闭")
	}

	l := logx.WithContext(ctx)
	startTime := time.Now()

	// 检查 API 服务器
	_, err := c.kubeClient.Discovery().ServerVersion()
	if err != nil {
		l.Errorf("API 服务器检查失败: %v", err)
		atomic.StoreInt32(&c.healthy, 0)
		return fmt.Errorf("API 服务器检查失败: %w", err)
	}

	// 尝试列出ns资源
	_, err = c.kubeClient.CoreV1().Namespaces().List(ctx, metav1.ListOptions{Limit: 1})
	if err != nil {
		l.Errorf("资源列表检查失败: %v", err)
		atomic.StoreInt32(&c.healthy, 0)
		return fmt.Errorf("资源列表检查失败: %w", err)
	}
	// 更新健康状态
	c.healthMutex.Lock()
	c.lastHealthCheckTime = time.Now()
	c.lastHealthCheckError = err
	atomic.StoreInt32(&c.healthy, 1)
	c.healthMutex.Unlock()
	l.Infof("集群 %s 健康检查成功 (耗时: %v)", c.config.ID, time.Since(startTime))
	return nil
}

// buildRESTConfig 创建 REST 配置
func buildRESTConfig(config Config) (*rest.Config, error) {
	var restConfig *rest.Config
	var err error
	switch config.AuthType {
	case AuthTypeKubeConfig:
		// 判断kubeconfig文件是否存在
		kubeConfigData := []byte(config.KubeConfigData)
		clientConfig, err := clientcmd.NewClientConfigFromBytes(kubeConfigData)
		if err != nil {
			logx.Errorf("解析kubeconfig数据失败: %v", err)
			return nil, fmt.Errorf("解析kubeconfig数据失败: %w", err)
		}
		restConfig, err = clientConfig.ClientConfig()
	case AuthTypeInCluster:
		restConfig, err = rest.InClusterConfig()
	case AuthTypeToken:
		restConfig = &rest.Config{
			Host:        config.APIServer,
			BearerToken: config.Token,
			TLSClientConfig: rest.TLSClientConfig{
				Insecure: config.Insecure,
			},
		}
	case AuthTypeCertificate:
		restConfig = &rest.Config{
			Host: config.APIServer,
			TLSClientConfig: rest.TLSClientConfig{
				CAFile:   config.CAFile,
				CertFile: config.CertFile,
				KeyFile:  config.KeyFile,
				Insecure: config.Insecure,
			},
		}
	default:
		logx.Errorf("不支持的认证类型: %s", config.AuthType)
		return nil, fmt.Errorf("不支持的认证类型: %s", config.AuthType)
	}
	if err != nil {
		logx.Errorf("创建集群失败: %v", err)
		return nil, fmt.Errorf("创建集群失败: %w", err)
	}
	return restConfig, nil
}

// 基本信息获取方法

func (c *clusterClient) GetID() string {
	return c.config.ID
}

func (c *clusterClient) GetName() string {
	if c.config.Name != "" {
		return c.config.Name
	}
	return string(c.config.ID)
}

func (c *clusterClient) GetConfig() Config {
	return c.config
}

func (c *clusterClient) GetLogger() logx.Logger {
	return c.l
}

// 原始客户端访问

func (c *clusterClient) GetKubeClient() kubernetes.Interface {
	return c.kubeClient
}

func (c *clusterClient) GetDynamicClient() dynamic.Interface {
	return c.dynamicClient
}

func (c *clusterClient) GetRESTConfig() *rest.Config {
	return c.restConfig
}

func (c *clusterClient) GetInformerFactory() informers.SharedInformerFactory {
	if c.informerManager != nil {
		return c.informerManager.GetInformerFactory()
	}
	return nil
}

// Close 关闭客户端
func (c *clusterClient) Close() error {
	if !atomic.CompareAndSwapInt32(&c.closed, 0, 1) {
		return nil
	}

	c.l.Infof("关闭集群客户端: %s", c.config.ID)

	// 停止 Informer 管理器
	if c.informerManager != nil {
		c.informerManager.Stop()
	}

	// 等待所有协程退出
	done := make(chan struct{})
	go func() {
		c.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		c.l.Infof("集群客户端已关闭: %s", c.config.ID)
	case <-time.After(10 * time.Second):
		c.l.Errorf("集群客户端关闭超时: %s", c.config.ID)
	}

	return nil
}

// IsHealthy 检查是否健康
func (c *clusterClient) IsHealthy() bool {
	if atomic.LoadInt32(&c.closed) == 1 {
		return false
	}
	return atomic.LoadInt32(&c.healthy) == 1
}

func (c *clusterClient) GetVersionInfo() (*ClusterVersionInfo, error) {
	info, err := c.kubeClient.Discovery().ServerVersion()
	if err != nil {
		return nil, err
	}
	// 获取 kube-system namespace 的创建时间作为集群创建时间
	kubeSystemNs, err := c.kubeClient.CoreV1().Namespaces().Get(context.Background(), "kube-system", metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	// 将 BuildDate 字符串转换为 Unix 时间戳 (int64)
	buildTime, err := time.Parse(time.RFC3339, info.BuildDate)
	if err != nil {
		return nil, err
	}
	return &ClusterVersionInfo{
		ClusterCreateAt: kubeSystemNs.CreationTimestamp.Unix(),
		GitCommit:       info.GitCommit,
		Version:         info.GitVersion,
		Platform:        info.Platform,
		VersionBuildAt:  buildTime.Unix(),
	}, nil
}

// WaitForCacheSync 等待缓存同步
func (c *clusterClient) WaitForCacheSync(ctx context.Context) error {
	// 如果不需要 Informers，则返回 nil
	if c.informerManager == nil {
		return nil
	}

	logger := logx.WithContext(ctx)

	// 如果集群关闭则返回
	if atomic.LoadInt32(&c.ready) == 1 {
		return nil
	}

	// 等待缓存同步
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Errorf("等待缓存同步超时:", ctx.Err())
			return fmt.Errorf("等待缓存同步超时: %w", ctx.Err())
		case <-ticker.C:
			if atomic.LoadInt32(&c.ready) == 1 {
				logger.Infof("集群 %s 缓存同步完成", c.config.ID)
				return nil
			}
		}
	}
}

// HasSynced 检查是否已同步
func (c *clusterClient) HasSynced() bool {
	// 如果 informerManager 为空，则认为已同步
	if c.informerManager == nil {
		return true
	}
	// 调用 informerManager 的 HasSynced 方法
	return c.informerManager.HasSynced()
}
