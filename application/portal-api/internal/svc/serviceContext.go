package svc

import (
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/config"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/client/alertservice"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/client/portalservice"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/client/sitemessagesservice"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/client/storageservice"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/client/sysauthservice"
	"github.com/yanshicheng/kube-nova/common/interceptors"
	"github.com/yanshicheng/kube-nova/common/middleware"
	"github.com/yanshicheng/kube-nova/common/verify"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/zrpc"
)

type ServiceContext struct {
	Config            config.Config
	Cache             *redis.Redis
	PortalRpc         portalservice.PortalService
	SysAuthRpc        sysauthservice.SysAuthService
	StoreRpc          storageservice.StorageService
	Validator         *verify.ValidatorInstance
	AlertPortalRpc    alertservice.AlertService
	JWTAuthMiddleware rest.Middleware
	SiteMessagesRpc   sitemessagesservice.SiteMessagesService
	SiteMessageHub    *SiteMessageHub
}

func NewServiceContext(c config.Config) *ServiceContext {
	validator, err := verify.InitValidator(verify.LocaleZH)
	if err != nil {
		panic(err)
	}

	rdb := redis.MustNewRedis(c.Cache)

	// 自定义拦截器
	protalRpc := zrpc.MustNewClient(c.PortalRpc,
		zrpc.WithUnaryClientInterceptor(interceptors.ClientMetadataInterceptor()),
		zrpc.WithUnaryClientInterceptor(interceptors.ClientErrorInterceptor()),
	)
	alertRpc := zrpc.MustNewClient(c.PortalRpc,
		zrpc.WithUnaryClientInterceptor(interceptors.ClientMetadataInterceptor()),
		zrpc.WithUnaryClientInterceptor(interceptors.ClientErrorInterceptor()),
	)
	authRpc := zrpc.MustNewClient(c.PortalRpc,
		zrpc.WithUnaryClientInterceptor(interceptors.ClientMetadataInterceptor()),
		zrpc.WithUnaryClientInterceptor(interceptors.ClientErrorInterceptor()),
	)
	sysAuthRpc := sysauthservice.NewSysAuthService(authRpc)

	// 初始化并启动 SiteMessageHub
	messageHub := NewSiteMessageHub(rdb)
	messageHub.Start()

	logx.Info("站内消息 WebSocket Hub 已启动")

	return &ServiceContext{
		Config:         c,
		Cache:          rdb,
		Validator:      validator,
		PortalRpc:      portalservice.NewPortalService(protalRpc),
		AlertPortalRpc: alertservice.NewAlertService(alertRpc),
		SysAuthRpc:     sysAuthRpc,
		StoreRpc:       storageservice.NewStorageService(protalRpc),
		JWTAuthMiddleware: middleware.NewJWTAuthMiddleware(
			sysAuthRpc,
		).Handle,
		SiteMessagesRpc: sitemessagesservice.NewSiteMessagesService(protalRpc),
		SiteMessageHub:  messageHub,
	}
}
