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

// ==================== 常量定义 ====================

const (
	// AlertWebhookQueueKey 告警 webhook 消息队列的 Redis Key
	AlertWebhookQueueKey = "alertmanager:webhook:queue"

	// ConsumerWorkerCount 消费者工作协程数量
	// 可根据实际负载情况调整，建议值为 CPU 核心数的 1-2 倍
	ConsumerWorkerCount = 3

	// MaxRetryCount 消息最大重试次数
	// 超过此次数后消息将被移入死信队列
	MaxRetryCount = 3

	// RetryDelayBase 重试延迟基数
	// 实际延迟 = RetryDelayBase * 重试次数（指数退避）
	RetryDelayBase = 1 * time.Second

	// DeadLetterQueueKey 死信队列的 Redis Key
	// 处理失败且超过重试次数的消息将被移入此队列
	DeadLetterQueueKey = "alertmanager:webhook:dlq"

	// BlpopTimeout 阻塞式获取消息的超时时间
	// 超时后会重新检查 context 是否已取消，然后继续等待
	BlpopTimeout = 30 * time.Second
)

// ==================== 数据结构定义 ====================

// AlertInstance 告警实例结构体，用于 RPC 传递给通知服务
// 包含告警的所有详细信息，包括关联的集群、项目、工作空间等上下文
type AlertInstance struct {
	ID             uint64            `json:"id"`                       // 数据库主键 ID
	Instance       string            `json:"instance"`                 // 告警实例标识
	Fingerprint    string            `json:"fingerprint"`              // 告警指纹，用于去重和关联
	ClusterUUID    string            `json:"clusterUuid"`              // 关联的集群 UUID
	ClusterName    string            `json:"clusterName"`              // 关联的集群名称
	ProjectID      uint64            `json:"projectId"`                // 关联的项目 ID
	ProjectName    string            `json:"projectName"`              // 关联的项目名称
	WorkspaceID    uint64            `json:"workspaceId"`              // 关联的工作空间 ID
	WorkspaceName  string            `json:"workspaceName"`            // 关联的工作空间名称
	AlertName      string            `json:"alertName"`                // 告警规则名称
	Severity       string            `json:"severity"`                 // 告警级别：critical、warning、info
	Status         string            `json:"status"`                   // 告警状态：firing、resolved
	Labels         map[string]string `json:"labels"`                   // 告警标签
	Annotations    map[string]string `json:"annotations"`              // 告警注解
	GeneratorURL   string            `json:"generatorUrl"`             // 告警生成器 URL
	StartsAt       time.Time         `json:"startsAt"`                 // 告警开始时间
	EndsAt         *time.Time        `json:"endsAt,omitempty"`         // 告警结束时间
	ResolvedAt     *time.Time        `json:"resolvedAt,omitempty"`     // 告警恢复时间
	Duration       uint              `json:"duration"`                 // 告警持续时长（秒）
	RepeatCount    uint              `json:"repeatCount"`              // 告警重复触发次数
	LastNotifiedAt *time.Time        `json:"lastNotifiedAt,omitempty"` // 最后通知时间
}

// AlertConsumerDeps 告警消费者依赖项
// 包含消费者运行所需的所有外部依赖
type AlertConsumerDeps struct {
	Redis                     *redis.Redis                    // Redis 客户端，用于消息队列操作
	AlertInstancesModel       model.AlertInstancesModel       // 告警实例数据模型
	OnecClusterModel          model.OnecClusterModel          // 集群数据模型
	OnecProjectModel          model.OnecProjectModel          // 项目数据模型
	OnecProjectClusterModel   model.OnecProjectClusterModel   // 项目-集群关联数据模型
	OnecProjectWorkspaceModel model.OnecProjectWorkspaceModel // 项目-工作空间数据模型
	AlertRpc                  alertservice.AlertService       // 告警通知 RPC 客户端
}

