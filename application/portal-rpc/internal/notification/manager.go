package notification

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/model"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

// DefaultAlertGroupID 默认告警组ID
const DefaultAlertGroupID uint64 = 2

// ManagerConfig 管理器配置
type ManagerConfig struct {
	PortalName string
	PortalUrl  string

	// 数据库模型 - 直接使用 model 包中的接口
	SysUserModel                 model.SysUserModel
	AlertChannelsModel           model.AlertChannelsModel
	AlertGroupsModel             model.AlertGroupsModel
	AlertGroupMembersModel       model.AlertGroupMembersModel
	AlertGroupLevelChannelsModel model.AlertGroupLevelChannelsModel
	AlertNotificationsModel      model.AlertNotificationsModel
	AlertGroupAppsModel          model.AlertGroupAppsModel
	SiteMessagesModel            model.SiteMessagesModel
	Redis                        *redis.Redis
	AggregatorConfig             *AggregatorConfig
}

// manager 告警管理器实现
type manager struct {
	// 告警客户端存储
	channels      map[string]*channelWrapper
	mu            sync.RWMutex
	messagePusher *MessagePushService

	// Portal 信息
	portalName string
	portalUrl  string

	// 日志
	l logx.Logger

	// 数据库模型 - 直接使用 model 包中的接口
	sysUserModel                 model.SysUserModel
	alertChannelsModel           model.AlertChannelsModel
	alertGroupsModel             model.AlertGroupsModel
	alertGroupMembersModel       model.AlertGroupMembersModel
	alertGroupLevelChannelsModel model.AlertGroupLevelChannelsModel
	alertNotificationsModel      model.AlertNotificationsModel
	alertGroupAppsModel          model.AlertGroupAppsModel
	siteMessagesModel            model.SiteMessagesModel

	// 状态
	closed bool

	// 统计信息
	statsMu sync.RWMutex

	aggregator *AlertAggregator
}

// channelWrapper 渠道包装器
type channelWrapper struct {
	client     Client
	config     Config
	createTime time.Time
	lastAccess time.Time
	mu         sync.RWMutex
	metrics    WrapperMetrics
}

// WrapperMetrics 包装器指标
type WrapperMetrics struct {
	AccessCount   int64
	ErrorCount    int64
	LastErrorTime time.Time
}

// NewManager 创建告警管理器
func NewManager(config ManagerConfig) Manager {
	m := &manager{
		channels:                     make(map[string]*channelWrapper),
		l:                            logx.WithContext(context.Background()),
		portalName:                   config.PortalName,
		portalUrl:                    config.PortalUrl,
		sysUserModel:                 config.SysUserModel,
		alertChannelsModel:           config.AlertChannelsModel,
		alertGroupsModel:             config.AlertGroupsModel,
		alertGroupMembersModel:       config.AlertGroupMembersModel,
		alertGroupLevelChannelsModel: config.AlertGroupLevelChannelsModel,
		alertNotificationsModel:      config.AlertNotificationsModel,
		alertGroupAppsModel:          config.AlertGroupAppsModel,
		siteMessagesModel:            config.SiteMessagesModel,
	}
	if config.Redis != nil {
		m.messagePusher = NewMessagePushService(config.Redis)
		m.l.Info("[告警管理器] 消息推送服务已启用")
	} else {
		m.l.Errorf("[告警管理器] Redis 未配置，消息推送功能不可用")
	}

	// 初始化聚合器
	if config.AggregatorConfig != nil && config.Redis != nil {
		m.aggregator = NewAlertAggregatorWithManager(config.Redis, m, *config.AggregatorConfig)
		if m.aggregator != nil {
			m.l.Infof("[告警管理器] 聚合器已启用 | Critical=%v | Warning=%v | Info=%v | Default=%v",
				config.AggregatorConfig.SeverityWindows.Critical,
				config.AggregatorConfig.SeverityWindows.Warning,
				config.AggregatorConfig.SeverityWindows.Info,
				config.AggregatorConfig.SeverityWindows.Default,
			)
		} else {
			m.l.Error("[告警管理器] 聚合器初始化失败，将使用直接发送")
		}
	} else {
		m.l.Info("[告警管理器] 聚合器未启用（Redis或配置缺失），使用直接发送")
	}

	m.l.Infof("告警管理器已创建: portalName=%s, portalUrl=%s", config.PortalName, config.PortalUrl)
	return m
}

// SetPortalInfo 设置平台信息
func (m *manager) SetPortalInfo(portalName, portalUrl string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.portalName = portalName
	m.portalUrl = portalUrl
	m.l.Infof("设置平台信息: name=%s, url=%s", portalName, portalUrl)
}

// GetPortalName 获取平台名称
func (m *manager) GetPortalName() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.portalName
}

// GetPortalUrl 获取平台URL
func (m *manager) GetPortalUrl() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.portalUrl
}

