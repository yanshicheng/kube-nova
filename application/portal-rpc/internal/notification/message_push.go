package notification

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

const (
	// UserMessageChannelFormat Redis 频道格式（匹配 Hub 的订阅模式）
	UserMessageChannelFormat = "site_message:push:user:%d"
	// GlobalMessageChannel 全局消息频道
	GlobalMessageChannel = "site_message:push:global"
	// maxConcurrentPush 最大并发推送数，避免同时推送过多消息
	maxConcurrentPush = 50
)

// MessagePushService 消息推送服务
type MessagePushService struct {
	redis  *redis.Redis
	logger logx.Logger
	// stats 推送统计
	stats struct {
		totalPushed  int64
		totalFailed  int64
		lastPushTime int64
	}
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
		atomic.AddInt64(&s.stats.totalFailed, 1)
		return errorx.Msg(fmt.Sprintf("序列化消息失败: %v", err))
	}

	// 发布到用户专属频道（支持 K8s 多副本）
	channel := fmt.Sprintf(UserMessageChannelFormat, msg.UserID)
	if _, err := s.redis.PublishCtx(ctx, channel, string(data)); err != nil {
		atomic.AddInt64(&s.stats.totalFailed, 1)
		return errorx.Msg(fmt.Sprintf("发布到 Redis 失败: %v", err))
	}

	atomic.AddInt64(&s.stats.totalPushed, 1)
	atomic.StoreInt64(&s.stats.lastPushTime, time.Now().Unix())

	s.logger.Infof("[消息推送] 已推送消息: userId=%d, channel=%s, title=%s",
		msg.UserID, channel, msg.Title)
	return nil
}

// publishTask 发布任务结构
type publishTask struct {
	channel string
	data    string
	userID  uint64
	title   string
}

// PushBatchMessages 批量推送消息
// 优化: 使用并发控制和预先序列化提高效率
func (s *MessagePushService) PushBatchMessages(ctx context.Context, msgs []*SiteMessageData) error {
	if s.redis == nil {
		s.logger.Error("[消息推送] Redis 未初始化，跳过批量推送")
		return nil
	}

	if len(msgs) == 0 {
		return nil
	}

	// 预先序列化所有消息，避免在并发中重复序列化
	var tasks []publishTask
	serializeErrors := 0

	for _, msg := range msgs {
		redisMsg := RedisMessageData{
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

		data, err := json.Marshal(redisMsg)
		if err != nil {
			s.logger.Errorf("[消息推送] 序列化失败: userId=%d, error=%v", msg.UserID, err)
			serializeErrors++
			continue
		}

		channel := fmt.Sprintf(UserMessageChannelFormat, msg.UserID)
		tasks = append(tasks, publishTask{
			channel: channel,
			data:    string(data),
			userID:  msg.UserID,
			title:   msg.Title,
		})
	}

	if len(tasks) == 0 {
		s.logger.Errorf("[消息推送] 所有消息序列化失败: 总数=%d", len(msgs))
		return errorx.Msg("所有消息序列化失败")
	}

	// 使用带缓冲的 channel 控制并发数
	semaphore := make(chan struct{}, maxConcurrentPush)
	var wg sync.WaitGroup
	var successCount int64
	var failCount int64

	for _, task := range tasks {
		wg.Add(1)
		semaphore <- struct{}{} // 获取信号量

		go func(t publishTask) {
			defer wg.Done()
			defer func() { <-semaphore }() // 释放信号量

			// 检查 context 是否已取消
			select {
			case <-ctx.Done():
				atomic.AddInt64(&failCount, 1)
				return
			default:
			}

			if _, err := s.redis.PublishCtx(ctx, t.channel, t.data); err != nil {
				s.logger.Errorf("[消息推送] 发布失败: channel=%s, userId=%d, error=%v",
					t.channel, t.userID, err)
				atomic.AddInt64(&failCount, 1)
			} else {
				atomic.AddInt64(&successCount, 1)
			}
		}(task)
	}

	wg.Wait()

	// 更新统计
	atomic.AddInt64(&s.stats.totalPushed, successCount)
	atomic.AddInt64(&s.stats.totalFailed, failCount+int64(serializeErrors))
	atomic.StoreInt64(&s.stats.lastPushTime, time.Now().Unix())

	s.logger.Infof("[消息推送] 批量推送完成: 总数=%d, 成功=%d, 失败=%d, 序列化失败=%d",
		len(msgs), successCount, failCount, serializeErrors)

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

// GetStats 获取推送统计信息
func (s *MessagePushService) GetStats() map[string]int64 {
	return map[string]int64{
		"totalPushed":  atomic.LoadInt64(&s.stats.totalPushed),
		"totalFailed":  atomic.LoadInt64(&s.stats.totalFailed),
		"lastPushTime": atomic.LoadInt64(&s.stats.lastPushTime),
	}
}