// AlertConsumer 告警消费者
// 负责从 Redis 队列中消费告警消息，入库并发送通知
type AlertConsumer struct {
	deps          *AlertConsumerDeps // 外部依赖
	ctx           context.Context    // 消费者上下文，用于控制生命周期
	cancel        context.CancelFunc // 取消函数，用于停止消费者
	wg            sync.WaitGroup     // 等待组，用于等待所有 worker 退出
	workerNum     int                // 工作协程数量
	blockingNodes []redis.RedisNode  // 每个 worker 专用的阻塞连接
}

// ClusterInfo 集群信息
type ClusterInfo struct {
	Name string // 集群名称
}

// ProjectInfo 项目信息
type ProjectInfo struct {
	Id   uint64 // 项目 ID
	Name string // 项目名称
}

// WorkspaceInfo 工作空间信息
type WorkspaceInfo struct {
	Id   uint64 // 工作空间 ID
	Name string // 工作空间名称
}

// ProcessResult 消息处理结果
type ProcessResult struct {
	TotalCount   int      // 总告警数量
	SuccessCount int      // 成功处理数量
	FailedCount  int      // 失败数量
	FailedAlerts []string // 失败的告警指纹列表
}

// WebhookMessage 队列中的消息结构
type WebhookMessage struct {
	Webhook    *AlertmanagerWebhook `json:"webhook"`    // Alertmanager webhook 原始数据
	ReceivedAt int64                `json:"receivedAt"` // 消息接收时间戳
	MessageID  string               `json:"messageId"`  // 消息唯一标识
	RetryCount int                  `json:"retryCount"` // 当前重试次数
}

// AlertmanagerWebhook Alertmanager 发送的 webhook 数据结构
type AlertmanagerWebhook struct {
	Receiver          string            `json:"receiver"`          // 接收者名称
	Status            string            `json:"status"`            // 整体状态
	Alerts            []Alert           `json:"alerts"`            // 告警列表
	GroupLabels       map[string]string `json:"groupLabels"`       // 分组标签
	CommonLabels      map[string]string `json:"commonLabels"`      // 公共标签
	CommonAnnotations map[string]string `json:"commonAnnotations"` // 公共注解
	ExternalURL       string            `json:"externalURL"`       // Alertmanager 外部 URL
	Version           string            `json:"version"`           // API 版本
	GroupKey          string            `json:"groupKey"`          // 分组键
	TruncatedAlerts   int32             `json:"truncatedAlerts"`   // 被截断的告警数量
}

// Alert 单条告警数据结构
type Alert struct {
	Status       string            `json:"status"`       // 告警状态：firing 或 resolved
	Labels       map[string]string `json:"labels"`       // 告警标签
	Annotations  map[string]string `json:"annotations"`  // 告警注解
	StartsAt     string            `json:"startsAt"`     // 告警开始时间（RFC3339 格式）
	EndsAt       string            `json:"endsAt"`       // 告警结束时间（RFC3339 格式）
	GeneratorURL string            `json:"generatorURL"` // 告警生成器 URL
	Fingerprint  string            `json:"fingerprint"`  // 告警指纹
}

// ==================== 构造函数 ====================

// NewAlertConsumer 创建告警消费者实例
// 参数：
//   - deps: 消费者依赖项
//
// 返回：
//   - *AlertConsumer: 消费者实例
func NewAlertConsumer(deps *AlertConsumerDeps) *AlertConsumer {
	ctx, cancel := context.WithCancel(context.Background())

	return &AlertConsumer{
		deps:          deps,
		ctx:           ctx,
		cancel:        cancel,
		workerNum:     ConsumerWorkerCount,
		blockingNodes: make([]redis.RedisNode, ConsumerWorkerCount),
	}
}

// ==================== 消费者接口实现 ====================

// Name 返回消费者名称
// 实现 Consumer 接口
func (c *AlertConsumer) Name() string {
	return "AlertConsumer"
}

