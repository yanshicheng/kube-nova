package config

import (
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	rest.RestConf
	Cache       redis.RedisConf
	ManagerRpc  zrpc.RpcClientConf
	InjectImage string
	PortalRpc   zrpc.RpcClientConf
}
