package notification

import (
	"context"
	"time"
)

// AlertType 告警类型枚举
// 定义系统支持的所有告警渠道类型
type AlertType string

const (
	// AlertTypeDingTalk 钉钉告警渠道
	AlertTypeDingTalk AlertType = "dingtalk"
	// AlertTypeWeChat 企业微信告警渠道
	AlertTypeWeChat AlertType = "wechat"
	// AlertTypeFeiShu 飞书告警渠道
	AlertTypeFeiShu AlertType = "feishu"
	// AlertTypeEmail 邮件告警渠道
	AlertTypeEmail AlertType = "email"
	// AlertTypeSMS 短信告警渠道（预留）
	AlertTypeSMS AlertType = "sms"
	// AlertTypeVoiceCall 语音告警渠道（预留）
	AlertTypeVoiceCall AlertType = "voice_call"
	// AlertTypeWebhook 自定义 Webhook 告警渠道
	AlertTypeWebhook AlertType = "webhook"
	// AlertTypeSiteMessage 站内信渠道
	AlertTypeSiteMessage AlertType = "site_message"
)

// AlertLevel 告警级别枚举
// 用于区分告警的严重程度和通知类型
type AlertLevel string

const (
	// AlertLevelDefault 默认级别，当未找到特定级别配置时使用
	AlertLevelDefault AlertLevel = "default"
	// AlertLevelInfo 信息级别，一般性通知
	AlertLevelInfo AlertLevel = "info"
	// AlertLevelWarning 警告级别，需要关注但不紧急
	AlertLevelWarning AlertLevel = "warning"
	// AlertLevelCritical 严重级别，需要立即处理
	AlertLevelCritical AlertLevel = "critical"
	// AlertLevelNotification 通知级别，用于系统通知等非告警消息
	AlertLevelNotification AlertLevel = "notification"
)

// AlertStatus 告警状态枚举
// 表示告警的当前状态
type AlertStatus string

const (
	// AlertStatusFiring 告警触发中，问题仍然存在
	AlertStatusFiring AlertStatus = "firing"
	// AlertStatusResolved 告警已恢复，问题已解决
	AlertStatusResolved AlertStatus = "resolved"
)

// String 返回告警类型的字符串表示
func (t AlertType) String() string {
	return string(t)
}

// String 返回告警级别的字符串表示
func (l AlertLevel) String() string {
	return string(l)
}

// AlertInstance Prometheus 告警实例
// 包含单个告警的所有详细信息
type AlertInstance struct {
	// ID 告警实例唯一标识，数据库主键
	ID uint64 `json:"id"`
	// Instance 告警来源实例，通常是 IP:Port 格式
	Instance string `json:"instance"`
	// Fingerprint 告警指纹，用于去重和关联
	Fingerprint string `json:"fingerprint"`
	// ClusterUUID 所属集群的 UUID
	ClusterUUID string `json:"clusterUuid"`
	// ClusterName 所属集群名称
	ClusterName string `json:"clusterName"`
	// ProjectID 所属项目 ID，0 表示集群级告警
	ProjectID uint64 `json:"projectId"`
	// ProjectName 所属项目名称
	ProjectName string `json:"projectName"`
	// WorkspaceID 所属工作空间 ID
	WorkspaceID uint64 `json:"workspaceId"`
	// WorkspaceName 所属工作空间名称
	WorkspaceName string `json:"workspaceName"`
	// AlertName 告警规则名称
	AlertName string `json:"alertName"`
	// Severity 告警级别: info、warning、critical
	Severity string `json:"severity"`
	// Status 告警状态: firing、resolved
	Status string `json:"status"`
	// Labels 告警标签，包含 Prometheus 告警的所有标签
	Labels map[string]string `json:"labels"`
	// Annotations 告警注解，包含 summary、description 等
	Annotations map[string]string `json:"annotations"`
	// GeneratorURL 告警生成器 URL，通常指向 Prometheus 或 Alertmanager
	GeneratorURL string `json:"generatorUrl"`
	// StartsAt 告警开始时间
	StartsAt time.Time `json:"startsAt"`
	// EndsAt 告警结束时间（如果已恢复）
	EndsAt *time.Time `json:"endsAt"`
	// ResolvedAt 告警恢复时间
	ResolvedAt *time.Time `json:"resolvedAt"`
	// Duration 告警持续时间（秒）
	Duration uint `json:"duration"`
	// RepeatCount 告警重复次数
	RepeatCount uint `json:"repeatCount"`
	// LastNotifiedAt 最后通知时间
	LastNotifiedAt *time.Time `json:"lastNotifiedAt"`
}

