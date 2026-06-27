package config

import (
	"time"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/zrpc"
)

type WebhookConfig struct {
	Token string `json:",optional"`
}

type ElasticsearchFieldMapping struct {
	Timestamp     string `json:",optional"`
	Message       string `json:",optional"`
	ProjectUuid   string `json:",optional"`
	ClusterUuid   string `json:",optional"`
	NamespaceName string `json:",optional"`
	ResourceName  string `json:",optional"`
	PodName       string `json:",optional"`
	ContainerName string `json:",optional"`
	PodIp         string `json:",optional"`
	Host          string `json:",optional"`
	SourceType    string `json:",optional"`
	LogType       string `json:",optional"`
	Level         string `json:",optional"`
}

type ElasticsearchLogSearchConfig struct {
	DataStream   string                    `json:",optional"`
	IndexPattern string                    `json:",optional"`
	FieldMapping ElasticsearchFieldMapping `json:",optional"`
}

type LogHTTPPoolConfig struct {
	MaxIdleConns          int           `json:",default=100"`
	MaxIdleConnsPerHost   int           `json:",default=20"`
	MaxConnsPerHost       int           `json:",default=50"`
	IdleConnTimeout       time.Duration `json:",default=90s"`
	ResponseHeaderTimeout time.Duration `json:",default=15s"`
	TLSHandshakeTimeout   time.Duration `json:",default=10s"`
	ExpectContinueTimeout time.Duration `json:",default=1s"`
}

type LogSearchConfig struct {
	QueryMode     string                       `json:",default=platform"`
	HTTPPool      LogHTTPPoolConfig            `json:",optional"`
	Elasticsearch ElasticsearchLogSearchConfig `json:",optional"`
}

type LogAlertEngineConfig struct {
	Enabled                 bool          `json:",default=true"`
	Mode                    string        `json:",default=platform"`
	EvalInterval            time.Duration `json:",default=60s"`
	MinEvalInterval         time.Duration `json:",default=15s"`
	MaxEvalInterval         time.Duration `json:",default=300s"`
	RuleSyncInterval        time.Duration `json:",default=30s"`
	LockEnabled             bool          `json:",default=true"`
	LockTTL                 time.Duration `json:",default=45s"`
	NotifyRetryScanInterval time.Duration `json:",default=15s"`
	NotifyRetryBase         time.Duration `json:",default=30s"`
	SilenceWindow           time.Duration `json:",default=10m"`
	MaxNotifyRetry          int           `json:",default=5"`
	ClusterWorkers          int           `json:",default=2"`
	MaxBatchRules           int           `json:",default=100"`
	MaxQueryLimit           int64         `json:",default=500"`
}

type InspectionEngineConfig struct {
	Enabled        bool          `json:",default=true"`
	ScanInterval   time.Duration `json:",default=30s"`
	LockEnabled    bool          `json:",default=true"`
	LockTTL        time.Duration `json:",default=10m"`
	ClusterWorkers int           `json:",default=2"`
	MaxBatchTasks  int           `json:",default=50"`
}

type Config struct {
	zrpc.RpcServerConf
	Mysql struct {
		DataSource      string
		MaxOpenConns    int           // 最大连接数
		MaxIdleConns    int           // 最大空闲连接数
		ConnMaxLifetime time.Duration // 连接的最大生命周期
	}
	DBCache          cache.CacheConf
	Cache            redis.RedisConf
	PortalRpc        zrpc.RpcClientConf
	Webhook          WebhookConfig
	LogSearch        LogSearchConfig
	LogAlertEngine   LogAlertEngineConfig
	InspectionEngine InspectionEngineConfig

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
