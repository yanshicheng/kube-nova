package model

import (
	"context"
	"errors"
	"strings"

	"github.com/yanshicheng/kube-nova/common/devops/channelvars"
)

type defaultChannelGroupSeed struct {
	Name                string
	Code                string
	Description         string
	SortOrder           int64
	GroupType           string
	AllowedChannelTypes []string
	Icon                string
	IconColor           string
}

type defaultChannelTypeSeed struct {
	Code             string
	Name             string
	GroupCode        string
	CredentialTypes  []string
	ConfigSchema     string
	MappingFields    string
	TestStrategy     string
	Icon             string
	IconColor        string
	ConnectionMode   string
	MetadataStrategy string
}

type defaultStepChannelParamSeed struct {
	ParamType         string
	ParamName         string
	GroupCode         string
	ChannelTypeFilter string
	Description       string
	SortOrder         int64
}

const (
	BuildChannelGroupCode = "build_engine"
	BuildUsageScope       = "build"

	DefaultDataBootstrapCode        = "devops_default_data"
	DefaultDataBootstrapVersion     = "2026060101"
	DefaultDataBootstrapDescription = "DevOps 默认数据初始化"
)

var defaultChannelGroups = []defaultChannelGroupSeed{
	{Name: "构建渠道", Code: BuildChannelGroupCode, Description: "系统内置构建执行渠道分组", SortOrder: 10, GroupType: BuildChannelGroupCode, AllowedChannelTypes: []string{"jenkins", "tekton"}, Icon: "ri:flow-chart", IconColor: "#2563eb"},
	{Name: "代码仓库渠道", Code: "code_repo", Description: "系统内置代码仓库渠道分组", SortOrder: 20, GroupType: "code_repo", AllowedChannelTypes: []string{"gitlab", "svn", "gitee", "github"}, Icon: "ri:git-repository-line", IconColor: "#f97316"},
	{Name: "制品渠道", Code: "artifact_repo", Description: "系统内置制品渠道分组", SortOrder: 30, GroupType: "artifact_repo", AllowedChannelTypes: []string{"nexus", "jfrog"}, Icon: "ri:archive-stack-line", IconColor: "#16a34a"},
	{Name: "镜像仓库渠道", Code: "image_registry", Description: "系统内置镜像仓库渠道分组", SortOrder: 40, GroupType: "image_registry", AllowedChannelTypes: []string{"harbor", "registry", "aliyun_registry"}, Icon: "ri:box-3-line", IconColor: "#0891b2"},
	{Name: "代码扫描渠道", Code: "code_scan", Description: "系统内置代码扫描渠道分组", SortOrder: 50, GroupType: "code_scan", AllowedChannelTypes: []string{"sonarqube", "spotbugs"}, Icon: "ri:scan-line", IconColor: "#4b9fd5"},
	{Name: "镜像安全渠道", Code: "image_security", Description: "系统内置镜像安全渠道分组", SortOrder: 60, GroupType: "image_security", AllowedChannelTypes: []string{"trivy", "kube_bench"}, Icon: "ri:shield-check-line", IconColor: "#0f766e"},
	{Name: "部署渠道", Code: "deploy_target", Description: "系统内置部署渠道分组", SortOrder: 70, GroupType: "deploy_target", AllowedChannelTypes: []string{"kubernetes", "host", "host_group", "kube-nova", "argocd"}, Icon: "ri:rocket-line", IconColor: "#7c3aed"},
	{Name: "工具渠道", Code: "tool", Description: "系统内置工具类渠道分组", SortOrder: 80, GroupType: "tool", AllowedChannelTypes: []string{"buildkit"}, Icon: "ri:tools-line", IconColor: "#64748b"},
	{Name: "自定义渠道", Code: "custom", Description: "管理员维护的自定义渠道分组", SortOrder: 90, GroupType: "custom", Icon: "ri:apps-2-line", IconColor: "#64748b"},
}