// GetChannel 获取告警渠道（用于测试）
func (m *manager) GetChannel(ctx context.Context, uuid string) (Client, error) {
	m.mu.RLock()
	wrapper, exists := m.channels[uuid]
	m.mu.RUnlock()

	if !exists {
		// 尝试从数据库加载
		if m.alertChannelsModel != nil {
			channelData, err := m.alertChannelsModel.FindOneByUuid(ctx, uuid)
			if err != nil {
				m.l.Errorf("告警渠道 %s 不存在: %v", uuid, err)
				return nil, fmt.Errorf("告警渠道 %s 不存在", uuid)
			}

			// 解析配置并添加渠道
			config, err := m.parseChannelFromDB(channelData)
			if err != nil {
				return nil, fmt.Errorf("解析渠道配置失败: %w", err)
			}

			if err := m.AddChannel(*config); err != nil {
				return nil, fmt.Errorf("添加渠道失败: %w", err)
			}

			m.mu.RLock()
			wrapper, exists = m.channels[uuid]
			m.mu.RUnlock()

			if !exists {
				return nil, fmt.Errorf("渠道添加后仍不存在")
			}
		} else {
			return nil, fmt.Errorf("告警渠道 %s 不存在", uuid)
		}
	}

	// 更新访问时间
	wrapper.mu.Lock()
	wrapper.lastAccess = time.Now()
	wrapper.metrics.AccessCount++
	wrapper.mu.Unlock()

	if wrapper.client == nil {
		return nil, fmt.Errorf("告警渠道 %s 未初始化", uuid)
	}

	return wrapper.client.WithContext(ctx), nil
}

// AddChannel 添加告警渠道
func (m *manager) AddChannel(config Config) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return fmt.Errorf("告警管理器已关闭")
	}

	if err := validateConfig(config); err != nil {
		return fmt.Errorf("告警配置验证失败: %w", err)
	}

	if _, exists := m.channels[config.UUID]; exists {
		return nil // 已存在，跳过
	}

	client, err := m.createClient(config)
	if err != nil {
		return fmt.Errorf("创建告警渠道失败: %w", err)
	}

	m.channels[config.UUID] = &channelWrapper{
		client:     client,
		config:     config,
		createTime: time.Now(),
		lastAccess: time.Now(),
	}

	m.l.Infof("添加告警渠道成功: %s (类型: %s)", config.UUID, config.Type)
	return nil
}

// createClient 根据配置创建客户端
func (m *manager) createClient(config Config) (Client, error) {
	switch config.Type {
	case AlertTypeDingTalk:
		return NewDingTalkClient(config)
	case AlertTypeWeChat:
		return NewWeChatClient(config)
	case AlertTypeFeiShu:
		return NewFeiShuClient(config)
	case AlertTypeEmail:
		return NewEmailClient(config)
	case AlertTypeWebhook:
		return NewWebhookClient(config)
	case AlertTypeSiteMessage:
		store := &siteMessageStoreImpl{m: m}
		client, err := NewSiteMessageClient(config, store)
		if err != nil {
			return nil, err
		}

		if siteClient, ok := client.(*siteMessageClient); ok && m.messagePusher != nil {
			siteClient.SetPushCallback(m.messagePusher)
			m.l.Info("[站内消息] 已设置消息推送回调")
		}
		return client, nil
	default:
		return nil, fmt.Errorf("不支持的告警类型: %s", config.Type)
	}
}

// RemoveChannel 移除告警渠道
func (m *manager) RemoveChannel(uuid string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	wrapper, exists := m.channels[uuid]
	if !exists {
		return fmt.Errorf("告警渠道 %s 不存在", uuid)
	}

	if wrapper.client != nil {
		_ = wrapper.client.Close()
	}

	delete(m.channels, uuid)
	m.l.Infof("移除告警渠道成功: %s", uuid)
	return nil
}

// UpdateChannel 更新告警渠道
func (m *manager) UpdateChannel(config Config) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	wrapper, exists := m.channels[config.UUID]
	if !exists {
		return fmt.Errorf("告警渠道 %s 不存在", config.UUID)
	}

	if wrapper.client != nil {
		_ = wrapper.client.Close()
	}

	newClient, err := m.createClient(config)
	if err != nil {
		return err
	}

	wrapper.client = newClient
	wrapper.config = config
	return nil
}

// ListChannels 列出所有告警渠道
func (m *manager) ListChannels() []Client {
	m.mu.RLock()
	defer m.mu.RUnlock()

	clients := make([]Client, 0, len(m.channels))
	for _, wrapper := range m.channels {
		if wrapper.client != nil {
			clients = append(clients, wrapper.client)
		}
	}
	return clients
}

// GetChannelCount 获取告警渠道数量
func (m *manager) GetChannelCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.channels)
}

// GetChannelsByType 根据类型获取告警渠道
func (m *manager) GetChannelsByType(alertType AlertType) []Client {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var clients []Client
	for _, wrapper := range m.channels {
		if wrapper.client != nil && wrapper.client.GetType() == alertType {
			clients = append(clients, wrapper.client)
		}
	}
	return clients
}

// Close 关闭管理器
func (m *manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return nil
	}

	// 停止聚合器
	if m.aggregator != nil {
		m.l.Info("[告警管理器] 正在刷新聚合器缓冲...")
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		m.aggregator.FlushAll(ctx)
		cancel()

		m.aggregator.Stop()
		m.l.Info("[告警管理器] 聚合器已停止")
	}

	for uuid, wrapper := range m.channels {
		if wrapper.client != nil {
			if err := wrapper.client.Close(); err != nil {
				m.l.Errorf("关闭告警渠道 %s 失败: %v", uuid, err)
			}
		}
	}

	m.channels = make(map[string]*channelWrapper)
	m.closed = true
	m.l.Info("告警管理器已关闭")
	return nil
}

// ========== alertGroupInfo 告警分组信息 ==========

type alertGroupInfo struct {
	ProjectID   uint64
	ProjectName string
	ClusterName string
	Severity    string
	Alerts      []*AlertInstance
}

// ========== PrometheusAlertNotification 核心方法（只接收 ctx 和 alerts）==========

