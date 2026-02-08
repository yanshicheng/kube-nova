package incremental

import (
	"context"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
	"k8s.io/client-go/kubernetes"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/common/k8smanager/cluster"
)

// LeaderManagerConfig Leader 模式管理器配置
type LeaderManagerConfig struct {
	// 节点标识，为空则自动生成
	NodeID string

	// K8s 集群管理器
	K8sManager cluster.Manager

	// 集群数据库模型
	ClusterModel model.OnecClusterModel

	// Leader Election 配置
	LeaderElection struct {
		// 用于创建 Lease 的 K8s 客户端
		// 通常使用管理集群的客户端
		KubeClient kubernetes.Interface

		// Lease 资源配置
		LeaseName      string // 默认 "manager-rpc-watch-leader"
		LeaseNamespace string // 默认 "kube-nova"

		// 时间配置
		LeaseDuration time.Duration // 默认 15s
		RenewDeadline time.Duration // 默认 10s
		RetryPeriod   time.Duration // 默认 2s
	}

	// Processor 配置
	WorkerCount    int           // 工作协程数，默认 10
	ProcessTimeout time.Duration // 单个事件处理超时，默认 30s

	// Watcher 配置
	WatcherStopTimeout time.Duration // 监听器停止超时，默认 30s
}

// LeaderManager 基于 Leader Election 的增量同步管理器
// 只有 Leader 节点会运行 Informer 和处理事件
type LeaderManager struct {
	nodeID             string
	k8sManager         cluster.Manager
	clusterModel       model.OnecClusterModel
	workerCount        int
	processTimeout     time.Duration
	watcherStopTimeout time.Duration

	// Leader Election
	leaderElector *LeaderElector
	leaderConfig  LeaderManagerConfig

	// Processor 和 Handler（只有 Leader 才会初始化）
	processor *WorkQueueProcessor
	handler   EventHandler
	watchers  map[string]*ClusterWatcher

	ctx       context.Context
	cancel    context.CancelFunc
	mu        sync.RWMutex
	wg        sync.WaitGroup
	isRunning int32
	isLeader  int32
	stopOnce  sync.Once
	startTime time.Time
}

// NewLeaderManager 创建基于 Leader Election 的管理器
func NewLeaderManager(cfg LeaderManagerConfig) *LeaderManager {
	nodeID := cfg.NodeID
	if nodeID == "" {
		hostname, _ := os.Hostname()
		nodeID = fmt.Sprintf("%s-%d", hostname, os.Getpid())
	}

	if cfg.WorkerCount <= 0 {
		cfg.WorkerCount = 10
	}
	if cfg.ProcessTimeout <= 0 {
		cfg.ProcessTimeout = 30 * time.Second
	}
	if cfg.WatcherStopTimeout <= 0 {
		cfg.WatcherStopTimeout = 30 * time.Second
	}

	return &LeaderManager{
		nodeID:             nodeID,
		k8sManager:         cfg.K8sManager,
		clusterModel:       cfg.ClusterModel,
		workerCount:        cfg.WorkerCount,
		processTimeout:     cfg.ProcessTimeout,
		watcherStopTimeout: cfg.WatcherStopTimeout,
		leaderConfig:       cfg,
		watchers:           make(map[string]*ClusterWatcher),
	}
}

// SetHandler 设置事件处理器
func (m *LeaderManager) SetHandler(h EventHandler) {
	m.handler = h
}

// Start 启动管理器
// 此方法会阻塞，直到 context 被取消或发生错误
func (m *LeaderManager) Start() error {
	if !atomic.CompareAndSwapInt32(&m.isRunning, 0, 1) {
		return ErrManagerAlreadyRunning
	}

	if m.handler == nil {
		atomic.StoreInt32(&m.isRunning, 0)
		return ErrHandlerNotSet
	}
	if m.k8sManager == nil {
		atomic.StoreInt32(&m.isRunning, 0)
		return ErrK8sManagerNotSet
	}
	if m.clusterModel == nil {
		atomic.StoreInt32(&m.isRunning, 0)
		return ErrClusterModelNotSet
	}
	if m.leaderConfig.LeaderElection.KubeClient == nil {
		atomic.StoreInt32(&m.isRunning, 0)
		return fmt.Errorf("LeaderElection.KubeClient is required")
	}

	m.startTime = time.Now()
	m.stopOnce = sync.Once{}
	m.ctx, m.cancel = context.WithCancel(context.Background())

	logx.Infof("[LeaderManager] 正在启动, NodeID=%s", m.nodeID)

	// 创建 Leader Elector
	var err error
	m.leaderElector, err = NewLeaderElector(LeaderElectionConfig{
		LeaseName:      m.leaderConfig.LeaderElection.LeaseName,
		LeaseNamespace: m.leaderConfig.LeaderElection.LeaseNamespace,
		LeaseDuration:  m.leaderConfig.LeaderElection.LeaseDuration,
		RenewDeadline:  m.leaderConfig.LeaderElection.RenewDeadline,
		RetryPeriod:    m.leaderConfig.LeaderElection.RetryPeriod,
		KubeClient:     m.leaderConfig.LeaderElection.KubeClient,
		OnStartedLeading: func(ctx context.Context) {
			m.onStartedLeading(ctx)
		},
		OnStoppedLeading: func() {
			m.onStoppedLeading()
		},
		OnNewLeader: func(identity string) {
			logx.Infof("[LeaderManager] 当前 Leader: %s", identity)
		},
	})
	if err != nil {
		atomic.StoreInt32(&m.isRunning, 0)
		return fmt.Errorf("创建 LeaderElector 失败: %w", err)
	}

	// 启动 Leader Election（阻塞）
	logx.Infof("[LeaderManager] 开始 Leader 选举...")
	return m.leaderElector.Start(m.ctx)
}

