package consumer

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/client/alertservice"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

const (
	AlertWebhookQueueKey = "alertmanager:webhook:queue"
	ConsumerWorkerCount  = 3
	MaxRetryCount        = 3
	RetryDelayBase       = 1 * time.Second
	DeadLetterQueueKey   = "alertmanager:webhook:dlq"
)

// AlertInstance 告警实例（用于 RPC 传递）
type AlertInstance struct {
	ID             uint64            `json:"id"`
	Instance       string            `json:"instance"`
	Fingerprint    string            `json:"fingerprint"`
	ClusterUUID    string            `json:"clusterUuid"`
	ClusterName    string            `json:"clusterName"`
	ProjectID      uint64            `json:"projectId"`
	ProjectName    string            `json:"projectName"`
	WorkspaceID    uint64            `json:"workspaceId"`
	WorkspaceName  string            `json:"workspaceName"`
	AlertName      string            `json:"alertName"`
	Severity       string            `json:"severity"`
	Status         string            `json:"status"`
	Labels         map[string]string `json:"labels"`
	Annotations    map[string]string `json:"annotations"`
	GeneratorURL   string            `json:"generatorUrl"`
	StartsAt       time.Time         `json:"startsAt"`
	EndsAt         *time.Time        `json:"endsAt,omitempty"`
	ResolvedAt     *time.Time        `json:"resolvedAt,omitempty"`
	Duration       uint              `json:"duration"`
	RepeatCount    uint              `json:"repeatCount"`
	LastNotifiedAt *time.Time        `json:"lastNotifiedAt,omitempty"`
}

// AlertConsumerDeps 告警消费者依赖
type AlertConsumerDeps struct {
	Redis                     *redis.Redis
	AlertInstancesModel       model.AlertInstancesModel
	OnecClusterModel          model.OnecClusterModel
	OnecProjectModel          model.OnecProjectModel
	OnecProjectClusterModel   model.OnecProjectClusterModel
	OnecProjectWorkspaceModel model.OnecProjectWorkspaceModel
	AlertRpc                  alertservice.AlertService
}

// AlertConsumer 告警消费者
type AlertConsumer struct {
	deps      *AlertConsumerDeps
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	workerNum int
}

// NewAlertConsumer 创建告警消费者
func NewAlertConsumer(deps *AlertConsumerDeps) *AlertConsumer {
	ctx, cancel := context.WithCancel(context.Background())

	return &AlertConsumer{
		deps:      deps,
		ctx:       ctx,
		cancel:    cancel,
		workerNum: ConsumerWorkerCount,
	}
}

// Name 返回消费者名称
func (c *AlertConsumer) Name() string {
	return "AlertConsumer"
}

// Start 启动消费者
func (c *AlertConsumer) Start(ctx context.Context) error {
	logx.Infof("🚀 启动告警消费者，工作协程数: %d", c.workerNum)

	for i := 0; i < c.workerNum; i++ {
		c.wg.Add(1)
		go c.work(i)
	}

	logx.Info("✅ 告警消费者启动完成")
	return nil
}

// Stop 停止消费者
func (c *AlertConsumer) Stop() error {
	logx.Info("🛑 停止告警消费者...")
	c.cancel()
	c.wg.Wait()
	logx.Info("✅ 告警消费者已停止")
	return nil
}

// work 工作协程
func (c *AlertConsumer) work(workerID int) {
	defer c.wg.Done()

	logx.Infof("⚙️  Worker[%d] 启动", workerID)
	defer logx.Infof("🛑 Worker[%d] 停止", workerID)

	for {
		select {
		case <-c.ctx.Done():
			return
		default:
			if err := c.consume(workerID); err != nil {
				logx.Errorf("❌ Worker[%d] 消费消息失败: %v", workerID, err)
				time.Sleep(time.Second)
			}
		}
	}
}

