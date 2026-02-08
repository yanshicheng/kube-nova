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

	"github.com/yanshicheng/kube-nova/common/handler/errorx"
)

// dingTalkClient 钉钉告警客户端
// 实现通过钉钉机器人 Webhook 发送告警消息
type dingTalkClient struct {
	*baseClient
	config     *DingTalkConfig
	httpClient *http.Client // 复用的 HTTP 客户端，避免每次请求创建新连接
}

// dingTalkMessage 钉钉消息结构
// 支持 text 和 markdown 两种消息类型
type dingTalkMessage struct {
	MsgType  string            `json:"msgtype"`            // 消息类型: text 或 markdown
	Text     *dingTalkText     `json:"text,omitempty"`     // 文本消息内容
	Markdown *dingTalkMarkdown `json:"markdown,omitempty"` // Markdown 消息内容
	At       *dingTalkAt       `json:"at,omitempty"`       // @ 人配置
}

// dingTalkText 文本消息内容
type dingTalkText struct {
	Content string `json:"content"` // 消息内容
}

// dingTalkMarkdown Markdown 消息内容
type dingTalkMarkdown struct {
	Title string `json:"title"` // 首屏会话透出的展示内容
	Text  string `json:"text"`  // Markdown 格式的消息内容
}

// dingTalkAt @ 人配置
type dingTalkAt struct {
	AtMobiles []string `json:"atMobiles,omitempty"` // 被 @ 人的手机号列表
	AtUserIds []string `json:"atUserIds,omitempty"` // 被 @ 人的用户 ID 列表
	IsAtAll   bool     `json:"isAtAll,omitempty"`   // 是否 @ 所有人
}

// dingTalkResponse 钉钉 API 响应结构
type dingTalkResponse struct {
	ErrCode int    `json:"errcode"` // 错误码，0 表示成功
	ErrMsg  string `json:"errmsg"`  // 错误信息
}

// NewDingTalkClient 创建钉钉客户端
// config 必须包含有效的 DingTalk 配置
func NewDingTalkClient(config Config) (FullChannel, error) {
	if config.DingTalk == nil {
		return nil, errorx.Msg("钉钉配置不能为空")
	}

	bc := newBaseClient(config)

	// 创建可复用的 HTTP 客户端
	httpClient := &http.Client{
		Timeout: config.Options.Timeout,
		Transport: &http.Transport{
			MaxIdleConns:        100,              // 最大空闲连接数
			MaxIdleConnsPerHost: 10,               // 每个主机最大空闲连接数
			IdleConnTimeout:     90 * time.Second, // 空闲连接超时时间
		},
	}

	client := &dingTalkClient{
		baseClient: bc,
		config:     config.DingTalk,
		httpClient: httpClient,
	}

	client.l.Infof("创建钉钉告警客户端: %s", config.UUID)
	return client, nil
}

// WithContext 设置上下文
// 返回带有新上下文的客户端副本
func (c *dingTalkClient) WithContext(ctx context.Context) Client {
	// 创建新的 baseClient 副本
	newBaseClient := c.baseClient.WithContext(ctx).(*baseClient)

	// 创建新的 dingTalkClient 实例（共享 config 和 httpClient）
	return &dingTalkClient{
		baseClient: newBaseClient,
		config:     c.config,
		httpClient: c.httpClient,
	}
}

