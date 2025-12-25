package config

import (
    "github.com/zeromicro/go-zero/zrpc"
    "github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/redis"
    "time"
)

type Config struct {
	zrpc.RpcServerConf
    Mysql struct {
        DataSource      string
        MaxOpenConns    int           // 最大连接数
        MaxIdleConns    int           // 最大空闲连接数
        ConnMaxLifetime time.Duration // 连接的最大生命周期
    }
    DBCache cache.CacheConf
    Cache   redis.RedisConf
}