var defaultChannelTypes = []defaultChannelTypeSeed{
	{Code: "jenkins", Name: "Jenkins", GroupCode: BuildChannelGroupCode, CredentialTypes: defaultChannelCredentialTypes(), ConfigSchema: schema(`{"key":"jobName","label":"默认任务","type":"input"},{"key":"crumbIssuer","label":"启用 Jenkins Crumb","type":"switch"}`), MappingFields: standardVariableSchema("build_engine", "jenkins", configVariable("config.jobName", "默认任务")), TestStrategy: "http", Icon: "simple-icons:jenkins", IconColor: "#d24939", ConnectionMode: "server", MetadataStrategy: "product_version"},
	{Code: "tekton", Name: "Tekton", GroupCode: BuildChannelGroupCode, CredentialTypes: tektonChannelCredentialTypes(), ConfigSchema: schema(`{"key":"taskNamespace","label":"Task 命名空间","type":"input","required":true,"defaultValue":"tekton"}`), MappingFields: standardVariableSchema("build_engine", "tekton", configVariable("config.taskNamespace", "Task 命名空间")), TestStrategy: "kubernetes", Icon: "ri:flow-chart", IconColor: "#2563eb", ConnectionMode: "kubernetes", MetadataStrategy: "api_capability"},
	{Code: "buildkit", Name: "BuildKit", GroupCode: "tool", CredentialTypes: defaultChannelCredentialTypes("certificate", "json"), ConfigSchema: schema(`{"key":"driver","label":"驱动","type":"input","defaultValue":"remote"}`), MappingFields: variableSchema(configVariable("config.driver", "驱动")), TestStrategy: "tcp", Icon: "simple-icons:docker", IconColor: "#0369a1", ConnectionMode: "server", MetadataStrategy: "engine_version"},
	{Code: "gitlab", Name: "GitLab", GroupCode: "code_repo", CredentialTypes: defaultChannelCredentialTypes("ssh_key", "json"), ConfigSchema: schema(`{"key":"apiPath","label":"API 路径","type":"input","defaultValue":"/api/v4"},{"key":"project","label":"默认项目","type":"input"},{"key":"branch","label":"默认分支","type":"input"},{"key":"tag","label":"默认 Tag","type":"input"}`), MappingFields: standardVariableSchema("code_repo", "gitlab", configVariable("config.apiPath", "API 路径"), configVariable("config.project", "默认项目"), configVariable("config.branch", "默认分支"), configVariable("config.tag", "默认 Tag")), TestStrategy: "http", Icon: "simple-icons:gitlab", IconColor: "#fc6d26", ConnectionMode: "server", MetadataStrategy: "product_version"},
	{Code: "svn", Name: "SVN", GroupCode: "code_repo", CredentialTypes: defaultChannelCredentialTypes("ssh_key", "json"), ConfigSchema: schema(`{"key":"repositoryRoot","label":"仓库根路径","type":"input"},{"key":"project","label":"默认项目","type":"input"},{"key":"branch","label":"默认分支","type":"input"},{"key":"tag","label":"默认 Tag","type":"input"}`), MappingFields: standardVariableSchema("code_repo", "svn", configVariable("config.repositoryRoot", "仓库根路径"), configVariable("config.project", "默认项目"), configVariable("config.branch", "默认分支"), configVariable("config.tag", "默认 Tag")), TestStrategy: "tcp", Icon: "ri:git-repository-line", IconColor: "#64748b", ConnectionMode: "protocol", MetadataStrategy: "capability"},
	{Code: "gitee", Name: "Gitee", GroupCode: "code_repo", CredentialTypes: defaultChannelCredentialTypes("ssh_key", "json"), ConfigSchema: schema(`{"key":"apiPath","label":"API 路径","type":"input","defaultValue":"/api/v5"},{"key":"owner","label":"默认空间","type":"input"},{"key":"project","label":"默认项目","type":"input"},{"key":"branch","label":"默认分支","type":"input"},{"key":"tag","label":"默认 Tag","type":"input"}`), MappingFields: standardVariableSchema("code_repo", "gitee", configVariable("config.apiPath", "API 路径"), configVariable("config.owner", "默认空间"), configVariable("config.project", "默认项目"), configVariable("config.branch", "默认分支"), configVariable("config.tag", "默认 Tag")), TestStrategy: "http", Icon: "simple-icons:gitee", IconColor: "#c71d23", ConnectionMode: "api", MetadataStrategy: "api_capability"},
	{Code: "github", Name: "GitHub", GroupCode: "code_repo", CredentialTypes: defaultChannelCredentialTypes("ssh_key", "json"), ConfigSchema: schema(`{"key":"apiPath","label":"API 路径","type":"input","defaultValue":"/api/v3"},{"key":"owner","label":"默认组织","type":"input"},{"key":"project","label":"默认仓库","type":"input"},{"key":"branch","label":"默认分支","type":"input"},{"key":"tag","label":"默认 Tag" ,"type":"input"}`), MappingFields: standardVariableSchema("code_repo", "github", configVariable("config.apiPath", "API 路径"), configVariable("config.owner", "默认组织"), configVariable("config.project", "默认仓库"), configVariable("config.branch", "默认分支"), configVariable("config.tag", "默认 Tag")), TestStrategy: "http", Icon: "simple-icons:github", IconColor: "#24292f", ConnectionMode: "api", MetadataStrategy: "api_capability"},
	{Code: "nexus", Name: "Nexus", GroupCode: "artifact_repo", CredentialTypes: defaultChannelCredentialTypes(), ConfigSchema: schema(`{"key":"repository","label":"默认仓库","type":"input"}`), MappingFields: standardVariableSchema("artifact_repo", "nexus", configVariable("config.repository", "默认仓库")), TestStrategy: "http", Icon: "ri:archive-stack-line", IconColor: "#16a34a", ConnectionMode: "server", MetadataStrategy: "product_version"},
	{Code: "jfrog", Name: "JFrog", GroupCode: "artifact_repo", CredentialTypes: defaultChannelCredentialTypes(), ConfigSchema: schema(`{"key":"repository","label":"默认仓库","type":"input"}`), MappingFields: standardVariableSchema("artifact_repo", "jfrog", configVariable("config.repository", "默认仓库")), TestStrategy: "http", Icon: "simple-icons:jfrog", IconColor: "#0f9f6e", ConnectionMode: "server", MetadataStrategy: "product_version"},
	{Code: "harbor", Name: "Harbor", GroupCode: "image_registry", CredentialTypes: defaultChannelCredentialTypes("certificate", "json"), ConfigSchema: schema(`{"key":"project","label":"默认项目","type":"input"},{"key":"repository","label":"默认仓库","type":"input"},{"key":"image","label":"默认镜像","type":"input"}`), MappingFields: standardVariableSchema("image_registry", "harbor", configVariable("config.project", "默认项目"), configVariable("config.repository", "默认仓库"), configVariable("config.image", "默认镜像")), TestStrategy: "http", Icon: "simple-icons:harbor", IconColor: "#0f766e", ConnectionMode: "server", MetadataStrategy: "product_version"},
	{Code: "registry", Name: "Docker Registry", GroupCode: "image_registry", CredentialTypes: defaultChannelCredentialTypes("certificate", "json"), ConfigSchema: schema(`{"key":"namespace","label":"命名空间","type":"input"}`), MappingFields: standardVariableSchema("image_registry", "registry", configVariable("config.namespace", "命名空间")), TestStrategy: "http", Icon: "simple-icons:docker", IconColor: "#0369a1", ConnectionMode: "api", MetadataStrategy: "api_capability"},
	{Code: "aliyun_registry", Name: "阿里云镜像仓库", GroupCode: "image_registry", CredentialTypes: defaultChannelCredentialTypes("json"), ConfigSchema: schema(`{"key":"namespace","label":"命名空间","type":"input"},{"key":"region","label":"地域","type":"input"}`), MappingFields: standardVariableSchema("image_registry", "aliyun_registry", configVariable("config.namespace", "命名空间"), configVariable("config.region", "地域")), TestStrategy: "http", Icon: "ri:cloud-line", IconColor: "#ff6a00", ConnectionMode: "api", MetadataStrategy: "api_capability"},
	{Code: "sonarqube", Name: "SonarQube", GroupCode: "code_scan", CredentialTypes: defaultChannelCredentialTypes(), ConfigSchema: schema(`{"key":"projectKey","label":"默认项目 Key","type":"input"}`), MappingFields: standardVariableSchema("code_scan", "sonarqube", configVariable("config.projectKey", "默认项目 Key")), TestStrategy: "http", Icon: "simple-icons:sonarqube", IconColor: "#4b9fd5", ConnectionMode: "server", MetadataStrategy: "product_version"},
	{Code: "spotbugs", Name: "SpotBugs", GroupCode: "code_scan", CredentialTypes: defaultChannelCredentialTypes("secret_text", "json"), ConfigSchema: schema(`{"key":"profile","label":"扫描配置","type":"input"}`), MappingFields: variableSchema(configVariable("config.profile", "扫描配置")), TestStrategy: "none", Icon: "ri:bug-line", IconColor: "#b45309", ConnectionMode: "tool", MetadataStrategy: "engine_version"},
	{Code: "trivy", Name: "Trivy", GroupCode: "image_security", CredentialTypes: defaultChannelCredentialTypes("secret_text", "json"), ConfigSchema: schema(`{"key":"severity","label":"严重等级","type":"input","defaultValue":"HIGH,CRITICAL"},{"key":"engineVersion","label":"引擎版本","type":"input"},{"key":"dbVersion","label":"漏洞库版本","type":"input"}`), MappingFields: variableSchema(configVariable("config.severity", "严重等级"), configVariable("config.engineVersion", "引擎版本"), configVariable("config.dbVersion", "漏洞库版本")), TestStrategy: "tool", Icon: "ri:shield-check-line", IconColor: "#0f766e", ConnectionMode: "tool", MetadataStrategy: "engine_version"},
	{Code: "kube_bench", Name: "kube-bench", GroupCode: "image_security", CredentialTypes: defaultChannelCredentialTypes("kubeconfig", "json"), ConfigSchema: schema(`{"key":"profile","label":"检测基线","type":"input","defaultValue":"cis"},{"key":"engineVersion","label":"引擎版本","type":"input"}`), MappingFields: variableSchema(configVariable("config.profile", "检测基线"), configVariable("config.engineVersion", "引擎版本")), TestStrategy: "tool", Icon: "ri:shield-keyhole-line", IconColor: "#7c3aed", ConnectionMode: "tool", MetadataStrategy: "benchmark"},
	{Code: "kubernetes", Name: "kubernetes", GroupCode: "deploy_target", CredentialTypes: defaultChannelCredentialTypes("kubeconfig", "certificate", "json"), ConfigSchema: schema(`{"key":"namespace","label":"默认命名空间","type":"input"}`), MappingFields: standardVariableSchema("deploy_target", "kubernetes", configVariable("config.namespace", "默认命名空间"), configVariable("config.cluster", "集群")), TestStrategy: "kubernetes", Icon: "simple-icons:kubernetes", IconColor: "#326ce5", ConnectionMode: "server", MetadataStrategy: "product_version"},
	{Code: "host", Name: "主机", GroupCode: "deploy_target", CredentialTypes: defaultChannelCredentialTypes("ssh_key", "json"), ConfigSchema: schema(`{"key":"hostId","label":"主机资产","type":"host_select","required":true}`), MappingFields: standardVariableSchema("deploy_target", "host"), TestStrategy: "host", Icon: "ri:server-line", IconColor: "#475569", ConnectionMode: "host", MetadataStrategy: "capability"},
	{Code: "host_group", Name: "主机组", GroupCode: "deploy_target", CredentialTypes: defaultChannelCredentialTypes("ssh_key", "json"), ConfigSchema: schema(`{"key":"hostIds","label":"主机资产","type":"host_multi","required":true}`), MappingFields: standardVariableSchema("deploy_target", "host_group"), TestStrategy: "host_group", Icon: "ri:server-line", IconColor: "#7c3aed", ConnectionMode: "host_group", MetadataStrategy: "capability"},
	{Code: "kube-nova", Name: "kube-nova", GroupCode: "deploy_target", CredentialTypes: defaultChannelCredentialTypes("secret_text", "json"), ConfigSchema: schema(`{"key":"workspace","label":"工作空间","type":"input"}`), MappingFields: standardVariableSchema("deploy_target", "kube-nova", configVariable("config.workspace", "工作空间")), TestStrategy: "http", Icon: "ri:cloud-line", IconColor: "#0891b2", ConnectionMode: "server", MetadataStrategy: "product_version"},
	{Code: "argocd", Name: "Argo CD", GroupCode: "deploy_target", CredentialTypes: defaultChannelCredentialTypes("json"), ConfigSchema: schema(`{"key":"project","label":"默认项目","type":"input"},{"key":"appPrefix","label":"应用前缀","type":"input"}`), MappingFields: variableSchema(configVariable("config.project", "默认项目"), configVariable("config.appPrefix", "应用前缀"), dynamicVariable("dynamic.project", "项目", "argocd.projects", depChannel()), dynamicVariable("dynamic.application", "应用", "argocd.applications", depChannel(), depParam("dynamic.project", "项目", true)), addressVariable(channelvars.FieldAddressProjectURL, "项目地址", "argocd.projectUrl", "endpoint + project", depChannel())), TestStrategy: "http", Icon: "simple-icons:argo", IconColor: "#ef7b4d", ConnectionMode: "server", MetadataStrategy: "product_version"},
}