// consume 消费单条消息
func (c *AlertConsumer) consume(workerID int) error {
	// 使用 Rpop 非阻塞获取消息
	messageData, err := c.deps.Redis.Rpop(AlertWebhookQueueKey)
	if err != nil {
		if errors.Is(err, redis.Nil) {
			// 队列为空，这是正常的
			time.Sleep(100 * time.Millisecond)
			return nil
		}
		// 真正的错误
		logx.Errorf("❌ Worker[%d] 从队列获取消息失败: %v", workerID, err)
		return fmt.Errorf("从队列获取消息失败: %w", err)
	}

	// 检查消息是否为空
	if messageData == "" {
		time.Sleep(100 * time.Millisecond)
		return nil
	}

	logx.Infof("📨 Worker[%d] 获取到消息: len=%d", workerID, len(messageData))

	var msg WebhookMessage
	if err := json.Unmarshal([]byte(messageData), &msg); err != nil {
		logx.Errorf("❌ Worker[%d] 解析消息失败: %v, 原始数据: %s", workerID, err, messageData)
		c.moveToDeadLetterQueue(messageData, fmt.Sprintf("解析失败: %v", err))
		return nil
	}

	// 处理消息
	startTime := time.Now()
	result, err := c.processMessage(workerID, &msg)
	elapsed := time.Since(startTime)

	if err != nil {
		logx.Errorf("❌ Worker[%d] 处理消息失败: messageId=%s, error=%v, elapsed=%dms",
			workerID, msg.MessageID, err, elapsed.Milliseconds())

		if msg.RetryCount < MaxRetryCount {
			c.retryMessage(&msg)
		} else {
			logx.Errorf("💀 Worker[%d] 消息重试超限: messageId=%s", workerID, msg.MessageID)
			c.moveToDeadLetterQueue(messageData, fmt.Sprintf("重试超限: %v", err))
		}

		return err
	}

	logx.Infof("✅ Worker[%d] 消息处理成功: messageId=%s, total=%d, success=%d, failed=%d, elapsed=%dms",
		workerID, msg.MessageID, result.TotalCount, result.SuccessCount, result.FailedCount, elapsed.Milliseconds())

	if result.FailedCount > 0 {
		logx.Errorf("⚠️  Worker[%d] 部分告警处理失败: messageId=%s, failed=%v",
			workerID, msg.MessageID, result.FailedAlerts)
	}

	return nil
}

// processMessage 处理消息
func (c *AlertConsumer) processMessage(workerID int, msg *WebhookMessage) (*ProcessResult, error) {
	result := &ProcessResult{
		TotalCount:   len(msg.Webhook.Alerts),
		SuccessCount: 0,
		FailedCount:  0,
		FailedAlerts: make([]string, 0),
	}

	// 🔥 收集成功保存的告警实例
	alertInstances := make([]*AlertInstance, 0, len(msg.Webhook.Alerts))

	// 遍历所有告警并入库
	for _, alert := range msg.Webhook.Alerts {
		// 🔥 saveAlert 返回 AlertInstance
		alertInstance, err := c.saveAlert(&alert, msg.Webhook)
		if err != nil {
			logx.Errorf("❌ Worker[%d] 保存告警失败: fingerprint=%s, error=%v",
				workerID, alert.Fingerprint, err)
			result.FailedCount++
			result.FailedAlerts = append(result.FailedAlerts, alert.Fingerprint)
			continue
		}

		// 🔥 收集成功的告警实例
		alertInstances = append(alertInstances, alertInstance)
		result.SuccessCount++
	}

	if result.FailedCount > 0 {
		return result, fmt.Errorf("部分告警保存失败: %d/%d", result.FailedCount, result.TotalCount)
	}

	// 🔥 所有告警都入库成功后，调用通知接口
	c.sendNotification(workerID, msg, alertInstances)

	return result, nil
}

