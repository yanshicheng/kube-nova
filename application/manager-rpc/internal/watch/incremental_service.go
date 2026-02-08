package watch

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/watch/incremental"
	"github.com/zeromicro/go-zero/core/logx"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

// IncrementalSyncService 增量同步服务
type IncrementalSyncService struct {
	svcCtx *svc.ServiceContext // 服务上下文，包含所有依赖

	rpcAddr        string        // RPC 服务地址，用于健康检查
	startupTimeout time.Duration // 启动超时时间
	checkInterval  time.Duration // 健康检查间隔

	ctx    context.Context    // 服务上下文
	cancel context.CancelFunc // 用于取消服务
}

// IncrementalSyncServiceConfig 服务配置
type IncrementalSyncServiceConfig struct {
	SvcCtx         *svc.ServiceContext
	RPCAddr        string
	StartupTimeout time.Duration // 默认 60s
	CheckInterval  time.Duration // 默认 500ms
}

// NewIncrementalSyncService 创建增量同步服务实例
func NewIncrementalSyncService(cfg IncrementalSyncServiceConfig) *IncrementalSyncService {
	// 设置默认值
	if cfg.StartupTimeout <= 0 {
		cfg.StartupTimeout = 60 * time.Second
	}
	if cfg.CheckInterval <= 0 {
		cfg.CheckInterval = 500 * time.Millisecond
	}

	// 处理地址格式：将 0.0.0.0 替换为 127.0.0.1
	rpcAddr := cfg.RPCAddr
	if strings.HasPrefix(rpcAddr, "0.0.0.0:") {
		rpcAddr = strings.Replace(rpcAddr, "0.0.0.0:", "127.0.0.1:", 1)
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &IncrementalSyncService{
		svcCtx:         cfg.SvcCtx,
		rpcAddr:        rpcAddr,
		startupTimeout: cfg.StartupTimeout,
		checkInterval:  cfg.CheckInterval,
		ctx:            ctx,
		cancel:         cancel,
	}
}

// Start 启动增量同步服务
func (s *IncrementalSyncService) Start() {
	logx.Info("[IncrementalSyncService] 服务启动中，等待 RPC 服务就绪...")

	// 等待 RPC 服务就绪（通过实际调用 RPC 方法验证）
	if err := s.waitForRPCReady(); err != nil {
		logx.Errorf("[IncrementalSyncService] 等待 RPC 服务就绪失败: %v", err)
		return
	}

	logx.Info("[IncrementalSyncService] RPC 服务已就绪，开始初始化增量同步...")

	// Leader Election 模式需要设置 K8s 客户端
	if s.svcCtx.Config.LeaderElection.Enabled {
		if err := s.setupLeaderElectionKubeClient(); err != nil {
			logx.Errorf("[IncrementalSyncService] 设置 Leader Election K8s 客户端失败: %v", err)
			return
		}
	}

	// 设置事件处理器（Handler）
	if !SetupIncrementalSync(s.svcCtx) {
		logx.Error("[IncrementalSyncService] 增量同步设置失败")
		return
	}

	// 启动增量同步管理器
	if err := StartIncrementalSync(s.svcCtx); err != nil {
		logx.Errorf("[IncrementalSyncService] 增量同步启动失败: %v", err)
		return
	}

	logx.Info("[IncrementalSyncService] 增量同步启动成功")

	//  阻塞等待，直到收到停止信号
	<-s.ctx.Done()

	logx.Info("[IncrementalSyncService] 收到停止信号，服务即将关闭")
}

// setupLeaderElectionKubeClient 设置 Leader Election 所需的 K8s 客户端
// 按优先级尝试：InCluster -> ~/.kube/config -> 降级到 Redis 模式
func (s *IncrementalSyncService) setupLeaderElectionKubeClient() error {
	cfg := s.svcCtx.Config.LeaderElection

	// 获取 K8s 配置（自动尝试 InCluster 和 kubeconfig）
	restConfig, configSource, err := getKubeConfig()
	if err != nil {
		logx.Infof("[IncrementalSyncService] 无法获取 K8s 配置，降级到 Redis 分布式锁模式: %v", err)
		s.svcCtx.Config.LeaderElection.Enabled = false
		return nil
	}

	kubeClient, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		logx.Infof("[IncrementalSyncService] 创建 K8s 客户端失败，降级到 Redis 分布式锁模式: %v", err)
		s.svcCtx.Config.LeaderElection.Enabled = false
		return nil
	}

	// 创建 LeaderManager
	s.svcCtx.LeaderSyncManager = incremental.NewLeaderManager(incremental.LeaderManagerConfig{
		K8sManager:         s.svcCtx.K8sManager,
		ClusterModel:       s.svcCtx.OnecClusterModel,
		WorkerCount:        20,
		ProcessTimeout:     30 * time.Second,
		WatcherStopTimeout: 30 * time.Second,
		LeaderElection: struct {
			KubeClient     kubernetes.Interface
			LeaseName      string
			LeaseNamespace string
			LeaseDuration  time.Duration
			RenewDeadline  time.Duration
			RetryPeriod    time.Duration
		}{
			KubeClient:     kubeClient,
			LeaseName:      cfg.LeaseName,
			LeaseNamespace: cfg.LeaseNamespace,
			LeaseDuration:  cfg.LeaseDuration,
			RenewDeadline:  cfg.RenewDeadline,
			RetryPeriod:    cfg.RetryPeriod,
		},
	})

	logx.Infof("[IncrementalSyncService] Leader Election 配置完成 (%s), Lease=%s/%s",
		configSource, cfg.LeaseNamespace, cfg.LeaseName)

	return nil
}

