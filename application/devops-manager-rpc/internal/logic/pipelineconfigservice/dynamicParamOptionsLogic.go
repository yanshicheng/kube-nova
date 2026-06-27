package pipelineconfigservicelogic

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/devops/authutil"
	"github.com/yanshicheng/kube-nova/common/devops/channelvars"
	devopsgitlab "github.com/yanshicheng/kube-nova/common/devops/gitlab"
	devopsharbor "github.com/yanshicheng/kube-nova/common/devops/harbor"
	devopsjfrog "github.com/yanshicheng/kube-nova/common/devops/jfrog"
	devopskubenova "github.com/yanshicheng/kube-nova/common/devops/kubeNova"
	devopskubernetes "github.com/yanshicheng/kube-nova/common/devops/kubernetes"
	devopsnexus "github.com/yanshicheng/kube-nova/common/devops/nexus"
	devopsregistry "github.com/yanshicheng/kube-nova/common/devops/registry"
	devopssonar "github.com/yanshicheng/kube-nova/common/devops/sonar"
	devopstypes "github.com/yanshicheng/kube-nova/common/devops/types"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type DynamicParamOptionsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

const gitRepositoryURLCredentialComponentValue = "includeHttpCredential"

func NewDynamicParamOptionsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DynamicParamOptionsLogic {
	return &DynamicParamOptionsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *DynamicParamOptionsLogic) DynamicParamOptions(in *pb.DynamicParamOptionsReq) (*pb.DynamicParamOptionsResp, error) {
	if err := ensureProjectAccess(l.ctx, l.svcCtx, in.ProjectId, in.CurrentUserId, in.CurrentRoles); err != nil {
		l.Errorf("动态参数选项失败: %v", err)
		return nil, err
	}
	paramType := strings.TrimSpace(in.ParamType)
	groupCode := strings.TrimSpace(in.ConfigTypeCode)
	if groupCode == "" {
		groupCode = channelvars.ChannelGroupCode(paramType)
	}
	mappingField := normalizeChannelVariableMappingField(groupCode, in.ComponentValue, in.MappingField)
	dependencyValues := parseDynamicDependencyValues(in.DependencyValues)
	if paramType == channelvars.ParamChannelVariable {
		paramType = channelVariableDynamicParamType(strings.TrimSpace(in.ComponentValue), mappingField)
		if paramType == "" {
			paramType = channelvars.ParamChannelVariable
		}
	}
	applyDynamicDependencyValues(in, paramType, mappingField, dependencyValues)
	page := in.Page
	if page == 0 {
		page = 1
	}
	pageSize := in.PageSize
	if pageSize == 0 {
		pageSize = 20
	}
	if pageSize > 200 {
		pageSize = 200
	}
	if shouldQueryChannelBindingOptions(paramType) {
		options, total, err := l.channelBindingOptions(in, paramType, page, pageSize)
		if err != nil {
			l.Errorf("动态参数选项失败: %v", err)
			return nil, err
		}
		return &pb.DynamicParamOptionsResp{Data: dynamicOptionsToPb(options), Total: total}, nil
	}
	switch paramType {
	case channelvars.ParamMavenConfig:
		options, total, err := l.mavenConfigOptions(in, page, pageSize)
		if err != nil {
			l.Errorf("动态参数选项失败: %v", err)
			return nil, err
		}
		return &pb.DynamicParamOptionsResp{Data: dynamicOptionsToPb(options), Total: total}, nil
	case channelvars.ParamConfigCenter:
		options, total, err := l.projectConfigOptions(in, page, pageSize)
		if err != nil {
			l.Errorf("动态参数选项失败: %v", err)
			return nil, err
		}
		return &pb.DynamicParamOptionsResp{Data: dynamicOptionsToPb(options), Total: total}, nil
	case channelvars.ParamSonarAddress:
		options, total, err := l.sonarAddressOptions(in, page, pageSize)
		if err != nil {
			l.Errorf("动态参数选项失败: %v", err)
			return nil, err
		}
		return &pb.DynamicParamOptionsResp{Data: dynamicOptionsToPb(options), Total: total}, nil
	case channelvars.ParamSonarProjectName, channelvars.ParamSonarProjectKey:
		options, total, err := l.sonarProjectKeyOptions(in, page, pageSize)
		if err != nil {
			l.Errorf("动态参数选项失败: %v", err)
			return nil, dynamicParamError(paramType, err)
		}
		return &pb.DynamicParamOptionsResp{Data: dynamicOptionsToPb(options), Total: total}, nil
	case channelvars.ParamHostGroupTargets:
		options, total, err := l.hostGroupTargetOptions(in, page, pageSize)
		if err != nil {
			l.Errorf("动态参数选项失败: %v", err)
			return nil, err
		}
		return &pb.DynamicParamOptionsResp{Data: dynamicOptionsToPb(options), Total: total}, nil
	case channelvars.ParamHostConfig:
		options, total, err := l.hostConfigOptions(in, page, pageSize)
		if err != nil {
			l.Errorf("动态参数选项失败: %v", err)
			return nil, err
		}
		return &pb.DynamicParamOptionsResp{Data: dynamicOptionsToPb(options), Total: total}, nil
	case channelvars.ParamHostGroupConfig:
		options, total, err := l.hostGroupConfigOptions(in, page, pageSize)
		if err != nil {
			l.Errorf("动态参数选项失败: %v", err)
			return nil, err
		}
		return &pb.DynamicParamOptionsResp{Data: dynamicOptionsToPb(options), Total: total}, nil
	}
	if paramType == "voucherModel" {
		options, total, err := l.credentialOptions(in, page, pageSize)
		if err != nil {
			l.Errorf("动态参数选项失败: %v", err)
			return nil, err
		}
		return &pb.DynamicParamOptionsResp{Data: dynamicOptionsToPb(options), Total: total}, nil
	}
	if paramType == channelvars.ParamKubernetesWorkloadType {
		options := kubernetesWorkloadTypeOptions()
		return &pb.DynamicParamOptionsResp{Data: dynamicOptionsToPb(options), Total: uint64(len(options))}, nil
	}
	req, binding, channel, err := l.dynamicProviderRequest(in)
	if err != nil {
		if canResolveGitRefByRepositoryURL(paramType, in.ProjectValue) {
			req, binding, err = l.gitRepositoryURLProviderRequest(in)
			channel = nil
			if err != nil {
				l.Errorf("动态参数选项失败: %v", err)
				return nil, err
			}
		} else if canResolveImageRegistryByAddress(paramType, in.ProjectValue, dependencyValues) {
			req, binding, err = l.imageRegistryAddressProviderRequest(in, dependencyValues)
			channel = nil
			if err != nil {
				l.Errorf("动态参数选项失败: %v", err)
				return nil, err
			}
		} else if canResolveArtifactRepositoryByAddress(paramType, in.ProjectValue, dependencyValues) {
			req, binding, err = l.artifactRepositoryAddressProviderRequest(in, dependencyValues)
			channel = nil
			if err != nil {
				l.Errorf("动态参数选项失败: %v", err)
				return nil, err
			}
		} else if shouldReturnEmptyDynamicOptions(paramType, in, dependencyValues) {
			return &pb.DynamicParamOptionsResp{Data: nil, Total: 0}, nil
		} else {
			l.Errorf("动态参数选项失败: %v", err)
			return nil, err
		}
	}
	var options []devopstypes.DynamicOption
	var total uint64
	switch paramType {
	case channelvars.ParamGitProject, channelvars.ParamGitRepositoryURL:
		if !isRepositoryChannelType(binding.ChannelType) {
			l.Errorf("Git 项目参数必须绑定代码仓库渠道")
			return nil, errorx.Msg("Git 项目参数必须绑定代码仓库渠道")
		}
		provider, ok := repositoryProvider(binding.ChannelType).(interface {
			ListProjects(context.Context, devopstypes.Request, string, uint64, uint64) ([]devopstypes.DynamicOption, uint64, error)
		})
		if !ok {
			l.Errorf("代码仓库项目查询不可用")
			return nil, errorx.Msg("代码仓库项目查询不可用")
		}
		options, total, err = provider.ListProjects(l.ctx, req, in.Keyword, page, pageSize)
		if err == nil && paramType == channelvars.ParamGitRepositoryURL && wantsGitRepositoryURLCredential(in.ComponentValue) {
			options, err = withGitRepositoryURLCredential(options, req)
		}
	case channelvars.ParamGitBranch:
		if !isRepositoryChannelType(binding.ChannelType) {
			l.Errorf("Git 分支参数必须绑定代码仓库渠道")
			return nil, errorx.Msg("Git 分支参数必须绑定代码仓库渠道")
		}
		provider, ok := repositoryProvider(binding.ChannelType).(interface {
			ListBranches(context.Context, devopstypes.Request, string, string, uint64, uint64) ([]devopstypes.DynamicOption, uint64, error)
		})
		if !ok {
			l.Errorf("代码仓库分支查询不可用")
			return nil, errorx.Msg("代码仓库分支查询不可用")
		}
		options, total, err = provider.ListBranches(l.ctx, req, normalizeGitProjectValue(in.ProjectValue), in.Keyword, page, pageSize)
	case channelvars.ParamGitTag:
		if !isRepositoryChannelType(binding.ChannelType) {
			l.Errorf("Git Tag 参数必须绑定代码仓库渠道")
			return nil, errorx.Msg("Git Tag 参数必须绑定代码仓库渠道")
		}
		provider, ok := repositoryProvider(binding.ChannelType).(interface {
			ListTags(context.Context, devopstypes.Request, string, string, uint64, uint64) ([]devopstypes.DynamicOption, uint64, error)
		})
		if !ok {
			l.Errorf("代码仓库 Tag 查询不可用")
			return nil, errorx.Msg("代码仓库 Tag 查询不可用")
		}
		options, total, err = provider.ListTags(l.ctx, req, normalizeGitProjectValue(in.ProjectValue), in.Keyword, page, pageSize)
	case channelvars.ParamNexusRepository, channelvars.ParamNexusRepositoryURL:
		if binding.ChannelType != "nexus" {
			l.Errorf("Nexus 参数必须绑定 Nexus 渠道")
			return nil, errorx.Msg("Nexus 参数必须绑定 Nexus 渠道")
		}
		provider, ok := devopsnexus.New().(interface {
			ListRepositories(context.Context, devopstypes.Request, string, uint64, uint64) ([]devopstypes.DynamicOption, uint64, error)
		})
		if !ok {
			l.Errorf("Nexus 仓库查询不可用")
			return nil, errorx.Msg("Nexus 仓库查询不可用")
		}
		options, total, err = provider.ListRepositories(l.ctx, req, in.Keyword, page, pageSize)
	case channelvars.ParamNexusArtifactName, channelvars.ParamNexusArtifact:
		if binding.ChannelType != "nexus" {
			l.Errorf("Nexus 制品参数必须绑定 Nexus 渠道")
			return nil, errorx.Msg("Nexus 制品参数必须绑定 Nexus 渠道")
		}
		if strings.TrimSpace(in.ProjectValue) == "" {
			return &pb.DynamicParamOptionsResp{Data: nil, Total: 0}, nil
		}
		provider, ok := devopsnexus.New().(interface {
			ListComponents(context.Context, devopstypes.Request, string, string, uint64) ([]devopstypes.DynamicOption, uint64, error)
		})
		if !ok {
			l.Errorf("Nexus 制品查询不可用")
			return nil, errorx.Msg("Nexus 制品查询不可用")
		}
		options, total, err = provider.ListComponents(l.ctx, req, in.ProjectValue, in.Keyword, pageSize)
	case channelvars.ParamNexusArtifactVersion:
		if binding.ChannelType != "nexus" {
			l.Errorf("Nexus 制品版本参数必须绑定 Nexus 渠道")
			return nil, errorx.Msg("Nexus 制品版本参数必须绑定 Nexus 渠道")
		}
		if strings.TrimSpace(in.ProjectValue) == "" {
			return &pb.DynamicParamOptionsResp{Data: nil, Total: 0}, nil
		}
		if strings.TrimSpace(in.ComponentValue) == "" {
			return &pb.DynamicParamOptionsResp{Data: nil, Total: 0}, nil
		}
		provider, ok := devopsnexus.New().(interface {
			ListVersions(context.Context, devopstypes.Request, string, string, string, uint64) ([]devopstypes.DynamicOption, uint64, error)
		})
		if !ok {
			l.Errorf("Nexus 制品版本查询不可用")
			return nil, errorx.Msg("Nexus 制品版本查询不可用")
		}
		options, total, err = provider.ListVersions(l.ctx, req, in.ProjectValue, in.ComponentValue, in.Keyword, pageSize)
	case channelvars.ParamHarborProject, channelvars.ParamHarborProjectURL:
		if binding.ChannelType != "harbor" {
			l.Errorf("Harbor 项目参数必须绑定 Harbor 渠道")
			return nil, errorx.Msg("Harbor 项目参数必须绑定 Harbor 渠道")
		}
		provider, ok := devopsharbor.New().(interface {
			ListProjects(context.Context, devopstypes.Request, string, uint64, uint64) ([]devopstypes.DynamicOption, uint64, error)
		})
		if !ok {
			l.Errorf("Harbor 项目查询不可用")
			return nil, errorx.Msg("Harbor 项目查询不可用")
		}
		options, total, err = provider.ListProjects(l.ctx, req, in.Keyword, page, pageSize)
	case channelvars.ParamHarborImage, channelvars.ParamHarborImageURL, channelvars.ParamHarborImageTagURL:
		if binding.ChannelType != "harbor" {
			l.Errorf("Harbor 镜像参数必须绑定 Harbor 渠道")
			return nil, errorx.Msg("Harbor 镜像参数必须绑定 Harbor 渠道")
		}
		if strings.TrimSpace(in.ProjectValue) == "" {
			return &pb.DynamicParamOptionsResp{Data: nil, Total: 0}, nil
		}
		provider, ok := devopsharbor.New().(interface {
			ListRepositories(context.Context, devopstypes.Request, string, string, uint64, uint64) ([]devopstypes.DynamicOption, uint64, error)
		})
		if !ok {
			l.Errorf("Harbor 镜像查询不可用")
			return nil, errorx.Msg("Harbor 镜像查询不可用")
		}
		options, total, err = provider.ListRepositories(l.ctx, req, in.ProjectValue, in.Keyword, page, pageSize)
	case channelvars.ParamHarborImageTag:
		if binding.ChannelType != "harbor" {
			l.Errorf("Harbor 镜像 Tag 参数必须绑定 Harbor 渠道")
			return nil, errorx.Msg("Harbor 镜像 Tag 参数必须绑定 Harbor 渠道")
		}
		if strings.TrimSpace(in.ProjectValue) == "" {
			return &pb.DynamicParamOptionsResp{Data: nil, Total: 0}, nil
		}
		provider, ok := devopsharbor.New().(interface {
			ListTags(context.Context, devopstypes.Request, string, string, string, uint64, uint64) ([]devopstypes.DynamicOption, uint64, error)
		})
		if !ok {
			l.Errorf("Harbor 镜像 Tag 查询不可用")
			return nil, errorx.Msg("Harbor 镜像 Tag 查询不可用")
		}
		if strings.TrimSpace(in.ComponentValue) == "" {
			if strings.TrimSpace(mappingField) != channelvars.FieldDynamicImageTag {
				return &pb.DynamicParamOptionsResp{Data: nil, Total: 0}, nil
			}
			options, total, err = l.harborImageTagPairOptions(req, in.ProjectValue, in.Keyword, page, pageSize)
			break
		}
		options, total, err = provider.ListTags(l.ctx, req, in.ProjectValue, in.ComponentValue, in.Keyword, page, pageSize)
	case channelvars.ParamRegistryRepository:
		if binding.ChannelType != "registry" && binding.ChannelType != "aliyun_registry" {
			l.Errorf("镜像仓库参数必须绑定镜像仓库渠道")
			return nil, errorx.Msg("镜像仓库参数必须绑定镜像仓库渠道")
		}
		provider, ok := devopsregistry.New().(interface {
			ListRepositories(context.Context, devopstypes.Request, string, uint64, uint64) ([]devopstypes.DynamicOption, uint64, error)
		})
		if !ok {
			l.Errorf("镜像仓库查询不可用")
			return nil, errorx.Msg("镜像仓库查询不可用")
		}
		options, total, err = provider.ListRepositories(l.ctx, req, in.Keyword, page, pageSize)
		options, total = registryProjectOptions(options)
	case channelvars.ParamRegistryImage, channelvars.ParamRegistryImageURL, channelvars.ParamRegistryImageTagURL:
		if binding.ChannelType != "registry" && binding.ChannelType != "aliyun_registry" {
			l.Errorf("镜像参数必须绑定镜像仓库渠道")
			return nil, errorx.Msg("镜像参数必须绑定镜像仓库渠道")
		}
		if strings.TrimSpace(in.ProjectValue) == "" {
			return &pb.DynamicParamOptionsResp{Data: nil, Total: 0}, nil
		}
		options, total, err = l.registryImageOptions(req, in.ProjectValue, in.Keyword, page, pageSize)
	case channelvars.ParamRegistryImageTag:
		if binding.ChannelType != "registry" && binding.ChannelType != "aliyun_registry" {
			l.Errorf("镜像 Tag 参数必须绑定镜像仓库渠道")
			return nil, errorx.Msg("镜像 Tag 参数必须绑定镜像仓库渠道")
		}
		if strings.TrimSpace(in.ProjectValue) == "" {
			return &pb.DynamicParamOptionsResp{Data: nil, Total: 0}, nil
		}
		provider, ok := devopsregistry.New().(interface {
			ListTags(context.Context, devopstypes.Request, string, string, string, uint64, uint64) ([]devopstypes.DynamicOption, uint64, error)
		})
		if !ok {
			l.Errorf("镜像 Tag 查询不可用")
			return nil, errorx.Msg("镜像 Tag 查询不可用")
		}
		if strings.TrimSpace(in.ComponentValue) == "" {
			if strings.TrimSpace(mappingField) != channelvars.FieldDynamicImageTag {
				return &pb.DynamicParamOptionsResp{Data: nil, Total: 0}, nil
			}
			options, total, err = l.registryImageTagPairOptions(req, in.ProjectValue, in.Keyword, page, pageSize)
			break
		}
		options, total, err = provider.ListTags(l.ctx, req, in.ProjectValue, registryImageRepository(in.ProjectValue, in.ComponentValue), in.Keyword, page, pageSize)
	case channelvars.ParamJfrogRepository, channelvars.ParamJfrogRepositoryURL:
		if binding.ChannelType != "jfrog" {
			l.Errorf("JFrog 仓库参数必须绑定 JFrog 渠道")
			return nil, errorx.Msg("JFrog 仓库参数必须绑定 JFrog 渠道")
		}
		provider, ok := devopsjfrog.New().(interface {
			ListRepositories(context.Context, devopstypes.Request, string, uint64, uint64) ([]devopstypes.DynamicOption, uint64, error)
		})
		if !ok {
			l.Errorf("JFrog 仓库查询不可用")
			return nil, errorx.Msg("JFrog 仓库查询不可用")
		}
		options, total, err = provider.ListRepositories(l.ctx, req, in.Keyword, page, pageSize)
	case channelvars.ParamJfrogArtifactName, channelvars.ParamJfrogArtifact:
		if binding.ChannelType != "jfrog" {
			l.Errorf("JFrog 制品参数必须绑定 JFrog 渠道")
			return nil, errorx.Msg("JFrog 制品参数必须绑定 JFrog 渠道")
		}
		if strings.TrimSpace(in.ProjectValue) == "" {
			return &pb.DynamicParamOptionsResp{Data: nil, Total: 0}, nil
		}
		provider, ok := devopsjfrog.New().(interface {
			ListArtifacts(context.Context, devopstypes.Request, string, string, uint64, uint64) ([]devopstypes.DynamicOption, uint64, error)
		})
		if !ok {
			l.Errorf("JFrog 制品查询不可用")
			return nil, errorx.Msg("JFrog 制品查询不可用")
		}
		options, total, err = provider.ListArtifacts(l.ctx, req, in.ProjectValue, in.Keyword, page, pageSize)
	case channelvars.ParamJfrogArtifactVersion:
		if binding.ChannelType != "jfrog" {
			l.Errorf("JFrog 制品版本参数必须绑定 JFrog 渠道")
			return nil, errorx.Msg("JFrog 制品版本参数必须绑定 JFrog 渠道")
		}
		if strings.TrimSpace(in.ProjectValue) == "" {
			return &pb.DynamicParamOptionsResp{Data: nil, Total: 0}, nil
		}
		if strings.TrimSpace(in.ComponentValue) == "" {
			return &pb.DynamicParamOptionsResp{Data: nil, Total: 0}, nil
		}
		provider, ok := devopsjfrog.New().(interface {
			ListVersions(context.Context, devopstypes.Request, string, string, string, uint64, uint64) ([]devopstypes.DynamicOption, uint64, error)
		})
		if !ok {
			l.Errorf("JFrog 制品版本查询不可用")
			return nil, errorx.Msg("JFrog 制品版本查询不可用")
		}
		options, total, err = provider.ListVersions(l.ctx, req, in.ProjectValue, in.ComponentValue, in.Keyword, page, pageSize)
	case channelvars.ParamKubeNovaProject:
		if !isKubeNovaBinding(binding, req) {
			l.Errorf("kube-nova 项目参数必须绑定 kube-nova 渠道")
			return nil, errorx.Msg("kube-nova 项目参数必须绑定 kube-nova 渠道")
		}
		if err := l.ensureKubeNovaCredentialReady(binding, channel); err != nil {
			return nil, err
		}
		provider := devopskubenova.New()
		options, total, err = provider.ListProjects(l.ctx, req, in.Keyword, page, pageSize)
	case channelvars.ParamKubeNovaCluster:
		if !isKubeNovaBinding(binding, req) {
			l.Errorf("kube-nova 集群参数必须绑定 kube-nova 渠道")
			return nil, errorx.Msg("kube-nova 集群参数必须绑定 kube-nova 渠道")
		}
		if strings.TrimSpace(in.ProjectValue) == "" {
			return &pb.DynamicParamOptionsResp{Data: nil, Total: 0}, nil
		}
		if err := l.ensureKubeNovaCredentialReady(binding, channel); err != nil {
			return nil, err
		}
		provider := devopskubenova.New()
		options, total, err = provider.ListClusters(l.ctx, req, in.ProjectValue, in.Keyword, page, pageSize)
	case channelvars.ParamKubeNovaWorkspace:
		if !isKubeNovaBinding(binding, req) {
			l.Errorf("kube-nova 工作空间参数必须绑定 kube-nova 渠道")
			return nil, errorx.Msg("kube-nova 工作空间参数必须绑定 kube-nova 渠道")
		}
		if strings.TrimSpace(in.ProjectValue) == "" {
			return &pb.DynamicParamOptionsResp{Data: nil, Total: 0}, nil
		}
		if err := l.ensureKubeNovaCredentialReady(binding, channel); err != nil {
			return nil, err
		}
		provider := devopskubenova.New()
		options, total, err = provider.ListWorkspaces(l.ctx, req, in.ProjectValue, in.Keyword, page, pageSize)
	case channelvars.ParamKubeNovaApplication:
		if !isKubeNovaBinding(binding, req) {
			l.Errorf("kube-nova 应用参数必须绑定 kube-nova 渠道")
			return nil, errorx.Msg("kube-nova 应用参数必须绑定 kube-nova 渠道")
		}
		if strings.TrimSpace(in.ProjectValue) == "" {
			return &pb.DynamicParamOptionsResp{Data: nil, Total: 0}, nil
		}
		if err := l.ensureKubeNovaCredentialReady(binding, channel); err != nil {
			return nil, err
		}
		provider := devopskubenova.New()
		options, total, err = provider.ListApplications(l.ctx, req, in.ProjectValue, in.Keyword, page, pageSize)
	case channelvars.ParamKubeNovaVersion:
		if !isKubeNovaBinding(binding, req) {
			l.Errorf("kube-nova 版本参数必须绑定 kube-nova 渠道")
			return nil, errorx.Msg("kube-nova 版本参数必须绑定 kube-nova 渠道")
		}
		if strings.TrimSpace(in.ProjectValue) == "" {
			return &pb.DynamicParamOptionsResp{Data: nil, Total: 0}, nil
		}
		if err := l.ensureKubeNovaCredentialReady(binding, channel); err != nil {
			return nil, err
		}
		provider := devopskubenova.New()
		options, total, err = provider.ListVersions(l.ctx, req, in.ProjectValue, in.Keyword, page, pageSize)
	case channelvars.ParamKubeNovaContainer:
		if !isKubeNovaBinding(binding, req) {
			l.Errorf("kube-nova 容器参数必须绑定 kube-nova 渠道")
			return nil, errorx.Msg("kube-nova 容器参数必须绑定 kube-nova 渠道")
		}
		if strings.TrimSpace(in.ProjectValue) == "" {
			return &pb.DynamicParamOptionsResp{Data: nil, Total: 0}, nil
		}
		if err := l.ensureKubeNovaCredentialReady(binding, channel); err != nil {
			return nil, err
		}
		provider := devopskubenova.New()
		options, total, err = provider.ListContainers(l.ctx, req, in.ProjectValue, in.Keyword)
	case channelvars.ParamKubernetesNamespace:
		if !isKubernetesBinding(binding, req) {
			l.Errorf("Kubernetes 命名空间参数必须绑定 Kubernetes 渠道")
			return nil, errorx.Msg("Kubernetes 命名空间参数必须绑定 Kubernetes 渠道")
		}
		if err := l.ensureKubernetesDeployCredentialReady(binding, channel); err != nil {
			return nil, err
		}
		provider, ok := devopskubernetes.New().(interface {
			ListNamespaces(context.Context, devopstypes.Request, string, uint64, uint64) ([]devopstypes.DynamicOption, uint64, error)
		})
		if !ok {
			l.Errorf("Kubernetes 命名空间查询不可用")
			return nil, errorx.Msg("Kubernetes 命名空间查询不可用")
		}
		options, total, err = provider.ListNamespaces(l.ctx, req, in.Keyword, page, pageSize)
	case channelvars.ParamKubernetesWorkloadType:
		options = kubernetesWorkloadTypeOptions()
		total = uint64(len(options))
	case channelvars.ParamKubernetesResource:
		if !isKubernetesBinding(binding, req) {
			l.Errorf("Kubernetes 资源参数必须绑定 Kubernetes 渠道")
			return nil, errorx.Msg("Kubernetes 资源参数必须绑定 Kubernetes 渠道")
		}
		if strings.TrimSpace(in.ProjectValue) == "" {
			return &pb.DynamicParamOptionsResp{Data: nil, Total: 0}, nil
		}
		if strings.TrimSpace(in.ComponentValue) == "" {
			return &pb.DynamicParamOptionsResp{Data: nil, Total: 0}, nil
		}
		if err := l.ensureKubernetesDeployCredentialReady(binding, channel); err != nil {
			return nil, err
		}
		provider, ok := devopskubernetes.New().(interface {
			ListResources(context.Context, devopstypes.Request, string, string, string, uint64, uint64) ([]devopstypes.DynamicOption, uint64, error)
		})
		if !ok {
			l.Errorf("Kubernetes 资源查询不可用")
			return nil, errorx.Msg("Kubernetes 资源查询不可用")
		}
		options, total, err = provider.ListResources(l.ctx, req, in.ProjectValue, in.ComponentValue, in.Keyword, page, pageSize)
	case channelvars.ParamKubernetesContainer:
		if !isKubernetesBinding(binding, req) {
			l.Errorf("Kubernetes 容器参数必须绑定 Kubernetes 渠道")
			return nil, errorx.Msg("Kubernetes 容器参数必须绑定 Kubernetes 渠道")
		}
		if strings.TrimSpace(in.ProjectValue) == "" {
			return &pb.DynamicParamOptionsResp{Data: nil, Total: 0}, nil
		}
		if strings.TrimSpace(in.ComponentValue) == "" {
			return &pb.DynamicParamOptionsResp{Data: nil, Total: 0}, nil
		}
		if strings.TrimSpace(in.ConfigTypeCode) == "" {
			return &pb.DynamicParamOptionsResp{Data: nil, Total: 0}, nil
		}
		if err := l.ensureKubernetesDeployCredentialReady(binding, channel); err != nil {
			return nil, err
		}
		provider, ok := devopskubernetes.New().(interface {
			ListContainers(context.Context, devopstypes.Request, string, string, string, string) ([]devopstypes.DynamicOption, uint64, error)
		})
		if !ok {
			l.Errorf("Kubernetes 容器查询不可用")
			return nil, errorx.Msg("Kubernetes 容器查询不可用")
		}
		options, total, err = provider.ListContainers(l.ctx, req, in.ProjectValue, in.ComponentValue, in.ConfigTypeCode, in.Keyword)
	case channelvars.ParamKubernetesImage:
		if !isKubernetesBinding(binding, req) {
			l.Errorf("Kubernetes 镜像参数必须绑定 Kubernetes 渠道")
			return nil, errorx.Msg("Kubernetes 镜像参数必须绑定 Kubernetes 渠道")
		}
		if strings.TrimSpace(in.ProjectValue) == "" {
			return &pb.DynamicParamOptionsResp{Data: nil, Total: 0}, nil
		}
		if strings.TrimSpace(in.ComponentValue) == "" {
			return &pb.DynamicParamOptionsResp{Data: nil, Total: 0}, nil
		}
		if strings.TrimSpace(in.ConfigTypeCode) == "" {
			return &pb.DynamicParamOptionsResp{Data: nil, Total: 0}, nil
		}
		if err := l.ensureKubernetesDeployCredentialReady(binding, channel); err != nil {
			return nil, err
		}
		provider, ok := devopskubernetes.New().(interface {
			ListImages(context.Context, devopstypes.Request, string, string, string, string, string) ([]devopstypes.DynamicOption, uint64, error)
		})
		if !ok {
			l.Errorf("Kubernetes 镜像查询不可用")
			return nil, errorx.Msg("Kubernetes 镜像查询不可用")
		}
		options, total, err = provider.ListImages(l.ctx, req, in.ProjectValue, in.ComponentValue, in.ConfigTypeCode, dependencyValues[channelvars.FieldDynamicContainerName], in.Keyword)
	default:
		l.Errorf("%s", "不支持的动态参数类型："+paramType)
		return nil, errorx.Msg("不支持的动态参数类型：" + paramType)
	}
	if err != nil {
		l.Errorf("动态参数选项失败: %v", err)
		return nil, dynamicParamError(paramType, err)
	}
	options = applyDynamicAddressOptionValues(options, mappingField, req, in)
	options = applyImageRegistryDynamicOptionValues(options, mappingField, in)

	return &pb.DynamicParamOptionsResp{Data: dynamicOptionsToPb(options), Total: total}, nil
}

