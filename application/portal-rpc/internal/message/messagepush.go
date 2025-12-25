package message

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

const (
	// Redis 消息推送频道前缀
	MessagePushChannelPrefix = "site_message:push:"
	// 用户消息频道格式: site_message:push:user:{userId}
	UserMessageChannel = "site_message:push:user:%d"
	// 全局消息频道（系统广播）
	GlobalMessageChannel = "site_message:push:global"
)

// MessagePushData 推送到 Redis 的消息结构
type MessagePushData struct {
	MessageID   uint64 `json:"message_id"`
	UUID        string `json:"uuid"`
	UserID      uint64 `json:"user_id"`
	Title       string `json:"title"`
	MessageType string `json:"message_type"`
	Severity    string `json:"severity"`
	Category    string `json:"category"`
	IsRead      int64  `json:"is_read"`
	CreatedAt   int64  `json:"created_at"`
	// 完整消息内容（可选，用于首次推送）
	FullMessage interface{} `json:"full_message,omitempty"`
}

// MessagePusher 消息推送器
type MessagePusher struct {
	rdb    *redis.Redis
	logger logx.Logger
}

// NewMessagePusher 创建消息推送器
func NewMessagePusher(rdb *redis.Redis) *MessagePusher {
	return &MessagePusher{
		rdb:    rdb,
		logger: logx.WithContext(context.Background()),
	}
}

// PushToUser 推送消息给指定用户
func (p *MessagePusher) PushToUser(ctx context.Context, userId uint64, data *MessagePushData) error {
	channel := fmt.Sprintf(UserMessageChannel, userId)
	return p.publish(ctx, channel, data)
}

// PushToGlobal 推送全局消息（系统广播）
func (p *MessagePusher) PushToGlobal(ctx context.Context, data *MessagePushData) error {
	return p.publish(ctx, GlobalMessageChannel, data)
}

// publish 发布消息到 Redis
func (p *MessagePusher) publish(ctx context.Context, channel string, data *MessagePushData) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		p.logger.Errorf("序列化推送消息失败: %v", err)
		return err
	}

	// 发布到 Redis Pub/Sub
	_, err = p.rdb.PublishCtx(ctx, channel, string(jsonData))
	if err != nil {
		p.logger.Errorf("推送消息到 Redis 失败, channel=%s, error=%v", channel, err)
		return err
	}

	p.logger.Infof("消息推送成功: channel=%s, messageId=%d, userId=%d",
		channel, data.MessageID, data.UserID)
	return nil
}

// BatchPushToUsers 批量推送消息给多个用户
func (p *MessagePusher) BatchPushToUsers(ctx context.Context, userIds []uint64, data *MessagePushData) error {
	for _, userId := range userIds {
		// 为每个用户创建独立的推送数据
		userData := *data
		userData.UserID = userId

		if err := p.PushToUser(ctx, userId, &userData); err != nil {
			p.logger.Errorf("批量推送失败: userId=%d, error=%v", userId, err)
			// 继续推送其他用户，不中断
			continue
		}
	}
	return nil
}

// StoreUnreadCount 更新用户未读消息计数（用于快速查询）
func (p *MessagePusher) StoreUnreadCount(ctx context.Context, userId uint64, count uint64) error {
	key := fmt.Sprintf("site_message:unread_count:%d", userId)
	return p.rdb.SetCtx(ctx, key, fmt.Sprintf("%d", count))
}

// GetUnreadCount 获取用户未读消息计数
func (p *MessagePusher) GetUnreadCount(ctx context.Context, userId uint64) (uint64, error) {
	key := fmt.Sprintf("site_message:unread_count:%d", userId)
	val, err := p.rdb.GetCtx(ctx, key)
	if err != nil {
		if err == redis.Nil {
			return 0, nil
		}
		return 0, err
	}

	var count uint64
	_, err = fmt.Sscanf(val, "%d", &count)
	return count, err
}

// IncrUnreadCount 增加未读消息计数
func (p *MessagePusher) IncrUnreadCount(ctx context.Context, userId uint64) error {
	key := fmt.Sprintf("site_message:unread_count:%d", userId)
	_, err := p.rdb.IncrCtx(ctx, key)
	return err
}

// DecrUnreadCount 减少未读消息计数
func (p *MessagePusher) DecrUnreadCount(ctx context.Context, userId uint64, count int64) error {
	if count <= 0 {
		return nil
	}
	key := fmt.Sprintf("site_message:unread_count:%d", userId)
	_, err := p.rdb.DecrbyCtx(ctx, key, count)
	return err
}
