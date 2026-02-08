package incremental

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
	"k8s.io/client-go/util/workqueue"
)

// WorkQueueProcessor 基于 WorkQueue 的事件处理器
// 使用 K8s 原生的 WorkQueue 实现，无需 Redis 分布式锁
// 适用于 Leader Election 模式，只有 Leader 节点运行
type WorkQueueProcessor struct {
	handler        EventHandler
	workerCount    int
	processTimeout time.Duration

	// WorkQueue - 使用限速队列，支持去重和重试
	queue workqueue.RateLimitingInterface

	// 事件处理完成回调
	onEventCompleted func(clusterUUID string)

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// 状态控制
	started  int32
	stopped  int32
	stopOnce sync.Once

	// 统计信息
	processedCount int64
	errorCount     int64
	requeueCount   int64

	// 按集群统计
	clusterStatsMu sync.RWMutex
	clusterStats   map[string]*ClusterEventStats

	// 事件缓存（用于从 key 获取完整事件）
	eventCacheMu sync.RWMutex
	eventCache   map[string]*ResourceEvent
}

// WorkQueueProcessorConfig 配置
type WorkQueueProcessorConfig struct {
	Handler          EventHandler             // 事件处理器
	WorkerCount      int                      // 工作协程数量，默认 10
	ProcessTimeout   time.Duration            // 单个事件处理超时，默认 30s
	OnEventCompleted func(clusterUUID string) // 事件处理完成回调
}

// NewWorkQueueProcessor 创建基于 WorkQueue 的事件处理器
func NewWorkQueueProcessor(cfg WorkQueueProcessorConfig) *WorkQueueProcessor {
	if cfg.WorkerCount <= 0 {
		cfg.WorkerCount = 10
	}
	if cfg.ProcessTimeout <= 0 {
		cfg.ProcessTimeout = 30 * time.Second
	}

	// 创建限速队列
	// 使用默认的指数退避限速器：
	// - 基础延迟 5ms
	// - 最大延迟 1000s
	// - 每次失败延迟翻倍
	queue := workqueue.NewRateLimitingQueue(
		workqueue.DefaultControllerRateLimiter(),
	)

	return &WorkQueueProcessor{
		handler:          cfg.Handler,
		workerCount:      cfg.WorkerCount,
		processTimeout:   cfg.ProcessTimeout,
		queue:            queue,
		onEventCompleted: cfg.OnEventCompleted,
		clusterStats:     make(map[string]*ClusterEventStats),
		eventCache:       make(map[string]*ResourceEvent),
	}
}

// Start 启动处理器
func (p *WorkQueueProcessor) Start(ctx context.Context) {
	if !atomic.CompareAndSwapInt32(&p.started, 0, 1) {
		logx.Error("[WorkQueueProcessor] 已经启动，跳过重复启动")
		return
	}

	p.ctx, p.cancel = context.WithCancel(ctx)

	// 启动 worker
	for i := 0; i < p.workerCount; i++ {
		p.wg.Add(1)
		go p.worker(i)
	}

	logx.Infof("[WorkQueueProcessor] 启动完成，工作协程: %d, 处理超时: %v",
		p.workerCount, p.processTimeout)
}

// Stop 停止处理器
func (p *WorkQueueProcessor) Stop() {
	p.stopOnce.Do(func() {
		logx.Info("[WorkQueueProcessor] 开始停止...")

		atomic.StoreInt32(&p.stopped, 1)

		// 关闭队列，不再接受新事件
		p.queue.ShutDown()

		// 取消 context
		if p.cancel != nil {
			p.cancel()
		}

		// 等待所有 worker 退出
		p.wg.Wait()

		// 清理事件缓存
		p.eventCacheMu.Lock()
		p.eventCache = make(map[string]*ResourceEvent)
		p.eventCacheMu.Unlock()

		logx.Infof("[WorkQueueProcessor] 已停止, 处理: %d, 重入队: %d, 错误: %d",
			atomic.LoadInt64(&p.processedCount),
			atomic.LoadInt64(&p.requeueCount),
			atomic.LoadInt64(&p.errorCount))
	})
}