func kubernetesWorkloadTypeOptions() []devopstypes.DynamicOption {
	return []devopstypes.DynamicOption{
		{Label: "Deployment", Value: "deployment"},
		{Label: "StatefulSet", Value: "statefulset"},
		{Label: "DaemonSet", Value: "daemonset"},
		{Label: "CronJob", Value: "cronjob"},
	}
}

func shouldQueryChannelBindingOptions(paramType string) bool {
	paramType = strings.TrimSpace(paramType)
	return channelvars.IsChannelParamType(paramType) &&
		!channelvars.IsDynamicParamType(paramType) &&
		!channelvars.IsCompositeAddressParamType(paramType)
}

func shouldReturnEmptyDynamicOptions(paramType string, in *pb.DynamicParamOptionsReq, dependencyValues map[string]string) bool {
	if in == nil {
		return false
	}
	if strings.TrimSpace(in.ChannelBindingId) != "" {
		return false
	}
	switch strings.TrimSpace(paramType) {
	case channelvars.ParamGitProject, channelvars.ParamGitBranch, channelvars.ParamGitTag,
		channelvars.ParamNexusRepository, channelvars.ParamNexusArtifactName, channelvars.ParamNexusArtifactVersion, channelvars.ParamNexusArtifact,
		channelvars.ParamHarborProject, channelvars.ParamHarborImage, channelvars.ParamHarborImageTag,
		channelvars.ParamRegistryRepository, channelvars.ParamRegistryImage, channelvars.ParamRegistryImageTag,
		channelvars.ParamJfrogRepository, channelvars.ParamJfrogArtifactName, channelvars.ParamJfrogArtifactVersion, channelvars.ParamJfrogArtifact,
		channelvars.ParamSonarProjectName, channelvars.ParamSonarProjectKey,
		channelvars.ParamKubeNovaProject, channelvars.ParamKubeNovaCluster, channelvars.ParamKubeNovaWorkspace, channelvars.ParamKubeNovaApplication, channelvars.ParamKubeNovaVersion, channelvars.ParamKubeNovaContainer,
		channelvars.ParamKubernetesNamespace, channelvars.ParamKubernetesResource, channelvars.ParamKubernetesContainer, channelvars.ParamKubernetesImage:
		return imageRegistryAddressFromValues(in.ProjectValue, dependencyValues) == "" &&
			artifactRepositoryAddressFromValues(in.ProjectValue, dependencyValues) == ""
	default:
		return false
	}
}

