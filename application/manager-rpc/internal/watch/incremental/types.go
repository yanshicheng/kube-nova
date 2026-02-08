package incremental

import (
	"context"
	"errors"
	"time"
)

// EventType 事件类型
type EventType string

const (
	EventAdd    EventType = "ADD"
	EventUpdate EventType = "UPDATE"
	EventDelete EventType = "DELETE"
)

// EventPriority 事件优先级
type EventPriority int

const (
	PriorityLow    EventPriority = 0
	PriorityNormal EventPriority = 1
	PriorityHigh   EventPriority = 2 // DELETE 事件使用高优先级
)

// DELETE 事件相关常量
const (
	// MaxDeleteRetry DELETE 事件最大重试次数
	MaxDeleteRetry = 10
	// DeleteRetryBaseDelay DELETE 事件重试基础延迟
	DeleteRetryBaseDelay = 500 * time.Millisecond
	// DeleteRetryMaxDelay DELETE 事件重试最大延迟
	DeleteRetryMaxDelay = 5 * time.Second
	// DeleteEnqueueTimeout DELETE 事件入队超时时间
	DeleteEnqueueTimeout = 10 * time.Second
)

// 预定义错误
var (
	// ErrManagerNotRunning 管理器未运行
	ErrManagerNotRunning = errors.New("incremental sync manager is not running")
	// ErrManagerAlreadyRunning 管理器已在运行
	ErrManagerAlreadyRunning = errors.New("incremental sync manager is already running")
	// ErrHandlerNotSet 事件处理器未设置
	ErrHandlerNotSet = errors.New("event handler is not set")
	// ErrK8sManagerNotSet K8s管理器未设置
	ErrK8sManagerNotSet = errors.New("k8s manager is not set")
	// ErrClusterModelNotSet 集群模型未设置
	ErrClusterModelNotSet = errors.New("cluster model is not set")
	// ErrWatcherStopTimeout 监听器停止超时
	ErrWatcherStopTimeout = errors.New("watcher stop timeout")
	// ErrProcessorNotRunning 处理器未运行
	ErrProcessorNotRunning = errors.New("event processor is not running")
)

// ResourceEvent K8s 资源事件
type ResourceEvent struct {
	Type         EventType     // 事件类型
	ClusterUUID  string        // 集群 UUID
	ResourceType string        // 资源类型: namespace, deployment, statefulset, daemonset, cronjob, resourcequota, limitrange
	Namespace    string        // 命名空间
	Name         string        // 资源名称
	OldObject    interface{}   // 旧对象（UPDATE/DELETE 时有效）
	NewObject    interface{}   // 新对象（ADD/UPDATE 时有效）
	Timestamp    time.Time     // 事件时间戳
	RetryCount   int           // 重试次数（用于 DELETE 事件重试）
	Priority     EventPriority // 事件优先级
	EnqueueTime  time.Time     // 入队时间（用于监控事件延迟）
}

// GetPriority 获取事件优先级
func (e *ResourceEvent) GetPriority() EventPriority {
	if e.Priority != 0 {
		return e.Priority
	}
	// DELETE 事件默认高优先级
	if e.Type == EventDelete {
		return PriorityHigh
	}
	return PriorityNormal
}

// UniqueKey 返回事件的唯一标识（用于去重和锁）
func (e *ResourceEvent) UniqueKey() string {
	return e.ClusterUUID + ":" + e.ResourceType + ":" + e.Namespace + ":" + e.Name
}

// IsDelete 判断是否为 DELETE 事件
func (e *ResourceEvent) IsDelete() bool {
	return e.Type == EventDelete
}

// CanRetry 判断 DELETE 事件是否可以重试
func (e *ResourceEvent) CanRetry() bool {
	return e.Type == EventDelete && e.RetryCount < MaxDeleteRetry
}

// Clone 复制事件（用于重试时避免并发修改）
func (e *ResourceEvent) Clone() *ResourceEvent {
	return &ResourceEvent{
		Type:         e.Type,
		ClusterUUID:  e.ClusterUUID,
		ResourceType: e.ResourceType,
		Namespace:    e.Namespace,
		Name:         e.Name,
		OldObject:    e.OldObject,
		NewObject:    e.NewObject,
		Timestamp:    e.Timestamp,
		RetryCount:   e.RetryCount,
		Priority:     e.Priority,
		EnqueueTime:  e.EnqueueTime,
	}
}

