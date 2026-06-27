package svc

import (
	"context"
	"time"

	portalprojectservice "github.com/yanshicheng/kube-nova/application/portal-rpc/client/portalprojectservice"
	"github.com/yanshicheng/kube-nova/common/interceptors"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/adapter"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/channelcheck"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/config"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/common/devops/channelvars"
	"github.com/yanshicheng/kube-nova/common/devops/channelvars/providers"
	"github.com/yanshicheng/kube-nova/pkg/storage"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/zrpc"
)

type ServiceContext struct {
	Config                   config.Config
	Cache                    *redis.Redis
	ProjectModel             *model.DevopsProjectModel
	ProjectMavenModel        *model.DevopsProjectMavenConfigModel
	ConfigTypeModel          *model.DevopsConfigTypeModel
	ProjectConfigModel       *model.DevopsProjectConfigModel
	ChannelGroupModel        *model.DevopsChannelGroupModel
	ChannelTypeModel         *model.DevopsChannelTypeModel
	ChannelModel             *model.DevopsChannelModel
	CredentialModel          *model.DevopsCredentialModel
	CredentialSyncModel      *model.DevopsCredentialSyncModel
	HostModel                *model.DevopsHostModel
	PipelineUsageModel       *model.DevopsPipelineUsageModel
	ProjectChannelModel      *model.DevopsProjectChannelBindingModel
	ProjectMemberModel       *model.DevopsProjectMemberModel
	SystemModel              *model.DevopsSystemModel
	PipelineEnvModel         *model.DevopsPipelineEnvironmentModel
	StepCategoryModel        *model.DevopsStepCategoryModel
	StepTemplateModel        *model.DevopsStepTemplateModel
	StepSyncModel            *model.DevopsStepSyncModel
	PipelineTemplateModel    *model.DevopsPipelineTemplateModel
	JenkinsAgentModel        *model.DevopsJenkinsAgentModel
	ChannelChecker           *channelcheck.Checker
	StepChannelParamModel    *model.DevopsStepChannelParamModel
	ChannelParamLookup       *stepChannelParamLookup
	Storage                  storage.Uploader
	ChannelVariableSpecModel model.ChannelVariableSpecModel
	PortalRpc                portalprojectservice.PortalProjectService
	ChannelManagerAdapter    *adapter.ChannelManagerAdapter
	CredentialManagerAdapter *adapter.CredentialManagerAdapter
}