// onStartedLeading 成为 Leader 时的回调
func (m *LeaderManager) onStartedLeading(ctx context.Context) {
	atomic.StoreInt32(&m.isLeader, 1)
	logx.Infof("[LeaderManager] 成为 Leader，开始初始化 Informer...")

	// 创建 Processor
	m.processor = NewWorkQueueProcessor(WorkQueueProcessorConfig{
		Handler:          m.handler,
		WorkerCount:      m.workerCount,
		ProcessTimeout:   m.processTimeout,
		OnEventCompleted: m.onEventCompleted,
	})
	m.processor.Start(ctx)

	// 初始化集群监听器
	if err := m.initClusterWatchers(); err != nil {
		logx.Errorf("[LeaderManager] 初始化集群监听器失败: %v", err)
	}

	m.mu.RLock()
	watcherCount := len(m.watchers)
	m.mu.RUnlock()

	logx.Infof("[LeaderManager] Leader 初始化完成, Workers=%d, Watchers=%d",
		m.workerCount, watcherCount)
}

// onStoppedLeading 失去 Leader 时的回调
func (m *LeaderManager) onStoppedLeading() {
	atomic.StoreInt32(&m.isLeader, 0)
	logx.Infof("[LeaderManager] 失去 Leader 身份，停止所有 Informer...")

	// 停止所有 watcher
	m.mu.Lock()
	for clusterUUID, watcher := range m.watchers {
		logx.Debugf("[LeaderManager] 停止集群 %s 的监听器", clusterUUID)
		watcher.Stop()
	}
	m.watchers = make(map[string]*ClusterWatcher)
	m.mu.Unlock()

	// 等待所有 watcher goroutine 退出
	m.wg.Wait()

	// 停止 processor
	if m.processor != nil {
		m.processor.Stop()
		m.processor = nil
	}

	logx.Infof("[LeaderManager] 已停止所有 Informer，等待重新选举...")
}

// Stop 停止管理器
func (m *LeaderManager) Stop() error {
	m.stopOnce.Do(func() {
		if atomic.LoadInt32(&m.isRunning) == 0 {
			return
		}

		logx.Infof("[LeaderManager] 正在停止, NodeID=%s", m.nodeID)

		// 停止 Leader Election
		if m.leaderElector != nil {
			m.leaderElector.Stop()
		}

		// 取消 context
		if m.cancel != nil {
			m.cancel()
		}

		atomic.StoreInt32(&m.isRunning, 0)

		uptime := time.Since(m.startTime)
		logx.Infof("[LeaderManager] 已停止, NodeID=%s, 运行时长=%v",
			m.nodeID, uptime.Round(time.Second))
	})

	return nil
}

// onEventCompleted 事件处理完成回调
func (m *LeaderManager) onEventCompleted(clusterUUID string) {
	m.mu.RLock()
	watcher, exists := m.watchers[clusterUUID]
	m.mu.RUnlock()

	if exists && watcher != nil {
		watcher.DecrementPending()
	}
}

// initClusterWatchers 初始化集群监听器
func (m *LeaderManager) initClusterWatchers() error {
	queryCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	clusters, err := m.clusterModel.GetAllClusters(queryCtx)
	if err != nil {
		return fmt.Errorf("查询集群列表失败: %v", err)
	}

	if len(clusters) == 0 {
		logx.Infof("[LeaderManager] 数据库中未发现任何集群")
		return nil
	}

	logx.Infof("[LeaderManager] 发现 %d 个集群，开始启动监听器", len(clusters))

	var startedCount int
	for _, clusterInfo := range clusters {
		if atomic.LoadInt32(&m.isLeader) == 0 {
			logx.Info("[LeaderManager] 不再是 Leader，中断集群初始化")
			break
		}

		if err := m.addWatcherInternal(clusterInfo.Uuid, clusterInfo.Name); err != nil {
			logx.Errorf("[LeaderManager] 集群 %s(%s) 监听器启动失败: %v",
				clusterInfo.Name, clusterInfo.Uuid, err)
			continue
		}
		startedCount++
	}

	logx.Infof("[LeaderManager] 集群监听器启动完成: 成功 %d/%d",
		startedCount, len(clusters))

	return nil
}

