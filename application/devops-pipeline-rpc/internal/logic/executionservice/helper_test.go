package executionservicelogic

import (
	"context"
	"encoding/base64"
	"strings"
	"testing"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/client/pipelineconfigservice"
	"github.com/yanshicheng/kube-nova/application/devops-pipeline-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/common/devops/channelvars"
	devopstekton "github.com/yanshicheng/kube-nova/common/devops/tekton"
)

func TestValidateStepRequiredParamsRejectsInvalidOptionalObjectList(t *testing.T) {
	step := model.PipelineStep{
		NodeName:    "多仓库克隆",
		ParamValues: `{"REPOS_JSON":{"repoName":"app-service"}}`,
	}
	params := []*pipelineconfigservice.DevopsStepParam{
		{
			Name:      "仓库列表",
			Code:      "REPOS_JSON",
			ParamType: objectListParamType,
			Mode:      "env",
			Config: &pipelineconfigservice.StepParamConfig{
				CompoundFields: []*pipelineconfigservice.CompoundParamField{
					{Name: "仓库名称", Code: "repoName", ParamType: "string", Mode: "params"},
				},
			},
		},
	}

	err := validateStepRequiredParams(step, params)
	if err == nil || !strings.Contains(err.Error(), "必须是对象数组") {
		t.Fatalf("optional objectList with object value should be rejected, got %v", err)
	}
}

func TestValidateJenkinsRuntimeRejectsTektonChannel(t *testing.T) {
	runtime := &pipelineconfigservice.ResolvePipelineRuntimeResp{
		Binding: &pipelineconfigservice.DevopsProjectChannelBinding{ChannelType: engineTekton},
		Channel: &pipelineconfigservice.DevopsChannel{ChannelType: engineTekton},
	}

	err := validateJenkinsRuntime(runtime)
	if err == nil || !strings.Contains(err.Error(), "Jenkins 流水线必须选择 Jenkins 构建渠道") {
		t.Fatalf("tekton channel should be rejected for jenkins runtime, got %v", err)
	}
}

func TestValidateJenkinsRuntimeAcceptsJenkinsChannel(t *testing.T) {
	runtime := &pipelineconfigservice.ResolvePipelineRuntimeResp{
		Binding: &pipelineconfigservice.DevopsProjectChannelBinding{ChannelType: engineJenkins},
		Channel: &pipelineconfigservice.DevopsChannel{ChannelType: engineJenkins},
	}

	if err := validateJenkinsRuntime(runtime); err != nil {
		t.Fatalf("jenkins runtime should be accepted: %v", err)
	}
}

func TestValidatePipelinePostConditionUniquenessRejectsDuplicate(t *testing.T) {
	steps := []model.PipelineStep{
		{
			ID:           "notify",
			NodeName:     "成功通知",
			ParentNodeID: workflowPostNodeID,
			Enabled:      true,
			StageContent: "success { echo 'ok' }",
		},
		{
			ID:           "archive",
			NodeName:     "成功归档",
			ParentNodeID: workflowPostNodeID,
			Enabled:      true,
			StageContent: "success { echo 'again' }",
		},
	}

	err := validatePipelinePostConditionUniqueness(steps)
	if err == nil || !strings.Contains(err.Error(), "post 条件 success 只能配置一次") {
		t.Fatalf("duplicate success post condition should be rejected, got %v", err)
	}
}

func TestValidatePipelineObjectListParamRejectsParamsModeField(t *testing.T) {
	param := model.PipelineParam{
		Name:        "仓库列表",
		Code:        "REPOS_JSON",
		ParamType:   objectListParamType,
		Mode:        "env",
		RuntimeMode: "env",
		Config: model.StepParamConfig{
			CompoundFields: []model.CompoundParamField{
				{Name: "仓库名称", Code: "repoName", ParamType: "string", Mode: "params"},
			},
		},
	}

	err := validatePipelineObjectListParam(param)
	if err == nil || !strings.Contains(err.Error(), "只支持环境变量或平台变量") {
		t.Fatalf("objectList field params mode should be rejected, got %v", err)
	}
}

func TestValidatePipelineObjectListParamRejectsParamsRuntimeField(t *testing.T) {
	param := model.PipelineParam{
		Name:        "仓库列表",
		Code:        "REPOS_JSON",
		ParamType:   objectListParamType,
		Mode:        "env",
		RuntimeMode: "env",
		Config: model.StepParamConfig{
			CompoundFields: []model.CompoundParamField{
				{
					Name:        "仓库地址",
					Code:        "repoUrl",
					ParamType:   channelvars.ParamGitProject,
					Mode:        "paas",
					RuntimeMode: "params",
					Config: model.StepParamConfig{
						ChannelBindingID: "binding-id",
					},
				},
			},
		},
	}

	err := validatePipelineObjectListParam(param)
	if err == nil || !strings.Contains(err.Error(), "不支持执行变量映射") {
		t.Fatalf("objectList field params runtime should be rejected, got %v", err)
	}
}

func TestValidatePipelineObjectListParamAllowsPlatformMode(t *testing.T) {
	param := model.PipelineParam{
		Name:        "仓库列表",
		Code:        "REPOS_JSON",
		ParamType:   objectListParamType,
		Mode:        "paas",
		RuntimeMode: "params",
		Config: model.StepParamConfig{
			CompoundFields: []model.CompoundParamField{
				{Name: "仓库名称", Code: "repoName", ParamType: "string", Mode: "env"},
			},
		},
	}

	if err := validatePipelineObjectListParam(param); err != nil {
		t.Fatalf("platform objectList should be accepted: %v", err)
	}
}

func TestValidatePipelineParamsAllowsEnvText(t *testing.T) {
	param := model.PipelineParam{
		Name:         "配置文本",
		Code:         "CONFIG_TEXT",
		ParamType:    "text",
		Mode:         "env",
		DefaultValue: "文本1\n文本2\n文本3",
	}

	if err := validatePipelineParams(engineJenkins, []model.PipelineParam{param}); err != nil {
		t.Fatalf("env text should be allowed, got %v", err)
	}
}

func TestNormalizePipelineParamConfigPreservesFullRow(t *testing.T) {
	param := model.PipelineParam{
		Name:      "命令",
		Code:      "BASH_COMMAND",
		ParamType: "text",
		Mode:      "env",
		Config: model.StepParamConfig{
			FullRow: true,
		},
	}

	got := normalizePipelineParamConfig(param)
	if !got.FullRow {
		t.Fatal("fullRow should be preserved")
	}
}

func TestValidatePipelineParamsAllowsProjectDynamicParamTypes(t *testing.T) {
	params := []model.PipelineParam{
		{
			Name:        "Maven 配置",
			Code:        "MAVEN_CONFIG",
			ParamType:   channelvars.ParamMavenConfig,
			Mode:        "paas",
			RuntimeMode: "env",
		},
		{
			Name:        "Sonar 地址",
			Code:        "SONAR_URL",
			ParamType:   channelvars.ParamSonarAddress,
			Mode:        "paas",
			RuntimeMode: "env",
		},
		{
			Name:        "Sonar 项目 Key",
			Code:        "SONAR_PROJECT_KEY",
			ParamType:   channelvars.ParamSonarProjectKey,
			Mode:        "paas",
			RuntimeMode: "env",
			Config: model.StepParamConfig{
				ChannelParamCode: "SONAR_URL",
			},
		},
		{
			Name:        "主机组",
			Code:        "HOSTS",
			ParamType:   channelvars.ParamHostGroupTargets,
			Mode:        "paas",
			RuntimeMode: "env",
			Config: model.StepParamConfig{
				RenderMode: "normal",
			},
		},
	}

	if err := validatePipelineParams(engineJenkins, params); err != nil {
		t.Fatalf("project dynamic param types should be allowed: %v", err)
	}
}

func TestValidatePipelineParamsAllowsHostGroupExtendedRenderModes(t *testing.T) {
	for _, renderMode := range []string{"json", "jsonBase64", "ansible", "ansibleBase64"} {
		t.Run(renderMode, func(t *testing.T) {
			params := []model.PipelineParam{
				{
					Name:        "主机组",
					Code:        "HOSTS",
					ParamType:   channelvars.ParamHostGroupTargets,
					Mode:        "paas",
					RuntimeMode: "env",
					Config: model.StepParamConfig{
						RenderMode: renderMode,
					},
				},
			}
			if err := validatePipelineParams(engineJenkins, params); err != nil {
				t.Fatalf("host group render mode %s should be allowed: %v", renderMode, err)
			}
		})
	}
}

func TestValidatePipelineParamsAllowsAllDynamicParamTypes(t *testing.T) {
	for _, paramType := range channelvars.DynamicParamTypes() {
		if paramType == channelvars.ParamConfigCenter {
			continue
		}
		t.Run(paramType, func(t *testing.T) {
			param := model.PipelineParam{
				Name:        paramType,
				Code:        strings.ToUpper(paramType),
				ParamType:   paramType,
				Mode:        "paas",
				RuntimeMode: "params",
			}
			if err := validatePipelineParams(engineJenkins, []model.PipelineParam{param}); err != nil {
				t.Fatalf("dynamic param type should be allowed: %v", err)
			}
		})
	}
}

func TestValidatePipelineParamsAllowsKubeNovaDeployConfig(t *testing.T) {
	param := model.PipelineParam{
		Name:         "kube-nova 部署配置",
		Code:         "KUBE_NOVA_DEPLOY_CONFIG",
		ParamType:    channelvars.ParamKubeNovaDeployConfig,
		Mode:         "paas",
		RuntimeMode:  "params",
		Required:     true,
		CurrentValue: `{"baseUrl":"https://kube-nova.example.com","tokenBase64":"dG9rZW4=","projectId":1,"workspaceId":2,"applicationId":3,"versionId":4,"containerName":"app"}`,
	}

	if err := validatePipelineParams(engineJenkins, []model.PipelineParam{param}); err != nil {
		t.Fatalf("kube-nova deploy config should be allowed: %v", err)
	}

	config := normalizePipelineParamConfig(param)
	if !config.FullRow {
		t.Fatal("kube-nova deploy config should be full row")
	}
	if config.ChannelGroupCode != channelvars.GroupDeployTarget || config.MappingField != channelvars.FieldAddressDeployConfig {
		t.Fatalf("unexpected kube-nova channel mapping: %#v", config)
	}
	if got := renderJenkinsParamType(param, config); got != "string" {
		t.Fatalf("kube-nova deploy config should render as Jenkins string parameter, got %s", got)
	}
}

