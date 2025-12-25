package cluster

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

const (
	// Redis 集群列表 key（Set 类型，存储所有集群 ID）
	clusterSetKey = "ikubeops:k8s:clusters"
)

var (
	managerInstance Manager
	managerOnce     sync.Once
)

// manager 集群管理器实现
type manager struct {
	// Redis 客户端（用于存储集群列表）
	redis *redis.Redis

	// 本地客户端缓存（只缓存已创建的 client 实例）
	localClients map[string]*clientWrapper
	mu           sync.RWMutex

	managerRpc managerservice.ManagerService

	// 状态
	closed bool

	// 集群初始化等待组
	initWg sync.WaitGroup

	// 报警回调
	alertCallback func(clusterID string, err error)
}

// clientWrapper 客户端包装器
type clientWrapper struct {
	client     Client
	config     Config
	createTime time.Time
	mu         sync.RWMutex
	lastAccess time.Time
	metrics    WrapperMetrics
}

// WrapperMetrics 包装器指标
type WrapperMetrics struct {
	AccessCount   int64
	ErrorCount    int64
	LastErrorTime time.Time
}

// NewManager 获取单例管理器
func NewManager(managerRpc managerservice.ManagerService, rds *redis.Redis) Manager {
	managerOnce.Do(func() {
		if rds == nil {
			logx.Error("Redis 客户端不能为空")
			panic("Redis 客户端不能为空")
		}
		managerInstance = &manager{
			redis:        rds,
			localClients: make(map[string]*clientWrapper),
			managerRpc:   managerRpc,
		}
		logx.Info("集群管理器单例已经创建")
	})
	return managerInstance
}

// 获取集群
func (m *manager) GetCluster(ctx context.Context, uuid string) (Client, error) {
	// 第一次检查（读锁）- 检查本地缓存
	m.mu.RLock()
	wrapper, exists := m.localClients[uuid]
	m.mu.RUnlock()

	if exists {
		// 本地缓存存在，直接使用
		return m.useExistingCluster(ctx, wrapper)
	}

	// 本地缓存不存在，需要添加
	// 使用写锁进行双重检查
	m.mu.Lock()
	defer m.mu.Unlock()

	// 第二次检查
	wrapper, exists = m.localClients[uuid]
	if exists {
		// 其他goroutine已经添加了集群
		return m.useExistingClusterWithLock(ctx, wrapper)
	}

	// 确认本地不存在，检查 Redis 中是否存在
	existsInRedis, err := m.redis.Sismember(clusterSetKey, uuid)
	if err != nil {
		logx.Errorf("检查 Redis 集群列表失败: %v", err)
		// Redis 出错，降级处理，继续尝试从 RPC 获取
	}

	if !existsInRedis {
		// Redis 中也不存在，从 RPC 获取并添加
		logx.Infof("集群 %s 不存在，开始从 RPC 添加", uuid)
	}

	// 获取集群信息
	clusterAuthInfo, err := m.managerRpc.GetClusterAuthInfo(ctx, &managerservice.GetClusterAuthInfoReq{
		ClusterUuid: uuid,
	})
	if err != nil {
		logx.Errorf("获取集群 %s 认证信息失败: %v", uuid, err)
		return nil, fmt.Errorf("获取集群 %s 认证信息失败: %w", uuid, err)
	}

	// 构建集群配置
	clusterConfig := Config{
		Name:     uuid,
		ID:       uuid,
		AuthType: AuthType(clusterAuthInfo.AuthType),
	}

	logx.Infof("添加集群: %s", uuid)

	switch clusterAuthInfo.AuthType {
	case string("kubeconfig"):
		logx.Infof("使用 kubeconfig 认证")
		clusterConfig.KubeConfigData = clusterAuthInfo.KubeFile
	case string("token"):
		clusterConfig.APIServer = clusterAuthInfo.ApiServerHost
		clusterConfig.Token = clusterAuthInfo.Token
		clusterConfig.Insecure = clusterAuthInfo.InsecureSkipVerify == 1
	case string("certificate"):
		clusterConfig.APIServer = clusterAuthInfo.ApiServerHost
		clusterConfig.CertFile = clusterAuthInfo.CertFile
		clusterConfig.KeyFile = clusterAuthInfo.KeyFile
		clusterConfig.CAFile = clusterAuthInfo.CaFile
		clusterConfig.Insecure = clusterAuthInfo.InsecureSkipVerify == 1
	case string("incluster"):
		// incluster 模式配置
	default:
		return nil, fmt.Errorf("不支持的认证模式: %s", clusterAuthInfo.AuthType)
	}

	// 直接在锁保护下添加集群（不调用AddCluster避免重复加锁）
	if err := m.addClusterWithoutLock(clusterConfig); err != nil {
		return nil, fmt.Errorf("添加本地集群失败: %w", err)
	}

	// 添加到 Redis
	if _, err := m.redis.Sadd(clusterSetKey, uuid); err != nil {
		logx.Errorf("添加集群到 Redis 失败: %v", err)
		// 不影响流程，继续执行
	}

	// 获取刚添加的集群
	wrapper = m.localClients[uuid]
	return m.useExistingClusterWithLock(ctx, wrapper)
}

