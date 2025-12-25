package notification

import (
	"time"
)

// Config 告警配置结构
type Config struct {
	// UUID 告警渠道唯一标识
	UUID string `json:"uuid" yaml:"uuid"`
	// Name 告警渠道名称
	Name string `json:"name" yaml:"name"`
	// Type 告警类型
	Type AlertType `json:"type" yaml:"type"`

	// DingTalk 钉钉配置
	DingTalk *DingTalkConfig `json:"dingtalk,omitempty" yaml:"dingtalk,omitempty"`
	// WeChat 企业微信配置
	WeChat *WeChatConfig `json:"wechat,omitempty" yaml:"wechat,omitempty"`
	// FeiShu 飞书配置
	FeiShu *FeiShuConfig `json:"feishu,omitempty" yaml:"feishu,omitempty"`
	// Email 邮件配置
	Email *EmailConfig `json:"email,omitempty" yaml:"email,omitempty"`
	// SMS 短信配置（预留）
	SMS *SMSConfig `json:"sms,omitempty" yaml:"sms,omitempty"`
	// VoiceCall 语音告警配置（预留）
	VoiceCall *VoiceCallConfig `json:"voiceCall,omitempty" yaml:"voiceCall,omitempty"`
	// Webhook Webhook配置
	Webhook *WebhookConfig `json:"webhook,omitempty" yaml:"webhook,omitempty"`

	// Options 通用选项
	Options ClientOptions `json:"options,omitempty" yaml:"options,omitempty"`
}

// ClientOptions 客户端选项
type ClientOptions struct {
	// Timeout 请求超时时间
	Timeout time.Duration `json:"timeout,omitempty" yaml:"timeout,omitempty"`
	// RetryCount 重试次数
	RetryCount int `json:"retryCount,omitempty" yaml:"retryCount,omitempty"`
	// RetryInterval 重试间隔
	RetryInterval time.Duration `json:"retryInterval,omitempty" yaml:"retryInterval,omitempty"`
	// EnableMetrics 是否启用指标统计
	EnableMetrics bool `json:"enableMetrics" yaml:"enableMetrics"`
	// RateLimitPerMinute 每分钟速率限制（0表示不限制）
	RateLimitPerMinute int `json:"rateLimitPerMinute,omitempty" yaml:"rateLimitPerMinute,omitempty"`
}

// DingTalkConfig 钉钉配置
type DingTalkConfig struct {
	// Webhook 钉钉机器人 Webhook 地址
	Webhook string `json:"webhook" yaml:"webhook"`
	// Secret 签名密钥（可选）
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
	// Secret 签名密钥（可选）
	Secret string `json:"secret,omitempty" yaml:"secret,omitempty"`
}

// EmailConfig 邮件配置
type EmailConfig struct {
	// SMTPHost SMTP 服务器地址
	SMTPHost string `json:"smtpHost" yaml:"smtpHost"`
	// SMTPPort SMTP 服务器端口
	SMTPPort int `json:"smtpPort" yaml:"smtpPort"`
	// Username 发件人邮箱
	Username string `json:"username" yaml:"username"`
	// Password 发件人密码或授权码
	Password string `json:"password" yaml:"password"`
	// UseTLS 是否使用 TLS
	UseTLS bool `json:"useTls" yaml:"useTls"`
	// UseSSL 是否使用 SSL（兼容字段，与 UseTLS 功能相同，465端口通常用此字段）
	UseSSL bool `json:"useSSL" yaml:"useSSL"`
	// UseStartTLS 是否使用 STARTTLS（587端口通常用此字段）
	UseStartTLS bool `json:"useStartTls" yaml:"useStartTls"`
	// From 发件人名称
	From string `json:"from,omitempty" yaml:"from,omitempty"`
}

// ShouldUseTLS 判断是否应该使用 TLS（兼容 UseTLS 和 UseSSL）
func (c *EmailConfig) ShouldUseTLS() bool {
	return c.UseTLS || c.UseSSL
}

// SMSConfig 短信配置（预留）
type SMSConfig struct {
	// Provider 短信服务提供商（aliyun, tencent, etc.）
	Provider string `json:"provider" yaml:"provider"`
	// AccessKeyId 访问密钥ID
	AccessKeyId string `json:"accessKeyId" yaml:"accessKeyId"`
	// AccessKeySecret 访问密钥
	AccessKeySecret string `json:"accessKeySecret" yaml:"accessKeySecret"`
	// SignName 短信签名
	SignName string `json:"signName" yaml:"signName"`
	// TemplateCode 短信模板代码
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
	// URL Webhook 地址
	URL string `json:"url" yaml:"url"`
	// Method HTTP 方法
	Method string `json:"method,omitempty" yaml:"method,omitempty"`
	// Headers 请求头
	Headers map[string]string `json:"headers,omitempty" yaml:"headers,omitempty"`
	// AuthType 认证类型（none, basic, bearer）
	AuthType string `json:"authType,omitempty" yaml:"authType,omitempty"`
	// AuthToken 认证令牌
	AuthToken string `json:"authToken,omitempty" yaml:"authToken,omitempty"`
	// Username 用户名（用于 Basic Auth）
	Username string `json:"username,omitempty" yaml:"username,omitempty"`
	// Password 密码（用于 Basic Auth）
	Password string `json:"password,omitempty" yaml:"password,omitempty"`
}

// DefaultClientOptions 默认客户端选项
func DefaultClientOptions() ClientOptions {
	return ClientOptions{
		Timeout:            10 * time.Second,
		RetryCount:         3,
		RetryInterval:      2 * time.Second,
		EnableMetrics:      true,
		RateLimitPerMinute: 60,
	}
}
