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
	"sync/atomic"
	"time"
)

// feiShuClient 飞书告警客户端
type feiShuClient struct {
	*baseClient
	config *FeiShuConfig
}

// feiShuMessage 飞书消息结构
type feiShuMessage struct {
	Timestamp string         `json:"timestamp,omitempty"`
	Sign      string         `json:"sign,omitempty"`
	MsgType   string         `json:"msg_type"`
	Content   *feiShuContent `json:"content"`
}

type feiShuContent struct {
	Text string      `json:"text,omitempty"`
	Post *feiShuPost `json:"post,omitempty"`
}

type feiShuPost struct {
	ZhCn *feiShuPostContent `json:"zh_cn"`
}

type feiShuPostContent struct {
	Title   string                     `json:"title"`
	Content [][]map[string]interface{} `json:"content"`
}

type feiShuResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

// NewFeiShuClient 创建飞书客户端
func NewFeiShuClient(config Config) (FullChannel, error) {
	if config.FeiShu == nil {
		return nil, fmt.Errorf("飞书配置不能为空")
	}

	bc := newBaseClient(config)
	client := &feiShuClient{
		baseClient: bc,
		config:     config.FeiShu,
	}

	client.l.Infof("创建飞书告警客户端: %s", config.UUID)
	return client, nil
}

// WithContext 设置上下文
func (c *feiShuClient) WithContext(ctx context.Context) Client {
	c.baseClient.ctx = ctx
	return c
}