// 使用已存在的集群（无锁版本）
func (m *manager) useExistingCluster(ctx context.Context, wrapper *clientWrapper) (Client, error) {
	if wrapper == nil {
		return nil, fmt.Errorf("集群不存在")
	}

	// 更新访问时间
	wrapper.mu.Lock()
	wrapper.lastAccess = time.Now()
	wrapper.metrics.AccessCount++
	wrapper.mu.Unlock()

	// 检查客户端是否存在
	if wrapper.client == nil {
		logx.Errorf("集群客户端不存在")
		return nil, fmt.Errorf("集群客户端不存在")
	}

	// 为客户端设置上下文
	clientWithContext := wrapper.client.WithContext(ctx)

	return clientWithContext, nil
}

// 使用已存在的集群（在写锁保护下）
func (m *manager) useExistingClusterWithLock(ctx context.Context, wrapper *clientWrapper) (Client, error) {
	if wrapper == nil {
		return nil, fmt.Errorf("集群不存在")
	}

	// 更新访问时间
	wrapper.mu.Lock()
	wrapper.lastAccess = time.Now()
	wrapper.metrics.AccessCount++
	wrapper.mu.Unlock()

	// 检查客户端是否存在
	if wrapper.client == nil {
		logx.Errorf("集群客户端不存在")
		return nil, fmt.Errorf("集群客户端不存在")
	}

	// 为客户端设置上下文
	clientWithContext := wrapper.client.WithContext(ctx)

	return clientWithContext, nil
}

// 添加集群（无锁版本，在调用者已持有写锁时使用）
func (m *manager) addClusterWithoutLock(config Config) error {
	if m.closed {
		logx.Error("集群管理器已关闭")
		return fmt.Errorf("集群管理器已关闭")
	}

	// 检查本地是否已经存在
	if _, exists := m.localClients[config.ID]; exists {
		logx.Errorf("集群 %s 已经存在于本地缓存", config.ID)
		return fmt.Errorf("集群 %s 已经存在", config.ID)
	}

	// 创建集群客户端
	client, err := NewClient(config)
	if err != nil {
		logx.Errorf("创建集群客户端失败: %w", err)
		return err
	}

	// 封装集群客户端
	wrapper := &clientWrapper{
		client:     client,
		config:     config,
		createTime: time.Now(),
		lastAccess: time.Now(),
		metrics: WrapperMetrics{
			AccessCount:   0,
			ErrorCount:    0,
			LastErrorTime: time.Time{},
		},
	}
	m.localClients[config.ID] = wrapper
	logx.Infof("添加集群成功: %s", config.ID)
	return nil
}

