package notification

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/yanshicheng/kube-nova/common/handler/errorx"
)

// webhookClient Webhook 告警客户端
// 实现通过自定义 Webhook 发送告警消息
type webhookClient struct {
	*baseClient
	config     *WebhookConfig
	httpClient *http.Client // 复用的 HTTP 客户端，避免每次请求创建新连接
}

// WebhookPayload Webhook 发送的数据结构
type WebhookPayload struct {
	// EventType 事件类型
	EventType string `json:"eventType"`
	// Timestamp 时间戳
	Timestamp int64 `json:"timestamp"`
	// Portal 平台信息
	Portal struct {
		Name string `json:"name"`
		URL  string `json:"url"`
	} `json:"portal"`
	// AlertOptions 告警选项（Prometheus 告警时有值）
	AlertOptions *AlertOptions `json:"alertOptions,omitempty"`
	// Alerts 告警实例列表（Prometheus 告警时有值）
	Alerts []*AlertInstance `json:"alerts,omitempty"`
	// NotificationOptions 通知选项（通用通知时有值）
	NotificationOptions *NotificationOptions `json:"notificationOptions,omitempty"`
}

// NewWebhookClient 创建 Webhook 客户端
func NewWebhookClient(config Config) (FullChannel, error) {
	if config.Webhook == nil {
		return nil, errorx.Msg("webhook config is required")
	}

	// 设置默认 Method
	if config.Webhook.Method == "" {
		config.Webhook.Method = "POST"
	}

	bc := newBaseClient(config)

	// 创建可复用的 HTTP 客户端，避免每次请求创建新连接
	// 这样可以复用 TCP 连接，减少连接建立开销
	httpClient := &http.Client{
		Timeout: config.Options.Timeout,
		Transport: &http.Transport{
			MaxIdleConns:        100,              // 最大空闲连接数
			MaxIdleConnsPerHost: 10,               // 每个主机最大空闲连接数
			IdleConnTimeout:     90 * time.Second, // 空闲连接超时时间
		},
	}

	client := &webhookClient{
		baseClient: bc,
		config:     config.Webhook,
		httpClient: httpClient,
	}

	client.l.Infof("创建 Webhook 告警客户端: %s", config.UUID)
	return client, nil
}

// WithContext 设置上下文
// 返回带有新上下文的客户端副本
func (c *webhookClient) WithContext(ctx context.Context) Client {
	// 创建新的 baseClient 副本
	newBaseClient := c.baseClient.WithContext(ctx).(*baseClient)

	// 创建新的 webhookClient 实例（共享 config 和 httpClient）
	return &webhookClient{
		baseClient: newBaseClient,
		config:     c.config,
		httpClient: c.httpClient,
	}
}

// SendPrometheusAlert 发送 Prometheus 告警
func (c *webhookClient) SendPrometheusAlert(ctx context.Context, opts *AlertOptions, alerts []*AlertInstance) (*SendResult, error) {
	startTime := time.Now()

	// 检查客户端状态
	if atomic.LoadInt32(&c.closed) == 1 {
		return nil, errorx.Msg("client is closed")
	}

	// 检查速率限制
	if err := c.checkRateLimit(); err != nil {
		c.recordFailure()
		return &SendResult{
			Success:   false,
			Message:   err.Error(),
			Timestamp: time.Now(),
			Error:     err,
		}, err
	}

	// 构建 Payload
	payload := &WebhookPayload{
		EventType:    "prometheus_alert",
		Timestamp:    time.Now().Unix(),
		AlertOptions: opts,
		Alerts:       alerts,
	}
	payload.Portal.Name = opts.PortalName
	payload.Portal.URL = opts.PortalUrl

	// 构建重试上下文
	retryCtx := &RetryContext{
		ChannelType: string(AlertTypeWebhook),
		ChannelUUID: c.baseClient.GetUUID(),
		ChannelName: c.baseClient.GetName(),
		Operation:   "send_prometheus_alert",
		TargetInfo:  fmt.Sprintf("URL: %s, 告警数量: %d, 项目: %s", c.config.URL, len(alerts), opts.ProjectName),
	}

	// 发送请求（带重试）
	var result *SendResult
	err := c.retryWithBackoffContext(ctx, retryCtx, func() error {
		var sendErr error
		result, sendErr = c.sendWebhook(ctx, payload)
		return sendErr
	})

	// 记录统计
	if err == nil && result != nil && result.Success {
		c.recordSuccess(time.Since(startTime))
		result.CostMs = time.Since(startTime).Milliseconds()
	} else {
		c.recordFailure()
	}

	return result, err
}

// SendNotification 发送通用通知
func (c *webhookClient) SendNotification(ctx context.Context, opts *NotificationOptions) (*SendResult, error) {
	startTime := time.Now()

	// 检查客户端状态
	if atomic.LoadInt32(&c.closed) == 1 {
		return nil, errorx.Msg("client is closed")
	}

	// 检查速率限制
	if err := c.checkRateLimit(); err != nil {
		c.recordFailure()
		return &SendResult{
			Success:   false,
			Message:   err.Error(),
			Timestamp: time.Now(),
			Error:     err,
		}, err
	}

	// 构建 Payload
	payload := &WebhookPayload{
		EventType:           "notification",
		Timestamp:           time.Now().Unix(),
		NotificationOptions: opts,
	}
	payload.Portal.Name = opts.PortalName
	payload.Portal.URL = opts.PortalUrl

	// 构建重试上下文
	retryCtx := &RetryContext{
		ChannelType: string(AlertTypeWebhook),
		ChannelUUID: c.baseClient.GetUUID(),
		ChannelName: c.baseClient.GetName(),
		Operation:   "send_notification",
		TargetInfo:  fmt.Sprintf("URL: %s, 标题: %s", c.config.URL, opts.Title),
	}

	// 发送请求（带重试）
	var result *SendResult
	err := c.retryWithBackoffContext(ctx, retryCtx, func() error {
		var sendErr error
		result, sendErr = c.sendWebhook(ctx, payload)
		return sendErr
	})

	// 记录统计
	if err == nil && result != nil && result.Success {
		c.recordSuccess(time.Since(startTime))
		result.CostMs = time.Since(startTime).Milliseconds()
	} else {
		c.recordFailure()
	}

	return result, err
}

