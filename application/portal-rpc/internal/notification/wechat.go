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

	"github.com/yanshicheng/kube-nova/common/handler/errorx"
)

// weChatClient 企业微信告警客户端
// 实现通过企业微信群机器人 Webhook 发送告警消息
type weChatClient struct {
	*baseClient
	config     *WeChatConfig
	httpClient *http.Client // 复用的 HTTP 客户端
}

// weChatMessage 企业微信消息结构
// 支持 text 和 markdown 两种消息类型
type weChatMessage struct {
	MsgType  string          `json:"msgtype"`            // 消息类型
	Text     *weChatText     `json:"text,omitempty"`     // 文本消息
	Markdown *weChatMarkdown `json:"markdown,omitempty"` // Markdown 消息
}

// weChatText 文本消息内容
// 文本消息支持 @ 手机号和 @ 成员功能
type weChatText struct {
	Content             string   `json:"content"`                         // 消息内容，最长 2048 字节
	MentionedList       []string `json:"mentioned_list,omitempty"`        // userid 列表，@ 指定成员
	MentionedMobileList []string `json:"mentioned_mobile_list,omitempty"` // 手机号列表，@ 指定手机号
}

// weChatMarkdown Markdown 消息内容
// 注意: Markdown 消息不支持 mentioned_list 和 mentioned_mobile_list
// 需要在 content 中使用 <@userid> 语法来 @ 人
type weChatMarkdown struct {
	Content string `json:"content"` // Markdown 内容，最长 4096 字节
}

// weChatResponse 企业微信 API 响应结构
type weChatResponse struct {
	ErrCode int    `json:"errcode"` // 错误码，0 表示成功
	ErrMsg  string `json:"errmsg"`  // 错误信息
}

// NewWeChatClient 创建企业微信客户端
func NewWeChatClient(config Config) (FullChannel, error) {
	if config.WeChat == nil {
		return nil, errorx.Msg("企业微信配置不能为空")
	}

	bc := newBaseClient(config)

	// 创建可复用的 HTTP 客户端
	httpClient := &http.Client{
		Timeout: config.Options.Timeout,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
		},
	}

	client := &weChatClient{
		baseClient: bc,
		config:     config.WeChat,
		httpClient: httpClient,
	}

	client.l.Infof("创建企业微信告警客户端: %s", config.UUID)
	return client, nil
}

// WithContext 设置上下文
// 返回带有新上下文的客户端副本
func (c *weChatClient) WithContext(ctx context.Context) Client {
	// 创建新的 baseClient 副本
	newBaseClient := c.baseClient.WithContext(ctx).(*baseClient)

	// 创建新的 weChatClient 实例（共享 config 和 httpClient）
	return &weChatClient{
		baseClient: newBaseClient,
		config:     c.config,
		httpClient: c.httpClient,
	}
}

