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

	"github.com/yanshicheng/kube-nova/common/handler/errorx"
)

// feiShuClient 飞书告警客户端
// 实现通过飞书机器人 Webhook 发送告警消息
type feiShuClient struct {
	*baseClient
	config     *FeiShuConfig
	httpClient *http.Client // 复用的 HTTP 客户端
}

// feiShuMessage 飞书消息结构
type feiShuMessage struct {
	Timestamp string         `json:"timestamp,omitempty"` // 时间戳（秒），用于签名验证
	Sign      string         `json:"sign,omitempty"`      // 签名
	MsgType   string         `json:"msg_type"`            // 消息类型
	Content   *feiShuContent `json:"content"`             // 消息内容
}

// feiShuContent 飞书消息内容
type feiShuContent struct {
	Text string      `json:"text,omitempty"` // 文本内容
	Post *feiShuPost `json:"post,omitempty"` // 富文本内容
}

// feiShuPost 飞书富文本消息
type feiShuPost struct {
	ZhCn *feiShuPostContent `json:"zh_cn"` // 中文内容
}

// feiShuPostContent 飞书富文本内容
type feiShuPostContent struct {
	Title   string                     `json:"title"`   // 标题
	Content [][]map[string]interface{} `json:"content"` // 内容，二维数组表示段落和行内元素
}

// feiShuResponse 飞书 API 响应结构
type feiShuResponse struct {
	Code int    `json:"code"` // 错误码，0 表示成功
	Msg  string `json:"msg"`  // 错误信息
}

// NewFeiShuClient 创建飞书客户端
func NewFeiShuClient(config Config) (FullChannel, error) {
	if config.FeiShu == nil {
		return nil, errorx.Msg("飞书配置不能为空")
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

	client := &feiShuClient{
		baseClient: bc,
		config:     config.FeiShu,
		httpClient: httpClient,
	}

	client.l.Infof("创建飞书告警客户端: %s", config.UUID)
	return client, nil
}

// WithContext 设置上下文
// 返回带有新上下文的客户端副本
func (c *feiShuClient) WithContext(ctx context.Context) Client {
	// 创建新的 baseClient 副本
	newBaseClient := c.baseClient.WithContext(ctx).(*baseClient)

	// 创建新的 feiShuClient 实例（共享 config 和 httpClient）
	return &feiShuClient{
		baseClient: newBaseClient,
		config:     c.config,
		httpClient: c.httpClient,
	}
}

// SendPrometheusAlert 发送 Prometheus 告警
func (c *feiShuClient) SendPrometheusAlert(ctx context.Context, opts *AlertOptions, alerts []*AlertInstance) (*SendResult, error) {
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
	var title string
	var content [][]map[string]any
	if opts.Severity == "mixed" || len(alerts) > 1 {
		// 聚合告警，使用聚合格式化
		aggregatedGroup := buildAggregatedAlertGroupFromAlerts(alerts, opts.ProjectName)
		title, content = formatter.FormatAggregatedAlertForFeiShu(aggregatedGroup)
	} else {
		// 单条告警，使用原有格式化
		title, content = formatter.FormatRichTextForFeiShu(opts, alerts)
	}

	// 添加 @ 人配置
	// 注意: 飞书不支持 @ 手机号，只支持 @ 用户 ID 或 @all
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

	// 如果配置了密钥，添加签名
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
	title, content := formatter.FormatNotificationForFeiShu(opts)

	// 添加 @ 人配置
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

	// 如果配置了密钥，添加签名
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

// appendMentions 添加 @ 人信息到内容中
// 飞书的 @ 功能说明:
// 1. 飞书不支持 @ 手机号，只支持 @ 用户 ID (open_id/user_id) 或 @all
// 2. 必须使用飞书平台的真实用户ID，不能使用内部系统用户ID
// 3. 使用 {"tag": "at", "user_id": "xxx"} 格式
// 修复: 删除错误的降级逻辑，只使用飞书用户ID
func (c *feiShuClient) appendMentions(content [][]map[string]interface{}, mentions *MentionConfig) [][]map[string]interface{} {
	if mentions == nil {
		return content
	}

	// 情况 1: 配置了 @all
	if mentions.IsAtAll {
		content = append(content, []map[string]interface{}{
			{"tag": "at", "user_id": "all"},
		})
		c.l.Infof("[飞书] @ 所有人")
		return content
	}

	// 只使用飞书用户ID（必须是飞书平台的真实用户ID）
	if len(mentions.FeishuUserIds) > 0 {
		atLine := []map[string]interface{}{}
		for _, userId := range mentions.FeishuUserIds {
			if userId != "" {
				atLine = append(atLine, map[string]interface{}{
					"tag":     "at",
					"user_id": userId,
				})
			}
		}
		if len(atLine) > 0 {
			content = append(content, atLine)
			c.l.Infof("[飞书] @ 人配置 | 用户数: %d", len(atLine))
		}
	} else {
		c.l.Infof("[飞书] 未配置飞书用户ID，消息将不会 @ 任何人")
	}

	return content
}

// sendToFeiShu 发送消息到飞书
func (c *feiShuClient) sendToFeiShu(ctx context.Context, message *feiShuMessage) (*SendResult, error) {
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
	var feishuResp feiShuResponse
	if err := json.Unmarshal(respBody, &feishuResp); err != nil {
		return nil, errorx.Msg(fmt.Sprintf("解析响应失败: %v", err))
	}

	// 检查响应结果
	if feishuResp.Code != 0 {
		errMsg := fmt.Sprintf("飞书返回错误码 %d: %s", feishuResp.Code, feishuResp.Msg)
		return &SendResult{
			Success:   false,
			Message:   feishuResp.Msg,
			Timestamp: time.Now(),
			Error:     errorx.Msg(errMsg),
		}, errorx.Msg(errMsg)
	}

	c.l.Infof("[飞书] 发送成功 | UUID: %s | 名称: %s", c.baseClient.GetUUID(), c.baseClient.GetName())
	return &SendResult{
		Success:   true,
		Message:   "发送成功",
		Timestamp: time.Now(),
	}, nil
}

// generateSign 生成飞书签名
// 飞书签名算法（官方文档）:
// 1. 拼接 timestamp + "\n" + secret 得到 stringToSign
// 2. 使用 HmacSHA256 算法计算签名，密钥为 stringToSign
// 3. 对签名结果进行 Base64 编码
//
// 参考文档: https://open.feishu.cn/document/ukTMukTMukTM/ucTM5YjL3ETO24yNxkjN
func (c *feiShuClient) generateSign(timestamp int64, secret string) string {
	// 步骤1: 拼接 timestamp + "\n" + secret
	stringToSign := fmt.Sprintf("%d\n%s", timestamp, secret)

	// 步骤2: 使用 HmacSHA256 计算签名，密钥为 stringToSign 本身
	h := hmac.New(sha256.New, []byte(stringToSign))
	// 不需要写入数据，直接计算空消息的 HMAC

	// 步骤3: Base64 编码
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
				"webhook":   maskSensitiveURL(c.config.Webhook),
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
			"webhook":   maskSensitiveURL(c.config.Webhook),
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
		Message:       "飞书客户端健康",
		LastCheckTime: time.Now(),
	}, nil
}

// Close 关闭客户端
func (c *feiShuClient) Close() error {
	if c.httpClient != nil {
		c.httpClient.CloseIdleConnections()
	}
	return c.baseClient.Close()
}
