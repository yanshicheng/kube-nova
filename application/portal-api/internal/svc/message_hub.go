// internal/svc/message_hub.go
package svc

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	red "github.com/redis/go-redis/v9"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/logic/common/wsutil"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

const (
	UserMessageChannelPattern = "site_message:push:user:*"
	GlobalMessageChannel      = "site_message:push:global"
)

type SiteMessageHub struct {
	clients      map[uint64]map[*wsutil.WSConnection]bool
	clientsMux   sync.RWMutex
	rdb          *redis.Redis // go-zero Redis（用于 Publish）
	nativeClient *red.Client  // 原生客户端（仅用于 PubSub）
	register     chan *ClientRegistration
	unregister   chan *ClientUnregistration
	broadcast    chan *BroadcastMessage
	ctx          context.Context
	cancel       context.CancelFunc
	logger       logx.Logger
}

type ClientRegistration struct {
	UserId uint64
	Conn   *wsutil.WSConnection
}

type ClientUnregistration struct {
	UserId uint64
	Conn   *wsutil.WSConnection
}

type BroadcastMessage struct {
	UserId  uint64
	Message []byte
}

type RedisMessageData struct {
	MessageID   uint64      `json:"message_id"`
	UUID        string      `json:"uuid"`
	UserID      uint64      `json:"user_id"`
	Title       string      `json:"title"`
	MessageType string      `json:"message_type"`
	Severity    string      `json:"severity"`
	Category    string      `json:"category"`
	IsRead      int64       `json:"is_read"`
	CreatedAt   int64       `json:"created_at"`
	FullMessage interface{} `json:"full_message,omitempty"`
}

// NewSiteMessageHub 创建 Hub
func NewSiteMessageHub(rdb *redis.Redis) *SiteMessageHub {
	ctx, cancel := context.WithCancel(context.Background())

	// 从 go-zero Redis 获取地址和密码，创建原生客户端用于订阅
	nativeClient := red.NewClient(&red.Options{
		Addr:     rdb.Addr,
		Password: rdb.Pass,
		DB:       0,
	})

	return &SiteMessageHub{
		clients:      make(map[uint64]map[*wsutil.WSConnection]bool),
		rdb:          rdb,
		nativeClient: nativeClient,
		register:     make(chan *ClientRegistration, 100),
		unregister:   make(chan *ClientUnregistration, 100),
		broadcast:    make(chan *BroadcastMessage, 256),
		ctx:          ctx,
		cancel:       cancel,
		logger:       logx.WithContext(ctx),
	}
}

func (h *SiteMessageHub) Start() {
	h.logger.Info("站内消息 WebSocket Hub 启动")
	go h.subscribeRedis()
	go h.run()
}

func (h *SiteMessageHub) Stop() {
	h.logger.Info("站内消息 WebSocket Hub 停止中...")
	h.cancel()

	h.clientsMux.Lock()
	for _, clients := range h.clients {
		for conn := range clients {
			conn.Close()
		}
	}
	h.clientsMux.Unlock()

	if h.nativeClient != nil {
		h.nativeClient.Close()
	}

	h.logger.Info("站内消息 WebSocket Hub 已停止")
}

func (h *SiteMessageHub) run() {
	for {
		select {
		case reg := <-h.register:
			h.registerClient(reg)
		case unreg := <-h.unregister:
			h.unregisterClient(unreg)
		case msg := <-h.broadcast:
			if msg.UserId == 0 {
				h.broadcastToAll(msg.Message)
			} else {
				h.broadcastToUser(msg.UserId, msg.Message)
			}
		case <-h.ctx.Done():
			h.logger.Info("Hub 主循环退出")
			return
		}
	}
}

func (h *SiteMessageHub) subscribeRedis() {
	h.logger.Info("开始订阅 Redis 消息频道")

	pubsub := h.nativeClient.PSubscribe(h.ctx, UserMessageChannelPattern, GlobalMessageChannel)
	defer pubsub.Close()

	ch := pubsub.Channel()

	for {
		select {
		case msg := <-ch:
			if msg == nil {
				continue
			}
			h.handleRedisMessage(msg.Channel, msg.Payload)
		case <-h.ctx.Done():
			h.logger.Info("Redis 订阅协程退出")
			return
		}
	}
}

