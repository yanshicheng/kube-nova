package authz

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
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
	Action    string `json:"action"`    // 动作类型，支持 reload、add、remove
	Timestamp int64  `json:"timestamp"` // 消息时间戳
	Source    string `json:"source"`    // 消息来源实例标识
	RoleCode  string `json:"role_code"` // 变更的角色编码，可选
}

// RedisWatcher 分布式策略同步器，支持单节点和集群模式
type RedisWatcher struct {
	redisClient *redis.Redis        // go-zero Redis客户端，用于普通操作
	rdb         red.UniversalClient // 原生Redis客户端，用于发布订阅，支持单节点和集群
	callback    func(string)        // 策略更新回调函数
	instanceId  string              // 当前实例唯一标识
	ctx         context.Context     // 上下文
	cancel      context.CancelFunc  // 取消函数
	isCluster   bool                // 是否为集群模式
}

// NewRedisWatcher 创建分布式策略同步器
// 参数 redisConf 为 go-zero 的 Redis 配置，支持单节点和集群模式
func NewRedisWatcher(redisConf redis.RedisConf) (*RedisWatcher, error) {
	ctx, cancel := context.WithCancel(context.Background())

	// 生成实例唯一标识，使用时间戳确保唯一性
	instanceId := fmt.Sprintf("casbin-%d", time.Now().UnixNano())

	// 创建 go-zero Redis 客户端，用于普通操作
	redisClient := redis.MustNewRedis(redisConf)

	// 判断是否为集群模式
	isCluster := redisConf.Type == redis.ClusterType

	// 创建原生 Redis 客户端，用于发布订阅
	var rdb red.UniversalClient
	if isCluster {
		// 集群模式，解析多个节点地址
		hosts := parseClusterHosts(redisConf.Host)
		rdb = red.NewClusterClient(&red.ClusterOptions{
			Addrs:    hosts,
			Password: redisConf.Pass,
			Username: redisConf.User,
		})
		logx.Infof("[Watcher] 使用 Redis 集群模式，节点数量: %d", len(hosts))
	} else {
		// 单节点模式
		rdb = red.NewClient(&red.Options{
			Addr:     redisConf.Host,
			Password: redisConf.Pass,
			Username: redisConf.User,
		})
		logx.Info("[Watcher] 使用 Redis 单节点模式")
	}

	// 测试连接是否正常
	if err := rdb.Ping(ctx).Err(); err != nil {
		cancel()
		return nil, fmt.Errorf("redis 连接失败: %w", err)
	}

	watcher := &RedisWatcher{
		redisClient: redisClient,
		rdb:         rdb,
		instanceId:  instanceId,
		ctx:         ctx,
		cancel:      cancel,
		isCluster:   isCluster,
	}

	logx.Infof("[Watcher] 创建实例成功: %s", instanceId)
	return watcher, nil
}

// parseClusterHosts 解析集群节点地址
// 支持逗号分隔的多个地址格式，例如: "host1:6379,host2:6379,host3:6379"
func parseClusterHosts(host string) []string {
	hosts := strings.Split(host, ",")
	result := make([]string, 0, len(hosts))
	for _, h := range hosts {
		h = strings.TrimSpace(h)
		if h != "" {
			result = append(result, h)
		}
	}
	return result
}

// SetUpdateCallback 设置策略更新回调函数
// 当收到其他实例的策略更新通知时，会调用此回调函数
func (w *RedisWatcher) SetUpdateCallback(callback func(string)) error {
	w.callback = callback
	return nil
}

// Update 发布策略更新通知
// 通知所有实例重新加载策略
func (w *RedisWatcher) Update() error {
	return w.publishUpdate("reload", "")
}

// UpdateForAddPolicy 添加策略时发布通知
func (w *RedisWatcher) UpdateForAddPolicy(sec, ptype string, params ...string) error {
	return w.publishUpdate("add", "")
}

// UpdateForRemovePolicy 删除策略时发布通知
func (w *RedisWatcher) UpdateForRemovePolicy(sec, ptype string, params ...string) error {
	return w.publishUpdate("remove", "")
}

// UpdateForRemoveFilteredPolicy 删除过滤策略时发布通知
func (w *RedisWatcher) UpdateForRemoveFilteredPolicy(sec, ptype string, fieldIndex int, fieldValues ...string) error {
	return w.publishUpdate("remove", "")
}

// UpdateForSavePolicy 保存策略时发布通知
func (w *RedisWatcher) UpdateForSavePolicy(sec, ptype string, params ...string) error {
	return w.publishUpdate("reload", "")
}

// UpdateForAddPolicies 批量添加策略时发布通知
func (w *RedisWatcher) UpdateForAddPolicies(sec, ptype string, rules ...[]string) error {
	return w.publishUpdate("reload", "")
}

// UpdateForRemovePolicies 批量删除策略时发布通知
func (w *RedisWatcher) UpdateForRemovePolicies(sec, ptype string, rules ...[]string) error {
	return w.publishUpdate("reload", "")
}

// publishUpdate 发布更新消息到 Redis 频道
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

	// 发布消息到 Redis 频道
	if err := w.rdb.Publish(w.ctx, CasbinPolicyChannel, string(data)).Err(); err != nil {
		return fmt.Errorf("发布消息失败: %w", err)
	}

	logx.Infof("[Watcher] 实例 %s 发布策略更新: action=%s, role=%s",
		w.instanceId, action, roleCode)
	return nil
}

// Start 启动监听
// 开始订阅 Redis 频道，接收其他实例的策略更新通知
func (w *RedisWatcher) Start() error {
	go w.subscribe()
	logx.Infof("[Watcher] 实例 %s 开始监听策略更新", w.instanceId)
	return nil
}

// subscribe 订阅 Redis 频道
func (w *RedisWatcher) subscribe() {
	pubsub := w.rdb.Subscribe(w.ctx, CasbinPolicyChannel)
	defer func(pubsub *red.PubSub) {
		err := pubsub.Close()
		if err != nil {
		}
	}(pubsub)

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

			// 忽略自己发送的消息，避免重复处理
			if policyMsg.Source == w.instanceId {
				continue
			}

			logx.Infof("[Watcher] 实例 %s 收到策略更新通知: action=%s, from=%s",
				w.instanceId, policyMsg.Action, policyMsg.Source)

			// 触发回调函数
			if w.callback != nil {
				w.callback(policyMsg.Action)
			}
		}
	}
}

// Close 关闭 Watcher
// 停止监听并释放资源
func (w *RedisWatcher) Close() {
	w.cancel()
	if w.rdb != nil {
		err := w.rdb.Close()

		if err != nil {
			return
		}
	}
	logx.Infof("[Watcher] 实例 %s 已关闭", w.instanceId)
}

// 确保实现了 persist.Watcher 接口
var _ persist.Watcher = (*RedisWatcher)(nil)
