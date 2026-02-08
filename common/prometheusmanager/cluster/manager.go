package cluster

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/yanshicheng/kube-nova/common/prometheusmanager/operator"
	"github.com/yanshicheng/kube-nova/common/prometheusmanager/types"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

const (
	// Redis key 前缀
	prometheusConfigPrefix = "prometheus:config:"
	prometheusListKey      = "prometheus:list"
	configCacheTTL         = 604800 // Redis 配置缓存时间（秒）- 7天

	// 本地缓存 TTL
	localCacheTTL = 5 * time.Minute // 本地缓存 5 分钟后重新验证
)

// cachedClient 带时间戳的缓存客户端
type cachedClient struct {
	client    types.PrometheusClient
	createdAt time.Time
}

// isExpired 检查缓存是否过期
func (c *cachedClient) isExpired() bool {
	return time.Since(c.createdAt) > localCacheTTL
}

// PrometheusManager Prometheus 多实例管理器（支持多副本部署）
type PrometheusManager struct {
	mu         sync.RWMutex
	localCache map[string]*cachedClient // 本地客户端缓存（带 TTL）
	redis      *redis.Redis             // Redis 存储
	log        logx.Logger
	ctx        context.Context
	cancel     context.CancelFunc // 用于关闭 cleanupLoop
	managerRpc managerservice.ManagerService
}

// NewPrometheusManager 创建 Prometheus 管理器
func NewPrometheusManager(managerRpc managerservice.ManagerService, redisClient *redis.Redis) *PrometheusManager {
	ctx, cancel := context.WithCancel(context.Background())
	pm := &PrometheusManager{
		localCache: make(map[string]*cachedClient),
		redis:      redisClient,
		log:        logx.WithContext(ctx),
		ctx:        ctx,
		cancel:     cancel,
		managerRpc: managerRpc,
	}

	// 启动后台清理任务
	go pm.cleanupLoop()

	return pm
}

// cleanupLoop 定期清理过期的本地缓存
func (m *PrometheusManager) cleanupLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.cleanupExpiredCache()
		case <-m.ctx.Done():
			return
		}
	}
}

// cleanupExpiredCache 清理过期缓存
func (m *PrometheusManager) cleanupExpiredCache() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for uuid, cached := range m.localCache {
		if cached.isExpired() {
			if err := cached.client.Close(); err != nil {
				m.log.Errorf("关闭过期客户端失败: uuid=%s, error=%v", uuid, err)
			}
			delete(m.localCache, uuid)
			m.log.Debugf("清理过期本地缓存: uuid=%s", uuid)
		}
	}
}

// Add 添加 Prometheus 实例配置到 Redis
func (m *PrometheusManager) Add(config *types.PrometheusConfig) error {
	// 先验证连接
	client, err := operator.NewPrometheusClient(m.ctx, config)
	if err != nil {
		m.log.Errorf("创建 Prometheus 客户端失败: uuid=%s, error=%v", config.UUID, err)
		return fmt.Errorf("创建 Prometheus 客户端失败: %v", err)
	}

	if err := client.Ping(); err != nil {
		client.Close()
		m.log.Errorf("连接 Prometheus 失败: uuid=%s, error=%v", config.UUID, err)
		return fmt.Errorf("连接 Prometheus 失败: %v", err)
	}

	// 序列化配置
	configBytes, err := json.Marshal(config)
	if err != nil {
		client.Close()
		return fmt.Errorf("序列化配置失败: %v", err)
	}

	// 存储到 Redis
	configKey := prometheusConfigPrefix + config.UUID
	err = m.redis.SetexCtx(m.ctx, configKey, string(configBytes), configCacheTTL)
	if err != nil {
		client.Close()
		return fmt.Errorf("保存配置到 Redis 失败: %v", err)
	}

	// 添加到列表
	_, err = m.redis.SaddCtx(m.ctx, prometheusListKey, config.UUID)
	if err != nil {
		m.log.Errorf("添加到列表失败: %v", err)
	}

	// 更新本地缓存
	m.mu.Lock()
	if oldCached, exists := m.localCache[config.UUID]; exists {
		oldCached.client.Close()
	}
	m.localCache[config.UUID] = &cachedClient{
		client:    client,
		createdAt: time.Now(),
	}
	m.mu.Unlock()

	m.log.Infof("Prometheus 客户端添加成功: uuid=%s, endpoint=%s", config.UUID, config.Endpoint)
	return nil
}