// defaultStepChannelParams 步骤参数类型到渠道分组的映射，替代 channelvars 硬编码。
var defaultStepChannelParams = []defaultStepChannelParamSeed{
	{ParamType: "repositoryChannel", ParamName: "代码仓库渠道", GroupCode: "code_repo", Description: "步骤参数映射到代码仓库渠道分组", SortOrder: 10},
	{ParamType: "gitProject", ParamName: "Git 项目", GroupCode: "code_repo", ChannelTypeFilter: "gitlab", Description: "步骤参数映射到代码仓库项目字段", SortOrder: 11},
	{ParamType: "gitRepositoryUrl", ParamName: "Git 仓库地址", GroupCode: "code_repo", ChannelTypeFilter: "gitlab", Description: "步骤参数映射到代码仓库地址字段", SortOrder: 12},
	{ParamType: "gitBranch", ParamName: "Git 分支", GroupCode: "code_repo", ChannelTypeFilter: "gitlab", Description: "步骤参数映射到代码仓库分支字段", SortOrder: 13},
	{ParamType: "gitTag", ParamName: "Git Tag", GroupCode: "code_repo", ChannelTypeFilter: "gitlab", Description: "步骤参数映射到代码仓库 Tag 字段", SortOrder: 14},
	{ParamType: "gitProject", ParamName: "Gitee 项目", GroupCode: "code_repo", ChannelTypeFilter: "gitee", Description: "步骤参数映射到 Gitee 项目字段", SortOrder: 15},
	{ParamType: "gitRepositoryUrl", ParamName: "Gitee 仓库地址", GroupCode: "code_repo", ChannelTypeFilter: "gitee", Description: "步骤参数映射到 Gitee 仓库地址字段", SortOrder: 16},
	{ParamType: "gitBranch", ParamName: "Gitee 分支", GroupCode: "code_repo", ChannelTypeFilter: "gitee", Description: "步骤参数映射到 Gitee 分支字段", SortOrder: 17},
	{ParamType: "gitTag", ParamName: "Gitee Tag", GroupCode: "code_repo", ChannelTypeFilter: "gitee", Description: "步骤参数映射到 Gitee Tag 字段", SortOrder: 18},
	{ParamType: "gitProject", ParamName: "GitHub 仓库", GroupCode: "code_repo", ChannelTypeFilter: "github", Description: "步骤参数映射到 GitHub 仓库字段", SortOrder: 19},
	{ParamType: "gitRepositoryUrl", ParamName: "GitHub 仓库地址", GroupCode: "code_repo", ChannelTypeFilter: "github", Description: "步骤参数映射到 GitHub 仓库地址字段", SortOrder: 20},
	{ParamType: "gitBranch", ParamName: "GitHub 分支", GroupCode: "code_repo", ChannelTypeFilter: "github", Description: "步骤参数映射到 GitHub 分支字段", SortOrder: 21},
	{ParamType: "gitTag", ParamName: "GitHub Tag", GroupCode: "code_repo", ChannelTypeFilter: "github", Description: "步骤参数映射到 GitHub Tag 字段", SortOrder: 22},
	{ParamType: "gitProject", ParamName: "SVN 项目", GroupCode: "code_repo", ChannelTypeFilter: "svn", Description: "步骤参数映射到 SVN 项目字段", SortOrder: 23},
	{ParamType: "gitRepositoryUrl", ParamName: "SVN 项目地址", GroupCode: "code_repo", ChannelTypeFilter: "svn", Description: "步骤参数映射到 SVN 项目地址字段", SortOrder: 24},
	{ParamType: "gitBranch", ParamName: "SVN 分支", GroupCode: "code_repo", ChannelTypeFilter: "svn", Description: "步骤参数映射到 SVN 分支字段", SortOrder: 25},
	{ParamType: "gitTag", ParamName: "SVN Tag", GroupCode: "code_repo", ChannelTypeFilter: "svn", Description: "步骤参数映射到 SVN Tag 字段", SortOrder: 26},
	{ParamType: "harborChannel", ParamName: "Harbor 镜像仓库渠道", GroupCode: "image_registry", ChannelTypeFilter: "harbor", Description: "步骤参数映射到镜像仓库渠道分组", SortOrder: 20},
	{ParamType: "harborProject", ParamName: "Harbor 项目", GroupCode: "image_registry", ChannelTypeFilter: "harbor", Description: "步骤参数映射到 Harbor 项目字段", SortOrder: 21},
	{ParamType: "harborImage", ParamName: "Harbor 镜像", GroupCode: "image_registry", ChannelTypeFilter: "harbor", Description: "步骤参数映射到 Harbor 镜像字段", SortOrder: 22},
	{ParamType: "harborProjectUrl", ParamName: "Harbor 项目地址", GroupCode: "image_registry", ChannelTypeFilter: "harbor", Description: "步骤参数映射到 Harbor 项目地址字段", SortOrder: 23},
	{ParamType: "harborImageUrl", ParamName: "Harbor 镜像地址", GroupCode: "image_registry", ChannelTypeFilter: "harbor", Description: "步骤参数映射到 Harbor 镜像地址字段", SortOrder: 24},
	{ParamType: "harborImageTagUrl", ParamName: "Harbor 镜像地址带 Tag", GroupCode: "image_registry", ChannelTypeFilter: "harbor", Description: "步骤参数映射到 Harbor 镜像地址带 Tag 字段", SortOrder: 25},
	{ParamType: "nexusChannel", ParamName: "Nexus 制品渠道", GroupCode: "artifact_repo", ChannelTypeFilter: "nexus", Description: "步骤参数映射到制品渠道分组", SortOrder: 30},
	{ParamType: "nexusRepository", ParamName: "Nexus 仓库", GroupCode: "artifact_repo", ChannelTypeFilter: "nexus", Description: "步骤参数映射到 Nexus 仓库字段", SortOrder: 31},
	{ParamType: "nexusRepositoryUrl", ParamName: "Nexus 仓库地址", GroupCode: "artifact_repo", ChannelTypeFilter: "nexus", Description: "步骤参数映射到 Nexus 仓库地址字段", SortOrder: 32},
	{ParamType: "jfrogChannel", ParamName: "JFrog 制品渠道", GroupCode: "artifact_repo", ChannelTypeFilter: "jfrog", Description: "步骤参数映射到制品渠道分组", SortOrder: 40},
	{ParamType: "jfrogRepository", ParamName: "JFrog 仓库", GroupCode: "artifact_repo", ChannelTypeFilter: "jfrog", Description: "步骤参数映射到 JFrog 仓库字段", SortOrder: 41},
	{ParamType: "jfrogRepositoryUrl", ParamName: "JFrog 仓库地址", GroupCode: "artifact_repo", ChannelTypeFilter: "jfrog", Description: "步骤参数映射到 JFrog 仓库地址字段", SortOrder: 42},
	{ParamType: "kubeNovaDeployConfig", ParamName: "kube-nova 部署渠道", GroupCode: "deploy_target", ChannelTypeFilter: "kube-nova", Description: "步骤参数映射到部署渠道分组", SortOrder: 50},
	{ParamType: "kubernetesDeployConfig", ParamName: "Kubernetes 部署渠道", GroupCode: "deploy_target", ChannelTypeFilter: "kubernetes", Description: "步骤参数映射到部署渠道分组", SortOrder: 60},
	{ParamType: "sonarAddress", ParamName: "Sonar 扫描渠道", GroupCode: "code_scan", ChannelTypeFilter: "sonarqube", Description: "步骤参数映射到代码扫描渠道分组", SortOrder: 70},
	{ParamType: "hostGroupTargets", ParamName: "主机组部署渠道", GroupCode: "deploy_target", ChannelTypeFilter: "host_group", Description: "步骤参数映射到部署渠道分组", SortOrder: 80},
}

