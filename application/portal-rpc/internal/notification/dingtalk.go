package notification

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync/atomic"
	"time"
)

// dingTalkClient 钉钉告警客户端
type dingTalkClient struct {
	*baseClient
	config *DingTalkConfig
}

// dingTalkMessage 钉钉消息结构
type dingTalkMessage struct {
	MsgType  string            `json:"msgtype"`
	Text     *dingTalkText     `json:"text,omitempty"`
	Markdown *dingTalkMarkdown `json:"markdown,omitempty"`
	At       *dingTalkAt       `json:"at,omitempty"`
}

type dingTalkText struct {
	Content string `json:"content"`
}

type dingTalkMarkdown struct {
	Title string `json:"title"`
	Text  string `json:"text"`
}

type dingTalkAt struct {
	AtMobiles []string `json:"atMobiles,omitempty"`
	AtUserIds []string `json:"atUserIds,omitempty"`
	IsAtAll   bool     `json:"isAtAll,omitempty"`
}

type dingTalkResponse struct {
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
}

// NewDingTalkClient 创建钉钉客户端
func NewDingTalkClient(config Config) (FullChannel, error) {
	if config.DingTalk == nil {
		return nil, fmt.Errorf("钉钉配置不能为空")
	}

	bc := newBaseClient(config)
	client := &dingTalkClient{
		baseClient: bc,
		config:     config.DingTalk,
	}

	client.l.Infof("创建钉钉告警客户端: %s", config.UUID)
	return client, nil
}

// WithContext 设置上下文
func (c *dingTalkClient) WithContext(ctx context.Context) Client {
	c.baseClient.ctx = ctx
	c.baseClient.l = c.baseClient.l
	return c
}

