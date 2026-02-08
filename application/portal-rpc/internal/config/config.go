package config

import (
	"time"

	"github.com/yanshicheng/kube-nova/pkg/storage"
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	zrpc.RpcServerConf
	Mysql struct {
		DataSource      string
		MaxOpenConns    int           // 最大连接数
		MaxIdleConns    int           // 最大空闲连接数
		ConnMaxLifetime time.Duration // 连接的最大生命周期
	}
	DBCache     cache.CacheConf
	Cache       redis.RedisConf
	StorageConf storage.UploaderOptions
	AuthConfig  struct {
		AccessSecret  string
		AccessExpire  int64
		RefreshSecret string
		RefreshExpire int64
		RefreshAfter  int64
	}
	DemoMode   bool
	PortalName string
	PortalUrl  string

	// 告警聚合器配置
	Aggregator AggregatorConfig

	// Leader Election 配置（用于告警聚合器）
	LeaderElection LeaderElectionConfig
}

// SeverityWindowConfig 不同级别的时间窗口配置
type SeverityWindowConfig struct {
	Critical time.Duration `json:",optional"` // Critical 级别窗口（通常为0，立即发送）
	Warning  time.Duration `json:",optional"` // Warning 级别窗口
	Info     time.Duration `json:",optional"` // Info 级别窗口
	Default  time.Duration `json:",optional"` // 默认级别窗口
}

// AggregatorConfig 告警聚合器配置
type AggregatorConfig struct {
	// 是否启用聚合器
	Enabled bool `json:",optional"`

	// 不同级别的聚合时间窗口
	SeverityWindows SeverityWindowConfig `json:",optional"`

	// 最大缓冲区大小
	MaxBufferSize int `json:",optional"`

	// 全局缓冲窗口，用于跨调用聚合
	GlobalBufferWindow time.Duration `json:",optional"`
}

// LeaderElectionConfig Leader 选举配置
type LeaderElectionConfig struct {
	// 是否启用 Leader Election 模式
	// 启用后使用 K8s Lease 进行选举，只有 Leader 运行聚合器处理循环
	// 自动按优先级获取 K8s 配置：InCluster -> ~/.kube/config -> 降级到 Redis 模式
	Enabled bool `json:",optional"`

	// Lease 资源配置（会自动创建）
	LeaseName      string `json:",optional"`
	LeaseNamespace string `json:",optional"`

	// 时间配置
	LeaseDuration time.Duration `json:",optional"` // 租约有效期
	RenewDeadline time.Duration `json:",optional"` // 续约超时
	RetryPeriod   time.Duration `json:",optional"` // 重试间隔
}
