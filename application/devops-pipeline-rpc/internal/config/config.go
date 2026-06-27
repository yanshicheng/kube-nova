package config

import (
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	zrpc.RpcServerConf
	Mongo struct {
		Url string
		Db  string
	}
	Cache            redis.RedisConf
	DevopsManagerRpc zrpc.RpcClientConf
	DevopsQualityRpc zrpc.RpcClientConf
}