func channelVariableDynamicParamType(channelType, mappingField string) string {
	channelType = strings.TrimSpace(channelType)
	switch strings.TrimSpace(mappingField) {
	case channelvars.FieldDynamicProject:
		if isRepositoryChannelType(channelType) {
			return channelvars.ParamGitProject
		}
		switch channelType {
		case "harbor":
			return channelvars.ParamHarborProject
		case "registry", "aliyun_registry":
			return channelvars.ParamRegistryRepository
		}
		if channelType == "nexus" {
			return channelvars.ParamNexusRepository
		}
		if channelType == "jfrog" {
			return channelvars.ParamJfrogRepository
		}
		if channelType == "kube-nova" {
			return channelvars.ParamKubeNovaProject
		}
	case channelvars.FieldDynamicBranch:
		if isRepositoryChannelType(channelType) {
			return channelvars.ParamGitBranch
		}
	case channelvars.FieldDynamicTag:
		if isRepositoryChannelType(channelType) {
			return channelvars.ParamGitTag
		}
		switch channelType {
		case "harbor":
			return channelvars.ParamHarborImageTag
		case "registry", "aliyun_registry":
			return channelvars.ParamRegistryImageTag
		}
	case channelvars.FieldAddressProjectURL:
		if isRepositoryChannelType(channelType) {
			return channelvars.ParamGitRepositoryURL
		}
		switch channelType {
		case "harbor":
			return channelvars.ParamHarborProjectURL
		case "registry", "aliyun_registry":
			return channelvars.ParamRegistryRepositoryURL
		}
		if channelType == "sonarqube" {
			return channelvars.ParamSonarAddress
		}
	case channelvars.FieldDynamicRegistry:
		switch channelType {
		case "harbor":
			return channelvars.ParamHarborProject
		case "registry", "aliyun_registry":
			return channelvars.ParamRegistryRepository
		}
	case channelvars.FieldDynamicRepository:
		switch channelType {
		case "harbor":
			return channelvars.ParamHarborProject
		case "registry", "aliyun_registry":
			return channelvars.ParamRegistryRepository
		}
		switch channelType {
		case "nexus":
			return channelvars.ParamNexusRepository
		case "jfrog":
			return channelvars.ParamJfrogRepository
		}
	case channelvars.FieldDynamicImage:
		switch channelType {
		case "harbor":
			return channelvars.ParamHarborImage
		case "registry", "aliyun_registry":
			return channelvars.ParamRegistryImage
		}
	case channelvars.FieldDynamicImageTag:
		switch channelType {
		case "harbor":
			return channelvars.ParamHarborImageTag
		case "registry", "aliyun_registry":
			return channelvars.ParamRegistryImageTag
		}
	case channelvars.FieldAddressRegistryURL:
		switch channelType {
		case "harbor":
			return channelvars.ParamHarborProjectURL
		case "registry", "aliyun_registry":
			return channelvars.ParamRegistryRepositoryURL
		}
	case channelvars.FieldAddressImageURL:
		switch channelType {
		case "harbor":
			return channelvars.ParamHarborImageURL
		case "registry", "aliyun_registry":
			return channelvars.ParamRegistryImageURL
		}
	case channelvars.FieldAddressImageTagURL:
		switch channelType {
		case "harbor":
			return channelvars.ParamHarborImageTagURL
		case "registry", "aliyun_registry":
			return channelvars.ParamRegistryImageTagURL
		}
	case channelvars.FieldDynamicArtifactName, channelvars.FieldDynamicArtifact:
		if channelType == "nexus" {
			return channelvars.ParamNexusArtifactName
		}
		if channelType == "jfrog" {
			return channelvars.ParamJfrogArtifactName
		}
	case channelvars.FieldDynamicArtifactVersion, channelvars.FieldDynamicArtifactTag:
		if channelType == "nexus" {
			return channelvars.ParamNexusArtifactVersion
		}
		if channelType == "jfrog" {
			return channelvars.ParamJfrogArtifactVersion
		}
	case channelvars.FieldAddressRepositoryURL:
		if channelType == "nexus" {
			return channelvars.ParamNexusRepositoryURL
		}
		if channelType == "jfrog" {
			return channelvars.ParamJfrogRepositoryURL
		}
	case channelvars.FieldAddressArtifactURL:
		if channelType == "nexus" {
			return channelvars.ParamNexusArtifactURL
		}
		if channelType == "jfrog" {
			return channelvars.ParamJfrogArtifactURL
		}
	case channelvars.FieldAddressArtifactVersionURL, channelvars.FieldAddressArtifactTagURL:
		if channelType == "nexus" {
			return channelvars.ParamNexusArtifactVersionURL
		}
		if channelType == "jfrog" {
			return channelvars.ParamJfrogArtifactVersionURL
		}
	case channelvars.FieldDynamicProjectName:
		if channelType == "sonarqube" {
			return channelvars.ParamSonarProjectName
		}
	case channelvars.FieldDynamicProjectKey:
		if channelType == "sonarqube" {
			return channelvars.ParamSonarProjectKey
		}
	case channelvars.FieldAddressProjectKeyURL:
		if channelType == "sonarqube" {
			return channelvars.ParamSonarProjectKey
		}
	case channelvars.FieldDynamicCluster:
		if channelType == "kube-nova" {
			return channelvars.ParamKubeNovaCluster
		}
	case channelvars.FieldDynamicWorkspace:
		if channelType == "kube-nova" {
			return channelvars.ParamKubeNovaWorkspace
		}
	case channelvars.FieldDynamicApplication:
		if channelType == "kube-nova" {
			return channelvars.ParamKubeNovaApplication
		}
	case channelvars.FieldDynamicVersion:
		if channelType == "kube-nova" {
			return channelvars.ParamKubeNovaVersion
		}
	case channelvars.FieldDynamicNamespace:
		if channelType == "kubernetes" {
			return channelvars.ParamKubernetesNamespace
		}
	case channelvars.FieldDynamicWorkloadType:
		if channelType == "kubernetes" {
			return channelvars.ParamKubernetesWorkloadType
		}
	case channelvars.FieldDynamicResourceName:
		if channelType == "kubernetes" {
			return channelvars.ParamKubernetesResource
		}
	case channelvars.FieldDynamicDeployment, channelvars.FieldDynamicStatefulSet, channelvars.FieldDynamicDaemonSet, channelvars.FieldDynamicCronJob:
		if channelType == "kubernetes" {
			return channelvars.ParamKubernetesResource
		}
	case channelvars.FieldDynamicContainerName, channelvars.FieldDynamicContainer:
		if channelType == "kube-nova" {
			return channelvars.ParamKubeNovaContainer
		}
		if channelType == "kubernetes" {
			return channelvars.ParamKubernetesContainer
		}
	case channelvars.FieldDynamicImageName:
		if channelType == "kubernetes" {
			return channelvars.ParamKubernetesImage
		}
	case channelvars.FieldDynamicDeployConfig:
		if channelType == "kube-nova" {
			return channelvars.ParamKubeNovaDeployConfig
		}
		if channelType == "kubernetes" {
			return channelvars.ParamKubernetesDeployConfig
		}
	case channelvars.FieldAddressDeployConfig:
		if channelType == "kube-nova" {
			return channelvars.ParamKubeNovaDeployConfig
		}
	case channelvars.FieldAddressDeploymentConfig:
		if channelType == "kubernetes" {
			return channelvars.ParamKubernetesDeployConfig
		}
	case channelvars.FieldAddressHostConfig:
		if channelType == "host" {
			return channelvars.ParamHostConfig
		}
	case channelvars.FieldAddressGroupConfig:
		if channelType == "host_group" {
			return channelvars.ParamHostGroupConfig
		}
	}
	return ""
}

func parseDynamicDependencyValues(raw string) map[string]string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	values := map[string]string{}
	var stringValues map[string]string
	if err := json.Unmarshal([]byte(raw), &stringValues); err == nil {
		for key, value := range stringValues {
			if strings.TrimSpace(key) != "" {
				values[strings.TrimSpace(key)] = strings.TrimSpace(value)
			}
		}
		return values
	}
	var anyValues map[string]any
	if err := json.Unmarshal([]byte(raw), &anyValues); err != nil {
		return nil
	}
	for key, value := range anyValues {
		if strings.TrimSpace(key) == "" {
			continue
		}
		switch typed := value.(type) {
		case string:
			values[strings.TrimSpace(key)] = strings.TrimSpace(typed)
		case float64:
			values[strings.TrimSpace(key)] = strings.TrimSpace(strconv.FormatFloat(typed, 'f', -1, 64))
		case bool:
			values[strings.TrimSpace(key)] = strconv.FormatBool(typed)
		}
	}
	return values
}

func applyDynamicDependencyValues(in *pb.DynamicParamOptionsReq, paramType, mappingField string, values map[string]string) {
	if in == nil || len(values) == 0 {
		return
	}
	if strings.TrimSpace(in.ProjectValue) == "" {
		for _, field := range []string{
			channelvars.FieldDynamicProject,
			channelvars.FieldDynamicRepository,
			channelvars.FieldDynamicNamespace,
			channelvars.FieldDynamicCluster,
			channelvars.FieldDynamicWorkspace,
			channelvars.FieldDynamicApplication,
			channelvars.FieldDynamicVersion,
			"project",
			"repository",
			"namespace",
		} {
			if value := strings.TrimSpace(values[field]); value != "" {
				in.ProjectValue = value
				break
			}
		}
	}
	if strings.TrimSpace(in.ComponentValue) == "" {
		for _, field := range []string{
			channelvars.FieldDynamicImage,
			channelvars.FieldDynamicArtifactName,
			channelvars.FieldDynamicArtifact,
			channelvars.FieldDynamicWorkloadType,
			channelvars.FieldDynamicDeployment,
			channelvars.FieldDynamicStatefulSet,
			channelvars.FieldDynamicDaemonSet,
			channelvars.FieldDynamicCronJob,
			"image",
			"artifact",
			"workload",
		} {
			if value := strings.TrimSpace(values[field]); value != "" {
				in.ComponentValue = value
				break
			}
		}
	}
	if strings.TrimSpace(in.ConfigTypeCode) == "" {
		for _, field := range []string{
			channelvars.FieldDynamicDeployment,
			channelvars.FieldDynamicStatefulSet,
			channelvars.FieldDynamicDaemonSet,
			channelvars.FieldDynamicCronJob,
			channelvars.FieldDynamicResourceName,
			channelvars.FieldDynamicImageTag,
			"resource",
			"tag",
		} {
			if value := strings.TrimSpace(values[field]); value != "" {
				in.ConfigTypeCode = value
				break
			}
		}
	}
	switch strings.TrimSpace(mappingField) {
	case channelvars.FieldDynamicDeployment:
		in.ComponentValue = "deployment"
	case channelvars.FieldDynamicStatefulSet:
		in.ComponentValue = "statefulset"
	case channelvars.FieldDynamicDaemonSet:
		in.ComponentValue = "daemonset"
	case channelvars.FieldDynamicCronJob:
		in.ComponentValue = "cronjob"
	}
	switch paramType {
	case channelvars.ParamGitBranch, channelvars.ParamGitTag:
		if !setDynamicProjectValue(in, values, channelvars.FieldDynamicProject) {
			setDynamicProjectValue(in, values, channelvars.FieldAddressProjectURL)
		}
	case channelvars.ParamNexusArtifactName, channelvars.ParamNexusArtifactVersion, channelvars.ParamNexusArtifact,
		channelvars.ParamJfrogArtifactName, channelvars.ParamJfrogArtifactVersion, channelvars.ParamJfrogArtifact:
		if !setDynamicProjectValue(in, values, channelvars.FieldDynamicRepository) {
			setDynamicProjectValue(in, values, channelvars.FieldAddressRepositoryURL)
		}
		if strings.TrimSpace(in.ComponentValue) == "" {
			setDynamicComponentValue(in, values, channelvars.FieldDynamicArtifactName)
		}
	case channelvars.ParamHarborImage, channelvars.ParamHarborImageURL, channelvars.ParamHarborImageTag, channelvars.ParamHarborImageTagURL:
		if !setDynamicProjectValue(in, values, channelvars.FieldDynamicProject) {
			if !setDynamicProjectValue(in, values, channelvars.FieldDynamicRepository) {
				if !setDynamicProjectValue(in, values, channelvars.FieldAddressProjectURL) {
					if !setDynamicProjectValue(in, values, channelvars.FieldDynamicRegistry) {
						setDynamicProjectValue(in, values, channelvars.FieldAddressRegistryURL)
					}
				}
			}
		}
	case channelvars.ParamRegistryImage, channelvars.ParamRegistryImageURL, channelvars.ParamRegistryImageTag, channelvars.ParamRegistryImageTagURL:
		if !setDynamicProjectValue(in, values, channelvars.FieldDynamicProject) {
			if !setDynamicProjectValue(in, values, channelvars.FieldDynamicRepository) {
				if !setDynamicProjectValue(in, values, channelvars.FieldAddressProjectURL) {
					if !setDynamicProjectValue(in, values, channelvars.FieldDynamicRegistry) {
						setDynamicProjectValue(in, values, channelvars.FieldAddressRegistryURL)
					}
				}
			}
		}
	case channelvars.ParamKubeNovaCluster:
		setDynamicProjectValue(in, values, channelvars.FieldDynamicProject)
	case channelvars.ParamKubeNovaWorkspace:
		setDynamicProjectValue(in, values, channelvars.FieldDynamicCluster)
	case channelvars.ParamKubeNovaApplication:
		setDynamicProjectValue(in, values, channelvars.FieldDynamicWorkspace)
	case channelvars.ParamKubeNovaVersion:
		setDynamicProjectValue(in, values, channelvars.FieldDynamicApplication)
	case channelvars.ParamKubeNovaContainer:
		setDynamicProjectValue(in, values, channelvars.FieldDynamicVersion)
	case channelvars.ParamKubernetesResource, channelvars.ParamKubernetesContainer, channelvars.ParamKubernetesImage:
		setDynamicProjectValue(in, values, channelvars.FieldDynamicNamespace)
		if strings.TrimSpace(in.ComponentValue) == "" {
			setDynamicComponentValue(in, values, channelvars.FieldDynamicWorkloadType)
		}
		if strings.TrimSpace(in.ConfigTypeCode) == "" {
			setDynamicConfigTypeValue(in, values, channelvars.FieldDynamicResourceName)
		}
	}
	if paramType == channelvars.ParamKubernetesContainer {
		if resourceType, resourceName := kubernetesContainerDependency(values); resourceType != "" && resourceName != "" {
			in.ComponentValue = resourceType
			in.ConfigTypeCode = resourceName
		}
	}
	if strings.TrimSpace(in.ConfigTypeCode) == "" && strings.TrimSpace(values[channelvars.FieldDynamicImageTag]) != "" {
		in.ConfigTypeCode = strings.TrimSpace(values[channelvars.FieldDynamicImageTag])
	}
}

func setDynamicProjectValue(in *pb.DynamicParamOptionsReq, values map[string]string, field string) bool {
	if value := strings.TrimSpace(values[field]); value != "" {
		in.ProjectValue = value
		return true
	}
	return false
}

func setDynamicComponentValue(in *pb.DynamicParamOptionsReq, values map[string]string, field string) bool {
	if value := strings.TrimSpace(values[field]); value != "" {
		in.ComponentValue = value
		return true
	}
	return false
}

func setDynamicConfigTypeValue(in *pb.DynamicParamOptionsReq, values map[string]string, field string) bool {
	if value := strings.TrimSpace(values[field]); value != "" {
		in.ConfigTypeCode = value
		return true
	}
	return false
}