// Start 启动消费者
// 创建阻塞连接并启动工作协程
// 参数：
//   - ctx: 上下文（此处未使用，使用内部 context）
//
// 返回：
//   - error: 启动失败时返回错误
func (c *AlertConsumer) Start(ctx context.Context) error {
	logx.Infof("[AlertConsumer] 启动告警消费者，工作协程数: %d", c.workerNum)

	// 为每个 worker 创建专用的阻塞连接
	// 阻塞连接用于 BLPOP 操作，避免连接被其他操作占用
	for i := 0; i < c.workerNum; i++ {
		node, err := redis.CreateBlockingNode(c.deps.Redis)
		if err != nil {
			logx.Errorf("[AlertConsumer] 创建阻塞连接失败: worker=%d, error=%v", i, err)
			return fmt.Errorf("创建阻塞连接失败: %w", err)
		}
		c.blockingNodes[i] = node
		logx.Infof("[AlertConsumer] Worker[%d] 阻塞连接创建成功", i)
	}

	// 启动工作协程
	for i := 0; i < c.workerNum; i++ {
		c.wg.Add(1)
		go c.work(i)
	}

	logx.Info("[AlertConsumer] 告警消费者启动完成")
	return nil
}

// Stop 停止消费者
// 取消上下文并等待所有工作协程退出
// 返回：
//   - error: 停止失败时返回错误
func (c *AlertConsumer) Stop() error {
	logx.Info("[AlertConsumer] 停止告警消费者...")

	// 取消上下文，通知所有 worker 退出
	c.cancel()

	// 等待所有 worker 退出
	c.wg.Wait()

	// 关闭所有阻塞连接
	for i, node := range c.blockingNodes {
		if node != nil {
			if closer, ok := node.(interface{ Close() error }); ok {
				if err := closer.Close(); err != nil {
					logx.Errorf("[AlertConsumer] 关闭阻塞连接失败: worker=%d, error=%v", i, err)
				}
			}
		}
	}

	logx.Info("[AlertConsumer] 告警消费者已停止")
	return nil
}

// ==================== 工作协程 ====================

// work 工作协程主循环
// 持续从队列中消费消息，直到收到停止信号
// 参数：
//   - workerID: 工作协程编号，用于日志标识
func (c *AlertConsumer) work(workerID int) {
	defer c.wg.Done()

	logx.Infof("[AlertConsumer] Worker[%d] 启动", workerID)
	defer logx.Infof("[AlertConsumer] Worker[%d] 停止", workerID)

	for {
		select {
		case <-c.ctx.Done():
			// 收到停止信号，退出循环
			return
		default:
			if err := c.consume(workerID); err != nil {
				logx.Errorf("[AlertConsumer] Worker[%d] 消费消息失败: %v", workerID, err)
				// 出错时短暂等待，避免错误风暴导致日志刷屏
				time.Sleep(time.Second)
			}
		}
	}
}

