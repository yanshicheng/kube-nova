package notification

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
)

// SiteMessageStore 站内信存储接口（需要外部实现）
type SiteMessageStore interface {
	// SaveMessage 保存站内信
	SaveMessage(ctx context.Context, msg *SiteMessageData) error
	// SaveBatchMessages 批量保存站内信
	SaveBatchMessages(ctx context.Context, msgs []*SiteMessageData) error
}

// MessagePushCallback 消息推送回调接口（新增）
type MessagePushCallback interface {
	// OnMessageCreated 当消息创建后回调（用于推送到 WebSocket）
	OnMessageCreated(ctx context.Context, msg *SiteMessageData) error
	// OnBatchMessagesCreated 批量消息创建后回调
	OnBatchMessagesCreated(ctx context.Context, msgs []*SiteMessageData) error
}

// siteMessageClient 站内信客户端
type siteMessageClient struct {
	*baseClient
	store        SiteMessageStore
	pushCallback MessagePushCallback // 新增：推送回调
}

// NewSiteMessageClient 创建站内信客户端
func NewSiteMessageClient(config Config, store SiteMessageStore) (FullChannel, error) {
	if store == nil {
		return nil, fmt.Errorf("站内信存储接口不能为空")
	}

	bc := newBaseClient(config)
	client := &siteMessageClient{
		baseClient: bc,
		store:      store,
	}

	client.l.Infof("创建站内信客户端: %s", config.UUID)
	return client, nil
}

// SetPushCallback 设置推送回调（新增）
func (c *siteMessageClient) SetPushCallback(callback MessagePushCallback) {
	c.pushCallback = callback
}

// WithContext 设置上下文
func (c *siteMessageClient) WithContext(ctx context.Context) Client {
	c.baseClient.ctx = ctx
	return c
}

// SendPrometheusAlert 发送 Prometheus 告警站内信
func (c *siteMessageClient) SendPrometheusAlert(ctx context.Context, opts *AlertOptions, alerts []*AlertInstance) (*SendResult, error) {
	startTime := time.Now()

	// 检查客户端状态
	if atomic.LoadInt32(&c.closed) == 1 {
		return nil, fmt.Errorf("客户端已关闭")
	}

	// 获取用户ID列表
	if opts.Mentions == nil {
		return &SendResult{
			Success:   false,
			Message:   "站内信需要指定用户ID",
			Timestamp: time.Now(),
			Error:     fmt.Errorf("未指定用户ID"),
		}, fmt.Errorf("未指定用户ID")
	}

	// 使用格式化器构建消息
	formatter := NewMessageFormatter(opts.PortalName, opts.PortalUrl)

	var msgs []*SiteMessageData
	expireAt := time.Now().AddDate(0, 0, 30)

	// 为每个用户的每个告警创建独立的站内消息
	for _, userIdStr := range opts.Mentions.AtUserIds {
		var userId uint64
		_, err := fmt.Sscanf(userIdStr, "%d", &userId)
		if err != nil {
			continue
		}

		// 为每个告警实例创建一条消息
		for _, alert := range alerts {
			// 构建单个告警的标题和内容
			title := fmt.Sprintf("%s %s - %s",
				formatter.GetSeverityEmoji(alert.Severity),
				alert.AlertName,
				opts.ClusterName)

			content := c.buildSingleAlertContent(formatter, alert, opts)

			msg := &SiteMessageData{
				UUID:           uuid.New().String(),
				UserID:         userId,
				InstanceID:     alert.ID,
				NotificationID: 0,
				Title:          title,
				Content:        content,
				MessageType:    "alert",
				Severity:       alert.Severity,
				Category:       "prometheus",
				ActionURL:      alert.GeneratorURL,
				ActionText:     "查看详情",
				IsRead:         0,
				IsStarred:      0,
				ExpireAt:       expireAt,
			}
			msgs = append(msgs, msg)
		}
	}

	if len(msgs) == 0 {
		return &SendResult{
			Success:   false,
			Message:   "没有有效的用户ID或告警",
			Timestamp: time.Now(),
			Error:     fmt.Errorf("没有有效的用户ID或告警"),
		}, fmt.Errorf("没有有效的用户ID或告警")
	}

	// 批量保存
	if err := c.store.SaveBatchMessages(ctx, msgs); err != nil {
		c.recordFailure()
		c.l.Errorf("[站内信] 批量保存失败 | UUID: %s | 消息数: %d | 错误: %v",
			c.baseClient.GetUUID(), len(msgs), err)
		return &SendResult{
			Success:   false,
			Message:   err.Error(),
			Timestamp: time.Now(),
			Error:     err,
		}, err
	}

	if c.pushCallback != nil {
		// 异步推送，不阻塞主流程
		go func() {
			pushCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			if err := c.pushCallback.OnBatchMessagesCreated(pushCtx, msgs); err != nil {
				c.l.Errorf("[站内信] Redis 推送失败 | UUID: %s | 错误: %v",
					c.baseClient.GetUUID(), err)
			}
		}()
	}

	c.recordSuccess(time.Since(startTime))
	c.l.Infof("[站内信] 发送成功 | UUID: %s | 消息数: %d | 用户数: %d | 告警数: %d | 耗时: %dms",
		c.baseClient.GetUUID(), len(msgs), len(opts.Mentions.AtUserIds), len(alerts), time.Since(startTime).Milliseconds())

	return &SendResult{
		Success: true,
		Message: fmt.Sprintf("站内信发送成功，用户数: %d，告警数: %d，总消息数: %d",
			len(opts.Mentions.AtUserIds), len(alerts), len(msgs)),
		Timestamp: time.Now(),
		CostMs:    time.Since(startTime).Milliseconds(),
	}, nil
}

