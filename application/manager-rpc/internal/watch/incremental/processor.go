package incremental

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

// EventProcessor 事件处理器
// 负责接收来自各集群监听器的事件，通过分布式锁协调多副本处理，确保同一事件只被处理一次
type EventProcessor struct {
	redis          *redis.Redis
	locker         DistributedLocker
	handler        EventHandler
	eventChan      chan *ResourceEvent // 普通事件通道
	priorityChan   chan *ResourceEvent // 高优先级事件通道（DELETE 事件）
	workerCount    int
	dedupeWindow   time.Duration
	lockTTL        time.Duration
	processTimeout time.Duration

	// 事件处理完成回调，用于通知 Watcher 事件已处理
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
	droppedCount   int64
	errorCount     int64
	skippedCount   int64
	retryCount     int64
	deleteDropped  int64

	// 按集群统计
	clusterStatsMu sync.RWMutex
	clusterStats   map[string]*ClusterEventStats

	// 用于优雅关闭时等待 DELETE 事件重试 goroutine
	retryWg sync.WaitGroup
}

// EventProcessorConfig 事件处理器配置
type EventProcessorConfig struct {
	Redis            *redis.Redis             // Redis 客户端，用于分布式锁和去重
	Handler          EventHandler             // 事件处理器接口实现
	NodeID           string                   // 当前节点标识
	WorkerCount      int                      // 工作协程数量，默认 10
	EventBuffer      int                      // 事件缓冲区大小，默认 2000
	DedupeWindow     time.Duration            // 去重时间窗口，默认 5 秒
	LockTTL          time.Duration            // 分布式锁过期时间，默认 30 秒
	ProcessTimeout   time.Duration            // 单个事件处理超时，默认 25 秒
	EnableAutoRenew  bool                     // 是否启用锁自动续期
	OnEventCompleted func(clusterUUID string) // 事件处理完成回调
}

// NewEventProcessor 创建事件处理器
func NewEventProcessor(cfg EventProcessorConfig) *EventProcessor {
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
	if cfg.ProcessTimeout >= cfg.LockTTL {
		cfg.ProcessTimeout = cfg.LockTTL - 5*time.Second
		if cfg.ProcessTimeout <= 0 {
			cfg.ProcessTimeout = cfg.LockTTL / 2
		}
	}

	var locker DistributedLocker
	if cfg.Redis != nil {
		if cfg.EnableAutoRenew {
			locker = NewLockWithAutoRenew(cfg.Redis, cfg.NodeID)
		} else {
			locker = NewRedisDistributedLock(cfg.Redis, cfg.NodeID)
		}
	} else {
		locker = NewNoopLocker(cfg.NodeID)
	}

	return &EventProcessor{
		redis:            cfg.Redis,
		locker:           locker,
		handler:          cfg.Handler,
		eventChan:        make(chan *ResourceEvent, cfg.EventBuffer),
		priorityChan:     make(chan *ResourceEvent, cfg.EventBuffer/4),
		workerCount:      cfg.WorkerCount,
		dedupeWindow:     cfg.DedupeWindow,
		lockTTL:          cfg.LockTTL,
		processTimeout:   cfg.ProcessTimeout,
		onEventCompleted: cfg.OnEventCompleted,
		clusterStats:     make(map[string]*ClusterEventStats),
	}
}

