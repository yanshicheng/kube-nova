package notification

import (
	"time"
)

// Config 告警配置结构
// 包含所有告警渠道的通用配置和各渠道特定配置
type Config struct {
	// UUID 告警渠道唯一标识，用于在系统中唯一识别一个告警渠道
	UUID string `json:"uuid" yaml:"uuid"`
	// Name 告警渠道名称，用于显示和日志记录
	Name string `json:"name" yaml:"name"`
	// Type 告警类型，决定使用哪种渠道发送告警
	Type AlertType `json:"type" yaml:"type"`

	// DingTalk 钉钉配置，当 Type 为 dingtalk 时使用
	DingTalk *DingTalkConfig `json:"dingtalk,omitempty" yaml:"dingtalk,omitempty"`
	// WeChat 企业微信配置，当 Type 为 wechat 时使用
	WeChat *WeChatConfig `json:"wechat,omitempty" yaml:"wechat,omitempty"`
	// FeiShu 飞书配置，当 Type 为 feishu 时使用
	FeiShu *FeiShuConfig `json:"feishu,omitempty" yaml:"feishu,omitempty"`
	// Email 邮件配置，当 Type 为 email 时使用
	Email *EmailConfig `json:"email,omitempty" yaml:"email,omitempty"`
	// SMS 短信配置（预留），当 Type 为 sms 时使用
	SMS *SMSConfig `json:"sms,omitempty" yaml:"sms,omitempty"`
	// VoiceCall 语音告警配置（预留），当 Type 为 voice_call 时使用
	VoiceCall *VoiceCallConfig `json:"voiceCall,omitempty" yaml:"voiceCall,omitempty"`
	// Webhook Webhook配置，当 Type 为 webhook 时使用
	Webhook *WebhookConfig `json:"webhook,omitempty" yaml:"webhook,omitempty"`

	// Options 通用选项，包含超时、重试等配置
	Options ClientOptions `json:"options,omitempty" yaml:"options,omitempty"`
}

// ClientOptions 客户端选项
// 定义所有告警客户端的通用行为参数
type ClientOptions struct {
	// Timeout 请求超时时间，超过此时间未响应则视为失败
	Timeout time.Duration `json:"timeout,omitempty" yaml:"timeout,omitempty"`
	// RetryCount 重试次数，发送失败后最多重试的次数
	RetryCount int `json:"retryCount,omitempty" yaml:"retryCount,omitempty"`
	// RetryInterval 重试间隔，每次重试之间等待的时间
	RetryInterval time.Duration `json:"retryInterval,omitempty" yaml:"retryInterval,omitempty"`
	// EnableMetrics 是否启用指标统计，用于性能监控
	EnableMetrics bool `json:"enableMetrics" yaml:"enableMetrics"`
	// RateLimitPerMinute 每分钟速率限制，0 表示不限制
	RateLimitPerMinute int `json:"rateLimitPerMinute,omitempty" yaml:"rateLimitPerMinute,omitempty"`
}

// DingTalkConfig 钉钉配置
type DingTalkConfig struct {
	// Webhook 钉钉机器人 Webhook 地址，从钉钉群设置中获取
	Webhook string `json:"webhook" yaml:"webhook"`
	// Secret 签名密钥（可选），用于消息安全验证
	Secret string `json:"secret,omitempty" yaml:"secret,omitempty"`
}

// WeChatConfig 企业微信配置
type WeChatConfig struct {
	// Webhook 企业微信机器人 Webhook 地址
	Webhook string `json:"webhook" yaml:"webhook"`
}

// FeiShuConfig 飞书配置
type FeiShuConfig struct {
	// Webhook 飞书机器人 Webhook 地址
	Webhook string `json:"webhook" yaml:"webhook"`
	// Secret 签名密钥（可选），用于消息安全验证
	Secret string `json:"secret,omitempty" yaml:"secret,omitempty"`
}