// PrometheusAlertNotification 发送 Prometheus 告警通知
// 只接收 ctx 和 alerts，内部自动按 ProjectID + Severity 分组处理
func (m *manager) PrometheusAlertNotification(ctx context.Context, alerts []*AlertInstance) error {
	if len(alerts) == 0 {
		m.l.Infof("[告警通知] 告警列表为空，跳过处理")
		return nil
	}

	m.l.Infof("[告警通知] 开始处理 %d 条告警", len(alerts))

	// 如果启用聚合器，使用聚合器处理
	if m.aggregator != nil && m.aggregator.config.Enabled {
		m.l.Info("[告警通知] 使用聚合器处理告警")
		return m.aggregator.AddAlert(ctx, alerts)
	}
	m.l.Info("[告警通知] 使用直接发送（未启用聚合）")

	// 1. 按 ProjectID + Severity 分组
	groups := m.groupAlertsByProjectAndSeverity(alerts)
	m.l.Infof("[告警通知] 分组完成，共 %d 个分组", len(groups))

	// 2. 并发处理每个分组
	var wg sync.WaitGroup
	var mu sync.Mutex
	var errors []error

	for key, group := range groups {
		wg.Add(1)
		go func(groupKey string, groupInfo *alertGroupInfo) {
			defer wg.Done()

			m.l.Infof("[告警通知] 处理分组: %s | ProjectID: %d | Severity: %s | 告警数: %d",
				groupKey, groupInfo.ProjectID, groupInfo.Severity, len(groupInfo.Alerts))

			if err := m.processAlertGroup(ctx, groupInfo); err != nil {
				mu.Lock()
				errors = append(errors, fmt.Errorf("分组 %s 处理失败: %w", groupKey, err))
				mu.Unlock()
				m.l.Errorf("[告警通知] 分组 %s 处理失败: %v", groupKey, err)
			} else {
				m.l.Infof("[告警通知] 分组 %s 处理成功", groupKey)
			}
		}(key, group)
	}

	wg.Wait()

	if len(errors) > 0 {
		m.l.Errorf("[告警通知] 完成，但有 %d 个分组处理失败", len(errors))
		return errors[0]
	}

	m.l.Infof("[告警通知] 全部处理完成")
	return nil
}

// 实现 InternalManager 接口，供聚合器调用
func (m *manager) SendAlertsDirectly(ctx context.Context, alerts []*AlertInstance) error {
	if len(alerts) == 0 {
		return nil
	}

	m.l.Infof("[告警发送] 直接发送 %d 条告警（绕过聚合器）", len(alerts))

	// 临时禁用聚合器，防止循环调用
	originalAgg := m.aggregator
	m.aggregator = nil
	defer func() {
		m.aggregator = originalAgg
	}()

	// 直接调用原有的处理逻辑
	return m.PrometheusAlertNotification(ctx, alerts)
}

// groupAlertsByProjectAndSeverity 按项目ID和级别分组告警
func (m *manager) groupAlertsByProjectAndSeverity(alerts []*AlertInstance) map[string]*alertGroupInfo {
	groups := make(map[string]*alertGroupInfo)

	for _, alert := range alerts {
		// 生成分组 key: projectID-severity
		key := fmt.Sprintf("%d-%s", alert.ProjectID, alert.Severity)

		if _, exists := groups[key]; !exists {
			groups[key] = &alertGroupInfo{
				ProjectID:   alert.ProjectID,
				ProjectName: alert.ProjectName,
				ClusterName: alert.ClusterName,
				Severity:    alert.Severity,
				Alerts:      make([]*AlertInstance, 0),
			}
		}

		groups[key].Alerts = append(groups[key].Alerts, alert)

		// 更新项目名称和集群名称（取第一个非空的）
		if groups[key].ProjectName == "" && alert.ProjectName != "" {
			groups[key].ProjectName = alert.ProjectName
		}
		if groups[key].ClusterName == "" && alert.ClusterName != "" {
			groups[key].ClusterName = alert.ClusterName
		}
	}

	return groups
}

// processAlertGroup 处理单个告警分组
func (m *manager) processAlertGroup(ctx context.Context, group *alertGroupInfo) error {
	// 1. 解析告警组（ProjectID=0 使用默认告警组 ID=2）
	groupID, err := m.resolveAlertGroup(ctx, group.ProjectID)
	if err != nil {
		return fmt.Errorf("解析告警组失败: %w", err)
	}
	m.l.Infof("[告警分组] ProjectID: %d -> GroupID: %d", group.ProjectID, groupID)

	// 2. 获取对应级别的渠道列表
	channels, err := m.getChannelsForSeverity(ctx, groupID, group.Severity)
	if err != nil {
		return fmt.Errorf("获取渠道失败: %w", err)
	}
	if len(channels) == 0 {
		m.l.Infof("[告警分组] 告警组 %d 级别 %s 没有配置渠道，跳过", groupID, group.Severity)
		return nil
	}
	m.l.Infof("[告警分组] 获取到 %d 个渠道", len(channels))

	// 3. 获取告警组成员用于@人
	mentions, err := m.resolveMentions(ctx, groupID, nil)
	if err != nil {
		m.l.Errorf("[告警分组] 解析@人配置失败: %v，将继续发送但不@人", err)
	}

	// 4. 构建告警选项
	opts := &AlertOptions{
		PortalName:  m.portalName,
		PortalUrl:   m.portalUrl,
		Mentions:    mentions,
		ProjectName: group.ProjectName,
		ClusterName: group.ClusterName,
		Severity:    group.Severity,
	}

	// 5. 并发发送到所有渠道
	if err := m.sendToChannels(ctx, channels, opts, group.Alerts, groupID); err != nil {
		return err
	}

	// 这样避免重复创建
	if !m.hasChannelType(channels, AlertTypeSiteMessage) {
		if err := m.createSiteMessagesForAlerts(ctx, group.Alerts, mentions, opts); err != nil {
			m.l.Errorf("[告警分组] 创建站内信失败: %v", err)
			// 站内信失败不影响整体流程
		}
	}

	return nil
}