// consume 消费单条消息
// 从队列中阻塞式获取消息并处理
// 参数：
//   - workerID: 工作协程编号
//
// 返回：
//   - error: 消费失败时返回错误
func (c *AlertConsumer) consume(workerID int) error {
	// 获取该 worker 的专用阻塞连接
	node := c.blockingNodes[workerID]
	if node == nil {
		return fmt.Errorf("阻塞连接未初始化")
	}

	// 使用 BLPOP 阻塞式获取消息
	// 超时后会返回 redis.Nil 错误，这是正常的
	messageData, err := c.deps.Redis.BlpopWithTimeout(node, BlpopTimeout, AlertWebhookQueueKey)
	if err != nil {
		// BLPOP 超时返回 redis.Nil，这是正常情况
		if errors.Is(err, redis.Nil) {
			// 超时了，没有消息，检查是否需要退出
			select {
			case <-c.ctx.Done():
				return nil
			default:
				// 继续等待下一轮
				return nil
			}
		}
		// 真正的错误
		logx.Errorf("[AlertConsumer] Worker[%d] 从队列获取消息失败: %v", workerID, err)
		return fmt.Errorf("从队列获取消息失败: %w", err)
	}

	// 检查消息是否为空
	if messageData == "" {
		return nil
	}

	logx.Infof("[AlertConsumer] Worker[%d] 获取到消息: len=%d", workerID, len(messageData))

	// 解析消息 JSON
	var msg WebhookMessage
	if err := json.Unmarshal([]byte(messageData), &msg); err != nil {
		logx.Errorf("[AlertConsumer] Worker[%d] 解析消息失败: %v, 原始数据: %s", workerID, err, messageData)
		// 解析失败的消息无法重试，直接移入死信队列
		c.moveToDeadLetterQueue(messageData, fmt.Sprintf("JSON解析失败: %v", err))
		return nil
	}

	// 校验消息完整性
	if msg.Webhook == nil {
		logx.Errorf("[AlertConsumer] Worker[%d] 消息内容为空: messageId=%s", workerID, msg.MessageID)
		c.moveToDeadLetterQueue(messageData, "webhook 数据为空")
		return nil
	}

	// 处理消息
	startTime := time.Now()
	result, err := c.processMessage(workerID, &msg)
	elapsed := time.Since(startTime)

	if err != nil {
		logx.Errorf("[AlertConsumer] Worker[%d] 处理消息失败: messageId=%s, error=%v, elapsed=%dms",
			workerID, msg.MessageID, err, elapsed.Milliseconds())

		// 判断是否需要重试
		if msg.RetryCount < MaxRetryCount {
			c.retryMessage(&msg)
		} else {
			logx.Errorf("[AlertConsumer] Worker[%d] 消息重试超限，移入死信队列: messageId=%s", workerID, msg.MessageID)
			c.moveToDeadLetterQueue(messageData, fmt.Sprintf("重试超限: %v", err))
		}

		return err
	}

	logx.Infof("[AlertConsumer] Worker[%d] 消息处理成功: messageId=%s, total=%d, success=%d, failed=%d, elapsed=%dms",
		workerID, msg.MessageID, result.TotalCount, result.SuccessCount, result.FailedCount, elapsed.Milliseconds())

	if result.FailedCount > 0 {
		logx.Errorf("[AlertConsumer] Worker[%d] 部分告警处理失败: messageId=%s, failed=%v",
			workerID, msg.MessageID, result.FailedAlerts)
	}

	return nil
}

// ==================== 消息处理 ====================

// processMessage 处理 webhook 消息
// 遍历所有告警，逐条入库，然后发送通知
// 参数：
//   - workerID: 工作协程编号
//   - msg: webhook 消息
//
// 返回：
//   - *ProcessResult: 处理结果
//   - error: 处理失败时返回错误
func (c *AlertConsumer) processMessage(workerID int, msg *WebhookMessage) (*ProcessResult, error) {
	result := &ProcessResult{
		TotalCount:   len(msg.Webhook.Alerts),
		SuccessCount: 0,
		FailedCount:  0,
		FailedAlerts: make([]string, 0),
	}

	// 收集成功保存的告警实例，用于后续发送通知
	alertInstances := make([]*AlertInstance, 0, len(msg.Webhook.Alerts))

	// 创建带超时的上下文，避免单条告警处理时间过长
	ctx, cancel := context.WithTimeout(c.ctx, 60*time.Second)
	defer cancel()

	// 遍历所有告警并入库
	for _, alert := range msg.Webhook.Alerts {
		alertInstance, err := c.saveAlert(ctx, &alert, msg.Webhook)
		if err != nil {
			logx.Errorf("[AlertConsumer] Worker[%d] 保存告警失败: fingerprint=%s, error=%v",
				workerID, alert.Fingerprint, err)
			result.FailedCount++
			result.FailedAlerts = append(result.FailedAlerts, alert.Fingerprint)
			continue
		}

		alertInstances = append(alertInstances, alertInstance)
		result.SuccessCount++
	}

	// 如果有失败的告警，返回错误触发重试
	if result.FailedCount > 0 {
		return result, fmt.Errorf("部分告警保存失败: %d/%d", result.FailedCount, result.TotalCount)
	}

	// 所有告警都入库成功后，调用通知接口
	c.sendNotification(workerID, msg, alertInstances)

	return result, nil
}