func TestResolvePlatformParamValueKeepsKubeNovaDeployConfigJSON(t *testing.T) {
	value := `{"baseUrl":"https://kube-nova.example.com","tokenBase64":"dG9rZW4=","projectId":1,"workspaceId":2,"applicationId":3,"versionId":4,"containerName":"app"}`
	param := model.PipelineParam{
		Name:         "kube-nova 部署配置",
		Code:         "KUBE_NOVA_DEPLOY_CONFIG",
		ParamType:    channelvars.ParamKubeNovaDeployConfig,
		Mode:         "paas",
		RuntimeMode:  "params",
		CurrentValue: value,
	}

	got, err := resolvePlatformParamValue(context.Background(), nil, &model.DevopsPipeline{ProjectID: "project-1"}, param, pipelineRenderContext{})
	if err != nil {
		t.Fatalf("resolve kube-nova deploy config should not fail: %v", err)
	}
	if got != value {
		t.Fatalf("kube-nova deploy config json should be preserved, got %s", got)
	}
}

func TestChannelVariableDeployConfigUsesDeployConfigRuntime(t *testing.T) {
	value := `{"channelBindingId":"0123456789abcdef01234567","baseUrl":"https://kube-nova.example.com","projectId":1,"workspaceId":2,"applicationId":3,"versionId":4,"containerName":"app"}`
	param := model.PipelineParam{
		Name:         "kube-nova 部署配置",
		Code:         "KUBE_NOVA_DEPLOY_CONFIG",
		ParamType:    channelvars.ParamChannelVariable,
		Mode:         "paas",
		RuntimeMode:  "params",
		CurrentValue: value,
		Config: model.StepParamConfig{
			ChannelGroupCode:  channelvars.GroupDeployTarget,
			ChannelTypeFilter: "kube-nova",
			MappingField:      channelvars.FieldAddressDeployConfig,
		},
	}
	config := normalizePipelineParamConfig(param)
	if !isKubeNovaDeployConfigPipelineParam(param, config) {
		t.Fatal("channel variable address.deployConfig should use kube-nova deploy config runtime")
	}
	if got := renderJenkinsParamType(param, config); got != "string" {
		t.Fatalf("channel variable deploy config should render as Jenkins string parameter, got %s", got)
	}
	got, err := resolvePlatformParamValue(context.Background(), nil, &model.DevopsPipeline{ProjectID: "project-1"}, param, pipelineRenderContext{})
	if err != nil {
		t.Fatalf("resolve channel variable kube-nova deploy config should not fail: %v", err)
	}
	if got != value {
		t.Fatalf("channel variable kube-nova deploy config json should be preserved without svc, got %s", got)
	}
}

func TestResolvePlatformParamValueKeepsChannelDynamicChildValue(t *testing.T) {
	param := model.PipelineParam{
		Name:         "镜像项目",
		Code:         "BUILDKIT_REGISTRY_PROJECT",
		ParamType:    channelvars.ParamChannelVariable,
		Mode:         "paas",
		RuntimeMode:  "params",
		CurrentValue: "public",
		Config: model.StepParamConfig{
			ChannelGroupCode:  channelvars.GroupImageRepo,
			ChannelTypeFilter: "harbor",
			ChannelParamCode:  "BUILDKIT_REGISTRY_URL",
			MappingField:      channelvars.FieldDynamicRepository,
		},
	}

	got, err := resolvePlatformParamValue(context.Background(), nil, &model.DevopsPipeline{ProjectID: "project-1"}, param, pipelineRenderContext{})
	if err != nil {
		t.Fatalf("resolve channel dynamic child should not fail: %v", err)
	}
	if got != "public" {
		t.Fatalf("channel dynamic child value should be preserved, got %s", got)
	}
}

func TestPipelineImageRegistryLegacyRepositoryFieldNormalizesToProject(t *testing.T) {
	param := model.PipelineParam{
		ParamType: channelvars.ParamChannelVariable,
		Config: model.StepParamConfig{
			ChannelGroupCode:  channelvars.GroupImageRepo,
			ChannelTypeFilter: "harbor",
			MappingField:      channelvars.FieldDynamicRepository,
		},
	}

	config := normalizePipelineParamConfig(param)
	if config.MappingField != channelvars.FieldDynamicProject {
		t.Fatalf("image registry legacy repository field = %s, want %s", config.MappingField, channelvars.FieldDynamicProject)
	}
	if got := pipelineDynamicParamType(param); got != channelvars.ParamHarborProject {
		t.Fatalf("image registry legacy repository type = %s, want %s", got, channelvars.ParamHarborProject)
	}
}

func TestResolveChannelCredentialParamSkipsOptionalEmptySource(t *testing.T) {
	source := model.PipelineParam{
		Name:      "仓库地址",
		Code:      "NPM_REGISTRY",
		ParamType: channelvars.ParamChannelVariable,
		Mode:      "paas",
		Config: model.StepParamConfig{
			ChannelGroupCode:  channelvars.GroupArtifactRepo,
			ChannelTypeFilter: "nexus",
			MappingField:      channelvars.FieldAddressRepositoryURL,
		},
	}
	param := model.PipelineParam{
		Name:      "仓库凭证",
		Code:      "NPM_REGISTRY_CREDENTIALS_ID",
		ParamType: channelvars.ParamChannelCredential,
		Mode:      "paas",
		Config: model.StepParamConfig{
			CredentialMode:            credentialModeJenkinsID,
			CredentialSourceParamCode: "NPM_REGISTRY",
		},
	}

	got, err := resolvePlatformParamValue(context.Background(), nil, &model.DevopsPipeline{
		Name:      "前端npm构建",
		ProjectID: "project-1",
		Params:    []model.PipelineParam{source, param},
	}, param, pipelineRenderContext{})
	if err != nil {
		t.Fatalf("optional channel credential with empty source should be skipped: %v", err)
	}
	if got != "" {
		t.Fatalf("optional channel credential should resolve empty value, got %q", got)
	}
}

func TestResolvePlatformParamValueKeepsCustomChannelDynamicValue(t *testing.T) {
	param := model.PipelineParam{
		Name:         "Argo CD 应用",
		Code:         "ARGOCD_APPLICATION",
		ParamType:    channelvars.ParamChannelVariable,
		Mode:         "paas",
		RuntimeMode:  "params",
		CurrentValue: "pay-app",
		Config: model.StepParamConfig{
			ChannelGroupCode:  channelvars.GroupDeployTarget,
			ChannelTypeFilter: "argocd",
			ChannelParamCode:  "ARGOCD_CHANNEL",
			MappingField:      channelvars.FieldDynamicApplication,
		},
	}

	got, err := resolvePlatformParamValue(context.Background(), nil, &model.DevopsPipeline{ProjectID: "project-1"}, param, pipelineRenderContext{})
	if err != nil {
		t.Fatalf("resolve custom channel dynamic value should not fail: %v", err)
	}
	if got != "pay-app" {
		t.Fatalf("custom channel dynamic value should be preserved, got %s", got)
	}
}

func TestResolvePlatformParamValueAllowsManualChannelValue(t *testing.T) {
	param := model.PipelineParam{
		Name:         "镜像仓库地址",
		Code:         "BUILDKIT_REGISTRY_URL",
		ParamType:    channelvars.ParamChannelVariable,
		Mode:         "paas",
		RuntimeMode:  "params",
		CurrentValue: "public",
		Config: model.StepParamConfig{
			ChannelGroupCode:  channelvars.GroupImageRepo,
			ChannelTypeFilter: "harbor",
			MappingField:      channelvars.FieldEndpoint,
		},
	}

	got, err := resolvePlatformParamValue(context.Background(), nil, &model.DevopsPipeline{Name: "前端npm构建", ProjectID: "project-1"}, param, pipelineRenderContext{})
	if err != nil {
		t.Fatalf("manual channel value should be allowed: %v", err)
	}
	if got != "public" {
		t.Fatalf("manual channel value should be preserved, got %s", got)
	}
}

func TestResolveCredentialSourceBindingIDUsesParentChannelForCustomDynamicValue(t *testing.T) {
	items := []model.PipelineParam{
		{
			Code:         "ARGOCD_CHANNEL",
			ParamType:    channelvars.ParamChannelVariable,
			CurrentValue: "binding-1",
			Config: model.StepParamConfig{
				ChannelGroupCode:  channelvars.GroupDeployTarget,
				ChannelTypeFilter: "argocd",
				MappingField:      channelvars.FieldEndpoint,
			},
		},
		{
			Code:         "ARGOCD_APPLICATION",
			ParamType:    channelvars.ParamChannelVariable,
			CurrentValue: "pay-app",
			Config: model.StepParamConfig{
				ChannelGroupCode:  channelvars.GroupDeployTarget,
				ChannelTypeFilter: "argocd",
				ChannelParamCode:  "ARGOCD_CHANNEL",
				MappingField:      channelvars.FieldDynamicApplication,
			},
		},
	}

	got := resolveCredentialSourceBindingID(items, items[1], pipelineRenderContext{})
	if got != "binding-1" {
		t.Fatalf("custom dynamic source should resolve parent channel binding, got %s", got)
	}
}

func TestPipelineParamsHaveRenderSources(t *testing.T) {
	params := []model.PipelineParam{
		{Code: "GIT_REPO_URL", SourceCode: "GIT_REPO_URL", StepNodeID: "node-1"},
		{Code: "BUILDKIT_REGISTRY_PROJECT", SourceCode: "BUILDKIT_REGISTRY_PROJECT", StepNodeID: "node-2"},
	}

	if !pipelineParamsHaveRenderSources(params) {
		t.Fatalf("params with source and step node should skip render-time template lookup")
	}

	params[1].SourceCode = ""
	if pipelineParamsHaveRenderSources(params) {
		t.Fatalf("params missing source code should keep fallback lookup")
	}
}