// SendPrometheusAlert 发送 Prometheus 告警
func (c *dingTalkClient) SendPrometheusAlert(ctx context.Context, opts *AlertOptions, alerts []*AlertInstance) (*SendResult, error) {
	startTime := time.Now()

	// 检查客户端状态
	if atomic.LoadInt32(&c.closed) == 1 {
		return nil, fmt.Errorf("客户端已关闭")
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

	// 使用格式化器构建消息
	formatter := NewMessageFormatter(opts.PortalName, opts.PortalUrl)
	title, content := formatter.FormatMarkdownForDingTalk(opts, alerts)

	// 构建钉钉消息
	dingMsg := &dingTalkMessage{
		MsgType: "markdown",
		Markdown: &dingTalkMarkdown{
			Title: title,
			Text:  content,
		},
	}

	// 添加 @人配置
	if opts.Mentions != nil {
		if len(opts.Mentions.AtMobiles) > 0 || len(opts.Mentions.AtUserIds) > 0 || opts.Mentions.IsAtAll {
			dingMsg.At = &dingTalkAt{
				AtMobiles: opts.Mentions.AtMobiles,
				AtUserIds: opts.Mentions.AtUserIds,
				IsAtAll:   opts.Mentions.IsAtAll,
			}
		}
	}

	// 构建重试上下文
	retryCtx := &RetryContext{
		ChannelType: string(AlertTypeDingTalk),
		ChannelUUID: c.config.Webhook,
		ChannelName: c.baseClient.GetName(),
		Operation:   "send_prometheus_alert",
		TargetInfo:  fmt.Sprintf("告警数量: %d, 项目: %s, 集群: %s", len(alerts), opts.ProjectName, opts.ClusterName),
	}

	// 发送消息（带重试）
	var result *SendResult
	err := c.retryWithBackoffContext(ctx, retryCtx, func() error {
		var sendErr error
		result, sendErr = c.sendToDingTalk(ctx, dingMsg)
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
func (c *dingTalkClient) SendNotification(ctx context.Context, opts *NotificationOptions) (*SendResult, error) {
	startTime := time.Now()

	// 检查客户端状态
	if atomic.LoadInt32(&c.closed) == 1 {
		return nil, fmt.Errorf("客户端已关闭")
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

	// 使用格式化器构建消息
	formatter := NewMessageFormatter(opts.PortalName, opts.PortalUrl)
	title, content := formatter.FormatNotificationForDingTalk(opts)

	// 构建钉钉消息
	dingMsg := &dingTalkMessage{
		MsgType: "markdown",
		Markdown: &dingTalkMarkdown{
			Title: title,
			Text:  content,
		},
	}

	// 添加 @人配置
	if opts.Mentions != nil {
		if len(opts.Mentions.AtMobiles) > 0 || len(opts.Mentions.AtUserIds) > 0 || opts.Mentions.IsAtAll {
			dingMsg.At = &dingTalkAt{
				AtMobiles: opts.Mentions.AtMobiles,
				AtUserIds: opts.Mentions.AtUserIds,
				IsAtAll:   opts.Mentions.IsAtAll,
			}
		}
	}

	// 构建重试上下文
	atInfo := ""
	if opts.Mentions != nil {
		atInfo = fmt.Sprintf("@手机号: %v, @用户ID: %v", opts.Mentions.AtMobiles, opts.Mentions.AtUserIds)
	}
	retryCtx := &RetryContext{
		ChannelType: string(AlertTypeDingTalk),
		ChannelUUID: c.baseClient.GetUUID(),
		ChannelName: c.baseClient.GetName(),
		Operation:   "send_notification",
		TargetInfo:  fmt.Sprintf("标题: %s, %s", opts.Title, atInfo),
	}

	// 发送消息（带重试）
	var result *SendResult
	err := c.retryWithBackoffContext(ctx, retryCtx, func() error {
		var sendErr error
		result, sendErr = c.sendToDingTalk(ctx, dingMsg)
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

// sendToDingTalk 发送到钉钉
func (c *dingTalkClient) sendToDingTalk(ctx context.Context, message *dingTalkMessage) (*SendResult, error) {
	// 构建 Webhook URL
	webhookURL := c.config.Webhook

	// 如果有密钥，添加签名
	if c.config.Secret != "" {
		timestamp := time.Now().UnixMilli()
		sign := c.generateSign(timestamp, c.config.Secret)
		webhookURL = fmt.Sprintf("%s&timestamp=%d&sign=%s", webhookURL, timestamp, url.QueryEscape(sign))
	}

	// 序列化消息
	body, err := json.Marshal(message)
	if err != nil {
		return nil, fmt.Errorf("序列化消息失败: %w", err)
	}

	// 创建 HTTP 请求
	req, err := http.NewRequestWithContext(ctx, "POST", webhookURL, bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// 发送请求
	client := &http.Client{
		Timeout: c.baseClient.config.Options.Timeout,
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("发送请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	// 解析响应
	var dingResp dingTalkResponse
	if err := json.Unmarshal(respBody, &dingResp); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	// 检查响应结果
	if dingResp.ErrCode != 0 {
		return &SendResult{
			Success:   false,
			Message:   dingResp.ErrMsg,
			Timestamp: time.Now(),
			Error:     fmt.Errorf("钉钉返回错误码 %d: %s", dingResp.ErrCode, dingResp.ErrMsg),
		}, fmt.Errorf("钉钉返回错误码 %d: %s", dingResp.ErrCode, dingResp.ErrMsg)
	}

	c.l.Infof("[钉钉] 发送成功 | UUID: %s | 名称: %s", c.baseClient.GetUUID(), c.baseClient.GetName())
	return &SendResult{
		Success:   true,
		Message:   "发送成功",
		Timestamp: time.Now(),
	}, nil
}

// generateSign 生成签名
func (c *dingTalkClient) generateSign(timestamp int64, secret string) string {
	stringToSign := fmt.Sprintf("%d\n%s", timestamp, secret)
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(stringToSign))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// TestConnection 测试连接
func (c *dingTalkClient) TestConnection(ctx context.Context, toEmail []string) (*TestResult, error) {
	startTime := time.Now()

	// 创建测试消息
	testOpts := &NotificationOptions{
		PortalName: "测试平台",
		PortalUrl:  "https://example.com",
		Title:      "钉钉连接测试",
		Content:    "这是一条测试消息，用于验证钉钉告警配置是否正确。",
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
				"webhook":   c.config.Webhook,
				"hasSecret": c.config.Secret != "",
			},
		}, err
	}

	c.updateHealthStatus(true, nil)
	return &TestResult{
		Success:   result.Success,
		Message:   "钉钉连接测试成功",
		Latency:   latency,
		Timestamp: time.Now(),
		Details: map[string]interface{}{
			"webhook":   c.config.Webhook,
			"hasSecret": c.config.Secret != "",
		},
	}, nil
}

// HealthCheck 健康检查
func (c *dingTalkClient) HealthCheck(ctx context.Context) (*HealthStatus, error) {
	if atomic.LoadInt32(&c.closed) == 1 {
		return &HealthStatus{
			Healthy:       false,
			Message:       "客户端已关闭",
			LastCheckTime: time.Now(),
		}, fmt.Errorf("客户端已关闭")
	}

	// 检查 Webhook 地址格式
	if c.config.Webhook == "" {
		c.updateHealthStatus(false, fmt.Errorf("Webhook 地址为空"))
		return &HealthStatus{
			Healthy:       false,
			Message:       "Webhook 地址为空",
			LastCheckTime: time.Now(),
		}, fmt.Errorf("Webhook 地址为空")
	}

	c.updateHealthStatus(true, nil)
	return &HealthStatus{
		Healthy:       true,
		Message:       "钉钉客户端健康",
		LastCheckTime: time.Now(),
	}, nil
}