// sendNotification 发送告警通知
// 只发送 firing 状态的告警，resolved 状态不发送
// 参数：
//   - workerID: 工作协程编号
//   - msg: webhook 消息
//   - alertInstances: 已入库的告警实例列表
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
	logx.Infof("[AlertConsumer] Worker[%d] 告警统计: messageId=%s, receiver=%s",
		workerID, msg.MessageID, msg.Webhook.Receiver)
	logx.Infof("[AlertConsumer] 总告警数: %d, 触发中: %d, 已恢复: %d",
		totalAlerts, firingCount, resolvedCount)

	if len(severityStats) > 0 {
		logx.Infof("[AlertConsumer] 按级别统计:")
		for severity, count := range severityStats {
			logx.Infof("[AlertConsumer]   %s: %d", severity, count)
		}
	}

	// 筛选出 firing 状态的告警，只发送这些
	firingAlerts := make([]*AlertInstance, 0)
	for _, alert := range alertInstances {
		if alert.Status == "firing" {
			firingAlerts = append(firingAlerts, alert)
		}
	}

	// 如果没有 firing 告警，不需要发送通知
	if len(firingAlerts) == 0 {
		logx.Infof("[AlertConsumer] Worker[%d] 没有需要发送的告警（全部已恢复）", workerID)
		return
	}

	// 序列化告警数据
	alertData, err := json.Marshal(firingAlerts)
	if err != nil {
		logx.Errorf("[AlertConsumer] Worker[%d] 序列化告警数据失败: %v", workerID, err)
		return
	}

	logx.Infof("[AlertConsumer] Worker[%d] 准备发送 %d 条 firing 告警通知", workerID, len(firingAlerts))

	// 调用 RPC 发送通知，设置超时
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err = c.deps.AlertRpc.AlertNotify(ctx, &pb.AlertNotifyReq{
		AlertType: "prometheus",
		AlertData: string(alertData),
	})

	if err != nil {
		logx.Errorf("[AlertConsumer] Worker[%d] 调用告警通知RPC失败: %v", workerID, err)
		// 不返回错误，避免消息重试（告警已入库，通知失败可以后续补发）
		return
	}

	logx.Infof("[AlertConsumer] Worker[%d] 告警通知RPC调用成功: messageId=%s, count=%d",
		workerID, msg.MessageID, len(firingAlerts))
}

// ==================== 告警入库 ====================

