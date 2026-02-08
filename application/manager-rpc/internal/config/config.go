package config

import (
	"time"

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
	DBCache   cache.CacheConf
	Cache     redis.RedisConf
	PortalRpc zrpc.RpcClientConf

	// Leader Election 配置
	LeaderElection LeaderElectionConfig
}

// LeaderElectionConfig Leader 选举配置
type LeaderElectionConfig struct {
	// 是否启用 Leader Election 模式
	// 启用后使用 K8s Lease 进行选举，只有 Leader 运行 Informer
	// 自动按优先级获取 K8s 配置：InCluster -> ~/.kube/config -> 降级到 Redis 模式
	Enabled bool `json:",default=false"`

	// Lease 资源配置（会自动创建）
	LeaseName      string `json:",default=manager-rpc-watch-leader"`
	LeaseNamespace string `json:",default=kube-nova"`

	// 时间配置
	LeaseDuration time.Duration `json:",default=15s"` // 租约有效期
	RenewDeadline time.Duration `json:",default=10s"` // 续约超时
	RetryPeriod   time.Duration `json:",default=2s"`  // 重试间隔
}