func TestKubeNovaTokenFromCredentialJSON(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{name: "token", value: `{"token":"abc"}`, want: "abc"},
		{name: "secret text", value: `{"secretText":"secret"}`, want: "secret"},
		{name: "empty", value: `{"username":"robot"}`, want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := kubeNovaTokenFromCredentialJSON(tt.value); got != tt.want {
				t.Fatalf("unexpected token: got %s want %s", got, tt.want)
			}
		})
	}
}

func TestAppendMavenSettingsArgSkipsExplicitSettings(t *testing.T) {
	got := appendMavenSettingsArg("sh 'mvn -s custom-settings.xml clean package'")
	if got != "sh 'mvn -s custom-settings.xml clean package'" {
		t.Fatalf("explicit settings should be preserved: %s", got)
	}

	got = appendMavenSettingsArg("sh 'mvn --settings custom-settings.xml clean package'")
	if got != "sh 'mvn --settings custom-settings.xml clean package'" {
		t.Fatalf("explicit long settings should be preserved: %s", got)
	}
}

func TestAppendMavenSettingsArgAddsSettings(t *testing.T) {
	got := appendMavenSettingsArg("sh 'mvn clean package && ./mvnw test'")
	if !strings.Contains(got, "mvn -s ./maven/settings.xml clean package") {
		t.Fatalf("mvn should include settings: %s", got)
	}
	if !strings.Contains(got, "./mvnw -s ./maven/settings.xml test") {
		t.Fatalf("mvnw should include settings: %s", got)
	}
}

func TestAppendMavenSettingsArgHandlesMixedCommands(t *testing.T) {
	got := appendMavenSettingsArg("sh 'mvn -s custom-settings.xml clean package && ./mvnw test'")
	if !strings.Contains(got, "mvn -s custom-settings.xml clean package") {
		t.Fatalf("explicit mvn settings should be preserved: %s", got)
	}
	if !strings.Contains(got, "./mvnw -s ./maven/settings.xml test") {
		t.Fatalf("mvnw without settings should include generated settings: %s", got)
	}
}

func TestValidatePipelineParamsAllowsGitBranchWithRepositoryURL(t *testing.T) {
	params := []model.PipelineParam{
		{
			Name:        "仓库地址",
			Code:        "GIT_HTTP_REPO_URL",
			ParamType:   channelvars.ParamGitRepositoryURL,
			Mode:        "paas",
			RuntimeMode: "env",
		},
		{
			Name:        "分支",
			Code:        "BRANCH",
			ParamType:   channelvars.ParamGitBranch,
			Mode:        "paas",
			RuntimeMode: "env",
			Config: model.StepParamConfig{
				ProjectParamCode: "GIT_HTTP_REPO_URL",
			},
		},
	}

	if err := validatePipelineParams(engineJenkins, params); err != nil {
		t.Fatalf("git branch should allow gitRepositoryUrl dependency: %v", err)
	}
}

func TestValidatePipelineParamsAllowsAllCompositeAddressTypes(t *testing.T) {
	paramTypes := []string{
		channelvars.ParamGitRepositoryURL,
		channelvars.ParamNexusRepositoryURL,
		channelvars.ParamNexusArtifactURL,
		channelvars.ParamHarborProjectURL,
		channelvars.ParamHarborImageURL,
		channelvars.ParamHarborImageTagURL,
		channelvars.ParamRegistryRepositoryURL,
		channelvars.ParamRegistryImageURL,
		channelvars.ParamRegistryImageTagURL,
		channelvars.ParamJfrogRepositoryURL,
		channelvars.ParamJfrogArtifactURL,
		channelvars.ParamSonarAddress,
		channelvars.ParamSonarProjectURL,
		channelvars.ParamSonarProjectKeyURL,
	}
	for _, paramType := range paramTypes {
		t.Run(paramType, func(t *testing.T) {
			param := model.PipelineParam{
				Name:        paramType,
				Code:        strings.ToUpper(paramType),
				ParamType:   paramType,
				Mode:        "paas",
				RuntimeMode: "params",
			}
			if err := validatePipelineParams(engineJenkins, []model.PipelineParam{param}); err != nil {
				t.Fatalf("composite address type should be allowed: %v", err)
			}
			if !isSupportedTektonPipelineParamType(paramType) {
				t.Fatalf("tekton pipeline should support composite address type")
			}
			if !isSupportedTektonDagPipelineParamType(paramType) {
				t.Fatalf("tekton dag should support composite address type")
			}
		})
	}
}

func TestValidatePipelineParamsAllowsChannelVariableBranchManualInput(t *testing.T) {
	params := []model.PipelineParam{
		{
			Name:        "GitLab 渠道",
			Code:        "GIT_CHANNEL",
			ParamType:   channelvars.ParamChannelVariable,
			Mode:        "paas",
			RuntimeMode: "env",
			Config: model.StepParamConfig{
				ChannelGroupCode:  channelvars.GroupCodeRepo,
				ChannelTypeFilter: "gitlab",
				MappingField:      channelvars.FieldEndpoint,
			},
		},
		{
			Name:        "Git 分支",
			Code:        "BRANCH",
			ParamType:   channelvars.ParamChannelVariable,
			Mode:        "paas",
			RuntimeMode: "params",
			Config: model.StepParamConfig{
				ChannelGroupCode:  channelvars.GroupCodeRepo,
				ChannelTypeFilter: "gitlab",
				MappingField:      "dynamic.branch",
				ChannelParamCode:  "GIT_CHANNEL",
			},
		},
	}

	err := validatePipelineParams(engineJenkins, params)
	if err != nil {
		t.Fatalf("channel variable branch should allow manual input without dependencies: %v", err)
	}
}

func TestResolveCredentialSourceBindingIDIgnoresManualEndpoint(t *testing.T) {
	items := []model.PipelineParam{
		{
			Code:      "GIT_CHANNEL",
			ParamType: channelvars.ParamChannelVariable,
			Config: model.StepParamConfig{
				ChannelGroupCode:  channelvars.GroupCodeRepo,
				ChannelTypeFilter: "gitlab",
				MappingField:      channelvars.FieldEndpoint,
			},
		},
	}
	renderCtx := pipelineRenderContext{Params: map[string]string{"GIT_CHANNEL": "https://gitlab.example.com"}}

	got := resolveCredentialSourceBindingID(items, items[0], renderCtx)
	if got != "" {
		t.Fatalf("manual endpoint should not be treated as binding id, got %s", got)
	}
}

func TestCredentialSourceMappingFieldUsesAddressSource(t *testing.T) {
	source := model.PipelineParam{
		ParamType: channelvars.ParamChannelVariable,
		Config: model.StepParamConfig{
			ChannelGroupCode:  channelvars.GroupCodeRepo,
			ChannelTypeFilter: "gitlab",
			MappingField:      channelvars.FieldAddressProjectURL,
		},
	}

	got := credentialSourceMappingField(source, "username")
	if got != channelvars.FieldAddressProjectURL {
		t.Fatalf("credential source mapping field = %s, want %s", got, channelvars.FieldAddressProjectURL)
	}
}

func TestValidatePipelineParamsAllowsChannelVariableBranchDependencies(t *testing.T) {
	params := []model.PipelineParam{
		{
			Name:        "GitLab 渠道",
			Code:        "GIT_CHANNEL",
			ParamType:   channelvars.ParamChannelVariable,
			Mode:        "paas",
			RuntimeMode: "env",
			Config: model.StepParamConfig{
				ChannelGroupCode:  channelvars.GroupCodeRepo,
				ChannelTypeFilter: "gitlab",
				MappingField:      "endpoint",
			},
		},
		{
			Name:        "Git 项目",
			Code:        "GIT_PROJECT",
			ParamType:   channelvars.ParamChannelVariable,
			Mode:        "paas",
			RuntimeMode: "params",
			Config: model.StepParamConfig{
				ChannelGroupCode:  channelvars.GroupCodeRepo,
				ChannelTypeFilter: "gitlab",
				MappingField:      "dynamic.project",
				ChannelParamCode:  "GIT_CHANNEL",
			},
		},
		{
			Name:        "Git 分支",
			Code:        "BRANCH",
			ParamType:   channelvars.ParamChannelVariable,
			Mode:        "paas",
			RuntimeMode: "params",
			Config: model.StepParamConfig{
				ChannelGroupCode:  channelvars.GroupCodeRepo,
				ChannelTypeFilter: "gitlab",
				MappingField:      "dynamic.branch",
				ChannelParamCode:  "GIT_CHANNEL",
				DependencyParamCodes: []model.StepParamDependency{
					{Field: channelvars.FieldDynamicProject, ParamCode: "GIT_PROJECT"},
				},
			},
		},
	}

	if err := validatePipelineParams(engineJenkins, params); err != nil {
		t.Fatalf("channel variable branch dependencies should be allowed: %v", err)
	}
}

func TestValidatePipelineParamsAllowsImageRegistryAddressDependencies(t *testing.T) {
	params := []model.PipelineParam{
		imageRegistryPipelineParam("IMG_PROJECT_URL", channelvars.FieldAddressProjectURL, nil, ""),
		imageRegistryPipelineParam("IMG_URL", channelvars.FieldAddressImageURL, nil, ""),
		imageRegistryPipelineParam("IMG_IMAGE", channelvars.FieldDynamicImage, []model.StepParamDependency{
			{Field: channelvars.FieldAddressProjectURL, ParamCode: "IMG_PROJECT_URL"},
		}, ""),
		imageRegistryPipelineParam("IMG_TAG", channelvars.FieldDynamicTag, []model.StepParamDependency{
			{Field: channelvars.FieldAddressImageURL, ParamCode: "IMG_URL"},
		}, ""),
		imageRegistryPipelineParam("IMG_IMAGE_TAG", channelvars.FieldDynamicImageTag, []model.StepParamDependency{
			{Field: channelvars.FieldAddressProjectURL, ParamCode: "IMG_PROJECT_URL"},
		}, ""),
	}

	if err := validatePipelineParams(engineJenkins, params); err != nil {
		t.Fatalf("image registry address dependencies should be allowed: %v", err)
	}

	tektonParams := append([]model.PipelineParam(nil), params...)
	for i := range tektonParams {
		tektonParams[i].Mode = "params"
		tektonParams[i].RuntimeMode = "params"
	}
	if err := validatePipelineParams(engineTekton, tektonParams); err != nil {
		t.Fatalf("tekton image registry address dependencies should be allowed: %v", err)
	}
}