func NewServiceContext(c config.Config) *ServiceContext {
	portalRpc := portalprojectservice.NewPortalProjectService(zrpc.MustNewClient(c.PortalRpc, zrpc.WithUnaryClientInterceptor(interceptors.ClientErrorInterceptor())))
	projectModel := model.NewDevopsProjectModel(c.Mongo.Url, c.Mongo.Db)
	projectMavenModel := model.NewDevopsProjectMavenConfigModel(c.Mongo.Url, c.Mongo.Db)
	configTypeModel := model.NewDevopsConfigTypeModel(c.Mongo.Url, c.Mongo.Db)
	projectConfigModel := model.NewDevopsProjectConfigModel(c.Mongo.Url, c.Mongo.Db)
	channelGroupModel := model.NewDevopsChannelGroupModel(c.Mongo.Url, c.Mongo.Db)
	channelTypeModel := model.NewDevopsChannelTypeModel(c.Mongo.Url, c.Mongo.Db)
	channelModel := model.NewDevopsChannelModel(c.Mongo.Url, c.Mongo.Db)
	credentialModel := model.NewDevopsCredentialModel(c.Mongo.Url, c.Mongo.Db)
	credentialSyncModel := model.NewDevopsCredentialSyncModel(c.Mongo.Url, c.Mongo.Db)
	hostModel := model.NewDevopsHostModel(c.Mongo.Url, c.Mongo.Db)
	pipelineUsageModel := model.NewDevopsPipelineUsageModel(c.Mongo.Url, c.Mongo.Db)
	projectChannelModel := model.NewDevopsProjectChannelBindingModel(c.Mongo.Url, c.Mongo.Db)
	projectMemberModel := model.NewDevopsProjectMemberModel(c.Mongo.Url, c.Mongo.Db)
	systemModel := model.NewDevopsSystemModel(c.Mongo.Url, c.Mongo.Db)
	pipelineEnvModel := model.NewDevopsPipelineEnvironmentModel(c.Mongo.Url, c.Mongo.Db)
	stepCategoryModel := model.NewDevopsStepCategoryModel(c.Mongo.Url, c.Mongo.Db)
	stepTemplateModel := model.NewDevopsStepTemplateModel(c.Mongo.Url, c.Mongo.Db)
	stepSyncModel := model.NewDevopsStepSyncModel(c.Mongo.Url, c.Mongo.Db)
	pipelineTemplateModel := model.NewDevopsPipelineTemplateModel(c.Mongo.Url, c.Mongo.Db)
	jenkinsAgentModel := model.NewDevopsJenkinsAgentModel(c.Mongo.Url, c.Mongo.Db)
	stepChannelParamModel := model.NewDevopsStepChannelParamModel(c.Mongo.Url, c.Mongo.Db)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if c.Bootstrap.DefaultDataEnabled {
		bootstrapStateModel := model.NewDevopsBootstrapStateModel(c.Mongo.Url, c.Mongo.Db)
		defaultDataApplied, err := bootstrapStateModel.IsVersionApplied(ctx, model.DefaultDataBootstrapCode, model.DefaultDataBootstrapVersion)
		logx.Must(err)
		if !defaultDataApplied {
			logx.Infof("开始初始化 DevOps 默认数据，版本: %s", model.DefaultDataBootstrapVersion)
			logx.Must(model.EnsureDefaultChannels(ctx, channelGroupModel, channelTypeModel, channelModel, projectChannelModel, stepChannelParamModel))
			logx.Must(model.EnsureDefaultJenkinsAgents(ctx, channelModel, jenkinsAgentModel))
			logx.Must(model.EnsureDefaultPipelineData(ctx, stepCategoryModel, pipelineEnvModel, stepTemplateModel))
			logx.Must(model.EnsureDefaultConfigCenterData(ctx, configTypeModel, projectMavenModel, projectConfigModel))
			logx.Must(bootstrapStateModel.MarkVersionApplied(ctx, model.DefaultDataBootstrapCode, model.DefaultDataBootstrapVersion, model.DefaultDataBootstrapDescription, "system"))
		}
	} else {
		logx.Infof("DevOps 默认数据启动初始化已关闭")
	}

	// 加载步骤参数到渠道分组的映射缓存，并注册到 channelvars 全局单例。
	channelParamLookup := newStepChannelParamLookup(stepChannelParamModel)
	logx.Must(channelParamLookup.Load(ctx))

	channelChecker := channelcheck.NewChecker(channelModel, credentialModel, hostModel)
	channelChecker.StartDaily()
	var uploader storage.Uploader
	if c.StorageConf.Provider != "" {
		var err error
		uploader, err = storage.NewUploader(c.StorageConf)
		logx.Must(err)
	}

	// 初始化渠道变量相关模型和适配器
	channelVariableSpecModel := model.NewChannelVariableSpecModel(c.Mongo.Url, c.Mongo.Db)
	channelManagerAdapter := adapter.NewChannelManagerAdapter(channelModel, channelTypeModel)
	credentialManagerAdapter := adapter.NewCredentialManagerAdapter(credentialModel)

	// 注册所有 Provider
	registerProviders(channelManagerAdapter)

	svcCtx := &ServiceContext{
		Config:                   c,
		PortalRpc:                portalRpc,
		Cache:                    redis.MustNewRedis(c.Cache),
		ProjectModel:             projectModel,
		ProjectMavenModel:        projectMavenModel,
		ConfigTypeModel:          configTypeModel,
		ProjectConfigModel:       projectConfigModel,
		ChannelGroupModel:        channelGroupModel,
		ChannelTypeModel:         channelTypeModel,
		ChannelModel:             channelModel,
		CredentialModel:          credentialModel,
		CredentialSyncModel:      credentialSyncModel,
		HostModel:                hostModel,
		PipelineUsageModel:       pipelineUsageModel,
		ProjectChannelModel:      projectChannelModel,
		ProjectMemberModel:       projectMemberModel,
		SystemModel:              systemModel,
		PipelineEnvModel:         pipelineEnvModel,
		StepCategoryModel:        stepCategoryModel,
		StepTemplateModel:        stepTemplateModel,
		StepSyncModel:            stepSyncModel,
		PipelineTemplateModel:    pipelineTemplateModel,
		JenkinsAgentModel:        jenkinsAgentModel,
		ChannelChecker:           channelChecker,
		StepChannelParamModel:    stepChannelParamModel,
		ChannelParamLookup:       channelParamLookup,
		Storage:                  uploader,
		ChannelVariableSpecModel: channelVariableSpecModel,
		ChannelManagerAdapter:    channelManagerAdapter,
		CredentialManagerAdapter: credentialManagerAdapter,
	}
	svcCtx.ensureIndexesAsync()
	return svcCtx
}