func (h *SiteMessageHub) handleRedisMessage(channel, payload string) {
	if channel == GlobalMessageChannel {
		h.logger.Infof("收到全局消息，准备广播")
		h.broadcast <- &BroadcastMessage{
			UserId:  0,
			Message: []byte(payload),
		}
		return
	}

	var userId uint64
	if _, err := fmt.Sscanf(channel, "site_message:push:user:%d", &userId); err != nil {
		h.logger.Errorf("解析频道失败: channel=%s, error=%v", channel, err)
		return
	}

	var msgData RedisMessageData
	if err := json.Unmarshal([]byte(payload), &msgData); err != nil {
		h.logger.Errorf("解析消息失败: %v", err)
		return
	}

	h.logger.Infof("收到用户消息: userId=%d, messageId=%d, title=%s",
		userId, msgData.MessageID, msgData.Title)

	h.broadcast <- &BroadcastMessage{
		UserId:  userId,
		Message: []byte(payload),
	}
}

func (h *SiteMessageHub) registerClient(reg *ClientRegistration) {
	h.clientsMux.Lock()
	defer h.clientsMux.Unlock()

	if _, ok := h.clients[reg.UserId]; !ok {
		h.clients[reg.UserId] = make(map[*wsutil.WSConnection]bool)
	}
	h.clients[reg.UserId][reg.Conn] = true

	h.logger.Infof("客户端已注册: userId=%d, 当前该用户连接数=%d, 总在线用户=%d",
		reg.UserId, len(h.clients[reg.UserId]), len(h.clients))
}

func (h *SiteMessageHub) unregisterClient(unreg *ClientUnregistration) {
	h.clientsMux.Lock()
	defer h.clientsMux.Unlock()

	if clients, ok := h.clients[unreg.UserId]; ok {
		if _, exists := clients[unreg.Conn]; exists {
			delete(clients, unreg.Conn)
			if len(clients) == 0 {
				delete(h.clients, unreg.UserId)
			}
		}
	}

	h.logger.Infof("客户端已注销: userId=%d, 剩余该用户连接数=%d, 总在线用户=%d",
		unreg.UserId, len(h.clients[unreg.UserId]), len(h.clients))
}

func (h *SiteMessageHub) broadcastToUser(userId uint64, message []byte) {
	h.clientsMux.RLock()
	defer h.clientsMux.RUnlock()

	clients, ok := h.clients[userId]
	if !ok || len(clients) == 0 {
		h.logger.Debugf("用户不在线，跳过推送: userId=%d", userId)
		return
	}

	wsMsg := wsutil.WSMessage{
		Type: wsutil.TypeNewMessage,
		Data: json.RawMessage(message),
	}

	successCount := 0
	failCount := 0

	for conn := range clients {
		if err := conn.WriteJSON(wsMsg); err != nil {
			h.logger.Errorf("推送消息失败: userId=%d, error=%v", userId, err)
			go func(c *wsutil.WSConnection) {
				h.unregister <- &ClientUnregistration{
					UserId: userId,
					Conn:   c,
				}
			}(conn)
			failCount++
		} else {
			successCount++
		}
	}

	h.logger.Infof("推送消息完成: userId=%d, 成功=%d, 失败=%d", userId, successCount, failCount)
}

func (h *SiteMessageHub) broadcastToAll(message []byte) {
	h.clientsMux.RLock()
	defer h.clientsMux.RUnlock()

	wsMsg := wsutil.WSMessage{
		Type: "global_message",
		Data: json.RawMessage(message),
	}

	totalUsers := len(h.clients)
	successCount := 0
	failCount := 0

	for userId, clients := range h.clients {
		for conn := range clients {
			if err := conn.WriteJSON(wsMsg); err != nil {
				h.logger.Errorf("广播消息失败: userId=%d, error=%v", userId, err)
				go func(uid uint64, c *wsutil.WSConnection) {
					h.unregister <- &ClientUnregistration{
						UserId: uid,
						Conn:   c,
					}
				}(userId, conn)
				failCount++
			} else {
				successCount++
			}
		}
	}

	h.logger.Infof("全局广播完成: 在线用户=%d, 成功推送=%d, 失败=%d", totalUsers, successCount, failCount)
}

func (h *SiteMessageHub) Register(userId uint64, conn *wsutil.WSConnection) {
	h.register <- &ClientRegistration{
		UserId: userId,
		Conn:   conn,
	}
}

func (h *SiteMessageHub) Unregister(userId uint64, conn *wsutil.WSConnection) {
	h.unregister <- &ClientUnregistration{
		UserId: userId,
		Conn:   conn,
	}
}

func (h *SiteMessageHub) GetOnlineUserCount() int {
	h.clientsMux.RLock()
	defer h.clientsMux.RUnlock()
	return len(h.clients)
}

func (h *SiteMessageHub) IsUserOnline(userId uint64) bool {
	h.clientsMux.RLock()
	defer h.clientsMux.RUnlock()

	clients, ok := h.clients[userId]
	return ok && len(clients) > 0
}