func TestValidatePipelineParamsAllowsImageRegistryTagProjectURLManualInput(t *testing.T) {
	params := []model.PipelineParam{
		imageRegistryPipelineParam("IMG_PROJECT_URL", channelvars.FieldAddressProjectURL, nil, ""),
		imageRegistryPipelineParam("IMG_TAG", channelvars.FieldDynamicTag, []model.StepParamDependency{
			{Field: channelvars.FieldAddressProjectURL, ParamCode: "IMG_PROJECT_URL"},
		}, ""),
	}

	err := validatePipelineParams(engineJenkins, params)
	if err != nil {
		t.Fatalf("image tag with project URL should allow manual input without image dependency: %v", err)
	}
}

func imageRegistryPipelineParam(code, mappingField string, deps []model.StepParamDependency, channelParamCode string) model.PipelineParam {
	return model.PipelineParam{
		Name:        code,
		Code:        code,
		ParamType:   channelvars.ParamChannelVariable,
		Mode:        "paas",
		RuntimeMode: "params",
		Config: model.StepParamConfig{
			ChannelGroupCode:     channelvars.GroupImageRepo,
			ChannelTypeFilter:    "harbor",
			MappingField:         mappingField,
			ChannelParamCode:     channelParamCode,
			DependencyParamCodes: deps,
		},
	}
}

func TestChannelVariableAddressParamsAreNotRemoteSelects(t *testing.T) {
	cases := []struct {
		name        string
		groupCode   string
		channelType string
		field       string
		wantType    string
	}{
		{name: "harbor image tag url", groupCode: channelvars.GroupImageRepo, channelType: "harbor", field: channelvars.FieldAddressImageTagURL, wantType: channelvars.ParamHarborImageTagURL},
		{name: "registry image url", groupCode: channelvars.GroupImageRepo, channelType: "registry", field: channelvars.FieldAddressImageURL, wantType: channelvars.ParamRegistryImageURL},
		{name: "nexus artifact url", groupCode: channelvars.GroupArtifactRepo, channelType: "nexus", field: channelvars.FieldAddressArtifactURL, wantType: channelvars.ParamNexusArtifactURL},
		{name: "jfrog artifact url", groupCode: channelvars.GroupArtifactRepo, channelType: "jfrog", field: channelvars.FieldAddressArtifactURL, wantType: channelvars.ParamJfrogArtifactURL},
		{name: "sonar project key url", groupCode: channelvars.GroupCodeScan, channelType: "sonarqube", field: channelvars.FieldAddressProjectKeyURL, wantType: channelvars.ParamSonarProjectKeyURL},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			param := model.PipelineParam{
				ParamType: channelvars.ParamChannelVariable,
				Mode:      "paas",
				Config: model.StepParamConfig{
					ChannelGroupCode:  tt.groupCode,
					ChannelTypeFilter: tt.channelType,
					MappingField:      tt.field,
				},
			}
			if got := pipelineDynamicParamType(param); got != tt.wantType {
				t.Fatalf("pipeline dynamic type = %s, want %s", got, tt.wantType)
			}
			if isDynamicPipelineParam(param) {
				t.Fatalf("%s should be treated as address input, not remote select", tt.field)
			}
		})
	}
}

func TestValidateTektonPipelineParamsAllowsPlatformTypesAsParams(t *testing.T) {
	params := []model.PipelineParam{
		{
			Name:        "仓库地址",
			Code:        "GIT_URL",
			ParamType:   channelvars.ParamGitRepositoryURL,
			Mode:        "params",
			RuntimeMode: "params",
		},
		{
			Name:         "认证 Secret",
			Code:         "GIT_SECRET",
			ParamType:    channelvars.ParamKubernetesSecretName,
			Mode:         "params",
			RuntimeMode:  "params",
			CurrentValue: "git-basic-auth",
		},
	}

	if err := validatePipelineParams(engineTekton, params); err != nil {
		t.Fatalf("tekton platform params should be mapped to params: %v", err)
	}
}

func TestValidateTektonPipelineParamsAllowsChannelCredentialParam(t *testing.T) {
	params := []model.PipelineParam{
		{
			Name:        "源码渠道",
			Code:        "GIT_CHANNEL",
			ParamType:   channelvars.ParamChannelVariable,
			Mode:        "params",
			RuntimeMode: "params",
			Config: model.StepParamConfig{
				ChannelGroupCode:  channelvars.GroupCodeRepo,
				ChannelTypeFilter: "gitlab",
				MappingField:      channelvars.FieldEndpoint,
			},
		},
		{
			Name:        "渠道凭证",
			Code:        "GIT_CREDENTIAL",
			ParamType:   channelvars.ParamChannelCredential,
			Mode:        "params",
			RuntimeMode: "params",
			Config: model.StepParamConfig{
				CredentialSourceParamCode: "GIT_CHANNEL",
				MappingField:              "username",
			},
		},
	}

	if err := validatePipelineParams(engineTekton, params); err != nil {
		t.Fatalf("tekton channel credential param should be accepted: %v", err)
	}
}

func TestValidateTektonPipelineParamsRejectsChannelCredentialJenkinsID(t *testing.T) {
	params := []model.PipelineParam{
		{
			Name:        "源码渠道",
			Code:        "GIT_CHANNEL",
			ParamType:   channelvars.ParamChannelVariable,
			Mode:        "params",
			RuntimeMode: "params",
			Config: model.StepParamConfig{
				ChannelGroupCode:  channelvars.GroupCodeRepo,
				ChannelTypeFilter: "gitlab",
				MappingField:      channelvars.FieldEndpoint,
			},
		},
		{
			Name:        "渠道凭证",
			Code:        "GIT_CREDENTIAL",
			ParamType:   channelvars.ParamChannelCredential,
			Mode:        "params",
			RuntimeMode: "params",
			Config: model.StepParamConfig{
				CredentialSourceParamCode: "GIT_CHANNEL",
				CredentialMode:            "jenkinsCredentialId",
				MappingField:              "credentialId",
			},
		},
	}

	err := validatePipelineParams(engineTekton, params)
	if err == nil || !strings.Contains(err.Error(), "Jenkins credentialId") {
		t.Fatalf("tekton channel credential should reject Jenkins credentialId mode, got %v", err)
	}
}

func TestValidateTektonPipelineParamsRejectsUnknownType(t *testing.T) {
	param := model.PipelineParam{
		Name:        "未知类型",
		Code:        "UNKNOWN",
		ParamType:   "unknownType",
		Mode:        "params",
		RuntimeMode: "params",
	}

	err := validatePipelineParams(engineTekton, []model.PipelineParam{param})
	if err == nil || !strings.Contains(err.Error(), "参数类型不支持") {
		t.Fatalf("tekton should reject unknown param type, got %v", err)
	}
}

func TestValidateTektonDagPipelineParamsRejectsDeprecatedWorkspaceParam(t *testing.T) {
	params := []model.PipelineParam{
		{
			Name:        "认证 Secret",
			Code:        "GIT_SECRET",
			ParamType:   channelvars.ParamKubernetesSecretName,
			Mode:        "params",
			RuntimeMode: "params",
		},
	}

	err := validateTektonDagPipelineParams(params)
	if err == nil || !strings.Contains(err.Error(), "Tekton DAG 参数类型不支持") {
		t.Fatalf("tekton dag should reject deprecated workspace param type, got %v", err)
	}
}

func TestTektonSecretWorkspaceUsesKubernetesSecretParam(t *testing.T) {
	taskYaml := `apiVersion: tekton.dev/v1
kind: Task
metadata:
  name: git-clone-java
spec:
  params:
    - name: url
      type: string
  workspaces:
    - name: output
    - name: basic-auth
      optional: true
  results:
    - name: commit
  steps:
    - name: clone
      image: alpine/git:latest
      script: git --version
`
	resource, err := devopstekton.ParseStepResource(taskYaml)
	if err != nil {
		t.Fatalf("parse tekton task: %v", err)
	}
	param := model.PipelineParam{
		Name:         "Git BasicAuth Secret",
		Code:         "GIT_BASIC_AUTH_SECRET",
		ParamType:    channelvars.ParamKubernetesSecretName,
		Mode:         "params",
		RuntimeMode:  "params",
		CurrentValue: "git-basic-auth",
		StepNodeID:   "clone",
		Config: model.StepParamConfig{
			MappingField: "basic-auth",
		},
	}
	pipeline := &model.DevopsPipeline{
		Code: "demo",
		Steps: []model.PipelineStep{
			{ID: "clone", StepID: "step-1", StepCode: "git-clone", Enabled: true, StageContent: taskYaml},
		},
		Params: []model.PipelineParam{param},
	}
	workspaces := tektonTaskWorkspaces(resource.Object, "git-clone", pipeline, []model.PipelineParam{param}, nil)
	secretWorkspace := tektonSecretWorkspaceName("git-clone", "basic-auth", param.Code)
	if len(workspaces) != 2 || workspaces[1]["workspace"] != secretWorkspace {
		t.Fatalf("optional secret workspace should be bound, got %#v", workspaces)
	}
	taskParams := tektonTaskParams([]model.PipelineParam{param})
	if len(taskParams) != 0 {
		t.Fatalf("workspace secret param should not be sent to task params, got %#v", taskParams)
	}

	runWorkspaces, err := tektonPipelineRunWorkspaces(pipeline, nil)
	if err != nil {
		t.Fatalf("render pipeline run workspaces: %v", err)
	}
	if len(runWorkspaces) != 2 {
		t.Fatalf("pipeline run should include source and secret workspaces, got %#v", runWorkspaces)
	}
	secret, ok := runWorkspaces[1]["secret"].(map[string]any)
	if !ok || secret["secretName"] != "git-basic-auth" {
		t.Fatalf("pipeline run secret workspace mismatch: %#v", runWorkspaces)
	}
}