// EventHandler 事件处理器接口
type EventHandler interface {
	HandleNamespaceEvent(ctx context.Context, event *ResourceEvent) error
	HandleDeploymentEvent(ctx context.Context, event *ResourceEvent) error
	HandleStatefulSetEvent(ctx context.Context, event *ResourceEvent) error
	HandleDaemonSetEvent(ctx context.Context, event *ResourceEvent) error
	HandleCronJobEvent(ctx context.Context, event *ResourceEvent) error
	HandleResourceQuotaEvent(ctx context.Context, event *ResourceEvent) error
	HandleLimitRangeEvent(ctx context.Context, event *ResourceEvent) error
}

// DistributedLocker 分布式锁接口
type DistributedLocker interface {
	TryLock(ctx context.Context, key string, ttl time.Duration) (acquired bool, release func(), err error)
	IsLocked(ctx context.Context, key string) (bool, error)
	NodeID() string
}

// EventEnqueuer 事件入队接口
// 用于将事件发送到处理器，支持不同的处理器实现
type EventEnqueuer interface {
	EnqueueEvent(event *ResourceEvent) bool
}

// ProcessorStats 处理器统计信息
type ProcessorStats struct {
	ProcessedCount int64                         `json:"processed_count"`
	DroppedCount   int64                         `json:"dropped_count"`
	ErrorCount     int64                         `json:"error_count"`
	SkippedCount   int64                         `json:"skipped_count"`
	RetryCount     int64                         `json:"retry_count"`
	DeleteDropped  int64                         `json:"delete_dropped"`
	QueueLength    int                           `json:"queue_length"`
	QueueCapacity  int                           `json:"queue_capacity"`
	PriorityQueue  int                           `json:"priority_queue_length"`
	WorkerCount    int                           `json:"worker_count"`
	ClusterStats   map[string]*ClusterEventStats `json:"cluster_stats,omitempty"`
}

// ClusterEventStats 单个集群的事件统计
type ClusterEventStats struct {
	ClusterUUID    string `json:"cluster_uuid"`
	ProcessedCount int64  `json:"processed_count"`
	ErrorCount     int64  `json:"error_count"`
}

// WatcherStats 监听器统计信息
type WatcherStats struct {
	ClusterUUID    string    `json:"cluster_uuid"`
	IsRunning      bool      `json:"is_running"`
	StartTime      time.Time `json:"start_time,omitempty"`
	EventsReceived int64     `json:"events_received"`
	PendingEvents  int64     `json:"pending_events"`
}

// ManagerStats 管理器统计信息
type ManagerStats struct {
	NodeID         string                   `json:"node_id"`
	IsRunning      bool                     `json:"is_running"`
	Uptime         string                   `json:"uptime,omitempty"`
	WatcherCount   int                      `json:"watcher_count"`
	WorkerCount    int                      `json:"worker_count"`
	EventBuffer    int                      `json:"event_buffer"`
	DedupeWindow   string                   `json:"dedupe_window"`
	Clusters       []string                 `json:"clusters"`
	ProcessorStats *ProcessorStats          `json:"processor_stats,omitempty"`
	WatcherStats   map[string]*WatcherStats `json:"watcher_stats,omitempty"`
}

// SyncResult 批量同步结果
type SyncResult struct {
	Added        []string         // 成功添加的集群 UUID 列表
	Removed      []string         // 成功移除的集群 UUID 列表
	AddErrors    map[string]error // 添加失败的集群及错误信息
	RemoveErrors map[string]error // 移除失败的集群及错误信息
}

// HasErrors 检查同步结果是否包含错误
func (r *SyncResult) HasErrors() bool {
	return len(r.AddErrors) > 0 || len(r.RemoveErrors) > 0
}

// TotalErrors 返回错误总数
func (r *SyncResult) TotalErrors() int {
	return len(r.AddErrors) + len(r.RemoveErrors)
}