// addWatcherInternal 内部方法：添加集群监听器
func (m *LeaderManager) addWatcherInternal(clusterUUID, clusterName string) error {
	client, err := m.k8sManager.GetCluster(m.ctx, clusterUUID)
	if err != nil {
		return fmt.Errorf("获取集群客户端失败: %v", err)
	}

	return m.startClusterWatcher(clusterUUID, clusterName, client.GetKubeClient())
}

// startCluerWatcher 启动集群监听器
func (m *LeaderManager) startClusterWatcher(clusterUUID, clusterName string, client kubernetes.Interface) error {
	m.mu.Lock()

	if _, exists := m.watchers[clusterUUID]; exists {
		m.mu.Unlock()
		return nil
	}

	if m.processor == nil || m.processor.IsStopped() {
		m.mu.Unlock()
		return ErrProcessorNotRunning
	}

	// 创建 watcher，直接使用 processor（实现了 EventEnqueuer 接口）
	watcher := NewClusterWatcher(clusterUUID, client, m.processor)
	m.watchers[clusterUUID] = watcher
	m.mu.Unlock()

	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		logx.Infof("[LeaderManager] 已添加集群 %s(%s) 的监听器", clusterName, clusterUUID)
		watcher.Start(m.ctx)
	}()

	return nil
}

// AddClusterWatcher 添加集群监听器（外部调用）
func (m *LeaderManager) AddClusterWatcher(clusterUUID string) error {
	if atomic.LoadInt32(&m.isRunning) == 0 {
		return ErrManagerNotRunning
	}
	if atomic.LoadInt32(&m.isLeader) == 0 {
		return fmt.Errorf("当前节点不是 Leader，无法添加监听器")
	}

	// 查询集群信息
	queryCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	clusterInfo, err := m.clusterModel.FindOneByUuid(queryCtx, clusterUUID)
	if err != nil {
		return fmt.Errorf("查询集群 %s 信息失败: %v", clusterUUID, err)
	}

	return m.addWatcherInternal(clusterUUID, clusterInfo.Name)
}

// RemoveClusterWatcher 移除集群监听器
func (m *LeaderManager) RemoveClusterWatcher(clusterUUID string) error {
	m.mu.Lock()
	watcher, exists := m.watchers[clusterUUID]
	if !exists {
		m.mu.Unlock()
		return nil
	}
	delete(m.watchers, clusterUUID)
	m.mu.Unlock()

	watcher.Stop()

	if !watcher.WaitForStop(m.watcherStopTimeout) {
		logx.Errorf("[LeaderManager] 等待集群 %s 监听器停止超时", clusterUUID)
	}

	logx.Infof("[LeaderManager] 已移除集群 %s 的监听器", clusterUUID)
	return nil
}

// HasClusterWatcher 检查是否存在指定集群的监听器
func (m *LeaderManager) HasClusterWatcher(clusterUUID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, exists := m.watchers[clusterUUID]
	return exists
}

// IsRunning 返回管理器是否正在运行
func (m *LeaderManager) IsRunning() bool {
	return atomic.LoadInt32(&m.isRunning) == 1
}

// IsLeader 返回当前节点是否是 Leader
func (m *LeaderManager) IsLeader() bool {
	return atomic.LoadInt32(&m.isLeader) == 1
}

// GetNodeID 返回节点 ID
func (m *LeaderManager) GetNodeID() string {
	return m.nodeID
}

// GetWatcherCount 返回当前监听器数量
func (m *LeaderManager) GetWatcherCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.watchers)
}

// GetStats 返回统计信息
func (m *LeaderManager) GetStats() *ManagerStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &ManagerStats{
		NodeID:       m.nodeID,
		IsRunning:    atomic.LoadInt32(&m.isRunning) == 1,
		WatcherCount: len(m.watchers),
		WorkerCount:  m.workerCount,
	}

	if stats.IsRunning {
		stats.Uptime = time.Since(m.startTime).Round(time.Second).String()
	}

	// 添加 Leader 信息
	if m.leaderElector != nil {
		info := m.leaderElector.GetInfo()
		stats.NodeID = fmt.Sprintf("%s (Leader: %v)", info.Identity, info.IsLeader)
	}

	clusters := make([]string, 0, len(m.watchers))
	watcherStats := make(map[string]*WatcherStats, len(m.watchers))
	for uuid, watcher := range m.watchers {
		clusters = append(clusters, uuid)
		watcherStats[uuid] = watcher.GetStats()
	}
	stats.Clusters = clusters
	stats.WatcherStats = watcherStats

	if m.processor != nil {
		stats.ProcessorStats = m.processor.GetStats()
	}

	return stats
}

// HealthCheck 健康检查
func (m *LeaderManager) HealthCheck() bool {
	if atomic.LoadInt32(&m.isRunning) == 0 {
		return false
	}
	// 如果是 Leader，检查 processor 健康状态
	if atomic.LoadInt32(&m.isLeader) == 1 && m.processor != nil {
		return m.processor.IsHealthy()
	}
	// 非 Leader 节点只要在运行就是健康的
	return true
}
