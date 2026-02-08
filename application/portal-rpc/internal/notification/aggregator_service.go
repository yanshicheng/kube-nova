package notification

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

// AggregatorService 告警聚合器服务
// 负责启动和管理告警聚合器，支持三级降级策略
type AggregatorService struct {
	aggregator     *AlertAggregator
	leaderElector  *LeaderElector
	redis          *redis.Redis
	manager        InternalManager
	config         AggregatorConfig
	leaderConfig   LeaderElectionConfig
	ctx            context.Context
	cancel         context.CancelFunc
	useK8sLeader   bool // 是否使用 K8s Leader Election
	useRedisLeader bool // 是否使用 Redis Leader Election
	logger         logx.Logger

	// 健康检查和监控
	lastLeaderCheck   time.Time
	leaderCheckFailed int
	maxLeaderFailures int
	healthCheckTicker *time.Ticker
}

// LeaderElectionConfig Leader 选举配置
type LeaderElectionConfig struct {
	Enabled        bool
	LeaseName      string
	LeaseNamespace string
	LeaseDuration  time.Duration
	RenewDeadline  time.Duration
	RetryPeriod    time.Duration
}

// AggregatorServiceConfig 聚合器服务配置
type AggregatorServiceConfig struct {
	Redis        *redis.Redis
	Manager      InternalManager
	Config       AggregatorConfig
	LeaderConfig LeaderElectionConfig
}

// NewAggregatorService 创建告警聚合器服务
func NewAggregatorService(cfg AggregatorServiceConfig) *AggregatorService {
	ctx, cancel := context.WithCancel(context.Background())

	return &AggregatorService{
		redis:             cfg.Redis,
		manager:           cfg.Manager,
		config:            cfg.Config,
		leaderConfig:      cfg.LeaderConfig,
		ctx:               ctx,
		cancel:            cancel,
		logger:            logx.WithContext(context.Background()),
		maxLeaderFailures: 3, // 最大连续失败次数
		lastLeaderCheck:   time.Now(),
	}
}

// Start 启动聚合器服务
// 按优先级尝试：InCluster -> Kubeconfig -> Redis 降级 -> 单机模式
func (s *AggregatorService) Start() error {
	if !s.config.Enabled {
		s.logger.Info("[AggregatorService] 聚合器未启用")
		return nil
	}

	s.logger.Info("[AggregatorService] 聚合器服务启动中...")

	// 如果启用了 Leader Election，尝试设置 K8s 客户端
	if s.leaderConfig.Enabled {
		if err := s.setupLeaderElection(); err != nil {
			s.logger.Errorf("[AggregatorService] K8s Leader Election 设置失败: %v", err)
			s.logger.Info("[AggregatorService] 降级到 Redis 分布式锁模式")
			s.useK8sLeader = false
			s.useRedisLeader = true
		} else {
			s.useK8sLeader = true
			s.useRedisLeader = false
		}
	} else {
		s.logger.Info("[AggregatorService] Leader Election 未启用，使用单机模式")
		s.useK8sLeader = false
		s.useRedisLeader = false
	}

	// 启动健康检查
	s.startHealthCheck()

	// 根据模式启动服务
	if s.useK8sLeader {
		s.logger.Info("[AggregatorService] 使用 K8s Leader Election 模式")
		return s.startWithK8sLeaderElection()
	} else if s.useRedisLeader {
		s.logger.Info("[AggregatorService] 使用 Redis 分布式锁模式")
		return s.startWithRedisLock()
	} else {
		s.logger.Info("[AggregatorService] 使用单机模式（无 Leader Election）")
		return s.startWithoutLeaderElection()
	}
}