// Start 启动事件处理器
func (p *EventProcessor) Start(ctx context.Context) {
	if !atomic.CompareAndSwapInt32(&p.started, 0, 1) {
		logx.Error("[EventProcessor] 已经启动，跳过重复启动")
		return
	}

	p.ctx, p.cancel = context.WithCancel(ctx)

	// 启动普通 worker（同时也会处理高优先级事件）
	for i := 0; i < p.workerCount; i++ {
		p.wg.Add(1)
		go p.worker(i)
	}

	// 启动专用高优先级 worker（只处理 DELETE 等高优先级事件）
	priorityWorkerCount := p.workerCount / 4
	if priorityWorkerCount < 2 {
		priorityWorkerCount = 2
	}
	for i := 0; i < priorityWorkerCount; i++ {
		p.wg.Add(1)
		go p.priorityWorker(i)
	}

	logx.Infof("[EventProcessor] 启动完成，普通工作协程: %d, 高优先级协程: %d, 普通缓冲区: %d, 高优先级缓冲区: %d, 锁TTL: %v, 处理超时: %v",
		p.workerCount, priorityWorkerCount, cap(p.eventChan), cap(p.priorityChan), p.lockTTL, p.processTimeout)
}

// Stop 停止事件处理器（幂等，可多次调用）
func (p *EventProcessor) Stop() {
	p.stopOnce.Do(func() {
		logx.Info("[EventProcessor] 开始停止...")

		// 标记为停止状态
		atomic.StoreInt32(&p.stopped, 1)

		// 等待所有重试 goroutine 完成（给一定的超时时间）
		done := make(chan struct{})
		go func() {
			p.retryWg.Wait()
			close(done)
		}()

		select {
		case <-done:
			logx.Info("[EventProcessor] 所有 DELETE 重试 goroutine 已完成")
		case <-time.After(30 * time.Second):
			logx.Errorf("[EventProcessor] 等待 DELETE 重试 goroutine 超时")
		}

		// 取消 context，通知所有 worker 退出
		if p.cancel != nil {
			p.cancel()
		}

		// 等待所有 worker 退出
		p.wg.Wait()

		logx.Infof("[EventProcessor] 已停止, 处理: %d, 跳过: %d, 重试: %d, 丢弃: %d, DELETE丢弃: %d, 错误: %d",
			atomic.LoadInt64(&p.processedCount),
			atomic.LoadInt64(&p.skippedCount),
			atomic.LoadInt64(&p.retryCount),
			atomic.LoadInt64(&p.droppedCount),
			atomic.LoadInt64(&p.deleteDropped),
			atomic.LoadInt64(&p.errorCount))
	})
}

// EnqueueEvent 将事件放入队列
// DELETE 事件使用带超时的阻塞入队，确保不会被轻易丢弃
func (p *EventProcessor) EnqueueEvent(event *ResourceEvent) bool {
	// 设置入队时间
	if event.EnqueueTime.IsZero() {
		event.EnqueueTime = time.Now()
	}

	// 检查是否已停止
	if atomic.LoadInt32(&p.stopped) == 1 {
		if event.IsDelete() {
			atomic.AddInt64(&p.deleteDropped, 1)
			logx.Errorf("[EventProcessor] Processor 已停止，DELETE 事件被丢弃: %s/%s/%s/%s (重试次数: %d)",
				event.ClusterUUID, event.ResourceType, event.Namespace, event.Name, event.RetryCount)
		}
		return false
	}

	// DELETE 事件特殊处理：优先放入高优先级队列，如果满了则带超时等待
	if event.IsDelete() {
		return p.enqueueDeleteEvent(event)
	}

	// 非 DELETE 事件：非阻塞发送到普通队列
	select {
	case p.eventChan <- event:
		return true
	default:
		atomic.AddInt64(&p.droppedCount, 1)
		logx.Errorf("[EventProcessor] 普通事件队列已满，丢弃事件: %s/%s/%s/%s/%s",
			event.Type, event.ClusterUUID, event.ResourceType, event.Namespace, event.Name)
		return false
	}
}

