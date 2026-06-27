// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package svc

import (
	"github.com/yanshicheng/kube-nova/application/devops-api/internal/config"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/client/channelservice"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/client/pipelineconfigservice"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/client/projectservice"
	"github.com/yanshicheng/kube-nova/application/devops-pipeline-rpc/client/executionservice"
	"github.com/yanshicheng/kube-nova/application/devops-quality-rpc/client/qualityservice"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/client/sysauthservice"
	"github.com/yanshicheng/kube-nova/common/interceptors"
	"github.com/yanshicheng/kube-nova/common/middleware"
	"github.com/yanshicheng/kube-nova/common/verify"
	"github.com/yanshicheng/kube-nova/pkg/storage"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/zrpc"
)

type ServiceContext struct {
	Config            config.Config
	Cache             *redis.Redis
	Validator         *verify.ValidatorInstance
	ProjectRpc        projectservice.ProjectService
	ChannelRpc        channelservice.ChannelService
	PipelineRpc       pipelineconfigservice.PipelineConfigService
	PipelineExecRpc   executionservice.ExecutionService
	QualityRpc        qualityservice.QualityService
	SysAuthRpc        sysauthservice.SysAuthService
	Storage           storage.Uploader
	JWTAuthMiddleware rest.Middleware
}

func NewServiceContext(c config.Config) *ServiceContext {
	validator, err := verify.InitValidator(verify.LocaleZH)
	if err != nil {
		panic(err)
	}
	devopsManagerRpc := zrpc.MustNewClient(c.DevopsManagerRpc,
		zrpc.WithUnaryClientInterceptor(interceptors.ClientMetadataInterceptor()),
		zrpc.WithUnaryClientInterceptor(interceptors.ClientErrorInterceptor()),
	)
	portalRpc := zrpc.MustNewClient(c.PortalRpc,
		zrpc.WithUnaryClientInterceptor(interceptors.ClientMetadataInterceptor()),
		zrpc.WithUnaryClientInterceptor(interceptors.ClientErrorInterceptor()),
	)
	devopsPipelineRpc := zrpc.MustNewClient(c.DevopsPipelineRpc,
		zrpc.WithUnaryClientInterceptor(interceptors.ClientMetadataInterceptor()),
		zrpc.WithUnaryClientInterceptor(interceptors.ClientErrorInterceptor()),
	)
	devopsQualityRpc := zrpc.MustNewClient(c.DevopsQualityRpc,
		zrpc.WithUnaryClientInterceptor(interceptors.ClientMetadataInterceptor()),
		zrpc.WithUnaryClientInterceptor(interceptors.ClientErrorInterceptor()),
	)
	sysAuthRpc := sysauthservice.NewSysAuthService(portalRpc)
	var uploader storage.Uploader
	if c.StorageConf.Provider != "" {
		uploader, err = storage.NewUploader(c.StorageConf)
		logx.Must(err)
	}

	return &ServiceContext{
		Config:            c,
		Cache:             redis.MustNewRedis(c.Cache),
		Validator:         validator,
		ProjectRpc:        projectservice.NewProjectService(devopsManagerRpc),
		ChannelRpc:        channelservice.NewChannelService(devopsManagerRpc),
		PipelineRpc:       pipelineconfigservice.NewPipelineConfigService(devopsManagerRpc),
		PipelineExecRpc:   executionservice.NewExecutionService(devopsPipelineRpc),
		QualityRpc:        qualityservice.NewQualityService(devopsQualityRpc),
		SysAuthRpc:        sysAuthRpc,
		Storage:           uploader,
		JWTAuthMiddleware: middleware.NewJWTAuthMiddleware(sysAuthRpc).Handle,
	}
}