// EmailConfig 邮件配置
type EmailConfig struct {
	// SMTPHost SMTP 服务器地址，如 smtp.163.com
	SMTPHost string `json:"smtpHost" yaml:"smtpHost"`
	// SMTPPort SMTP 服务器端口，常用: 25(无加密)、465(SSL)、587(STARTTLS)
	SMTPPort int `json:"smtpPort" yaml:"smtpPort"`
	// Username 发件人邮箱地址，用于 SMTP 认证
	Username string `json:"username" yaml:"username"`
	// Password 发件人密码或授权码，部分邮箱需要使用授权码
	Password string `json:"password" yaml:"password"`
	// UseTLS 是否使用 TLS 加密（465 端口）
	UseTLS bool `json:"useTls" yaml:"useTls"`
	// UseSSL 是否使用 SSL 加密，与 UseTLS 功能相同，兼容旧配置
	UseSSL bool `json:"useSSL" yaml:"useSSL"`
	// UseStartTLS 是否使用 STARTTLS 加密（587 端口）
	UseStartTLS bool `json:"useStartTls" yaml:"useStartTls"`
	// From 发件人显示名称，如 "系统告警 <alert@example.com>"
	From string `json:"from,omitempty" yaml:"from,omitempty"`
	// InsecureSkipVerify 是否跳过 TLS 证书验证
	// 警告: 生产环境建议设为 false，设为 true 会有中间人攻击风险
	InsecureSkipVerify bool `json:"insecureSkipVerify,omitempty" yaml:"insecureSkipVerify,omitempty"`
}

// ShouldUseTLS 判断是否应该使用 TLS 加密
// 兼容 UseTLS、UseSSL 和 useSSL 三种配置方式
func (c *EmailConfig) ShouldUseTLS() bool {
	return c.UseTLS || c.UseSSL
}

// SMSConfig 短信配置（预留）
type SMSConfig struct {
	// Provider 短信服务提供商，如 aliyun、tencent 等
	Provider string `json:"provider" yaml:"provider"`
	// AccessKeyId 访问密钥ID，从云服务商控制台获取
	AccessKeyId string `json:"accessKeyId" yaml:"accessKeyId"`
	// AccessKeySecret 访问密钥，从云服务商控制台获取
	AccessKeySecret string `json:"accessKeySecret" yaml:"accessKeySecret"`
	// SignName 短信签名，需要在云服务商平台申请
	SignName string `json:"signName" yaml:"signName"`
	// TemplateCode 短信模板代码，需要在云服务商平台申请
	TemplateCode string `json:"templateCode" yaml:"templateCode"`
}

// VoiceCallConfig 语音告警配置（预留）
type VoiceCallConfig struct {
	// Provider 语音服务提供商
	Provider string `json:"provider" yaml:"provider"`
	// AccessKeyId 访问密钥ID
	AccessKeyId string `json:"accessKeyId" yaml:"accessKeyId"`
	// AccessKeySecret 访问密钥
	AccessKeySecret string `json:"accessKeySecret" yaml:"accessKeySecret"`
	// TtsCode 文本转语音模板代码
	TtsCode string `json:"ttsCode" yaml:"ttsCode"`
}

// WebhookConfig Webhook配置
type WebhookConfig struct {
	// URL Webhook 接收地址
	URL string `json:"url" yaml:"url"`
	// Method HTTP 请求方法，默认 POST
	Method string `json:"method,omitempty" yaml:"method,omitempty"`
	// Headers 自定义请求头
	Headers map[string]string `json:"headers,omitempty" yaml:"headers,omitempty"`
	// AuthType 认证类型: none（无认证）、basic（基本认证）、bearer（令牌认证）
	AuthType string `json:"authType,omitempty" yaml:"authType,omitempty"`
	// AuthToken 认证令牌，用于 bearer 认证
	AuthToken string `json:"authToken,omitempty" yaml:"authToken,omitempty"`
	// Username 用户名，用于 basic 认证
	Username string `json:"username,omitempty" yaml:"username,omitempty"`
	// Password 密码，用于 basic 认证
	Password string `json:"password,omitempty" yaml:"password,omitempty"`
}

// DefaultClientOptions 返回默认客户端选项
// 提供合理的默认值，避免零值导致的问题
func DefaultClientOptions() ClientOptions {
	return ClientOptions{
		Timeout:            10 * time.Second, // 默认 10 秒超时
		RetryCount:         3,                // 默认重试 3 次
		RetryInterval:      2 * time.Second,  // 默认重试间隔 2 秒
		EnableMetrics:      true,             // 默认启用指标统计
		RateLimitPerMinute: 60,               // 默认每分钟 60 次
	}
}
