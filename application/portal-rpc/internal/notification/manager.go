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
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

// DefaultAlertGroupID 默认告警组ID
// 当项目未绑定告警组时，使用此默认告警组
const DefaultAlertGroupID uint64 = 2

// ManagerConfig 管理器配置
// 包含所有管理器初始化所需的依赖和配置
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
// 负责管理所有告警渠道，处理告警分发和通知
type manager struct {
	// channels 告警客户端存储，key 为渠道 UUID
	channels map[string]*channelWrapper
	// mu 保护 channels 和 closed 的读写锁
	mu sync.RWMutex
	// messagePusher 消息推送服务，用于 WebSocket 实时推送
	messagePusher *MessagePushService

	// portalName 平台名称，显示在告警消息中
	portalName string
	// portalUrl 平台 URL，用于告警详情链接
	portalUrl string

	// l 日志记录器
	l logx.Logger

	// 数据库模型 - 用于查询告警组、渠道、用户等信息
	sysUserModel                 model.SysUserModel
	alertChannelsModel           model.AlertChannelsModel
	alertGroupsModel             model.AlertGroupsModel
	alertGroupMembersModel       model.AlertGroupMembersModel
	alertGroupLevelChannelsModel model.AlertGroupLevelChannelsModel
	alertNotificationsModel      model.AlertNotificationsModel
	alertGroupAppsModel          model.AlertGroupAppsModel
	siteMessagesModel            model.SiteMessagesModel

	// closed 管理器关闭状态
	closed bool

	// statsMu 保护统计信息的读写锁
	statsMu sync.RWMutex

	// aggregator 告警聚合器，用于按时间窗口聚合告警
	aggregator *AlertAggregator

	// pushWg 追踪异步推送 goroutine，确保优雅关闭
	pushWg sync.WaitGroup
}

// channelWrapper 渠道包装器
// 包装渠道客户端并记录访问统计
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

	// 初始化消息推送服务
	if config.Redis != nil {
		m.messagePusher = NewMessagePushService(config.Redis)
		m.l.Info("[告警管理器] 消息推送服务已启用")
	} else {
		m.l.Errorf("[告警管理器] Redis 未配置，消息推送功能不可用")
	}

	// 初始化聚合器 - 优化初始化逻辑
	if config.Redis != nil {
		// 如果没有提供聚合器配置，使用默认配置
		aggregatorConfig := DefaultAggregatorConfig()
		if config.AggregatorConfig != nil {
			aggregatorConfig = *config.AggregatorConfig
		} else {
			m.l.Info("[告警管理器] 未提供聚合器配置，使用默认配置")
		}

		m.aggregator = NewAlertAggregatorWithManager(config.Redis, m, aggregatorConfig)
		if m.aggregator != nil {
			m.l.Infof("[告警管理器] 聚合器已初始化 | 启用=%v | Critical=%v | Warning=%v | Info=%v | Default=%v",
				aggregatorConfig.Enabled,
				aggregatorConfig.SeverityWindows.Critical,
				aggregatorConfig.SeverityWindows.Warning,
				aggregatorConfig.SeverityWindows.Info,
				aggregatorConfig.SeverityWindows.Default,
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
				return nil, errorx.Msg(fmt.Sprintf("告警渠道 %s 不存在", uuid))
			}

			// 解析配置并添加渠道
			config, err := m.parseChannelFromDB(channelData)
			if err != nil {
				return nil, errorx.Msg(fmt.Sprintf("解析渠道配置失败: %v", err))
			}

			if err := m.AddChannel(*config); err != nil {
				return nil, errorx.Msg(fmt.Sprintf("添加渠道失败: %v", err))
			}

			m.mu.RLock()
			wrapper, exists = m.channels[uuid]
			m.mu.RUnlock()

			if !exists {
				return nil, errorx.Msg("渠道添加后仍不存在")
			}
		} else {
			return nil, errorx.Msg(fmt.Sprintf("告警渠道 %s 不存在", uuid))
		}
	}

	// 更新访问时间
	wrapper.mu.Lock()
	wrapper.lastAccess = time.Now()
	wrapper.metrics.AccessCount++
	wrapper.mu.Unlock()

	if wrapper.client == nil {
		return nil, errorx.Msg(fmt.Sprintf("告警渠道 %s 未初始化", uuid))
	}

	return wrapper.client.WithContext(ctx), nil
}

// AddChannel 添加告警渠道
func (m *manager) AddChannel(config Config) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return errorx.Msg("告警管理器已关闭")
	}

	if err := validateConfig(config); err != nil {
		return errorx.Msg(fmt.Sprintf("告警配置验证失败: %v", err))
	}

	if _, exists := m.channels[config.UUID]; exists {
		return nil // 已存在，跳过
	}

	client, err := m.createClient(config)
	if err != nil {
		return errorx.Msg(fmt.Sprintf("创建告警渠道失败: %v", err))
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
		return nil, errorx.Msg(fmt.Sprintf("不支持的告警类型: %s", config.Type))
	}
}