func TestTektonRequiredWorkspacesUseSeparateEmptyDirByDefault(t *testing.T) {
	taskYaml := `apiVersion: tekton.dev/v1
kind: Task
metadata:
  name: maven-build
spec:
  workspaces:
    - name: source
    - name: maven-settings
  steps:
    - name: build
      image: maven:3.9-eclipse-temurin-17
      script: mvn -v
`
	resource, err := devopstekton.ParseStepResource(taskYaml)
	if err != nil {
		t.Fatalf("parse tekton task: %v", err)
	}
	pipeline := &model.DevopsPipeline{
		Code: "demo",
		Steps: []model.PipelineStep{
			{ID: "build", StepCode: "maven-build", Enabled: true, StageContent: taskYaml},
		},
	}
	workspaces := tektonTaskWorkspaces(resource.Object, "maven-build", pipeline, nil, nil)
	if len(workspaces) != 2 {
		t.Fatalf("required workspaces should be bound, got %#v", workspaces)
	}
	if workspaces[0]["workspace"] != "source" {
		t.Fatalf("source workspace should keep shared source binding, got %#v", workspaces)
	}
	expectedSettingsWorkspace := tektonDefaultPipelineWorkspaceName("maven-build", "maven-settings")
	if workspaces[1]["workspace"] != expectedSettingsWorkspace {
		t.Fatalf("non-source workspace should use isolated binding, got %#v", workspaces)
	}
	runWorkspaces, err := tektonPipelineRunWorkspaces(pipeline, nil)
	if err != nil {
		t.Fatalf("render pipeline run workspaces: %v", err)
	}
	if len(runWorkspaces) != 2 {
		t.Fatalf("pipeline run should include two emptyDir workspaces, got %#v", runWorkspaces)
	}
	if _, ok := runWorkspaces[1]["emptyDir"].(map[string]any); !ok {
		t.Fatalf("isolated required workspace should use emptyDir, got %#v", runWorkspaces)
	}
}

func TestTektonRequiredPVCWorkspaceRejectsEmptyBinding(t *testing.T) {
	taskYaml := `apiVersion: tekton.dev/v1
kind: Task
metadata:
  name: build
spec:
  workspaces:
    - name: cache
  steps:
    - name: build
      image: alpine:latest
      script: echo ok
`
	pipeline := &model.DevopsPipeline{
		Code: "demo",
		Steps: []model.PipelineStep{
			{ID: "build", StepCode: "build", Enabled: true, StageContent: taskYaml},
		},
		Params: []model.PipelineParam{
			{
				Name:        "缓存 PVC",
				Code:        "CACHE_PVC",
				ParamType:   channelvars.ParamKubernetesPVCName,
				Mode:        "params",
				RuntimeMode: "params",
				StepNodeID:  "build",
				Config: model.StepParamConfig{
					MappingField: "cache",
				},
			},
		},
	}
	_, err := tektonPipelineRunWorkspaces(pipeline, nil)
	if err == nil {
		t.Fatalf("required pvc workspace should reject empty binding")
	}
}

func TestTektonRequiredSecretWorkspaceUsesSecretBinding(t *testing.T) {
	taskYaml := `apiVersion: tekton.dev/v1
kind: Task
metadata:
  name: git-clone-java
spec:
  workspaces:
    - name: basic-auth
  steps:
    - name: clone
      image: alpine/git:latest
      script: git --version
`
	resource, err := devopstekton.ParseStepResource(taskYaml)
	if err != nil {
		t.Fatalf("parse tekton task: %v", err)
	}
	param := model.PipelineParam{
		Name:         "Git BasicAuth Secret",
		Code:         "GIT_BASIC_AUTH_SECRET",
		ParamType:    channelvars.ParamKubernetesSecretName,
		Mode:         "params",
		RuntimeMode:  "params",
		CurrentValue: "git-basic-auth",
		StepNodeID:   "clone",
		Config: model.StepParamConfig{
			MappingField: "basic-auth",
		},
	}
	pipeline := &model.DevopsPipeline{Code: "demo"}
	workspaces := tektonTaskWorkspaces(resource.Object, "git-clone", pipeline, []model.PipelineParam{param}, nil)
	secretWorkspace := tektonSecretWorkspaceName("git-clone", "basic-auth", param.Code)
	if len(workspaces) != 1 || workspaces[0]["workspace"] != secretWorkspace {
		t.Fatalf("required secret workspace should be bound to secret workspace, got %#v", workspaces)
	}
}

func TestTektonManagedSecretWorkspaceBindsWithoutExternalSecretName(t *testing.T) {
	taskYaml := `apiVersion: tekton.dev/v1
kind: Task
metadata:
  name: git-clone-java
spec:
  workspaces:
    - name: basic-auth
      optional: true
  steps:
    - name: clone
      image: alpine/git:latest
      script: git --version
`
	resource, err := devopstekton.ParseStepResource(taskYaml)
	if err != nil {
		t.Fatalf("parse tekton task: %v", err)
	}
	param := model.PipelineParam{
		Name:         "Git BasicAuth Secret",
		Code:         "GIT_BASIC_AUTH_SECRET",
		ParamType:    channelvars.ParamKubernetesSecretName,
		Mode:         "params",
		RuntimeMode:  "params",
		StepNodeID:   "clone",
		CurrentValue: "0123456789abcdef01234567",
		Config: model.StepParamConfig{
			MappingField: "basic-auth",
			VoucherModel: "username_password",
			Provider:     "basic-auth",
		},
	}
	pipeline := &model.DevopsPipeline{
		Code: "demo",
		Steps: []model.PipelineStep{
			{ID: "clone", StepID: "step-1", StepCode: "git-clone", Enabled: true, StageContent: taskYaml},
		},
		Params: []model.PipelineParam{param},
	}
	workspaces := tektonTaskWorkspaces(resource.Object, "git-clone", pipeline, []model.PipelineParam{param}, nil)
	secretWorkspace := tektonSecretWorkspaceName("git-clone", "basic-auth", param.Code)
	if len(workspaces) != 1 || workspaces[0]["workspace"] != secretWorkspace {
		t.Fatalf("managed secret workspace should be bound, got %#v", workspaces)
	}
	runWorkspaces, err := tektonPipelineRunWorkspaces(pipeline, nil)
	if err != nil {
		t.Fatalf("render pipeline run workspaces: %v", err)
	}
	if len(runWorkspaces) != 1 {
		t.Fatalf("pipeline run should include managed secret workspace, got %#v", runWorkspaces)
	}
	secret, ok := runWorkspaces[0]["secret"].(map[string]any)
	if !ok || secret["secretName"] == "" || secret["secretName"] == param.CurrentValue {
		t.Fatalf("managed secret should render generated k8s secret name, got %#v", runWorkspaces)
	}
}

func TestTektonPVCWorkspaceUsesPersistentVolumeClaim(t *testing.T) {
	taskYaml := `apiVersion: tekton.dev/v1
kind: Task
metadata:
  name: build
spec:
  workspaces:
    - name: cache
  steps:
    - name: build
      image: alpine:latest
      script: echo ok
`
	resource, err := devopstekton.ParseStepResource(taskYaml)
	if err != nil {
		t.Fatalf("parse tekton task: %v", err)
	}
	param := model.PipelineParam{
		Name:         "缓存 PVC",
		Code:         "CACHE_PVC",
		ParamType:    channelvars.ParamKubernetesPVCName,
		Mode:         "params",
		RuntimeMode:  "params",
		CurrentValue: "maven-cache",
		StepNodeID:   "build",
		Config: model.StepParamConfig{
			MappingField: "cache",
		},
	}
	pipeline := &model.DevopsPipeline{
		Code: "demo",
		Steps: []model.PipelineStep{
			{ID: "build", StepID: "step-1", StepCode: "build", Enabled: true, StageContent: taskYaml},
		},
		Params: []model.PipelineParam{param},
	}
	workspaces := tektonTaskWorkspaces(resource.Object, "build", pipeline, []model.PipelineParam{param}, nil)
	pvcWorkspace := tektonPVCWorkspaceName("build", "cache", param.Code)
	if len(workspaces) != 1 || workspaces[0]["workspace"] != pvcWorkspace {
		t.Fatalf("pvc workspace should be bound, got %#v", workspaces)
	}
	if taskParams := tektonTaskParams([]model.PipelineParam{param}); len(taskParams) != 0 {
		t.Fatalf("workspace pvc param should not be sent to task params, got %#v", taskParams)
	}
	runWorkspaces, err := tektonPipelineRunWorkspaces(pipeline, nil)
	if err != nil {
		t.Fatalf("render pipeline run workspaces: %v", err)
	}
	if len(runWorkspaces) != 1 {
		t.Fatalf("pipeline run should include pvc workspace, got %#v", runWorkspaces)
	}
	pvc, ok := runWorkspaces[0]["persistentVolumeClaim"].(map[string]any)
	if !ok || pvc["claimName"] != "maven-cache" {
		t.Fatalf("pipeline run pvc workspace mismatch: %#v", runWorkspaces)
	}
}

func TestTektonPipelineRunWorkspacesUsesNodeIDWithoutStepID(t *testing.T) {
	taskYaml := `apiVersion: tekton.dev/v1
kind: Task
metadata:
  name: build
spec:
  workspaces:
    - name: cache
  steps:
    - name: build
      image: alpine:latest
      script: echo ok
`
	param := model.PipelineParam{
		Name:         "缓存 PVC",
		Code:         "CACHE_PVC",
		ParamType:    channelvars.ParamKubernetesPVCName,
		Mode:         "params",
		RuntimeMode:  "params",
		CurrentValue: "maven-cache",
		StepNodeID:   "node-build",
		Config: model.StepParamConfig{
			MappingField: "cache",
		},
	}
	pipeline := &model.DevopsPipeline{
		Code: "demo",
		Steps: []model.PipelineStep{
			{ID: "node-build", StepCode: "build", Enabled: true, StageContent: taskYaml},
		},
		Params: []model.PipelineParam{param},
	}
	runWorkspaces, err := tektonPipelineRunWorkspaces(pipeline, nil)
	if err != nil {
		t.Fatalf("render pipeline run workspaces: %v", err)
	}
	if len(runWorkspaces) != 1 {
		t.Fatalf("pipeline run should include node workspace without stepId, got %#v", runWorkspaces)
	}
	pvc, ok := runWorkspaces[0]["persistentVolumeClaim"].(map[string]any)
	if !ok || pvc["claimName"] != "maven-cache" {
		t.Fatalf("pipeline run pvc workspace mismatch: %#v", runWorkspaces)
	}
}

