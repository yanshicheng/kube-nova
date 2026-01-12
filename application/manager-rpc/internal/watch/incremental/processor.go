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
type EventProcessor struct {
	redis          *redis.Redis
	locker         DistributedLocker
	handler        EventHandler
	eventChan      chan *ResourceEvent
	workerCount    int
	dedupeWindow   time.Duration
	lockTTL        time.Duration
	processTimeout time.Duration

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// 状态控制
	started  int32     // 原子操作
	stopped  int32     // 原子操作
	stopOnce sync.Once // 确保只停止一次

	// 统计信息
	processedCount int64
	droppedCount   int64
	errorCount     int64
	skippedCount   int64

	// 按集群统计
	clusterStatsMu sync.RWMutex
	clusterStats   map[string]*ClusterEventStats
}

// EventProcessorConfig 事件处理器配置
type EventProcessorConfig struct {
	Redis           *redis.Redis
	Handler         EventHandler
	NodeID          string
	WorkerCount     int
	EventBuffer     int
	DedupeWindow    time.Duration
	LockTTL         time.Duration
	ProcessTimeout  time.Duration
	EnableAutoRenew bool
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
		redis:          cfg.Redis,
		locker:         locker,
		handler:        cfg.Handler,
		eventChan:      make(chan *ResourceEvent, cfg.EventBuffer),
		workerCount:    cfg.WorkerCount,
		dedupeWindow:   cfg.DedupeWindow,
		lockTTL:        cfg.LockTTL,
		processTimeout: cfg.ProcessTimeout,
		clusterStats:   make(map[string]*ClusterEventStats),
	}
}

// Start 启动事件处理器
func (p *EventProcessor) Start(ctx context.Context) {
	if !atomic.CompareAndSwapInt32(&p.started, 0, 1) {
		logx.Error("[EventProcessor] 已经启动，跳过重复启动")
		return
	}

	p.ctx, p.cancel = context.WithCancel(ctx)

	for i := 0; i < p.workerCount; i++ {
		p.wg.Add(1)
		go p.worker(i)
	}

	logx.Infof("[EventProcessor] 启动完成，工作协程数: %d, 缓冲区大小: %d, 锁TTL: %v, 处理超时: %v",
		p.workerCount, cap(p.eventChan), p.lockTTL, p.processTimeout)
}

// Stop 停止事件处理器（幂等，可多次调用）
// 关键设计：不关闭 channel，避免竞态条件导致的 panic
func (p *EventProcessor) Stop() {
	p.stopOnce.Do(func() {
		// 1. 设置停止标志（让 EnqueueEvent 快速返回）
		atomic.StoreInt32(&p.stopped, 1)

		// 2. 取消 context（通知 worker 退出）
		if p.cancel != nil {
			p.cancel()
		}

		// 3. 等待所有 worker 完成
		p.wg.Wait()

		// 4. 输出统计日志
		logx.Infof("[EventProcessor] 已停止, 处理: %d, 跳过: %d, 丢弃: %d, 错误: %d",
			atomic.LoadInt64(&p.processedCount),
			atomic.LoadInt64(&p.skippedCount),
			atomic.LoadInt64(&p.droppedCount),
			atomic.LoadInt64(&p.errorCount))
	})
}

// EnqueueEvent 将事件放入队列
func (p *EventProcessor) EnqueueEvent(event *ResourceEvent) {
	if atomic.LoadInt32(&p.stopped) == 1 {
		return
	}

	// 非阻塞发送
	select {
	case p.eventChan <- event:
		// 成功入队
	default:
		// 队列满了
		atomic.AddInt64(&p.droppedCount, 1)
		logx.Errorf("[EventProcessor] 事件队列已满，丢弃事件: %s/%s/%s/%s",
			event.ClusterUUID, event.ResourceType, event.Namespace, event.Name)
	}
}

// worker 工作协程
func (p *EventProcessor) worker(id int) {
	defer p.wg.Done()

	for {
		select {
		case <-p.ctx.Done():
			// 收到停止信号，先处理完队列中剩余的事件
			p.drainEvents(id)
			logx.Debugf("[EventProcessor] Worker %d 收到停止信号，已处理完剩余事件，退出", id)
			return
		case event := <-p.eventChan:
			if event != nil {
				p.processEvent(event)
			}
		}
	}
}

// drainEvents 处理队列中剩余的事件
func (p *EventProcessor) drainEvents(workerID int) {
	drainCount := 0
	for {
		select {
		case event := <-p.eventChan:
			if event != nil {
				p.processEvent(event)
				drainCount++
			}
		default:
			if drainCount > 0 {
				logx.Debugf("[EventProcessor] Worker %d 处理了 %d 个剩余事件", workerID, drainCount)
			}
			return
		}
	}
}

// processEvent 处理单个事件
func (p *EventProcessor) processEvent(event *ResourceEvent) {
	// 检查 context 是否已取消
	select {
	case <-p.ctx.Done():
		return
	default:
	}

	// 去重检查
	if p.isDuplicate(event) {
		atomic.AddInt64(&p.skippedCount, 1)
		logx.Debugf("[EventProcessor] 跳过重复事件: %s/%s/%s/%s/%s",
			event.ClusterUUID, event.ResourceType, event.Type, event.Namespace, event.Name)
		return
	}

	// 尝试获取分布式锁
	lockKey := p.lockKey(event)
	acquired, release, err := p.locker.TryLock(p.ctx, lockKey, p.lockTTL)
	if err != nil {
		if p.ctx.Err() != nil {
			return // context 已取消
		}
		logx.Errorf("[EventProcessor] 获取分布式锁失败: %s, error: %v", lockKey, err)
		atomic.AddInt64(&p.errorCount, 1)
		return
	}

	if !acquired {
		atomic.AddInt64(&p.skippedCount, 1)
		logx.Debugf("[EventProcessor] 事件正在被其他节点处理: %s/%s/%s/%s",
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
			logx.Errorf("[EventProcessor] 处理事件超时: %s/%s/%s/%s, timeout: %v",
				event.ClusterUUID, event.ResourceType, event.Namespace, event.Name, p.processTimeout)
		} else if errors.Is(processCtx.Err(), context.Canceled) {
			return
		} else {
			logx.Errorf("[EventProcessor] 处理事件失败: %s/%s/%s/%s, err: %v",
				event.ClusterUUID, event.ResourceType, event.Namespace, event.Name, processErr)
		}
	} else {
		p.markEventProcessed(event)
		atomic.AddInt64(&p.processedCount, 1)
		p.updateClusterStats(event.ClusterUUID, true)
	}
}

// eventKey 生成事件的唯一键
func (p *EventProcessor) eventKey(event *ResourceEvent) string {
	return fmt.Sprintf("incr:event:%s:%s:%s:%s:%s:%s",
		event.ClusterUUID, event.ResourceType, event.Type, event.Namespace, event.Name,
		event.Timestamp.Format("20060102150405"))
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
		QueueLength:    len(p.eventChan),
		QueueCapacity:  cap(p.eventChan),
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
	queueLen := len(p.eventChan)
	queueCap := cap(p.eventChan)
	return float64(queueLen)/float64(queueCap) < 0.9
}

// IsStopped 是否已停止
func (p *EventProcessor) IsStopped() bool {
	return atomic.LoadInt32(&p.stopped) == 1
}