// SendPrometheusAlert 发送 Prometheus 告警
// opts 包含告警的基本信息，alerts 是具体的告警实例列表
func (c *dingTalkClient) SendPrometheusAlert(ctx context.Context, opts *AlertOptions, alerts []*AlertInstance) (*SendResult, error) {
	startTime := time.Now()

	// 检查客户端是否已关闭
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

	// 使用格式化器构建 Markdown 消息
	formatter := NewMessageFormatter(opts.PortalName, opts.PortalUrl)

	// 检测是否为聚合告警（同一项目的多条告警）
	// 如果 opts.Severity == "mixed"，说明是 manager 聚合后的告警
	var title, content string
	if opts.Severity == "mixed" || len(alerts) > 1 {
		// 聚合告警，使用聚合格式化
		aggregatedGroup := buildAggregatedAlertGroupFromAlerts(alerts, opts.ProjectName)
		title, content = formatter.FormatAggregatedAlertForDingTalk(aggregatedGroup)
	} else {
		// 单条告警，使用原有格式化
		title, content = formatter.FormatMarkdownForDingTalk(opts, alerts)
	}

	// 添加 @ 人配置
	// 根据钉钉官方文档，支持三种@人方式：
	// 1. atUserIds: 钉钉用户ID列表（推荐，最可靠）
	// 2. atMobiles: 手机号列表（降级方案）
	// 3. isAtAll: @所有人
	// 重要：钉钉@人需要在消息文本中也包含@标记，否则不会生效
	var atConfig *dingTalkAt
	if opts.Mentions != nil {
		// 优先使用钉钉用户ID
		if len(opts.Mentions.DingtalkUserIds) > 0 || opts.Mentions.IsAtAll {
			atConfig = &dingTalkAt{
				AtUserIds: opts.Mentions.DingtalkUserIds,
				IsAtAll:   opts.Mentions.IsAtAll,
			}
			c.l.Infof("[钉钉] @ 人配置 | 用户ID数: %d | @all: %v",
				len(opts.Mentions.DingtalkUserIds), opts.Mentions.IsAtAll)

			// 在消息文本末尾添加@标记（钉钉要求）
			if opts.Mentions.IsAtAll {
				content += "\n\n@all"
			} else if len(opts.Mentions.DingtalkUserIds) > 0 {
				// 添加@用户ID标记
				for _, userId := range opts.Mentions.DingtalkUserIds {
					content += fmt.Sprintf(" @%s", userId)
				}
			}
		} else if len(opts.Mentions.AtMobiles) > 0 {
			// 降级：如果没有钉钉用户ID，使用手机号
			atConfig = &dingTalkAt{
				AtMobiles: opts.Mentions.AtMobiles,
				IsAtAll:   opts.Mentions.IsAtAll,
			}
			c.l.Infof("[钉钉] @ 人配置(降级) | 手机号数: %d | @all: %v",
				len(opts.Mentions.AtMobiles), opts.Mentions.IsAtAll)

			// 在消息文本末尾添加@手机号标记
			for _, mobile := range opts.Mentions.AtMobiles {
				content += fmt.Sprintf(" @%s", mobile)
			}
		} else if opts.Mentions.IsAtAll {
			// 仅@所有人
			atConfig = &dingTalkAt{
				IsAtAll: true,
			}
			content += "\n\n@all"
			c.l.Infof("[钉钉] @ 所有人")
		} else {
			c.l.Infof("[钉钉] 未配置@人信息，消息将不会 @ 任何人")
		}
	}

	// 构建钉钉消息结构
	dingMsg := &dingTalkMessage{
		MsgType: "markdown",
		Markdown: &dingTalkMarkdown{
			Title: title,
			Text:  content,
		},
		At: atConfig,
	}

	// 构建重试上下文，用于日志记录
	// 注意: 这里使用 baseClient.GetUUID() 而不是 c.config.Webhook
	retryCtx := &RetryContext{
		ChannelType: string(AlertTypeDingTalk),
		ChannelUUID: c.baseClient.GetUUID(), // 修复: 使用正确的 UUID
		ChannelName: c.baseClient.GetName(),
		Operation:   "send_prometheus_alert",
		TargetInfo:  fmt.Sprintf("告警数量: %d, 项目: %s, 集群: %s", len(alerts), opts.ProjectName, opts.ClusterName),
	}

	// 发送消息（带重试机制）
	var result *SendResult
	err := c.retryWithBackoffContext(ctx, retryCtx, func() error {
		var sendErr error
		result, sendErr = c.sendToDingTalk(ctx, dingMsg)
		return sendErr
	})

	// 记录统计信息
	if err == nil && result != nil && result.Success {
		c.recordSuccess(time.Since(startTime))
		result.CostMs = time.Since(startTime).Milliseconds()
	} else {
		c.recordFailure()
	}

	return result, err
}

// SendNotification 发送通用通知
// 用于发送系统通知等非 Prometheus 告警消息
func (c *dingTalkClient) SendNotification(ctx context.Context, opts *NotificationOptions) (*SendResult, error) {
	startTime := time.Now()

	// 检查客户端是否已关闭
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
	title, content := formatter.FormatNotificationForDingTalk(opts)

	// 构建钉钉消息
	dingMsg := &dingTalkMessage{
		MsgType: "markdown",
		Markdown: &dingTalkMarkdown{
			Title: title,
			Text:  content,
		},
	}

	// 添加 @ 人配置
	if opts.Mentions != nil {
		// 优先使用钉钉用户ID（从用户表绑定的dingtalk_id）
		if len(opts.Mentions.DingtalkUserIds) > 0 || len(opts.Mentions.AtUserIds) > 0 || opts.Mentions.IsAtAll {
			// 合并钉钉用户ID和AtUserIds（兼容旧的配置方式）
			userIds := make([]string, 0, len(opts.Mentions.DingtalkUserIds)+len(opts.Mentions.AtUserIds))
			userIds = append(userIds, opts.Mentions.DingtalkUserIds...)
			userIds = append(userIds, opts.Mentions.AtUserIds...)

			if len(userIds) > 0 || opts.Mentions.IsAtAll {
				dingMsg.At = &dingTalkAt{
					AtUserIds: userIds,
					IsAtAll:   opts.Mentions.IsAtAll,
				}
			}
		}
		// 降级：如果没有钉钉用户ID，使用手机号
		if dingMsg.At == nil && len(opts.Mentions.AtMobiles) > 0 {
			dingMsg.At = &dingTalkAt{
				AtMobiles: opts.Mentions.AtMobiles,
				IsAtAll:   opts.Mentions.IsAtAll,
			}
		}
	}

	// 构建 @ 人信息用于日志
	atInfo := ""
	if opts.Mentions != nil {
		atInfo = fmt.Sprintf("@手机号: %v, @用户ID: %v", opts.Mentions.AtMobiles, opts.Mentions.AtUserIds)
	}

	// 构建重试上下文
	retryCtx := &RetryContext{
		ChannelType: string(AlertTypeDingTalk),
		ChannelUUID: c.baseClient.GetUUID(), // 修复: 使用正确的 UUID
		ChannelName: c.baseClient.GetName(),
		Operation:   "send_notification",
		TargetInfo:  fmt.Sprintf("标题: %s, %s", opts.Title, atInfo),
	}

	// 发送消息（带重试机制）
	var result *SendResult
	err := c.retryWithBackoffContext(ctx, retryCtx, func() error {
		var sendErr error
		result, sendErr = c.sendToDingTalk(ctx, dingMsg)
		return sendErr
	})

	// 记录统计信息
	if err == nil && result != nil && result.Success {
		c.recordSuccess(time.Since(startTime))
		result.CostMs = time.Since(startTime).Milliseconds()
	} else {
		c.recordFailure()
	}

	return result, err
}