// enqueueDeleteEvent DELETE 事件入队逻辑
// 返回 true 表示成功入队，false 表示入队失败
func (p *EventProcessor) enqueueDeleteEvent(event *ResourceEvent) bool {
	// 1. 首先尝试非阻塞发送到高优先级队列
	select {
	case p.priorityChan <- event:
		logx.Debugf("[EventProcessor] DELETE 事件入队高优先级队列: %s/%s/%s/%s",
			event.ClusterUUID, event.ResourceType, event.Namespace, event.Name)
		return true
	default:
	}

	// 2. 高优先级队列满，尝试非阻塞发送到普通队列
	select {
	case p.eventChan <- event:
		logx.Infof("[EventProcessor] DELETE 事件降级到普通队列: %s/%s/%s/%s",
			event.ClusterUUID, event.ResourceType, event.Namespace, event.Name)
		return true
	default:
	}

	// 3. 两个队列都满，使用带超时的阻塞等待
	logx.Errorf("[EventProcessor] 所有队列已满，DELETE 事件进入阻塞等待: %s/%s/%s/%s (超时: %v)",
		event.ClusterUUID, event.ResourceType, event.Namespace, event.Name, DeleteEnqueueTimeout)

	timeout := time.NewTimer(DeleteEnqueueTimeout)
	defer timeout.Stop()

	select {
	case p.priorityChan <- event:
		logx.Infof("[EventProcessor] DELETE 事件阻塞等待后成功入队高优先级队列: %s/%s/%s/%s",
			event.ClusterUUID, event.ResourceType, event.Namespace, event.Name)
		return true
	case p.eventChan <- event:
		logx.Infof("[EventProcessor] DELETE 事件阻塞等待后成功入队普通队列: %s/%s/%s/%s",
			event.ClusterUUID, event.ResourceType, event.Namespace, event.Name)
		return true
	case <-timeout.C:
		atomic.AddInt64(&p.deleteDropped, 1)
		logx.Errorf("[EventProcessor] DELETE 事件入队超时，被迫丢弃: %s/%s/%s/%s (重试次数: %d, 等待: %v)",
			event.ClusterUUID, event.ResourceType, event.Namespace, event.Name, event.RetryCount, DeleteEnqueueTimeout)
		return false
	case <-p.ctx.Done():
		atomic.AddInt64(&p.deleteDropped, 1)
		logx.Errorf("[EventProcessor] Processor 正在关闭，DELETE 事件被丢弃: %s/%s/%s/%s",
			event.ClusterUUID, event.ResourceType, event.Namespace, event.Name)
		return false
	}
}

// worker 普通工作协程
// 优先处理高优先级队列，使用嵌套 select 而非并列 case
func (p *EventProcessor) worker(id int) {
	defer p.wg.Done()

	for {
		// 检查是否需要退出
		select {
		case <-p.ctx.Done():
			p.drainEvents(id, false)
			logx.Debugf("[EventProcessor] Worker %d 已退出", id)
			return
		default:
		}

		// 优先处理高优先级队列（使用嵌套 select 确保优先级）
		select {
		case event := <-p.priorityChan:
			if event != nil {
				p.processEvent(event)
			}
		default:
			// 高优先级队列为空，再处理普通队列或等待
			select {
			case <-p.ctx.Done():
				p.drainEvents(id, false)
				logx.Debugf("[EventProcessor] Worker %d 已退出", id)
				return
			case event := <-p.priorityChan:
				if event != nil {
					p.processEvent(event)
				}
			case event := <-p.eventChan:
				if event != nil {
					p.processEvent(event)
				}
			}
		}
	}
}

// priorityWorker 高优先级工作协程（专门处理 DELETE 事件）
func (p *EventProcessor) priorityWorker(id int) {
	defer p.wg.Done()

	for {
		select {
		case <-p.ctx.Done():
			p.drainEvents(id, true)
			logx.Debugf("[EventProcessor] PriorityWorker %d 已退出", id)
			return
		case event := <-p.priorityChan:
			if event != nil {
				p.processEvent(event)
			}
		}
	}
}

