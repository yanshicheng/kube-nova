package notification

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// 告警通知系统 Prometheus 指标
var (
	// 告警发送总数
	alertsSentTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "kube_nova_alerts_sent_total",
			Help: "Total number of alerts sent by channel type and status",
		},
		[]string{"channel_type", "status", "project_name"},
	)

	// 告警发送延迟
	alertSendDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "kube_nova_alert_send_duration_seconds",
			Help:    "Time taken to send alerts",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"channel_type", "project_name"},
	)

	// 聚合器状态指标
	aggregatorEnabled = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "kube_nova_aggregator_enabled",
			Help: "Whether the alert aggregator is enabled (1=enabled, 0=disabled)",
		},
		[]string{"instance_id"},
	)

	// Leader 选举状态
	aggregatorIsLeader = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "kube_nova_aggregator_is_leader",
			Help: "Whether this instance is the aggregator leader (1=leader, 0=follower)",
		},
		[]string{"instance_id", "leader_type"},
	)

	// 缓冲的告警数量
	alertsBuffered = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "kube_nova_alerts_buffered_total",
			Help: "Total number of alerts buffered for aggregation",
		},
		[]string{"project_name", "severity"},
	)

	// 立即发送的告警数量
	alertsImmediateSent = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "kube_nova_alerts_immediate_sent_total",
			Help: "Total number of alerts sent immediately (Critical level)",
		},
		[]string{"project_name"},
	)

	// Leader 选举次数
	leaderElections = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "kube_nova_leader_elections_total",
			Help: "Total number of leader elections",
		},
		[]string{"instance_id", "election_type"},
	)

	// Leader 续约次数
	leaderRenewals = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "kube_nova_leader_renewals_total",
			Help: "Total number of leader renewals",
		},
		[]string{"instance_id", "election_type"},
	)

	// Leader 选举失败次数
	leaderElectionFailures = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "kube_nova_leader_election_failures_total",
			Help: "Total number of leader election failures",
		},
		[]string{"instance_id", "election_type", "error_type"},
	)

	// 邮件发送状态
	emailSendTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "kube_nova_email_send_total",
			Help: "Total number of emails sent by status",
		},
		[]string{"smtp_host", "status", "project_name"},
	)

	// 邮件发送延迟
	emailSendDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "kube_nova_email_send_duration_seconds",
			Help:    "Time taken to send emails",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"smtp_host", "project_name"},
	)

	// 通道健康状态
	channelHealthStatus = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "kube_nova_channel_health_status",
			Help: "Health status of notification channels (1=healthy, 0=unhealthy)",
		},
		[]string{"channel_type", "channel_uuid", "channel_name"},
	)

	// 通道最后检查时间
	channelLastCheckTime = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "kube_nova_channel_last_check_timestamp",
			Help: "Timestamp of last health check for notification channels",
		},
		[]string{"channel_type", "channel_uuid", "channel_name"},
	)

	// Redis 缓冲区大小
	redisBufferSize = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "kube_nova_redis_buffer_size",
			Help: "Number of alerts currently in Redis buffer",
		},
		[]string{"project_name"},
	)

	// 聚合窗口过期的告警数量
	aggregatedAlertsExpired = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "kube_nova_aggregated_alerts_expired_total",
			Help: "Total number of aggregated alerts that expired and were sent",
		},
		[]string{"project_name", "alert_count_range"},
	)
)

// MetricsCollector 指标收集器
type MetricsCollector struct {
	mu sync.RWMutex
	// 实例级别的统计
	instanceStats map[string]*InstanceStats
}

// InstanceStats 实例统计信息
type InstanceStats struct {
	InstanceID      string
	StartTime       time.Time
	LastHealthCheck time.Time
	IsLeader        bool
	LeaderType      string // "k8s", "redis", "standalone"
	AlertsSent      int64
	AlertsFailed    int64
	EmailsSent      int64
	EmailsFailed    int64
	LeaderElections int64
	LeaderRenewals  int64
	LeaderFailures  int64
}

// NewMetricsCollector 创建指标收集器
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		instanceStats: make(map[string]*InstanceStats),
	}
}

// RecordAlertSent 记录告警发送
func (mc *MetricsCollector) RecordAlertSent(channelType, status, projectName string, duration time.Duration) {
	alertsSentTotal.WithLabelValues(channelType, status, projectName).Inc()
	if duration > 0 {
		alertSendDuration.WithLabelValues(channelType, projectName).Observe(duration.Seconds())
	}
}