// SendPrometheusAlert 发送 Prometheus 告警
func (c *weChatClient) SendPrometheusAlert(ctx context.Context, opts *AlertOptions, alerts []*AlertInstance) (*SendResult, error) {
	startTime := time.Now()

	// 检查客户端状态
	if atomic.LoadInt32(&c.closed) == 1 {
		return nil, errorx.Msg("客户端已关闭")
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

	// 检测是否为聚合告警（同一项目的多条告警）
	var content string
	if opts.Severity == "mixed" || len(alerts) > 1 {
		// 聚合告警，使用聚合格式化
		aggregatedGroup := buildAggregatedAlertGroupFromAlerts(alerts, opts.ProjectName)
		content = formatter.FormatAggregatedAlertForWeChat(aggregatedGroup)
	} else {
		// 单条告警，使用原有格式化
		content = formatter.FormatMarkdownForWeChat(opts, alerts)
	}

	// 添加 @ 人配置
	// 注意: 企业微信 Markdown 消息需要在 content 中使用 <@userid> 语法
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
		return nil, errorx.Msg("客户端已关闭")
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

	// 添加 @ 人配置
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

	// 构建日志信息
	atInfo := ""
	if opts.Mentions != nil {
		atInfo = fmt.Sprintf("@手机号: %v", opts.Mentions.AtMobiles)
	}

	// 构建重试上下文
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

// appendMentions 添加 @ 人信息到内容中
// 根据企业微信 API 文档，Markdown 消息需要在 content 中使用 <@userid> 扩展语法
// 注意:
// 1. <@userid> 中的 userid 必须是企业微信的成员 ID
// 2. 如果用户没有绑定企业微信ID，则不添加 @ 人信息
func (c *weChatClient) appendMentions(content string, mentions *MentionConfig) string {
	if mentions == nil {
		return content
	}

	var mentionParts []string

	// @ 所有人
	if mentions.IsAtAll {
		mentionParts = append(mentionParts, "@all")
	} else {
		// 优先使用企业微信用户ID（从用户表绑定的wechat_id）
		if len(mentions.WechatUserIds) > 0 {
			for _, userId := range mentions.WechatUserIds {
				if userId != "" {
					mentionParts = append(mentionParts, fmt.Sprintf("<@%s>", userId))
				}
			}
		}

		// 降级：如果没有企业微信用户ID，不使用手机号（Markdown消息不支持）
		// 注意：企业微信 Markdown 消息不支持通过手机号 @ 人
		// 如果需要手机号@人功能，需要改用 text 类型消息
	}

	// 如果有 @ 人信息，在 content 末尾添加
	if len(mentionParts) > 0 {
		content = content + "\n\n" + strings.Join(mentionParts, " ")
	}

	return content
}

// sendToWeChat 发送消息到企业微信
func (c *weChatClient) sendToWeChat(ctx context.Context, message *weChatMessage) (*SendResult, error) {
	// 序列化消息
	body, err := json.Marshal(message)
	if err != nil {
		return nil, errorx.Msg(fmt.Sprintf("序列化消息失败: %v", err))
	}

	// 创建 HTTP 请求
	req, err := http.NewRequestWithContext(ctx, "POST", c.config.Webhook, bytes.NewBuffer(body))
	if err != nil {
		return nil, errorx.Msg(fmt.Sprintf("创建请求失败: %v", err))
	}

	req.Header.Set("Content-Type", "application/json")

	// 使用复用的 HTTP 客户端发送请求
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, errorx.Msg(fmt.Sprintf("发送请求失败: %v", err))
	}
	defer resp.Body.Close()

	// 读取响应
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errorx.Msg(fmt.Sprintf("读取响应失败: %v", err))
	}

	// 解析响应
	var wechatResp weChatResponse
	if err := json.Unmarshal(respBody, &wechatResp); err != nil {
		return nil, errorx.Msg(fmt.Sprintf("解析响应失败: %v", err))
	}

	// 检查响应结果
	if wechatResp.ErrCode != 0 {
		errMsg := fmt.Sprintf("企业微信返回错误码 %d: %s", wechatResp.ErrCode, wechatResp.ErrMsg)
		return &SendResult{
			Success:   false,
			Message:   wechatResp.ErrMsg,
			Timestamp: time.Now(),
			Error:     errorx.Msg(errMsg),
		}, errorx.Msg(errMsg)
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
				"webhook": maskSensitiveURL(c.config.Webhook),
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
			"webhook": maskSensitiveURL(c.config.Webhook),
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
		}, errorx.Msg("客户端已关闭")
	}

	// 检查 Webhook 地址格式
	if c.config.Webhook == "" {
		c.updateHealthStatus(false, errorx.Msg("Webhook 地址为空"))
		return &HealthStatus{
			Healthy:       false,
			Message:       "Webhook 地址为空",
			LastCheckTime: time.Now(),
		}, errorx.Msg("Webhook 地址为空")
	}

	c.updateHealthStatus(true, nil)
	return &HealthStatus{
		Healthy:       true,
		Message:       "企业微信客户端健康",
		LastCheckTime: time.Now(),
	}, nil
}

// Close 关闭客户端
func (c *weChatClient) Close() error {
	if c.httpClient != nil {
		c.httpClient.CloseIdleConnections()
	}
	return c.baseClient.Close()
}