func kubernetesContainerDependency(values map[string]string) (string, string) {
	for _, item := range []struct {
		field        string
		resourceType string
	}{
		{channelvars.FieldDynamicDeployment, "deployment"},
		{channelvars.FieldDynamicStatefulSet, "statefulset"},
		{channelvars.FieldDynamicDaemonSet, "daemonset"},
		{channelvars.FieldDynamicCronJob, "cronjob"},
	} {
		if value := strings.TrimSpace(values[item.field]); value != "" {
			return item.resourceType, value
		}
	}
	return "", ""
}

func applyDynamicAddressOptionValues(options []devopstypes.DynamicOption, mappingField string, req devopstypes.Request, in *pb.DynamicParamOptionsReq) []devopstypes.DynamicOption {
	switch strings.TrimSpace(mappingField) {
	case channelvars.FieldAddressProjectURL:
		if req.Channel.Type == "harbor" || req.Channel.Type == "registry" || req.Channel.Type == "aliyun_registry" {
			return mapDynamicAddressOptions(options, func(item devopstypes.DynamicOption) string {
				return joinAddress(req.Channel.Endpoint, item.Value)
			})
		}
		return mapDynamicAddressOptions(options, func(item devopstypes.DynamicOption) string {
			for _, key := range []string{"httpUrlToRepo", "webUrl", "url"} {
				if value := metadataString(item.Metadata, key); value != "" {
					return value
				}
			}
			if req.Channel.Type == "sonarqube" {
				return joinAddress(req.Channel.Endpoint, "dashboard?id="+url.QueryEscape(item.Value))
			}
			return joinAddress(req.Channel.Endpoint, item.Value)
		})
	case channelvars.FieldAddressProjectKeyURL:
		return mapDynamicAddressOptions(options, func(item devopstypes.DynamicOption) string {
			return joinAddress(req.Channel.Endpoint, "dashboard?id="+url.QueryEscape(item.Value))
		})
	case channelvars.FieldAddressRegistryURL, channelvars.FieldAddressRepositoryURL:
		return mapDynamicAddressOptions(options, func(item devopstypes.DynamicOption) string {
			return joinAddress(req.Channel.Endpoint, item.Value)
		})
	case channelvars.FieldAddressImageURL:
		return mapDynamicAddressOptions(options, func(item devopstypes.DynamicOption) string {
			return joinAddress(req.Channel.Endpoint, in.ProjectValue, item.Value)
		})
	case channelvars.FieldAddressImageTagURL:
		return mapDynamicAddressOptions(options, func(item devopstypes.DynamicOption) string {
			image := strings.Trim(strings.TrimSpace(in.ComponentValue), "/")
			if image == "" {
				image = strings.Trim(strings.TrimSpace(item.Value), "/")
				return joinAddress(req.Channel.Endpoint, in.ProjectValue, image)
			}
			return joinAddress(req.Channel.Endpoint, in.ProjectValue, image) + ":" + strings.TrimSpace(item.Value)
		})
	case channelvars.FieldAddressArtifactURL:
		return mapDynamicAddressOptions(options, func(item devopstypes.DynamicOption) string {
			if value := metadataString(item.Metadata, "url"); value != "" {
				return value
			}
			return joinAddress(req.Channel.Endpoint, in.ProjectValue, item.Value)
		})
	case channelvars.FieldAddressArtifactVersionURL, channelvars.FieldAddressArtifactTagURL:
		return mapDynamicAddressOptions(options, func(item devopstypes.DynamicOption) string {
			if value := metadataString(item.Metadata, "url"); value != "" {
				return value
			}
			artifact := strings.Trim(strings.TrimSpace(in.ComponentValue), "/")
			if artifact == "" {
				artifact = strings.Trim(strings.TrimSpace(item.Value), "/")
				return joinAddress(req.Channel.Endpoint, in.ProjectValue, artifact)
			}
			return joinAddress(req.Channel.Endpoint, in.ProjectValue, artifact) + ":" + strings.TrimSpace(item.Value)
		})
	default:
		return options
	}
}

func applyImageRegistryDynamicOptionValues(options []devopstypes.DynamicOption, mappingField string, in *pb.DynamicParamOptionsReq) []devopstypes.DynamicOption {
	if strings.TrimSpace(mappingField) != channelvars.FieldDynamicImageTag {
		return options
	}
	image := strings.Trim(strings.TrimSpace(in.ComponentValue), "/")
	if image == "" {
		return options
	}
	result := make([]devopstypes.DynamicOption, 0, len(options))
	for _, item := range options {
		tag := strings.TrimSpace(item.Value)
		if tag == "" || strings.Contains(tag, ":") {
			result = append(result, item)
			continue
		}
		next := item
		next.Value = image + ":" + tag
		if strings.TrimSpace(next.Label) == tag {
			next.Label = next.Value
		}
		if next.Metadata == nil {
			next.Metadata = map[string]any{}
		}
		next.Metadata["image"] = image
		next.Metadata["tag"] = tag
		result = append(result, next)
	}
	return result
}

func (l *DynamicParamOptionsLogic) harborImageTagPairOptions(req devopstypes.Request, project, keyword string, page, pageSize uint64) ([]devopstypes.DynamicOption, uint64, error) {
	provider := devopsharbor.New()
	repoProvider, ok := provider.(interface {
		ListRepositories(context.Context, devopstypes.Request, string, string, uint64, uint64) ([]devopstypes.DynamicOption, uint64, error)
	})
	if !ok {
		return nil, 0, errors.New("Harbor 镜像查询不可用")
	}
	tagProvider, ok := provider.(interface {
		ListTags(context.Context, devopstypes.Request, string, string, string, uint64, uint64) ([]devopstypes.DynamicOption, uint64, error)
	})
	if !ok {
		return nil, 0, errors.New("Harbor 镜像 Tag 查询不可用")
	}
	images, _, err := repoProvider.ListRepositories(l.ctx, req, project, "", page, pageSize)
	if err != nil {
		return nil, 0, err
	}
	result := make([]devopstypes.DynamicOption, 0)
	for _, image := range images {
		imageName := strings.TrimSpace(image.Value)
		if imageName == "" {
			continue
		}
		remaining := remainingOptionLimit(result, pageSize)
		if remaining == 0 {
			break
		}
		tags, _, err := tagProvider.ListTags(l.ctx, req, project, imageName, keyword, 1, pageSize)
		if err != nil {
			return nil, 0, err
		}
		result = append(result, imageTagPairOptions(imageName, tags, remaining)...)
	}
	return result, uint64(len(result)), nil
}

func (l *DynamicParamOptionsLogic) registryImageOptions(req devopstypes.Request, project, keyword string, page, pageSize uint64) ([]devopstypes.DynamicOption, uint64, error) {
	provider := devopsregistry.New()
	repoProvider, ok := provider.(interface {
		ListRepositories(context.Context, devopstypes.Request, string, uint64, uint64) ([]devopstypes.DynamicOption, uint64, error)
	})
	if !ok {
		return nil, 0, errors.New("镜像查询不可用")
	}
	repositories, _, err := repoProvider.ListRepositories(l.ctx, req, keyword, page, pageSize)
	if err != nil {
		return nil, 0, err
	}
	options := registryImageOptionsFromRepositories(repositories, project, keyword)
	if len(options) > 0 {
		return options, uint64(len(options)), nil
	}
	imageProvider, ok := provider.(interface {
		ListImages(context.Context, devopstypes.Request, string, string, uint64, uint64) ([]devopstypes.DynamicOption, uint64, error)
	})
	if !ok {
		return nil, 0, errors.New("镜像查询不可用")
	}
	return imageProvider.ListImages(l.ctx, req, project, keyword, page, pageSize)
}

func (l *DynamicParamOptionsLogic) registryImageTagPairOptions(req devopstypes.Request, project, keyword string, page, pageSize uint64) ([]devopstypes.DynamicOption, uint64, error) {
	images, _, err := l.registryImageOptions(req, project, "", page, pageSize)
	if err != nil {
		return nil, 0, err
	}
	provider, ok := devopsregistry.New().(interface {
		ListTags(context.Context, devopstypes.Request, string, string, string, uint64, uint64) ([]devopstypes.DynamicOption, uint64, error)
	})
	if !ok {
		return nil, 0, errors.New("镜像 Tag 查询不可用")
	}
	result := make([]devopstypes.DynamicOption, 0)
	for _, image := range images {
		imageName := strings.TrimSpace(image.Value)
		if imageName == "" {
			continue
		}
		remaining := remainingOptionLimit(result, pageSize)
		if remaining == 0 {
			break
		}
		tags, _, err := provider.ListTags(l.ctx, req, project, registryImageRepository(project, imageName), keyword, 1, pageSize)
		if err != nil {
			return nil, 0, err
		}
		result = append(result, imageTagPairOptions(imageName, tags, remaining)...)
	}
	return result, uint64(len(result)), nil
}

func registryProjectOptions(repositories []devopstypes.DynamicOption) ([]devopstypes.DynamicOption, uint64) {
	result := make([]devopstypes.DynamicOption, 0, len(repositories))
	seen := map[string]struct{}{}
	for _, item := range repositories {
		project := strings.Trim(strings.TrimSpace(item.Value), "/")
		if idx := strings.Index(project, "/"); idx > 0 {
			project = project[:idx]
		}
		if project == "" {
			continue
		}
		if _, ok := seen[project]; ok {
			continue
		}
		seen[project] = struct{}{}
		result = append(result, devopstypes.DynamicOption{
			Label: project,
			Value: project,
		})
	}
	return result, uint64(len(result))
}

func registryImageOptionsFromRepositories(repositories []devopstypes.DynamicOption, project, keyword string) []devopstypes.DynamicOption {
	project = strings.Trim(strings.TrimSpace(project), "/")
	keyword = strings.ToLower(strings.TrimSpace(keyword))
	prefix := project + "/"
	result := make([]devopstypes.DynamicOption, 0, len(repositories))
	seen := map[string]struct{}{}
	for _, item := range repositories {
		repository := strings.Trim(strings.TrimSpace(item.Value), "/")
		if repository == "" {
			continue
		}
		image := ""
		if project != "" && strings.HasPrefix(repository, prefix) {
			image = strings.TrimPrefix(repository, prefix)
		} else if repository == project {
			image = repository
		}
		if image == "" {
			continue
		}
		if keyword != "" && !strings.Contains(strings.ToLower(image), keyword) {
			continue
		}
		if _, ok := seen[image]; ok {
			continue
		}
		seen[image] = struct{}{}
		result = append(result, devopstypes.DynamicOption{
			Label: image,
			Value: image,
			Metadata: map[string]any{
				"fullName": repository,
			},
		})
	}
	return result
}

func registryImageRepository(project, image string) string {
	project = strings.Trim(strings.TrimSpace(project), "/")
	image = strings.Trim(strings.TrimSpace(image), "/")
	if image == "" {
		return project
	}
	if strings.Contains(image, "/") || project == "" || image == project {
		return image
	}
	return project + "/" + image
}

func imageTagPairOptions(image string, tags []devopstypes.DynamicOption, limit uint64) []devopstypes.DynamicOption {
	if limit == 0 {
		return nil
	}
	result := make([]devopstypes.DynamicOption, 0, len(tags))
	for _, item := range tags {
		if uint64(len(result)) >= limit {
			break
		}
		tag := strings.TrimSpace(item.Value)
		if tag == "" {
			continue
		}
		value := strings.Trim(strings.TrimSpace(image), "/") + ":" + tag
		metadata := copyOptionMetadata(item.Metadata)
		metadata["image"] = strings.Trim(strings.TrimSpace(image), "/")
		metadata["tag"] = tag
		result = append(result, devopstypes.DynamicOption{
			Label:    value,
			Value:    value,
			Metadata: metadata,
		})
	}
	return result
}

func copyOptionMetadata(metadata map[string]any) map[string]any {
	result := make(map[string]any, len(metadata)+2)
	for key, value := range metadata {
		result[key] = value
	}
	return result
}

func remainingOptionLimit(items []devopstypes.DynamicOption, pageSize uint64) uint64 {
	if pageSize == 0 {
		return 0
	}
	if uint64(len(items)) >= pageSize {
		return 0
	}
	return pageSize - uint64(len(items))
}

func mapDynamicAddressOptions(options []devopstypes.DynamicOption, build func(devopstypes.DynamicOption) string) []devopstypes.DynamicOption {
	result := make([]devopstypes.DynamicOption, 0, len(options))
	for _, item := range options {
		value := strings.TrimSpace(build(item))
		if value == "" {
			value = item.Value
		}
		next := item
		next.Value = value
		if next.Metadata == nil {
			next.Metadata = map[string]any{}
		}
		next.Metadata["injectedValue"] = value
		result = append(result, next)
	}
	return result
}

func joinAddress(endpoint string, parts ...string) string {
	value := strings.TrimRight(strings.TrimSpace(endpoint), "/")
	for _, part := range parts {
		part = strings.Trim(strings.TrimSpace(part), "/")
		if part == "" {
			continue
		}
		if strings.Contains(part, "?") {
			value += "/" + strings.TrimLeft(part, "/")
			continue
		}
		value += "/" + part
	}
	return value
}

func normalizeGitProjectValue(value string) string {
	value = strings.TrimSpace(value)
	if strings.Contains(value, " / ") {
		parts := strings.Split(value, " / ")
		value = strings.TrimSpace(parts[len(parts)-1])
	}
	if projectPath := gitProjectPathFromRepositoryURL(value); projectPath != "" {
		return projectPath
	}
	return value
}

func wantsGitRepositoryURLCredential(value string) bool {
	switch strings.TrimSpace(value) {
	case gitRepositoryURLCredentialComponentValue, "withCredential", "includeCredential", "true":
		return true
	default:
		return false
	}
}

func withGitRepositoryURLCredential(options []devopstypes.DynamicOption, req devopstypes.Request) ([]devopstypes.DynamicOption, error) {
	username, secret, ok := gitRepositoryHTTPAuth(req)
	if !ok {
		logx.Errorf("渠道未配置可用于 HTTP 克隆的凭证")
		return nil, errorx.Msg("渠道未配置可用于 HTTP 克隆的凭证")
	}
	result := make([]devopstypes.DynamicOption, 0, len(options))
	for _, item := range options {
		next := item
		metadata := make(map[string]any, len(item.Metadata)+1)
		for key, value := range item.Metadata {
			metadata[key] = value
		}
		repositoryURL := metadataString(metadata, "httpUrlToRepo")
		if repositoryURL == "" {
			projectPath := metadataString(metadata, "pathWithNamespace")
			if projectPath == "" {
				projectPath = strings.TrimSpace(item.Value)
			}
			if projectPath != "" {
				repositoryURL = gitRepositoryURLFromEndpoint(req.Channel.Endpoint, projectPath)
			}
		}
		if value, ok := gitRepositoryURLWithCredential(repositoryURL, username, secret); ok {
			metadata["httpUrlToRepo"] = value
			metadata["credentialInjected"] = true
		}
		next.Metadata = metadata
		result = append(result, next)
	}
	return result, nil
}

func gitRepositoryHTTPAuth(req devopstypes.Request) (string, string, bool) {
	if req.Credential != nil {
		switch strings.TrimSpace(req.Credential.Type) {
		case "username_password":
			return nonEmptyPair(req.Credential.Username, req.Credential.Password)
		case "token":
			if strings.TrimSpace(req.Credential.Token) != "" {
				return gitHTTPUsername(req.Credential.Username), strings.TrimSpace(req.Credential.Token), true
			}
		case "secret_text":
			if strings.TrimSpace(req.Credential.SecretText) != "" {
				return gitHTTPUsername(req.Credential.Username), strings.TrimSpace(req.Credential.SecretText), true
			}
		}
	}
	switch strings.TrimSpace(req.Channel.AuthType) {
	case "username_password":
		return nonEmptyPair(req.Channel.Username, req.Channel.Password)
	case "token":
		if strings.TrimSpace(req.Channel.Token) != "" {
			return gitHTTPUsername(req.Channel.Username), strings.TrimSpace(req.Channel.Token), true
		}
	}
	return "", "", false
}

func gitHTTPUsername(username string) string {
	username = strings.TrimSpace(username)
	if username == "" {
		return "oauth2"
	}
	return username
}

func nonEmptyPair(username, password string) (string, string, bool) {
	username = strings.TrimSpace(username)
	password = strings.TrimSpace(password)
	return username, password, username != "" && password != ""
}