// RecordEmailSent 记录邮件发送
func (mc *MetricsCollector) RecordEmailSent(smtpHost, status, projectName string, duration time.Duration) {
	emailSendTotal.WithLabelValues(smtpHost, status, projectName).Inc()
	if duration > 0 {
		emailSendDuration.WithLabelValues(smtpHost, projectName).Observe(duration.Seconds())
	}
}

// RecordAggregatorStatus 记录聚合器状态
func (mc *MetricsCollector) RecordAggregatorStatus(instanceID string, enabled bool, isLeader bool, leaderType string) {
	if enabled {
		aggregatorEnabled.WithLabelValues(instanceID).Set(1)
	} else {
		aggregatorEnabled.WithLabelValues(instanceID).Set(0)
	}

	if isLeader {
		aggregatorIsLeader.WithLabelValues(instanceID, leaderType).Set(1)
	} else {
		aggregatorIsLeader.WithLabelValues(instanceID, leaderType).Set(0)
	}
}

// RecordAlertBuffered 记录缓冲的告警
func (mc *MetricsCollector) RecordAlertBuffered(projectName, severity string, count int) {
	alertsBuffered.WithLabelValues(projectName, severity).Add(float64(count))
}

// RecordAlertImmediateSent 记录立即发送的告警
func (mc *MetricsCollector) RecordAlertImmediateSent(projectName string, count int) {
	alertsImmediateSent.WithLabelValues(projectName).Add(float64(count))
}

// RecordLeaderElection 记录 Leader 选举
func (mc *MetricsCollector) RecordLeaderElection(instanceID, electionType string) {
	leaderElections.WithLabelValues(instanceID, electionType).Inc()
}

// RecordLeaderRenewal 记录 Leader 续约
func (mc *MetricsCollector) RecordLeaderRenewal(instanceID, electionType string) {
	leaderRenewals.WithLabelValues(instanceID, electionType).Inc()
}

// RecordLeaderElectionFailure 记录 Leader 选举失败
func (mc *MetricsCollector) RecordLeaderElectionFailure(instanceID, electionType, errorType string) {
	leaderElectionFailures.WithLabelValues(instanceID, electionType, errorType).Inc()
}

// RecordChannelHealth 记录通道健康状态
func (mc *MetricsCollector) RecordChannelHealth(channelType, channelUUID, channelName string, healthy bool) {
	if healthy {
		channelHealthStatus.WithLabelValues(channelType, channelUUID, channelName).Set(1)
	} else {
		channelHealthStatus.WithLabelValues(channelType, channelUUID, channelName).Set(0)
	}
	channelLastCheckTime.WithLabelValues(channelType, channelUUID, channelName).Set(float64(time.Now().Unix()))
}

// RecordRedisBufferSize 记录 Redis 缓冲区大小
func (mc *MetricsCollector) RecordRedisBufferSize(projectName string, size int) {
	redisBufferSize.WithLabelValues(projectName).Set(float64(size))
}

// RecordAggregatedAlertsExpired 记录聚合告警过期
func (mc *MetricsCollector) RecordAggregatedAlertsExpired(projectName string, alertCount int) {
	var countRange string
	switch {
	case alertCount == 1:
		countRange = "1"
	case alertCount <= 5:
		countRange = "2-5"
	case alertCount <= 10:
		countRange = "6-10"
	case alertCount <= 50:
		countRange = "11-50"
	default:
		countRange = "50+"
	}
	aggregatedAlertsExpired.WithLabelValues(projectName, countRange).Inc()
}

// UpdateInstanceStats 更新实例统计信息
func (mc *MetricsCollector) UpdateInstanceStats(instanceID string, stats *InstanceStats) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.instanceStats[instanceID] = stats
}

// GetInstanceStats 获取实例统计信息
func (mc *MetricsCollector) GetInstanceStats(instanceID string) *InstanceStats {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	return mc.instanceStats[instanceID]
}

// GetAllInstanceStats 获取所有实例统计信息
func (mc *MetricsCollector) GetAllInstanceStats() map[string]*InstanceStats {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	result := make(map[string]*InstanceStats)
	for k, v := range mc.instanceStats {
		result[k] = v
	}
	return result
}

// 全局指标收集器实例
var GlobalMetricsCollector = NewMetricsCollector()