// sendNotification 发送通知（调用 RPC）
func (c *AlertConsumer) sendNotification(workerID int, msg *WebhookMessage, alertInstances []*AlertInstance) {
	// 统计信息
	totalAlerts := len(alertInstances)
	firingCount := 0
	resolvedCount := 0
	severityStats := make(map[string]int)

	for _, alert := range alertInstances {
		if alert.Status == "firing" {
			firingCount++
		} else if alert.Status == "resolved" {
			resolvedCount++
		}
		severityStats[alert.Severity]++
	}

	// 输出统计信息
	logx.Infof("📢 Worker[%d] 告警统计: messageId=%s, receiver=%s",
		workerID, msg.MessageID, msg.Webhook.Receiver)
	logx.Infof("📊 总告警数: %d | 🔥 告警: %d | ✅ 恢复: %d",
		totalAlerts, firingCount, resolvedCount)

	if len(severityStats) > 0 {
		logx.Infof("📈 按级别统计:")
		for severity, count := range severityStats {
			emoji := getSeverityEmoji(severity)
			logx.Infof("   %s %s: %d", emoji, severity, count)
		}
	}

	// 🔥 过滤 firing 状态的告警（resolved 由 Manager 内部处理）
	firingAlerts := make([]*AlertInstance, 0)
	for _, alert := range alertInstances {
		if alert.Status == "firing" {
			firingAlerts = append(firingAlerts, alert)
		}
	}

	if len(firingAlerts) == 0 {
		logx.Infof("📭 Worker[%d] 没有需要发送的告警（全部已恢复）", workerID)
		return
	}

	// 🔥 序列化告警数据为 JSON
	alertData, err := json.Marshal(firingAlerts)
	if err != nil {
		logx.Errorf("❌ Worker[%d] 序列化告警数据失败: %v", workerID, err)
		return
	}

	logx.Infof("📦 Worker[%d] 准备发送 %d 条 firing 告警通知", workerID, len(firingAlerts))

	// 🔥 调用 RPC 发送通知
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err = c.deps.AlertRpc.AlertNotify(ctx, &pb.AlertNotifyReq{
		AlertType: "prometheus",      // 🔥 告警类型
		AlertData: string(alertData), // 🔥 告警数据 JSON 字符串
		// UserIds 和 Title 在 prometheus 类型中不需要
	})

	if err != nil {
		logx.Errorf("❌ Worker[%d] 调用告警通知RPC失败: %v", workerID, err)
		// 不返回错误，避免消息重试（告警已入库）
		return
	}

	logx.Infof("✅ Worker[%d] 告警通知RPC调用成功: messageId=%s, count=%d",
		workerID, msg.MessageID, len(firingAlerts))
}

// getSeverityEmoji 根据severity返回emoji
func getSeverityEmoji(severity string) string {
	switch severity {
	case "critical":
		return "🔴"
	case "warning":
		return "🟡"
	case "info":
		return "🔵"
	default:
		return "⚪"
	}
}

