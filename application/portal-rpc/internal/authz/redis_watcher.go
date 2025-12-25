package authz

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/casbin/casbin/v2/persist"
	red "github.com/redis/go-redis/v9"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

const (
	// CasbinPolicyChannel 是Redis策略更新频道
	CasbinPolicyChannel = "casbin:policy:update"
)

// PolicyUpdateMessage 策略更新消息
type PolicyUpdateMessage struct {
	Action    string `json:"action"`    // 动作: reload, add, remove
	Timestamp int64  `json:"timestamp"` // 时间戳
	Source    string `json:"source"`    // 来源实例ID
	RoleCode  string `json:"role_code"` // 变更的角色（可选）
}

// RedisWatcher K8s分布式策略同步器
type RedisWatcher struct {
	redisClient *redis.Redis
	rdb         *red.Client // 原生Redis客户端用于Pub/Sub
	callback    func(string)
	instanceId  string
	ctx         context.Context
	cancel      context.CancelFunc
}

// NewRedisWatcher 创建Watcher
// redisConf: go-zero的Redis配置
func NewRedisWatcher(redisConf redis.RedisConf) (*RedisWatcher, error) {
	ctx, cancel := context.WithCancel(context.Background())

	// 生成实例ID（使用Pod名称或时间戳）
	instanceId := fmt.Sprintf("casbin-%d", time.Now().UnixNano())

	// 创建go-zero Redis客户端（用于普通操作）
	redisClient := redis.MustNewRedis(redisConf)

	// 创建原生Redis客户端（用于Pub/Sub）
	var rdb *red.Client
	if redisConf.Type == redis.ClusterType {
		// Cluster模式（暂不实现，可根据需要添加）
		return nil, fmt.Errorf("暂不支持Redis Cluster模式，请使用Node模式")
	} else {
		// Node模式
		rdb = red.NewClient(&red.Options{
			Addr:     redisConf.Host,
			Password: redisConf.Pass,
		})
	}

	// 测试连接
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("Redis连接失败: %w", err)
	}

	watcher := &RedisWatcher{
		redisClient: redisClient,
		rdb:         rdb,
		instanceId:  instanceId,
		ctx:         ctx,
		cancel:      cancel,
	}

	logx.Infof("[Watcher] 创建实例: %s", instanceId)
	return watcher, nil
}

// SetUpdateCallback 设置策略更新回调
func (w *RedisWatcher) SetUpdateCallback(callback func(string)) error {
	w.callback = callback
	return nil
}

// Update 发布策略更新通知
func (w *RedisWatcher) Update() error {
	return w.publishUpdate("reload", "")
}

// UpdateForAddPolicy 添加策略通知
func (w *RedisWatcher) UpdateForAddPolicy(sec, ptype string, params ...string) error {
	return w.publishUpdate("add", "")
}

// UpdateForRemovePolicy 删除策略通知
func (w *RedisWatcher) UpdateForRemovePolicy(sec, ptype string, params ...string) error {
	return w.publishUpdate("remove", "")
}

// UpdateForRemoveFilteredPolicy 删除过滤策略通知
func (w *RedisWatcher) UpdateForRemoveFilteredPolicy(sec, ptype string, fieldIndex int, fieldValues ...string) error {
	return w.publishUpdate("remove", "")
}

// UpdateForSavePolicy 保存策略通知
func (w *RedisWatcher) UpdateForSavePolicy(sec, ptype string, params ...string) error {
	return w.publishUpdate("reload", "")
}

// UpdateForAddPolicies 批量添加策略通知
func (w *RedisWatcher) UpdateForAddPolicies(sec, ptype string, rules ...[]string) error {
	return w.publishUpdate("reload", "")
}

// UpdateForRemovePolicies 批量删除策略通知
func (w *RedisWatcher) UpdateForRemovePolicies(sec, ptype string, rules ...[]string) error {
	return w.publishUpdate("reload", "")
}

// publishUpdate 发布更新消息
func (w *RedisWatcher) publishUpdate(action, roleCode string) error {
	msg := PolicyUpdateMessage{
		Action:    action,
		Timestamp: time.Now().Unix(),
		Source:    w.instanceId,
		RoleCode:  roleCode,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("序列化消息失败: %w", err)
	}

	// 使用原生Redis客户端发布
	if err := w.rdb.Publish(w.ctx, CasbinPolicyChannel, string(data)).Err(); err != nil {
		return fmt.Errorf("发布消息失败: %w", err)
	}

	logx.Infof("[Watcher] 实例 %s 发布策略更新: action=%s, role=%s",
		w.instanceId, action, roleCode)
	return nil
}

// Start 启动监听
func (w *RedisWatcher) Start() error {
	go w.subscribe()
	logx.Infof("[Watcher] 实例 %s 开始监听策略更新", w.instanceId)
	return nil
}

// subscribe 订阅Redis频道
func (w *RedisWatcher) subscribe() {
	pubsub := w.rdb.Subscribe(w.ctx, CasbinPolicyChannel)
	defer pubsub.Close()

	ch := pubsub.Channel()

	for {
		select {
		case <-w.ctx.Done():
			logx.Infof("[Watcher] 实例 %s 停止监听", w.instanceId)
			return

		case msg := <-ch:
			if msg == nil {
				continue
			}

			// 解析消息
			var policyMsg PolicyUpdateMessage
			if err := json.Unmarshal([]byte(msg.Payload), &policyMsg); err != nil {
				logx.Errorf("[Watcher] 解析消息失败: %v", err)
				continue
			}

			// 忽略自己发送的消息
			if policyMsg.Source == w.instanceId {
				continue
			}

			logx.Infof("[Watcher] 实例 %s 收到策略更新通知: action=%s, from=%s",
				w.instanceId, policyMsg.Action, policyMsg.Source)

			// 触发回调
			if w.callback != nil {
				w.callback(policyMsg.Action)
			}
		}
	}
}

// Close 关闭Watcher
func (w *RedisWatcher) Close() {
	w.cancel()
	if w.rdb != nil {
		w.rdb.Close()
	}
	logx.Infof("[Watcher] 实例 %s 已关闭", w.instanceId)
}

// 确保实现了persist.Watcher接口
var _ persist.Watcher = (*RedisWatcher)(nil)