func EnsureDefaultChannels(ctx context.Context, groupModel *DevopsChannelGroupModel, typeModel *DevopsChannelTypeModel, channelModel *DevopsChannelModel, bindingModel *DevopsProjectChannelBindingModel, stepParamModel *DevopsStepChannelParamModel) error {
	for _, item := range defaultChannelGroups {
		if _, err := ensureDefaultChannelGroup(ctx, groupModel, item); err != nil {
			return err
		}
	}

	for _, item := range defaultChannelTypes {
		if err := ensureDefaultChannelType(ctx, typeModel, item); err != nil {
			return err
		}
	}

	if stepParamModel != nil {
		for _, item := range defaultStepChannelParams {
			if err := ensureDefaultStepChannelParam(ctx, stepParamModel, item); err != nil {
				return err
			}
		}
	}

	if err := migrateBuildKitToToolGroup(ctx, groupModel, channelModel, bindingModel); err != nil {
		return err
	}

	// 默认数据只维护分组和类型字典，不创建渠道实例。
	if err := channelModel.DeleteSystemPlaceholders(ctx, "system"); err != nil {
		return err
	}
	cleanupLegacySystemGroup(ctx, groupModel, channelModel)
	cleanupLegacyEmptyGroup(ctx, groupModel, channelModel, "notification")
	cleanupLegacyEmptyGroup(ctx, groupModel, channelModel, "scanner")
	cleanupLegacyEmptyGroup(ctx, groupModel, channelModel, "code_repository")
	cleanupLegacyEmptyGroup(ctx, groupModel, channelModel, "artifact_repository")
	cleanupLegacyEmptyGroup(ctx, groupModel, channelModel, "code_scanner")
	if err := channelModel.ReplaceChannelType(ctx, "artifactory", "jfrog", "system"); err != nil {
		return err
	}
	if err := channelModel.ReplaceChannelType(ctx, "kubenova", "kube-nova", "system"); err != nil {
		return err
	}
	if bindingModel != nil {
		if err := bindingModel.ReplaceChannelType(ctx, "artifactory", "jfrog", "system"); err != nil {
			return err
		}
		if err := bindingModel.ReplaceChannelType(ctx, "kubenova", "kube-nova", "system"); err != nil {
			return err
		}
	}
	if err := typeModel.DisableByCode(ctx, "artifactory", "system"); err != nil {
		return err
	}
	if err := typeModel.DisableByCode(ctx, "kubenova", "system"); err != nil {
		return err
	}

	return nil
}