func TestTektonVolumeClaimTemplateWorkspace(t *testing.T) {
	taskYaml := `apiVersion: tekton.dev/v1
kind: Task
metadata:
  name: build
spec:
  workspaces:
    - name: source
  steps:
    - name: build
      image: alpine:latest
      script: echo ok
`
	params := []model.PipelineParam{
		{
			Name:         "源码容量",
			Code:         "SOURCE_STORAGE",
			ParamType:    "string",
			Mode:         "params",
			RuntimeMode:  "params",
			CurrentValue: "5Gi",
			StepNodeID:   "build",
			Config: model.StepParamConfig{
				MappingField: "source",
				Provider:     "volumeClaimTemplate.storage",
			},
		},
		{
			Name:         "访问模式",
			Code:         "SOURCE_ACCESS_MODE",
			ParamType:    "choice",
			Mode:         "params",
			RuntimeMode:  "params",
			CurrentValue: "ReadWriteMany",
			StepNodeID:   "build",
			Config: model.StepParamConfig{
				MappingField: "source",
				Provider:     "volumeClaimTemplate.accessMode",
			},
		},
	}
	pipeline := &model.DevopsPipeline{
		Code: "demo",
		Steps: []model.PipelineStep{
			{ID: "build", StepID: "step-1", StepCode: "build", Enabled: true, StageContent: taskYaml},
		},
		Params: params,
	}
	runWorkspaces, err := tektonPipelineRunWorkspaces(pipeline, nil)
	if err != nil {
		t.Fatalf("render pipeline run workspaces: %v", err)
	}
	if len(runWorkspaces) != 1 {
		t.Fatalf("pipeline run should include dynamic pvc workspace, got %#v", runWorkspaces)
	}
	claim, ok := runWorkspaces[0]["volumeClaimTemplate"].(map[string]any)
	if !ok {
		t.Fatalf("pipeline run should render volumeClaimTemplate, got %#v", runWorkspaces)
	}
	spec := claim["spec"].(map[string]any)
	resources := spec["resources"].(map[string]any)
	requests := resources["requests"].(map[string]any)
	if requests["storage"] != "5Gi" {
		t.Fatalf("storage mismatch: %#v", runWorkspaces)
	}
}

func TestTektonOptionalVolumeClaimTemplateWorkspaceCanBeEmpty(t *testing.T) {
	taskYaml := `apiVersion: tekton.dev/v1
kind: Task
metadata:
  name: cache
spec:
  workspaces:
    - name: cache
      optional: true
  steps:
    - name: cache
      image: alpine:latest
      script: echo ok
`
	pipeline := &model.DevopsPipeline{
		Code: "demo",
		Steps: []model.PipelineStep{
			{ID: "cache", StepCode: "cache", Enabled: true, StageContent: taskYaml},
		},
		Params: []model.PipelineParam{
			{
				Name:        "缓存容量",
				Code:        "CACHE_STORAGE",
				ParamType:   "string",
				Mode:        "params",
				RuntimeMode: "params",
				StepNodeID:  "cache",
				Config: model.StepParamConfig{
					MappingField: "cache",
					Provider:     "volumeClaimTemplate.storage",
				},
			},
		},
	}
	runWorkspaces, err := tektonPipelineRunWorkspaces(pipeline, nil)
	if err != nil {
		t.Fatalf("optional dynamic pvc workspace should be skipped when empty: %v", err)
	}
	if len(runWorkspaces) != 0 {
		t.Fatalf("optional dynamic pvc workspace should be omitted when empty, got %#v", runWorkspaces)
	}
}

func TestTektonPipelineRunWorkspacesDoesNotDeduplicateEmptyOptionalBinding(t *testing.T) {
	optionalTaskYaml := `apiVersion: tekton.dev/v1
kind: Task
metadata:
  name: optional-cache
spec:
  workspaces:
    - name: cache
      optional: true
  steps:
    - name: optional-cache
      image: alpine:latest
      script: echo optional
`
	requiredTaskYaml := `apiVersion: tekton.dev/v1
kind: Task
metadata:
  name: required-cache
spec:
  workspaces:
    - name: cache
  steps:
    - name: required-cache
      image: alpine:latest
      script: echo required
`
	pipeline := &model.DevopsPipeline{
		Code: "demo",
		Steps: []model.PipelineStep{
			{ID: "optional", StepCode: "optional-cache", Enabled: true, SortOrder: 1, StageContent: optionalTaskYaml},
			{ID: "required", StepCode: "required-cache", Enabled: true, SortOrder: 2, StageContent: requiredTaskYaml},
		},
		Params: []model.PipelineParam{
			{
				Name:        "可选缓存",
				Code:        "OPTIONAL_CACHE_PVC",
				ParamType:   channelvars.ParamKubernetesPVCName,
				Mode:        "params",
				RuntimeMode: "params",
				StepNodeID:  "optional",
				Config: model.StepParamConfig{
					MappingField: "cache",
				},
			},
			{
				Name:         "必填缓存",
				Code:         "REQUIRED_CACHE_PVC",
				ParamType:    channelvars.ParamKubernetesPVCName,
				Mode:         "params",
				RuntimeMode:  "params",
				CurrentValue: "maven-cache",
				StepNodeID:   "required",
				Config: model.StepParamConfig{
					MappingField: "cache",
				},
			},
		},
	}
	runWorkspaces, err := tektonPipelineRunWorkspaces(pipeline, nil)
	if err != nil {
		t.Fatalf("render pipeline run workspaces: %v", err)
	}
	if len(runWorkspaces) != 1 {
		t.Fatalf("pipeline run should include later non-empty binding, got %#v", runWorkspaces)
	}
	pvc, ok := runWorkspaces[0]["persistentVolumeClaim"].(map[string]any)
	if !ok || pvc["claimName"] != "maven-cache" {
		t.Fatalf("pipeline run pvc workspace mismatch: %#v", runWorkspaces)
	}
}

func TestTektonPipelineRunWorkspacesHonorsEmptyRuntimeOverride(t *testing.T) {
	taskYaml := `apiVersion: tekton.dev/v1
kind: Task
metadata:
  name: optional-cache
spec:
  workspaces:
    - name: cache
      optional: true
  steps:
    - name: optional-cache
      image: alpine:latest
      script: echo optional
`
	pipeline := &model.DevopsPipeline{
		Code: "demo",
		Steps: []model.PipelineStep{
			{ID: "optional", StepCode: "optional-cache", Enabled: true, StageContent: taskYaml},
		},
		Params: []model.PipelineParam{
			{
				Name:         "可选缓存",
				Code:         "OPTIONAL_CACHE_PVC",
				ParamType:    channelvars.ParamKubernetesPVCName,
				Mode:         "params",
				RuntimeMode:  "params",
				DefaultValue: "maven-cache",
				StepNodeID:   "optional",
				Config: model.StepParamConfig{
					MappingField: "cache",
				},
			},
		},
	}
	runWorkspaces, err := tektonPipelineRunWorkspaces(pipeline, map[string]string{"OPTIONAL_CACHE_PVC": ""})
	if err != nil {
		t.Fatalf("render pipeline run workspaces: %v", err)
	}
	if len(runWorkspaces) != 0 {
		t.Fatalf("empty runtime override should omit optional pvc workspace, got %#v", runWorkspaces)
	}
}

func TestValidateReadonlyPipelineParamsAllowsMetadataChange(t *testing.T) {
	oldParams := []model.PipelineParam{
		{
			Name:          "旧名称",
			Code:          "APP_NAME",
			CurrentValue:  "demo",
			ParamType:     "string",
			Mode:          "env",
			Readonly:      true,
			Description:   "旧描述",
			SortOrder:     1,
			RuntimeConfig: false,
		},
	}
	newParams := []model.PipelineParam{
		{
			Name:          "新名称",
			Code:          "APP_NAME",
			CurrentValue:  "demo",
			ParamType:     "string",
			Mode:          "env",
			Readonly:      true,
			Description:   "新描述",
			SortOrder:     9,
			RuntimeConfig: true,
		},
	}

	if err := validateReadonlyPipelineParams(oldParams, newParams); err != nil {
		t.Fatalf("readonly metadata change should be allowed when value is unchanged: %v", err)
	}
}

func TestValidateReadonlyPipelineParamsAllowsRemovedParam(t *testing.T) {
	oldParams := []model.PipelineParam{
		{
			Name:         "Buildkit镜像",
			Code:         "DOCKER_BUILDKIT_IMAGE",
			CurrentValue: "harbor.ikubeops.local/public/buildkit:latest",
			ParamType:    "string",
			Mode:         "params",
			Readonly:     true,
		},
	}

	if err := validateReadonlyPipelineParams(oldParams, nil); err != nil {
		t.Fatalf("removed readonly param should be allowed when step is removed: %v", err)
	}
}

func TestValidateReadonlyPipelineParamsRejectsValueChange(t *testing.T) {
	oldParams := []model.PipelineParam{
		{
			Name:         "应用名称",
			Code:         "APP_NAME",
			CurrentValue: "demo",
			ParamType:    "string",
			Mode:         "env",
			Readonly:     true,
		},
	}
	newParams := []model.PipelineParam{
		{
			Name:         "应用名称",
			Code:         "APP_NAME",
			CurrentValue: "other",
			ParamType:    "string",
			Mode:         "env",
			Readonly:     true,
		},
	}

	err := validateReadonlyPipelineParams(oldParams, newParams)
	if err == nil || !strings.Contains(err.Error(), "只能查看") {
		t.Fatalf("readonly value change should be rejected, got %v", err)
	}
}