// sendWebhook 发送 Webhook 请求
func (c *webhookClient) sendWebhook(ctx context.Context, payload *WebhookPayload) (*SendResult, error) {
	// 序列化 Payload
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, errorx.Msg(fmt.Sprintf("failed to marshal payload: %v", err))
	}

	// 创建 HTTP 请求
	req, err := http.NewRequestWithContext(ctx, c.config.Method, c.config.URL, bytes.NewBuffer(body))
	if err != nil {
		return nil, errorx.Msg(fmt.Sprintf("failed to create request: %v", err))
	}

	// 设置默认 Content-Type
	req.Header.Set("Content-Type", "application/json")

	// 添加自定义请求头
	for k, v := range c.config.Headers {
		req.Header.Set(k, v)
	}

	// 添加认证
	c.addAuth(req)

	// 使用复用的 HTTP 客户端发送请求
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, errorx.Msg(fmt.Sprintf("failed to send request: %v", err))
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// 读取响应
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errorx.Msg(fmt.Sprintf("failed to read response: %v", err))
	}

	// 检查响应状态码
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		errMsg := fmt.Sprintf("http status code: %d, response: %s", resp.StatusCode, string(respBody))
		return &SendResult{
			Success:   false,
			Message:   fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(respBody)),
			Timestamp: time.Now(),
			Error:     errorx.Msg(fmt.Sprintf("http status code: %d", resp.StatusCode)),
		}, errorx.Msg(errMsg)
	}

	c.l.Infof("[Webhook] 发送成功 | UUID: %s | URL: %s", c.baseClient.GetUUID(), c.config.URL)
	return &SendResult{
		Success:   true,
		Message:   "发送成功",
		Timestamp: time.Now(),
	}, nil
}

// addAuth 添加认证信息
func (c *webhookClient) addAuth(req *http.Request) {
	switch c.config.AuthType {
	case "basic":
		auth := base64.StdEncoding.EncodeToString([]byte(c.config.Username + ":" + c.config.Password))
		req.Header.Set("Authorization", "Basic "+auth)
	case "bearer":
		req.Header.Set("Authorization", "Bearer "+c.config.AuthToken)
	}
}

// TestConnection 测试连接
func (c *webhookClient) TestConnection(ctx context.Context, _ []string) (*TestResult, error) {
	startTime := time.Now()

	testOpts := &NotificationOptions{
		PortalName: "测试平台",
		PortalUrl:  "https://example.com",
		Title:      "Webhook 连接测试",
		Content:    "这是一条测试消息，用于验证 Webhook 配置是否正确。",
	}

	result, err := c.SendNotification(ctx, testOpts)

	latency := time.Since(startTime)

	if err != nil {
		c.updateHealthStatus(false, err)
		return &TestResult{
			Success:   false,
			Message:   fmt.Sprintf("测试失败: %v", err),
			Latency:   latency,
			Timestamp: time.Now(),
			Error:     err,
			Details: map[string]interface{}{
				"url":      c.config.URL,
				"method":   c.config.Method,
				"authType": c.config.AuthType,
			},
		}, err
	}

	c.updateHealthStatus(true, nil)
	return &TestResult{
		Success:   result.Success,
		Message:   "Webhook 连接测试成功",
		Latency:   latency,
		Timestamp: time.Now(),
		Details: map[string]interface{}{
			"url":      c.config.URL,
			"method":   c.config.Method,
			"authType": c.config.AuthType,
		},
	}, nil
}

// HealthCheck 健康检查
func (c *webhookClient) HealthCheck(_ context.Context) (*HealthStatus, error) {
	if atomic.LoadInt32(&c.closed) == 1 {
		return &HealthStatus{
			Healthy:       false,
			Message:       "客户端已关闭",
			LastCheckTime: time.Now(),
		}, errorx.Msg("client is closed")
	}

	// 检查 URL 配置
	if c.config.URL == "" {
		c.updateHealthStatus(false, errorx.Msg("webhook URL is empty"))
		return &HealthStatus{
			Healthy:       false,
			Message:       "Webhook URL 为空",
			LastCheckTime: time.Now(),
		}, errorx.Msg("webhook URL is empty")
	}

	c.updateHealthStatus(true, nil)
	return &HealthStatus{
		Healthy:       true,
		Message:       "Webhook 客户端健康",
		LastCheckTime: time.Now(),
	}, nil
}

// Close 关闭客户端
// 关闭 HTTP 客户端的空闲连接
func (c *webhookClient) Close() error {
	if c.httpClient != nil {
		c.httpClient.CloseIdleConnections()
	}
	return c.baseClient.Close()
}