// MentionConfig 提及/通知人配置
// 用于在发送消息时指定要通知的人员
type MentionConfig struct {
	// InternalUserIds 内部系统用户ID列表（用于站内信）
	// 注意: 这是系统内部的用户ID (uint64类型)，不是第三方平台的用户ID
	InternalUserIds []uint64 `json:"internalUserIds,omitempty"`

	// 第三方平台用户ID（用于各平台的 @ 功能）
	// DingtalkUserIds 钉钉用户ID列表，必须是钉钉平台的真实用户ID
	DingtalkUserIds []string `json:"dingtalkUserIds,omitempty"`
	// WechatUserIds 企业微信用户ID列表，必须是企业微信平台的真实用户ID
	WechatUserIds []string `json:"wechatUserIds,omitempty"`
	// FeishuUserIds 飞书用户ID列表，必须是飞书平台的真实用户ID (open_id 或 user_id)
	FeishuUserIds []string `json:"feishuUserIds,omitempty"`

	// IsAtAll 是否通知所有人
	IsAtAll bool `json:"isAtAll,omitempty"`

	// 邮件相关
	// EmailTo 邮件收件人列表
	EmailTo []string `json:"emailTo,omitempty"`
	// EmailCC 邮件抄送列表
	EmailCC []string `json:"emailCC,omitempty"`
	// EmailBCC 邮件密送列表
	EmailBCC []string `json:"emailBCC,omitempty"`

	// 已废弃字段（保留用于向后兼容）
	// AtMobiles 手机号列表（已废弃，不推荐使用）
	AtMobiles []string `json:"atMobiles,omitempty"`
	// AtUserIds 旧版用户ID字段（已废弃，请使用 InternalUserIds）
	AtUserIds []string `json:"atUserIds,omitempty"`
}

// AlertOptions Prometheus 告警发送选项
// 定义发送 Prometheus 告警时的参数
type AlertOptions struct {
	// PortalName 平台名称，显示在告警消息中
	PortalName string `json:"portalName"`
	// PortalUrl 平台 URL，用于告警详情链接
	PortalUrl string `json:"portalUrl"`
	// Mentions 提及人配置
	Mentions *MentionConfig `json:"mentions,omitempty"`
	// ProjectName 项目名称
	ProjectName string `json:"projectName"`
	// ClusterName 集群名称
	ClusterName string `json:"clusterName"`
	// Severity 告警级别
	Severity string `json:"severity"`
}

// NotificationOptions 通用通知选项
// 用于发送系统通知等非 Prometheus 告警消息
type NotificationOptions struct {
	// PortalName 平台名称
	PortalName string `json:"portalName"`
	// PortalUrl 平台 URL
	PortalUrl string `json:"portalUrl"`
	// Title 通知标题
	Title string `json:"title"`
	// Content 通知内容
	Content string `json:"content"`
	// Mentions 提及人配置
	Mentions *MentionConfig `json:"mentions,omitempty"`
}

// SendResult 发送结果
// 记录单次消息发送的结果
type SendResult struct {
	// Success 是否发送成功
	Success bool `json:"success"`
	// Message 结果消息，成功时为确认信息，失败时为错误描述
	Message string `json:"message"`
	// MessageID 消息 ID（部分渠道会返回）
	MessageID string `json:"messageId,omitempty"`
	// Timestamp 发送时间戳
	Timestamp time.Time `json:"timestamp"`
	// Error 错误信息（如果失败）
	Error error `json:"error,omitempty"`
	// CostMs 发送耗时（毫秒）
	CostMs int64 `json:"costMs,omitempty"`
}