// setupLeaderElection 设置 Leader Election
// 按优先级尝试：InCluster -> ~/.kube/config -> 降级到 Redis 模式
func (s *AggregatorService) setupLeaderElection() error {
	// 获取 K8s 配置（自动尝试 InCluster 和 kubeconfig）
	restConfig, configSource, err := getKubeConfig()
	if err != nil {
		return errorx.Msg(fmt.Sprintf("无法获取 K8s 配置: %v", err))
	}

	kubeClient, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return errorx.Msg(fmt.Sprintf("创建 K8s 客户端失败: %v", err))
	}

	// 生成唯一的 Identity
	hostname, _ := os.Hostname()
	identity := fmt.Sprintf("%s-%d", hostname, os.Getpid())

	// 创建 Leader Elector
	s.leaderElector, err = NewLeaderElector(K8sLeaderElectionConfig{
		KubeClient:     kubeClient,
		LeaseName:      s.leaderConfig.LeaseName,
		LeaseNamespace: s.leaderConfig.LeaseNamespace,
		LeaseDuration:  s.leaderConfig.LeaseDuration,
		RenewDeadline:  s.leaderConfig.RenewDeadline,
		RetryPeriod:    s.leaderConfig.RetryPeriod,
		Identity:       identity,
		OnStartedLeading: func(ctx context.Context) {
			s.logger.Info("[AggregatorService] 成为 Leader，启动聚合器处理循环")
			s.startProcessLoop()
		},
		OnStoppedLeading: func() {
			s.logger.Info("[AggregatorService] 失去 Leader 身份，停止聚合器处理循环")
			s.stopProcessLoop()
		},
		OnNewLeader: func(newIdentity string) {
			if newIdentity != identity {
				s.logger.Infof("[AggregatorService] 新 Leader 选出: %s", newIdentity)
			}
		},
	})

	if err != nil {
		return errorx.Msg(fmt.Sprintf("创建 Leader Elector 失败: %v", err))
	}

	s.useK8sLeader = true
	s.logger.Infof("[AggregatorService] Leader Election 配置完成 (%s), Lease=%s/%s",
		configSource, s.leaderConfig.LeaseNamespace, s.leaderConfig.LeaseName)

	return nil
}

// getKubeConfig 获取 K8s 配置
// 优先级：InCluster -> ~/.kube/config
// 返回：配置、配置来源描述、错误
func getKubeConfig() (*rest.Config, string, error) {
	// 1. 优先尝试 InCluster 配置 (Pod 内部)
	if config, err := rest.InClusterConfig(); err == nil {
		return config, "InCluster", nil
	}

	// 2. 尝试默认路径 ~/.kube/config
	if home := homedir.HomeDir(); home != "" {
		kubeconfigPath := filepath.Join(home, ".kube", "config")
		if config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath); err == nil {
			return config, fmt.Sprintf("LocalFile(%s)", kubeconfigPath), nil
		}
	}

	return nil, "", errorx.Msg("无法获取 K8s 配置: 既非 InCluster 模式，也未找到有效的 ~/.kube/config")
}

// startWithK8sLeaderElection 使用 K8s Leader Election 模式启动
func (s *AggregatorService) startWithK8sLeaderElection() error {
	// 创建聚合器（不启动 Redis Leader 选举循环）
	s.aggregator = NewAlertAggregatorWithManager(s.redis, s.manager, s.config)
	if s.aggregator == nil {
		return errorx.Msg("创建聚合器失败")
	}

	// 启动 K8s Leader Election（阻塞）
	go s.leaderElector.Run(s.ctx)

	s.logger.Info("[AggregatorService] K8s Leader Election 模式启动成功")
	return nil
}

// startWithRedisLock 使用 Redis 分布式锁模式启动
func (s *AggregatorService) startWithRedisLock() error {
	// 创建聚合器（会自动启动 Redis Leader 选举循环）
	s.aggregator = NewAlertAggregatorWithManager(s.redis, s.manager, s.config)
	if s.aggregator == nil {
		return errorx.Msg("创建聚合器失败")
	}

	s.logger.Info("[AggregatorService] Redis 分布式锁模式启动成功")
	return nil
}

// startProcessLoop 启动聚合器处理循环
func (s *AggregatorService) startProcessLoop() {
	if s.aggregator != nil {
		// 启动处理循环
		go s.aggregator.processLoopWithStop(make(chan struct{}))
	}
}

// stopProcessLoop 停止聚合器处理循环
func (s *AggregatorService) stopProcessLoop() {
	// 处理循环会在 Leader 失去身份时自动停止
}