func migrateBuildKitToToolGroup(ctx context.Context, groupModel *DevopsChannelGroupModel, channelModel *DevopsChannelModel, bindingModel *DevopsProjectChannelBindingModel) error {
	toolGroup, err := groupModel.FindOneByCode(ctx, "tool")
	if err != nil {
		return err
	}
	if channelModel != nil {
		if err := channelModel.MoveChannelTypeGroup(ctx, "buildkit", toolGroup.ID.Hex(), "system"); err != nil {
			return err
		}
	}
	if bindingModel != nil {
		if err := bindingModel.MoveChannelTypeGroup(ctx, "buildkit", "tool", "system"); err != nil {
			return err
		}
	}
	return nil
}

func ensureDefaultChannelGroup(ctx context.Context, groupModel *DevopsChannelGroupModel, item defaultChannelGroupSeed) (*DevopsChannelGroup, error) {
	group, err := groupModel.FindOneByCode(ctx, item.Code)
	if err != nil {
		if !errors.Is(err, ErrNotFound) {
			return nil, err
		}
		group = &DevopsChannelGroup{
			Name:                item.Name,
			Code:                item.Code,
			Description:         item.Description,
			SortOrder:           item.SortOrder,
			IsSystem:            true,
			Status:              1,
			GroupType:           item.GroupType,
			AllowedChannelTypes: item.AllowedChannelTypes,
			Icon:                item.Icon,
			IconColor:           item.IconColor,
			CreatedBy:           "system",
			UpdatedBy:           "system",
		}
		if err := groupModel.Insert(ctx, group); err != nil && !isDuplicateKey(err) {
			return nil, err
		}
		if group.ID.IsZero() {
			group, err = groupModel.FindOneByCode(ctx, item.Code)
			if err != nil {
				return nil, err
			}
		}
		return group, nil
	}
	return group, nil
}

