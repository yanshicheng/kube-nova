package config

import (
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/zrpc"
)

type Webhook struct {
	AlertmanagerToken string `json:"AlertmanagerToken"`
}
type Config struct {
	rest.RestConf
	Cache      redis.RedisConf
	ManagerRpc zrpc.RpcClientConf
	PortalRpc  zrpc.RpcClientConf
	Webhook    Webhook `json:"Webhook"`
}