// Delete 删除 Prometheus 实例
func (m *PrometheusManager) Delete(uuid string) error {
	// 从 Redis 删除配置
	configKey := prometheusConfigPrefix + uuid
	_, err := m.redis.DelCtx(m.ctx, configKey)
	if err != nil {
		m.log.Errorf("从 Redis 删除配置失败: uuid=%s, error=%v", uuid, err)
	}

	// 从列表移除
	_, err = m.redis.SremCtx(m.ctx, prometheusListKey, uuid)
	if err != nil {
		m.log.Errorf("从列表移除失败: %v", err)
	}

	// 关闭本地客户端
	m.mu.Lock()
	if cached, exists := m.localCache[uuid]; exists {
		if err := cached.client.Close(); err != nil {
			m.log.Errorf("关闭 Prometheus 客户端失败: uuid=%s, error=%v", uuid, err)
		}
		delete(m.localCache, uuid)
	}
	m.mu.Unlock()

	m.log.Infof("Prometheus 客户端删除成功: uuid=%s", uuid)
	return nil
}

// getConfigFromRedis 从 Redis 获取配置
func (m *PrometheusManager) getConfigFromRedis(uuid string) (*types.PrometheusConfig, error) {
	configKey := prometheusConfigPrefix + uuid
	configStr, err := m.redis.GetCtx(m.ctx, configKey)
	if err != nil {
		if err == redis.Nil {
			return nil, fmt.Errorf("配置不存在")
		}
		return nil, fmt.Errorf("从 Redis 获取配置失败: %v", err)
	}

	if configStr == "" {
		return nil, fmt.Errorf("配置不存在")
	}

	var config types.PrometheusConfig
	if err := json.Unmarshal([]byte(configStr), &config); err != nil {
		return nil, fmt.Errorf("反序列化配置失败: %v", err)
	}

	return &config, nil
}

// Get 获取 Prometheus 客户端
func (m *PrometheusManager) Get(uuid string) (types.PrometheusClient, error) {
	// 1. 先尝试从本地缓存获取（未过期的）
	m.mu.RLock()
	cached, exists := m.localCache[uuid]
	m.mu.RUnlock()

	if exists && !cached.isExpired() {
		m.log.Debugf("从本地缓存获取 Prometheus 客户端: uuid=%s", uuid)
		return cached.client, nil
	}

	// 2. 缓存不存在或已过期，从 Redis 获取配置
	config, err := m.getConfigFromRedis(uuid)
	if err == nil {
		// Redis 中有配置，创建客户端
		return m.createAndCacheClient(config)
	}

	// 3. Redis 中没有配置，从 RPC 加载
	m.log.Infof("Redis 中不存在配置，从 RPC 加载 Prometheus: uuid=%s", uuid)
	return m.loadFromRPC(uuid)
}

// createAndCacheClient 创建客户端并缓存到本地
func (m *PrometheusManager) createAndCacheClient(config *types.PrometheusConfig) (types.PrometheusClient, error) {
	client, err := operator.NewPrometheusClient(m.ctx, config)
	if err != nil {
		m.log.Errorf("创建 Prometheus 客户端失败: uuid=%s, error=%v", config.UUID, err)
		return nil, fmt.Errorf("创建 Prometheus 客户端失败: %v", err)
	}

	if err := client.Ping(); err != nil {
		client.Close()
		m.log.Errorf("连接 Prometheus 失败: uuid=%s, error=%v", config.UUID, err)
		return nil, fmt.Errorf("连接 Prometheus 失败: %v", err)
	}

	// 缓存到本地
	m.mu.Lock()
	if oldCached, exists := m.localCache[config.UUID]; exists {
		oldCached.client.Close()
	}
	m.localCache[config.UUID] = &cachedClient{
		client:    client,
		createdAt: time.Now(),
	}
	m.mu.Unlock()

	m.log.Infof("成功创建并缓存 Prometheus 客户端: uuid=%s", config.UUID)
	return client, nil
}

