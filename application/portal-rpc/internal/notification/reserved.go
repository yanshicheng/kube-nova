package notification

import (
	"context"

	"github.com/yanshicheng/kube-nova/common/handler/errorx"
)

// ==================== 短信告警（预留） ====================

// smsClient 短信告警客户端（预留）
type smsClient struct {
	*baseClient
	config *SMSConfig
}

// NewSMSClient 创建短信客户端（预留）
func NewSMSClient(config Config) (FullChannel, error) {
	return nil, errorx.Msg("短信告警功能暂未实现")
}

// WithContext 设置上下文
func (c *smsClient) WithContext(ctx context.Context) Client {
	c.baseClient.ctx = ctx
	return c
}

// SendPrometheusAlert 发送 Prometheus 告警（预留）
func (c *smsClient) SendPrometheusAlert(ctx context.Context, opts *AlertOptions, alerts []*AlertInstance) (*SendResult, error) {
	return nil, errorx.Msg("短信告警功能暂未实现")
}

// SendNotification 发送通用通知（预留）
func (c *smsClient) SendNotification(ctx context.Context, opts *NotificationOptions) (*SendResult, error) {
	return nil, errorx.Msg("短信告警功能暂未实现")
}

// TestConnection 测试连接（预留）
func (c *smsClient) TestConnection(ctx context.Context, toEmail []string) (*TestResult, error) {
	return nil, errorx.Msg("短信告警功能暂未实现")
}

// HealthCheck 健康检查（预留）
func (c *smsClient) HealthCheck(ctx context.Context) (*HealthStatus, error) {
	return nil, errorx.Msg("短信告警功能暂未实现")
}

// ==================== 语音告警（预留） ====================

// voiceCallClient 语音告警客户端（预留）
type voiceCallClient struct {
	*baseClient
	config *VoiceCallConfig
}

// NewVoiceCallClient 创建语音告警客户端（预留）
func NewVoiceCallClient(config Config) (FullChannel, error) {
	return nil, errorx.Msg("语音告警功能暂未实现")
}

// WithContext 设置上下文
func (c *voiceCallClient) WithContext(ctx context.Context) Client {
	c.baseClient.ctx = ctx
	return c
}

// SendPrometheusAlert 发送 Prometheus 告警（预留）
func (c *voiceCallClient) SendPrometheusAlert(ctx context.Context, opts *AlertOptions, alerts []*AlertInstance) (*SendResult, error) {
	return nil, errorx.Msg("语音告警功能暂未实现")
}

// SendNotification 发送通用通知（预留）
func (c *voiceCallClient) SendNotification(ctx context.Context, opts *NotificationOptions) (*SendResult, error) {
	return nil, errorx.Msg("语音告警功能暂未实现")
}

// TestConnection 测试连接（预留）
func (c *voiceCallClient) TestConnection(ctx context.Context, toEmail []string) (*TestResult, error) {
	return nil, errorx.Msg("语音告警功能暂未实现")
}

// HealthCheck 健康检查（预留）
func (c *voiceCallClient) HealthCheck(ctx context.Context) (*HealthStatus, error) {
	return nil, errorx.Msg("语音告警功能暂未实现")
}