// TestResult 连接测试结果
// 用于测试告警渠道配置是否正确
type TestResult struct {
	// Success 测试是否成功
	Success bool `json:"success"`
	// Message 测试结果消息
	Message string `json:"message"`
	// Latency 测试延迟
	Latency time.Duration `json:"latency"`
	// Timestamp 测试时间戳
	Timestamp time.Time `json:"timestamp"`
	// Error 错误信息（如果失败）
	Error error `json:"error,omitempty"`
	// Details 详细信息，包含渠道配置等
	Details map[string]interface{} `json:"details,omitempty"`
}

// HealthStatus 健康状态
// 用于监控告警渠道的健康状况
type HealthStatus struct {
	// Healthy 是否健康
	Healthy bool `json:"healthy"`
	// Message 状态消息
	Message string `json:"message"`
	// LastCheckTime 最后检查时间
	LastCheckTime time.Time `json:"lastCheckTime"`
	// Error 错误信息（如果不健康）
	Error error `json:"error,omitempty"`
}

// Statistics 统计信息
// 记录告警渠道的发送统计数据
type Statistics struct {
	// TotalSent 总发送次数
	TotalSent int64 `json:"totalSent"`
	// TotalSuccess 成功次数
	TotalSuccess int64 `json:"totalSuccess"`
	// TotalFailed 失败次数
	TotalFailed int64 `json:"totalFailed"`
	// LastSentTime 最后发送时间
	LastSentTime time.Time `json:"lastSentTime"`
	// LastSuccessTime 最后成功时间
	LastSuccessTime time.Time `json:"lastSuccessTime"`
	// LastFailedTime 最后失败时间
	LastFailedTime time.Time `json:"lastFailedTime"`
	// TotalLatency 累计延迟，用于计算平均延迟
	TotalLatency time.Duration `json:"totalLatency"`
	// AverageLatency 平均延迟
	AverageLatency time.Duration `json:"averageLatency"`
}

// Client 告警客户端基础接口
// 定义所有告警渠道必须实现的基本方法
type Client interface {
	// GetUUID 获取渠道唯一标识
	GetUUID() string
	// GetName 获取渠道名称
	GetName() string
	// GetType 获取渠道类型
	GetType() AlertType
	// GetConfig 获取渠道配置
	GetConfig() Config

	// HealthCheck 执行健康检查
	HealthCheck(ctx context.Context) (*HealthStatus, error)
	// IsHealthy 快速检查是否健康
	IsHealthy() bool

	// TestConnection 测试连接，toEmail 参数用于邮件渠道指定测试收件人
	TestConnection(ctx context.Context, toEmail []string) (*TestResult, error)

	// GetStatistics 获取统计信息
	GetStatistics() *Statistics
	// ResetStatistics 重置统计信息
	ResetStatistics()

	// Close 关闭客户端，释放资源
	Close() error

	// WithContext 设置上下文，返回带有新上下文的客户端
	WithContext(ctx context.Context) Client
}

// PrometheusAlertChannel Prometheus 告警渠道接口
// 支持发送 Prometheus 格式告警的渠道需要实现此接口
type PrometheusAlertChannel interface {
	Client
	// SendPrometheusAlert 发送 Prometheus 告警
	// opts 包含告警的通用选项，alerts 是具体的告警实例列表
	SendPrometheusAlert(ctx context.Context, opts *AlertOptions, alerts []*AlertInstance) (*SendResult, error)
}

// NotificationChannel 通用通知渠道接口
// 支持发送通用通知消息的渠道需要实现此接口
type NotificationChannel interface {
	Client
	// SendNotification 发送通用通知
	SendNotification(ctx context.Context, opts *NotificationOptions) (*SendResult, error)
}

// FullChannel 完整渠道接口
// 同时支持 Prometheus 告警和通用通知的渠道需要实现此接口
type FullChannel interface {
	PrometheusAlertChannel
	NotificationChannel
}