func gitRepositoryURLWithCredential(repositoryURL, username, secret string) (string, bool) {
	parsed, err := url.Parse(strings.TrimSpace(repositoryURL))
	if err != nil || parsed.Host == "" {
		return "", false
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", false
	}
	parsed.User = url.UserPassword(username, secret)
	return parsed.String(), true
}

func gitRepositoryURLFromEndpoint(endpoint, projectPath string) string {
	endpoint = strings.TrimRight(strings.TrimSpace(endpoint), "/")
	projectPath = strings.Trim(strings.TrimSpace(projectPath), "/")
	if endpoint == "" || projectPath == "" {
		return ""
	}
	value := endpoint + "/" + projectPath
	if !strings.HasSuffix(value, ".git") {
		value += ".git"
	}
	return value
}

func metadataString(metadata map[string]any, key string) string {
	if metadata == nil {
		return ""
	}
	value, _ := metadata[key].(string)
	return strings.TrimSpace(value)
}

func canResolveGitRefByRepositoryURL(paramType, projectValue string) bool {
	switch strings.TrimSpace(paramType) {
	case channelvars.ParamGitBranch, channelvars.ParamGitTag:
		return gitProjectPathFromRepositoryURL(projectValue) != ""
	default:
		return false
	}
}

func canResolveImageRegistryByAddress(paramType, projectValue string, dependencyValues map[string]string) bool {
	if imageRegistryAddressFromValues(projectValue, dependencyValues) == "" {
		return false
	}
	switch strings.TrimSpace(paramType) {
	case channelvars.ParamHarborImage, channelvars.ParamHarborImageURL, channelvars.ParamHarborImageTag,
		channelvars.ParamHarborImageTagURL, channelvars.ParamRegistryImage, channelvars.ParamRegistryImageURL,
		channelvars.ParamRegistryImageTag, channelvars.ParamRegistryImageTagURL:
		return true
	default:
		return false
	}
}

func canResolveArtifactRepositoryByAddress(paramType, projectValue string, dependencyValues map[string]string) bool {
	if artifactRepositoryAddressFromValues(projectValue, dependencyValues) == "" {
		return false
	}
	switch strings.TrimSpace(paramType) {
	case channelvars.ParamNexusArtifactName, channelvars.ParamNexusArtifactVersion, channelvars.ParamNexusArtifact,
		channelvars.ParamJfrogArtifactName, channelvars.ParamJfrogArtifactVersion, channelvars.ParamJfrogArtifact:
		return true
	default:
		return false
	}
}