// drainEvents 处理队列中剩余的事件（优雅关闭时调用）
func (p *EventProcessor) drainEvents(workerID int, priorityOnly bool) {
	drainCount := 0
	deleteCount := 0

	// 先处理高优先级队列（主要是 DELETE 事件）
	for {
		select {
		case event := <-p.priorityChan:
			if event != nil {
				p.processEvent(event)
				drainCount++
				if event.IsDelete() {
					deleteCount++
				}
			}
		default:
			goto drainNormal
		}
	}

drainNormal:
	if priorityOnly {
		if drainCount > 0 {
			logx.Infof("[EventProcessor] PriorityWorker %d 清理了 %d 个剩余高优先级事件（其中 DELETE: %d）",
				workerID, drainCount, deleteCount)
		}
		return
	}

	// 再处理普通队列
	normalCount := 0
	for {
		select {
		case event := <-p.eventChan:
			if event != nil {
				p.processEvent(event)
				normalCount++
				if event.IsDelete() {
					deleteCount++
				}
			}
		default:
			drainCount += normalCount
			if drainCount > 0 {
				logx.Infof("[EventProcessor] Worker %d 清理了 %d 个剩余事件（其中 DELETE: %d）",
					workerID, drainCount, deleteCount)
			}
			return
		}
	}
}

// processEvent 处理单个事件
func (p *EventProcessor) processEvent(event *ResourceEvent) {
	// 确保无论如何都会通知事件处理完成
	defer p.notifyEventCompleted(event.ClusterUUID)

	// 检查 context 是否已取消
	select {
	case <-p.ctx.Done():
		// 即使 context 取消，DELETE 事件也要尽量处理
		if !event.IsDelete() {
			return
		}
		logx.Infof("[EventProcessor] Context 已取消，但仍尝试处理 DELETE 事件: %s/%s/%s/%s",
			event.ClusterUUID, event.ResourceType, event.Namespace, event.Name)
	default:
	}

	// 去重检查（DELETE 事件跳过去重，确保一定被处理）
	if !event.IsDelete() && p.isDuplicate(event) {
		atomic.AddInt64(&p.skippedCount, 1)
		logx.Debugf("[EventProcessor] 跳过重复事件: %s/%s/%s/%s/%s",
			event.ClusterUUID, event.ResourceType, event.Type, event.Namespace, event.Name)
		return
	}

	// 尝试获取分布式锁
	lockKey := p.lockKey(event)
	acquired, release, err := p.locker.TryLock(p.ctx, lockKey, p.lockTTL)
	if err != nil {
		// 检查是否是 context 取消导致的错误
		if p.ctx.Err() != nil {
			if event.IsDelete() {
				logx.Errorf("[EventProcessor] Context 取消，DELETE 事件获取锁失败: %s/%s/%s/%s",
					event.ClusterUUID, event.ResourceType, event.Namespace, event.Name)
			}
			return
		}
		logx.Errorf("[EventProcessor] 获取分布式锁失败: %s, error: %v", lockKey, err)
		atomic.AddInt64(&p.errorCount, 1)

		// DELETE 事件获取锁失败也要重试
		if event.IsDelete() {
			p.retryDeleteEvent(event, fmt.Sprintf("锁获取失败: %v", err))
		}
		return
	}

	if !acquired {
		// DELETE 事件不能跳过，必须重试
		if event.IsDelete() {
			p.retryDeleteEvent(event, "锁竞争")
			return
		}

		atomic.AddInt64(&p.skippedCount, 1)
		logx.Debugf("[EventProcessor] 事件正在被其他节点处理，跳过: %s/%s/%s/%s",
			event.ClusterUUID, event.ResourceType, event.Namespace, event.Name)
		return
	}
	defer release()

	// 创建带超时的 context
	processCtx, cancel := context.WithTimeout(p.ctx, p.processTimeout)
	defer cancel()

	// 根据资源类型分发处理
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
		logx.Errorf("[EventProcessor] 未知资源类型: %s", event.ResourceType)
		return
	}

	// 处理结果
	if processErr != nil {
		atomic.AddInt64(&p.errorCount, 1)
		p.updateClusterStats(event.ClusterUUID, false)

		if errors.Is(processCtx.Err(), context.DeadlineExceeded) {
			logx.Errorf("[EventProcessor] 处理事件超时: %s/%s/%s/%s/%s, timeout: %v",
				event.Type, event.ClusterUUID, event.ResourceType, event.Namespace, event.Name, p.processTimeout)
		} else if errors.Is(processCtx.Err(), context.Canceled) {
			if event.IsDelete() {
				logx.Errorf("[EventProcessor] Context 取消，DELETE 事件处理中断: %s/%s/%s/%s",
					event.ClusterUUID, event.ResourceType, event.Namespace, event.Name)
			}
			return
		} else {
			logx.Errorf("[EventProcessor] 处理事件失败: %s/%s/%s/%s/%s, err: %v",
				event.Type, event.ClusterUUID, event.ResourceType, event.Namespace, event.Name, processErr)
		}

		// DELETE 事件处理失败也要重试
		if event.IsDelete() {
			p.retryDeleteEvent(event, fmt.Sprintf("处理失败: %v", processErr))
		}
	} else {
		p.markEventProcessed(event)
		atomic.AddInt64(&p.processedCount, 1)
		p.updateClusterStats(event.ClusterUUID, true)

		// 计算事件延迟（从入队到处理完成）
		latency := time.Since(event.EnqueueTime)
		logx.Infof("[EventProcessor] 事件处理成功: %s/%s/%s/%s/%s (延迟: %v, 重试: %d)",
			event.Type, event.ClusterUUID, event.ResourceType, event.Namespace, event.Name,
			latency.Round(time.Millisecond), event.RetryCount)
	}
}