func ensureDefaultChannelType(ctx context.Context, typeModel *DevopsChannelTypeModel, item defaultChannelTypeSeed) error {
	mappingFields, err := channelvars.NormalizeVariableSchema(item.MappingFields)
	if err != nil {
		return err
	}
	channelType := &DevopsChannelType{
		Name:             item.Name,
		Code:             item.Code,
		GroupCode:        item.GroupCode,
		CredentialTypes:  item.CredentialTypes,
		ConfigSchema:     item.ConfigSchema,
		MappingFields:    mappingFields,
		TestStrategy:     item.TestStrategy,
		Icon:             item.Icon,
		IconColor:        item.IconColor,
		ConnectionMode:   item.ConnectionMode,
		MetadataStrategy: item.MetadataStrategy,
		IsSystem:         true,
		Status:           1,
		CreatedBy:        "system",
		UpdatedBy:        "system",
	}
	exist, err := typeModel.FindOneByCode(ctx, item.Code)
	if err != nil {
		if !errors.Is(err, ErrNotFound) {
			return err
		}
		if err := typeModel.Insert(ctx, channelType); err != nil && !isDuplicateKey(err) {
			return err
		}
		return nil
	}
	_ = exist
	return nil
}

func ensureDefaultStepChannelParam(ctx context.Context, model *DevopsStepChannelParamModel, item defaultStepChannelParamSeed) error {
	_, err := model.FindOneByTarget(ctx, item.ParamType, item.GroupCode, item.ChannelTypeFilter)
	if err != nil {
		if !errors.Is(err, ErrNotFound) {
			return err
		}
		data := &DevopsStepChannelParam{
			ParamType:         item.ParamType,
			ParamName:         item.ParamName,
			GroupCode:         item.GroupCode,
			ChannelTypeFilter: item.ChannelTypeFilter,
			Description:       item.Description,
			SortOrder:         item.SortOrder,
			IsSystem:          true,
			Status:            1,
			CreatedBy:         "system",
			UpdatedBy:         "system",
		}
		if err := model.Insert(ctx, data); err != nil && !isDuplicateKey(err) {
			return err
		}
		return nil
	}
	return nil
}

