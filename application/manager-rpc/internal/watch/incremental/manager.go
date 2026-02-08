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
	NodeID             string                 // 节点标识，为空则自动生成
	Redis              *redis.Redis           // Redis 客户端
	K8sManager         cluster.Manager        // K8s 集群管理器
	ClusterModel       model.OnecClusterModel // 集群数据库模型
	WorkerCount        int                    // 工作协程数，默认 10
	EventBuffer        int                    // 事件缓冲区大小，默认 2000
	DedupeWindow       time.Duration          // 去重时间窗口，默认 5s
	LockTTL            time.Duration          // 分布式锁 TTL，默认 30s
	ProcessTimeout     time.Duration          // 单个事件处理超时，默认 25s
	EnableAutoRenew    bool                   // 是否启用锁自动续期
	WatcherStopTimeout time.Duration          // 监听器停止超时时间，默认 30s
}

// Manager 增量同步管理器
// 管理多个集群的资源监听器，协调事件处理，支持动态添加和删除集群
type Manager struct {
	nodeID             string
	redis              *redis.Redis
	k8sManager         cluster.Manager
	clusterModel       model.OnecClusterModel
	workerCount        int
	eventBuffer        int
	dedupeWindow       time.Duration
	lockTTL            time.Duration
	processTimeout     time.Duration
	enableAutoRenew    bool
	watcherStopTimeout time.Duration

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
		cfg.WorkerCount = 20
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
	if cfg.WatcherStopTimeout <= 0 {
		cfg.WatcherStopTimeout = 30 * time.Second
	}

	return &Manager{
		nodeID:             nodeID,
		redis:              cfg.Redis,
		k8sManager:         cfg.K8sManager,
		clusterModel:       cfg.ClusterModel,
		workerCount:        cfg.WorkerCount,
		eventBuffer:        cfg.EventBuffer,
		dedupeWindow:       cfg.DedupeWindow,
		lockTTL:            cfg.LockTTL,
		processTimeout:     cfg.ProcessTimeout,
		enableAutoRenew:    cfg.EnableAutoRenew,
		watcherStopTimeout: cfg.WatcherStopTimeout,
		watchers:           make(map[string]*ClusterWatcher),
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

	m.startTime = time.Now()
	m.stopOnce = sync.Once{}
	m.ctx, m.cancel = context.WithCancel(context.Background())

	logx.Infof("[IncrementalSync] 正在启动增量同步管理器, NodeID=%s", m.nodeID)

	// 创建事件处理器，设置事件完成回调
	m.processor = NewEventProcessor(EventProcessorConfig{
		Redis:            m.redis,
		Handler:          m.handler,
		NodeID:           m.nodeID,
		WorkerCount:      m.workerCount,
		EventBuffer:      m.eventBuffer,
		DedupeWindow:     m.dedupeWindow,
		LockTTL:          m.lockTTL,
		ProcessTimeout:   m.processTimeout,
		EnableAutoRenew:  m.enableAutoRenew,
		OnEventCompleted: m.onEventCompleted,
	})

	m.processor.Start(m.ctx)

	// 初始化集群监听器
	if err := m.initClusterWatchers(); err != nil {
		logx.Errorf("[IncrementalSync] 初始化集群监听器失败: %v", err)
	}

	m.mu.RLock()
	watcherCount := len(m.watchers)
	m.mu.RUnlock()

	logx.Infof("[IncrementalSync] 增量同步管理器启动成功, NodeID=%s, Workers=%d, Buffer=%d, Watchers=%d, StopTimeout=%v",
		m.nodeID, m.workerCount, m.eventBuffer, watcherCount, m.watcherStopTimeout)

	return nil
}

// Stop 停止增量同步管理器（幂等）
func (m *Manager) Stop() error {
	m.stopOnce.Do(func() {
		if atomic.LoadInt32(&m.isRunning) == 0 {
			return
		}

		logx.Infof("[IncrementalSync] 正在停止增量同步管理器, NodeID=%s", m.nodeID)

		// 先取消 context，通知所有组件停止
		if m.cancel != nil {
			m.cancel()
		}

		// 停止所有 watcher
		m.mu.Lock()
		watcherCount := len(m.watchers)
		for clusterUUID, watcher := range m.watchers {
			logx.Debugf("[IncrementalSync] 停止集群 %s 的监听器", clusterUUID)
			watcher.Stop()
		}
		m.watchers = make(map[string]*ClusterWatcher)
		m.mu.Unlock()

		// 等待所有 watcher goroutine 退出
		m.wg.Wait()

		// 最后停止 processor（确保所有事件都有机会被处理）
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

// onEventCompleted 事件处理完成回调
// 由 EventProcessor 在每个事件处理完成后调用，用于更新对应 Watcher 的待处理计数
func (m *Manager) onEventCompleted(clusterUUID string) {
	m.mu.RLock()
	watcher, exists := m.watchers[clusterUUID]
	m.mu.RUnlock()

	if exists && watcher != nil {
		watcher.DecrementPending()
	}
}

// initClusterWatchers 初始化集群监听器
// 从数据库查询所有集群并启动对应的监听器
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

	const maxRetries = 3
	var startedCount int
	var failedClusters []string

	for _, clusterInfo := range clusters {
		if atomic.LoadInt32(&m.isRunning) == 0 {
			logx.Error("[IncrementalSync] 管理器正在停止，中断集群初始化")
			break
		}

		// 重试逻辑
		var lastErr error
		success := false
		for retry := 0; retry < maxRetries; retry++ {
			if retry > 0 {
				logx.Infof("[IncrementalSync] 第 %d 次重试启动集群 %s(%s) 监听器",
					retry+1, clusterInfo.Name, clusterInfo.Uuid)
				time.Sleep(time.Duration(retry) * time.Second) // 递增等待
			}

			if err := m.addWatcherInternal(clusterInfo.Uuid, clusterInfo.Name); err != nil {
				lastErr = err
				continue
			}
			success = true
			break
		}

		if success {
			startedCount++
		} else {
			failedClusters = append(failedClusters, fmt.Sprintf("%s(%s)", clusterInfo.Name, clusterInfo.Uuid))
			logx.Errorf("[IncrementalSync] 集群 %s(%s) 监听器启动失败（已重试 %d 次）: %v",
				clusterInfo.Name, clusterInfo.Uuid, maxRetries, lastErr)
		}
	}

	// 输出最终结果
	logx.Infof("[IncrementalSync] 集群监听器启动完成: 成功 %d 个, 失败 %d 个",
		startedCount, len(failedClusters))

	if len(failedClusters) > 0 {
		logx.Errorf("[IncrementalSync] 以下集群监听器启动失败，请检查集群连接状态: %v", failedClusters)
	}

	return nil
}

// AddClusterWatcher 添加集群监听器
// 如果集群已存在监听器，则会先停止旧的监听器再启动新的（重启行为）
// 此方法会阻塞直到监听器启动完成
func (m *Manager) AddClusterWatcher(clusterUUID string) error {
	if atomic.LoadInt32(&m.isRunning) == 0 {
		return ErrManagerNotRunning
	}

	// 使用写锁保护整个操作流程，避免竞态条件
	m.mu.Lock()

	// 检查是否已存在，如果存在则先停止（实现重启行为）
	if existingWatcher, exists := m.watchers[clusterUUID]; exists {
		logx.Infof("[IncrementalSync] 集群 %s 的监听器已存在，将重启", clusterUUID)

		// 从 map 中移除
		delete(m.watchers, clusterUUID)
		m.mu.Unlock()

		// 停止旧的 watcher（在锁外进行，避免死锁）
		existingWatcher.Stop()

		// 等待旧 watcher 完全停止
		if !existingWatcher.WaitForStop(m.watcherStopTimeout) {
			logx.Errorf("[IncrementalSync] 等待集群 %s 旧监听器停止超时", clusterUUID)
		}

		// 等待待处理事件完成
		if !existingWatcher.WaitForPendingEvents(m.watcherStopTimeout) {
			logx.Errorf("[IncrementalSync] 等待集群 %s 待处理事件完成超时", clusterUUID)
		}

		// 重新获取锁继续添加
		m.mu.Lock()
	}
	m.mu.Unlock()

	// 查询集群信息
	queryCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	clusterInfo, err := m.clusterModel.FindOneByUuid(queryCtx, clusterUUID)
	if err != nil {
		return fmt.Errorf("查询集群 %s 信息失败: %v", clusterUUID, err)
	}

	return m.addWatcherInternal(clusterUUID, clusterInfo.Name)
}

// RemoveClusterWatcher 移除集群监听器（同步版本，等待完成）
// 此方法会阻塞直到监听器完全停止且所有待处理事件处理完成
func (m *Manager) RemoveClusterWatcher(clusterUUID string) error {
	return m.removeClusterWatcherWithOptions(clusterUUID, true)
}

// RemoveClusterWatcherAsync 移除集群监听器（异步版本，不等待）
// 此方法只发送停止信号，立即返回，监听器会在后台停止
func (m *Manager) RemoveClusterWatcherAsync(clusterUUID string) error {
	return m.removeClusterWatcherWithOptions(clusterUUID, false)
}

// removeClusterWatcherWithOptions 移除集群监听器的内部实现
// waitForCompletion 参数控制是否等待监听器完全停止
func (m *Manager) removeClusterWatcherWithOptions(clusterUUID string, waitForCompletion bool) error {
	m.mu.Lock()
	watcher, exists := m.watchers[clusterUUID]
	if !exists {
		m.mu.Unlock()
		logx.Debugf("[IncrementalSync] 集群 %s 的监听器不存在，无需移除", clusterUUID)
		return nil
	}
	delete(m.watchers, clusterUUID)
	m.mu.Unlock()

	// 发送停止信号
	watcher.Stop()

	if !waitForCompletion {
		logx.Infof("[IncrementalSync] 已发送停止信号到集群 %s 的监听器（异步模式）", clusterUUID)
		return nil
	}

	// 等待 watcher 停止
	logx.Infof("[IncrementalSync] 等待集群 %s 的监听器停止...", clusterUUID)
	if !watcher.WaitForStop(m.watcherStopTimeout) {
		logx.Errorf("[IncrementalSync] 等待集群 %s 监听器停止超时", clusterUUID)
		return fmt.Errorf("%w: cluster %s", ErrWatcherStopTimeout, clusterUUID)
	}

	// 等待待处理事件完成
	pendingCount := watcher.GetPendingCount()
	if pendingCount > 0 {
		logx.Infof("[IncrementalSync] 等待集群 %s 的 %d 个待处理事件完成...", clusterUUID, pendingCount)
		if !watcher.WaitForPendingEvents(m.watcherStopTimeout) {
			remaining := watcher.GetPendingCount()
			logx.Errorf("[IncrementalSync] 等待集群 %s 待处理事件超时，剩余 %d 个", clusterUUID, remaining)
			return fmt.Errorf("%w: cluster %s, remaining events: %d", ErrWatcherStopTimeout, clusterUUID, remaining)
		}
	}

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
// 等效于先移除再添加，会等待旧监听器完全停止后再启动新的
func (m *Manager) RestartClusterWatcher(clusterUUID string) error {
	if err := m.RemoveClusterWatcher(clusterUUID); err != nil {
		return fmt.Errorf("移除监听器失败: %w", err)
	}
	return m.AddClusterWatcher(clusterUUID)
}

// SyncClustersFromDB 从数据库同步集群监听器
// 自动添加新集群的监听器，移除已不存在集群的监听器
// 返回同步结果，包含成功和失败的集群列表
func (m *Manager) SyncClustersFromDB() (*SyncResult, error) {
	if atomic.LoadInt32(&m.isRunning) == 0 {
		return nil, ErrManagerNotRunning
	}

	// 查询数据库中的所有集群
	queryCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	clusters, err := m.clusterModel.GetAllClusters(queryCtx)
	if err != nil {
		return nil, fmt.Errorf("查询集群列表失败: %v", err)
	}

	// 构建目标集群 UUID 集合
	targetClusters := make(map[string]string) // uuid -> name
	for _, c := range clusters {
		targetClusters[c.Uuid] = c.Name
	}

	// 获取当前监听的集群列表
	m.mu.RLock()
	currentClusters := make(map[string]bool)
	for uuid := range m.watchers {
		currentClusters[uuid] = true
	}
	m.mu.RUnlock()

	result := &SyncResult{
		Added:        make([]string, 0),
		Removed:      make([]string, 0),
		AddErrors:    make(map[string]error),
		RemoveErrors: make(map[string]error),
	}

	// 找出需要添加的集群（在数据库中但不在当前监听列表中）
	var toAdd []string
	for uuid := range targetClusters {
		if !currentClusters[uuid] {
			toAdd = append(toAdd, uuid)
		}
	}

	// 找出需要移除的集群（在当前监听列表中但不在数据库中）
	var toRemove []string
	for uuid := range currentClusters {
		if _, exists := targetClusters[uuid]; !exists {
			toRemove = append(toRemove, uuid)
		}
	}

	logx.Infof("[IncrementalSync] 同步集群监听器: 需要添加 %d 个, 需要移除 %d 个",
		len(toAdd), len(toRemove))

	// 先移除不再需要的监听器
	for _, uuid := range toRemove {
		if err := m.RemoveClusterWatcher(uuid); err != nil {
			result.RemoveErrors[uuid] = err
			logx.Errorf("[IncrementalSync] 移除集群 %s 监听器失败: %v", uuid, err)
		} else {
			result.Removed = append(result.Removed, uuid)
		}
	}

	// 添加新的监听器
	for _, uuid := range toAdd {
		if err := m.AddClusterWatcher(uuid); err != nil {
			result.AddErrors[uuid] = err
			logx.Errorf("[IncrementalSync] 添加集群 %s 监听器失败: %v", uuid, err)
		} else {
			result.Added = append(result.Added, uuid)
		}
	}

	logx.Infof("[IncrementalSync] 同步完成: 添加成功 %d/%d, 移除成功 %d/%d",
		len(result.Added), len(toAdd), len(result.Removed), len(toRemove))

	return result, nil
}

// addWatcherInternal 内部方法：添加集群监听器
func (m *Manager) addWatcherInternal(clusterUUID, clusterName string) error {
	client, err := m.k8sManager.GetCluster(m.ctx, clusterUUID)
	if err != nil {
		return fmt.Errorf("获取集群客户端失败: %v", err)
	}

	return m.startClusterWatcher(clusterUUID, clusterName, client.GetKubeClient())
}

// startClusterWatcher 启动集群监听器
func (m *Manager) startClusterWatcher(clusterUUID, clusterName string, client kubernetes.Interface) error {
	m.mu.Lock()

	// 再次检查是否已存在（双重检查）
	if _, exists := m.watchers[clusterUUID]; exists {
		m.mu.Unlock()
		return nil
	}

	if m.processor == nil || m.processor.IsStopped() {
		m.mu.Unlock()
		return ErrProcessorNotRunning
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

// GetWatcherStopTimeout 获取监听器停止超时时间
func (m *Manager) GetWatcherStopTimeout() time.Duration {
	return m.watcherStopTimeout
}

// SetWatcherStopTimeout 设置监听器停止超时时间
// 注意：建议在 Start 之前设置，运行时修改可能导致行为不一致
func (m *Manager) SetWatcherStopTimeout(timeout time.Duration) {
	m.watcherStopTimeout = timeout
}