// RemoveChannel 移除告警渠道
func (m *manager) RemoveChannel(uuid string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	wrapper, exists := m.channels[uuid]
	if !exists {
		return errorx.Msg(fmt.Sprintf("告警渠道 %s 不存在", uuid))
	}

	if wrapper.client != nil {
		if err := wrapper.client.Close(); err != nil {
			m.l.Errorf("[渠道管理] 关闭渠道失败: %s, 错误: %v", uuid, err)
		}
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
		return errorx.Msg(fmt.Sprintf("告警渠道 %s 不存在", config.UUID))
	}

	if wrapper.client != nil {
		if err := wrapper.client.Close(); err != nil {
			m.l.Errorf("[渠道管理] 更新渠道时关闭旧客户端失败: %s, 错误: %v", config.UUID, err)
		}
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
// 修复: 避免在持有锁时调用可能需要锁的方法，防止死锁
func (m *manager) Close() error {
	// 首先检查并设置关闭状态
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return nil
	}
	m.closed = true

	// 获取需要关闭的资源引用，然后释放锁
	// 这样在调用 aggregator 和 channel 的方法时不会持有锁
	agg := m.aggregator
	channels := make(map[string]*channelWrapper)
	for k, v := range m.channels {
		channels[k] = v
	}
	m.mu.Unlock()

	// 等待异步推送完成
	m.l.Info("[告警管理器] 等待异步推送完成...")
	waitDone := make(chan struct{})
	go func() {
		m.pushWg.Wait()
		close(waitDone)
	}()

	select {
	case <-waitDone:
		m.l.Info("[告警管理器] 所有异步推送已完成")
	case <-time.After(10 * time.Second):
		m.l.Info("[告警管理器] 等待异步推送超时，继续关闭")
	}

	// 处理聚合器（不持有锁，避免死锁）
	if agg != nil {
		m.l.Info("[告警管理器] 正在刷新聚合器缓冲...")
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		agg.FlushAll(ctx)
		cancel()
		agg.Stop()
		m.l.Info("[告警管理器] 聚合器已停止")
	}

	// 关闭所有渠道
	for uuid, wrapper := range channels {
		if wrapper.client != nil {
			if err := wrapper.client.Close(); err != nil {
				m.l.Errorf("关闭告警渠道 %s 失败: %v", uuid, err)
			}
		}
	}

	// 清理内部状态
	m.mu.Lock()
	m.channels = make(map[string]*channelWrapper)
	m.mu.Unlock()

	m.l.Info("告警管理器已关闭")
	return nil
}

// alertGroupInfo 告警分组信息
// 用于按项目、级别和告警名称分组告警
type alertGroupInfo struct {
	ProjectID   uint64
	ProjectName string
	ClusterName string
	Severity    string
	AlertName   string
	Alerts      []*AlertInstance
}

// groupAlertsByProjectSeverityAndName 按项目、级别和告警名称分组
// 同一个告警名称的多个实例会被聚合到同一个分组
func (m *manager) groupAlertsByProjectSeverityAndName(alerts []*AlertInstance) map[string]*alertGroupInfo {
	groups := make(map[string]*alertGroupInfo)

	for _, alert := range alerts {
		// 分组键包含 alertName，确保同类告警聚合在一起
		key := fmt.Sprintf("%d-%s-%s", alert.ProjectID, alert.Severity, alert.AlertName)

		if _, exists := groups[key]; !exists {
			groups[key] = &alertGroupInfo{
				ProjectID:   alert.ProjectID,
				ProjectName: alert.ProjectName,
				ClusterName: alert.ClusterName,
				Severity:    alert.Severity,
				AlertName:   alert.AlertName,
				Alerts:      make([]*AlertInstance, 0),
			}
		}

		groups[key].Alerts = append(groups[key].Alerts, alert)

		// 更新项目名称和集群名称
		if groups[key].ProjectName == "" && alert.ProjectName != "" {
			groups[key].ProjectName = alert.ProjectName
		}
		if groups[key].ClusterName == "" && alert.ClusterName != "" {
			groups[key].ClusterName = alert.ClusterName
		}
	}

	return groups
}

// PrometheusAlertNotification 发送 Prometheus 告警通知
// 这是告警处理的核心入口方法
func (m *manager) PrometheusAlertNotification(ctx context.Context, alerts []*AlertInstance) error {
	if len(alerts) == 0 {
		m.l.Infof("[告警通知] 告警列表为空，跳过处理")
		return nil
	}

	m.l.Infof("[告警通知] 开始处理 %d 条告警", len(alerts))

	// 统一使用聚合器处理所有告警（聚合器内部会根据配置决定是否聚合）
	if m.aggregator != nil {
		m.l.Info("[告警通知] 使用聚合器统一处理")
		return m.aggregator.AddAlert(ctx, alerts)
	}

	// 如果聚合器未初始化，降级到直接发送（但记录警告）
	m.l.Errorf("[告警通知] 聚合器未初始化，降级到直接发送")

	// 按项目、级别和告警名称分组
	groups := m.groupAlertsByProjectSeverityAndName(alerts)
	m.l.Infof("[告警通知] 分组完成，共 %d 个分组", len(groups))

	var wg sync.WaitGroup
	var mu sync.Mutex
	var errors []error

	for key, group := range groups {
		wg.Add(1)
		go func(groupKey string, groupInfo *alertGroupInfo) {
			defer wg.Done()

			m.l.Infof("[告警通知] 处理分组: %s | 告警名称: %s | 实例数: %d",
				groupKey, groupInfo.AlertName, len(groupInfo.Alerts))

			if err := m.processAlertGroup(ctx, groupInfo); err != nil {
				mu.Lock()
				errors = append(errors, errorx.Msg(fmt.Sprintf("分组 %s 处理失败: %v", groupKey, err)))
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

// SendAlertsDirectly 实现 InternalManager 接口，供聚合器调用
// 直接发送告警，绕过聚合器
// 优化：实现按渠道维度的二次聚合，解决相同渠道分别发送导致的频率限制问题
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

	// 新的渠道维度聚合逻辑：
	// 1. 先按项目+级别分组（获取告警组配置）
	// 2. 然后将相同渠道的告警合并后批量发送
	return m.sendAlertsWithChannelAggregation(ctx, alerts)
}

// sendAlertsWithChannelAggregation 按渠道维度聚合发送告警
// 核心思想：在同一个告警组内，如果不同级别使用了相同的渠道，则将这些告警聚合后一起发送
// 这样可以避免同一告警组的不同级别告警分别发送到同一渠道导致的频率限制问题
func (m *manager) sendAlertsWithChannelAggregation(ctx context.Context, alerts []*AlertInstance) error {
	// 第一步：按项目分组（不区分级别）
	projectGroups := m.groupAlertsByProjectAndSeverity(alerts)

	// 第二步：按告警组维度收集渠道和告警
	// groupChannelKey = "groupID:channelUUID"
	type groupChannelData struct {
		groupID     uint64
		channelUUID string
		channelInfo *ChannelInfo
		alerts      []*AlertInstance
		projectID   uint64
		projectName string
		clusterName string
		mentions    *MentionConfig
		severities  map[string]bool // 记录包含的级别
	}

	groupChannelMap := make(map[string]*groupChannelData)

	for _, group := range projectGroups {
		// 解析告警组
		groupID, err := m.resolveAlertGroup(ctx, group.ProjectID)
		if err != nil {
			m.l.Errorf("[渠道聚合] 解析告警组失败: ProjectID=%d, error=%v", group.ProjectID, err)
			continue
		}

		// 收集该项目所有告警的级别
		severitiesInGroup := make(map[string]bool)
		for _, alert := range group.Alerts {
			severitiesInGroup[alert.Severity] = true
		}

		// 获取所有级别的渠道并合并（去重）
		channelMap := make(map[string]*ChannelInfo) // UUID -> ChannelInfo
		for severity := range severitiesInGroup {
			channels, err := m.getChannelsForSeverity(ctx, groupID, severity)
			if err != nil {
				m.l.Errorf("[渠道聚合] 获取渠道失败: GroupID=%d, Severity=%s, error=%v", groupID, severity, err)
				continue
			}

			// 合并渠道（按 UUID 去重）
			for _, ch := range channels {
				channelMap[ch.UUID] = ch
			}
		}

		if len(channelMap) == 0 {
			m.l.Infof("[渠道聚合] 告警组 %d 没有配置渠道，跳过", groupID)
			continue
		}

		// 获取告警组成员用于 @ 人
		mentions, err := m.resolveMentions(ctx, groupID, nil)
		if err != nil {
			m.l.Errorf("[渠道聚合] 解析 @ 人配置失败: %v", err)
			// 继续发送但不 @ 人
		}

		// 将该项目的所有告警分配到每个渠道
		for _, ch := range channelMap {
			// 使用 groupID + channelUUID 作为唯一键
			groupChannelKey := fmt.Sprintf("%d:%s", groupID, ch.UUID)

			if _, exists := groupChannelMap[groupChannelKey]; !exists {
				groupChannelMap[groupChannelKey] = &groupChannelData{
					groupID:     groupID,
					channelUUID: ch.UUID,
					channelInfo: ch,
					alerts:      make([]*AlertInstance, 0),
					projectID:   group.ProjectID,
					projectName: group.ProjectName,
					clusterName: group.ClusterName,
					mentions:    mentions,
					severities:  make(map[string]bool),
				}
			}

			data := groupChannelMap[groupChannelKey]

			// 将所有告警添加到该渠道（避免重复添加）
			for _, alert := range group.Alerts {
				data.severities[alert.Severity] = true

				found := false
				for _, existing := range data.alerts {
					if existing.Fingerprint == alert.Fingerprint {
						found = true
						break
					}
				}
				if !found {
					data.alerts = append(data.alerts, alert)
				}
			}
		}
	}

	// 第三步：按告警组+渠道批量发送
	m.l.Infof("[渠道聚合] 共聚合到 %d 个告警组-渠道组合，开始批量发送", len(groupChannelMap))

	var wg sync.WaitGroup
	var mu sync.Mutex
	var errors []error

	for groupChannelKey, data := range groupChannelMap {
		if len(data.alerts) == 0 {
			continue
		}

		wg.Add(1)
		go func(key string, gcd *groupChannelData) {
			defer wg.Done()

			// 获取渠道客户端
			client, err := m.GetChannel(ctx, gcd.channelUUID)
			if err != nil {
				mu.Lock()
				errors = append(errors, errorx.Msg(fmt.Sprintf("获取渠道 %s 失败: %v", gcd.channelUUID, err)))
				mu.Unlock()
				return
			}

			// 构建聚合告警组
			aggregatedGroup := m.buildAggregatedAlertGroup(gcd.alerts, gcd.projectID, gcd.projectName)

			// 构建告警选项
			opts := &AlertOptions{
				PortalName:  m.portalName,
				PortalUrl:   m.portalUrl,
				Mentions:    gcd.mentions,
				ProjectName: gcd.projectName,
				ClusterName: gcd.clusterName,
				Severity:    "mixed", // 标记为混合级别
			}

			// 发送到渠道（使用聚合格式化方法）
			if fullCh, ok := client.(FullChannel); ok {
				var result *SendResult
				var err error

				// 根据渠道类型调用对应的聚合格式化方法
				switch gcd.channelInfo.Type {
				case AlertTypeDingTalk:
					// 钉钉使用聚合格式化
					result, err = m.sendAggregatedAlertToDingTalk(ctx, fullCh, opts, aggregatedGroup)
				case AlertTypeWeChat:
					// 企业微信使用聚合格式化
					result, err = m.sendAggregatedAlertToWeChat(ctx, fullCh, opts, aggregatedGroup)
				case AlertTypeFeiShu:
					// 飞书使用聚合格式化
					result, err = m.sendAggregatedAlertToFeiShu(ctx, fullCh, opts, aggregatedGroup)
				case AlertTypeEmail:
					// 邮件使用聚合格式化
					result, err = m.sendAggregatedAlertToEmail(ctx, fullCh, opts, aggregatedGroup)
				default:
					// 其他渠道使用原有方法
					result, err = fullCh.SendPrometheusAlert(ctx, opts, gcd.alerts)
				}

				if err != nil {
					mu.Lock()
					errors = append(errors, errorx.Msg(fmt.Sprintf("渠道 %s 发送失败: %v", gcd.channelUUID, err)))
					mu.Unlock()
					m.l.Errorf("[渠道聚合] 发送失败: GroupID=%d, ChannelUUID=%s, Count=%d, Error=%v",
						gcd.groupID, gcd.channelUUID, len(gcd.alerts), err)
				} else {
					m.l.Infof("[渠道聚合] 发送成功: GroupID=%d, ChannelUUID=%s, Count=%d, Result=%s",
						gcd.groupID, gcd.channelUUID, len(gcd.alerts), result.Message)
				}

				// 记录通知日志
				m.recordNotification(ctx, gcd.alerts, gcd.groupID, gcd.channelInfo, opts, result, err, 0)
			}
		}(groupChannelKey, data)
	}

	wg.Wait()

	// 处理站内信（需要单独处理，因为站内信是按用户创建的）
	if err := m.createSiteMessagesForAggregatedAlerts(ctx, alerts, projectGroups); err != nil {
		m.l.Errorf("[渠道聚合] 创建站内信失败: %v", err)
		// 站内信失败不影响整体流程
	}

	if len(errors) > 0 {
		m.l.Errorf("[渠道聚合] 完成，但有 %d 个渠道发送失败", len(errors))
		return errors[0]
	}

	m.l.Infof("[渠道聚合] 全部发送完成")
	return nil
}

// createSiteMessagesForAggregatedAlerts 为聚合后的告警创建站内信
// 修复: 使用 InternalUserIds 而不是 AtUserIds，避免类型转换
func (m *manager) createSiteMessagesForAggregatedAlerts(ctx context.Context, allAlerts []*AlertInstance, groups map[string]*alertGroupInfo) error {
	if m.siteMessagesModel == nil {
		return nil
	}

	// 收集所有需要接收站内信的用户ID
	userAlerts := make(map[uint64][]*AlertInstance)
	mentionsMap := make(map[uint64]*MentionConfig)
	projectsMap := make(map[uint64]string)
	clustersMap := make(map[uint64]string)
	severityMap := make(map[uint64]string)

	for _, group := range groups {
		// 解析告警组
		groupID, err := m.resolveAlertGroup(ctx, group.ProjectID)
		if err != nil {
			continue
		}

		mentions, err := m.resolveMentions(ctx, groupID, nil)
		if err != nil || mentions == nil {
			continue
		}

		// 优先使用 InternalUserIds
		var userIds []uint64
		if len(mentions.InternalUserIds) > 0 {
			userIds = mentions.InternalUserIds
		} else if len(mentions.AtUserIds) > 0 {
			// 向后兼容：如果 InternalUserIds 为空，尝试使用 AtUserIds
			for _, userIdStr := range mentions.AtUserIds {
				var userId uint64
				if _, err := fmt.Sscanf(userIdStr, "%d", &userId); err != nil {
					m.l.Errorf("[站内信] 用户ID转换失败: %s, 错误: %v", userIdStr, err)
					continue
				}
				userIds = append(userIds, userId)
			}
		}

		// 为每个用户聚合告警
		for _, userId := range userIds {
			userAlerts[userId] = append(userAlerts[userId], group.Alerts...)
			mentionsMap[userId] = mentions
			projectsMap[userId] = group.ProjectName
			clustersMap[userId] = group.ClusterName
			severityMap[userId] = group.Severity
		}
	}

	// 为每个用户创建站内信
	formatter := NewMessageFormatter(m.portalName, m.portalUrl)
	expireAt := time.Now().AddDate(0, 0, 30)
	successCount := 0
	failedCount := 0

	for userId, alerts := range userAlerts {
		if len(alerts) == 0 {
			continue
		}

		// 按告警名称分组
		alertsByName := make(map[string][]*AlertInstance)
		for _, alert := range alerts {
			alertsByName[alert.AlertName] = append(alertsByName[alert.AlertName], alert)
		}

		// 为每个告警名称创建一条消息
		for alertName, alertGroup := range alertsByName {
			title := alertName
			if len(alertGroup) > 1 {
				title = fmt.Sprintf("%s (%d个实例)", alertName, len(alertGroup))
			}

			opts := &AlertOptions{
				PortalName:  m.portalName,
				PortalUrl:   m.portalUrl,
				Mentions:    mentionsMap[userId],
				ProjectName: projectsMap[userId],
				ClusterName: clustersMap[userId],
				Severity:    severityMap[userId],
			}

			content := m.buildAggregatedAlertContent(formatter, alertGroup, opts)

			firstAlert := alertGroup[0]
			msgUUID := uuid.New().String()

			siteMsg := &model.SiteMessages{
				Uuid:           msgUUID,
				NotificationId: 0,
				InstanceId:     firstAlert.ID,
				UserId:         userId,
				Title:          title,
				Content:        content,
				MessageType:    "alert",
				Severity:       firstAlert.Severity,
				Category:       "prometheus",
				ExtraData:      "{}",
				ActionUrl:      firstAlert.GeneratorURL,
				ActionText:     "查看详情",
				IsRead:         0,
				IsStarred:      0,
				ExpireAt:       expireAt,
			}

			if _, err := m.siteMessagesModel.Insert(ctx, siteMsg); err != nil {
				failedCount++
				m.l.Errorf("[站内信] 创建失败: UserID=%d, Error=%v", userId, err)
				continue
			}

			successCount++

			// 推送到 WebSocket
			if m.messagePusher != nil {
				siteMessageData := &SiteMessageData{
					UUID:        msgUUID,
					UserID:      userId,
					InstanceID:  firstAlert.ID,
					Title:       title,
					Content:     content,
					MessageType: "alert",
					Severity:    firstAlert.Severity,
					Category:    "prometheus",
					ActionURL:   firstAlert.GeneratorURL,
					ActionText:  "查看详情",
				}
				m.messagePusher.PushMessage(ctx, siteMessageData)
			}
		}
	}

	m.l.Infof("[站内信] 聚合告警创建完成 | 成功: %d | 失败: %d", successCount, failedCount)

	return nil
}

// groupAlertsByProjectAndSeverity 按项目ID分组告警（不区分级别）
// 修改为支持跨级别聚合
func (m *manager) groupAlertsByProjectAndSeverity(alerts []*AlertInstance) map[string]*alertGroupInfo {
	groups := make(map[string]*alertGroupInfo)

	for _, alert := range alerts {
		// 生成分组 key: 只按 projectID 分组，不区分 severity
		key := fmt.Sprintf("%d", alert.ProjectID)

		if _, exists := groups[key]; !exists {
			groups[key] = &alertGroupInfo{
				ProjectID:   alert.ProjectID,
				ProjectName: alert.ProjectName,
				ClusterName: alert.ClusterName,
				Severity:    "mixed", // 标记为混合级别
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
		return errorx.Msg(fmt.Sprintf("解析告警组失败: %v", err))
	}
	m.l.Infof("[告警分组] ProjectID: %d -> GroupID: %d", group.ProjectID, groupID)

	// 2. 获取对应级别的渠道列表
	channels, err := m.getChannelsForSeverity(ctx, groupID, group.Severity)
	if err != nil {
		return errorx.Msg(fmt.Sprintf("获取渠道失败: %v", err))
	}
	if len(channels) == 0 {
		m.l.Infof("[告警分组] 告警组 %d 级别 %s 没有配置渠道，跳过", groupID, group.Severity)
		return nil
	}
	m.l.Infof("[告警分组] 获取到 %d 个渠道", len(channels))

	// 3. 获取告警组成员用于 @ 人
	mentions, err := m.resolveMentions(ctx, groupID, nil)
	if err != nil {
		m.l.Errorf("[告警分组] 解析 @ 人配置失败: %v，将继续发送但不 @ 人", err)
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

	// 6. 如果渠道列表中没有站内信渠道，则单独创建站内信
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

// createSiteMessagesForAlerts 为告警创建站内信
// 修复: 使用 InternalUserIds 而不是 AtUserIds，避免类型转换
func (m *manager) createSiteMessagesForAlerts(ctx context.Context, alerts []*AlertInstance, mentions *MentionConfig, opts *AlertOptions) error {
	if m.siteMessagesModel == nil || mentions == nil {
		return nil
	}

	if len(alerts) == 0 {
		return nil
	}

	// 优先使用 InternalUserIds
	var userIds []uint64
	if len(mentions.InternalUserIds) > 0 {
		userIds = mentions.InternalUserIds
	} else if len(mentions.AtUserIds) > 0 {
		// 向后兼容：如果 InternalUserIds 为空，尝试使用 AtUserIds
		m.l.Infof("[站内信] 使用向后兼容模式: AtUserIds")
		for _, userIdStr := range mentions.AtUserIds {
			var userId uint64
			if _, err := fmt.Sscanf(userIdStr, "%d", &userId); err != nil {
				m.l.Errorf("[站内信] 用户ID转换失败: %s, 错误: %v", userIdStr, err)
				continue
			}
			userIds = append(userIds, userId)
		}
	}

	if len(userIds) == 0 {
		m.l.Infof("[站内信] 没有有效的用户ID，跳过创建站内信")
		return nil
	}

	formatter := NewMessageFormatter(opts.PortalName, opts.PortalUrl)
	expireAt := time.Now().AddDate(0, 0, 30)

	firstAlert := alerts[0]

	// 构建标题，多实例时显示数量
	title := firstAlert.AlertName
	if len(alerts) > 1 {
		title = fmt.Sprintf("%s (%d个实例)", firstAlert.AlertName, len(alerts))
	}

	// 构建聚合内容
	content := m.buildAggregatedAlertContent(formatter, alerts, opts)

	// 为每个用户创建一条聚合消息
	successCount := 0
	failedCount := 0

	for _, userId := range userIds {
		msgUUID := uuid.New().String()

		// 确保 ExtraData 是有效的 JSON
		extraData := fmt.Sprintf(`{"instance_count":%d,"alert_name":"%s"}`, len(alerts), firstAlert.AlertName)
		if extraData == "" {
			extraData = "{}"
		}

		siteMsg := &model.SiteMessages{
			Uuid:           msgUUID,
			NotificationId: 0,
			InstanceId:     firstAlert.ID,
			UserId:         userId,
			Title:          title,
			Content:        content,
			MessageType:    "alert",
			Severity:       firstAlert.Severity,
			Category:       "prometheus",
			ExtraData:      extraData,
			ActionUrl:      opts.PortalUrl,
			ActionText:     "查看详情",
			IsRead:         0,
			IsStarred:      0,
			ExpireAt:       expireAt,
		}

		result, err := m.siteMessagesModel.Insert(ctx, siteMsg)
		if err != nil {
			failedCount++
			m.l.Errorf("[站内信] 创建失败: alertName=%s, userID=%d, instanceCount=%d, err=%v",
				firstAlert.AlertName, userId, len(alerts), err)
			continue
		}

		successCount++

		// 推送消息到 Redis，使用 WaitGroup 追踪
		if m.messagePusher != nil {
			msgID, _ := result.LastInsertId()
			pushData := &SiteMessageData{
				UUID:        msgUUID,
				UserID:      userId,
				Title:       title,
				Content:     content,
				MessageType: "alert",
				Severity:    firstAlert.Severity,
				Category:    "prometheus",
				ActionURL:   opts.PortalUrl,
				ActionText:  "查看详情",
				IsRead:      0,
				InstanceID:  firstAlert.ID,
			}
			_ = msgID

			m.pushWg.Add(1) // 追踪异步推送
			go func(data *SiteMessageData, alertName string, uid uint64, count int) {
				defer m.pushWg.Done()

				pushCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()

				if err := m.messagePusher.PushMessage(pushCtx, data); err != nil {
					m.l.Errorf("[站内信] Redis推送失败: alertName=%s, userID=%d, err=%v", alertName, uid, err)
				} else {
					m.l.Infof("[站内信] 推送成功: alertName=%s, userID=%d, 实例数=%d", alertName, uid, count)
				}
			}(pushData, firstAlert.AlertName, userId, len(alerts))
		}
	}

	m.l.Infof("[站内信] 创建完成 | 成功: %d | 失败: %d | 告警: %s", successCount, failedCount, firstAlert.AlertName)

	return nil
}

// DefaultNotification 发送通用通知（如创建账号等系统通知）
// 查询默认告警组(ID=2)的 notification 级别渠道，没有则用 default
func (m *manager) DefaultNotification(ctx context.Context, userIDs []uint64, title, content string) error {
	if len(userIDs) == 0 {
		return errorx.Msg("用户ID列表为空")
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
		return errorx.Msg(fmt.Sprintf("获取用户信息失败: %v", err))
	}

	// 4. 构建 @ 人配置
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

	// 7. 如果渠道列表中没有站内信渠道，则单独创建站内信
	if !m.hasChannelType(channels, AlertTypeSiteMessage) {
		if err := m.createSiteMessagesForNotification(ctx, userIDs, title, content); err != nil {
			m.l.Errorf("[通用通知] 创建站内信失败: %v", err)
		}
	}

	return nil
}

// createSiteMessagesForNotification 为通知创建站内信
// 修复: 直接使用 uint64 类型的 userIDs，避免类型转换
func (m *manager) createSiteMessagesForNotification(ctx context.Context, userIDs []uint64, title, content string) error {
	if m.siteMessagesModel == nil {
		return nil
	}

	if len(userIDs) == 0 {
		m.l.Infof("[站内信] 没有有效的用户ID，跳过创建站内信")
		return nil
	}

	expireAt := time.Now().AddDate(0, 0, 30)
	successCount := 0
	failedCount := 0

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
			IsStarred:      0,
			ExpireAt:       expireAt,
		}

		result, err := m.siteMessagesModel.Insert(ctx, siteMsg)
		if err != nil {
			failedCount++
			m.l.Errorf("[站内信] 创建失败: userID=%d, err=%v", userId, err)
			continue
		}

		successCount++

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

			// 异步推送，使用 WaitGroup 追踪
			m.pushWg.Add(1)
			go func(data *SiteMessageData, uid uint64) {
				defer m.pushWg.Done()

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

	m.l.Infof("[站内信] 创建完成 | 成功: %d | 失败: %d | 标题: %s", successCount, failedCount, title)

	return nil
}

// resolveAlertGroup 解析告警组
// ProjectID=0 时返回默认告警组
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
		return nil, errorx.Msg("告警组级别渠道模型未初始化")
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
		return nil, errorx.Msg("告警渠道模型未初始化")
	}

	idStrs := strings.Split(channelIDStr, ",")
	var channels []*ChannelInfo
	failedCount := 0

	for _, idStr := range idStrs {
		idStr = strings.TrimSpace(idStr)
		if idStr == "" {
			continue
		}

		channelID, err := strconv.ParseUint(idStr, 10, 64)
		if err != nil {
			m.l.Errorf("[渠道解析] 解析渠道ID失败: %s", idStr)
			failedCount++
			continue
		}

		channel, err := m.alertChannelsModel.FindOne(ctx, channelID)
		if err != nil {
			m.l.Errorf("[渠道解析] 查询渠道失败: channelID=%d, err=%v", channelID, err)
			failedCount++
			continue
		}

		channels = append(channels, m.convertToChannelInfo(channel))
	}

	// 记录部分失败的情况
	if failedCount > 0 && len(channels) > 0 {
		m.l.Infof("[渠道解析] 部分渠道解析失败: 成功=%d, 失败=%d", len(channels), failedCount)
	}

	// 如果全部失败则返回错误
	if len(channels) == 0 && len(idStrs) > 0 {
		return nil, errorx.Msg(fmt.Sprintf("所有渠道解析失败，共 %d 个", len(idStrs)))
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

// resolveMentions 解析 @ 人配置
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
		return nil, errorx.Msg("用户模型未初始化")
	}

	var users []*UserInfo
	for _, userID := range userIDs {
		user, err := m.sysUserModel.FindOne(ctx, userID)
		if err != nil {
			m.l.Errorf("[用户查询] 查询用户失败: userID=%d, err=%v", userID, err)
			continue
		}

		// 提取平台ID（处理 sql.NullString 类型）
		dingtalkId := ""
		if user.DingtalkId.Valid && user.DingtalkId.String != "" {
			dingtalkId = user.DingtalkId.String
		}

		wechatId := ""
		if user.WechatId.Valid && user.WechatId.String != "" {
			wechatId = user.WechatId.String
		}

		feishuId := ""
		if user.FeishuId.Valid && user.FeishuId.String != "" {
			feishuId = user.FeishuId.String
		}

		users = append(users, &UserInfo{
			UserID:     user.Id,
			Username:   user.Username,
			Phone:      user.Phone,
			Email:      user.Email,
			DingtalkId: dingtalkId,
			WechatId:   wechatId,
			FeishuId:   feishuId,
		})
	}
	return users, nil
}

// buildMentionsFromUsers 从用户信息构建 @ 人配置
// 根据用户的平台ID构建对应平台的@人列表
// 修复: 明确区分内部用户ID和平台用户ID，避免混淆
func (m *manager) buildMentionsFromUsers(users []*UserInfo) *MentionConfig {
	if len(users) == 0 {
		return nil
	}

	mentions := &MentionConfig{
		InternalUserIds: make([]uint64, 0, len(users)),
		DingtalkUserIds: make([]string, 0),
		WechatUserIds:   make([]string, 0),
		FeishuUserIds:   make([]string, 0),
		EmailTo:         make([]string, 0),
		AtMobiles:       make([]string, 0),
		AtUserIds:       make([]string, 0), // 保留用于向后兼容
	}

	var missingPlatformIds []string

	for _, user := range users {
		// 保存内部用户ID（用于站内信）
		if user.UserID > 0 {
			mentions.InternalUserIds = append(mentions.InternalUserIds, user.UserID)
			// 向后兼容：同时填充 AtUserIds
			mentions.AtUserIds = append(mentions.AtUserIds, fmt.Sprintf("%d", user.UserID))
		}

		// 钉钉用户ID（如果存在则添加）
		if user.DingtalkId != "" {
			mentions.DingtalkUserIds = append(mentions.DingtalkUserIds, user.DingtalkId)
		} else if user.UserID > 0 {
			missingPlatformIds = append(missingPlatformIds, fmt.Sprintf("用户%d缺少钉钉ID", user.UserID))
		}

		// 企业微信用户ID（如果存在则添加）
		if user.WechatId != "" {
			mentions.WechatUserIds = append(mentions.WechatUserIds, user.WechatId)
		}

		// 飞书用户ID（如果存在则添加）
		if user.FeishuId != "" {
			mentions.FeishuUserIds = append(mentions.FeishuUserIds, user.FeishuId)
		}

		// 邮箱
		if user.Email != "" {
			mentions.EmailTo = append(mentions.EmailTo, user.Email)
		}

		// 手机号（已废弃，保留兼容性）
		if user.Phone != "" {
			mentions.AtMobiles = append(mentions.AtMobiles, user.Phone)
		}
	}

	// 记录缺少平台ID的用户（用于调试）
	if len(missingPlatformIds) > 0 {
		m.l.Infof("[@ 人配置] 部分用户缺少平台ID绑定: %v", missingPlatformIds)
	}

	// 统计信息
	m.l.Infof("[@ 人配置] 构建完成 | 内部用户: %d | 钉钉: %d | 企业微信: %d | 飞书: %d | 邮箱: %d",
		len(mentions.InternalUserIds),
		len(mentions.DingtalkUserIds),
		len(mentions.WechatUserIds),
		len(mentions.FeishuUserIds),
		len(mentions.EmailTo))

	return mentions
}

// sendToChannels 并发发送告警到所有渠道
func (m *manager) sendToChannels(ctx context.Context, channels []*ChannelInfo, opts *AlertOptions, alerts []*AlertInstance, groupID uint64) error {
	var wg sync.WaitGroup
	var mu sync.Mutex
	var errors []error

	for _, ch := range channels {
		wg.Add(1)
		go func(channelInfo *ChannelInfo) {
			defer wg.Done()

			startTime := time.Now()

			m.l.Infof("[渠道发送] 开始 | 类型: %s | UUID: %s | 名称: %s",
				channelInfo.Type, channelInfo.UUID, channelInfo.Name)

			client, err := m.getOrCreateChannelClient(ctx, channelInfo)
			if err != nil {
				mu.Lock()
				errors = append(errors, errorx.Msg(fmt.Sprintf("渠道 %s(%s) 获取客户端失败: %v", channelInfo.Name, channelInfo.UUID, err)))
				mu.Unlock()
				m.l.Errorf("[渠道发送] 获取客户端失败 | 类型: %s | UUID: %s | 错误: %v",
					channelInfo.Type, channelInfo.UUID, err)
				return
			}

			alertChannel, ok := client.(PrometheusAlertChannel)
			if !ok {
				mu.Lock()
				errors = append(errors, errorx.Msg(fmt.Sprintf("渠道 %s(%s) 不支持 Prometheus 告警", channelInfo.Name, channelInfo.UUID)))
				mu.Unlock()
				return
			}

			result, err := alertChannel.SendPrometheusAlert(ctx, opts, alerts)
			costMs := time.Since(startTime).Milliseconds()

			// 记录通知日志
			m.recordNotification(ctx, alerts, groupID, channelInfo, opts, result, err, costMs)

			if err != nil {
				mu.Lock()
				errors = append(errors, errorx.Msg(fmt.Sprintf("渠道 %s(%s) 发送失败: %v", channelInfo.Name, channelInfo.UUID, err)))
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
		wg.Add(1)
		go func(channelInfo *ChannelInfo) {
			defer wg.Done()

			startTime := time.Now()

			m.l.Infof("[通知发送] 开始 | 类型: %s | UUID: %s | 名称: %s",
				channelInfo.Type, channelInfo.UUID, channelInfo.Name)

			client, err := m.getOrCreateChannelClient(ctx, channelInfo)
			if err != nil {
				mu.Lock()
				errors = append(errors, errorx.Msg(fmt.Sprintf("渠道 %s(%s) 获取客户端失败: %v", channelInfo.Name, channelInfo.UUID, err)))
				mu.Unlock()
				return
			}

			notifChannel, ok := client.(NotificationChannel)
			if !ok {
				mu.Lock()
				errors = append(errors, errorx.Msg(fmt.Sprintf("渠道 %s(%s) 不支持通用通知", channelInfo.Name, channelInfo.UUID)))
				mu.Unlock()
				return
			}

			result, err := notifChannel.SendNotification(ctx, opts)
			costMs := time.Since(startTime).Milliseconds()

			if err != nil {
				mu.Lock()
				errors = append(errors, errorx.Msg(fmt.Sprintf("渠道 %s(%s) 发送失败: %v", channelInfo.Name, channelInfo.UUID, err)))
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

// buildAggregatedAlertContent 构建聚合告警内容
func (m *manager) buildAggregatedAlertContent(formatter *MessageFormatter, alerts []*AlertInstance, opts *AlertOptions) string {
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

	return content.String()
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
		return nil, errorx.Msg("创建渠道客户端失败")
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

// siteMessageStoreImpl 站内信存储实现
type siteMessageStoreImpl struct {
	m *manager
}

// SaveMessage 保存单条站内信
func (s *siteMessageStoreImpl) SaveMessage(ctx context.Context, msg *SiteMessageData) error {
	if s.m.siteMessagesModel == nil {
		return errorx.Msg("站内信模型未初始化")
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

// SaveBatchMessages 批量保存站内信
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
	UserID     uint64 `json:"userId"`
	Username   string `json:"username"`
	Phone      string `json:"phone"`
	Email      string `json:"email"`
	DingtalkId string `json:"dingtalkId"` // 钉钉用户ID
	WechatId   string `json:"wechatId"`   // 企业微信用户ID
	FeishuId   string `json:"feishuId"`   // 飞书用户ID
}

// buildAggregatedAlertGroup 构建聚合告警组
// 将告警列表转换为按级别分组的聚合结构
func (m *manager) buildAggregatedAlertGroup(alerts []*AlertInstance, projectID uint64, projectName string) *AggregatedAlertGroup {
	group := &AggregatedAlertGroup{
		ProjectID:        projectID,
		ProjectName:      projectName,
		AlertsBySeverity: make(map[string][]*AlertInstance),
		ResolvedAlerts:   make([]*AlertInstance, 0),
		ClusterStats:     make(map[string]*ClusterStat),
		TotalFiring:      0,
		TotalResolved:    0,
	}

	// 记录第一条和最后一条告警的时间
	var firstTime, lastTime time.Time

	for i, alert := range alerts {
		// 更新时间范围
		if i == 0 {
			firstTime = alert.StartsAt
			lastTime = alert.StartsAt
		} else {
			if alert.StartsAt.Before(firstTime) {
				firstTime = alert.StartsAt
			}
			if alert.StartsAt.After(lastTime) {
				lastTime = alert.StartsAt
			}
		}

		// 按状态分类
		if alert.Status == string(AlertStatusFiring) {
			// 按级别分组
			severity := strings.ToUpper(alert.Severity)
			group.AlertsBySeverity[severity] = append(group.AlertsBySeverity[severity], alert)
			group.TotalFiring++

			// 更新集群统计
			if stat, exists := group.ClusterStats[alert.ClusterName]; exists {
				stat.FiringCount++
			} else {
				group.ClusterStats[alert.ClusterName] = &ClusterStat{
					ClusterName:   alert.ClusterName,
					FiringCount:   1,
					ResolvedCount: 0,
				}
			}
		} else {
			// 已恢复的告警
			group.ResolvedAlerts = append(group.ResolvedAlerts, alert)
			group.TotalResolved++

			// 更新集群统计
			if stat, exists := group.ClusterStats[alert.ClusterName]; exists {
				stat.ResolvedCount++
			} else {
				group.ClusterStats[alert.ClusterName] = &ClusterStat{
					ClusterName:   alert.ClusterName,
					FiringCount:   0,
					ResolvedCount: 1,
				}
			}
		}
	}

	group.FirstAlertTime = firstTime
	group.LastAlertTime = lastTime

	return group
}

// GetAggregator 获取聚合器实例
func (m *manager) GetAggregator() *AlertAggregator {
	return m.aggregator
}

// EnableAggregator 动态启用聚合器
func (m *manager) EnableAggregator() error {
	if m.aggregator == nil {
		return errorx.Msg("聚合器未初始化")
	}

	if !m.aggregator.config.Enabled {
		m.aggregator.config.Enabled = true
		m.l.Info("[告警管理器] 聚合器已动态启用")
	}

	return nil
}

// DisableAggregator 动态禁用聚合器
func (m *manager) DisableAggregator() error {
	if m.aggregator == nil {
		return errorx.Msg("聚合器未初始化")
	}

	if m.aggregator.config.Enabled {
		m.aggregator.config.Enabled = false
		m.l.Info("[告警管理器] 聚合器已动态禁用")
	}

	return nil
}

// GetAggregatorStatus 获取聚合器状态
func (m *manager) GetAggregatorStatus() map[string]interface{} {
	status := map[string]interface{}{
		"initialized": m.aggregator != nil,
		"enabled":     false,
		"stats":       nil,
	}

	if m.aggregator != nil {
		status["enabled"] = m.aggregator.config.Enabled
		status["stats"] = m.aggregator.GetStats()
		status["healthInfo"] = m.aggregator.GetHealthInfo()
	}

	return status
}