// Manager 告警管理器接口
// 定义告警系统的核心管理功能
type Manager interface {
	// SetPortalInfo 设置平台信息
	SetPortalInfo(portalName, portalUrl string)
	// GetPortalName 获取平台名称
	GetPortalName() string
	// GetPortalUrl 获取平台 URL
	GetPortalUrl() string

	// AddChannel 添加告警渠道
	AddChannel(config Config) error
	// RemoveChannel 移除告警渠道
	RemoveChannel(uuid string) error
	// UpdateChannel 更新告警渠道配置
	UpdateChannel(config Config) error
	// GetChannel 获取指定渠道（用于测试）
	GetChannel(ctx context.Context, uuid string) (Client, error)
	// ListChannels 列出所有渠道
	ListChannels() []Client
	// GetChannelCount 获取渠道数量
	GetChannelCount() int
	// GetChannelsByType 根据类型获取渠道列表
	GetChannelsByType(alertType AlertType) []Client

	// PrometheusAlertNotification 发送 Prometheus 告警通知
	// 核心方法，内部自动完成以下步骤:
	// 1. 按 ProjectID + Severity 分组
	// 2. 查询项目绑定的告警组（ProjectID=0 使用默认告警组 ID=2）
	// 3. 获取对应级别的通知渠道（没有则用 default）
	// 4. 查询告警组成员获取通知人信息
	// 5. 并发发送到所有渠道
	// 6. 为每条告警创建站内信
	// 7. 记录通知日志
	PrometheusAlertNotification(ctx context.Context, alerts []*AlertInstance) error

	// SendAlertsDirectly 直接发送告警（绕过聚合器）
	// 用于聚合器内部调用，直接发送告警通知
	SendAlertsDirectly(ctx context.Context, alerts []*AlertInstance) error

	// DefaultNotification 发送通用通知（如创建账号等系统通知）
	// 查询默认告警组的 notification 级别渠道（没有则用 default）
	// userIDs: 要通知的用户 ID 列表
	// title: 通知标题
	// content: 通知内容
	DefaultNotification(ctx context.Context, userIDs []uint64, title, content string) error

	// 聚合器管理方法
	// GetAggregator 获取聚合器实例
	GetAggregator() *AlertAggregator
	// EnableAggregator 动态启用聚合器
	EnableAggregator() error
	// DisableAggregator 动态禁用聚合器
	DisableAggregator() error
	// GetAggregatorStatus 获取聚合器状态
	GetAggregatorStatus() map[string]interface{}

	// Close 关闭管理器，释放所有资源
	Close() error
}

// GroupChannelInfo 告警组渠道信息
// 用于存储告警组与渠道的关联关系
type GroupChannelInfo struct {
	// GroupID 告警组 ID
	GroupID uint64 `json:"groupId"`
	// Severity 告警级别
	Severity string `json:"severity"`
	// ChannelIDs 渠道 ID 列表
	ChannelIDs []uint64 `json:"channelIds"`
	// ChannelType 渠道类型
	ChannelType AlertType `json:"channelType"`
}

// ClusterStat 集群维度统计
// 用于统计单个集群的告警数量
type ClusterStat struct {
	// ClusterName 集群名称
	ClusterName string `json:"clusterName"`
	// FiringCount 触发中的告警数量
	FiringCount int `json:"firingCount"`
	// ResolvedCount 已恢复的告警数量
	ResolvedCount int `json:"resolvedCount"`
}

// AggregatedAlertGroup 聚合后的告警组
// 用于存储按项目聚合后的多级别告警数据
type AggregatedAlertGroup struct {
	// ProjectID 项目 ID
	ProjectID uint64 `json:"projectId"`
	// ProjectName 项目名称
	ProjectName string `json:"projectName"`
	// ChannelType 渠道类型（用于区分不同渠道的聚合）
	ChannelType AlertType `json:"channelType"`

	// AlertsBySeverity 按级别分组的告警
	// key: severity (CRITICAL, WARNING, INFO)
	// value: 该级别的告警列表
	AlertsBySeverity map[string][]*AlertInstance `json:"alertsBySeverity"`

	// ResolvedAlerts 已恢复的告警列表
	ResolvedAlerts []*AlertInstance `json:"resolvedAlerts"`

	// ClusterStats 集群维度统计
	// key: clusterName
	// value: 该集群的统计信息
	ClusterStats map[string]*ClusterStat `json:"clusterStats"`

	// 时间信息
	FirstAlertTime time.Time `json:"firstAlertTime"`
	LastAlertTime  time.Time `json:"lastAlertTime"`

	// 统计信息
	TotalFiring   int `json:"totalFiring"`
	TotalResolved int `json:"totalResolved"`
}