// sendToDingTalk 发送消息到钉钉
// 处理签名、HTTP 请求和响应解析
func (c *dingTalkClient) sendToDingTalk(ctx context.Context, message *dingTalkMessage) (*SendResult, error) {
	// 构建 Webhook URL
	webhookURL := c.config.Webhook

	// 如果配置了密钥，添加签名参数
	if c.config.Secret != "" {
		timestamp := time.Now().UnixMilli()
		sign := c.generateSign(timestamp, c.config.Secret)
		webhookURL = fmt.Sprintf("%s&timestamp=%d&sign=%s", webhookURL, timestamp, url.QueryEscape(sign))
	}

	// 序列化消息为 JSON
	body, err := json.Marshal(message)
	if err != nil {
		return nil, errorx.Msg(fmt.Sprintf("序列化消息失败: %v", err))
	}

	// 创建 HTTP 请求
	req, err := http.NewRequestWithContext(ctx, "POST", webhookURL, bytes.NewBuffer(body))
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

	// 读取响应内容
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errorx.Msg(fmt.Sprintf("读取响应失败: %v", err))
	}

	// 解析响应 JSON
	var dingResp dingTalkResponse
	if err := json.Unmarshal(respBody, &dingResp); err != nil {
		return nil, errorx.Msg(fmt.Sprintf("解析响应失败: %v", err))
	}

	// 检查钉钉返回的错误码
	if dingResp.ErrCode != 0 {
		errMsg := fmt.Sprintf("钉钉返回错误码 %d: %s", dingResp.ErrCode, dingResp.ErrMsg)
		return &SendResult{
			Success:   false,
			Message:   dingResp.ErrMsg,
			Timestamp: time.Now(),
			Error:     errorx.Msg(errMsg),
		}, errorx.Msg(errMsg)
	}

	c.l.Infof("[钉钉] 发送成功 | UUID: %s | 名称: %s", c.baseClient.GetUUID(), c.baseClient.GetName())
	return &SendResult{
		Success:   true,
		Message:   "发送成功",
		Timestamp: time.Now(),
	}, nil
}

// generateSign 生成钉钉签名
// 使用 HMAC-SHA256 算法对时间戳和密钥进行签名
func (c *dingTalkClient) generateSign(timestamp int64, secret string) string {
	// 构建待签名字符串: 时间戳 + 换行符 + 密钥
	stringToSign := fmt.Sprintf("%d\n%s", timestamp, secret)
	// 使用密钥对字符串进行 HMAC-SHA256 签名
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(stringToSign))
	// 返回 Base64 编码的签名
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// TestConnection 测试连接
// 发送一条测试消息来验证配置是否正确
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
				"webhook":   maskSensitiveURL(c.config.Webhook), // 脱敏处理
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
			"webhook":   maskSensitiveURL(c.config.Webhook), // 脱敏处理
			"hasSecret": c.config.Secret != "",
		},
	}, nil
}

// HealthCheck 健康检查
// 检查客户端状态和配置是否有效
func (c *dingTalkClient) HealthCheck(ctx context.Context) (*HealthStatus, error) {
	// 检查客户端是否已关闭
	if atomic.LoadInt32(&c.closed) == 1 {
		return &HealthStatus{
			Healthy:       false,
			Message:       "客户端已关闭",
			LastCheckTime: time.Now(),
		}, errorx.Msg("客户端已关闭")
	}

	// 检查 Webhook 地址是否配置
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
		Message:       "钉钉客户端健康",
		LastCheckTime: time.Now(),
	}, nil
}

// Close 关闭客户端
// 关闭 HTTP 客户端的空闲连接
func (c *dingTalkClient) Close() error {
	// 关闭 HTTP 客户端的空闲连接
	if c.httpClient != nil {
		c.httpClient.CloseIdleConnections()
	}
	return c.baseClient.Close()
}

// maskSensitiveURL 对敏感 URL 进行脱敏处理
// 用于日志记录，避免泄露完整的 Webhook 地址
func maskSensitiveURL(url string) string {
	if len(url) <= 30 {
		return "***"
	}
	// 只显示前 20 个字符和最后 5 个字符
	return url[:20] + "..." + url[len(url)-5:]
}