// saveAlert 保存单条告警，返回 AlertInstance
func (c *AlertConsumer) saveAlert(alert *Alert, webhook *AlertmanagerWebhook) (*AlertInstance, error) {
	if alert == nil {
		return nil, fmt.Errorf("alert is nil")
	}

	fingerprint := alert.Fingerprint
	if fingerprint == "" {
		return nil, fmt.Errorf("alert fingerprint is empty")
	}

	status := alert.Status
	if status == "" {
		status = webhook.Status
	}

	// 提取标签
	clusterUuid := ""
	namespace := ""
	alertName := ""
	severity := "info"
	instance := ""

	if alert.Labels != nil {
		if v, ok := alert.Labels["cluster_uuid"]; ok {
			clusterUuid = v
		} else if v, ok := alert.Labels["clusterUuid"]; ok {
			clusterUuid = v
		}

		if v, ok := alert.Labels["namespace"]; ok {
			namespace = v
		}
		if v, ok := alert.Labels["alertname"]; ok {
			alertName = v
		}
		if v, ok := alert.Labels["severity"]; ok {
			severity = v
		}
		if v, ok := alert.Labels["instance"]; ok {
			instance = v
		}
	}

	if instance == "" {
		instance = fingerprint
	}

	// 获取上下文信息
	clusterInfo, projectInfo, workspaceInfo := c.resolveAlertContext(clusterUuid, namespace)

	// 序列化
	labelsJSON := "{}"
	if alert.Labels != nil {
		if labelsBytes, err := json.Marshal(alert.Labels); err == nil {
			labelsJSON = string(labelsBytes)
		}
	}

	annotationsJSON := "{}"
	if alert.Annotations != nil {
		if annotationsBytes, err := json.Marshal(alert.Annotations); err == nil {
			annotationsJSON = string(annotationsBytes)
		}
	}

	// 解析时间
	startsAt := time.Now()
	if alert.StartsAt != "" {
		if parsedTime, err := time.Parse(time.RFC3339, alert.StartsAt); err == nil {
			startsAt = parsedTime
		}
	}

	var endsAt sql.NullTime
	if alert.EndsAt != "" && alert.EndsAt != "0001-01-01T00:00:00Z" {
		if parsedTime, err := time.Parse(time.RFC3339, alert.EndsAt); err == nil {
			endsAt = sql.NullTime{Time: parsedTime, Valid: true}
		}
	}

	ctx := context.Background()

	// 查询是否已存在
	existInstance, err := c.deps.AlertInstancesModel.FindOneByFingerprintStatusIsDeleted(
		ctx, fingerprint, status, 0,
	)

	if err != nil && !errors.Is(err, model.ErrNotFound) {
		return nil, fmt.Errorf("查询告警实例失败: %v", err)
	}

	// ============ 更新现有记录 ============
	if existInstance != nil {
		if status == "firing" && existInstance.Status == "firing" {
			existInstance.RepeatCount++
			logx.Infof("🔁 告警重复触发: fingerprint=%s, repeatCount=%d", fingerprint, existInstance.RepeatCount)
		} else if status == "firing" && existInstance.Status == "resolved" {
			existInstance.RepeatCount = 1
			logx.Infof("🔄 告警重新触发: fingerprint=%s, 重置 repeatCount=1", fingerprint)
		}
		existInstance.Status = status
		existInstance.Labels = labelsJSON
		existInstance.Annotations = annotationsJSON
		existInstance.EndsAt = endsAt
		existInstance.UpdatedBy = "alertmanager"
		existInstance.ClusterUuid = clusterUuid
		existInstance.ClusterName = clusterInfo.Name
		existInstance.ProjectId = projectInfo.Id
		existInstance.ProjectName = projectInfo.Name
		existInstance.WorkspaceId = workspaceInfo.Id
		existInstance.WorkspaceName = workspaceInfo.Name

		if status == "resolved" && endsAt.Valid {
			existInstance.ResolvedAt = endsAt
			existInstance.Duration = uint64(endsAt.Time.Sub(existInstance.StartsAt).Seconds())
			logx.Infof("✅ 告警已恢复: fingerprint=%s, duration=%ds", fingerprint, existInstance.Duration)
		} else if status == "firing" {
			currentDuration := uint64(time.Now().Sub(existInstance.StartsAt).Seconds())
			existInstance.Duration = currentDuration
			logx.Infof("🔥 告警持续中: fingerprint=%s, duration=%ds", fingerprint, currentDuration)
		}

		if err := c.deps.AlertInstancesModel.Update(ctx, existInstance); err != nil {
			return nil, fmt.Errorf("更新告警实例失败: %v", err)
		}

		logx.Infof("🔄 更新告警: fingerprint=%s, status=%s, id=%d, repeatCount=%d, duration=%ds",
			fingerprint, status, existInstance.Id, existInstance.RepeatCount, existInstance.Duration)

		// 返回 AlertInstance
		return c.buildAlertInstance(existInstance, alert.Labels, alert.Annotations), nil
	}

	// ============ 创建新记录 ============
	newInstance := &model.AlertInstances{
		Instance:          instance,
		Fingerprint:       fingerprint,
		ClusterUuid:       clusterUuid,
		ClusterName:       clusterInfo.Name,
		ProjectId:         projectInfo.Id,
		ProjectName:       projectInfo.Name,
		WorkspaceId:       workspaceInfo.Id,
		WorkspaceName:     workspaceInfo.Name,
		AlertName:         alertName,
		Severity:          severity,
		Status:            status,
		Labels:            labelsJSON,
		Annotations:       annotationsJSON,
		GeneratorUrl:      alert.GeneratorURL,
		StartsAt:          startsAt,
		EndsAt:            endsAt,
		ResolvedAt:        sql.NullTime{},
		Duration:          0,
		RepeatCount:       1, // 新告警，RepeatCount = 1
		NotifiedGroups:    "",
		NotificationCount: 0,
		LastNotifiedAt:    sql.NullTime{},
		CreatedBy:         "alertmanager",
		UpdatedBy:         "alertmanager",
		IsDeleted:         0,
	}

	// ✅ 新记录的 Duration 计算
	if status == "resolved" && endsAt.Valid {
		newInstance.ResolvedAt = endsAt
		newInstance.Duration = uint64(endsAt.Time.Sub(startsAt).Seconds())
	} else if status == "firing" {
		// 新的 firing 告警，Duration = 0（刚开始）
		newInstance.Duration = 0
	}

	result, err := c.deps.AlertInstancesModel.Insert(ctx, newInstance)
	if err != nil {
		return nil, fmt.Errorf("插入告警实例失败: %v", err)
	}

	// 获取新插入的 ID
	if id, err := result.LastInsertId(); err == nil {
		newInstance.Id = uint64(id)
		logx.Infof("✨ 创建告警: fingerprint=%s, status=%s, id=%d, duration=%ds",
			fingerprint, status, newInstance.Id, newInstance.Duration)
	} else {
		logx.Errorf("⚠️  获取新插入告警ID失败: %v, fingerprint=%s", err, fingerprint)
	}

	// 返回 AlertInstance
	return c.buildAlertInstance(newInstance, alert.Labels, alert.Annotations), nil
}