// findLatestAlertByFingerprint 按 fingerprint 查找最新的告警记录
// 不限制 status，返回该 fingerprint 下最新的一条记录
// 参数：
//   - ctx: 上下文
//   - fingerprint: 告警指纹
//
// 返回：
//   - *model.AlertInstances: 找到的记录，如果不存在返回 nil
//   - error: 查询失败时返回错误
func (c *AlertConsumer) findLatestAlertByFingerprint(ctx context.Context, fingerprint string) (*model.AlertInstances, error) {
	// 使用 SearchNoPage 查询，按 id 降序排列
	// 这样返回的第一条就是最新的记录
	instances, err := c.deps.AlertInstancesModel.SearchNoPage(
		ctx,
		"id",  // 按 id 排序
		false, // 降序（最新的在前面）
		"`fingerprint` = ?", fingerprint,
	)

	if err != nil {
		// 如果是 ErrNotFound，表示没有找到记录
		if errors.Is(err, model.ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}

	// 如果没有找到记录
	if len(instances) == 0 {
		return nil, nil
	}

	// 返回最新的一条
	return instances[0], nil
}

// saveAlert 保存单条告警
// 根据 fingerprint 查找已有记录，决定是更新还是创建
// 状态转换规则：
//   - firing -> firing: 增加重复次数
//   - firing -> resolved: 更新状态为已恢复
//   - resolved -> firing: 创建新记录（新的告警周期）
//   - resolved -> resolved: 跳过，不做处理
//
// 参数：
//   - ctx: 上下文
//   - alert: 单条告警数据
//   - webhook: webhook 原始数据
//
// 返回：
//   - *AlertInstance: 告警实例（用于 RPC 传递）
//   - error: 保存失败时返回错误
func (c *AlertConsumer) saveAlert(ctx context.Context, alert *Alert, webhook *AlertmanagerWebhook) (*AlertInstance, error) {
	// 参数校验
	if alert == nil {
		return nil, fmt.Errorf("告警数据为空")
	}

	fingerprint := alert.Fingerprint
	if fingerprint == "" {
		return nil, fmt.Errorf("告警指纹为空")
	}

	// 获取本次收到的状态
	newStatus := alert.Status
	if newStatus == "" {
		newStatus = webhook.Status
	}

	// 从标签中提取关键信息
	clusterUuid := ""
	namespace := ""
	alertName := ""
	severity := "info"
	instance := ""

	if alert.Labels != nil {
		// 尝试两种可能的集群 UUID 标签名
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

	// 如果没有 instance 标签，使用 fingerprint 作为默认值
	if instance == "" {
		instance = fingerprint
	}

	// 解析告警上下文（集群、项目、工作空间信息）
	clusterInfo, projectInfo, workspaceInfo := c.resolveAlertContext(ctx, clusterUuid, namespace)

	// 序列化标签和注解为 JSON 字符串
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

	// 解析告警时间
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

	// 按 fingerprint 查找最新的记录（不限制 status）
	existInstance, err := c.findLatestAlertByFingerprint(ctx, fingerprint)
	if err != nil {
		return nil, fmt.Errorf("查询告警实例失败: %v", err)
	}

	// 如果存在记录，根据状态转换决定如何处理
	if existInstance != nil {
		oldStatus := existInstance.Status

		switch {
		case oldStatus == "firing" && newStatus == "firing":
			// 告警持续触发，增加重复次数
			existInstance.RepeatCount++
			logx.Infof("[AlertConsumer] 告警重复触发: fingerprint=%s, repeatCount=%d", fingerprint, existInstance.RepeatCount)

		case oldStatus == "firing" && newStatus == "resolved":
			// 告警恢复，更新状态
			logx.Infof("[AlertConsumer] 告警恢复: fingerprint=%s", fingerprint)

		case oldStatus == "resolved" && newStatus == "firing":
			// 告警重新触发（之前已恢复），创建新记录
			logx.Infof("[AlertConsumer] 告警重新触发: fingerprint=%s, 创建新记录", fingerprint)
			return c.createNewAlertInstance(ctx, alert, fingerprint, newStatus, instance,
				clusterUuid, clusterInfo, projectInfo, workspaceInfo,
				alertName, severity, labelsJSON, annotationsJSON, startsAt, endsAt)

		case oldStatus == "resolved" && newStatus == "resolved":
			// 重复的恢复通知，直接返回现有记录
			logx.Infof("[AlertConsumer] 重复的恢复通知: fingerprint=%s, 跳过", fingerprint)
			return c.buildAlertInstance(existInstance, alert.Labels, alert.Annotations), nil
		}

		// 更新记录的通用字段
		existInstance.Status = newStatus
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

		// 计算持续时间
		if newStatus == "resolved" && endsAt.Valid {
			existInstance.ResolvedAt = endsAt
			existInstance.Duration = uint64(endsAt.Time.Sub(existInstance.StartsAt).Seconds())
			logx.Infof("[AlertConsumer] 告警已恢复: fingerprint=%s, duration=%ds", fingerprint, existInstance.Duration)
		} else if newStatus == "firing" {
			// 更新持续时间（从开始到现在）
			currentDuration := uint64(time.Now().Sub(existInstance.StartsAt).Seconds())
			existInstance.Duration = currentDuration
		}

		// 执行更新
		if err := c.deps.AlertInstancesModel.Update(ctx, existInstance); err != nil {
			return nil, fmt.Errorf("更新告警实例失败: %v", err)
		}

		logx.Infof("[AlertConsumer] 更新告警: fingerprint=%s, oldStatus=%s, newStatus=%s, id=%d, repeatCount=%d",
			fingerprint, oldStatus, newStatus, existInstance.Id, existInstance.RepeatCount)

		return c.buildAlertInstance(existInstance, alert.Labels, alert.Annotations), nil
	}

	// 不存在记录，创建新记录
	return c.createNewAlertInstance(ctx, alert, fingerprint, newStatus, instance,
		clusterUuid, clusterInfo, projectInfo, workspaceInfo,
		alertName, severity, labelsJSON, annotationsJSON, startsAt, endsAt)
}

// createNewAlertInstance 创建新的告警实例记录
// 参数：
//   - ctx: 上下文
//   - alert: 原始告警数据
//   - fingerprint: 告警指纹
//   - status: 告警状态
//   - instance: 实例标识
//   - clusterUuid: 集群 UUID
//   - clusterInfo: 集群信息
//   - projectInfo: 项目信息
//   - workspaceInfo: 工作空间信息
//   - alertName: 告警名称
//   - severity: 告警级别
//   - labelsJSON: 标签 JSON 字符串
//   - annotationsJSON: 注解 JSON 字符串
//   - startsAt: 开始时间
//   - endsAt: 结束时间
//
// 返回：
//   - *AlertInstance: 告警实例
//   - error: 创建失败时返回错误
func (c *AlertConsumer) createNewAlertInstance(
	ctx context.Context,
	alert *Alert,
	fingerprint, status, instance, clusterUuid string,
	clusterInfo ClusterInfo,
	projectInfo ProjectInfo,
	workspaceInfo WorkspaceInfo,
	alertName, severity, labelsJSON, annotationsJSON string,
	startsAt time.Time,
	endsAt sql.NullTime,
) (*AlertInstance, error) {

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
		RepeatCount:       1, // 新告警，重复次数为 1
		NotifiedGroups:    "",
		NotificationCount: 0,
		LastNotifiedAt:    sql.NullTime{},
		CreatedBy:         "alertmanager",
		UpdatedBy:         "alertmanager",
		IsDeleted:         0,
	}

	// 如果是已恢复状态，计算持续时间
	if status == "resolved" && endsAt.Valid {
		newInstance.ResolvedAt = endsAt
		newInstance.Duration = uint64(endsAt.Time.Sub(startsAt).Seconds())
	}

	// 使用 InsertOrUpdate 插入或更新数据库（处理并发冲突）
	// 当多个 Worker 并发处理同一个告警时，数据库会自动处理唯一键冲突
	insertedInstance, err := c.deps.AlertInstancesModel.InsertOrUpdate(ctx, newInstance)
	if err != nil {
		return nil, fmt.Errorf("插入或更新告警实例失败: %v", err)
	}

	logx.Infof("[AlertConsumer] 创建或更新告警: fingerprint=%s, status=%s, id=%d, repeatCount=%d",
		fingerprint, status, insertedInstance.Id, insertedInstance.RepeatCount)

	// 构建返回的 AlertInstance
	var labels, annotations map[string]string
	if alert.Labels != nil {
		labels = alert.Labels
	}
	if alert.Annotations != nil {
		annotations = alert.Annotations
	}

	return c.buildAlertInstance(insertedInstance, labels, annotations), nil
}

// buildAlertInstance 从数据库模型构建 AlertInstance
// 用于将数据库记录转换为 RPC 传递的结构
// 参数：
//   - dbInstance: 数据库模型实例
//   - labels: 标签 map
//   - annotations: 注解 map
//
// 返回：
//   - *AlertInstance: 告警实例
func (c *AlertConsumer) buildAlertInstance(dbInstance *model.AlertInstances, labels, annotations map[string]string) *AlertInstance {
	// 转换可空时间字段为指针
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
		Labels:         labels,
		Annotations:    annotations,
		GeneratorURL:   dbInstance.GeneratorUrl,
		StartsAt:       dbInstance.StartsAt,
		EndsAt:         endsAt,
		ResolvedAt:     resolvedAt,
		Duration:       uint(dbInstance.Duration),
		RepeatCount:    uint(dbInstance.RepeatCount),
		LastNotifiedAt: lastNotifiedAt,
	}
}