// loadFromRPC 从 RPC 加载配置
func (m *PrometheusManager) loadFromRPC(uuid string) (types.PrometheusClient, error) {
	prom, err := m.managerRpc.AppDefaultConfig(m.ctx, &managerservice.AppDefaultConfigReq{
		ClusterUuid: uuid,
		AppType:     1,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			m.log.Errorf("数据库中不存在该 Prometheus: uuid=%s", uuid)
			return nil, fmt.Errorf("prometheus 不存在: %s", uuid)
		}
		m.log.Errorf("查询数据库失败: uuid=%s, error=%v", uuid, err)
		return nil, fmt.Errorf("查询数据库失败: %v", err)
	}
	url := ""
	if prom.Protocol == "https" {
		url = fmt.Sprintf("https://%s:%d", prom.AppUrl, prom.Port)
	} else {
		url = fmt.Sprintf("http://%s:%d", prom.AppUrl, prom.Port)
	}

	// 构建配置
	config := &types.PrometheusConfig{
		UUID:     uuid,
		Name:     uuid,
		Endpoint: url,
		Username: prom.Username,
		Password: prom.Password,
		Insecure: prom.InsecureSkipVerify == 1,
		Timeout:  3,
	}

	// 添加到 Redis 和本地缓存
	if err := m.Add(config); err != nil {
		m.log.Errorf("添加 Prometheus 客户端失败: uuid=%s, error=%v", uuid, err)
		return nil, fmt.Errorf("添加 Prometheus 客户端失败: %v", err)
	}

	// 从本地缓存获取
	m.mu.RLock()
	cached, exists := m.localCache[uuid]
	m.mu.RUnlock()

	if !exists {
		m.log.Errorf("添加后仍无法获取客户端: uuid=%s", uuid)
		return nil, fmt.Errorf("获取客户端失败")
	}

	m.log.Infof("成功从 RPC 加载并创建 Prometheus 客户端: uuid=%s", uuid)
	return cached.client, nil
}

// GetByID 通过数据库 ID 获取 Prometheus 客户端
func (m *PrometheusManager) GetByID(id uint64) (types.PrometheusClient, error) {
	if id == 0 {
		return nil, fmt.Errorf("ID 不能为空或为0")
	}

	m.log.Debugf("通过 ID 获取 Prometheus 客户端: id=%d", id)

	// 通过 RPC 获取项目集群信息（包含 ClusterUuid）
	resp, err := m.managerRpc.ProjectClusterGetById(m.ctx, &managerservice.GetOnecProjectClusterByIdReq{
		Id: id,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			m.log.Errorf("数据库中不存在该集群记录: id=%d", id)
			return nil, fmt.Errorf("集群记录不存在: id=%d", id)
		}
		m.log.Errorf("查询集群记录失败: id=%d, error=%v", id, err)
		return nil, fmt.Errorf("查询集群记录失败: %v", err)
	}

	if resp.Data == nil {
		m.log.Errorf("返回的集群数据为空: id=%d", id)
		return nil, fmt.Errorf("集群数据为空")
	}

	clusterUuid := resp.Data.ClusterUuid
	if clusterUuid == "" {
		m.log.Errorf("集群 UUID 为空: id=%d", id)
		return nil, fmt.Errorf("集群 UUID 为空")
	}

	// 使用 ClusterUuid 获取 Prometheus 客户端
	m.log.Debugf("获取到集群 UUID: id=%d, uuid=%s", id, clusterUuid)
	return m.Get(clusterUuid)
}

// List 列出所有 Prometheus 实例的 UUID
func (m *PrometheusManager) List() ([]string, error) {
	members, err := m.redis.SmembersCtx(m.ctx, prometheusListKey)
	if err != nil {
		m.log.Errorf("从 Redis 获取列表失败: %v", err)
		return nil, fmt.Errorf("获取列表失败: %v", err)
	}
	return members, nil
}

// Count 返回 Prometheus 实例数量
func (m *PrometheusManager) Count() (int, error) {
	count, err := m.redis.ScardCtx(m.ctx, prometheusListKey)
	if err != nil {
		m.log.Errorf("获取数量失败: %v", err)
		return 0, fmt.Errorf("获取数量失败: %v", err)
	}
	return int(count), nil
}

