package webhook

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

// ==================== 常量定义 ====================

const (
	// AlertWebhookQueueKey 告警 webhook 消息队列的 Redis Key
	AlertWebhookQueueKey = "alertmanager:webhook:queue"

	// MessageExpireSeconds 消息过期时间（秒）
	// 用于防止队列堆积，超过此时间的消息可以被清理
	MessageExpireSeconds = 86400 // 24小时

	// TokenHeaderName 认证 Token 的 Header 名称
	TokenHeaderName = "Authorization"

	// MaxQueueLength 队列最大长度限制
	// 防止消费者宕机时 Redis 内存溢出
	MaxQueueLength = 100000
)

// ==================== 数据结构定义 ====================

// ReceiveAlertmanagerWebhookLogic Alertmanager Webhook 接收逻辑处理器
type ReceiveAlertmanagerWebhookLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// WebhookMessage 队列中的消息结构
// 包含原始 webhook 数据和元信息
type WebhookMessage struct {
	Webhook    *types.AlertmanagerWebhook `json:"webhook"`    // Alertmanager webhook 原始数据
	ReceivedAt int64                      `json:"receivedAt"` // 消息接收时间戳（Unix 秒）
	MessageID  string                     `json:"messageId"`  // 消息唯一标识
	RetryCount int                        `json:"retryCount"` // 当前重试次数
}

// ==================== 构造函数 ====================

// NewReceiveAlertmanagerWebhookLogic 创建 Webhook 接收逻辑处理器
// 参数：
//   - ctx: 请求上下文
//   - svcCtx: 服务上下文，包含配置和依赖
//
// 返回：
//   - *ReceiveAlertmanagerWebhookLogic: 处理器实例
func NewReceiveAlertmanagerWebhookLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ReceiveAlertmanagerWebhookLogic {
	return &ReceiveAlertmanagerWebhookLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// ==================== 业务逻辑 ====================

// ReceiveAlertmanagerWebhook 接收并处理 Alertmanager Webhook
// 主要流程：
//  1. 验证 Token 认证
//  2. 校验请求数据
//  3. 检查队列长度（防止堆积）
//  4. 将消息投递到 Redis 队列
//
// 参数：
//   - req: Alertmanager webhook 请求数据
//
// 返回：
//   - resp: 响应字符串，成功时返回 "ok"
//   - err: 处理失败时返回错误
func (l *ReceiveAlertmanagerWebhookLogic) ReceiveAlertmanagerWebhook(req *types.AlertmanagerWebhook) (resp string, err error) {
	startTime := time.Now()

	// 步骤 1：Token 认证
	if err := l.validateToken(); err != nil {
		l.Errorf("[Webhook] Token 认证失败: %v", err)
		return "", err
	}

	// 步骤 2：校验请求数据
	if req == nil {
		return "", fmt.Errorf("webhook 数据为空")
	}

	// 如果告警列表为空，直接返回成功（不需要入队）
	if len(req.Alerts) == 0 {
		l.Infof("[Webhook] 接收到空告警列表: receiver=%s, status=%s", req.Receiver, req.Status)
		return "ok", nil
	}

	// 步骤 3：检查队列长度，防止堆积导致 Redis 内存溢出
	queueLen, err := l.svcCtx.Cache.Llen(AlertWebhookQueueKey)
	if err != nil {
		// 获取队列长度失败，记录日志但不阻止消息入队
		l.Errorf("[Webhook] 获取队列长度失败: %v", err)
	} else if queueLen > MaxQueueLength {
		// 队列已满，拒绝新消息
		l.Errorf("[Webhook] 队列已满，拒绝新消息: queueLen=%d, maxLen=%d", queueLen, MaxQueueLength)
		return "", fmt.Errorf("告警队列已满，请稍后重试")
	}

	// 步骤 4：构建队列消息
	message := &WebhookMessage{
		Webhook:    req,
		ReceivedAt: time.Now().Unix(),
		MessageID:  l.generateMessageID(req),
		RetryCount: 0,
	}

	// 序列化消息为 JSON
	messageData, err := json.Marshal(message)
	if err != nil {
		l.Errorf("[Webhook] 序列化消息失败: %v", err)
		return "", fmt.Errorf("序列化消息失败: %w", err)
	}

	// 步骤 5：投递消息到 Redis 队列
	// 使用 LPUSH 将消息放到队列头部，消费者使用 BRPOP 从尾部获取（FIFO）
	result, err := l.svcCtx.Cache.Lpush(AlertWebhookQueueKey, string(messageData))
	if err != nil {
		l.Errorf("[Webhook] 投递消息失败: key=%s, error=%v", AlertWebhookQueueKey, err)
		return "", fmt.Errorf("投递消息失败: %w", err)
	}

	elapsed := time.Since(startTime).Milliseconds()
	l.Infof("[Webhook] 成功投递告警: receiver=%s, status=%s, alerts=%d, messageId=%s, queueLen=%d, elapsed=%dms",
		req.Receiver, req.Status, len(req.Alerts), message.MessageID, result, elapsed)

	return "ok", nil
}

// validateToken 验证请求的 Token
// 从 context 中安全地获取 token 并进行校验
// 返回：
//   - error: 认证失败时返回错误
func (l *ReceiveAlertmanagerWebhookLogic) validateToken() error {
	// 获取预期的 Token（从配置中读取）
	expectedToken := l.svcCtx.Config.Webhook.Token

	// 如果未配置 Token，跳过认证（但记录警告）
	if expectedToken == "" {
		l.Errorf("[Webhook] 警告：未配置 Webhook Token，跳过认证")
		return nil
	}

	// 安全地从 context 获取 token
	tokenValue := l.ctx.Value("token")
	if tokenValue == nil {
		return fmt.Errorf("缺少认证 Token")
	}

	// 安全地进行类型断言
	token, ok := tokenValue.(string)
	if !ok {
		return fmt.Errorf("Token 类型错误")
	}

	if token == "" {
		return fmt.Errorf("缺少认证 Token")
	}

	// 支持 Bearer Token 格式，自动去除前缀
	if len(token) > 7 && token[:7] == "Bearer " {
		token = token[7:]
	}

	// 校验 Token 是否匹配
	if token != expectedToken {
		// 注意：不要在错误信息中包含实际的 token 值，避免安全风险
		return fmt.Errorf("Token 认证失败")
	}

	return nil
}

// generateMessageID 生成消息唯一标识
// 使用 GroupKey 和纳秒级时间戳组合，确保唯一性
// 参数：
//   - webhook: Alertmanager webhook 数据
//
// 返回：
//   - string: 消息唯一标识
func (l *ReceiveAlertmanagerWebhookLogic) generateMessageID(webhook *types.AlertmanagerWebhook) string {
	groupKey := webhook.GroupKey
	// 如果 GroupKey 为空，使用 receiver 作为前缀
	if groupKey == "" {
		groupKey = webhook.Receiver
	}
	return fmt.Sprintf("%s-%d", groupKey, time.Now().UnixNano())
}