// EnqueueEvent 将事件放入队列
func (p *WorkQueueProcessor) EnqueueEvent(event *ResourceEvent) bool {
	if atomic.LoadInt32(&p.stopped) == 1 {
		logx.Debugf("[WorkQueueProcessor] 已停止，丢弃事件: %s/%s/%s/%s",
			event.ClusterUUID, event.ResourceType, event.Namespace, event.Name)
		return false
	}

	// 设置入队时间
	if event.EnqueueTime.IsZero() {
		event.EnqueueTime = time.Now()
	}

	// 生成事件 key
	key := p.eventKey(event)

	// 缓存事件（用于后续处理时获取完整信息）
	p.eventCacheMu.Lock()
	p.eventCache[key] = event
	p.eventCacheMu.Unlock()

	// 放入队列（WorkQueue 自动去重：同一个 key 多次 Add 只会处理一次）
	p.queue.Add(key)

	return true
}

// worker 工作协程
func (p *WorkQueueProcessor) worker(id int) {
	defer p.wg.Done()

	for {
		// 从队列获取事件 key
		key, shutdown := p.queue.Get()
		if shutdown {
			logx.Debugf("[WorkQueueProcessor] Worker %d 收到关闭信号，退出", id)
			return
		}

		// 处理事件
		p.processKey(key.(string))
	}
}

// processKey 处理单个事件 key
func (p *WorkQueueProcessor) processKey(key string) {
	// 标记处理完成（无论成功失败）
	defer p.queue.Done(key)

	// 从缓存获取事件
	p.eventCacheMu.RLock()
	event, exists := p.eventCache[key]
	p.eventCacheMu.RUnlock()

	if !exists {
		logx.Errorf("[WorkQueueProcessor] 事件缓存中找不到 key: %s", key)
		return
	}

	// 确保通知事件处理完成
	defer p.notifyEventCompleted(event.ClusterUUID)

	// 检查 context
	select {
	case <-p.ctx.Done():
		// context 取消，DELETE 事件仍尝试处理
		if !event.IsDelete() {
			return
		}
		logx.Infof("[WorkQueueProcessor] Context 已取消，但仍尝试处理 DELETE 事件: %s/%s/%s/%s",
			event.ClusterUUID, event.ResourceType, event.Namespace, event.Name)
	default:
	}

	// 创建带超时的 context
	processCtx, cancel := context.WithTimeout(p.ctx, p.processTimeout)
	defer cancel()

	// 处理事件
	var processErr error
	switch event.ResourceType {
	case "namespace":
		processErr = p.handler.HandleNamespaceEvent(processCtx, event)
	case "deployment":
		processErr = p.handler.HandleDeploymentEvent(processCtx, event)
	case "statefulset":
		processErr = p.handler.HandleStatefulSetEvent(processCtx, event)
	case "daemonset":
		processErr = p.handler.HandleDaemonSetEvent(processCtx, event)
	case "cronjob":
		processErr = p.handler.HandleCronJobEvent(processCtx, event)
	case "resourcequota":
		processErr = p.handler.HandleResourceQuotaEvent(processCtx, event)
	case "limitrange":
		processErr = p.handler.HandleLimitRangeEvent(processCtx, event)
	default:
		logx.Errorf("[WorkQueueProcessor] 未知资源类型: %s", event.ResourceType)
		p.removeFromCache(key)
		return
	}

	// 处理结果
	if processErr != nil {
		atomic.AddInt64(&p.errorCount, 1)
		p.updateClusterStats(event.ClusterUUID, false)

		if errors.Is(processCtx.Err(), context.DeadlineExceeded) {
			logx.Errorf("[WorkQueueProcessor] 处理事件超时: %s/%s/%s/%s/%s",
				event.Type, event.ClusterUUID, event.ResourceType, event.Namespace, event.Name)
		} else if errors.Is(processCtx.Err(), context.Canceled) {
			logx.Debugf("[WorkQueueProcessor] Context 取消: %s/%s/%s/%s",
				event.ClusterUUID, event.ResourceType, event.Namespace, event.Name)
			return
		} else {
			logx.Errorf("[WorkQueueProcessor] 处理事件失败: %s/%s/%s/%s/%s, err: %v",
				event.Type, event.ClusterUUID, event.ResourceType, event.Namespace, event.Name, processErr)
		}

		// 重新入队（WorkQueue 会自动应用退避策略）
		p.queue.AddRateLimited(key)
		atomic.AddInt64(&p.requeueCount, 1)
		logx.Infof("[WorkQueueProcessor] 事件重新入队: %s/%s/%s/%s, requeues=%d",
			event.ClusterUUID, event.ResourceType, event.Namespace, event.Name,
			p.queue.NumRequeues(key))
	} else {
		// 处理成功
		atomic.AddInt64(&p.processedCount, 1)
		p.updateClusterStats(event.ClusterUUID, true)

		// 从缓存中移除
		p.removeFromCache(key)

		// 清除重试计数
		p.queue.Forget(key)

		latency := time.Since(event.EnqueueTime)
		logx.Infof("[WorkQueueProcessor] 事件处理成功: %s/%s/%s/%s/%s (延迟: %v)",
			event.Type, event.ClusterUUID, event.ResourceType, event.Namespace, event.Name,
			latency.Round(time.Millisecond))
	}
}