// SendPrometheusAlert 发送 Prometheus 告警
func (c *feiShuClient) SendPrometheusAlert(ctx context.Context, opts *AlertOptions, alerts []*AlertInstance) (*SendResult, error) {
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
	title, content := formatter.FormatRichTextForFeiShu(opts, alerts)

	// 添加 @人配置（飞书不支持@手机号，只支持@用户ID或@all）
	if opts.Mentions != nil {
		content = c.appendMentions(content, opts.Mentions)
	}

	// 构建飞书消息
	feishuMsg := &feiShuMessage{
		MsgType: "post",
		Content: &feiShuContent{
			Post: &feiShuPost{
				ZhCn: &feiShuPostContent{
					Title:   title,
					Content: content,
				},
			},
		},
	}

	// 如果有密钥，添加签名
	if c.config.Secret != "" {
		timestamp := time.Now().Unix()
		sign := c.generateSign(timestamp, c.config.Secret)
		feishuMsg.Timestamp = fmt.Sprintf("%d", timestamp)
		feishuMsg.Sign = sign
	}

	// 构建重试上下文
	retryCtx := &RetryContext{
		ChannelType: string(AlertTypeFeiShu),
		ChannelUUID: c.baseClient.GetUUID(),
		ChannelName: c.baseClient.GetName(),
		Operation:   "send_prometheus_alert",
		TargetInfo:  fmt.Sprintf("告警数量: %d, 项目: %s, 集群: %s", len(alerts), opts.ProjectName, opts.ClusterName),
	}

	// 发送消息（带重试）
	var result *SendResult
	err := c.retryWithBackoffContext(ctx, retryCtx, func() error {
		var sendErr error
		result, sendErr = c.sendToFeiShu(ctx, feishuMsg)
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
func (c *feiShuClient) SendNotification(ctx context.Context, opts *NotificationOptions) (*SendResult, error) {
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
	title, content := formatter.FormatNotificationForFeiShu(opts)

	// 添加 @人配置
	if opts.Mentions != nil {
		content = c.appendMentions(content, opts.Mentions)
	}

	// 构建飞书消息
	feishuMsg := &feiShuMessage{
		MsgType: "post",
		Content: &feiShuContent{
			Post: &feiShuPost{
				ZhCn: &feiShuPostContent{
					Title:   title,
					Content: content,
				},
			},
		},
	}

	// 如果有密钥，添加签名
	if c.config.Secret != "" {
		timestamp := time.Now().Unix()
		sign := c.generateSign(timestamp, c.config.Secret)
		feishuMsg.Timestamp = fmt.Sprintf("%d", timestamp)
		feishuMsg.Sign = sign
	}

	// 构建重试上下文
	retryCtx := &RetryContext{
		ChannelType: string(AlertTypeFeiShu),
		ChannelUUID: c.baseClient.GetUUID(),
		ChannelName: c.baseClient.GetName(),
		Operation:   "send_notification",
		TargetInfo:  fmt.Sprintf("标题: %s, @all: %v", opts.Title, opts.Mentions != nil && opts.Mentions.IsAtAll),
	}

	// 发送消息（带重试）
	var result *SendResult
	err := c.retryWithBackoffContext(ctx, retryCtx, func() error {
		var sendErr error
		result, sendErr = c.sendToFeiShu(ctx, feishuMsg)
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

// appendMentions 添加 @人信息到内容中（飞书不支持@手机号，所以当有手机号配置时直接@all）
func (c *feiShuClient) appendMentions(content [][]map[string]interface{}, mentions *MentionConfig) [][]map[string]interface{} {
	if mentions == nil {
		return content
	}

	// 飞书不支持@手机号，如果配置了手机号但没有用户ID，则@全员
	if mentions.IsAtAll || (len(mentions.AtMobiles) > 0 && len(mentions.AtUserIds) == 0) {
		content = append(content, []map[string]interface{}{
			{"tag": "at", "user_id": "all"},
		})
		return content
	}

	// 添加 @用户ID
	if len(mentions.AtUserIds) > 0 {
		atLine := []map[string]interface{}{}
		for _, userId := range mentions.AtUserIds {
			atLine = append(atLine, map[string]interface{}{
				"tag":     "at",
				"user_id": userId,
			})
		}
		content = append(content, atLine)
	}

	return content
}

// sendToFeiShu 发送到飞书
func (c *feiShuClient) sendToFeiShu(ctx context.Context, message *feiShuMessage) (*SendResult, error) {
	// 序列化消息
	body, err := json.Marshal(message)
	if err != nil {
		return nil, fmt.Errorf("序列化消息失败: %w", err)
	}

	// 创建 HTTP 请求
	req, err := http.NewRequestWithContext(ctx, "POST", c.config.Webhook, bytes.NewBuffer(body))
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
	var feishuResp feiShuResponse
	if err := json.Unmarshal(respBody, &feishuResp); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	// 检查响应结果
	if feishuResp.Code != 0 {
		return &SendResult{
			Success:   false,
			Message:   feishuResp.Msg,
			Timestamp: time.Now(),
			Error:     fmt.Errorf("飞书返回错误码 %d: %s", feishuResp.Code, feishuResp.Msg),
		}, fmt.Errorf("飞书返回错误码 %d: %s", feishuResp.Code, feishuResp.Msg)
	}

	c.l.Infof("[飞书] 发送成功 | UUID: %s | 名称: %s", c.baseClient.GetUUID(), c.baseClient.GetName())
	return &SendResult{
		Success:   true,
		Message:   "发送成功",
		Timestamp: time.Now(),
	}, nil
}

// generateSign 生成签名
func (c *feiShuClient) generateSign(timestamp int64, secret string) string {
	stringToSign := fmt.Sprintf("%d\n%s", timestamp, secret)
	h := hmac.New(sha256.New, []byte(stringToSign))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// TestConnection 测试连接
func (c *feiShuClient) TestConnection(ctx context.Context, toEmail []string) (*TestResult, error) {
	startTime := time.Now()

	testOpts := &NotificationOptions{
		PortalName: "测试平台",
		PortalUrl:  "https://example.com",
		Title:      "飞书连接测试",
		Content:    "这是一条测试消息，用于验证飞书告警配置是否正确。",
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
		Message:   "飞书连接测试成功",
		Latency:   latency,
		Timestamp: time.Now(),
		Details: map[string]interface{}{
			"webhook":   c.config.Webhook,
			"hasSecret": c.config.Secret != "",
		},
	}, nil
}

// HealthCheck 健康检查
func (c *feiShuClient) HealthCheck(ctx context.Context) (*HealthStatus, error) {
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
		Message:       "飞书客户端健康",
		LastCheckTime: time.Now(),
	}, nil
}