// buildAlertInstance 从数据库模型构建 AlertInstance
func (c *AlertConsumer) buildAlertInstance(dbInstance *model.AlertInstances, labels, annotations map[string]string) *AlertInstance {
	// 转换时间指针
	var endsAt *time.Time
	if dbInstance.EndsAt.Valid {
		endsAt = &dbInstance.EndsAt.Time
	}

	var resolvedAt *time.Time
	if dbInstance.ResolvedAt.Valid {
		resolvedAt = &dbInstance.ResolvedAt.Time
	}

	var lastNotifiedAt *time.Time
	if dbInstance.LastNotifiedAt.Valid {
		lastNotifiedAt = &dbInstance.LastNotifiedAt.Time
	}

	return &AlertInstance{
		ID:             dbInstance.Id,
		Instance:       dbInstance.Instance,
		Fingerprint:    dbInstance.Fingerprint,
		ClusterUUID:    dbInstance.ClusterUuid,
		ClusterName:    dbInstance.ClusterName,
		ProjectID:      dbInstance.ProjectId,
		ProjectName:    dbInstance.ProjectName,
		WorkspaceID:    dbInstance.WorkspaceId,
		WorkspaceName:  dbInstance.WorkspaceName,
		AlertName:      dbInstance.AlertName,
		Severity:       dbInstance.Severity,
		Status:         dbInstance.Status,
		Labels:         labels,      // 🔥 直接使用传入的 map
		Annotations:    annotations, // 🔥 直接使用传入的 map
		GeneratorURL:   dbInstance.GeneratorUrl,
		StartsAt:       dbInstance.StartsAt,
		EndsAt:         endsAt,
		ResolvedAt:     resolvedAt,
		Duration:       uint(dbInstance.Duration),
		RepeatCount:    uint(dbInstance.RepeatCount),
		LastNotifiedAt: lastNotifiedAt,
	}
}