// ==================== 上下文解析 ====================

// resolveAlertContext 解析告警的上下文信息
// 根据集群 UUID 和 namespace 查找关联的集群、项目、工作空间
// 参数：
//   - ctx: 上下文
//   - clusterUuid: 集群 UUID
//   - namespace: Kubernetes namespace
//
// 返回：
//   - ClusterInfo: 集群信息
//   - ProjectInfo: 项目信息
//   - WorkspaceInfo: 工作空间信息
func (c *AlertConsumer) resolveAlertContext(ctx context.Context, clusterUuid, namespace string) (ClusterInfo, ProjectInfo, WorkspaceInfo) {
	clusterInfo := ClusterInfo{}
	projectInfo := ProjectInfo{}
	workspaceInfo := WorkspaceInfo{}

	// 查询集群信息
	if clusterUuid != "" {
		if cluster, err := c.deps.OnecClusterModel.FindOneByUuid(ctx, clusterUuid); err == nil {
			clusterInfo.Name = cluster.Name
		}
	}

	// 查询项目和工作空间信息
	if namespace != "" && clusterUuid != "" {
		// 先查找该集群关联的所有项目
		projectClusters, err := c.deps.OnecProjectClusterModel.SearchNoPage(
			ctx, "", false, "`cluster_uuid` = ?", clusterUuid,
		)

		if err == nil && len(projectClusters) > 0 {
			// 遍历项目，查找包含该 namespace 的工作空间
			for _, pc := range projectClusters {
				workspace, err := c.deps.OnecProjectWorkspaceModel.FindOneByProjectClusterIdNamespace(
					ctx, pc.Id, namespace,
				)

				if err == nil {
					workspaceInfo.Id = workspace.Id
					workspaceInfo.Name = workspace.Name
					projectInfo.Id = pc.ProjectId

					// 查询项目名称
					if project, err := c.deps.OnecProjectModel.FindOne(ctx, pc.ProjectId); err == nil {
						projectInfo.Name = project.Name
					}

					break // 找到后退出循环
				}
			}
		}
	}

	return clusterInfo, projectInfo, workspaceInfo
}

