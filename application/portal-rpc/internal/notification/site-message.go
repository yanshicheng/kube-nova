package notification

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
)

// SiteMessageStore 站内信存储接口（需要外部实现）
type SiteMessageStore interface {
	// SaveMessage 保存站内信
	SaveMessage(ctx context.Context, msg *SiteMessageData) error
	// SaveBatchMessages 批量保存站内信
	SaveBatchMessages(ctx context.Context, msgs []*SiteMessageData) error
}

// MessagePushCallback 消息推送回调接口
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
	pushCallback MessagePushCallback
	pushWg       sync.WaitGroup // 追踪推送 goroutine，确保优雅关闭
}

// NewSiteMessageClient 创建站内信客户端
func NewSiteMessageClient(config Config, store SiteMessageStore) (FullChannel, error) {
	if store == nil {
		return nil, errorx.Msg("站内信存储接口不能为空")
	}

	bc := newBaseClient(config)
	client := &siteMessageClient{
		baseClient: bc,
		store:      store,
	}

	client.l.Infof("创建站内信客户端: %s", config.UUID)
	return client, nil
}

// SetPushCallback 设置推送回调
func (c *siteMessageClient) SetPushCallback(callback MessagePushCallback) {
	c.pushCallback = callback
}

// WithContext 设置上下文
// 返回带有新上下文的客户端副本
func (c *siteMessageClient) WithContext(ctx context.Context) Client {
	// 创建新的 baseClient 副本
	newBaseClient := c.baseClient.WithContext(ctx).(*baseClient)

	// 创建新的 siteMessageClient 实例（共享 store 和 pushCallback）
	return &siteMessageClient{
		baseClient:   newBaseClient,
		store:        c.store,
		pushCallback: c.pushCallback,
		pushWg:       sync.WaitGroup{}, // 新的 WaitGroup，不影响原始客户端
	}
}