func TestNormalizePipelineStepBranchesDemotesSingleParallelBranch(t *testing.T) {
	steps := []model.PipelineStep{
		{ID: "clone", ParentNodeID: workflowStartNodeID, BranchType: "next", SortOrder: 1},
		{ID: "npm", ParentNodeID: "clone", BranchType: "parallel", SortOrder: 2},
		{ID: "build", ParentNodeID: "clone", BranchType: "next", SortOrder: 3},
	}

	normalizePipelineStepBranches(steps)
	for _, item := range steps {
		if item.ID == "npm" && item.BranchType != "next" {
			t.Fatalf("single parallel branch should be demoted to next, got %s", item.BranchType)
		}
	}
}

func TestNormalizePipelineStepBranchesKeepsParallelGroupAndLiftsContinuation(t *testing.T) {
	steps := []model.PipelineStep{
		{ID: "clone", ParentNodeID: workflowStartNodeID, BranchType: "next", SortOrder: 1},
		{ID: "sonar", ParentNodeID: "clone", BranchType: "parallel", SortOrder: 2},
		{ID: "npm", ParentNodeID: "clone", BranchType: "parallel", SortOrder: 3},
		{ID: "custom", ParentNodeID: "npm", BranchType: "next", SortOrder: 4},
	}

	normalizePipelineStepBranches(steps)
	byID := make(map[string]model.PipelineStep, len(steps))
	for _, item := range steps {
		byID[item.ID] = item
	}
	if byID["sonar"].BranchType != "parallel" || byID["npm"].BranchType != "parallel" {
		t.Fatalf("two parallel branches should keep parallel type: %#v", byID)
	}
	if byID["custom"].ParentNodeID != "clone" || byID["custom"].BranchType != "next" {
		t.Fatalf("parallel branch continuation should move after group, got %#v", byID["custom"])
	}
}

func TestValidateReadonlyPipelineParamsUsesStepNodeID(t *testing.T) {
	oldParams := []model.PipelineParam{
		{
			Name:         "Buildkit镜像",
			Code:         "DOCKER_BUILDKIT_IMAGE",
			SourceCode:   "DOCKER_BUILDKIT_IMAGE",
			StepNodeID:   "node-buildkit",
			CurrentValue: "harbor.ikubeops.local/public/buildkit:latest",
			ParamType:    "string",
			Mode:         "env",
			Readonly:     true,
		},
	}
	newParams := []model.PipelineParam{
		{
			Name:         "Buildkit镜像",
			Code:         "DOCKER_BUILDKIT_IMAGE",
			SourceCode:   "DOCKER_BUILDKIT_IMAGE",
			StepNodeID:   "node-buildkit",
			CurrentValue: "harbor.ikubeops.local/public/buildkit:latest",
			ParamType:    "string",
			Mode:         "env",
			Readonly:     true,
		},
		{
			Name:         "Buildkit镜像",
			Code:         "DOCKER_BUILDKIT_IMAGE_2",
			SourceCode:   "DOCKER_BUILDKIT_IMAGE",
			StepNodeID:   "node-buildkit-copy",
			CurrentValue: "harbor.ikubeops.local/public/buildkit:v2",
			ParamType:    "string",
			Mode:         "env",
			Readonly:     true,
		},
	}

	if err := validateReadonlyPipelineParams(oldParams, newParams); err != nil {
		t.Fatalf("readonly params with same source code in another step should be allowed: %v", err)
	}
}

func TestValidateReadonlyPipelineParamsKeepsLegacyCodeFallback(t *testing.T) {
	oldParams := []model.PipelineParam{
		{
			Name:         "Buildkit镜像",
			Code:         "DOCKER_BUILDKIT_IMAGE",
			CurrentValue: "harbor.ikubeops.local/public/buildkit:latest",
			ParamType:    "string",
			Mode:         "env",
			Readonly:     true,
		},
	}
	newParams := []model.PipelineParam{
		{
			Name:         "Buildkit镜像",
			Code:         "DOCKER_BUILDKIT_IMAGE",
			SourceCode:   "DOCKER_BUILDKIT_IMAGE",
			StepNodeID:   "node-buildkit",
			CurrentValue: "changed",
			ParamType:    "string",
			Mode:         "env",
			Readonly:     true,
		},
	}

	err := validateReadonlyPipelineParams(oldParams, newParams)
	if err == nil || !strings.Contains(err.Error(), "只能查看") {
		t.Fatalf("legacy readonly param value change should be rejected, got %v", err)
	}
}

func TestPipelineParamsMapIgnoresReadonlyRuntimeParamOverride(t *testing.T) {
	params := []model.PipelineParam{
		{
			Name:          "Buildkit镜像",
			Code:          "DOCKER_BUILDKIT_IMAGE",
			CurrentValue:  "harbor.ikubeops.local/public/buildkit:latest",
			ParamType:     "string",
			Mode:          "params",
			RuntimeMode:   "params",
			RuntimeConfig: true,
			Readonly:      true,
		},
	}

	got, err := pipelineParamsMap(params, `{"DOCKER_BUILDKIT_IMAGE":"other"}`)
	if err != nil {
		t.Fatalf("readonly runtime param override should be ignored: %v", err)
	}
	if got["DOCKER_BUILDKIT_IMAGE"] != "harbor.ikubeops.local/public/buildkit:latest" {
		t.Fatalf("readonly runtime param should keep configured value, got %q", got["DOCKER_BUILDKIT_IMAGE"])
	}
}

func TestPipelineParamsMapKeepsObjectListRuntimeParam(t *testing.T) {
	params := []model.PipelineParam{
		{
			Name:          "对象配置",
			Code:          "OBJECT_CONFIG",
			ParamType:     objectListParamType,
			Mode:          "params",
			RuntimeMode:   "params",
			RuntimeConfig: true,
			CurrentValue:  `{"image":"default"}`,
		},
	}

	got, err := pipelineParamsMap(params, `{"OBJECT_CONFIG":{"image":"runtime"}}`)
	if err != nil {
		t.Fatalf("objectList runtime param should be accepted: %v", err)
	}
	if got["OBJECT_CONFIG"] != `{"image":"runtime"}` {
		t.Fatalf("objectList runtime param should keep JSON value, got %q", got["OBJECT_CONFIG"])
	}
}

func TestResolveObjectListValueKeepsJSONString(t *testing.T) {
	param := model.PipelineParam{
		Name:         "对象配置",
		Code:         "OBJECT_CONFIG",
		ParamType:    objectListParamType,
		DefaultValue: `[{"name1":"value","name2":2}]`,
		Config: model.StepParamConfig{
			ValueMode: "string",
			CompoundFields: []model.CompoundParamField{
				{Name: "名称1", Code: "name1", ParamType: "string", Mode: "env"},
				{Name: "名称2", Code: "name2", ParamType: "number", Mode: "env"},
			},
		},
	}

	got, err := resolveObjectListValue(context.Background(), nil, nil, pipelineRenderContext{}, param)
	if err != nil {
		t.Fatalf("objectList json output should be accepted: %v", err)
	}
	if got != `[{"name1":"value","name2":2}]` {
		t.Fatalf("objectList json output = %s", got)
	}
}

func TestResolveObjectListValueEncodesBase64(t *testing.T) {
	param := model.PipelineParam{
		Name:         "对象配置",
		Code:         "OBJECT_CONFIG",
		ParamType:    objectListParamType,
		DefaultValue: `[{"name1":"value","name2":2}]`,
		Config: model.StepParamConfig{
			ValueMode: "base64",
			CompoundFields: []model.CompoundParamField{
				{Name: "名称1", Code: "name1", ParamType: "string", Mode: "env"},
				{Name: "名称2", Code: "name2", ParamType: "number", Mode: "env"},
			},
		},
	}

	got, err := resolveObjectListValue(context.Background(), nil, nil, pipelineRenderContext{}, param)
	if err != nil {
		t.Fatalf("objectList base64 output should be accepted: %v", err)
	}
	decoded, err := base64.StdEncoding.DecodeString(got)
	if err != nil {
		t.Fatalf("objectList base64 output should be valid: %v", err)
	}
	if string(decoded) != `[{"name1":"value","name2":2}]` {
		t.Fatalf("objectList base64 decoded = %s", decoded)
	}
}

func TestValidateObjectListJSONStringRejectsInvalidNumber(t *testing.T) {
	err := validateObjectListJSONString(
		"对象配置",
		`[{"name1":"value","name2":"abc"}]`,
		"",
		[]model.CompoundParamField{
			{Name: "名称2", Code: "name2", ParamType: "number", Mode: "env"},
		},
		false,
	)
	if err == nil || !strings.Contains(err.Error(), "必须是合法数字") {
		t.Fatalf("invalid objectList number field should be rejected, got %v", err)
	}
}

func TestValidateObjectListJSONStringRejectsMissingRequiredField(t *testing.T) {
	err := validateObjectListJSONString(
		"对象配置",
		`[{"name2":2}]`,
		"",
		[]model.CompoundParamField{
			{Name: "名称1", Code: "name1", ParamType: "string", Mode: "env", Required: true},
		},
		false,
	)
	if err == nil || !strings.Contains(err.Error(), "缺少必填字段") {
		t.Fatalf("missing required objectList field should be rejected, got %v", err)
	}
}

func TestValidatePipelineObjectListParamRejectsDuplicateFieldCode(t *testing.T) {
	param := model.PipelineParam{
		Name:        "对象配置",
		Code:        "OBJECT_CONFIG",
		ParamType:   objectListParamType,
		Mode:        "env",
		RuntimeMode: "env",
		Config: model.StepParamConfig{
			CompoundFields: []model.CompoundParamField{
				{Name: "名称1", Code: "name", ParamType: "string", Mode: "env"},
				{Name: "名称2", Code: "name", ParamType: "string", Mode: "env"},
			},
		},
	}

	err := validatePipelineObjectListParam(param)
	if err == nil || !strings.Contains(err.Error(), "字段编码不能重复") {
		t.Fatalf("duplicate objectList field code should be rejected, got %v", err)
	}
}

func TestTektonParamTypeTreatsObjectListAsString(t *testing.T) {
	if got := tektonParamType(objectListParamType); got != "string" {
		t.Fatalf("tekton objectList type = %s, want string", got)
	}
}