// ==================== 重试和死信队列 ====================

// retryMessage 将消息重新入队等待重试
// 使用指数退避策略，延迟时间 = 基数 * 重试次数
// 参数：
//   - msg: 需要重试的消息
func (c *AlertConsumer) retryMessage(msg *WebhookMessage) {
	msg.RetryCount++
	retryDelay := RetryDelayBase * time.Duration(msg.RetryCount)

	// 使用 goroutine 延迟入队，避免阻塞当前 worker
	go func() {
		// 创建一个带取消的 timer，便于服务停止时清理
		timer := time.NewTimer(retryDelay)
		defer timer.Stop()

		select {
		case <-c.ctx.Done():
			// 服务停止，放弃重试
			logx.Infof("[AlertConsumer] 服务停止，放弃重试: messageId=%s", msg.MessageID)
			return
		case <-timer.C:
			// 延迟时间到，重新入队
		}

		messageData, err := json.Marshal(msg)
		if err != nil {
			logx.Errorf("[AlertConsumer] 序列化重试消息失败: %v", err)
			return
		}

		// 使用 RPUSH 将消息放到队列尾部
		if _, err := c.deps.Redis.Rpush(AlertWebhookQueueKey, string(messageData)); err != nil {
			logx.Errorf("[AlertConsumer] 重试消息入队失败: %v", err)
		} else {
			logx.Infof("[AlertConsumer] 消息重试入队: messageId=%s, retryCount=%d", msg.MessageID, msg.RetryCount)
		}
	}()
}

// moveToDeadLetterQueue 将消息移入死信队列
// 处理失败且无法重试的消息会被移入此队列，便于后续人工处理
// 参数：
//   - messageData: 原始消息数据
//   - reason: 移入死信队列的原因
func (c *AlertConsumer) moveToDeadLetterQueue(messageData, reason string) {
	deadLetter := map[string]interface{}{
		"message":   messageData,
		"reason":    reason,
		"timestamp": time.Now().Unix(),
	}

	if dlData, err := json.Marshal(deadLetter); err == nil {
		_, err := c.deps.Redis.Rpush(DeadLetterQueueKey, string(dlData))
		if err != nil {
			logx.Errorf("[AlertConsumer] 消息移入死信队列失败: %v", err)
			return
		}
		logx.Errorf("[AlertConsumer] 消息已移入死信队列: reason=%s", reason)
	}
}