// getKubeConfig 获取 K8s 配置
// 优先级：InCluster -> ~/.kube/config
// 返回：配置、配置来源描述、错误
func getKubeConfig() (*rest.Config, string, error) {
	// 1. 优先尝试 InCluster 配置 (Pod 内部)
	// 这一步非常快，如果不在集群内会立刻返回 error，不会阻塞
	if config, err := rest.InClusterConfig(); err == nil {
		return config, "InCluster", nil
	}

	// 2. 尝试默认路径 ~/.kube/config
	if home := homedir.HomeDir(); home != "" {
		kubeconfigPath := filepath.Join(home, ".kube", "config")

		// 直接尝试加载。如果文件不存在或格式错误，err 会不为空
		if config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath); err == nil {
			return config, fmt.Sprintf("LocalFile(%s)", kubeconfigPath), nil
		}
	}

	return nil, "", fmt.Errorf("无法获取 K8s 配置: 既非 InCluster 模式，也未找到有效的 ~/.kube/config")
}

// Stop 停止增量同步服务
func (s *IncrementalSyncService) Stop() {
	logx.Info("[IncrementalSyncService] 正在停止增量同步服务...")

	// 取消 context，让 Start() 方法中的阻塞结束
	s.cancel()

	// 停止增量同步管理器
	if err := StopIncrementalSync(s.svcCtx); err != nil {
		logx.Errorf("[IncrementalSyncService] 停止增量同步失败: %v", err)
	} else {
		logx.Info("[IncrementalSyncService] 增量同步服务已停止")
	}
}

// waitForRPCReady 等待 RPC 服务就绪
func (s *IncrementalSyncService) waitForRPCReady() error {
	deadline := time.Now().Add(s.startupTimeout)

	logx.Infof("[IncrementalSyncService] 开始检测 RPC 服务 (%s)，超时时间: %v",
		s.rpcAddr, s.startupTimeout)

	attempt := 0
	var lastErr error

	for time.Now().Before(deadline) {
		attempt++

		// 检查是否已经收到停止信号
		select {
		case <-s.ctx.Done():
			return fmt.Errorf("服务被取消")
		default:
		}

		// 调用 rpc 方法 判断 服务是否就绪
		if err := s.checkRPCHealth(); err != nil {
			lastErr = err
			// 连接失败，等待后重试
			if attempt%10 == 0 {
				logx.Infof("[IncrementalSyncService] 等待 RPC 服务就绪，已尝试 %d 次, 最后错误: %v",
					attempt, err)
			}
			time.Sleep(s.checkInterval)
			continue
		}

		// RPC 调用成功，服务已就绪
		logx.Infof("[IncrementalSyncService] RPC 服务就绪，尝试次数: %d", attempt)
		return nil
	}

	return fmt.Errorf("等待 RPC 服务就绪超时 (地址: %s, 超时: %v, 最后错误: %v)",
		s.rpcAddr, s.startupTimeout, lastErr)
}

// checkRPCHealth 通过实际调用 RPC 方法来检查服务是否就绪
func (s *IncrementalSyncService) checkRPCHealth() error {
	// 使用 K8sManager 来检查 RPC 是否就绪
	// 这样可以确保使用的是同一个 RPC 客户端连接
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// 尝试获取一个不存在的集群，如果返回业务错误说明 RPC 已就绪
	_, err := s.svcCtx.K8sManager.GetCluster(ctx, "__health_check__")
	if err != nil {
		errStr := err.Error()
		// 如果错误是连接相关或超时，说明服务还没准备好
		if strings.Contains(errStr, "服务不可用") ||
			strings.Contains(errStr, "unavailable") ||
			strings.Contains(errStr, "connection refused") ||
			strings.Contains(errStr, "connection reset") ||
			strings.Contains(errStr, "transport") ||
			strings.Contains(errStr, "请求超时") ||
			strings.Contains(errStr, "timeout") ||
			strings.Contains(errStr, "context deadline exceeded") {
			return fmt.Errorf("RPC 服务未就绪: %w", err)
		}
		// 其他错误（如"集群认证信息不存在"）说明 RPC 服务已经在正常处理请求了
	}

	return nil
}

// GetStats 获取服务状态
func (s *IncrementalSyncService) GetStats() *incremental.ManagerStats {
	if s.svcCtx == nil || s.svcCtx.IncrementalSyncManager == nil {
		return nil
	}
	return s.svcCtx.IncrementalSyncManager.GetStats()
}

// IsRunning 检查服务是否在运行
func (s *IncrementalSyncService) IsRunning() bool {
	if s.svcCtx == nil || s.svcCtx.IncrementalSyncManager == nil {
		return false
	}
	return s.svcCtx.IncrementalSyncManager.IsRunning()
}