func cleanupLegacySystemGroup(ctx context.Context, groupModel *DevopsChannelGroupModel, channelModel *DevopsChannelModel) {
	cleanupLegacyEmptyGroup(ctx, groupModel, channelModel, "system")
}

func cleanupLegacyEmptyGroup(ctx context.Context, groupModel *DevopsChannelGroupModel, channelModel *DevopsChannelModel, code string) {
	count, err := groupModel.CountByCode(ctx, code)
	if err != nil || count == 0 {
		return
	}
	group, err := groupModel.FindOneByCode(ctx, code)
	if err != nil {
		return
	}
	total, err := channelModel.CountByGroup(ctx, group.ID.Hex())
	if err != nil || total > 0 {
		return
	}
	_ = groupModel.DeleteSoft(ctx, group.ID.Hex(), "system")
}

func schema(fields string) string {
	return `{"fields":[` + fields + `]}`
}

func mappingFields(fields string) string {
	return `{"version":2,"fields":[` + fields + `]}`
}

func variableSchema(fields ...channelvars.VariableField) string {
	return channelvars.VariableSchemaJSON(fields)
}

func standardVariableSchema(groupCode, channelType string, extras ...channelvars.VariableField) string {
	fields := make([]channelvars.VariableField, 0, len(extras)+16)
	fields = append(fields, extras...)
	fields = append(fields, channelvars.StandardVariableFields(groupCode, channelType)...)
	return variableSchema(fields...)
}

func configVariable(field, name string) channelvars.VariableField {
	return channelvars.VariableField{Field: field, Name: name, Kind: "default", UIControl: "readonly"}
}

func dynamicVariable(field, name, provider string, deps ...channelvars.VariableDependency) channelvars.VariableField {
	return channelvars.VariableField{
		Field:        field,
		Name:         name,
		Kind:         "dynamic",
		UIControl:    "select",
		Provider:     provider,
		Dependencies: deps,
	}
}

func addressVariable(field, name, provider, builder string, deps ...channelvars.VariableDependency) channelvars.VariableField {
	return channelvars.VariableField{
		Field:            field,
		Name:             name,
		Kind:             "address",
		UIControl:        "addressPicker",
		Provider:         provider,
		Dependencies:     addressDependencies(deps),
		AllowManualInput: true,
		AddressBuilder:   builder,
	}
}

func compositeVariable(field, name, provider string, deps ...channelvars.VariableDependency) channelvars.VariableField {
	return channelvars.VariableField{
		Field:        field,
		Name:         name,
		Kind:         "composite",
		UIControl:    "deployConfig",
		Provider:     provider,
		Dependencies: deps,
	}
}

func depChannel() channelvars.VariableDependency {
	return channelvars.VariableDependency{Field: channelvars.FieldEndpoint, Source: "channel", Name: "渠道实例", Required: true}
}

func depParam(field, name string, required bool) channelvars.VariableDependency {
	return channelvars.VariableDependency{Field: field, Source: "param", Name: name, Required: required}
}

func addressDependencies(deps []channelvars.VariableDependency) []channelvars.VariableDependency {
	result := make([]channelvars.VariableDependency, 0, 1)
	for _, dep := range deps {
		if strings.TrimSpace(dep.Source) == "channel" && strings.TrimSpace(dep.Field) == channelvars.FieldEndpoint {
			result = append(result, dep)
			break
		}
	}
	if len(result) == 0 {
		result = append(result, depChannel())
	}
	return result
}

func repoVariables(projectName, projectURLName string, extras ...channelvars.VariableField) []channelvars.VariableField {
	return append(extras,
		dynamicVariable(channelvars.FieldDynamicProject, projectName, "repo.projects", depChannel()),
		dynamicVariable(channelvars.FieldDynamicBranch, "分支", "repo.branches", depChannel(), depParam(channelvars.FieldDynamicProject, projectName, true)),
		dynamicVariable(channelvars.FieldDynamicTag, "Tag", "repo.tags", depChannel(), depParam(channelvars.FieldDynamicProject, projectName, true)),
		addressVariable(channelvars.FieldAddressProjectURL, projectURLName, "repo.projectUrl", "endpoint + project", depChannel()),
	)
}

