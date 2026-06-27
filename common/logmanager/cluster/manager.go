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
	"github.com/yanshicheng/kube-nova/common/logmanager/operator/elasticsearch"
	"github.com/yanshicheng/kube-nova/common/logmanager/operator/loki"
	"github.com/yanshicheng/kube-nova/common/logmanager/types"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

const (
	logConfigPrefix = "logmanager:config:"
	logListKey      = "logmanager:list"
	configCacheTTL  = 604800
	localCacheTTL   = 5 * time.Minute
)

type cachedClient struct {
	client    types.LogClient
	createdAt time.Time
}

func (c *cachedClient) isExpired() bool { return time.Since(c.createdAt) > localCacheTTL }

type LogManager struct {
	mu         sync.RWMutex
	localCache map[string]*cachedClient
	redis      *redis.Redis
	log        logx.Logger
	ctx        context.Context
	cancel     context.CancelFunc
	managerRpc managerservice.ManagerService
}

func NewLogManager(managerRpc managerservice.ManagerService, redisClient *redis.Redis) *LogManager {
	ctx, cancel := context.WithCancel(context.Background())
	m := &LogManager{localCache: make(map[string]*cachedClient), redis: redisClient, log: logx.WithContext(ctx), ctx: ctx, cancel: cancel, managerRpc: managerRpc}
	go m.cleanupLoop()
	return m
}

func (m *LogManager) cleanupLoop() {
	ticker := time.NewTicker(time.Minute)
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

func (m *LogManager) cleanupExpiredCache() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for uuid, cached := range m.localCache {
		if cached.isExpired() {
			_ = cached.client.Close()
			delete(m.localCache, uuid)
		}
	}
}

func (m *LogManager) buildClient(config *types.LogConfig) (types.LogClient, error) {
	switch config.AppCode {
	case "loki":
		return loki.NewClient(m.ctx, config)
	case "elasticsearch", "es":
		return elasticsearch.NewClient(m.ctx, config)
	default:
		return nil, fmt.Errorf("不支持的日志后端: %s", config.AppCode)
	}
}

func (m *LogManager) Add(config *types.LogConfig) error {
	client, err := m.buildClient(config)
	if err != nil {
		return err
	}
	if err := client.Ping(); err != nil {
		_ = client.Close()
		return err
	}
	configBytes, err := json.Marshal(config)
	if err != nil {
		_ = client.Close()
		return err
	}
	if err := m.redis.SetexCtx(m.ctx, logConfigPrefix+config.UUID, string(configBytes), configCacheTTL); err != nil {
		_ = client.Close()
		return err
	}
	_, _ = m.redis.SaddCtx(m.ctx, logListKey, config.UUID)
	m.mu.Lock()
	if old, ok := m.localCache[config.UUID]; ok {
		_ = old.client.Close()
	}
	m.localCache[config.UUID] = &cachedClient{client: client, createdAt: time.Now()}
	m.mu.Unlock()
	return nil
}

func (m *LogManager) getConfigFromRedis(uuid string) (*types.LogConfig, error) {
	configStr, err := m.redis.GetCtx(m.ctx, logConfigPrefix+uuid)
	if err != nil {
		if err == redis.Nil {
			return nil, fmt.Errorf("配置不存在")
		}
		return nil, err
	}
	var cfg types.LogConfig
	if err := json.Unmarshal([]byte(configStr), &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (m *LogManager) Get(uuid string) (types.LogClient, error) {
	m.mu.RLock()
	cached, exists := m.localCache[uuid]
	m.mu.RUnlock()
	if exists && !cached.isExpired() {
		return cached.client, nil
	}
	cfg, err := m.getConfigFromRedis(uuid)
	if err == nil {
		return m.createAndCacheClient(cfg)
	}
	return m.loadFromRPC(uuid)
}

func (m *LogManager) createAndCacheClient(config *types.LogConfig) (types.LogClient, error) {
	client, err := m.buildClient(config)
	if err != nil {
		return nil, err
	}
	if err := client.Ping(); err != nil {
		_ = client.Close()
		return nil, err
	}
	m.mu.Lock()
	if old, ok := m.localCache[config.UUID]; ok {
		_ = old.client.Close()
	}
	m.localCache[config.UUID] = &cachedClient{client: client, createdAt: time.Now()}
	m.mu.Unlock()
	return client, nil
}

func (m *LogManager) loadFromRPC(uuid string) (types.LogClient, error) {
	resp, err := m.managerRpc.AppDefaultConfig(m.ctx, &managerservice.AppDefaultConfigReq{ClusterUuid: uuid, AppType: 2})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("日志组件不存在: %s", uuid)
		}
		return nil, err
	}
	endpoint := fmt.Sprintf("http://%s:%d", resp.AppUrl, resp.Port)
	if resp.Protocol == "https" {
		endpoint = fmt.Sprintf("https://%s:%d", resp.AppUrl, resp.Port)
	}
	cfg := &types.LogConfig{UUID: uuid, Name: uuid, AppCode: resp.AppCode, Endpoint: endpoint, Username: resp.Username, Password: resp.Password, Token: resp.Token, Insecure: resp.InsecureSkipVerify == 1, Timeout: 10}
	if err := m.Add(cfg); err != nil {
		return nil, err
	}
	m.mu.RLock()
	cached, exists := m.localCache[uuid]
	m.mu.RUnlock()
	if !exists {
		return nil, fmt.Errorf("获取日志客户端失败")
	}
	return cached.client, nil
}

func (m *LogManager) Reload(uuid string) error {
	m.mu.Lock()
	if cached, ok := m.localCache[uuid]; ok {
		_ = cached.client.Close()
		delete(m.localCache, uuid)
	}
	m.mu.Unlock()
	_, err := m.Get(uuid)
	return err
}

func (m *LogManager) Close() error {
	if m.cancel != nil {
		m.cancel()
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for uuid, cached := range m.localCache {
		_ = cached.client.Close()
		delete(m.localCache, uuid)
	}
	return nil
}
