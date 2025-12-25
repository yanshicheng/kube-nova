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

const (
	// Redis队列Key
	AlertWebhookQueueKey = "alertmanager:webhook:queue"
	// 消息过期时间（防止队列堆积）
	MessageExpireSeconds = 86400 // 24小时
	// Token Header 名称
	TokenHeaderName = "Authorization"
)

type ReceiveAlertmanagerWebhookLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewReceiveAlertmanagerWebhookLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ReceiveAlertmanagerWebhookLogic {
	return &ReceiveAlertmanagerWebhookLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ReceiveAlertmanagerWebhookLogic) ReceiveAlertmanagerWebhook(req *types.AlertmanagerWebhook) (resp string, err error) {
	startTime := time.Now()

	// ========== Token 认证 ==========
	if err := l.validateToken(); err != nil {
		l.Errorf("Token 认证失败: %v", err)
		return "", err
	}

	if req == nil {
		return "", fmt.Errorf("webhook数据为空")
	}

	if len(req.Alerts) == 0 {
		l.Infof("接收到空告警列表: receiver=%s, status=%s", req.Receiver, req.Status)
		return "ok", nil
	}

	message := &WebhookMessage{
		Webhook:    req,
		ReceivedAt: time.Now().Unix(),
		MessageID:  l.generateMessageID(req),
		RetryCount: 0,
	}

	messageData, err := json.Marshal(message)
	if err != nil {
		l.Errorf("序列化webhook消息失败: %v", err)
		return "", fmt.Errorf("序列化消息失败: %w", err)
	}

	result, err := l.svcCtx.Cache.Lpush(AlertWebhookQueueKey, string(messageData))
	if err != nil {
		l.Errorf("投递消息到Redis队列失败: key=%s, error=%v", AlertWebhookQueueKey, err)
		return "", fmt.Errorf("投递消息失败: %w", err)
	}

	elapsed := time.Since(startTime).Milliseconds()
	l.Infof("成功投递告警: receiver=%s, status=%s, alerts=%d, messageId=%s, queueLen=%d, elapsed=%dms",
		req.Receiver, req.Status, len(req.Alerts), message.MessageID, result, elapsed)

	return "ok", nil
}

// validateToken 验证 Token
func (l *ReceiveAlertmanagerWebhookLogic) validateToken() error {
	// 从 HTTP Header 中获取 Token
	token := l.ctx.Value("token").(string)

	// 从配置中获取期望的 Token
	expectedToken := l.svcCtx.Config.Webhook.AlertmanagerToken

	if expectedToken == "" {
		l.Errorf("警告：未配置 Webhook Token，跳过认证")
		return nil
	}

	if token == "" {
		return fmt.Errorf("缺少认证 Token")
	}

	// 支持 Bearer Token 格式
	if len(token) > 7 && token[:7] == "Bearer " {
		token = token[7:]
	}

	if token != expectedToken {
		return fmt.Errorf("token 认证失败, token: %s", token)
	}

	return nil
}

// generateMessageID 生成消息ID
func (l *ReceiveAlertmanagerWebhookLogic) generateMessageID(webhook *types.AlertmanagerWebhook) string {
	return fmt.Sprintf("%s-%d", webhook.GroupKey, time.Now().UnixNano())
}

// WebhookMessage 队列消息结构
type WebhookMessage struct {
	Webhook    *types.AlertmanagerWebhook `json:"webhook"`
	ReceivedAt int64                      `json:"receivedAt"`
	MessageID  string                     `json:"messageId"`
	RetryCount int                        `json:"retryCount"`
}