// Stop 停止聚合器服务
func (s *AggregatorService) Stop() {
	s.logger.Info("[AggregatorService] 正在停止聚合器服务...")

	s.cancel()

	// 停止健康检查
	if s.healthCheckTicker != nil {
		s.healthCheckTicker.Stop()
		s.healthCheckTicker = nil
	}

	if s.aggregator != nil {
		s.aggregator.Stop()
	}

	if s.leaderElector != nil {
		s.leaderElector.Stop()
	}

	s.logger.Info("[AggregatorService] 聚合器服务已停止")
}

// startHealthCheck 启动健康检查
func (s *AggregatorService) startHealthCheck() {
	if s.healthCheckTicker != nil {
		return // 已经启动
	}

	s.healthCheckTicker = time.NewTicker(30 * time.Second)
	go s.healthCheckLoop()
	s.logger.Info("[AggregatorService] 健康检查已启动")
}

// healthCheckLoop 健康检查循环
func (s *AggregatorService) healthCheckLoop() {
	defer func() {
		if s.healthCheckTicker != nil {
			s.healthCheckTicker.Stop()
		}
	}()

	for {
		select {
		case <-s.healthCheckTicker.C:
			s.performHealthCheck()
		case <-s.ctx.Done():
			s.logger.Info("[AggregatorService] 健康检查循环已停止")
			return
		}
	}
}

// performHealthCheck 执行健康检查
func (s *AggregatorService) performHealthCheck() {
	now := time.Now()
	s.lastLeaderCheck = now

	// 检查 Leader Election 状态
	if s.useK8sLeader && s.leaderElector != nil {
		// K8s Leader Election 健康检查
		if !s.checkK8sLeaderHealth() {
			s.leaderCheckFailed++
			s.logger.Errorf("[AggregatorService] K8s Leader Election 健康检查失败 (%d/%d)",
				s.leaderCheckFailed, s.maxLeaderFailures)

			if s.leaderCheckFailed >= s.maxLeaderFailures {
				s.logger.Errorf("[AggregatorService] K8s Leader Election 连续失败，降级到 Redis 模式")
				s.fallbackToRedis()
			}
		} else {
			s.leaderCheckFailed = 0 // 重置失败计数
		}
	} else if s.useRedisLeader && s.aggregator != nil {
		// Redis Leader Election 健康检查
		if !s.checkRedisLeaderHealth() {
			s.leaderCheckFailed++
			s.logger.Errorf("[AggregatorService] Redis Leader Election 健康检查失败 (%d/%d)",
				s.leaderCheckFailed, s.maxLeaderFailures)

			if s.leaderCheckFailed >= s.maxLeaderFailures {
				s.logger.Errorf("[AggregatorService] Redis Leader Election 连续失败，降级到单机模式")
				s.fallbackToStandalone()
			}
		} else {
			s.leaderCheckFailed = 0 // 重置失败计数
		}
	}

	// 记录状态信息
	if s.leaderCheckFailed == 0 {
		s.logger.Infof("[AggregatorService] 健康检查通过 | K8s=%v | Redis=%v",
			s.useK8sLeader, s.useRedisLeader)
	}
}

// checkK8sLeaderHealth 检查 K8s Leader Election 健康状态
func (s *AggregatorService) checkK8sLeaderHealth() bool {
	if s.leaderElector == nil {
		return false
	}

	// 检查是否能获取当前 Leader 信息
	leader := s.leaderElector.GetLeader()
	if leader == "" {
		s.logger.Infof("[AggregatorService] K8s Leader Election: 无法获取当前 Leader")
		return false
	}

	s.logger.Infof("[AggregatorService] K8s Leader Election 健康 | 当前Leader=%s | 本实例Leader=%v",
		leader, s.leaderElector.IsLeader())
	return true
}

