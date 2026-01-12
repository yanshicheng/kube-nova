package incremental

import (
	"context"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"k8s.io/client-go/kubernetes"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/common/k8smanager/cluster"
)

// ManagerConfig 管理器配置
type ManagerConfig struct {
	NodeID          string                 // 节点标识，为空则自动生成
	Redis           *redis.Redis           // Redis 客户端
	K8sManager      cluster.Manager        // K8s 集群管理器
	ClusterModel    model.OnecClusterModel // 集群数据库模型
	WorkerCount     int                    // 工作协程数，默认 10
	EventBuffer     int                    // 事件缓冲区大小，默认 2000
	DedupeWindow    time.Duration          // 去重时间窗口，默认 5s
	LockTTL         time.Duration          // 分布式锁 TTL，默认 30s
	ProcessTimeout  time.Duration          // 单个事件处理超时，默认 25s
	EnableAutoRenew bool                   // 是否启用锁自动续期
}

// Manager 增量同步管理器
type Manager struct {
	nodeID          string
	redis           *redis.Redis
	k8sManager      cluster.Manager
	clusterModel    model.OnecClusterModel
	workerCount     int
	eventBuffer     int
	dedupeWindow    time.Duration
	lockTTL         time.Duration
	processTimeout  time.Duration
	enableAutoRenew bool

	processor *EventProcessor
	handler   EventHandler
	watchers  map[string]*ClusterWatcher

	ctx       context.Context
	cancel    context.CancelFunc
	mu        sync.RWMutex
	wg        sync.WaitGroup
	isRunning int32
	stopOnce  sync.Once
	startTime time.Time
}

// NewManager 创建增量同步管理器
func NewManager(cfg ManagerConfig) *Manager {
	nodeID := cfg.NodeID
	if nodeID == "" {
		hostname, _ := os.Hostname()
		nodeID = fmt.Sprintf("%s-%d", hostname, os.Getpid())
	}

	if cfg.WorkerCount <= 0 {
		cfg.WorkerCount = 10
	}
	if cfg.EventBuffer <= 0 {
		cfg.EventBuffer = 2000
	}
	if cfg.DedupeWindow <= 0 {
		cfg.DedupeWindow = 5 * time.Second
	}
	if cfg.LockTTL <= 0 {
		cfg.LockTTL = 30 * time.Second
	}
	if cfg.ProcessTimeout <= 0 {
		cfg.ProcessTimeout = 25 * time.Second
	}

	return &Manager{
		nodeID:          nodeID,
		redis:           cfg.Redis,
		k8sManager:      cfg.K8sManager,
		clusterModel:    cfg.ClusterModel,
		workerCount:     cfg.WorkerCount,
		eventBuffer:     cfg.EventBuffer,
		dedupeWindow:    cfg.DedupeWindow,
		lockTTL:         cfg.LockTTL,
		processTimeout:  cfg.ProcessTimeout,
		enableAutoRenew: cfg.EnableAutoRenew,
		watchers:        make(map[string]*ClusterWatcher),
	}
}

// SetHandler 设置事件处理器
func (m *Manager) SetHandler(h EventHandler) {
	m.handler = h
}