// Update 更新 Prometheus 实例配置
func (m *PrometheusManager) Update(config *types.PrometheusConfig) error {
	if err := m.Delete(config.UUID); err != nil {
		return fmt.Errorf("删除旧客户端失败: %v", err)
	}

	if err := m.Add(config); err != nil {
		return fmt.Errorf("添加新客户端失败: %v", err)
	}

	m.log.Infof("更新 Prometheus 客户端成功: uuid=%s", config.UUID)
	return nil
}

// Reload 重新加载 Prometheus 实例
func (m *PrometheusManager) Reload(uuid string) error {
	// 删除本地缓存，强制下次重新获取
	m.mu.Lock()
	if cached, exists := m.localCache[uuid]; exists {
		cached.client.Close()
		delete(m.localCache, uuid)
	}
	m.mu.Unlock()

	_, err := m.Get(uuid)
	if err != nil {
		m.log.Errorf("重新加载客户端失败: uuid=%s, error=%v", uuid, err)
		return fmt.Errorf("重新加载客户端失败: %v", err)
	}

	m.log.Infof("重新加载 Prometheus 客户端成功: uuid=%s", uuid)
	return nil
}

// Close 关闭所有本地缓存的客户端
func (m *PrometheusManager) Close() error {
	// 首先取消 context，停止 cleanupLoop
	if m.cancel != nil {
		m.cancel()
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	var lastErr error
	for uuid, cached := range m.localCache {
		if err := cached.client.Close(); err != nil {
			m.log.Errorf("关闭 Prometheus 客户端失败: uuid=%s, error=%v", uuid, err)
			lastErr = err
		}
	}

	m.localCache = make(map[string]*cachedClient)
	m.log.Info("所有本地 Prometheus 客户端已关闭")
	return lastErr
}

// CleanLocalCache 清理本地缓存（可用于手动清理）
func (m *PrometheusManager) CleanLocalCache() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for uuid, cached := range m.localCache {
		if err := cached.client.Close(); err != nil {
			m.log.Errorf("关闭客户端失败: uuid=%s, error=%v", uuid, err)
		}
		delete(m.localCache, uuid)
	}

	m.log.Info("本地缓存已清理")
}

// ==================== Pod 指标聚合查询 ====================

// GetPodMetricsFromAll 从所有 Prometheus 实例获取 Pod 指标
func (m *PrometheusManager) GetPodMetricsFromAll(namespace, pod string, timeRange *types.TimeRange) (map[string]*types.PodOverview, error) {
	m.log.Infof("从所有 Prometheus 获取 Pod 指标: namespace=%s, pod=%s", namespace, pod)

	uuids, err := m.List()
	if err != nil {
		return nil, fmt.Errorf("获取实例列表失败: %v", err)
	}

	results := make(map[string]*types.PodOverview)
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, uuid := range uuids {
		wg.Add(1)
		go func(uuid string) {
			defer wg.Done()

			client, err := m.Get(uuid)
			if err != nil {
				m.log.Errorf("获取 Prometheus 客户端失败 %s: %v", uuid, err)
				return
			}

			overview, err := client.Pod().GetPodOverview(namespace, pod, timeRange)
			if err != nil {
				m.log.Errorf("从 Prometheus %s 获取指标失败: %v", uuid, err)
				return
			}

			mu.Lock()
			results[uuid] = overview
			mu.Unlock()
		}(uuid)
	}

	wg.Wait()

	m.log.Infof("从 %d 个 Prometheus 实例获取到指标", len(results))
	return results, nil
}

// ListPodMetricsFromCluster 从指定 Prometheus 实例获取命名空间下的所有 Pod 指标
func (m *PrometheusManager) ListPodMetricsFromCluster(uuid, namespace string, timeRange *types.TimeRange) ([]types.PodOverview, error) {
	m.log.Infof("从 Prometheus 获取 Pod 列表: uuid=%s, namespace=%s", uuid, namespace)

	client, err := m.Get(uuid)
	if err != nil {
		return nil, err
	}

	overviews, err := client.Pod().ListPodsMetrics(namespace, timeRange)
	if err != nil {
		m.log.Errorf("获取 Pod 列表失败: uuid=%s, error=%v", uuid, err)
		return nil, fmt.Errorf("获取 Pod 列表失败: %v", err)
	}

	m.log.Infof("获取 Pod 列表成功: 找到 %d 个 Pod", len(overviews))
	return overviews, nil
}