// checkRedisLeaderHealth 检查 Redis Leader Election 健康状态
func (s *AggregatorService) checkRedisLeaderHealth() bool {
	if s.redis == nil {
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// 检查 Leader Election Key（同时测试 Redis 连接）
	currentLeader, err := s.redis.GetCtx(ctx, LeaderElectionKey)
	if err != nil && err.Error() != "redis: nil" {
		s.logger.Errorf("[AggregatorService] 无法检查 Redis Leader Election Key: %v", err)
		return false
	}

	s.logger.Infof("[AggregatorService] Redis Leader Election 健康 | 当前Leader=%s", currentLeader)
	return true
}

// fallbackToRedis 降级到 Redis 模式
func (s *AggregatorService) fallbackToRedis() {
	s.logger.Info("[AggregatorService] 开始降级到 Redis 模式...")

	// 停止 K8s Leader Election
	if s.leaderElector != nil {
		s.leaderElector.Stop()
		s.leaderElector = nil
	}

	// 切换到 Redis 模式
	s.useK8sLeader = false
	s.useRedisLeader = true
	s.leaderCheckFailed = 0

	// 重新启动聚合器
	if err := s.startWithRedisLock(); err != nil {
		s.logger.Errorf("[AggregatorService] Redis 模式启动失败: %v", err)
		s.fallbackToStandalone()
	} else {
		s.logger.Info("[AggregatorService] 成功降级到 Redis 模式")
	}
}

// fallbackToStandalone 降级到单机模式
func (s *AggregatorService) fallbackToStandalone() {
	s.logger.Info("[AggregatorService] 开始降级到单机模式...")

	// 停止所有 Leader Election
	if s.leaderElector != nil {
		s.leaderElector.Stop()
		s.leaderElector = nil
	}

	if s.aggregator != nil {
		s.aggregator.Stop()
		s.aggregator = nil
	}

	// 切换到单机模式
	s.useK8sLeader = false
	s.useRedisLeader = false
	s.leaderCheckFailed = 0

	// 启动单机模式
	if err := s.startWithoutLeaderElection(); err != nil {
		s.logger.Errorf("[AggregatorService] 单机模式启动失败: %v", err)
	} else {
		s.logger.Info("[AggregatorService] 成功降级到单机模式")
	}
}

// startWithoutLeaderElection 启动单机模式（无 Leader Election）
func (s *AggregatorService) startWithoutLeaderElection() error {
	// 创建聚合器，但禁用 Leader Election
	config := s.config
	config.Enabled = true // 确保聚合器启用

	// 创建一个特殊的聚合器配置，直接启动处理循环
	s.aggregator = NewAlertAggregatorWithManager(s.redis, s.manager, config)
	if s.aggregator == nil {
		return errorx.Msg("创建聚合器失败")
	}

	// 直接启动处理循环（无需 Leader Election）
	go s.aggregator.processLoopWithStop(make(chan struct{}))

	s.logger.Info("[AggregatorService] 单机模式启动成功")
	return nil
}

// GetAggregator 获取聚合器实例
func (s *AggregatorService) GetAggregator() *AlertAggregator {
	return s.aggregator
}

// GetHealthInfo 获取服务健康信息
func (s *AggregatorService) GetHealthInfo() map[string]interface{} {
	healthInfo := map[string]interface{}{
		"enabled":           s.config.Enabled,
		"useK8sLeader":      s.useK8sLeader,
		"useRedisLeader":    s.useRedisLeader,
		"lastLeaderCheck":   s.lastLeaderCheck.Format(time.RFC3339),
		"leaderCheckFailed": s.leaderCheckFailed,
		"maxLeaderFailures": s.maxLeaderFailures,
		"leaderConfig": map[string]interface{}{
			"enabled":        s.leaderConfig.Enabled,
			"leaseName":      s.leaderConfig.LeaseName,
			"leaseNamespace": s.leaderConfig.LeaseNamespace,
			"leaseDuration":  s.leaderConfig.LeaseDuration.String(),
			"renewDeadline":  s.leaderConfig.RenewDeadline.String(),
			"retryPeriod":    s.leaderConfig.RetryPeriod.String(),
		},
	}

	// 添加聚合器健康信息
	if s.aggregator != nil {
		healthInfo["aggregator"] = s.aggregator.GetHealthInfo()
	}

	// 添加 K8s Leader Election 信息
	if s.leaderElector != nil {
		healthInfo["k8sLeaderElection"] = map[string]interface{}{
			"isLeader":      s.leaderElector.IsLeader(),
			"currentLeader": s.leaderElector.GetLeader(),
		}
	}

	return healthInfo
}