// buildSingleAlertContent 构建单个告警的内容
func (c *siteMessageClient) buildSingleAlertContent(formatter *MessageFormatter, alert *AlertInstance, opts *AlertOptions) string {
	projectDisplay := opts.ProjectName
	if projectDisplay == "" {
		projectDisplay = "集群级"
	}

	content := fmt.Sprintf(`## 告警信息

- **项目**: %s
- **集群**: %s
- **告警规则**: %s
- **实例**: %s
- **级别**: %s
- **状态**: %s
- **持续时间**: %s

`, projectDisplay, opts.ClusterName, alert.AlertName, alert.Instance,
		alert.Severity, alert.Status, formatter.FormatDuration(alert.Duration))

	// 添加描述
	description := formatter.GetAlertDescription(alert)
	if description != "" {
		content += fmt.Sprintf("## 告警描述\n\n%s\n\n", description)
	}

	// 添加标签信息
	if len(alert.Labels) > 0 {
		content += "## 标签\n\n"
		for k, v := range alert.Labels {
			content += fmt.Sprintf("- **%s**: `%s`\n", k, v)
		}
		content += "\n"
	}

	// 添加注解信息
	if len(alert.Annotations) > 0 {
		content += "## 注解\n\n"
		for k, v := range alert.Annotations {
			content += fmt.Sprintf("- **%s**: %s\n", k, v)
		}
	}

	return content
}