// GetTopPodsByCPU 获取 CPU 使用排行
func (m *PrometheusManager) GetTopPodsByCPU(uuid, namespace string, limit int, timeRange *types.TimeRange) ([]types.PodRanking, error) {
	m.log.Infof("获取 CPU Top Pods: uuid=%s, namespace=%s, limit=%d", uuid, namespace, limit)

	client, err := m.Get(uuid)
	if err != nil {
		return nil, err
	}

	rankings, err := client.Pod().GetTopPodsByCPU(namespace, limit, timeRange)
	if err != nil {
		m.log.Errorf("获取 CPU Top Pods 失败: %v", err)
		return nil, fmt.Errorf("获取 CPU Top Pods 失败: %v", err)
	}

	m.log.Infof("获取 CPU Top Pods 成功: 返回 %d 个 Pod", len(rankings))
	return rankings, nil
}

// GetTopPodsByMemory 获取内存使用排行
func (m *PrometheusManager) GetTopPodsByMemory(uuid, namespace string, limit int, timeRange *types.TimeRange) ([]types.PodRanking, error) {
	m.log.Infof("获取内存 Top Pods: uuid=%s, namespace=%s, limit=%d", uuid, namespace, limit)

	client, err := m.Get(uuid)
	if err != nil {
		return nil, err
	}

	rankings, err := client.Pod().GetTopPodsByMemory(namespace, limit, timeRange)
	if err != nil {
		m.log.Errorf("获取内存 Top Pods 失败: %v", err)
		return nil, fmt.Errorf("获取内存 Top Pods 失败: %v", err)
	}

	m.log.Infof("获取内存 Top Pods 成功: 返回 %d 个 Pod", len(rankings))
	return rankings, nil
}

// GetTopPodsByNetwork 获取网络使用排行
func (m *PrometheusManager) GetTopPodsByNetwork(uuid, namespace string, limit int, timeRange *types.TimeRange) ([]types.PodRanking, error) {
	m.log.Infof("获取网络 Top Pods: uuid=%s, namespace=%s, limit=%d", uuid, namespace, limit)

	client, err := m.Get(uuid)
	if err != nil {
		return nil, err
	}

	rankings, err := client.Pod().GetTopPodsByNetwork(namespace, limit, timeRange)
	if err != nil {
		m.log.Errorf("获取网络 Top Pods 失败: %v", err)
		return nil, fmt.Errorf("获取网络 Top Pods 失败: %v", err)
	}

	m.log.Infof("获取网络 Top Pods 成功: 返回 %d 个 Pod", len(rankings))
	return rankings, nil
}

// ==================== 通用查询方法 ====================

// Query 执行即时查询
func (m *PrometheusManager) Query(uuid, query string, timestamp *types.TimeRange) ([]types.InstantQueryResult, error) {
	m.log.Infof("执行即时查询: uuid=%s, query=%s", uuid, query)

	client, err := m.Get(uuid)
	if err != nil {
		return nil, err
	}

	var ts *time.Time
	if timestamp != nil && !timestamp.Start.IsZero() {
		ts = &timestamp.Start
	}

	results, err := client.Query(query, ts)
	if err != nil {
		m.log.Errorf("即时查询失败: %v", err)
		return nil, fmt.Errorf("即时查询失败: %v", err)
	}

	m.log.Infof("即时查询成功: 返回 %d 条结果", len(results))
	return results, nil
}

// QueryRange 执行范围查询
func (m *PrometheusManager) QueryRange(uuid, query string, timeRange *types.TimeRange) ([]types.RangeQueryResult, error) {
	m.log.Infof("执行范围查询: uuid=%s, query=%s", uuid, query)

	client, err := m.Get(uuid)
	if err != nil {
		return nil, err
	}

	if timeRange == nil || timeRange.Start.IsZero() || timeRange.End.IsZero() {
		return nil, fmt.Errorf("时间范围不能为空")
	}

	step := timeRange.Step
	if step == "" {
		step = "1m"
	}

	results, err := client.QueryRange(query, timeRange.Start, timeRange.End, step)
	if err != nil {
		m.log.Errorf("范围查询失败: %v", err)
		return nil, fmt.Errorf("范围查询失败: %v", err)
	}

	m.log.Infof("范围查询成功: 返回 %d 个序列", len(results))
	return results, nil
}