// SendPrometheusAlert 发送 Prometheus 告警站内信
func (c *siteMessageClient) SendPrometheusAlert(ctx context.Context, opts *AlertOptions, alerts []*AlertInstance) (*SendResult, error) {
	startTime := time.Now()

	if atomic.LoadInt32(&c.closed) == 1 {
		return nil, errorx.Msg("客户端已关闭")
	}

	if opts.Mentions == nil {
		return &SendResult{
			Success:   false,
			Message:   "站内信需要指定用户ID",
			Timestamp: time.Now(),
			Error:     errorx.Msg("未指定用户ID"),
		}, errorx.Msg("未指定用户ID")
	}

	formatter := NewMessageFormatter(opts.PortalName, opts.PortalUrl)

	// 按告警名称聚合
	alertsByName := make(map[string][]*AlertInstance)
	for _, alert := range alerts {
		alertsByName[alert.AlertName] = append(alertsByName[alert.AlertName], alert)
	}

	var msgs []*SiteMessageData
	expireAt := time.Now().AddDate(0, 0, 30)

	// 修复: 使用 InternalUserIds 而不是 AtUserIds，避免类型转换
	if len(opts.Mentions.InternalUserIds) == 0 {
		// 向后兼容：如果 InternalUserIds 为空，尝试使用 AtUserIds
		if len(opts.Mentions.AtUserIds) > 0 {
			c.l.Infof("[站内信] 使用向后兼容模式: AtUserIds")
			for _, userIdStr := range opts.Mentions.AtUserIds {
				var userId uint64
				_, err := fmt.Sscanf(userIdStr, "%d", &userId)
				if err != nil {
					c.l.Errorf("[站内信] 用户ID转换失败: %s, 错误: %v", userIdStr, err)
					continue
				}

				// 为每个告警名称创建一条聚合消息
				for alertName, alertGroup := range alertsByName {
					title := c.buildAggregatedTitle(alertName, alertGroup, opts)
					content := c.buildAggregatedContent(formatter, alertGroup, opts)

					msg := &SiteMessageData{
						UUID:           uuid.New().String(),
						UserID:         userId,
						InstanceID:     alertGroup[0].ID,
						NotificationID: 0,
						Title:          title,
						Content:        content,
						MessageType:    "alert",
						Severity:       alertGroup[0].Severity,
						Category:       "prometheus",
						ActionURL:      alertGroup[0].GeneratorURL,
						ActionText:     "查看详情",
						IsRead:         0,
						IsStarred:      0,
						ExpireAt:       expireAt,
					}
					msgs = append(msgs, msg)
				}
			}
		} else {
			return &SendResult{
				Success:   false,
				Message:   "没有有效的用户ID",
				Timestamp: time.Now(),
				Error:     errorx.Msg("没有有效的用户ID"),
			}, errorx.Msg("没有有效的用户ID")
		}
	} else {
		// 使用新的 InternalUserIds（推荐方式）
		for _, userId := range opts.Mentions.InternalUserIds {
			// 为每个告警名称创建一条聚合消息
			for alertName, alertGroup := range alertsByName {
				title := c.buildAggregatedTitle(alertName, alertGroup, opts)
				content := c.buildAggregatedContent(formatter, alertGroup, opts)

				msg := &SiteMessageData{
					UUID:           uuid.New().String(),
					UserID:         userId,
					InstanceID:     alertGroup[0].ID,
					NotificationID: 0,
					Title:          title,
					Content:        content,
					MessageType:    "alert",
					Severity:       alertGroup[0].Severity,
					Category:       "prometheus",
					ActionURL:      alertGroup[0].GeneratorURL,
					ActionText:     "查看详情",
					IsRead:         0,
					IsStarred:      0,
					ExpireAt:       expireAt,
				}
				msgs = append(msgs, msg)
			}
		}
	}

	if len(msgs) == 0 {
		return &SendResult{
			Success:   false,
			Message:   "没有有效的用户ID或告警",
			Timestamp: time.Now(),
			Error:     errorx.Msg("没有有效的用户ID或告警"),
		}, errorx.Msg("没有有效的用户ID或告警")
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

	// 异步推送，使用 WaitGroup 追踪确保消息不丢失
	if c.pushCallback != nil {
		c.pushWg.Add(1) // 增加计数
		go func() {
			defer c.pushWg.Done() // 完成后减少计数

			pushCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			if err := c.pushCallback.OnBatchMessagesCreated(pushCtx, msgs); err != nil {
				c.l.Errorf("[站内信] Redis 推送失败 | UUID: %s | 错误: %v",
					c.baseClient.GetUUID(), err)
			}
		}()
	}

	c.recordSuccess(time.Since(startTime))

	c.l.Infof("[站内信] 发送成功 | UUID: %s | 告警类型数: %d | 消息数: %d | 用户数: %d | 原始告警数: %d | 耗时: %dms",
		c.baseClient.GetUUID(), len(alertsByName), len(msgs), len(opts.Mentions.InternalUserIds), len(alerts), time.Since(startTime).Milliseconds())

	return &SendResult{
		Success:   true,
		Message:   fmt.Sprintf("站内信发送成功，告警类型: %d，消息数: %d", len(alertsByName), len(msgs)),
		Timestamp: time.Now(),
		CostMs:    time.Since(startTime).Milliseconds(),
	}, nil
}

// buildAggregatedContent 构建聚合内容
func (c *siteMessageClient) buildAggregatedContent(formatter *MessageFormatter, alerts []*AlertInstance, opts *AlertOptions) string {
	if len(alerts) == 0 {
		return ""
	}

	first := alerts[0]
	projectDisplay := opts.ProjectName
	if projectDisplay == "" {
		projectDisplay = "集群级"
	}

	var content strings.Builder

	// 告警基本信息
	content.WriteString("## 告警信息\n\n")
	content.WriteString(fmt.Sprintf("- **项目**: %s\n", projectDisplay))
	content.WriteString(fmt.Sprintf("- **集群**: %s\n", opts.ClusterName))
	content.WriteString(fmt.Sprintf("- **告警规则**: %s\n", first.AlertName))
	content.WriteString(fmt.Sprintf("- **级别**: %s\n", first.Severity))
	content.WriteString(fmt.Sprintf("- **状态**: %s\n", first.Status))

	// 实例列表
	if len(alerts) > 1 {
		content.WriteString(fmt.Sprintf("\n## 受影响实例 (%d个)\n\n", len(alerts)))

		maxShow := 10
		for i, alert := range alerts {
			if i >= maxShow {
				content.WriteString(fmt.Sprintf("- ... 还有 %d 个实例\n", len(alerts)-maxShow))
				break
			}
			duration := formatter.FormatDuration(alert.Duration)
			content.WriteString(fmt.Sprintf("- `%s` (持续: %s)\n", alert.Instance, duration))
		}
	} else {
		content.WriteString(fmt.Sprintf("- **实例**: %s\n", first.Instance))
		content.WriteString(fmt.Sprintf("- **持续时间**: %s\n", formatter.FormatDuration(first.Duration)))
	}

	// 告警描述
	description := formatter.GetAlertDescription(first)
	if description != "" && description != "暂无描述" {
		content.WriteString(fmt.Sprintf("\n## 告警描述\n\n%s\n", description))
	}

	// 标签信息
	if len(first.Labels) > 0 {
		content.WriteString("\n## 标签\n\n")
		for k, v := range first.Labels {
			// 跳过已展示的标签
			if k == "alertname" || k == "severity" || k == "instance" {
				continue
			}
			content.WriteString(fmt.Sprintf("- **%s**: `%s`\n", k, v))
		}
	}

	return content.String()
}

// buildAggregatedTitle 构建聚合标题
func (c *siteMessageClient) buildAggregatedTitle(alertName string, alerts []*AlertInstance, opts *AlertOptions) string {
	if len(alerts) > 1 {
		return fmt.Sprintf("%s (%d个实例) - %s", alertName, len(alerts), opts.ClusterName)
	}
	return fmt.Sprintf("%s - %s", alertName, opts.ClusterName)
}

// buildSingleAlertContent 构建单个告警的内容
// 注意: 此方法当前未被使用，保留用于后续可能的单条告警场景
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
		return nil, errorx.Msg("客户端已关闭")
	}

	// 获取用户ID列表
	if opts.Mentions == nil {
		return &SendResult{
			Success:   false,
			Message:   "站内信需要指定用户ID",
			Timestamp: time.Now(),
			Error:     errorx.Msg("未指定用户ID"),
		}, errorx.Msg("未指定用户ID")
	}

	// 为每个用户创建站内信
	var msgs []*SiteMessageData
	expireAt := time.Now().AddDate(0, 0, 30)

	// 修复: 使用 InternalUserIds 而不是 AtUserIds
	if len(opts.Mentions.InternalUserIds) == 0 {
		// 向后兼容：如果 InternalUserIds 为空，尝试使用 AtUserIds
		if len(opts.Mentions.AtUserIds) > 0 {
			c.l.Infof("[站内信] 使用向后兼容模式: AtUserIds")
			for _, userIdStr := range opts.Mentions.AtUserIds {
				var userId uint64
				_, err := fmt.Sscanf(userIdStr, "%d", &userId)
				if err != nil {
					c.l.Errorf("[站内信] 用户ID转换失败: %s, 错误: %v", userIdStr, err)
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
		} else {
			return &SendResult{
				Success:   false,
				Message:   "没有有效的用户ID",
				Timestamp: time.Now(),
				Error:     errorx.Msg("没有有效的用户ID"),
			}, errorx.Msg("没有有效的用户ID")
		}
	} else {
		// 使用新的 InternalUserIds（推荐方式）
		for _, userId := range opts.Mentions.InternalUserIds {
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
	}

	if len(msgs) == 0 {
		return &SendResult{
			Success:   false,
			Message:   "没有有效的用户ID",
			Timestamp: time.Now(),
			Error:     errorx.Msg("没有有效的用户ID"),
		}, errorx.Msg("没有有效的用户ID")
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

	// 异步推送，使用 WaitGroup 追踪确保消息不丢失
	if c.pushCallback != nil {
		c.pushWg.Add(1) // 增加计数
		go func() {
			defer c.pushWg.Done() // 完成后减少计数

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
			Error:     errorx.Msg("存储接口未初始化"),
		}, errorx.Msg("存储接口未初始化")
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
		}, errorx.Msg("客户端已关闭")
	}

	if c.store == nil {
		c.updateHealthStatus(false, errorx.Msg("存储接口未初始化"))
		return &HealthStatus{
			Healthy:       false,
			Message:       "存储接口未初始化",
			LastCheckTime: time.Now(),
		}, errorx.Msg("存储接口未初始化")
	}

	c.updateHealthStatus(true, nil)
	return &HealthStatus{
		Healthy:       true,
		Message:       "站内信客户端健康",
		LastCheckTime: time.Now(),
	}, nil
}

// Close 关闭客户端
// 等待所有推送 goroutine 完成，确保消息不丢失
func (c *siteMessageClient) Close() error {
	// 等待所有推送完成，最多等待 10 秒
	done := make(chan struct{})
	go func() {
		c.pushWg.Wait()
		close(done)
	}()

	select {
	case <-done:
		c.l.Info("[站内信] 所有推送已完成")
	case <-time.After(10 * time.Second):
		c.l.Infof("[站内信] 等待推送超时，强制关闭")
	}

	return c.baseClient.Close()
}
