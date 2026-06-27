// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package config

import (
	"github.com/yanshicheng/kube-nova/pkg/storage"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	rest.RestConf
	Cache             redis.RedisConf
	PortalRpc         zrpc.RpcClientConf
	DevopsManagerRpc  zrpc.RpcClientConf
	DevopsPipelineRpc zrpc.RpcClientConf
	DevopsQualityRpc  zrpc.RpcClientConf
	StorageConf       storage.UploaderOptions
}
