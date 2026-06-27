package config

import (
	"github.com/yanshicheng/kube-nova/pkg/storage"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	zrpc.RpcServerConf
	Mongo struct {
		Url string
		Db  string
	}
	PortalRpc  zrpc.RpcClientConf
	Bootstrap struct {
		DefaultDataEnabled    bool
		TektonStepSyncEnabled bool
	}
	Cache       redis.RedisConf
	StorageConf storage.UploaderOptions
}
