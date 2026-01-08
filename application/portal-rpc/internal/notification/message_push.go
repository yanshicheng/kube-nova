package notification

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

const (
	// Redis 频道格式（匹配 Hub 的订阅模式）
	UserMessageChannelFormat = "site_message:push:user:%d"
	GlobalMessageChannel     = "site_message:push:global"
)

// MessagePushService 消息推送服务
type MessagePushService struct {
	redis  *redis.Redis
	logger logx.Logger
}

// NewMessagePushService 创建推送服务
func NewMessagePushService(rdb *redis.Redis) *MessagePushService {
	return &MessagePushService{
		redis:  rdb,
		logger: logx.WithContext(context.Background()),
	}
}

// RedisMessageData 匹配 Hub 的消息格式
type RedisMessageData struct {
	MessageID   uint64 `json:"message_id"`
	UUID        string `json:"uuid"`
	UserID      uint64 `json:"user_id"`
	Title       string `json:"title"`
	Content     string `json:"content"`
	MessageType string `json:"message_type"`
	Severity    string `json:"severity"`
	Category    string `json:"category"`
	IsRead      int64  `json:"is_read"`
	CreatedAt   int64  `json:"created_at"`
	ActionURL   string `json:"action_url,omitempty"`
	ActionText  string `json:"action_text,omitempty"`
}

// PushMessage 推送单条消息
func (s *MessagePushService) PushMessage(ctx context.Context, msg *SiteMessageData) error {
	if s.redis == nil {
		s.logger.Error("[消息推送] Redis 未初始化，跳过推送")
		return nil
	}

	// 构建符合 Hub 格式的消息
	redisMsg := RedisMessageData{
		MessageID:   0, // 如果有 ID 的话可以填充
		UUID:        msg.UUID,
		UserID:      msg.UserID,
		Title:       msg.Title,
		Content:     msg.Content,
		MessageType: msg.MessageType,
		Severity:    msg.Severity,
		Category:    msg.Category,
		IsRead:      msg.IsRead,
		CreatedAt:   time.Now().Unix(),
		ActionURL:   msg.ActionURL,
		ActionText:  msg.ActionText,
	}

	// 序列化
	data, err := json.Marshal(redisMsg)
	if err != nil {
		return fmt.Errorf("序列化消息失败: %w", err)
	}

	// 发布到用户专属频道（支持 K8s 多副本）
	channel := fmt.Sprintf(UserMessageChannelFormat, msg.UserID)
	if _, err := s.redis.PublishCtx(ctx, channel, string(data)); err != nil {
		return fmt.Errorf("发布到 Redis 失败: %w", err)
	}

	s.logger.Infof("[消息推送] 已推送消息: userId=%d, channel=%s, title=%s",
		msg.UserID, channel, msg.Title)
	return nil
}

// PushBatchMessages 批量推送消息
func (s *MessagePushService) PushBatchMessages(ctx context.Context, msgs []*SiteMessageData) error {
	if s.redis == nil {
		s.logger.Error("[消息推送] Redis 未初始化，跳过批量推送")
		return nil
	}

	// 按用户分组
	userMessages := make(map[uint64][]*SiteMessageData)
	for _, msg := range msgs {
		userMessages[msg.UserID] = append(userMessages[msg.UserID], msg)
	}

	// 逐个用户推送
	successCount := 0
	for userId, userMsgs := range userMessages {
		for _, msg := range userMsgs {
			if err := s.PushMessage(ctx, msg); err != nil {
				s.logger.Errorf("[消息推送] 推送失败: userId=%d, error=%v", userId, err)
				// 继续推送其他消息，不中断
			} else {
				successCount++
			}
		}
	}

	s.logger.Infof("[消息推送] 批量推送完成: 总数=%d, 成功=%d, 用户数=%d",
		len(msgs), successCount, len(userMessages))
	return nil
}

// OnMessageCreated 实现 MessagePushCallback 接口
func (s *MessagePushService) OnMessageCreated(ctx context.Context, msg *SiteMessageData) error {
	return s.PushMessage(ctx, msg)
}

// OnBatchMessagesCreated 实现 MessagePushCallback 接口
func (s *MessagePushService) OnBatchMessagesCreated(ctx context.Context, msgs []*SiteMessageData) error {
	return s.PushBatchMessages(ctx, msgs)
}