// eventKey 生成事件 key
func (p *WorkQueueProcessor) eventKey(event *ResourceEvent) string {
	return fmt.Sprintf("%s/%s/%s/%s", event.ClusterUUID, event.ResourceType, event.Namespace, event.Name)
}

// removeFromCache 从缓存中移除事件
func (p *WorkQueueProcessor) removeFromCache(key string) {
	p.eventCacheMu.Lock()
	delete(p.eventCache, key)
	p.eventCacheMu.Unlock()
}

// notifyEventCompleted 通知事件处理完成
func (p *WorkQueueProcessor) notifyEventCompleted(clusterUUID string) {
	if p.onEventCompleted != nil {
		p.onEventCompleted(clusterUUID)
	}
}

// updateClusterStats 更新集群统计
func (p *WorkQueueProcessor) updateClusterStats(clusterUUID string, success bool) {
	p.clusterStatsMu.Lock()
	defer p.clusterStatsMu.Unlock()

	stats, exists := p.clusterStats[clusterUUID]
	if !exists {
		stats = &ClusterEventStats{ClusterUUID: clusterUUID}
		p.clusterStats[clusterUUID] = stats
	}

	if success {
		stats.ProcessedCount++
	} else {
		stats.ErrorCount++
	}
}

// GetStats 获取统计信息
func (p *WorkQueueProcessor) GetStats() *ProcessorStats {
	stats := &ProcessorStats{
		ProcessedCount: atomic.LoadInt64(&p.processedCount),
		ErrorCount:     atomic.LoadInt64(&p.errorCount),
		RetryCount:     atomic.LoadInt64(&p.requeueCount),
		QueueLength:    p.queue.Len(),
		WorkerCount:    p.workerCount,
	}

	p.clusterStatsMu.RLock()
	if len(p.clusterStats) > 0 {
		stats.ClusterStats = make(map[string]*ClusterEventStats, len(p.clusterStats))
		for k, v := range p.clusterStats {
			stats.ClusterStats[k] = &ClusterEventStats{
				ClusterUUID:    v.ClusterUUID,
				ProcessedCount: v.ProcessedCount,
				ErrorCount:     v.ErrorCount,
			}
		}
	}
	p.clusterStatsMu.RUnlock()

	return stats
}

// IsHealthy 健康检查
func (p *WorkQueueProcessor) IsHealthy() bool {
	if atomic.LoadInt32(&p.stopped) == 1 {
		return false
	}
	// 队列长度超过 1000 认为不健康
	return p.queue.Len() < 1000
}

// IsStopped 是否已停止
func (p *WorkQueueProcessor) IsStopped() bool {
	return atomic.LoadInt32(&p.stopped) == 1
}

// GetQueueLength 获取队列长度
func (p *WorkQueueProcessor) GetQueueLength() int {
	return p.queue.Len()
}