// hasChannelType 检查渠道列表中是否包含指定类型
func (m *manager) hasChannelType(channels []*ChannelInfo, channelType AlertType) bool {
	for _, ch := range channels {
		if ch.Type == channelType {
			return true
		}
	}
	return false
}

func (m *manager) createSiteMessagesForAlerts(ctx context.Context, alerts []*AlertInstance, mentions *MentionConfig, opts *AlertOptions) error {
	if m.siteMessagesModel == nil || mentions == nil || len(mentions.AtUserIds) == 0 {
		return nil
	}

	formatter := NewMessageFormatter(opts.PortalName, opts.PortalUrl)
	expireAt := time.Now().AddDate(0, 0, 30)

	for _, alert := range alerts {
		for _, userIdStr := range mentions.AtUserIds {
			var userId uint64
			if _, err := fmt.Sscanf(userIdStr, "%d", &userId); err != nil {
				continue
			}

			// 构建站内信内容
			content := fmt.Sprintf("**实例:** %s\n**持续:** %s\n**描述:** %s",
				alert.Instance,
				formatter.FormatDuration(alert.Duration),
				formatter.GetAlertDescription(alert),
			)

			msgUUID := uuid.New().String()

			siteMsg := &model.SiteMessages{
				Uuid:           msgUUID,
				NotificationId: 0,
				InstanceId:     alert.ID,
				UserId:         userId,
				Title:          alert.AlertName,
				Content:        content,
				MessageType:    "alert",
				Severity:       alert.Severity,
				Category:       "prometheus",
				ExtraData:      "{}",
				ActionUrl:      opts.PortalUrl,
				ActionText:     "查看详情",
				IsRead:         0,
				ReadAt:         time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC),
				IsStarred:      0,
				ExpireAt:       expireAt,
			}

			result, err := m.siteMessagesModel.Insert(ctx, siteMsg)
			if err != nil {
				m.l.Errorf("[站内信] 创建失败: alertID=%d, userID=%d, err=%v", alert.ID, userId, err)
				continue
			}

			if m.messagePusher != nil {
				msgID, _ := result.LastInsertId()
				pushData := &SiteMessageData{
					UUID:        msgUUID,
					UserID:      userId,
					Title:       alert.AlertName,
					Content:     content,
					MessageType: "alert",
					Severity:    alert.Severity,
					Category:    "prometheus",
					ActionURL:   opts.PortalUrl,
					ActionText:  "查看详情",
					IsRead:      0,
					InstanceID:  alert.ID,
				}
				_ = msgID // msgID 可用于日志或其他用途

				// 异步推送，不阻塞主流程
				go func(data *SiteMessageData, alertID uint64, uid uint64) {
					pushCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					defer cancel()

					if err := m.messagePusher.PushMessage(pushCtx, data); err != nil {
						m.l.Errorf("[站内信] Redis推送失败: alertID=%d, userID=%d, err=%v", alertID, uid, err)
					} else {
						m.l.Infof("[站内信] 推送成功: alertID=%d, userID=%d", alertID, uid)
					}
				}(pushData, alert.ID, userId)
			}
		}
	}

	return nil
}

// ========== DefaultNotification 通用通知方法 ==========

// DefaultNotification 发送通用通知（如创建账号等系统通知）
// 查询默认告警组(ID=2)的 notification 级别渠道，没有则用 default
func (m *manager) DefaultNotification(ctx context.Context, userIDs []uint64, title, content string) error {
	if len(userIDs) == 0 {
		return fmt.Errorf("用户ID列表为空")
	}

	m.l.Infof("[通用通知] 开始发送: userCount=%d, title=%s", len(userIDs), title)

	// 1. 使用默认告警组
	groupID := DefaultAlertGroupID

	// 2. 先尝试获取 notification 级别渠道
	channels, err := m.getChannelsForSeverity(ctx, groupID, string(AlertLevelNotification))
	if err != nil || len(channels) == 0 {
		// 没有 notification 级别，尝试 default 级别
		m.l.Infof("[通用通知] notification 级别无渠道，尝试 default 级别")
		channels, err = m.getChannelsForSeverity(ctx, groupID, string(AlertLevelDefault))
		if err != nil || len(channels) == 0 {
			m.l.Errorf("[通用通知] 默认告警组没有配置渠道")
			return nil
		}
	}
	m.l.Infof("[通用通知] 获取到 %d 个渠道", len(channels))

	// 3. 获取用户信息
	users, err := m.getUserInfoByIDs(ctx, userIDs)
	if err != nil {
		return fmt.Errorf("获取用户信息失败: %w", err)
	}

	// 4. 构建@人配置
	mentions := m.buildMentionsFromUsers(users)

	// 5. 构建通知选项
	opts := &NotificationOptions{
		PortalName: m.portalName,
		PortalUrl:  m.portalUrl,
		Title:      title,
		Content:    content,
		Mentions:   mentions,
	}

	// 6. 发送通知
	if err := m.sendNotificationToChannels(ctx, channels, opts); err != nil {
		return err
	}

	if !m.hasChannelType(channels, AlertTypeSiteMessage) {
		if err := m.createSiteMessagesForNotification(ctx, userIDs, title, content); err != nil {
			m.l.Errorf("[通用通知] 创建站内信失败: %v", err)
		}
	}

	return nil
}

