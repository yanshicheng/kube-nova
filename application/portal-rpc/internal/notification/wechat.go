package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"time"
)

// weChatClient 企业微信告警客户端
type weChatClient struct {
	*baseClient
	config *WeChatConfig
}

// weChatMessage 企业微信消息结构
type weChatMessage struct {
	MsgType  string          `json:"msgtype"`
	Text     *weChatText     `json:"text,omitempty"`
	Markdown *weChatMarkdown `json:"markdown,omitempty"`
}

type weChatText struct {
	Content             string   `json:"content"`
	MentionedList       []string `json:"mentioned_list,omitempty"`
	MentionedMobileList []string `json:"mentioned_mobile_list,omitempty"`
}

type weChatMarkdown struct {
	Content string `json:"content"`
}

type weChatResponse struct {
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
}

// NewWeChatClient 创建企业微信客户端
func NewWeChatClient(config Config) (FullChannel, error) {
	if config.WeChat == nil {
		return nil, fmt.Errorf("企业微信配置不能为空")
	}

	bc := newBaseClient(config)
	client := &weChatClient{
		baseClient: bc,
		config:     config.WeChat,
	}

	client.l.Infof("创建企业微信告警客户端: %s", config.UUID)
	return client, nil
}

// WithContext 设置上下文
func (c *weChatClient) WithContext(ctx context.Context) Client {
	c.baseClient.ctx = ctx
	return c
}

// SendPrometheusAlert 发送 Prometheus 告警
func (c *weChatClient) SendPrometheusAlert(ctx context.Context, opts *AlertOptions, alerts []*AlertInstance) (*SendResult, error) {
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
	content := formatter.FormatMarkdownForWeChat(opts, alerts)

	// 添加 @人配置（企业微信的 @ 需要在 content 中添加 <@userid> 或 @all）
	if opts.Mentions != nil {
		content = c.appendMentions(content, opts.Mentions)
	}

	// 构建企业微信消息
	wechatMsg := &weChatMessage{
		MsgType: "markdown",
		Markdown: &weChatMarkdown{
			Content: content,
		},
	}

	// 构建重试上下文
	retryCtx := &RetryContext{
		ChannelType: string(AlertTypeWeChat),
		ChannelUUID: c.baseClient.GetUUID(),
		ChannelName: c.baseClient.GetName(),
		Operation:   "send_prometheus_alert",
		TargetInfo:  fmt.Sprintf("告警数量: %d, 项目: %s, 集群: %s", len(alerts), opts.ProjectName, opts.ClusterName),
	}

	// 发送消息（带重试）
	var result *SendResult
	err := c.retryWithBackoffContext(ctx, retryCtx, func() error {
		var sendErr error
		result, sendErr = c.sendToWeChat(ctx, wechatMsg)
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
func (c *weChatClient) SendNotification(ctx context.Context, opts *NotificationOptions) (*SendResult, error) {
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
	content := formatter.FormatNotificationForWeChat(opts)

	// 添加 @人配置
	if opts.Mentions != nil {
		content = c.appendMentions(content, opts.Mentions)
	}

	// 构建企业微信消息
	wechatMsg := &weChatMessage{
		MsgType: "markdown",
		Markdown: &weChatMarkdown{
			Content: content,
		},
	}

	// 构建重试上下文
	atInfo := ""
	if opts.Mentions != nil {
		atInfo = fmt.Sprintf("@手机号: %v", opts.Mentions.AtMobiles)
	}
	retryCtx := &RetryContext{
		ChannelType: string(AlertTypeWeChat),
		ChannelUUID: c.baseClient.GetUUID(),
		ChannelName: c.baseClient.GetName(),
		Operation:   "send_notification",
		TargetInfo:  fmt.Sprintf("标题: %s, %s", opts.Title, atInfo),
	}

	// 发送消息（带重试）
	var result *SendResult
	err := c.retryWithBackoffContext(ctx, retryCtx, func() error {
		var sendErr error
		result, sendErr = c.sendToWeChat(ctx, wechatMsg)
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

// appendMentions 添加 @人信息到内容中
// 根据企业微信API文档，需要在markdown的content中使用 <@userid> 扩展语法
func (c *weChatClient) appendMentions(content string, mentions *MentionConfig) string {
	if mentions == nil {
		return content
	}

	var mentionParts []string

	// @所有人
	if mentions.IsAtAll {
		mentionParts = append(mentionParts, "@all")
	} else {
		// @指定的userid（企业微信成员ID）
		for _, userId := range mentions.AtUserIds {
			mentionParts = append(mentionParts, fmt.Sprintf("<@%s>", userId))
		}

		// @指定的手机号
		for _, mobile := range mentions.AtMobiles {
			mentionParts = append(mentionParts, fmt.Sprintf("<@%s>", mobile))
		}
	}

	// 如果有@人信息，在content末尾添加
	if len(mentionParts) > 0 {
		content = content + "\n\n" + strings.Join(mentionParts, " ")
	}

	return content
}

// sendToWeChat 发送到企业微信
func (c *weChatClient) sendToWeChat(ctx context.Context, message *weChatMessage) (*SendResult, error) {
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
	var wechatResp weChatResponse
	if err := json.Unmarshal(respBody, &wechatResp); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	// 检查响应结果
	if wechatResp.ErrCode != 0 {
		return &SendResult{
			Success:   false,
			Message:   wechatResp.ErrMsg,
			Timestamp: time.Now(),
			Error:     fmt.Errorf("企业微信返回错误码 %d: %s", wechatResp.ErrCode, wechatResp.ErrMsg),
		}, fmt.Errorf("企业微信返回错误码 %d: %s", wechatResp.ErrCode, wechatResp.ErrMsg)
	}

	c.l.Infof("[企业微信] 发送成功 | UUID: %s | 名称: %s", c.baseClient.GetUUID(), c.baseClient.GetName())
	return &SendResult{
		Success:   true,
		Message:   "发送成功",
		Timestamp: time.Now(),
	}, nil
}

// TestConnection 测试连接
func (c *weChatClient) TestConnection(ctx context.Context, toEmail []string) (*TestResult, error) {
	startTime := time.Now()

	testOpts := &NotificationOptions{
		PortalName: "测试平台",
		PortalUrl:  "https://example.com",
		Title:      "企业微信连接测试",
		Content:    "这是一条测试消息，用于验证企业微信告警配置是否正确。",
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
				"webhook": c.config.Webhook,
			},
		}, err
	}

	c.updateHealthStatus(true, nil)
	return &TestResult{
		Success:   result.Success,
		Message:   "企业微信连接测试成功",
		Latency:   latency,
		Timestamp: time.Now(),
		Details: map[string]interface{}{
			"webhook": c.config.Webhook,
		},
	}, nil
}

// HealthCheck 健康检查
func (c *weChatClient) HealthCheck(ctx context.Context) (*HealthStatus, error) {
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
		Message:       "企业微信客户端健康",
		LastCheckTime: time.Now(),
	}, nil
}
