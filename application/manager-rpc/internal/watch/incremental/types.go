package incremental

import (
	"context"
	"time"
)

// EventType 事件类型
type EventType string

const (
	EventAdd    EventType = "ADD"
	EventUpdate EventType = "UPDATE"
	EventDelete EventType = "DELETE"
)

// ResourceEvent K8s 资源事件
type ResourceEvent struct {
	Type         EventType   // 事件类型
	ClusterUUID  string      // 集群 UUID
	ResourceType string      // 资源类型: namespace, deployment, statefulset, daemonset, cronjob, resourcequota, limitrange
	Namespace    string      // 命名空间
	Name         string      // 资源名称
	OldObject    interface{} // 旧对象（UPDATE/DELETE 时有效）
	NewObject    interface{} // 新对象（ADD/UPDATE 时有效）
	Timestamp    time.Time   // 事件时间戳
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

// ProcessorStats 处理器统计信息
type ProcessorStats struct {
	ProcessedCount int64                         `json:"processed_count"`
	DroppedCount   int64                         `json:"dropped_count"`
	ErrorCount     int64                         `json:"error_count"`
	SkippedCount   int64                         `json:"skipped_count"`
	QueueLength    int                           `json:"queue_length"`
	QueueCapacity  int                           `json:"queue_capacity"`
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