func imageRegistryAddressFromValues(projectValue string, dependencyValues map[string]string) string {
	for _, value := range []string{
		dependencyValues[channelvars.FieldAddressImageTagURL],
		dependencyValues[channelvars.FieldAddressImageURL],
		dependencyValues[channelvars.FieldAddressProjectURL],
		dependencyValues[channelvars.FieldAddressRegistryURL],
		projectValue,
	} {
		if looksLikeImageRegistryAddress(value) {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func artifactRepositoryAddressFromValues(projectValue string, dependencyValues map[string]string) string {
	for _, value := range []string{
		dependencyValues[channelvars.FieldAddressArtifactVersionURL],
		dependencyValues[channelvars.FieldAddressArtifactTagURL],
		dependencyValues[channelvars.FieldAddressArtifactURL],
		dependencyValues[channelvars.FieldAddressRepositoryURL],
		projectValue,
	} {
		if looksLikeArtifactRepositoryAddress(value) {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func looksLikeImageRegistryAddress(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || !strings.Contains(value, "/") {
		return false
	}
	if strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") {
		return true
	}
	return strings.Contains(value, ".") || strings.Contains(value, ":")
}

func looksLikeArtifactRepositoryAddress(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || !strings.Contains(value, "/") {
		return false
	}
	if strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") {
		return true
	}
	return strings.Contains(value, ".") || strings.Contains(value, ":")
}

type imageRegistryAddressParts struct {
	Host    string
	Project string
	Image   string
	Tag     string
}

func parseImageRegistryAddress(value string) imageRegistryAddressParts {
	return parseImageRegistryAddressWithEndpoint(value, "")
}

func parseImageRegistryAddressWithEndpoint(value, endpoint string) imageRegistryAddressParts {
	value = strings.TrimSpace(value)
	if value == "" {
		return imageRegistryAddressParts{}
	}
	if !strings.Contains(value, "://") {
		value = "https://" + value
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" {
		return imageRegistryAddressParts{}
	}
	path := strings.Trim(parsed.EscapedPath(), "/")
	if decoded, err := url.PathUnescape(path); err == nil {
		path = decoded
	}
	if _, endpointPath := endpointHostAndPath(endpoint); endpointPath != "" {
		if path == endpointPath {
			path = ""
		} else if strings.HasPrefix(path, endpointPath+"/") {
			path = strings.TrimPrefix(path, endpointPath+"/")
		}
	}
	parts := strings.Split(path, "/")
	result := imageRegistryAddressParts{Host: parsed.Host}
	if len(parts) == 0 || strings.TrimSpace(parts[0]) == "" {
		return result
	}
	result.Project = strings.TrimSpace(parts[0])
	if len(parts) <= 1 {
		return result
	}
	imageWithTag := strings.TrimSpace(strings.Join(parts[1:], "/"))
	if idx := strings.LastIndex(imageWithTag, ":"); idx > -1 {
		result.Image = strings.TrimSpace(imageWithTag[:idx])
		result.Tag = strings.TrimSpace(imageWithTag[idx+1:])
	} else {
		result.Image = imageWithTag
	}
	return result
}

type artifactRepositoryAddressParts struct {
	Host       string
	Repository string
	Artifact   string
	Version    string
}

func parseArtifactRepositoryAddress(value string) artifactRepositoryAddressParts {
	return parseArtifactRepositoryAddressWithEndpoint(value, "")
}

func parseArtifactRepositoryAddressWithEndpoint(value, endpoint string) artifactRepositoryAddressParts {
	value = strings.TrimSpace(value)
	if value == "" {
		return artifactRepositoryAddressParts{}
	}
	if !strings.Contains(value, "://") {
		value = "https://" + value
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" {
		return artifactRepositoryAddressParts{}
	}
	path := strings.Trim(parsed.EscapedPath(), "/")
	if decoded, err := url.PathUnescape(path); err == nil {
		path = decoded
	}
	if _, endpointPath := endpointHostAndPath(endpoint); endpointPath != "" {
		if path == endpointPath {
			path = ""
		} else if strings.HasPrefix(path, endpointPath+"/") {
			path = strings.TrimPrefix(path, endpointPath+"/")
		}
	}
	parts := strings.Split(path, "/")
	result := artifactRepositoryAddressParts{Host: parsed.Host}
	if len(parts) == 0 || strings.TrimSpace(parts[0]) == "" {
		return result
	}
	result.Repository = strings.TrimSpace(parts[0])
	if len(parts) <= 1 {
		return result
	}
	artifactWithVersion := strings.TrimSpace(strings.Join(parts[1:], "/"))
	if idx := strings.LastIndex(artifactWithVersion, ":"); idx > -1 {
		result.Artifact = strings.TrimSpace(artifactWithVersion[:idx])
		result.Version = strings.TrimSpace(artifactWithVersion[idx+1:])
	} else {
		result.Artifact = artifactWithVersion
	}
	return result
}

func gitProjectPathFromRepositoryURL(value string) string {
	_, path := gitRepositoryHostAndPath(value)
	return path
}

func gitRepositoryHostAndPath(value string) (string, string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", ""
	}
	if at := strings.Index(value, "@"); at > 0 && !strings.Contains(value[:at], "://") {
		rest := value[at+1:]
		if colon := strings.Index(rest, ":"); colon > 0 {
			return strings.TrimSpace(rest[:colon]), cleanGitRepositoryPath(rest[colon+1:])
		}
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" {
		return "", ""
	}
	path := strings.Trim(parsed.EscapedPath(), "/")
	if path == "" {
		return parsed.Host, ""
	}
	if decoded, err := url.PathUnescape(path); err == nil {
		path = decoded
	}
	return parsed.Host, cleanGitRepositoryPath(path)
}

func cleanGitRepositoryPath(path string) string {
	path = strings.Trim(strings.TrimSpace(path), "/")
	path = strings.TrimSuffix(path, ".git")
	return path
}

func dynamicParamError(paramType string, err error) error {
	if err == nil {
		return nil
	}
	message := devopstypes.TrimMessage(err.Error())
	switch paramType {
	case channelvars.ParamGitBranch:
		logx.Errorf("%s", "获取 Git 分支失败："+message)
		return errorx.Msg("获取 Git 分支失败：" + message)
	case channelvars.ParamGitTag:
		logx.Errorf("%s", "获取 Git Tag 失败："+message)
		return errorx.Msg("获取 Git Tag 失败：" + message)
	case channelvars.ParamGitProject, channelvars.ParamGitRepositoryURL:
		logx.Errorf("%s", "获取 Git 项目失败："+message)
		return errorx.Msg("获取 Git 项目失败：" + message)
	case channelvars.ParamNexusRepository, channelvars.ParamNexusRepositoryURL:
		logx.Errorf("%s", "获取 Nexus 仓库失败："+message)
		return errorx.Msg("获取 Nexus 仓库失败：" + message)
	case channelvars.ParamNexusArtifactName, channelvars.ParamNexusArtifactVersion, channelvars.ParamNexusArtifact:
		logx.Errorf("%s", "获取 Nexus 制品失败："+message)
		return errorx.Msg("获取 Nexus 制品失败：" + message)
	case channelvars.ParamHarborProject, channelvars.ParamHarborProjectURL:
		logx.Errorf("%s", "获取 Harbor 项目失败："+message)
		return errorx.Msg("获取 Harbor 项目失败：" + message)
	case channelvars.ParamHarborImage, channelvars.ParamHarborImageURL, channelvars.ParamHarborImageTagURL:
		logx.Errorf("%s", "获取 Harbor 镜像失败："+message)
		return errorx.Msg("获取 Harbor 镜像失败：" + message)
	case channelvars.ParamHarborImageTag:
		logx.Errorf("%s", "获取 Harbor 镜像 Tag 失败："+message)
		return errorx.Msg("获取 Harbor 镜像 Tag 失败：" + message)
	case channelvars.ParamRegistryRepository:
		logx.Errorf("%s", "获取镜像仓库失败："+message)
		return errorx.Msg("获取镜像仓库失败：" + message)
	case channelvars.ParamRegistryImage:
		logx.Errorf("%s", "获取镜像失败："+message)
		return errorx.Msg("获取镜像失败：" + message)
	case channelvars.ParamRegistryImageTag:
		logx.Errorf("%s", "获取镜像 Tag 失败："+message)
		return errorx.Msg("获取镜像 Tag 失败：" + message)
	case channelvars.ParamJfrogRepository, channelvars.ParamJfrogRepositoryURL:
		logx.Errorf("%s", "获取 JFrog 仓库失败："+message)
		return errorx.Msg("获取 JFrog 仓库失败：" + message)
	case channelvars.ParamJfrogArtifactName, channelvars.ParamJfrogArtifactVersion, channelvars.ParamJfrogArtifact:
		logx.Errorf("%s", "获取 JFrog 制品失败："+message)
		return errorx.Msg("获取 JFrog 制品失败：" + message)
	case channelvars.ParamKubeNovaProject:
		logx.Errorf("%s", "获取 kube-nova 项目失败："+message)
		return errorx.Msg("获取 kube-nova 项目失败：" + message)
	case channelvars.ParamKubeNovaCluster:
		logx.Errorf("%s", "获取 kube-nova 集群失败："+message)
		return errorx.Msg("获取 kube-nova 集群失败：" + message)
	case channelvars.ParamKubeNovaWorkspace:
		logx.Errorf("%s", "获取 kube-nova 工作空间失败："+message)
		return errorx.Msg("获取 kube-nova 工作空间失败：" + message)
	case channelvars.ParamKubeNovaApplication:
		logx.Errorf("%s", "获取 kube-nova 应用失败："+message)
		return errorx.Msg("获取 kube-nova 应用失败：" + message)
	case channelvars.ParamKubeNovaVersion:
		logx.Errorf("%s", "获取 kube-nova 版本失败："+message)
		return errorx.Msg("获取 kube-nova 版本失败：" + message)
	case channelvars.ParamKubeNovaContainer:
		logx.Errorf("%s", "获取 kube-nova 容器失败："+message)
		return errorx.Msg("获取 kube-nova 容器失败：" + message)
	case channelvars.ParamSonarProjectName:
		logx.Errorf("%s", "获取 Sonar 项目名称失败："+message)
		return errorx.Msg("获取 Sonar 项目名称失败：" + message)
	case channelvars.ParamSonarProjectKey:
		logx.Errorf("%s", "获取 Sonar 项目 Key 失败："+message)
		return errorx.Msg("获取 Sonar 项目 Key 失败：" + message)
	case channelvars.ParamKubernetesNamespace:
		logx.Errorf("%s", "获取 Kubernetes 命名空间失败："+message)
		return errorx.Msg("获取 Kubernetes 命名空间失败：" + message)
	case channelvars.ParamKubernetesWorkloadType:
		logx.Errorf("%s", "获取 Kubernetes 部署类型失败："+message)
		return errorx.Msg("获取 Kubernetes 部署类型失败：" + message)
	case channelvars.ParamKubernetesResource:
		logx.Errorf("%s", "获取 Kubernetes 资源失败："+message)
		return errorx.Msg("获取 Kubernetes 资源失败：" + message)
	case channelvars.ParamKubernetesContainer:
		logx.Errorf("%s", "获取 Kubernetes 容器失败："+message)
		return errorx.Msg("获取 Kubernetes 容器失败：" + message)
	case channelvars.ParamKubernetesImage:
		logx.Errorf("%s", "获取 Kubernetes 镜像失败："+message)
		return errorx.Msg("获取 Kubernetes 镜像失败：" + message)
	default:
		logx.Errorf("查询动态参数选项失败: %v", err)
		return err
	}
}

func (l *DynamicParamOptionsLogic) channelBindingOptions(in *pb.DynamicParamOptionsReq, paramType string, page, pageSize uint64) ([]devopstypes.DynamicOption, uint64, error) {
	groupCode := channelvars.ChannelGroupCode(paramType)
	if paramType == channelvars.ParamChannelVariable {
		groupCode = strings.TrimSpace(in.ConfigTypeCode)
	}
	if groupCode == "" {
		l.Errorf("平台渠道类型不支持")
		return nil, 0, errorx.Msg("平台渠道类型不支持")
	}
	channelType := channelvars.ChannelTypeFilter(paramType)
	if paramType == channelvars.ParamChannelVariable {
		channelType = strings.TrimSpace(in.ComponentValue)
	}
	bindings, total, err := l.svcCtx.ProjectChannelModel.List(l.ctx, model.DevopsProjectChannelBindingListFilter{
		ProjectID:        in.ProjectId,
		ChannelGroupCode: groupCode,
		ChannelType:      channelType,
		Status:           1,
		Page:             page,
		PageSize:         pageSize,
	})
	if err != nil {
		l.Errorf("查询渠道绑定选项失败: %v", err)
		return nil, 0, err
	}
	keyword := strings.ToLower(strings.TrimSpace(in.Keyword))
	channelIDs := make([]string, 0, len(bindings))
	for _, binding := range bindings {
		if binding != nil && strings.TrimSpace(binding.ChannelID) != "" {
			channelIDs = append(channelIDs, binding.ChannelID)
		}
	}
	channels, err := l.svcCtx.ChannelModel.FindByIDs(l.ctx, channelIDs)
	if err != nil {
		l.Errorf("查询渠道绑定选项失败: %v", err)
		return nil, 0, err
	}
	options := make([]devopstypes.DynamicOption, 0, len(bindings))
	for _, binding := range bindings {
		endpoint := ""
		var channel *model.DevopsChannel
		if strings.TrimSpace(binding.ChannelID) != "" {
			channel = channels[binding.ChannelID]
			if channel == nil {
				l.Errorf("查询渠道绑定选项失败: 渠道不存在")
				return nil, 0, model.ErrNotFound
			}
			endpoint = strings.TrimSpace(channel.Endpoint)
		}
		label := binding.ChannelName
		if binding.ChannelCode != "" {
			label = label + "（" + binding.ChannelCode + "）"
		}
		if keyword != "" &&
			!strings.Contains(strings.ToLower(label), keyword) &&
			!strings.Contains(strings.ToLower(binding.ChannelType), keyword) {
			continue
		}
		metadata := map[string]any{
			"channelId":        binding.ChannelID,
			"channelType":      binding.ChannelType,
			"channelGroupCode": binding.ChannelGroupCode,
			"channelCode":      binding.ChannelCode,
			"isDefault":        binding.IsDefault,
			"endpoint":         endpoint,
		}
		if isKubeNovaDeployBindingOption(paramType, channelType, binding.ChannelType) && channel != nil {
			tokenBase64, scope, message, err := l.kubeNovaTokenBase64(binding, channel)
			if err != nil {
				l.Errorf("查询 kube-nova 绑定凭据失败: %v", err)
				return nil, 0, err
			}
			metadata["allowUseGlobalCredential"] = binding.AllowUseGlobalCredential
			metadata["credentialScope"] = scope
			metadata["credentialMessage"] = message
			metadata["credentialReady"] = tokenBase64 != ""
		}
		if isKubernetesDeployBindingOption(paramType, channelType, binding.ChannelType) && channel != nil {
			ready, scope, message, credentialType, err := l.kubernetesCredentialStatus(binding, channel)
			if err != nil {
				l.Errorf("查询 Kubernetes 绑定凭据失败: %v", err)
				return nil, 0, err
			}
			metadata["allowUseGlobalCredential"] = binding.AllowUseGlobalCredential
			metadata["credentialReady"] = ready
			metadata["credentialScope"] = scope
			metadata["credentialType"] = credentialType
			metadata["credentialContentType"] = "auto"
			metadata["credentialMessage"] = message
			metadata["insecureSkipTLS"] = channel.InsecureSkipTLS
		}
		options = append(options, devopstypes.DynamicOption{
			Label:    label,
			Value:    binding.ID.Hex(),
			Metadata: metadata,
		})
	}
	if keyword != "" {
		total = uint64(len(options))
	}
	return options, total, nil
}

func isKubeNovaDeployBindingOption(paramType, requestedChannelType, bindingChannelType string) bool {
	if paramType == channelvars.ParamKubeNovaDeployConfig {
		return true
	}
	return paramType == channelvars.ParamChannelVariable &&
		strings.TrimSpace(requestedChannelType) == "kube-nova" &&
		strings.TrimSpace(bindingChannelType) == "kube-nova"
}

func isKubernetesDeployBindingOption(paramType, requestedChannelType, bindingChannelType string) bool {
	if paramType == channelvars.ParamKubernetesDeployConfig {
		return true
	}
	return paramType == channelvars.ParamChannelVariable &&
		strings.TrimSpace(requestedChannelType) == "kubernetes" &&
		strings.TrimSpace(bindingChannelType) == "kubernetes"
}

func (l *DynamicParamOptionsLogic) kubeNovaTokenBase64(binding *model.DevopsProjectChannelBinding, channel *model.DevopsChannel) (string, string, string, error) {
	if binding == nil || channel == nil {
		return "", "none", "kube-nova 实例绑定不存在", nil
	}
	projectCredentialID := strings.TrimSpace(binding.ProjectCredentialID)
	if projectCredentialID != "" {
		credential, err := l.svcCtx.CredentialModel.FindOne(l.ctx, projectCredentialID)
		if err != nil {
			return "", "project", "", err
		}
		if credential.ProjectID != binding.ProjectID || normalizeCredentialScope(credential.Scope, credential.ProjectID, credential.IsSystem) != "project" {
			return "", "project", "", errorx.Msg("项目渠道凭据必须属于当前项目")
		}
		tokenBase64, message, err := credentialTokenBase64(credential)
		if err != nil {
			return "", "project", "", err
		}
		if tokenBase64 == "" {
			if message != "" {
				return "", "project", message, nil
			}
			return "", "project", "项目绑定凭据不是 token 或 Secret Text", nil
		}
		return tokenBase64, "project", "已使用项目绑定 token 凭据", nil
	}
	if !binding.AllowUseGlobalCredential {
		return "", "none", "未绑定项目凭据，且未允许使用全局凭据", nil
	}
	credentialID := strings.TrimSpace(channel.CredentialID)
	if credentialID == "" {
		credentialID = strings.TrimSpace(channel.GlobalCredentialID)
	}
	if credentialID != "" {
		credential, err := l.svcCtx.CredentialModel.FindOne(l.ctx, credentialID)
		if err != nil {
			return "", "global", "", err
		}
		tokenBase64, message, err := credentialTokenBase64(credential)
		if err != nil {
			return "", "global", "", err
		}
		if tokenBase64 == "" {
			if message != "" {
				return "", "global", message, nil
			}
			return "", "global", "全局凭据不是 token 或 Secret Text", nil
		}
		return tokenBase64, "global", "已使用全局 token 凭据", nil
	}
	token := strings.TrimSpace(channel.Token)
	if token != "" {
		token = authutil.NormalizeBearerToken("kube-nova", token)
		return base64.StdEncoding.EncodeToString([]byte(token)), "global", "已使用渠道全局 token", nil
	}
	return "", "global", "已允许使用全局凭据，但渠道未配置 token 凭据", nil
}

func credentialTokenBase64(credential *model.DevopsCredential) (string, string, error) {
	if credential == nil {
		return "", "", nil
	}
	if credential.Status != 1 {
		return "", "", errorx.Msg("渠道凭据已停用")
	}
	secret, err := credential.Secret()
	if err != nil {
		return "", "", err
	}
	token := ""
	switch credential.CredentialType {
	case "token":
		token = strings.TrimSpace(secret.Token)
	case "secret_text":
		token = strings.TrimSpace(secret.SecretText)
	default:
		if strings.TrimSpace(secret.Token) != "" {
			token = strings.TrimSpace(secret.Token)
		} else if strings.TrimSpace(secret.SecretText) != "" {
			token = strings.TrimSpace(secret.SecretText)
		}
	}
	if token == "" {
		return "", "", nil
	}
	token = authutil.NormalizeBearerToken("kube-nova", token)
	return base64.StdEncoding.EncodeToString([]byte(token)), "", nil
}

func isKubeNovaBinding(binding *model.DevopsProjectChannelBinding, req devopstypes.Request) bool {
	if binding != nil && binding.ChannelType == "kube-nova" {
		return true
	}
	return req.Channel.Type == "kube-nova"
}

func (l *DynamicParamOptionsLogic) ensureKubeNovaCredentialReady(binding *model.DevopsProjectChannelBinding, channel *model.DevopsChannel) error {
	tokenBase64, _, message, err := l.kubeNovaTokenBase64(binding, channel)
	if err != nil {
		return err
	}
	if strings.TrimSpace(tokenBase64) == "" {
		if strings.TrimSpace(message) == "" {
			message = "当前 kube-nova 实例未配置可用 token 凭据"
		}
		l.Errorf("kube-nova 凭据不可用: %s", message)
		return errorx.Msg(message)
	}
	return nil
}

func (l *DynamicParamOptionsLogic) ensureKubernetesDeployCredentialReady(binding *model.DevopsProjectChannelBinding, channel *model.DevopsChannel) error {
	ready, _, message, _, err := l.kubernetesCredentialStatus(binding, channel)
	if err != nil {
		return err
	}
	if !ready {
		l.Errorf("Kubernetes 部署凭据不可用: %s", message)
		return errorx.Msg(message)
	}
	return nil
}

func (l *DynamicParamOptionsLogic) kubernetesCredentialStatus(binding *model.DevopsProjectChannelBinding, channel *model.DevopsChannel) (bool, string, string, string, error) {
	if binding == nil || channel == nil {
		return false, "none", "Kubernetes 渠道绑定不存在", "", nil
	}
	projectCredentialID := strings.TrimSpace(binding.ProjectCredentialID)
	if projectCredentialID != "" {
		credential, err := l.svcCtx.CredentialModel.FindOne(l.ctx, projectCredentialID)
		if err != nil {
			return false, "project", "", "", err
		}
		if credential.ProjectID != binding.ProjectID || credential.Scope != "project" || credential.IsSystem {
			return false, "project", "", "", errorx.Msg("项目渠道凭据必须属于当前项目")
		}
		credentialType, ready, err := kubernetesCredentialContentReady(credential)
		if err != nil || !ready {
			return false, "project", "项目绑定凭据必须是 token、Secret Text 或 kubeconfig", credentialType, err
		}
		return true, "project", "已使用项目绑定 Kubernetes 凭据", credentialType, nil
	}
	if !binding.AllowUseGlobalCredential {
		return false, "none", "未绑定项目凭据，且未允许使用全局凭据", "", nil
	}
	credentialID := strings.TrimSpace(channel.CredentialID)
	if credentialID == "" {
		credentialID = strings.TrimSpace(channel.GlobalCredentialID)
	}
	if credentialID == "" {
		return false, "global", "Kubernetes 渠道未绑定平台凭据，流水线部署必须通过 Jenkins 凭证传递 token 或 kubeconfig", "", nil
	}
	credential, err := l.svcCtx.CredentialModel.FindOne(l.ctx, credentialID)
	if err != nil {
		return false, "global", "", "", err
	}
	scope := normalizeCredentialScope(credential.Scope, credential.ProjectID, credential.IsSystem)
	if scope == "project" && credential.ProjectID != binding.ProjectID {
		return false, "global", "", "", errorx.Msg("项目渠道凭据必须属于当前项目")
	}
	credentialType, ready, err := kubernetesCredentialContentReady(credential)
	if err != nil || !ready {
		return false, "global", "全局凭据必须是 token、Secret Text 或 kubeconfig", credentialType, err
	}
	return true, scope, "已使用全局 Kubernetes 凭据", credentialType, nil
}

func kubernetesCredentialContentReady(credential *model.DevopsCredential) (string, bool, error) {
	if credential == nil {
		return "", false, nil
	}
	if credential.Status != 1 {
		return credential.CredentialType, false, errorx.Msg("渠道凭据已停用")
	}
	secret, err := credential.Secret()
	if err != nil {
		return credential.CredentialType, false, err
	}
	switch credential.CredentialType {
	case "token":
		return "token", strings.TrimSpace(secret.Token) != "", nil
	case "secret_text":
		return "secret_text", strings.TrimSpace(secret.SecretText) != "", nil
	case "kubeconfig":
		return "kubeconfig", strings.TrimSpace(secret.Kubeconfig) != "", nil
	default:
		if strings.TrimSpace(secret.Token) != "" {
			return "token", true, nil
		}
		if strings.TrimSpace(secret.SecretText) != "" {
			return "secret_text", true, nil
		}
		if strings.TrimSpace(secret.Kubeconfig) != "" {
			return "kubeconfig", true, nil
		}
		return credential.CredentialType, false, nil
	}
}

func isKubernetesBinding(binding *model.DevopsProjectChannelBinding, req devopstypes.Request) bool {
	if binding != nil && binding.ChannelType == "kubernetes" {
		return true
	}
	return req.Channel.Type == "kubernetes"
}

func (l *DynamicParamOptionsLogic) mavenConfigOptions(in *pb.DynamicParamOptionsReq, page, pageSize uint64) ([]devopstypes.DynamicOption, uint64, error) {
	items, total, err := l.svcCtx.ProjectConfigModel.List(l.ctx, model.DevopsProjectConfigListFilter{
		ProjectID: in.ProjectId,
		TypeCode:  model.DefaultMavenSettingsTypeCode,
		Name:      strings.TrimSpace(in.Keyword),
		Status:    1,
		Page:      page,
		PageSize:  pageSize,
	})
	if err != nil {
		l.Errorf("查询 Maven 配置选项失败: %v", err)
		return nil, 0, err
	}
	options := make([]devopstypes.DynamicOption, 0, len(items))
	for _, item := range items {
		label := item.Name
		if item.Code != "" {
			label = label + "（" + item.Code + "）"
		}
		options = append(options, devopstypes.DynamicOption{
			Label: label,
			Value: item.ID.Hex(),
			Metadata: map[string]any{
				"projectId": item.ProjectID,
				"code":      item.Code,
				"typeCode":  item.TypeCode,
			},
		})
	}
	return options, total, nil
}

func (l *DynamicParamOptionsLogic) projectConfigOptions(in *pb.DynamicParamOptionsReq, page, pageSize uint64) ([]devopstypes.DynamicOption, uint64, error) {
	configType, err := resolveProjectConfigType(l.ctx, l.svcCtx, in.ConfigTypeId, in.ConfigTypeCode)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			l.Errorf("配置类型不存在")
			return nil, 0, errorx.Msg("配置类型不存在")
		}
		l.Errorf("查询配置类型失败: %v", err)
		return nil, 0, err
	}
	items, total, err := l.svcCtx.ProjectConfigModel.List(l.ctx, model.DevopsProjectConfigListFilter{
		ProjectID: in.ProjectId,
		TypeCode:  configType.Code,
		Name:      strings.TrimSpace(in.Keyword),
		Status:    1,
		Page:      page,
		PageSize:  pageSize,
	})
	if err != nil {
		l.Errorf("查询配置中心选项失败: %v", err)
		return nil, 0, err
	}
	options := make([]devopstypes.DynamicOption, 0, len(items))
	for _, item := range items {
		label := item.Name
		if item.Code != "" {
			label = label + "（" + item.Code + "）"
		}
		options = append(options, devopstypes.DynamicOption{
			Label: label,
			Value: item.ID.Hex(),
			Metadata: map[string]any{
				"projectId": item.ProjectID,
				"typeId":    item.TypeID,
				"typeCode":  item.TypeCode,
				"code":      item.Code,
			},
		})
	}
	return options, total, nil
}

func (l *DynamicParamOptionsLogic) sonarAddressOptions(in *pb.DynamicParamOptionsReq, page, pageSize uint64) ([]devopstypes.DynamicOption, uint64, error) {
	bindings, err := l.activeSonarBindings(in.ProjectId)
	if err != nil {
		l.Errorf("查询 Sonar 地址选项失败: %v", err)
		return nil, 0, err
	}
	return l.addressBindingOptions(bindings, uint64(len(bindings)), in.Keyword)
}

func (l *DynamicParamOptionsLogic) hostGroupTargetOptions(in *pb.DynamicParamOptionsReq, page, pageSize uint64) ([]devopstypes.DynamicOption, uint64, error) {
	bindings, total, err := l.svcCtx.ProjectChannelModel.List(l.ctx, model.DevopsProjectChannelBindingListFilter{
		ProjectID:        in.ProjectId,
		ChannelGroupCode: channelvars.GroupDeployTarget,
		ChannelType:      "host_group",
		Status:           1,
		Page:             page,
		PageSize:         pageSize,
	})
	if err != nil {
		l.Errorf("查询主机组选项失败: %v", err)
		return nil, 0, err
	}
	return l.addressBindingOptions(bindings, total, in.Keyword)
}

func (l *DynamicParamOptionsLogic) hostConfigOptions(in *pb.DynamicParamOptionsReq, page, pageSize uint64) ([]devopstypes.DynamicOption, uint64, error) {
	_, binding, channel, err := l.dynamicProviderRequest(in)
	if err != nil {
		return nil, 0, err
	}
	if binding.ChannelType != "host" || channel.ChannelType != "host" {
		return nil, 0, errorx.Msg("主机配置必须绑定主机渠道")
	}
	hostIDs, err := hostIDsFromConfig(binding.BindingConfig, channel.Config)
	if err != nil {
		return nil, 0, err
	}
	targets, order, err := l.resolveHostOptionTargets(hostIDs)
	if err != nil {
		return nil, 0, err
	}
	start, end := optionPageBounds(len(order), page, pageSize)
	pagedOrder := order[start:end]
	options := make([]devopstypes.DynamicOption, 0, len(pagedOrder))
	keyword := strings.ToLower(strings.TrimSpace(in.Keyword))
	for _, name := range pagedOrder {
		target := targets[name]
		label := name
		if target.Host != "" && target.Host != name {
			label = label + "（" + target.Host + "）"
		}
		if keyword != "" &&
			!strings.Contains(strings.ToLower(label), keyword) &&
			!strings.Contains(strings.ToLower(target.Host), keyword) {
			continue
		}
		jsonValue, err := renderHostSingleValue(target, "json")
		if err != nil {
			return nil, 0, err
		}
		jsonBase64Value, err := renderHostSingleValue(target, "jsonBase64")
		if err != nil {
			return nil, 0, err
		}
		ansibleValue, err := renderHostGroupTargets(map[string]hostGroupTarget{name: target}, []string{name}, "ansible")
		if err != nil {
			return nil, 0, err
		}
		ansibleBase64Value, err := renderHostGroupTargets(map[string]hostGroupTarget{name: target}, []string{name}, "ansibleBase64")
		if err != nil {
			return nil, 0, err
		}
		options = append(options, devopstypes.DynamicOption{
			Label: label,
			Value: jsonValue,
			Metadata: map[string]any{
				"endpoint":      channel.Endpoint,
				"bindingId":     binding.ID.Hex(),
				"channelId":     channel.ID.Hex(),
				"channelType":   binding.ChannelType,
				"host":          target.Host,
				"port":          target.Port,
				"username":      target.Username,
				"json":          jsonValue,
				"jsonBase64":    jsonBase64Value,
				"ansible":       ansibleValue,
				"ansibleBase64": ansibleBase64Value,
			},
		})
	}
	total := uint64(len(order))
	if keyword != "" {
		total = uint64(len(options))
	}
	return options, total, nil
}

func (l *DynamicParamOptionsLogic) hostGroupConfigOptions(in *pb.DynamicParamOptionsReq, page, pageSize uint64) ([]devopstypes.DynamicOption, uint64, error) {
	_, binding, channel, err := l.dynamicProviderRequest(in)
	if err != nil {
		return nil, 0, err
	}
	if binding.ChannelType != "host_group" || channel.ChannelType != "host_group" {
		return nil, 0, errorx.Msg("主机组配置必须绑定主机组渠道")
	}
	hostIDs, err := hostIDsFromConfig(binding.BindingConfig, channel.Config)
	if err != nil {
		return nil, 0, err
	}
	targets, order, err := l.resolveHostOptionTargets(hostIDs)
	if err != nil {
		return nil, 0, err
	}
	jsonValue, err := renderHostGroupTargets(targets, order, "json")
	if err != nil {
		return nil, 0, err
	}
	jsonBase64Value, err := renderHostGroupTargets(targets, order, "jsonBase64")
	if err != nil {
		return nil, 0, err
	}
	ansibleValue, err := renderHostGroupTargets(targets, order, "ansible")
	if err != nil {
		return nil, 0, err
	}
	ansibleBase64Value, err := renderHostGroupTargets(targets, order, "ansibleBase64")
	if err != nil {
		return nil, 0, err
	}
	label := firstNotBlank(binding.ChannelName, channel.Name, binding.ChannelCode, channel.Code, "主机组配置")
	if len(order) > 0 {
		label = label + "（" + strconv.Itoa(len(order)) + " 台）"
	}
	if keyword := strings.ToLower(strings.TrimSpace(in.Keyword)); keyword != "" &&
		!strings.Contains(strings.ToLower(label), keyword) &&
		!strings.Contains(strings.ToLower(channel.Endpoint), keyword) {
		return nil, 0, nil
	}
	return []devopstypes.DynamicOption{{
		Label: label,
		Value: jsonValue,
		Metadata: map[string]any{
			"endpoint":      channel.Endpoint,
			"bindingId":     binding.ID.Hex(),
			"channelId":     channel.ID.Hex(),
			"channelType":   binding.ChannelType,
			"count":         len(order),
			"json":          jsonValue,
			"jsonBase64":    jsonBase64Value,
			"ansible":       ansibleValue,
			"ansibleBase64": ansibleBase64Value,
		},
	}}, 1, nil
}

func renderHostSingleValue(target hostGroupTarget, format string) (string, error) {
	switch normalizeHostGroupRenderMode(format) {
	case "normal", "json":
		data, err := json.MarshalIndent(target, "", "  ")
		if err != nil {
			return "", err
		}
		return string(data), nil
	case "jsonBase64":
		value, err := renderHostSingleValue(target, "json")
		if err != nil {
			return "", err
		}
		return base64.StdEncoding.EncodeToString([]byte(value)), nil
	case "inventory", "ansible":
		return renderHostGroupTargets(map[string]hostGroupTarget{target.Host: target}, []string{target.Host}, "ansible")
	case "ansibleBase64":
		return renderHostGroupTargets(map[string]hostGroupTarget{target.Host: target}, []string{target.Host}, "ansibleBase64")
	default:
		return "", errors.New("不支持的主机配置输出格式")
	}
}

func (l *DynamicParamOptionsLogic) resolveHostOptionTargets(hostIDs []string) (map[string]hostGroupTarget, []string, error) {
	return NewResolveHostGroupTargetsLogic(l.ctx, l.svcCtx).resolveHostTargets(hostIDs)
}

func optionPageBounds(total int, page, pageSize uint64) (int, int) {
	if pageSize == 0 || total == 0 {
		return 0, total
	}
	if page == 0 {
		page = 1
	}
	start := int((page - 1) * pageSize)
	if start >= total {
		return total, total
	}
	end := start + int(pageSize)
	if end > total {
		end = total
	}
	return start, end
}

func (l *DynamicParamOptionsLogic) sonarProjectKeyOptions(in *pb.DynamicParamOptionsReq, page, pageSize uint64) ([]devopstypes.DynamicOption, uint64, error) {
	if strings.TrimSpace(in.ChannelBindingId) == "" {
		return nil, 0, nil
	}
	req, _, _, err := l.sonarRequestByAddress(in)
	if err != nil {
		l.Errorf("构造 Sonar 项目 Key 请求失败: %v", err)
		return nil, 0, err
	}
	provider, ok := devopssonar.New().(interface {
		ListProjects(context.Context, devopstypes.Request, string, uint64, uint64) ([]devopstypes.DynamicOption, uint64, error)
	})
	if !ok {
		l.Errorf("Sonar 项目查询不可用")
		return nil, 0, errorx.Msg("Sonar 项目查询不可用")
	}
	return provider.ListProjects(l.ctx, req, in.Keyword, page, pageSize)
}

func (l *DynamicParamOptionsLogic) sonarRequestByAddress(in *pb.DynamicParamOptionsReq) (devopstypes.Request, *model.DevopsProjectChannelBinding, *model.DevopsChannel, error) {
	source := strings.TrimSpace(in.ChannelBindingId)
	if source == "" {
		l.Errorf("请选择 Sonar 地址")
		return devopstypes.Request{}, nil, nil, errorx.Msg("请选择 Sonar 地址")
	}
	if len(source) == 24 {
		binding, err := l.svcCtx.ProjectChannelModel.FindOne(l.ctx, source)
		if err == nil && binding.ProjectID == in.ProjectId && binding.Status == 1 {
			channel, err := l.svcCtx.ChannelModel.FindOne(l.ctx, binding.ChannelID)
			if err != nil {
				l.Errorf("查询 Sonar 渠道失败: %v", err)
				return devopstypes.Request{}, nil, nil, err
			}
			if channel.Status != 1 {
				l.Errorf("Sonar 渠道已停用")
				return devopstypes.Request{}, nil, nil, errorx.Msg("Sonar 渠道已停用")
			}
			if binding.ChannelType != "sonarqube" && channel.ChannelType != "sonarqube" {
				l.Errorf("Sonar 地址参数必须绑定 SonarQube 渠道")
				return devopstypes.Request{}, nil, nil, errorx.Msg("Sonar 地址参数必须绑定 SonarQube 渠道")
			}
			req, err := l.dynamicRequestFromBinding(binding, channel)
			return req, binding, channel, err
		}
	}
	binding, channel, err := l.sonarBindingByAddress(in.ProjectId, source)
	if err != nil {
		l.Errorf("匹配 Sonar 地址失败: %v", err)
		return devopstypes.Request{}, nil, nil, err
	}
	req, err := l.dynamicRequestFromBinding(binding, channel)
	return req, binding, channel, err
}

func (l *DynamicParamOptionsLogic) sonarBindingByAddress(projectID, source string) (*model.DevopsProjectChannelBinding, *model.DevopsChannel, error) {
	bindings, err := l.activeSonarBindings(projectID)
	if err != nil {
		l.Errorf("查询 Sonar 渠道绑定失败: %v", err)
		return nil, nil, errorx.Msg("查询 Sonar 渠道绑定失败")
	}
	sourceEndpoint := strings.TrimRight(strings.TrimSpace(source), "/")
	var fallbackBinding *model.DevopsProjectChannelBinding
	var fallbackChannel *model.DevopsChannel
	fallbackCount := 0
	for _, binding := range bindings {
		if binding == nil {
			continue
		}
		channel, err := l.svcCtx.ChannelModel.FindOne(l.ctx, binding.ChannelID)
		if err != nil {
			if errors.Is(err, model.ErrNotFound) {
				continue
			}
			l.Errorf("查询 Sonar 渠道失败: %v", err)
			return nil, nil, errorx.Msg("查询 Sonar 渠道失败")
		}
		if channel.Status != 1 {
			continue
		}
		endpoint := strings.TrimRight(strings.TrimSpace(channel.Endpoint), "/")
		if binding.ID.Hex() == source || strings.EqualFold(sourceEndpoint, endpoint) || repositoryURLMatchesEndpoint(source, channel.Endpoint) {
			return binding, channel, nil
		}
		fallbackBinding = binding
		fallbackChannel = channel
		fallbackCount++
	}
	if fallbackCount == 1 {
		return fallbackBinding, fallbackChannel, nil
	}
	if fallbackCount > 1 {
		l.Errorf("Sonar 地址无法匹配唯一渠道实例")
		return nil, nil, errorx.Msg("Sonar 地址无法匹配唯一渠道实例")
	}
	l.Errorf("当前项目未绑定 Sonar 渠道")
	return nil, nil, errorx.Msg("当前项目未绑定 Sonar 渠道")
}

func (l *DynamicParamOptionsLogic) activeSonarBindings(projectID string) ([]*model.DevopsProjectChannelBinding, error) {
	bindings, _, err := l.svcCtx.ProjectChannelModel.List(l.ctx, model.DevopsProjectChannelBindingListFilter{
		ProjectID: projectID,
		Status:    1,
		Page:      1,
		PageSize:  200,
	})
	if err != nil {
		return nil, err
	}
	result := make([]*model.DevopsProjectChannelBinding, 0, len(bindings))
	for _, binding := range bindings {
		if binding == nil {
			continue
		}
		if binding.ChannelType == "sonarqube" {
			result = append(result, binding)
			continue
		}
		channel, err := l.svcCtx.ChannelModel.FindOne(l.ctx, binding.ChannelID)
		if err != nil {
			if errors.Is(err, model.ErrNotFound) {
				continue
			}
			return nil, err
		}
		if channel.Status != 1 || channel.ChannelType != "sonarqube" {
			continue
		}
		binding.ChannelType = channel.ChannelType
		if binding.ChannelName == "" {
			binding.ChannelName = channel.Name
		}
		if binding.ChannelCode == "" {
			binding.ChannelCode = channel.Code
		}
		result = append(result, binding)
	}
	return result, nil
}

func (l *DynamicParamOptionsLogic) addressBindingOptions(bindings []*model.DevopsProjectChannelBinding, total uint64, keyword string) ([]devopstypes.DynamicOption, uint64, error) {
	keyword = strings.ToLower(strings.TrimSpace(keyword))
	options := make([]devopstypes.DynamicOption, 0, len(bindings))
	for _, binding := range bindings {
		channel, err := l.svcCtx.ChannelModel.FindOne(l.ctx, binding.ChannelID)
		if err != nil {
			l.Errorf("查询渠道绑定选项失败: %v", err)
			return nil, 0, err
		}
		if channel.Status != 1 {
			continue
		}
		if binding.ChannelType == "" {
			binding.ChannelType = channel.ChannelType
		}
		if binding.ChannelName == "" {
			binding.ChannelName = channel.Name
		}
		if binding.ChannelCode == "" {
			binding.ChannelCode = channel.Code
		}
		label := binding.ChannelName
		if binding.ChannelCode != "" {
			label = label + "（" + binding.ChannelCode + "）"
		}
		endpoint := strings.TrimSpace(channel.Endpoint)
		if keyword != "" &&
			!strings.Contains(strings.ToLower(label), keyword) &&
			!strings.Contains(strings.ToLower(endpoint), keyword) {
			continue
		}
		value := binding.ID.Hex()
		if binding.ChannelType == "sonarqube" {
			value = endpoint
		}
		options = append(options, devopstypes.DynamicOption{
			Label: label,
			Value: value,
			Metadata: map[string]any{
				"bindingId":        binding.ID.Hex(),
				"channelId":        binding.ChannelID,
				"channelType":      binding.ChannelType,
				"channelGroupCode": binding.ChannelGroupCode,
				"channelCode":      binding.ChannelCode,
				"endpoint":         endpoint,
			},
		})
	}
	if keyword != "" {
		total = uint64(len(options))
	}
	return options, total, nil
}

func (l *DynamicParamOptionsLogic) credentialOptions(in *pb.DynamicParamOptionsReq, page, pageSize uint64) ([]devopstypes.DynamicOption, uint64, error) {
	credentialType := strings.TrimSpace(in.ComponentValue)
	if credentialType == "" {
		l.Errorf("请选择凭证类型")
		return nil, 0, errorx.Msg("请选择凭证类型")
	}
	projectIDs, restricted, err := userProjectIDs(l.ctx, l.svcCtx, in.CurrentUserId, in.CurrentRoles)
	if err != nil {
		l.Errorf("查询凭证选项失败: %v", err)
		return nil, 0, err
	}
	items, total, err := l.svcCtx.CredentialModel.List(l.ctx, model.DevopsCredentialListFilter{
		Name:           in.Keyword,
		CredentialType: credentialType,
		Status:         1,
		ProjectID:      in.ProjectId,
		ProjectIDs:     projectIDs,
		Restricted:     restricted,
		Page:           page,
		PageSize:       pageSize,
	})
	if err != nil {
		l.Errorf("查询凭证选项失败: %v", err)
		return nil, 0, err
	}
	options := make([]devopstypes.DynamicOption, 0, len(items))
	for _, item := range items {
		label := strings.TrimSpace(item.Name)
		if item.Code != "" {
			label = label + "（" + item.Code + "）"
		}
		if label == "" {
			label = item.ID.Hex()
		}
		options = append(options, devopstypes.DynamicOption{
			Label: label,
			Value: item.ID.Hex(),
			Metadata: map[string]any{
				"credentialType": item.CredentialType,
				"scope":          item.Scope,
				"projectId":      item.ProjectID,
			},
		})
	}
	return options, total, nil
}

func repositoryProvider(channelType string) any {
	switch strings.TrimSpace(channelType) {
	case "gitlab":
		return devopsgitlab.New()
	case "github":
		return devopsgitlab.NewGitHub()
	case "gitee":
		return devopsgitlab.NewGitee()
	case "svn":
		return devopsgitlab.NewSVN()
	default:
		return nil
	}
}

func isRepositoryChannelType(channelType string) bool {
	switch strings.TrimSpace(channelType) {
	case "gitlab", "github", "gitee", "svn":
		return true
	default:
		return false
	}
}

func registryProjectOption(binding *model.DevopsProjectChannelBinding, channel *model.DevopsChannel) []devopstypes.DynamicOption {
	for _, raw := range []string{
		configStringValue(binding.BindingConfig, "namespace"),
		configStringValue(binding.BindingConfig, "project"),
		configStringValue(channel.Config, "namespace"),
		configStringValue(channel.Config, "project"),
		binding.ChannelCode,
		channel.Code,
	} {
		value := strings.TrimSpace(raw)
		if value == "" {
			continue
		}
		return []devopstypes.DynamicOption{{
			Label: value,
			Value: value,
			Metadata: map[string]any{
				"channelType": binding.ChannelType,
			},
		}}
	}
	return nil
}

func configStringValue(raw, key string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || key == "" {
		return ""
	}
	var data map[string]any
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		return ""
	}
	if value, ok := data[key].(string); ok {
		return value
	}
	return ""
}