// Start 启动增量同步管理器
func (m *Manager) Start() error {
	if !atomic.CompareAndSwapInt32(&m.isRunning, 0, 1) {
		logx.Errorf("[IncrementalSync] 管理器已经在运行中")
		return nil
	}

	if m.handler == nil {
		atomic.StoreInt32(&m.isRunning, 0)
		return fmt.Errorf("[IncrementalSync] EventHandler 未设置，请先调用 SetHandler()")
	}
	if m.k8sManager == nil {
		atomic.StoreInt32(&m.isRunning, 0)
		return fmt.Errorf("[IncrementalSync] K8sManager 未设置")
	}
	if m.clusterModel == nil {
		atomic.StoreInt32(&m.isRunning, 0)
		return fmt.Errorf("[IncrementalSync] ClusterModel 未设置")
	}

	m.startTime = time.Now()
	m.stopOnce = sync.Once{}
	m.ctx, m.cancel = context.WithCancel(context.Background())

	logx.Infof("[IncrementalSync] 正在启动增量同步管理器, NodeID=%s", m.nodeID)

	m.processor = NewEventProcessor(EventProcessorConfig{
		Redis:           m.redis,
		Handler:         m.handler,
		NodeID:          m.nodeID,
		WorkerCount:     m.workerCount,
		EventBuffer:     m.eventBuffer,
		DedupeWindow:    m.dedupeWindow,
		LockTTL:         m.lockTTL,
		ProcessTimeout:  m.processTimeout,
		EnableAutoRenew: m.enableAutoRenew,
	})

	m.processor.Start(m.ctx)

	if err := m.initClusterWatchers(); err != nil {
		logx.Errorf("[IncrementalSync] 初始化集群监听器失败: %v", err)
	}

	m.mu.RLock()
	watcherCount := len(m.watchers)
	m.mu.RUnlock()

	logx.Infof("[IncrementalSync] 增量同步管理器启动成功, NodeID=%s, Workers=%d, Buffer=%d, Watchers=%d",
		m.nodeID, m.workerCount, m.eventBuffer, watcherCount)

	return nil
}

// Stop 停止增量同步管理器（幂等）
func (m *Manager) Stop() error {
	m.stopOnce.Do(func() {
		if atomic.LoadInt32(&m.isRunning) == 0 {
			return
		}

		logx.Infof("[IncrementalSync] 正在停止增量同步管理器, NodeID=%s", m.nodeID)

		if m.cancel != nil {
			m.cancel()
		}

		m.mu.Lock()
		watcherCount := len(m.watchers)
		for clusterUUID, watcher := range m.watchers {
			logx.Debugf("[IncrementalSync] 停止集群 %s 的监听器", clusterUUID)
			watcher.Stop()
		}
		m.watchers = make(map[string]*ClusterWatcher)
		m.mu.Unlock()

		m.wg.Wait()

		if m.processor != nil {
			m.processor.Stop()
		}

		atomic.StoreInt32(&m.isRunning, 0)

		uptime := time.Since(m.startTime)
		logx.Infof("[IncrementalSync] 增量同步管理器已停止, NodeID=%s, 运行时长=%v, 停止监听器数=%d",
			m.nodeID, uptime.Round(time.Second), watcherCount)
	})

	return nil
}

func (m *Manager) initClusterWatchers() error {
	queryCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	clusters, err := m.clusterModel.GetAllClusters(queryCtx)
	if err != nil {
		return fmt.Errorf("查询集群列表失败: %v", err)
	}

	if len(clusters) == 0 {
		logx.Errorf("[IncrementalSync] 数据库中未发现任何集群")
		return nil
	}

	logx.Infof("[IncrementalSync] 发现 %d 个集群，开始启动监听器", len(clusters))

	var startedCount int
	for _, clusterInfo := range clusters {
		if atomic.LoadInt32(&m.isRunning) == 0 {
			logx.Error("[IncrementalSync] 管理器正在停止，中断集群初始化")
			break
		}

		if err := m.addWatcherInternal(clusterInfo.Uuid, clusterInfo.Name); err != nil {
			logx.Errorf("[IncrementalSync] 启动集群 %s(%s) 监听器失败: %v",
				clusterInfo.Name, clusterInfo.Uuid, err)
			continue
		}
		startedCount++
	}

	logx.Infof("[IncrementalSync] 成功启动 %d/%d 个集群监听器", startedCount, len(clusters))
	return nil
}

// AddClusterWatcher 添加集群监听器
func (m *Manager) AddClusterWatcher(clusterUUID string) error {
	if atomic.LoadInt32(&m.isRunning) == 0 {
		return fmt.Errorf("增量同步管理器未启动")
	}

	m.mu.RLock()
	if _, exists := m.watchers[clusterUUID]; exists {
		m.mu.RUnlock()
		return nil
	}
	m.mu.RUnlock()

	queryCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	clusterInfo, err := m.clusterModel.FindOneByUuid(queryCtx, clusterUUID)
	if err != nil {
		return fmt.Errorf("查询集群 %s 信息失败: %v", clusterUUID, err)
	}

	return m.addWatcherInternal(clusterUUID, clusterInfo.Name)
}