// notifyEventCompleted 通知事件处理完成
func (p *EventProcessor) notifyEventCompleted(clusterUUID string) {
	if p.onEventCompleted != nil {
		p.onEventCompleted(clusterUUID)
	}
}

// retryDeleteEvent DELETE 事件重试逻辑
func (p *EventProcessor) retryDeleteEvent(event *ResourceEvent, reason string) {
	if event.RetryCount >= MaxDeleteRetry {
		atomic.AddInt64(&p.deleteDropped, 1)
		logx.Errorf("[EventProcessor] DELETE 事件重试次数超限，放弃处理: %s/%s/%s/%s, retries=%d, reason=%s",
			event.ClusterUUID, event.ResourceType, event.Namespace, event.Name, event.RetryCount, reason)
		return
	}

	atomic.AddInt64(&p.retryCount, 1)

	// 计算延迟时间（指数退避，有上限）
	delay := time.Duration(event.RetryCount+1) * DeleteRetryBaseDelay
	if delay > DeleteRetryMaxDelay {
		delay = DeleteRetryMaxDelay
	}

	logx.Infof("[EventProcessor] DELETE 事件将重试(%d/%d): %s/%s/%s/%s, reason=%s, delay=%v",
		event.RetryCount+1, MaxDeleteRetry,
		event.ClusterUUID, event.ResourceType, event.Namespace, event.Name, reason, delay)

	// 使用 retryWg 追踪重试 goroutine，确保优雅关闭时等待它们完成
	p.retryWg.Add(1)
	go func(e *ResourceEvent) {
		defer p.retryWg.Done()

		// 使用 Clone 避免并发修改
		retryEvent := e.Clone()
		retryEvent.RetryCount++
		retryEvent.Timestamp = time.Now()

		select {
		case <-p.ctx.Done():
			// Processor 正在关闭，尝试最后一次入队
			logx.Infof("[EventProcessor] Processor 关闭中，立即重试 DELETE 事件: %s/%s/%s/%s",
				retryEvent.ClusterUUID, retryEvent.ResourceType, retryEvent.Namespace, retryEvent.Name)
			select {
			case p.priorityChan <- retryEvent:
				logx.Infof("[EventProcessor] DELETE 事件在关闭前成功入队: %s/%s/%s/%s",
					retryEvent.ClusterUUID, retryEvent.ResourceType, retryEvent.Namespace, retryEvent.Name)
			default:
				atomic.AddInt64(&p.deleteDropped, 1)
				logx.Errorf("[EventProcessor] Processor 关闭，DELETE 事件重试入队失败: %s/%s/%s/%s",
					retryEvent.ClusterUUID, retryEvent.ResourceType, retryEvent.Namespace, retryEvent.Name)
			}
		case <-time.After(delay):
			p.EnqueueEvent(retryEvent)
		}
	}(event)
}