func (l *DynamicParamOptionsLogic) dynamicProviderRequest(in *pb.DynamicParamOptionsReq) (devopstypes.Request, *model.DevopsProjectChannelBinding, *model.DevopsChannel, error) {
	bindingID := strings.TrimSpace(in.ChannelBindingId)
	if bindingID == "" {
		l.Errorf("请选择渠道绑定")
		return devopstypes.Request{}, nil, nil, errorx.Msg("请选择渠道绑定")
	}
	binding, err := l.svcCtx.ProjectChannelModel.FindOne(l.ctx, bindingID)
	if err != nil {
		l.Errorf("构造动态参数请求失败: %v", err)
		return devopstypes.Request{}, nil, nil, err
	}
	if binding.ProjectID != in.ProjectId {
		l.Errorf("渠道绑定不属于当前项目")
		return devopstypes.Request{}, nil, nil, errorx.Msg("渠道绑定不属于当前项目")
	}
	if binding.Status != 1 {
		l.Errorf("渠道绑定已停用")
		return devopstypes.Request{}, nil, nil, errorx.Msg("渠道绑定已停用")
	}
	channel, err := l.svcCtx.ChannelModel.FindOne(l.ctx, binding.ChannelID)
	if err != nil {
		l.Errorf("构造动态参数请求失败: %v", err)
		return devopstypes.Request{}, nil, nil, err
	}
	if channel.Status != 1 {
		l.Errorf("渠道已停用")
		return devopstypes.Request{}, nil, nil, errorx.Msg("渠道已停用")
	}
	req, err := l.dynamicRequestFromBinding(binding, channel)
	if err != nil {
		l.Errorf("构造动态参数请求失败: %v", err)
		return devopstypes.Request{}, nil, nil, err
	}
	return req, binding, channel, nil
}