func artifactVariables(extras ...channelvars.VariableField) []channelvars.VariableField {
	return append(extras,
		dynamicVariable(channelvars.FieldDynamicRepository, "制品仓库", "artifact.repository", depChannel()),
		dynamicVariable(channelvars.FieldDynamicArtifactName, "制品名称", "artifact.artifactName", depChannel(), depParam(channelvars.FieldDynamicRepository, "制品仓库", true)),
		dynamicVariable(channelvars.FieldDynamicArtifactVersion, "制品版本", "artifact.artifactVersion", depChannel(), depParam(channelvars.FieldDynamicRepository, "制品仓库", true), depParam(channelvars.FieldDynamicArtifactName, "制品名称", true)),
		addressVariable(channelvars.FieldAddressRepositoryURL, "制品仓库地址", "artifact.repositoryUrl", "endpoint + repository", depChannel()),
		addressVariable(channelvars.FieldAddressArtifactURL, "制品地址", "artifact.artifactUrl", "endpoint + repository + artifact", depChannel()),
		addressVariable(channelvars.FieldAddressArtifactVersionURL, "制品版本地址", "artifact.artifactVersionUrl", "endpoint + repository + artifact + version", depChannel()),
	)
}

func imageRegistryVariables(extras ...channelvars.VariableField) []channelvars.VariableField {
	return append(extras,
		dynamicVariable(channelvars.FieldDynamicProject, "镜像项目", "registry.projects", depChannel()),
		dynamicVariable(channelvars.FieldDynamicImage, "镜像", "registry.images", depChannel(), depParam(channelvars.FieldDynamicProject, "镜像项目", true)),
		dynamicVariable(channelvars.FieldDynamicTag, "镜像标签", "registry.tags", depChannel(), depParam(channelvars.FieldDynamicProject, "镜像项目", true), depParam(channelvars.FieldDynamicImage, "镜像", true)),
		dynamicVariable(channelvars.FieldDynamicImageTag, "镜像加标签", "registry.imageTag", depChannel(), depParam(channelvars.FieldDynamicProject, "镜像项目", true), depParam(channelvars.FieldDynamicImage, "镜像", true)),
		addressVariable(channelvars.FieldAddressProjectURL, "镜像项目地址", "registry.projectUrl", "endpoint + project", depChannel()),
		addressVariable(channelvars.FieldAddressImageURL, "镜像地址", "registry.imageUrl", "endpoint + repository + image", depChannel()),
		addressVariable(channelvars.FieldAddressImageTagURL, "镜像带 Tag 地址", "registry.imageTagUrl", "endpoint + repository + image + tag", depChannel()),
	)
}

func sonarVariables(extras ...channelvars.VariableField) []channelvars.VariableField {
	return append(extras,
		dynamicVariable(channelvars.FieldDynamicProjectName, "项目名称", "sonar.projects", depChannel()),
		dynamicVariable(channelvars.FieldDynamicProjectKey, "项目 Key", "sonar.projectKeys", depChannel()),
	)
}

func kubeNovaVariables(extras ...channelvars.VariableField) []channelvars.VariableField {
	return append(extras,
		compositeVariable(channelvars.FieldAddressDeployConfig, "kube-nova 部署配置", "kubenova.deployConfig", depChannel()),
	)
}

func kubernetesVariables(extras ...channelvars.VariableField) []channelvars.VariableField {
	return append(extras,
		dynamicVariable(channelvars.FieldDynamicNamespace, "Namespace", "kubernetes.namespaces", depChannel()),
		dynamicVariable(channelvars.FieldDynamicWorkloadType, "部署类型", "kubernetes.workloadType", depChannel()),
		dynamicVariable(channelvars.FieldDynamicResourceName, "资源名称", "kubernetes.resourceName", depChannel(), depParam(channelvars.FieldDynamicNamespace, "Namespace", true), depParam(channelvars.FieldDynamicWorkloadType, "部署类型", true)),
		dynamicVariable(channelvars.FieldDynamicContainerName, "容器名称", "kubernetes.containerName", depChannel(), depParam(channelvars.FieldDynamicNamespace, "Namespace", true), depParam(channelvars.FieldDynamicWorkloadType, "部署类型", true), depParam(channelvars.FieldDynamicResourceName, "资源名称", true)),
		dynamicVariable(channelvars.FieldDynamicImageName, "镜像名称", "kubernetes.imageName", depChannel(), depParam(channelvars.FieldDynamicNamespace, "Namespace", true), depParam(channelvars.FieldDynamicWorkloadType, "部署类型", true), depParam(channelvars.FieldDynamicResourceName, "资源名称", true), depParam(channelvars.FieldDynamicContainerName, "容器名称", true)),
		compositeVariable(channelvars.FieldAddressDeploymentConfig, "Kubernetes 部署配置", "kubernetes.deploymentConfig", depChannel()),
	)
}

func defaultChannelCredentialTypes(extra ...string) []string {
	items := append([]string{"username_password", "token"}, extra...)
	result := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		result = append(result, item)
	}
	return result
}

func tektonChannelCredentialTypes() []string {
	return []string{"kubeconfig", "token", "secret_text", "json"}
}

func sameStringSet(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	counts := make(map[string]int, len(left))
	for _, item := range left {
		counts[item]++
	}
	for _, item := range right {
		if counts[item] == 0 {
			return false
		}
		counts[item]--
	}
	return true
}