// RemoveClusterWatcher 移除集群监听器
func (m *Manager) RemoveClusterWatcher(clusterUUID string) error {
	m.mu.Lock()
	watcher, exists := m.watchers[clusterUUID]
	if !exists {
		m.mu.Unlock()
		return nil
	}
	delete(m.watchers, clusterUUID)
	m.mu.Unlock()

	watcher.Stop()

	logx.Infof("[IncrementalSync] 已移除集群 %s 的监听器", clusterUUID)
	return nil
}

// HasClusterWatcher 检查是否存在指定集群的监听器
func (m *Manager) HasClusterWatcher(clusterUUID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, exists := m.watchers[clusterUUID]
	return exists
}

// RestartClusterWatcher 重启集群监听器
func (m *Manager) RestartClusterWatcher(clusterUUID string) error {
	if err := m.RemoveClusterWatcher(clusterUUID); err != nil {
		return err
	}
	return m.AddClusterWatcher(clusterUUID)
}

func (m *Manager) addWatcherInternal(clusterUUID, clusterName string) error {
	client, err := m.k8sManager.GetCluster(m.ctx, clusterUUID)
	if err != nil {
		return fmt.Errorf("获取集群客户端失败: %v", err)
	}

	return m.startClusterWatcher(clusterUUID, clusterName, client.GetKubeClient())
}

func (m *Manager) startClusterWatcher(clusterUUID, clusterName string, client kubernetes.Interface) error {
	m.mu.Lock()

	if _, exists := m.watchers[clusterUUID]; exists {
		m.mu.Unlock()
		return nil
	}

	if m.processor == nil || m.processor.IsStopped() {
		m.mu.Unlock()
		return fmt.Errorf("事件处理器未运行")
	}

	watcher := NewClusterWatcher(clusterUUID, client, m.processor)
	m.watchers[clusterUUID] = watcher
	m.mu.Unlock()

	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		logx.Infof("[IncrementalSync] 已添加集群 %s(%s) 的监听器", clusterName, clusterUUID)
		watcher.Start(m.ctx)
	}()

	return nil
}

// IsRunning 返回管理器是否正在运行
func (m *Manager) IsRunning() bool {
	return atomic.LoadInt32(&m.isRunning) == 1
}

// GetNodeID 返回节点 ID
func (m *Manager) GetNodeID() string {
	return m.nodeID
}

// GetWatcherCount 返回当前监听器数量
func (m *Manager) GetWatcherCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.watchers)
}

// GetWatchedClusters 返回当前监听的所有集群 UUID 列表
func (m *Manager) GetWatchedClusters() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	clusters := make([]string, 0, len(m.watchers))
	for uuid := range m.watchers {
		clusters = append(clusters, uuid)
	}
	return clusters
}

// GetUptime 返回运行时长
func (m *Manager) GetUptime() time.Duration {
	if atomic.LoadInt32(&m.isRunning) == 0 {
		return 0
	}
	return time.Since(m.startTime)
}

// GetStats 返回统计信息
func (m *Manager) GetStats() *ManagerStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &ManagerStats{
		NodeID:       m.nodeID,
		IsRunning:    atomic.LoadInt32(&m.isRunning) == 1,
		WatcherCount: len(m.watchers),
		WorkerCount:  m.workerCount,
		EventBuffer:  m.eventBuffer,
		DedupeWindow: m.dedupeWindow.String(),
	}

	if stats.IsRunning {
		stats.Uptime = time.Since(m.startTime).Round(time.Second).String()
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
func (m *Manager) HealthCheck() bool {
	if atomic.LoadInt32(&m.isRunning) == 0 {
		return false
	}
	if m.processor != nil && !m.processor.IsHealthy() {
		return false
	}
	return true
}
