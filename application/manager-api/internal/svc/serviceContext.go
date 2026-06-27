package svc

import (
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/config"
	logservice "github.com/yanshicheng/kube-nova/application/manager-rpc/client/logservice"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	portalprojectservice "github.com/yanshicheng/kube-nova/application/portal-rpc/client/portalprojectservice"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/client/storageservice"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/client/sysauthservice"
	"github.com/yanshicheng/kube-nova/common/interceptors"
	logcluster "github.com/yanshicheng/kube-nova/common/logmanager/cluster"
	"github.com/yanshicheng/kube-nova/common/middleware"
	"github.com/yanshicheng/kube-nova/common/verify"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/zrpc"
)

type ServiceContext struct {
	Config            config.Config
	Cache             *redis.Redis
	Validator         *verify.ValidatorInstance
	JWTAuthMiddleware rest.Middleware
	StoreRpc          storageservice.StorageService
	ManagerRpc        managerservice.ManagerService
	ProjectRpc        portalprojectservice.PortalProjectService
	LogRpc            logservice.LogService
	LogManager        *logcluster.LogManager
}

func NewServiceContext(c config.Config) *ServiceContext {
	validator, err := verify.InitValidator(verify.LocaleZH)
	if err != nil {
		panic(err)
	}
	managerRpc := zrpc.MustNewClient(c.ManagerRpc,
		zrpc.WithUnaryClientInterceptor(interceptors.ClientMetadataInterceptor()),
		zrpc.WithUnaryClientInterceptor(interceptors.ClientErrorInterceptor()),
	)
	storeRpc := zrpc.MustNewClient(c.PortalRpc,
		zrpc.WithUnaryClientInterceptor(interceptors.ClientMetadataInterceptor()),
		zrpc.WithUnaryClientInterceptor(interceptors.ClientErrorInterceptor()),
	)
	projectRpc := zrpc.MustNewClient(c.PortalRpc,
		zrpc.WithUnaryClientInterceptor(interceptors.ClientMetadataInterceptor()),
		zrpc.WithUnaryClientInterceptor(interceptors.ClientErrorInterceptor()),
	)
	authRpc := zrpc.MustNewClient(c.PortalRpc,
		zrpc.WithUnaryClientInterceptor(interceptors.ClientMetadataInterceptor()),
		zrpc.WithUnaryClientInterceptor(interceptors.ClientErrorInterceptor()),
	)
	cacheClient := redis.MustNewRedis(c.Cache)
	managerService := managerservice.NewManagerService(managerRpc)
	return &ServiceContext{
		Config:    c,
		Cache:     cacheClient,
		Validator: validator,
		JWTAuthMiddleware: middleware.NewJWTAuthMiddleware(
			sysauthservice.NewSysAuthService(authRpc)).Handle,
		ManagerRpc: managerService,
		ProjectRpc: portalprojectservice.NewPortalProjectService(projectRpc),
		LogRpc:     logservice.NewLogService(managerRpc),
		LogManager: logcluster.NewLogManager(managerService, cacheClient),
		StoreRpc:   storageservice.NewStorageService(storeRpc),
	}
}
