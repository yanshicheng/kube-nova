package notification

import (
	"context"
	"time"
)

// AlertType 告警类型
type AlertType string

const (
	AlertTypeDingTalk    AlertType = "dingtalk"
	AlertTypeWeChat      AlertType = "wechat"
	AlertTypeFeiShu      AlertType = "feishu"
	AlertTypeEmail       AlertType = "email"
	AlertTypeSMS         AlertType = "sms"
	AlertTypeVoiceCall   AlertType = "voice_call"
	AlertTypeWebhook     AlertType = "webhook"
	AlertTypeSiteMessage AlertType = "site_message"
)

// AlertLevel 告警级别
type AlertLevel string

const (
	AlertLevelDefault      AlertLevel = "default"
	AlertLevelInfo         AlertLevel = "info"
	AlertLevelWarning      AlertLevel = "warning"
	AlertLevelCritical     AlertLevel = "critical"
	AlertLevelNotification AlertLevel = "notification"
)

// AlertStatus 告警状态
type AlertStatus string

const (
	AlertStatusFiring   AlertStatus = "firing"
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
type AlertInstance struct {
	ID             uint64            `json:"id"`
	Instance       string            `json:"instance"`
	Fingerprint    string            `json:"fingerprint"`
	ClusterUUID    string            `json:"clusterUuid"`
	ClusterName    string            `json:"clusterName"`
	ProjectID      uint64            `json:"projectId"`
	ProjectName    string            `json:"projectName"`
	WorkspaceID    uint64            `json:"workspaceId"`
	WorkspaceName  string            `json:"workspaceName"`
	AlertName      string            `json:"alertName"`
	Severity       string            `json:"severity"`
	Status         string            `json:"status"`
	Labels         map[string]string `json:"labels"`
	Annotations    map[string]string `json:"annotations"`
	GeneratorURL   string            `json:"generatorUrl"`
	StartsAt       time.Time         `json:"startsAt"`
	EndsAt         *time.Time        `json:"endsAt"`
	ResolvedAt     *time.Time        `json:"resolvedAt"`
	Duration       uint              `json:"duration"`
	RepeatCount    uint              `json:"repeatCount"`
	LastNotifiedAt *time.Time        `json:"lastNotifiedAt"`
}

// MentionConfig @人配置（发送时动态指定）
type MentionConfig struct {
	// 钉钉/微信 @手机号
	AtMobiles []string `json:"atMobiles,omitempty"`
	// 飞书/微信 @用户ID
	AtUserIds []string `json:"atUserIds,omitempty"`
	// 是否 @ 所有人
	IsAtAll bool `json:"isAtAll,omitempty"`
	// 邮件收件人
	EmailTo []string `json:"emailTo,omitempty"`
	// 邮件抄送
	EmailCC []string `json:"emailCC,omitempty"`
	// 邮件密送
	EmailBCC []string `json:"emailBCC,omitempty"`
}

// AlertOptions Prometheus 告警发送选项
type AlertOptions struct {
	// 平台名称
	PortalName string `json:"portalName"`
	// 平台URL
	PortalUrl string `json:"portalUrl"`
	// @人配置
	Mentions *MentionConfig `json:"mentions,omitempty"`
	// 项目名称
	ProjectName string `json:"projectName"`
	// 集群名称
	ClusterName string `json:"clusterName"`
	// 告警级别
	Severity string `json:"severity"`
}

// NotificationOptions 通用通知选项
type NotificationOptions struct {
	// 平台名称
	PortalName string `json:"portalName"`
	// 平台URL
	PortalUrl string `json:"portalUrl"`
	// 通知标题
	Title string `json:"title"`
	// 通知内容
	Content string `json:"content"`
	// @人配置
	Mentions *MentionConfig `json:"mentions,omitempty"`
}

// SendResult 发送结果
type SendResult struct {
	Success   bool      `json:"success"`
	Message   string    `json:"message"`
	MessageID string    `json:"messageId,omitempty"`
	Timestamp time.Time `json:"timestamp"`
	Error     error     `json:"error,omitempty"`
	CostMs    int64     `json:"costMs,omitempty"`
}

// TestResult 测试结果
type TestResult struct {
	Success   bool                   `json:"success"`
	Message   string                 `json:"message"`
	Latency   time.Duration          `json:"latency"`
	Timestamp time.Time              `json:"timestamp"`
	Error     error                  `json:"error,omitempty"`
	Details   map[string]interface{} `json:"details,omitempty"`
}

// HealthStatus 健康状态
type HealthStatus struct {
	Healthy       bool      `json:"healthy"`
	Message       string    `json:"message"`
	LastCheckTime time.Time `json:"lastCheckTime"`
	Error         error     `json:"error,omitempty"`
}

// Statistics 统计信息
type Statistics struct {
	TotalSent       int64         `json:"totalSent"`
	TotalSuccess    int64         `json:"totalSuccess"`
	TotalFailed     int64         `json:"totalFailed"`
	LastSentTime    time.Time     `json:"lastSentTime"`
	LastSuccessTime time.Time     `json:"lastSuccessTime"`
	LastFailedTime  time.Time     `json:"lastFailedTime"`
	AverageLatency  time.Duration `json:"averageLatency"`
}

// Client 告警客户端基础接口
type Client interface {
	// 基本信息
	GetUUID() string
	GetName() string
	GetType() AlertType
	GetConfig() Config

	// 健康检查
	HealthCheck(ctx context.Context) (*HealthStatus, error)
	IsHealthy() bool

	// 测试连接
	TestConnection(ctx context.Context, toEmail []string) (*TestResult, error)

	// 统计信息
	GetStatistics() *Statistics
	ResetStatistics()

	// 生命周期
	Close() error

	// 设置上下文
	WithContext(ctx context.Context) Client
}

// PrometheusAlertChannel Prometheus 告警渠道接口
type PrometheusAlertChannel interface {
	Client
	// SendPrometheusAlert 发送 Prometheus 告警
	SendPrometheusAlert(ctx context.Context, opts *AlertOptions, alerts []*AlertInstance) (*SendResult, error)
}

// NotificationChannel 通用通知渠道接口
type NotificationChannel interface {
	Client
	// SendNotification 发送通用通知
	SendNotification(ctx context.Context, opts *NotificationOptions) (*SendResult, error)
}

// FullChannel 完整渠道接口（同时支持 Prometheus 告警和通用通知）
type FullChannel interface {
	PrometheusAlertChannel
	NotificationChannel
}

// Manager 告警管理器接口
type Manager interface {
	// 设置平台信息
	SetPortalInfo(portalName, portalUrl string)
	GetPortalName() string
	GetPortalUrl() string

	// 告警渠道管理
	AddChannel(config Config) error
	RemoveChannel(uuid string) error
	UpdateChannel(config Config) error
	GetChannel(ctx context.Context, uuid string) (Client, error)
	ListChannels() []Client
	GetChannelCount() int
	GetChannelsByType(alertType AlertType) []Client

	// ========================================
	// Prometheus 告警发送（核心方法）
	// ========================================
	// 只接收 ctx 和 alerts，内部自动：
	// 1. 按 ProjectID + Severity 分组
	// 2. 查询项目绑定的告警组（ProjectID=0 使用默认告警组 ID=2）
	// 3. 获取对应级别的通知渠道（没有则用 default）
	// 4. 查询告警组成员获取@人信息
	// 5. 并发发送到所有渠道
	// 6. 为每条告警创建站内信
	// 7. 记录通知日志
	PrometheusAlertNotification(ctx context.Context, alerts []*AlertInstance) error

	// ========================================
	// 通用通知发送（如创建账号等系统通知）
	// ========================================
	// 查询默认告警组的 notification 级别渠道（没有则用 default）
	// userIDs: 用户ID列表
	// title: 通知标题
	// content: 通知内容
	DefaultNotification(ctx context.Context, userIDs []uint64, title, content string) error

	// 生命周期
	Close() error
}

// GroupChannelInfo 告警组渠道信息
type GroupChannelInfo struct {
	GroupID     uint64    `json:"groupId"`
	Severity    string    `json:"severity"`
	ChannelIDs  []uint64  `json:"channelIds"`
	ChannelType AlertType `json:"channelType"`
}
