package svc

import (
	"github.com/yanshicheng/kube-nova/application/console-api/internal/common/uploadcore"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/config"
	"github.com/yanshicheng/kube-nova/application/console-rpc/client/repositoryservice"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/client/sysauthservice"
	"github.com/yanshicheng/kube-nova/common/interceptors"
	"github.com/yanshicheng/kube-nova/common/k8smanager/cluster"
	"github.com/yanshicheng/kube-nova/common/middleware"
	cluster2 "github.com/yanshicheng/kube-nova/common/prometheusmanager/cluster"
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
	ManagerRpc        managerservice.ManagerService
	K8sManager        cluster.Manager
	UploadCoreClient  *uploadcore.Service
	RepositoryRpc     repositoryservice.RepositoryService
	PrometheusManager *cluster2.PrometheusManager
}

func NewServiceContext(c config.Config) *ServiceContext {
	validator, err := verify.InitValidator(verify.LocaleZH)
	if err != nil {
		panic(err)
	}

	// 创建 Redis 客户端（共享）
	cacheClient := redis.MustNewRedis(c.Cache)

	managerRpc := zrpc.MustNewClient(c.ManagerRpc,
		zrpc.WithUnaryClientInterceptor(interceptors.ClientMetadataInterceptor()),
		zrpc.WithUnaryClientInterceptor(interceptors.ClientErrorInterceptor()),
	)
	managerService := managerservice.NewManagerService(managerRpc)

	// K8s Manager 使用 Redis
	manager := cluster.NewManager(managerService, cacheClient)

	uploadServer := uploadcore.InitService(c.LocalCacheDir)
	repositoryRpc := zrpc.MustNewClient(c.ConsoleRpc,
		zrpc.WithUnaryClientInterceptor(interceptors.ClientMetadataInterceptor()),
		zrpc.WithUnaryClientInterceptor(interceptors.ClientErrorInterceptor()),
	)
	authRpc := zrpc.MustNewClient(c.PortalRpc,
		zrpc.WithUnaryClientInterceptor(interceptors.ClientMetadataInterceptor()),
		zrpc.WithUnaryClientInterceptor(interceptors.ClientErrorInterceptor()),
	)

	return &ServiceContext{
		Config:    c,
		Cache:     cacheClient,
		Validator: validator,
		JWTAuthMiddleware: middleware.NewJWTAuthMiddleware(
			sysauthservice.NewSysAuthService(authRpc)).Handle,
		ManagerRpc:       managerService,
		K8sManager:       manager,
		UploadCoreClient: uploadServer,
		RepositoryRpc:    repositoryservice.NewRepositoryService(repositoryRpc),
		// 传入 Redis 客户端，使配置在多副本间共享
		PrometheusManager: cluster2.NewPrometheusManager(managerService, cacheClient),
	}
}