// 修改原有的AddCluster方法
func (m *manager) AddCluster(config Config) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 添加到本地
	if err := m.addClusterWithoutLock(config); err != nil {
		return err
	}

	// 添加到 Redis
	if _, err := m.redis.Sadd(clusterSetKey, config.ID); err != nil {
		logx.Errorf("添加集群到 Redis 失败: %v", err)
		// 已经添加到本地了，这里只记录错误
	}

	return nil
}

// 移除集群
func (m *manager) RemoveCluster(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	wrapper, exists := m.localClients[id]
	if !exists {
		// 本地不存在，尝试从 Redis 移除
		if _, err := m.redis.Srem(clusterSetKey, id); err != nil {
			logx.Errorf("从 Redis 移除集群失败: %v", err)
		}
		return fmt.Errorf("集群 %s 不存在于本地缓存", id)
	}

	// 关闭客户端
	if wrapper.client != nil {
		if err := wrapper.client.Close(); err != nil {
			logx.Errorf("关闭集群 %s 客户端失败: %v", id, err)
		}
	}

	// 从本地删除
	delete(m.localClients, id)

	// 从 Redis 删除
	if _, err := m.redis.Srem(clusterSetKey, id); err != nil {
		logx.Errorf("从 Redis 移除集群失败: %v", err)
	}

	logx.Infof("移除集群成功: %s", id)

	return nil
}

// 更新集群
func (m *manager) UpdateCluster(config Config) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	wrapper, exists := m.localClients[config.ID]
	if !exists {
		return fmt.Errorf("集群 %s 不存在于本地缓存", config.ID)
	}

	// 关闭旧客户端
	if wrapper.client != nil {
		err := wrapper.client.Close()
		if err != nil {
			logx.Errorf("关闭集群 %s 旧客户端失败: %v", config.ID, err)
			return err
		}
	}

	// 创建新客户端
	newClient, err := NewClient(config)
	if err != nil {
		logx.Errorf("更新集群 %s 失败: %v", config.ID, err)
		return err
	}

	// 更新包装器
	wrapper.client = newClient
	wrapper.config = config

	logx.Infof("集群 %s 更新成功", config.ID)
	return nil
}

// listCluster 集群列表
func (m *manager) ListClusters() []Client {
	m.mu.RLock()
	defer m.mu.RUnlock()

	clients := make([]Client, 0, len(m.localClients))
	for _, wrapper := range m.localClients {
		if wrapper.client != nil {
			clients = append(clients, wrapper.client)
		}
	}
	return clients
}

// 集群总数（从 Redis 获取）
func (m *manager) GetClusterCount() int {
	count, err := m.redis.Scard(clusterSetKey)
	if err != nil {
		logx.Errorf("获取集群数量失败: %v", err)
		// 降级返回本地数量
		m.mu.RLock()
		defer m.mu.RUnlock()
		return len(m.localClients)
	}
	return int(count)
}

// 关闭集群
func (m *manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return nil
	}

	logx.Info("关闭集群管理器...")

	// 并发关闭所有客户端
	var wg sync.WaitGroup
	for id, wrapper := range m.localClients {
		wg.Add(1)
		go func(id string, wrapper *clientWrapper) {
			defer wg.Done()

			if wrapper.client != nil {
				if err := wrapper.client.Close(); err != nil {
					logx.Errorf("关闭集群 %s 失败: %v", id, err)
				}
			}
		}(id, wrapper)
	}

	// 等待所有客户端关闭
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		logx.Info("所有集群客户端已关闭")
	case <-time.After(30 * time.Second):
		logx.Error("部分集群客户端关闭超时")
	}

	// 清空本地缓存
	m.localClients = make(map[string]*clientWrapper)
	m.closed = true

	logx.Info("集群管理器已关闭")
	return nil
}

// Remove 删除某个集群
func (m *manager) Remove(id string) error {
	return m.RemoveCluster(id)
}