func TestShouldValidateRunDynamicParamsOnlyForTekton(t *testing.T) {
	cases := []struct {
		name     string
		pipeline *model.DevopsPipeline
		want     bool
	}{
		{name: "nil pipeline", pipeline: nil, want: false},
		{name: "jenkins pipeline", pipeline: &model.DevopsPipeline{EngineType: engineJenkins}, want: false},
		{name: "tekton pipeline", pipeline: &model.DevopsPipeline{EngineType: engineTekton}, want: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := shouldValidateRunDynamicParams(tc.pipeline); got != tc.want {
				t.Fatalf("shouldValidateRunDynamicParams=%v, want %v", got, tc.want)
			}
		})
	}
}

func TestRenderParamsKeepsNumberParamType(t *testing.T) {
	params := []model.PipelineParam{
		{
			Name:         "超时时间",
			Code:         "BASH_TIMEOUT_MINUTES",
			DefaultValue: "30",
			ParamType:    "number",
			Mode:         "params",
			RuntimeMode:  "params",
		},
	}
	rendered := renderParams(params)
	if len(rendered) != 1 || rendered[0].ParamType != "number" {
		t.Fatalf("render params should keep platform number type, got %#v", rendered)
	}
}

func TestRebuildPipelineParamsIsolatesDuplicateStepVariables(t *testing.T) {
	steps := []model.PipelineStep{
		{ID: "node-1", StepName: "自定义脚本", ParamValues: `{"BASH_COMMANDS":"echo one"}`},
		{ID: "node-2", StepName: "自定义脚本-2", ParamValues: `{"BASH_COMMANDS":"echo two"}`},
	}
	stepParams := []*pipelineconfigservice.DevopsStepParam{
		{Name: "命令", Code: "BASH_COMMANDS", ParamType: "text", Mode: "params", RuntimeMode: "params"},
	}

	params := rebuildPipelineParamsFromSteps(steps, map[string][]*pipelineconfigservice.DevopsStepParam{
		"node-1": stepParams,
		"node-2": stepParams,
	}, nil)

	if len(params) != 2 {
		t.Fatalf("expected two params, got %#v", params)
	}
	if params[0].Code != "BASH_COMMANDS" || params[0].SourceCode != "BASH_COMMANDS" || params[0].StepNodeID != "node-1" {
		t.Fatalf("first param mismatch: %#v", params[0])
	}
	if params[1].Code != "BASH_COMMANDS_2" || params[1].SourceCode != "BASH_COMMANDS" || params[1].StepNodeID != "node-2" {
		t.Fatalf("second param mismatch: %#v", params[1])
	}
	if params[0].CurrentValue != "echo one" || params[1].CurrentValue != "echo two" {
		t.Fatalf("param values should stay with their step instance: %#v", params)
	}
}

func TestRebuildPipelineParamsRemapsDynamicDependencies(t *testing.T) {
	steps := []model.PipelineStep{
		{ID: "node-1", StepName: "代码拉取"},
		{ID: "node-2", StepName: "代码拉取-2"},
	}
	stepParams := []*pipelineconfigservice.DevopsStepParam{
		{Name: "仓库", Code: "GIT_REPO", ParamType: channelvars.ParamGitProject, Mode: "paas", RuntimeMode: "env"},
		{
			Name:        "分支",
			Code:        "GIT_BRANCH",
			ParamType:   channelvars.ParamGitBranch,
			Mode:        "paas",
			RuntimeMode: "params",
			Config: &pipelineconfigservice.StepParamConfig{
				ProjectParamCode: "GIT_REPO",
			},
		},
	}

	params := rebuildPipelineParamsFromSteps(steps, map[string][]*pipelineconfigservice.DevopsStepParam{
		"node-1": stepParams,
		"node-2": stepParams,
	}, nil)

	var secondBranch model.PipelineParam
	for _, item := range params {
		if item.StepNodeID == "node-2" && item.SourceCode == "GIT_BRANCH" {
			secondBranch = item
			break
		}
	}
	if secondBranch.Code != "GIT_BRANCH_2" || secondBranch.Config.ProjectParamCode != "GIT_REPO_2" {
		t.Fatalf("second branch dependency should be remapped, got %#v", secondBranch)
	}
}

func TestRenderStepsFallsBackToLegacyParamOrder(t *testing.T) {
	steps := []model.PipelineStep{
		{ID: "node-1", NodeName: "自定义命令", StageContent: "steps { script { echo params.BASH_COMMANDS } }"},
		{ID: "node-2", NodeName: "自定义命令-1", StageContent: "steps { script { echo params.BASH_COMMANDS } }"},
	}
	params := []model.PipelineParam{
		{Code: "BASH_COMMANDS", SourceCode: "BASH_COMMANDS", StepNodeID: "node-1"},
		{Code: "BASH_COMMANDS_2"},
	}
	codeMaps := pipelineStepParamCodeMaps(params)
	sourceCodes := map[string]struct{}{"BASH_COMMANDS": {}}
	paramsBySource := pipelineParamsBySourceForRender(params, sourceCodes)
	usedBySource := map[string]map[string]struct{}{
		"BASH_COMMANDS": {"BASH_COMMANDS": {}},
	}
	if _, ok := codeMaps["node-2"]; !ok {
		codeMaps["node-2"] = make(map[string]string)
	}
	codeMaps["node-2"]["BASH_COMMANDS"] = nextRenderParamCode("BASH_COMMANDS", paramsBySource["BASH_COMMANDS"], usedBySource)

	rendered := renderSteps(steps, codeMaps, false)

	if rendered[1].ParamCodeMap["BASH_COMMANDS"] != "BASH_COMMANDS_2" {
		t.Fatalf("legacy params should fallback to second runtime code, got %#v", rendered[1].ParamCodeMap)
	}
}

func TestResolvePlatformParamValueRejectsCredentialPlainText(t *testing.T) {
	pipeline := &model.DevopsPipeline{Name: "前端流水线-A", ProjectID: "project-id"}
	item := model.PipelineParam{
		Name:         "认证token",
		Code:         "GIT_HTTP_REPO_PASSWORD",
		ParamType:    "voucherModel",
		Mode:         "paas",
		RuntimeMode:  "env",
		CurrentValue: "glpat-plain-token",
		Config: model.StepParamConfig{
			VoucherModel: "token",
			MappingField: "token",
		},
	}

	_, err := resolvePlatformParamValue(nil, nil, pipeline, item, pipelineRenderContext{})
	if err == nil || !strings.Contains(err.Error(), "凭证参数必须选择平台凭证") {
		t.Fatalf("plain credential value should be rejected, got %v", err)
	}
}

func TestNormalizeChannelCredentialModeSupportsJSONModes(t *testing.T) {
	if got := normalizeChannelCredentialMode("jsonData", ""); got != credentialModeJSONData {
		t.Fatalf("jsonData mode should be kept, got %s", got)
	}
	if got := normalizeChannelCredentialMode("jsonBase64", ""); got != credentialModeJSONBase64 {
		t.Fatalf("jsonBase64 mode should be kept, got %s", got)
	}
	if got := normalizeChannelCredentialMode("", "jsonData"); got != credentialModeJSONData {
		t.Fatalf("legacy jsonData mapping field should map to mode, got %s", got)
	}
	if got := normalizeCredentialMode("jsonData", ""); got != credentialModeJSONData {
		t.Fatalf("platform credential should keep jsonData mode, got %s", got)
	}
	if got := normalizeCredentialMode("", "jsonBase64"); got != credentialModeJSONBase64 {
		t.Fatalf("legacy platform credential jsonBase64 mapping field should map to mode, got %s", got)
	}
}

func TestCanValidateDynamicParamWithRepositoryURL(t *testing.T) {
	if !canValidateDynamicParamWithRepositoryURL(model.PipelineParam{ParamType: channelvars.ParamGitTag}, "https://git.example.com/group/repo.git") {
		t.Fatal("git tag should allow repository url validation")
	}
	if canValidateDynamicParamWithRepositoryURL(model.PipelineParam{ParamType: channelvars.ParamGitProject}, "https://git.example.com/group/repo.git") {
		t.Fatal("git project should not use repository url validation")
	}
}

func TestMatchScanItemForArtifactRequiresReportPath(t *testing.T) {
	items := []model.ScanPlanItem{
		{StepID: "scan", Tool: "trivy", ReportFormat: "json"},
		{StepID: "scan", Tool: "spotbugs", ReportFormat: "xml", ReportPath: "reports/spotbugs.xml"},
	}
	if got := matchScanItemForArtifact(items, "scan", "reports/trivy.json"); got != nil {
		t.Fatalf("scan item without report path should not match artifact, got %#v", got)
	}
	if got := matchScanItemForArtifact(items, "scan", "reports/spotbugs.xml"); got == nil || got.Tool != "spotbugs" {
		t.Fatalf("expected spotbugs item matched, got %#v", got)
	}
}

func TestValidatePipelineScanItems(t *testing.T) {
	cases := []struct {
		name    string
		enabled bool
		items   []model.ScanPlanItem
		wantErr string
	}{
		{name: "disabled allows empty", enabled: false},
		{name: "enabled requires items", enabled: true, wantErr: "扫描计划不能为空"},
		{
			name:    "service requires binding",
			enabled: true,
			items:   []model.ScanPlanItem{{Tool: "sonarqube", ToolMode: "service_api", ReportFormat: "json", TargetName: "app"}},
			wantErr: "服务型扫描工具必须绑定项目渠道",
		},
		{
			name:    "service api requires target",
			enabled: true,
			items:   []model.ScanPlanItem{{Tool: "sonarqube", ToolMode: "service_api", ToolBindingID: "binding-id", ReportFormat: "json"}},
			wantErr: "服务 API 型扫描工具必须配置目标或目标参数",
		},
		{
			name:    "local rejects binding",
			enabled: true,
			items:   []model.ScanPlanItem{{Tool: "spotbugs", ToolMode: "local", ToolBindingID: "binding-id", ReportFormat: "xml"}},
			wantErr: "本地扫描工具不能绑定项目渠道",
		},
		{
			name:    "local valid",
			enabled: true,
			items:   []model.ScanPlanItem{{Tool: "trivy", ToolMode: "local", ReportFormat: "json"}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validatePipelineScanItems(tc.enabled, tc.items)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("expected %q, got %v", tc.wantErr, err)
			}
		})
	}
}