// createSiteMessagesForNotification 为通知创建站内信
func (m *manager) createSiteMessagesForNotification(ctx context.Context, userIDs []uint64, title, content string) error {
	if m.siteMessagesModel == nil {
		return nil
	}

	expireAt := time.Now().AddDate(0, 0, 30)

	for _, userId := range userIDs {
		msgUUID := uuid.New().String()

		siteMsg := &model.SiteMessages{
			Uuid:           msgUUID,
			NotificationId: 0,
			InstanceId:     0,
			UserId:         userId,
			Title:          title,
			Content:        content,
			MessageType:    "notification",
			Severity:       "info",
			Category:       "system",
			ExtraData:      "{}",
			ActionUrl:      m.portalUrl,
			ActionText:     "查看详情",
			IsRead:         0,
			ReadAt:         time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC),
			IsStarred:      0,
			ExpireAt:       expireAt,
		}

		result, err := m.siteMessagesModel.Insert(ctx, siteMsg)
		if err != nil {
			m.l.Errorf("[站内信] 创建失败: userID=%d, err=%v", userId, err)
			continue
		}

		if m.messagePusher != nil {
			msgID, _ := result.LastInsertId()
			pushData := &SiteMessageData{
				UUID:        msgUUID,
				UserID:      userId,
				Title:       title,
				Content:     content,
				MessageType: "notification",
				Severity:    "info",
				Category:    "system",
				ActionURL:   m.portalUrl,
				ActionText:  "查看详情",
				IsRead:      0,
			}
			_ = msgID

			// 异步推送
			go func(data *SiteMessageData, uid uint64) {
				pushCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()

				if err := m.messagePusher.PushMessage(pushCtx, data); err != nil {
					m.l.Errorf("[站内信] Redis推送失败: userID=%d, err=%v", uid, err)
				} else {
					m.l.Infof("[站内信] 推送成功: userID=%d, title=%s", uid, data.Title)
				}
			}(pushData, userId)
		}
	}

	return nil
}

// ========== 内部方法 ==========

// resolveAlertGroup 解析告警组
func (m *manager) resolveAlertGroup(ctx context.Context, projectID uint64) (uint64, error) {
	if projectID == 0 {
		return DefaultAlertGroupID, nil
	}

	if m.alertGroupAppsModel == nil {
		return DefaultAlertGroupID, nil
	}

	// 查询项目绑定的告警组 (app_type=prometheus)
	apps, err := m.alertGroupAppsModel.SearchNoPage(ctx, "", true, "`app_type` = ? AND `app_id` = ?", "prometheus", projectID)
	if err != nil || len(apps) == 0 {
		m.l.Infof("[告警组解析] 项目 %d 未绑定告警组，使用默认告警组 ID=%d", projectID, DefaultAlertGroupID)
		return DefaultAlertGroupID, nil
	}

	return apps[0].GroupId, nil
}