// SendNotification 发送通用通知站内信
func (c *siteMessageClient) SendNotification(ctx context.Context, opts *NotificationOptions) (*SendResult, error) {
	startTime := time.Now()

	// 检查客户端状态
	if atomic.LoadInt32(&c.closed) == 1 {
		return nil, fmt.Errorf("客户端已关闭")
	}

	// 获取用户ID列表
	if opts.Mentions == nil {
		return &SendResult{
			Success:   false,
			Message:   "站内信需要指定用户ID",
			Timestamp: time.Now(),
			Error:     fmt.Errorf("未指定用户ID"),
		}, fmt.Errorf("未指定用户ID")
	}

	// 为每个用户创建站内信
	var msgs []*SiteMessageData
	expireAt := time.Now().AddDate(0, 0, 30)

	for _, userIdStr := range opts.Mentions.AtUserIds {
		var userId uint64
		_, err := fmt.Sscanf(userIdStr, "%d", &userId)
		if err != nil {
			continue
		}

		msg := &SiteMessageData{
			UUID:        uuid.New().String(),
			UserID:      userId,
			Title:       opts.Title,
			Content:     opts.Content,
			MessageType: "notification",
			Severity:    "info",
			Category:    "system",
			ActionURL:   opts.PortalUrl,
			ActionText:  "查看详情",
			IsRead:      0,
			IsStarred:   0,
			ExpireAt:    expireAt,
		}
		msgs = append(msgs, msg)
	}

	if len(msgs) == 0 {
		return &SendResult{
			Success:   false,
			Message:   "没有有效的用户ID",
			Timestamp: time.Now(),
			Error:     fmt.Errorf("没有有效的用户ID"),
		}, fmt.Errorf("没有有效的用户ID")
	}

	// 批量保存
	if err := c.store.SaveBatchMessages(ctx, msgs); err != nil {
		c.recordFailure()
		c.l.Errorf("[站内信] 批量保存失败 | UUID: %s | 用户数: %d | 错误: %v",
			c.baseClient.GetUUID(), len(msgs), err)
		return &SendResult{
			Success:   false,
			Message:   err.Error(),
			Timestamp: time.Now(),
			Error:     err,
		}, err
	}

	if c.pushCallback != nil {
		// 异步推送，不阻塞主流程
		go func() {
			pushCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			if err := c.pushCallback.OnBatchMessagesCreated(pushCtx, msgs); err != nil {
				c.l.Errorf("[站内信] Redis 推送失败 | UUID: %s | 错误: %v",
					c.baseClient.GetUUID(), err)
			}
		}()
	}

	c.recordSuccess(time.Since(startTime))
	c.l.Infof("[站内信] 发送成功 | UUID: %s | 用户数: %d | 耗时: %dms",
		c.baseClient.GetUUID(), len(msgs), time.Since(startTime).Milliseconds())
	return &SendResult{
		Success:   true,
		Message:   fmt.Sprintf("站内信发送成功，用户数: %d", len(msgs)),
		Timestamp: time.Now(),
		CostMs:    time.Since(startTime).Milliseconds(),
	}, nil
}

// TestConnection 测试连接
func (c *siteMessageClient) TestConnection(ctx context.Context, toEmail []string) (*TestResult, error) {
	startTime := time.Now()

	// 站内信测试只检查存储接口是否正常
	if c.store == nil {
		return &TestResult{
			Success:   false,
			Message:   "存储接口未初始化",
			Latency:   time.Since(startTime),
			Timestamp: time.Now(),
			Error:     fmt.Errorf("存储接口未初始化"),
		}, fmt.Errorf("存储接口未初始化")
	}

	return &TestResult{
		Success:   true,
		Message:   "站内信客户端正常",
		Latency:   time.Since(startTime),
		Timestamp: time.Now(),
	}, nil
}

// HealthCheck 健康检查
func (c *siteMessageClient) HealthCheck(ctx context.Context) (*HealthStatus, error) {
	if atomic.LoadInt32(&c.closed) == 1 {
		return &HealthStatus{
			Healthy:       false,
			Message:       "客户端已关闭",
			LastCheckTime: time.Now(),
		}, fmt.Errorf("客户端已关闭")
	}

	if c.store == nil {
		c.updateHealthStatus(false, fmt.Errorf("存储接口未初始化"))
		return &HealthStatus{
			Healthy:       false,
			Message:       "存储接口未初始化",
			LastCheckTime: time.Now(),
		}, fmt.Errorf("存储接口未初始化")
	}

	c.updateHealthStatus(true, nil)
	return &HealthStatus{
		Healthy:       true,
		Message:       "站内信客户端健康",
		LastCheckTime: time.Now(),
	}, nil
}