// resolveAlertContext 解析告警上下文
func (c *AlertConsumer) resolveAlertContext(clusterUuid, namespace string) (ClusterInfo, ProjectInfo, WorkspaceInfo) {
	clusterInfo := ClusterInfo{}
	projectInfo := ProjectInfo{}
	workspaceInfo := WorkspaceInfo{}

	ctx := context.Background()

	if clusterUuid != "" {
		if cluster, err := c.deps.OnecClusterModel.FindOneByUuid(ctx, clusterUuid); err == nil {
			clusterInfo.Name = cluster.Name
		}
	}

	if namespace != "" && clusterUuid != "" {
		projectClusters, err := c.deps.OnecProjectClusterModel.SearchNoPage(
			ctx, "", false, "`cluster_uuid` = ?", clusterUuid,
		)

		if err == nil && len(projectClusters) > 0 {
			for _, pc := range projectClusters {
				workspace, err := c.deps.OnecProjectWorkspaceModel.FindOneByProjectClusterIdNamespace(
					ctx, pc.Id, namespace,
				)

				if err == nil {
					workspaceInfo.Id = workspace.Id
					workspaceInfo.Name = workspace.Name
					projectInfo.Id = pc.ProjectId

					if project, err := c.deps.OnecProjectModel.FindOne(ctx, pc.ProjectId); err == nil {
						projectInfo.Name = project.Name
					}

					break
				}
			}
		}
	}

	return clusterInfo, projectInfo, workspaceInfo
}

// retryMessage 重试消息
func (c *AlertConsumer) retryMessage(msg *WebhookMessage) {
	msg.RetryCount++
	retryDelay := RetryDelayBase * time.Duration(msg.RetryCount)

	go func() {
		time.Sleep(retryDelay)

		messageData, err := json.Marshal(msg)
		if err != nil {
			logx.Errorf("序列化重试消息失败: %v", err)
			return
		}

		if _, err := c.deps.Redis.Lpush(AlertWebhookQueueKey, string(messageData)); err != nil {
			logx.Errorf("重试消息入队失败: %v", err)
		} else {
			logx.Infof("🔄 消息重试: messageId=%s, retryCount=%d", msg.MessageID, msg.RetryCount)
		}
	}()
}

// moveToDeadLetterQueue 移入死信队列
func (c *AlertConsumer) moveToDeadLetterQueue(messageData, reason string) {
	deadLetter := map[string]interface{}{
		"message":   messageData,
		"reason":    reason,
		"timestamp": time.Now().Unix(),
	}

	if dlData, err := json.Marshal(deadLetter); err == nil {
		_, err := c.deps.Redis.Lpush(DeadLetterQueueKey, string(dlData))
		if err != nil {
			return
		}
		logx.Errorf("💀 消息移入死信队列: reason=%s", reason)
	}
}

// ==================== 辅助结构体 ====================

type ClusterInfo struct {
	Name string
}

type ProjectInfo struct {
	Id   uint64
	Name string
}

type WorkspaceInfo struct {
	Id   uint64
	Name string
}

type ProcessResult struct {
	TotalCount   int
	SuccessCount int
	FailedCount  int
	FailedAlerts []string
}

type WebhookMessage struct {
	Webhook    *AlertmanagerWebhook `json:"webhook"`
	ReceivedAt int64                `json:"receivedAt"`
	MessageID  string               `json:"messageId"`
	RetryCount int                  `json:"retryCount"`
}

type AlertmanagerWebhook struct {
	Receiver          string            `json:"receiver"`
	Status            string            `json:"status"`
	Alerts            []Alert           `json:"alerts"`
	GroupLabels       map[string]string `json:"groupLabels"`
	CommonLabels      map[string]string `json:"commonLabels"`
	CommonAnnotations map[string]string `json:"commonAnnotations"`
	ExternalURL       string            `json:"externalURL"`
	Version           string            `json:"version"`
	GroupKey          string            `json:"groupKey"`
	TruncatedAlerts   int32             `json:"truncatedAlerts"`
}

type Alert struct {
	Status       string            `json:"status"`
	Labels       map[string]string `json:"labels"`
	Annotations  map[string]string `json:"annotations"`
	StartsAt     string            `json:"startsAt"`
	EndsAt       string            `json:"endsAt"`
	GeneratorURL string            `json:"generatorURL"`
	Fingerprint  string            `json:"fingerprint"`
}