// getChannelsForSeverity 获取对应级别的渠道列表
func (m *manager) getChannelsForSeverity(ctx context.Context, groupID uint64, severity string) ([]*ChannelInfo, error) {
	if m.alertGroupLevelChannelsModel == nil {
		return nil, fmt.Errorf("告警组级别渠道模型未初始化")
	}

	// 查询指定级别
	levelChannel, err := m.alertGroupLevelChannelsModel.FindOneByGroupIdSeverity(ctx, groupID, severity)
	if err != nil {
		// 如果没有配置，尝试 default 级别
		if severity != string(AlertLevelDefault) {
			m.l.Infof("[渠道查询] 级别 %s 无配置，尝试 default 级别", severity)
			levelChannel, err = m.alertGroupLevelChannelsModel.FindOneByGroupIdSeverity(ctx, groupID, string(AlertLevelDefault))
			if err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	if levelChannel == nil || levelChannel.ChannelId == "" {
		return nil, nil
	}

	// 解析渠道ID列表（逗号分隔）
	return m.parseChannelIDs(ctx, levelChannel.ChannelId)
}

// parseChannelIDs 解析渠道ID字符串并获取渠道信息
func (m *manager) parseChannelIDs(ctx context.Context, channelIDStr string) ([]*ChannelInfo, error) {
	if m.alertChannelsModel == nil {
		return nil, fmt.Errorf("告警渠道模型未初始化")
	}

	idStrs := strings.Split(channelIDStr, ",")
	var channels []*ChannelInfo

	for _, idStr := range idStrs {
		idStr = strings.TrimSpace(idStr)
		if idStr == "" {
			continue
		}

		channelID, err := strconv.ParseUint(idStr, 10, 64)
		if err != nil {
			m.l.Errorf("[渠道解析] 解析渠道ID失败: %s", idStr)
			continue
		}

		channel, err := m.alertChannelsModel.FindOne(ctx, channelID)
		if err != nil {
			m.l.Errorf("[渠道解析] 查询渠道失败: channelID=%d, err=%v", channelID, err)
			continue
		}

		channels = append(channels, m.convertToChannelInfo(channel))
	}

	return channels, nil
}

// convertToChannelInfo 将数据库模型转换为 ChannelInfo
func (m *manager) convertToChannelInfo(data *model.AlertChannels) *ChannelInfo {
	return &ChannelInfo{
		ID:     data.Id,
		UUID:   data.Uuid,
		Name:   data.ChannelName,
		Type:   AlertType(data.ChannelType),
		Config: data.Config,
		Options: ClientOptions{
			Timeout:            time.Duration(data.Timeout) * time.Second,
			RetryCount:         int(data.RetryTimes),
			RetryInterval:      2 * time.Second,
			EnableMetrics:      true,
			RateLimitPerMinute: int(data.RateLimit),
		},
	}
}

// parseChannelFromDB 从数据库记录解析渠道配置
func (m *manager) parseChannelFromDB(data *model.AlertChannels) (*Config, error) {
	info := m.convertToChannelInfo(data)
	return m.parseChannelConfig(info)
}

// parseChannelConfig 解析渠道配置
func (m *manager) parseChannelConfig(info *ChannelInfo) (*Config, error) {
	config := &Config{
		UUID:    info.UUID,
		Name:    info.Name,
		Type:    info.Type,
		Options: info.Options,
	}

	if config.Options.Timeout == 0 {
		config.Options = DefaultClientOptions()
	}

	// 解析 JSON 配置
	if info.Config != "" {
		if err := m.parseJSONConfig(info.Config, info.Type, config); err != nil {
			return nil, err
		}
	}

	return config, nil
}

// parseJSONConfig 解析 JSON 配置
func (m *manager) parseJSONConfig(jsonConfig string, channelType AlertType, config *Config) error {
	switch channelType {
	case AlertTypeDingTalk:
		var cfg DingTalkConfig
		if err := json.Unmarshal([]byte(jsonConfig), &cfg); err != nil {
			return err
		}
		config.DingTalk = &cfg
	case AlertTypeWeChat:
		var cfg WeChatConfig
		if err := json.Unmarshal([]byte(jsonConfig), &cfg); err != nil {
			return err
		}
		config.WeChat = &cfg
	case AlertTypeFeiShu:
		var cfg FeiShuConfig
		if err := json.Unmarshal([]byte(jsonConfig), &cfg); err != nil {
			return err
		}
		config.FeiShu = &cfg
	case AlertTypeEmail:
		var cfg EmailConfig
		if err := json.Unmarshal([]byte(jsonConfig), &cfg); err != nil {
			return err
		}
		config.Email = &cfg
	case AlertTypeWebhook:
		var cfg WebhookConfig
		if err := json.Unmarshal([]byte(jsonConfig), &cfg); err != nil {
			return err
		}
		config.Webhook = &cfg
	}
	return nil
}

// resolveMentions 解析@人配置
func (m *manager) resolveMentions(ctx context.Context, groupID uint64, mentions *MentionConfig) (*MentionConfig, error) {
	// 如果已提供有内容的配置，直接使用
	if mentions != nil && (len(mentions.AtMobiles) > 0 || len(mentions.AtUserIds) > 0 || len(mentions.EmailTo) > 0 || mentions.IsAtAll) {
		return mentions, nil
	}

	if m.alertGroupMembersModel == nil || m.sysUserModel == nil {
		return mentions, nil
	}

	// 查询告警组成员
	members, err := m.alertGroupMembersModel.SearchNoPage(ctx, "", true, "`group_id` = ?", groupID)
	if err != nil || len(members) == 0 {
		return mentions, nil
	}

	var userIDs []uint64
	for _, member := range members {
		userIDs = append(userIDs, member.UserId)
	}

	users, err := m.getUserInfoByIDs(ctx, userIDs)
	if err != nil {
		return mentions, err
	}

	return m.buildMentionsFromUsers(users), nil
}

// getUserInfoByIDs 根据用户ID获取用户信息
func (m *manager) getUserInfoByIDs(ctx context.Context, userIDs []uint64) ([]*UserInfo, error) {
	if m.sysUserModel == nil {
		return nil, fmt.Errorf("用户模型未初始化")
	}

	var users []*UserInfo
	for _, userID := range userIDs {
		user, err := m.sysUserModel.FindOne(ctx, userID)
		if err != nil {
			m.l.Errorf("[用户查询] 查询用户失败: userID=%d, err=%v", userID, err)
			continue
		}
		users = append(users, &UserInfo{
			UserID:   user.Id,
			Username: user.Username,
			Phone:    user.Phone,
			Email:    user.Email,
		})
	}
	return users, nil
}

// buildMentionsFromUsers 从用户信息构建@人配置
func (m *manager) buildMentionsFromUsers(users []*UserInfo) *MentionConfig {
	if len(users) == 0 {
		return nil
	}

	mentions := &MentionConfig{
		AtMobiles: make([]string, 0),
		AtUserIds: make([]string, 0),
		EmailTo:   make([]string, 0),
	}

	for _, user := range users {
		if user.Phone != "" {
			mentions.AtMobiles = append(mentions.AtMobiles, user.Phone)
		}
		if user.UserID > 0 {
			mentions.AtUserIds = append(mentions.AtUserIds, fmt.Sprintf("%d", user.UserID))
		}
		if user.Email != "" {
			mentions.EmailTo = append(mentions.EmailTo, user.Email)
		}
	}
	return mentions
}

// sendToChannels 并发发送告警到所有渠道
func (m *manager) sendToChannels(ctx context.Context, channels []*ChannelInfo, opts *AlertOptions, alerts []*AlertInstance, groupID uint64) error {
	var wg sync.WaitGroup
	var mu sync.Mutex
	var errors []error

	for _, ch := range channels {
		// 这样站内信的创建和推送都由 siteMessageClient 统一处理
		// 原代码:
		// if ch.Type == AlertTypeSiteMessage {
		// 	m.l.Infof("[渠道发送] 跳过站内信渠道（已统一处理）| UUID: %s", ch.UUID)
		// 	continue
		// }

		wg.Add(1)
		go func(channelInfo *ChannelInfo) {
			defer wg.Done()

			startTime := time.Now()

			m.l.Infof("[渠道发送] 开始 | 类型: %s | UUID: %s | 名称: %s",
				channelInfo.Type, channelInfo.UUID, channelInfo.Name)

			client, err := m.getOrCreateChannelClient(ctx, channelInfo)
			if err != nil {
				mu.Lock()
				errors = append(errors, fmt.Errorf("渠道 %s(%s) 获取客户端失败: %w", channelInfo.Name, channelInfo.UUID, err))
				mu.Unlock()
				m.l.Errorf("[渠道发送] 获取客户端失败 | 类型: %s | UUID: %s | 错误: %v",
					channelInfo.Type, channelInfo.UUID, err)
				return
			}

			alertChannel, ok := client.(PrometheusAlertChannel)
			if !ok {
				mu.Lock()
				errors = append(errors, fmt.Errorf("渠道 %s(%s) 不支持 Prometheus 告警", channelInfo.Name, channelInfo.UUID))
				mu.Unlock()
				return
			}

			result, err := alertChannel.SendPrometheusAlert(ctx, opts, alerts)
			costMs := time.Since(startTime).Milliseconds()

			// 记录通知日志
			m.recordNotification(ctx, alerts, groupID, channelInfo, opts, result, err, costMs)

			if err != nil {
				mu.Lock()
				errors = append(errors, fmt.Errorf("渠道 %s(%s) 发送失败: %w", channelInfo.Name, channelInfo.UUID, err))
				mu.Unlock()
				m.l.Errorf("[渠道发送] 失败 | 类型: %s | UUID: %s | 名称: %s | 耗时: %dms | 错误: %v",
					channelInfo.Type, channelInfo.UUID, channelInfo.Name, costMs, err)
				return
			}

			if result != nil && result.Success {
				m.l.Infof("[渠道发送] 成功 | 类型: %s | UUID: %s | 名称: %s | 耗时: %dms",
					channelInfo.Type, channelInfo.UUID, channelInfo.Name, costMs)
			}
		}(ch)
	}

	wg.Wait()

	if len(errors) > 0 {
		return errors[0]
	}
	return nil
}

// sendNotificationToChannels 并发发送通知到所有渠道
func (m *manager) sendNotificationToChannels(ctx context.Context, channels []*ChannelInfo, opts *NotificationOptions) error {
	var wg sync.WaitGroup
	var mu sync.Mutex
	var errors []error

	for _, ch := range channels {
		// 原代码:
		// if ch.Type == AlertTypeSiteMessage {
		// 	m.l.Infof("[通知发送] 跳过站内信渠道（已统一处理）| UUID: %s", ch.UUID)
		// 	continue
		// }

		wg.Add(1)
		go func(channelInfo *ChannelInfo) {
			defer wg.Done()

			startTime := time.Now()

			m.l.Infof("[通知发送] 开始 | 类型: %s | UUID: %s | 名称: %s",
				channelInfo.Type, channelInfo.UUID, channelInfo.Name)

			client, err := m.getOrCreateChannelClient(ctx, channelInfo)
			if err != nil {
				mu.Lock()
				errors = append(errors, fmt.Errorf("渠道 %s(%s) 获取客户端失败: %w", channelInfo.Name, channelInfo.UUID, err))
				mu.Unlock()
				return
			}

			notifChannel, ok := client.(NotificationChannel)
			if !ok {
				mu.Lock()
				errors = append(errors, fmt.Errorf("渠道 %s(%s) 不支持通用通知", channelInfo.Name, channelInfo.UUID))
				mu.Unlock()
				return
			}

			result, err := notifChannel.SendNotification(ctx, opts)
			costMs := time.Since(startTime).Milliseconds()

			if err != nil {
				mu.Lock()
				errors = append(errors, fmt.Errorf("渠道 %s(%s) 发送失败: %w", channelInfo.Name, channelInfo.UUID, err))
				mu.Unlock()
				m.l.Errorf("[通知发送] 失败 | 类型: %s | UUID: %s | 名称: %s | 耗时: %dms | 错误: %v",
					channelInfo.Type, channelInfo.UUID, channelInfo.Name, costMs, err)
				return
			}

			if result != nil && result.Success {
				m.l.Infof("[通知发送] 成功 | 类型: %s | UUID: %s | 名称: %s | 耗时: %dms",
					channelInfo.Type, channelInfo.UUID, channelInfo.Name, costMs)
			}
		}(ch)
	}

	wg.Wait()

	if len(errors) > 0 {
		return errors[0]
	}
	return nil
}

// getOrCreateChannelClient 获取或创建渠道客户端
func (m *manager) getOrCreateChannelClient(ctx context.Context, channelInfo *ChannelInfo) (Client, error) {
	m.mu.RLock()
	wrapper, exists := m.channels[channelInfo.UUID]
	m.mu.RUnlock()

	if exists && wrapper.client != nil {
		wrapper.mu.Lock()
		wrapper.lastAccess = time.Now()
		wrapper.metrics.AccessCount++
		wrapper.mu.Unlock()
		return wrapper.client.WithContext(ctx), nil
	}

	config, err := m.parseChannelConfig(channelInfo)
	if err != nil {
		return nil, err
	}

	if err := m.AddChannel(*config); err != nil {
		m.mu.RLock()
		wrapper, exists = m.channels[channelInfo.UUID]
		m.mu.RUnlock()
		if exists && wrapper.client != nil {
			return wrapper.client.WithContext(ctx), nil
		}
		return nil, err
	}

	m.mu.RLock()
	wrapper, exists = m.channels[channelInfo.UUID]
	m.mu.RUnlock()

	if !exists || wrapper.client == nil {
		return nil, fmt.Errorf("创建渠道客户端失败")
	}

	return wrapper.client.WithContext(ctx), nil
}

// recordNotification 记录通知日志
func (m *manager) recordNotification(ctx context.Context, alerts []*AlertInstance, groupID uint64, channel *ChannelInfo, opts *AlertOptions, result *SendResult, sendErr error, costMs int64) {
	if m.alertNotificationsModel == nil {
		return
	}

	status := "success"
	errMsg := ""
	if sendErr != nil {
		status = "failed"
		errMsg = sendErr.Error()
	} else if result != nil && !result.Success {
		status = "failed"
		errMsg = result.Message
	}

	// 构建收件人 JSON
	//recipients := "[]"
	//if opts.Mentions != nil {
	//	recipientData, err := json.Marshal(opts.Mentions)
	//	if err == nil {
	//		recipients = string(recipientData)
	//	}
	//}
	//Recipients:  recipients,
	notification := &model.AlertNotifications{
		Uuid:        uuid.New().String(),
		InstanceId:  0,
		GroupId:     groupID,
		Severity:    opts.Severity,
		ChannelId:   channel.ID,
		ChannelType: string(channel.Type),
		SendFormat:  "markdown",
		Recipients:  "[]",
		Subject:     fmt.Sprintf("%s 告警通知 (%d条)", m.portalName, len(alerts)),
		Status:      status,
		ErrorMsg:    errMsg,
		SentAt:      time.Now(),
		CostMs:      uint64(costMs),
		CreatedBy:   "system",
		UpdatedBy:   "system",
	}

	if _, err := m.alertNotificationsModel.Insert(ctx, notification); err != nil {
		m.l.Errorf("[通知记录] 记录通知日志失败: %v", err)
	}
}

// ========== 站内信存储实现 ==========

// SiteMessageData 站内信数据结构
type SiteMessageData struct {
	UUID           string    `json:"uuid"`
	NotificationID uint64    `json:"notificationId"`
	InstanceID     uint64    `json:"instanceId"`
	UserID         uint64    `json:"userId"`
	Title          string    `json:"title"`
	Content        string    `json:"content"`
	MessageType    string    `json:"messageType"` // notification, alert, announcement
	Severity       string    `json:"severity"`    // info, warning, critical
	Category       string    `json:"category"`    // system, alert, message
	ExtraData      string    `json:"extraData"`   // JSON 格式扩展数据
	ActionURL      string    `json:"actionUrl"`
	ActionText     string    `json:"actionText"`
	IsRead         int64     `json:"isRead"`
	ReadAt         time.Time `json:"readAt"`
	IsStarred      int64     `json:"isStarred"`
	ExpireAt       time.Time `json:"expireAt"`
}

type siteMessageStoreImpl struct {
	m *manager
}

func (s *siteMessageStoreImpl) SaveMessage(ctx context.Context, msg *SiteMessageData) error {
	if s.m.siteMessagesModel == nil {
		return fmt.Errorf("站内信模型未初始化")
	}

	// 确保 ExtraData 是有效的 JSON
	extraData := msg.ExtraData
	if extraData == "" {
		extraData = "{}"
	}

	// 处理 ReadAt 零值问题
	readAt := msg.ReadAt
	if readAt.IsZero() {
		readAt = time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)
	}

	// 处理 ExpireAt 零值问题
	expireAt := msg.ExpireAt
	if expireAt.IsZero() {
		expireAt = time.Now().AddDate(0, 1, 0)
	}

	siteMsg := &model.SiteMessages{
		Uuid:           msg.UUID,
		NotificationId: msg.NotificationID,
		InstanceId:     msg.InstanceID,
		UserId:         msg.UserID,
		Title:          msg.Title,
		Content:        msg.Content,
		MessageType:    msg.MessageType,
		Severity:       msg.Severity,
		Category:       msg.Category,
		ExtraData:      extraData,
		ActionUrl:      msg.ActionURL,
		ActionText:     msg.ActionText,
		IsRead:         msg.IsRead,
		ReadAt:         readAt,
		IsStarred:      msg.IsStarred,
		ExpireAt:       expireAt,
	}

	_, err := s.m.siteMessagesModel.Insert(ctx, siteMsg)
	return err
}

func (s *siteMessageStoreImpl) SaveBatchMessages(ctx context.Context, msgs []*SiteMessageData) error {
	for _, msg := range msgs {
		if err := s.SaveMessage(ctx, msg); err != nil {
			s.m.l.Errorf("[站内信] 保存失败: userID=%d, err=%v", msg.UserID, err)
		}
	}
	return nil
}

// ChannelInfo 渠道信息
type ChannelInfo struct {
	ID      uint64        `json:"id"`
	UUID    string        `json:"uuid"`
	Name    string        `json:"name"`
	Type    AlertType     `json:"type"`
	Config  string        `json:"config"`
	Options ClientOptions `json:"options,omitempty"`
}

// UserInfo 用户信息
type UserInfo struct {
	UserID   uint64 `json:"userId"`
	Username string `json:"username"`
	Phone    string `json:"phone"`
	Email    string `json:"email"`
}