// eventKey 生成事件的去重键
func (p *EventProcessor) eventKey(event *ResourceEvent) string {
	return fmt.Sprintf("incr:dedupe:%s:%s:%s:%s:%s",
		event.ClusterUUID, event.ResourceType, event.Type, event.Namespace, event.Name)
}

// lockKey 生成处理锁的键
func (p *EventProcessor) lockKey(event *ResourceEvent) string {
	return fmt.Sprintf("%s:%s:%s:%s",
		event.ClusterUUID, event.ResourceType, event.Namespace, event.Name)
}

// isDuplicate 检查事件是否在去重时间窗口内已处理
func (p *EventProcessor) isDuplicate(event *ResourceEvent) bool {
	if p.redis == nil {
		return false
	}

	key := p.eventKey(event)
	exists, err := p.redis.Exists(key)
	if err != nil {
		logx.Errorf("[EventProcessor] Redis EXISTS 失败: %v", err)
		return false
	}
	return exists
}

// markEventProcessed 标记事件已处理
func (p *EventProcessor) markEventProcessed(event *ResourceEvent) {
	if p.redis == nil {
		return
	}

	// DELETE 事件不标记去重（因为 DELETE 本身就跳过去重检查）
	if event.IsDelete() {
		return
	}

	key := p.eventKey(event)
	err := p.redis.Setex(key, "1", int(p.dedupeWindow.Seconds()))
	if err != nil {
		logx.Errorf("[EventProcessor] Redis SETEX 失败: %v", err)
	}
}

// updateClusterStats 更新集群统计
func (p *EventProcessor) updateClusterStats(clusterUUID string, success bool) {
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

// GetStats 获取处理器统计信息
func (p *EventProcessor) GetStats() *ProcessorStats {
	stats := &ProcessorStats{
		ProcessedCount: atomic.LoadInt64(&p.processedCount),
		DroppedCount:   atomic.LoadInt64(&p.droppedCount),
		ErrorCount:     atomic.LoadInt64(&p.errorCount),
		SkippedCount:   atomic.LoadInt64(&p.skippedCount),
		RetryCount:     atomic.LoadInt64(&p.retryCount),
		DeleteDropped:  atomic.LoadInt64(&p.deleteDropped),
		QueueLength:    len(p.eventChan),
		PriorityQueue:  len(p.priorityChan),
		QueueCapacity:  cap(p.eventChan) + cap(p.priorityChan),
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
func (p *EventProcessor) IsHealthy() bool {
	if atomic.LoadInt32(&p.stopped) == 1 {
		return false
	}
	// 检查普通队列和高优先级队列的使用率
	normalUsage := float64(len(p.eventChan)) / float64(cap(p.eventChan))
	priorityUsage := float64(len(p.priorityChan)) / float64(cap(p.priorityChan))

	// 如果高优先级队列使用率过高，需要告警
	if priorityUsage > 0.8 {
		logx.Errorf("[EventProcessor] 高优先级队列使用率过高: %.2f%%", priorityUsage*100)
	}

	return normalUsage < 0.9 && priorityUsage < 0.95
}

// IsStopped 是否已停止
func (p *EventProcessor) IsStopped() bool {
	return atomic.LoadInt32(&p.stopped) == 1
}

// GetQueueStats 获取队列统计
func (p *EventProcessor) GetQueueStats() (normal, priority int) {
	return len(p.eventChan), len(p.priorityChan)
}