// registerProviders 注册所有 Provider
func registerProviders(channelManager channelvars.ChannelManager) {
	registry := channelvars.GetProviderRegistry()

	// Git 代码仓库 (gitlab, github, gitee)
	registry.Register("git", providers.NewGitProvider(channelManager))

	// SVN 代码仓库
	registry.Register("svn", providers.NewSVNProvider(channelManager))

	// 镜像仓库
	registry.Register("harbor", providers.NewHarborProvider(channelManager))
	registry.Register("registry", providers.NewRegistryProvider(channelManager))

	// 制品仓库
	registry.Register("nexus", providers.NewNexusProvider(channelManager))
	registry.Register("jfrog", providers.NewJFrogProvider(channelManager))

	// 代码扫描
	registry.Register("sonarqube", providers.NewSonarQubeProvider(channelManager))
	registry.Register("spotbugs", providers.NewSpotBugsProvider(channelManager))

	// 镜像安全
	registry.Register("trivy", providers.NewTrivyProvider(channelManager))
	registry.Register("kube_bench", providers.NewKubeBenchProvider(channelManager))

	// 构建工具
	registry.Register("jenkins", providers.NewJenkinsProvider(channelManager))
	registry.Register("tekton", providers.NewTektonProvider(channelManager))
	registry.Register("buildkit", providers.NewBuildKitProvider(channelManager))

	// 部署目标
	registry.Register("kubernetes", providers.NewKubernetesProvider(channelManager, nil))
	registry.Register("host", providers.NewHostProvider(channelManager))
	registry.Register("host_group", providers.NewHostGroupProvider(channelManager, nil))
	registry.Register("kube-nova", providers.NewKubeNovaProvider(channelManager))
	registry.Register("argocd", providers.NewArgoCDProvider(channelManager))

	logx.Info("All 21 providers registered successfully")
}

func (s *ServiceContext) ensureIndexesAsync() {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := s.StepTemplateModel.EnsureIndexes(ctx); err != nil {
			logx.Errorf("初始化 DevOps 步骤模板索引失败: %v", err)
		}
		if err := s.StepCategoryModel.EnsureIndexes(ctx); err != nil {
			logx.Errorf("初始化 DevOps 步骤分类索引失败: %v", err)
		}
		if err := s.ChannelGroupModel.EnsureIndexes(ctx); err != nil {
			logx.Errorf("初始化 DevOps 渠道分组索引失败: %v", err)
		}
		if err := s.ChannelTypeModel.EnsureIndexes(ctx); err != nil {
			logx.Errorf("初始化 DevOps 渠道类型索引失败: %v", err)
		}
		if err := s.PipelineTemplateModel.EnsureIndexes(ctx); err != nil {
			logx.Errorf("初始化 DevOps 流水线模板索引失败: %v", err)
		}
		if err := s.ProjectMemberModel.EnsureIndexes(ctx); err != nil {
			logx.Errorf("初始化 DevOps 项目成员索引失败: %v", err)
		}
		if err := s.ProjectChannelModel.EnsureIndexes(ctx); err != nil {
			logx.Errorf("初始化 DevOps 项目渠道绑定索引失败: %v", err)
		}
		if err := s.ProjectConfigModel.EnsureIndexes(ctx); err != nil {
			logx.Errorf("初始化 DevOps 项目配置索引失败: %v", err)
		}
		if err := s.ChannelModel.EnsureIndexes(ctx); err != nil {
			logx.Errorf("初始化 DevOps 渠道索引失败: %v", err)
		}
		if err := s.CredentialModel.EnsureIndexes(ctx); err != nil {
			logx.Errorf("初始化 DevOps 凭证索引失败: %v", err)
		}
		if err := s.StepChannelParamModel.EnsureIndexes(ctx); err != nil {
			logx.Errorf("初始化 DevOps 步骤参数渠道映射索引失败: %v", err)
		}
	}()
}