func (l *DynamicParamOptionsLogic) dynamicRequestFromBinding(binding *model.DevopsProjectChannelBinding, channel *model.DevopsChannel) (devopstypes.Request, error) {
	credential, err := l.resolveDynamicCredential(binding, channel)
	if err != nil {
		l.Errorf("构造绑定动态参数请求失败: %v", err)
		return devopstypes.Request{}, err
	}
	return devopstypes.Request{
		Channel: devopstypes.Channel{
			ID:              channel.ID.Hex(),
			Name:            channel.Name,
			Code:            channel.Code,
			Type:            channel.ChannelType,
			Endpoint:        channel.Endpoint,
			Config:          channel.Config,
			AuthType:        channel.AuthType,
			Username:        channel.Username,
			Password:        channel.Password,
			Token:           channel.Token,
			InsecureSkipTLS: channel.InsecureSkipTLS,
		},
		Credential: credential,
	}, nil
}

func (l *DynamicParamOptionsLogic) gitRepositoryURLProviderRequest(in *pb.DynamicParamOptionsReq) (devopstypes.Request, *model.DevopsProjectChannelBinding, error) {
	repositoryURL := strings.TrimSpace(in.ProjectValue)
	if gitProjectPathFromRepositoryURL(repositoryURL) == "" {
		l.Errorf("Git 仓库地址不正确")
		return devopstypes.Request{}, nil, errorx.Msg("Git 仓库地址不正确")
	}
	bindings, _, err := l.svcCtx.ProjectChannelModel.List(l.ctx, model.DevopsProjectChannelBindingListFilter{
		ProjectID:        in.ProjectId,
		ChannelGroupCode: channelvars.GroupCodeRepo,
		Status:           1,
		Page:             1,
		PageSize:         200,
	})
	if err != nil {
		l.Errorf("构造 Git 仓库地址动态请求失败: %v", err)
		return devopstypes.Request{}, nil, err
	}
	var fallbackReq devopstypes.Request
	var fallbackBinding *model.DevopsProjectChannelBinding
	fallbackCount := 0
	for _, binding := range bindings {
		if !isRepositoryChannelType(binding.ChannelType) {
			continue
		}
		channel, err := l.svcCtx.ChannelModel.FindOne(l.ctx, binding.ChannelID)
		if err != nil {
			if errors.Is(err, model.ErrNotFound) {
				continue
			}
			l.Errorf("构造 Git 仓库地址动态请求失败: %v", err)
			return devopstypes.Request{}, nil, err
		}
		if channel.Status != 1 {
			continue
		}
		req, err := l.dynamicRequestFromBinding(binding, channel)
		if err != nil {
			l.Errorf("构造 Git 仓库地址动态请求失败: %v", err)
			return devopstypes.Request{}, nil, err
		}
		if repositoryURLMatchesEndpoint(repositoryURL, channel.Endpoint) {
			return req, binding, nil
		}
		fallbackReq = req
		fallbackBinding = binding
		fallbackCount++
	}
	if fallbackCount == 1 {
		return fallbackReq, fallbackBinding, nil
	}
	if fallbackCount > 1 {
		l.Errorf("Git 仓库地址无法匹配唯一渠道实例")
		return devopstypes.Request{}, nil, errorx.Msg("Git 仓库地址无法匹配唯一渠道实例")
	}
	l.Errorf("当前项目未绑定代码仓库渠道")
	return devopstypes.Request{}, nil, errorx.Msg("当前项目未绑定代码仓库渠道")
}

func (l *DynamicParamOptionsLogic) imageRegistryAddressProviderRequest(in *pb.DynamicParamOptionsReq, dependencyValues map[string]string) (devopstypes.Request, *model.DevopsProjectChannelBinding, error) {
	source := imageRegistryAddressFromValues(in.ProjectValue, dependencyValues)
	parts := parseImageRegistryAddress(source)
	if parts.Host == "" || parts.Project == "" {
		l.Errorf("镜像地址不正确")
		return devopstypes.Request{}, nil, errorx.Msg("镜像地址不正确")
	}
	bindings, _, err := l.svcCtx.ProjectChannelModel.List(l.ctx, model.DevopsProjectChannelBindingListFilter{
		ProjectID:        in.ProjectId,
		ChannelGroupCode: channelvars.GroupImageRepo,
		Status:           1,
		Page:             1,
		PageSize:         200,
	})
	if err != nil {
		l.Errorf("构造镜像地址动态请求失败: %v", err)
		return devopstypes.Request{}, nil, err
	}
	var fallbackReq devopstypes.Request
	var fallbackBinding *model.DevopsProjectChannelBinding
	fallbackCount := 0
	for _, binding := range bindings {
		if binding == nil || !isImageRegistryChannelType(binding.ChannelType) {
			continue
		}
		channel, err := l.svcCtx.ChannelModel.FindOne(l.ctx, binding.ChannelID)
		if err != nil {
			if errors.Is(err, model.ErrNotFound) {
				continue
			}
			l.Errorf("构造镜像地址动态请求失败: %v", err)
			return devopstypes.Request{}, nil, err
		}
		if channel.Status != 1 {
			continue
		}
		req, err := l.dynamicRequestFromBinding(binding, channel)
		if err != nil {
			l.Errorf("构造镜像地址动态请求失败: %v", err)
			return devopstypes.Request{}, nil, err
		}
		if imageRegistryURLMatchesEndpoint(source, channel.Endpoint) {
			matchedParts := parseImageRegistryAddressWithEndpoint(source, channel.Endpoint)
			if matchedParts.Project == "" {
				matchedParts = parts
			}
			applyImageRegistryAddressParts(in, matchedParts)
			return req, binding, nil
		}
		fallbackReq = req
		fallbackBinding = binding
		fallbackCount++
	}
	if fallbackCount == 1 {
		applyImageRegistryAddressParts(in, parts)
		return fallbackReq, fallbackBinding, nil
	}
	if fallbackCount > 1 {
		l.Errorf("镜像地址无法匹配唯一渠道实例")
		return devopstypes.Request{}, nil, errorx.Msg("镜像地址无法匹配唯一渠道实例")
	}
	l.Errorf("当前项目未绑定镜像仓库渠道")
	return devopstypes.Request{}, nil, errorx.Msg("当前项目未绑定镜像仓库渠道")
}

func (l *DynamicParamOptionsLogic) artifactRepositoryAddressProviderRequest(in *pb.DynamicParamOptionsReq, dependencyValues map[string]string) (devopstypes.Request, *model.DevopsProjectChannelBinding, error) {
	source := artifactRepositoryAddressFromValues(in.ProjectValue, dependencyValues)
	parts := parseArtifactRepositoryAddress(source)
	if parts.Host == "" || parts.Repository == "" {
		l.Errorf("制品地址不正确")
		return devopstypes.Request{}, nil, errorx.Msg("制品地址不正确")
	}
	bindings, _, err := l.svcCtx.ProjectChannelModel.List(l.ctx, model.DevopsProjectChannelBindingListFilter{
		ProjectID:        in.ProjectId,
		ChannelGroupCode: channelvars.GroupArtifactRepo,
		Status:           1,
		Page:             1,
		PageSize:         200,
	})
	if err != nil {
		l.Errorf("构造制品地址动态请求失败: %v", err)
		return devopstypes.Request{}, nil, err
	}
	var fallbackReq devopstypes.Request
	var fallbackBinding *model.DevopsProjectChannelBinding
	fallbackCount := 0
	for _, binding := range bindings {
		if binding == nil || !isArtifactRepositoryChannelType(binding.ChannelType) {
			continue
		}
		channel, err := l.svcCtx.ChannelModel.FindOne(l.ctx, binding.ChannelID)
		if err != nil {
			if errors.Is(err, model.ErrNotFound) {
				continue
			}
			l.Errorf("构造制品地址动态请求失败: %v", err)
			return devopstypes.Request{}, nil, err
		}
		if channel.Status != 1 {
			continue
		}
		req, err := l.dynamicRequestFromBinding(binding, channel)
		if err != nil {
			l.Errorf("构造制品地址动态请求失败: %v", err)
			return devopstypes.Request{}, nil, err
		}
		if repositoryURLMatchesEndpoint(source, channel.Endpoint) {
			matchedParts := parseArtifactRepositoryAddressWithEndpoint(source, channel.Endpoint)
			if matchedParts.Repository == "" {
				matchedParts = parts
			}
			applyArtifactRepositoryAddressParts(in, matchedParts)
			return req, binding, nil
		}
		fallbackReq = req
		fallbackBinding = binding
		fallbackCount++
	}
	if fallbackCount == 1 {
		applyArtifactRepositoryAddressParts(in, parts)
		return fallbackReq, fallbackBinding, nil
	}
	if fallbackCount > 1 {
		l.Errorf("制品地址无法匹配唯一渠道实例")
		return devopstypes.Request{}, nil, errorx.Msg("制品地址无法匹配唯一渠道实例")
	}
	l.Errorf("当前项目未绑定制品仓库渠道")
	return devopstypes.Request{}, nil, errorx.Msg("当前项目未绑定制品仓库渠道")
}

func applyImageRegistryAddressParts(in *pb.DynamicParamOptionsReq, parts imageRegistryAddressParts) {
	if strings.TrimSpace(in.ProjectValue) == "" || looksLikeImageRegistryAddress(in.ProjectValue) {
		in.ProjectValue = parts.Project
	}
	if strings.TrimSpace(in.ComponentValue) == "" && strings.TrimSpace(parts.Image) != "" {
		in.ComponentValue = parts.Image
	}
	if strings.TrimSpace(in.ConfigTypeCode) == "" && strings.TrimSpace(parts.Tag) != "" {
		in.ConfigTypeCode = parts.Tag
	}
}

func applyArtifactRepositoryAddressParts(in *pb.DynamicParamOptionsReq, parts artifactRepositoryAddressParts) {
	if strings.TrimSpace(in.ProjectValue) == "" || looksLikeArtifactRepositoryAddress(in.ProjectValue) {
		in.ProjectValue = parts.Repository
	}
	if strings.TrimSpace(in.ComponentValue) == "" && strings.TrimSpace(parts.Artifact) != "" {
		in.ComponentValue = parts.Artifact
	}
	if strings.TrimSpace(in.ConfigTypeCode) == "" && strings.TrimSpace(parts.Version) != "" {
		in.ConfigTypeCode = parts.Version
	}
}

func repositoryURLMatchesEndpoint(repositoryURL, endpoint string) bool {
	repoHost, repoPath := gitRepositoryHostAndPath(repositoryURL)
	if repoHost == "" {
		return false
	}
	endpointHost, endpointPath := endpointHostAndPath(endpoint)
	if endpointHost == "" {
		return false
	}
	if !addressHostMatches(repoHost, endpointHost) {
		return false
	}
	if endpointPath == "" {
		return true
	}
	return strings.HasPrefix(repoPath, endpointPath+"/") || repoPath == endpointPath
}

func imageRegistryURLMatchesEndpoint(imageURL, endpoint string) bool {
	sourceHost, sourcePath := imageRegistryHostAndPath(imageURL)
	if sourceHost == "" {
		return false
	}
	endpointHost, endpointPath := endpointHostAndPath(endpoint)
	if endpointHost == "" || !addressHostMatches(sourceHost, endpointHost) {
		return false
	}
	if endpointPath == "" {
		return true
	}
	return sourcePath == endpointPath || strings.HasPrefix(sourcePath, endpointPath+"/")
}

func imageRegistryHostAndPath(value string) (string, string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", ""
	}
	if !strings.Contains(value, "://") {
		value = "https://" + value
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" {
		return "", ""
	}
	path := strings.Trim(parsed.EscapedPath(), "/")
	if decoded, err := url.PathUnescape(path); err == nil {
		path = decoded
	}
	return parsed.Host, strings.Trim(path, "/")
}

func addressHostMatches(sourceHost, endpointHost string) bool {
	sourceName, sourcePort := splitAddressHost(sourceHost)
	endpointName, endpointPort := splitAddressHost(endpointHost)
	if sourceName == "" || endpointName == "" {
		return false
	}
	if !strings.EqualFold(sourceName, endpointName) {
		return false
	}
	return sourcePort == "" || endpointPort == "" || sourcePort == endpointPort
}

func splitAddressHost(host string) (string, string) {
	host = strings.Trim(strings.TrimSpace(host), "/")
	if host == "" {
		return "", ""
	}
	if parsed, err := url.Parse("https://" + host); err == nil && parsed.Host != "" {
		if parsed.Port() != "" {
			return strings.Trim(parsed.Hostname(), "[]"), parsed.Port()
		}
	}
	if name, port, err := net.SplitHostPort(host); err == nil {
		return strings.Trim(name, "[]"), port
	}
	return strings.Trim(host, "[]"), ""
}

func endpointHostAndPath(endpoint string) (string, string) {
	endpoint = strings.TrimRight(strings.TrimSpace(endpoint), "/")
	if endpoint == "" {
		return "", ""
	}
	if !strings.Contains(endpoint, "://") && !strings.Contains(endpoint, "@") {
		endpoint = "https://" + endpoint
	}
	return gitRepositoryHostAndPath(endpoint)
}

func (l *DynamicParamOptionsLogic) resolveDynamicCredential(binding *model.DevopsProjectChannelBinding, channel *model.DevopsChannel) (*devopstypes.Credential, error) {
	credentialID := strings.TrimSpace(binding.ProjectCredentialID)
	if credentialID == "" && binding.AllowUseGlobalCredential {
		credentialID = strings.TrimSpace(channel.CredentialID)
		if credentialID == "" {
			credentialID = strings.TrimSpace(channel.GlobalCredentialID)
		}
	}
	if credentialID == "" {
		return nil, nil
	}
	credential, err := l.svcCtx.CredentialModel.FindOne(l.ctx, credentialID)
	if err != nil {
		l.Errorf("解析动态凭证失败: %v", err)
		return nil, err
	}
	if credential.Status != 1 {
		l.Errorf("渠道凭据已停用")
		return nil, errorx.Msg("渠道凭据已停用")
	}
	if binding.ProjectCredentialID != "" && (credential.ProjectID != binding.ProjectID || credential.Scope != "project" || credential.IsSystem) {
		l.Errorf("项目渠道凭据必须属于当前项目")
		return nil, errorx.Msg("项目渠道凭据必须属于当前项目")
	}
	secret, err := credential.Secret()
	if err != nil {
		l.Errorf("解析动态凭证失败: %v", err)
		return nil, err
	}
	return &devopstypes.Credential{
		Type:        credential.CredentialType,
		Username:    secret.Username,
		Password:    secret.Password,
		Token:       secret.Token,
		PrivateKey:  secret.PrivateKey,
		Passphrase:  secret.Passphrase,
		Kubeconfig:  secret.Kubeconfig,
		SecretText:  secret.SecretText,
		Certificate: secret.Certificate,
		JsonData:    secret.JsonData,
	}, nil
}

func dynamicOptionsToPb(items []devopstypes.DynamicOption) []*pb.DynamicParamOption {
	result := make([]*pb.DynamicParamOption, 0, len(items))
	for _, item := range items {
		metadata := ""
		if len(item.Metadata) > 0 {
			data, _ := json.Marshal(item.Metadata)
			metadata = string(data)
		}
		result = append(result, &pb.DynamicParamOption{Label: item.Label, Value: item.Value, Metadata: metadata})
	}
	return result
}
