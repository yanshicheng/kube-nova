package pipelineconfigservicelogic

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/devops/channelvars"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestValidateTemplateStepRequiredParamsRejectsInvalidOptionalObjectList(t *testing.T) {
	step := model.PipelineTemplateStep{
		NodeName:    "多仓库克隆",
		ParamValues: `{"REPOS_JSON":{"repoName":"app-service"}}`,
	}
	params := []model.DevopsStepParam{
		{
			Name:      "仓库列表",
			Code:      "REPOS_JSON",
			ParamType: objectListParamType,
			Mode:      "env",
			Config: model.DevopsStepParamConfig{
				CompoundFields: []model.DevopsCompoundParamField{
					{Name: "仓库名称", Code: "repoName", ParamType: "string", Mode: "params"},
				},
			},
		},
	}

	err := validateTemplateStepRequiredParams(step, params)
	if err == nil || !strings.Contains(err.Error(), "必须是对象数组") {
		t.Fatalf("optional objectList with object value should be rejected, got %v", err)
	}
}

func TestCredentialAddressSourceGroupIncludesChannelVariableAddressTypes(t *testing.T) {
	cases := []struct {
		sourceType string
		groupCode  string
		types      []string
	}{
		{sourceType: channelvars.ParamRegistryImageTagURL, groupCode: channelvars.GroupImageRepo, types: []string{"registry", "aliyun_registry"}},
		{sourceType: channelvars.ParamNexusArtifactURL, groupCode: channelvars.GroupArtifactRepo, types: []string{"nexus"}},
		{sourceType: channelvars.ParamNexusArtifactVersionURL, groupCode: channelvars.GroupArtifactRepo, types: []string{"nexus"}},
		{sourceType: channelvars.ParamJfrogArtifactURL, groupCode: channelvars.GroupArtifactRepo, types: []string{"jfrog"}},
		{sourceType: channelvars.ParamJfrogArtifactVersionURL, groupCode: channelvars.GroupArtifactRepo, types: []string{"jfrog"}},
		{sourceType: channelvars.ParamSonarProjectKeyURL, groupCode: channelvars.GroupCodeScan, types: []string{"sonarqube"}},
		{sourceType: channelvars.ParamHostConfig, groupCode: channelvars.GroupDeployTarget, types: []string{"host"}},
		{sourceType: channelvars.ParamHostGroupConfig, groupCode: channelvars.GroupDeployTarget, types: []string{"host_group"}},
	}

	for _, tt := range cases {
		groupCode, channelTypes, ok := credentialAddressSourceGroup(tt.sourceType)
		if !ok {
			t.Fatalf("%s should be supported as credential source address", tt.sourceType)
		}
		if groupCode != tt.groupCode {
			t.Fatalf("%s group = %s, want %s", tt.sourceType, groupCode, tt.groupCode)
		}
		if strings.Join(channelTypes, ",") != strings.Join(tt.types, ",") {
			t.Fatalf("%s channel types = %v, want %v", tt.sourceType, channelTypes, tt.types)
		}
	}
}

func TestHostAddressMatchesEndpoint(t *testing.T) {
	cases := []struct {
		name     string
		address  string
		endpoint string
		want     bool
	}{
		{name: "json object host", address: `{"host":"192.168.1.1","port":22}`, endpoint: "192.168.1.1", want: true},
		{name: "json array host", address: `[{"host":"192.168.1.1","port":22}]`, endpoint: "192.168.1.1:22", want: true},
		{name: "ansible inventory", address: "[all]\n192.168.1.1 ansible_ssh_host=192.168.1.1 ansible_ssh_port=22", endpoint: "192.168.1.1", want: true},
		{name: "host mismatch", address: `{"host":"192.168.1.1","port":22}`, endpoint: "192.168.1.2", want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := credentialAddressMatchesEndpoint(channelvars.GroupDeployTarget, tc.address, tc.endpoint); got != tc.want {
				t.Fatalf("host address match = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestRenderHostSingleValueUsesJSONObject(t *testing.T) {
	target := hostGroupTarget{
		Host:     "192.168.1.1",
		Port:     22,
		Username: "root",
		Password: "password",
	}
	value, err := renderHostSingleValue(target, "json")
	if err != nil {
		t.Fatalf("render host single value: %v", err)
	}
	if strings.HasPrefix(strings.TrimSpace(value), "[") {
		t.Fatalf("single host config should be json object, got %s", value)
	}
	var got hostGroupTarget
	if err := json.Unmarshal([]byte(value), &got); err != nil {
		t.Fatalf("unmarshal host config: %v", err)
	}
	if got.Host != target.Host || got.Port != target.Port || got.Username != target.Username || got.Password != target.Password {
		t.Fatalf("host config = %#v, want %#v", got, target)
	}
	encoded, err := renderHostSingleValue(target, "jsonBase64")
	if err != nil {
		t.Fatalf("render host single value base64: %v", err)
	}
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("decode host config base64: %v", err)
	}
	if strings.TrimSpace(string(decoded)) != strings.TrimSpace(value) {
		t.Fatalf("decoded host config mismatch")
	}
}

func TestImageRegistryAddressMatchesEndpoint(t *testing.T) {
	cases := []struct {
		name     string
		address  string
		endpoint string
		want     bool
	}{
		{name: "no scheme", address: "registry.cn-hangzhou.aliyuncs.com/ikubeops/nginx:v1", endpoint: "https://registry.cn-hangzhou.aliyuncs.com", want: true},
		{name: "endpoint path", address: "https://harbor.ikubeops.local/registry/public/nginx:v1", endpoint: "https://harbor.ikubeops.local/registry", want: true},
		{name: "same port", address: "https://harbor.ikubeops.local:30081/public/nginx:v1", endpoint: "https://harbor.ikubeops.local:30081", want: true},
		{name: "different port", address: "https://harbor.ikubeops.local:30081/public/nginx:v1", endpoint: "https://harbor.ikubeops.local:30082", want: false},
		{name: "host mismatch", address: "https://harbor-a.local/public/nginx:v1", endpoint: "https://harbor-b.local", want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := credentialAddressMatchesEndpoint(channelvars.GroupImageRepo, tc.address, tc.endpoint); got != tc.want {
				t.Fatalf("image registry address match = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestRepositoryAddressMatchesEndpoint(t *testing.T) {
	cases := []struct {
		name     string
		address  string
		endpoint string
		want     bool
	}{
		{name: "http with credential", address: "http://git:token@172.16.1.21:30081/ikubeops/test-vue.git", endpoint: "http://172.16.1.21:30081", want: true},
		{name: "ssh url", address: "ssh://git@172.16.1.21:30081/ikubeops/test-vue.git", endpoint: "http://172.16.1.21:30081", want: true},
		{name: "scp like", address: "git@172.16.1.21:ikubeops/test-vue.git", endpoint: "http://172.16.1.21", want: true},
		{name: "endpoint path", address: "ssh://git@git.example.com/scm/ikubeops/test-vue.git", endpoint: "https://git.example.com/scm", want: true},
		{name: "different port", address: "http://172.16.1.21:30081/ikubeops/test-vue.git", endpoint: "http://172.16.1.21:30082", want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := credentialAddressMatchesEndpoint(channelvars.GroupCodeRepo, tc.address, tc.endpoint); got != tc.want {
				t.Fatalf("repository address match = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestArtifactRepositoryAddressMatchesEndpoint(t *testing.T) {
	cases := []struct {
		name     string
		address  string
		endpoint string
		want     bool
	}{
		{name: "same port", address: "https://nexus.example.com:8081/repository/npm-group/pkg:v1", endpoint: "https://nexus.example.com:8081/repository", want: true},
		{name: "different port", address: "https://nexus.example.com:8081/repository/npm-group/pkg:v1", endpoint: "https://nexus.example.com:8082/repository", want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := credentialAddressMatchesEndpoint(channelvars.GroupArtifactRepo, tc.address, tc.endpoint); got != tc.want {
				t.Fatalf("artifact address match = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestImageRegistryMappingFieldsUseWhitelist(t *testing.T) {
	fields := filterImageRegistryMappingFields([]channelvars.VariableField{
		{Field: channelvars.FieldChannelID},
		{Field: channelvars.FieldChannelName},
		{Field: channelvars.FieldChannelCode},
		{Field: channelvars.FieldEndpoint},
		{Field: "config.project"},
		{Field: channelvars.FieldDynamicProject},
		{Field: channelvars.FieldDynamicImage},
		{Field: channelvars.FieldDynamicTag},
		{Field: channelvars.FieldDynamicImageTag},
		{Field: channelvars.FieldAddressProjectURL},
		{Field: channelvars.FieldAddressImageURL},
		{Field: channelvars.FieldAddressImageTagURL},
		{Field: channelvars.FieldAddressRegistryURL},
	})
	got := make([]string, 0, len(fields))
	for _, field := range fields {
		got = append(got, field.Field)
	}
	want := []string{
		channelvars.FieldChannelID,
		channelvars.FieldChannelName,
		channelvars.FieldChannelCode,
		channelvars.FieldEndpoint,
		channelvars.FieldDynamicProject,
		channelvars.FieldDynamicImage,
		channelvars.FieldDynamicTag,
		channelvars.FieldDynamicImageTag,
		channelvars.FieldAddressProjectURL,
		channelvars.FieldAddressImageURL,
		channelvars.FieldAddressImageTagURL,
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("image registry mapping fields = %v, want %v", got, want)
	}
}

func TestCodeScanMappingFieldsUseWhitelist(t *testing.T) {
	fields := filterCodeScanMappingFields([]channelvars.VariableField{
		{Field: channelvars.FieldChannelID},
		{Field: channelvars.FieldChannelName},
		{Field: channelvars.FieldChannelCode},
		{Field: channelvars.FieldEndpoint},
		{Field: channelvars.FieldDynamicProjectName},
		{Field: channelvars.FieldDynamicProjectKey},
		{Field: channelvars.FieldDynamicProject},
		{Field: channelvars.FieldAddressProjectURL},
	})
	got := make([]string, 0, len(fields))
	for _, field := range fields {
		got = append(got, field.Field)
	}
	want := []string{
		channelvars.FieldChannelID,
		channelvars.FieldChannelName,
		channelvars.FieldChannelCode,
		channelvars.FieldEndpoint,
		channelvars.FieldDynamicProjectName,
		channelvars.FieldDynamicProjectKey,
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("code scan mapping fields = %v, want %v", got, want)
	}
}

func TestArtifactRepositoryMappingFieldsUseWhitelist(t *testing.T) {
	fields := filterArtifactRepositoryMappingFields([]channelvars.VariableField{
		{Field: channelvars.FieldChannelID},
		{Field: channelvars.FieldChannelName},
		{Field: channelvars.FieldChannelCode},
		{Field: channelvars.FieldEndpoint},
		{Field: channelvars.FieldDynamicRepository},
		{Field: channelvars.FieldDynamicArtifactName},
		{Field: channelvars.FieldDynamicArtifactVersion},
		{Field: channelvars.FieldAddressRepositoryURL},
		{Field: channelvars.FieldAddressArtifactURL},
		{Field: channelvars.FieldAddressArtifactVersionURL},
		{Field: channelvars.FieldDynamicArtifact},
		{Field: channelvars.FieldDynamicArtifactTag},
		{Field: channelvars.FieldAddressArtifactTagURL},
	})
	got := make([]string, 0, len(fields))
	for _, field := range fields {
		got = append(got, field.Field)
	}
	want := []string{
		channelvars.FieldChannelID,
		channelvars.FieldChannelName,
		channelvars.FieldChannelCode,
		channelvars.FieldEndpoint,
		channelvars.FieldDynamicRepository,
		channelvars.FieldDynamicArtifactName,
		channelvars.FieldDynamicArtifactVersion,
		channelvars.FieldAddressRepositoryURL,
		channelvars.FieldAddressArtifactURL,
		channelvars.FieldAddressArtifactVersionURL,
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("artifact repository mapping fields = %v, want %v", got, want)
	}
}

func TestDeployTargetHostMappingFieldsUseWhitelist(t *testing.T) {
	fields := filterDeployTargetMappingFields("host", []channelvars.VariableField{
		{Field: channelvars.FieldChannelID},
		{Field: channelvars.FieldChannelName},
		{Field: channelvars.FieldChannelCode},
		{Field: channelvars.FieldEndpoint},
		{Field: channelvars.FieldAddressHostConfig},
		{Field: channelvars.FieldDynamicHostGroup},
		{Field: "config.hostIds"},
	})
	got := make([]string, 0, len(fields))
	for _, field := range fields {
		got = append(got, field.Field)
	}
	want := []string{
		channelvars.FieldChannelID,
		channelvars.FieldChannelName,
		channelvars.FieldChannelCode,
		channelvars.FieldEndpoint,
		channelvars.FieldAddressHostConfig,
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("host mapping fields = %v, want %v", got, want)
	}
}

func TestNormalizeImageRegistryMappingFieldByGroupOrType(t *testing.T) {
	cases := []struct {
		name        string
		groupCode   string
		channelType string
		field       string
		want        string
	}{
		{
			name:      "image group legacy registry",
			groupCode: channelvars.GroupImageRepo,
			field:     channelvars.FieldDynamicRegistry,
			want:      channelvars.FieldDynamicProject,
		},
		{
			name:      "image group legacy repository",
			groupCode: channelvars.GroupImageRepo,
			field:     channelvars.FieldDynamicRepository,
			want:      channelvars.FieldDynamicProject,
		},
		{
			name:        "image type legacy address",
			channelType: "harbor",
			field:       channelvars.FieldAddressRegistryURL,
			want:        channelvars.FieldAddressProjectURL,
		},
		{
			name:      "artifact keeps repository",
			groupCode: channelvars.GroupArtifactRepo,
			field:     channelvars.FieldDynamicRepository,
			want:      channelvars.FieldDynamicRepository,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := normalizeChannelVariableMappingField(tc.groupCode, tc.channelType, tc.field)
			if got != tc.want {
				t.Fatalf("mapping field = %s, want %s", got, tc.want)
			}
		})
	}
}

func TestShouldQueryChannelBindingOptionsSkipsDynamicAndAddressTypes(t *testing.T) {
	channelvars.SetChannelParamTypes([]channelvars.StepChannelParamSpec{
		{ParamType: channelvars.ParamGitRepositoryURL, GroupCode: channelvars.GroupCodeRepo},
		{ParamType: channelvars.ParamGitProject, GroupCode: channelvars.GroupCodeRepo},
	})
	t.Cleanup(func() {
		channelvars.SetChannelParamTypes(nil)
	})

	if !shouldQueryChannelBindingOptions(channelvars.ParamRepositoryChannel) {
		t.Fatal("repositoryChannel should query channel bindings")
	}
	if shouldQueryChannelBindingOptions(channelvars.ParamGitRepositoryURL) {
		t.Fatal("gitRepositoryUrl should query Git projects, not channel bindings")
	}
	if shouldQueryChannelBindingOptions(channelvars.ParamGitProject) {
		t.Fatal("gitProject should query Git projects, not channel bindings")
	}
}

func TestValidatePostConditionsRejectsDuplicateCondition(t *testing.T) {
	err := validatePostConditions("success { echo '1' }\nsuccess { echo '2' }")
	if err == nil || !strings.Contains(err.Error(), "post 条件 success 只能配置一次") {
		t.Fatalf("duplicate success post condition should be rejected, got %v", err)
	}
}

func TestValidateObjectListParamRejectsParamsModeField(t *testing.T) {
	param := model.DevopsStepParam{
		Name:        "仓库列表",
		Code:        "REPOS_JSON",
		ParamType:   objectListParamType,
		Mode:        "env",
		RuntimeMode: "env",
		Config: model.DevopsStepParamConfig{
			CompoundFields: []model.DevopsCompoundParamField{
				{Name: "仓库名称", Code: "repoName", ParamType: "string", Mode: "params"},
			},
		},
	}

	err := validateObjectListParam(param)
	if err == nil || !strings.Contains(err.Error(), "只支持环境变量或平台变量") {
		t.Fatalf("objectList field params mode should be rejected, got %v", err)
	}
}

func TestValidateObjectListParamRejectsParamsRuntimeField(t *testing.T) {
	param := model.DevopsStepParam{
		Name:        "仓库列表",
		Code:        "REPOS_JSON",
		ParamType:   objectListParamType,
		Mode:        "env",
		RuntimeMode: "env",
		Config: model.DevopsStepParamConfig{
			CompoundFields: []model.DevopsCompoundParamField{
				{
					Name:        "仓库地址",
					Code:        "repoUrl",
					ParamType:   channelvars.ParamGitProject,
					Mode:        "paas",
					RuntimeMode: "params",
					Config: model.DevopsStepParamConfig{
						ChannelBindingID: "binding-id",
					},
				},
			},
		},
	}

	err := validateObjectListParam(param)
	if err == nil || !strings.Contains(err.Error(), "不支持执行变量映射") {
		t.Fatalf("objectList field params runtime should be rejected, got %v", err)
	}
}

func TestValidateObjectListParamAllowsPlatformMode(t *testing.T) {
	param := model.DevopsStepParam{
		Name:        "仓库列表",
		Code:        "REPOS_JSON",
		ParamType:   objectListParamType,
		Mode:        "paas",
		RuntimeMode: "params",
		Config: model.DevopsStepParamConfig{
			CompoundFields: []model.DevopsCompoundParamField{
				{Name: "仓库名称", Code: "repoName", ParamType: "string", Mode: "env"},
			},
		},
	}

	if err := validateObjectListParam(param); err != nil {
		t.Fatalf("platform objectList should be accepted: %v", err)
	}
}

func TestValidateTektonStepContentPreservesCommentsAndClearsNamespace(t *testing.T) {
	content := `apiVersion: tekton.dev/v1
kind: Task
metadata:
  # 名称保存时以步骤编码为准
  name: wrong-name
  namespace: user-input
spec:
  # 参数由平台参数映射到 Task params
  params:
    - name: url
      type: string
  steps:
    - name: clone
      image: alpine/git:latest
      script: |
        #!/usr/bin/env sh
        echo "$(params.url)"
`

	got, err := validateTektonStepContent(content, "git-clone")
	if err != nil {
		t.Fatalf("validateTektonStepContent() error = %v", err)
	}
	if !strings.Contains(got, "# 名称保存时以步骤编码为准") {
		t.Fatalf("normalized yaml should preserve metadata comment:\n%s", got)
	}
	if !strings.Contains(got, "# 参数由平台参数映射到 Task params") {
		t.Fatalf("normalized yaml should preserve spec comment:\n%s", got)
	}
	if !strings.Contains(got, "name: git-clone") {
		t.Fatalf("metadata.name should follow step code:\n%s", got)
	}
	if strings.Contains(got, "namespace:") {
		t.Fatalf("metadata.namespace should not be stored:\n%s", got)
	}
}

func TestValidateTektonPipelineTemplateConfigAllowsFinallyDesignEdge(t *testing.T) {
	content := `{
		"version": "tekton-dag/v1",
		"pipeline": {"name": "build-demo"},
		"nodes": [
			{"id": "build", "name": "build", "taskRef": {"kind": "ClusterTask", "name": "buildah"}},
			{"id": "notify", "type": "finally", "name": "notify", "taskRef": {"kind": "ClusterTask", "name": "send-message"}}
		],
		"edges": [
			{"source": "build", "target": "notify", "type": "finally"}
		]
	}`

	if _, err := validateTektonPipelineTemplateConfig(nil, nil, content, []string{"SUPER_ADMIN"}); err != nil {
		t.Fatalf("finally design edge should be accepted: %v", err)
	}
}

func TestValidateTektonPipelineTemplateConfigRejectsOnlyFinallyNodes(t *testing.T) {
	content := `{
		"version": "tekton-dag/v1",
		"pipeline": {"name": "build-demo"},
		"nodes": [
			{"id": "notify", "type": "finally", "name": "notify", "taskRef": {"kind": "ClusterTask", "name": "send-message"}}
		]
	}`

	if _, err := validateTektonPipelineTemplateConfig(nil, nil, content, []string{"SUPER_ADMIN"}); err == nil ||
		!strings.Contains(err.Error(), "至少需要一个启用 Task") {
		t.Fatalf("template with only finally nodes should be rejected, got %v", err)
	}
}

func TestValidateTektonPipelineTemplateConfigRejectsIncompleteResultEdge(t *testing.T) {
	content := `{
		"version": "tekton-dag/v1",
		"pipeline": {"name": "build-demo"},
		"nodes": [
			{"id": "build", "name": "build", "taskRef": {"kind": "ClusterTask", "name": "buildah"}, "results": ["image"]},
			{"id": "deploy", "name": "deploy", "taskRef": {"kind": "ClusterTask", "name": "kubectl"}}
		],
		"edges": [
			{"source": "build", "target": "deploy", "type": "result", "resultName": "image"}
		]
	}`

	if _, err := validateTektonPipelineTemplateConfig(nil, nil, content, []string{"SUPER_ADMIN"}); err == nil ||
		!strings.Contains(err.Error(), "Result 边") {
		t.Fatalf("incomplete result edge should be rejected, got %v", err)
	}
}

func TestValidateTektonPipelineTemplateConfigRejectsIncompleteWhenEdge(t *testing.T) {
	content := `{
		"version": "tekton-dag/v1",
		"pipeline": {"name": "build-demo"},
		"nodes": [
			{"id": "build", "name": "build", "taskRef": {"kind": "ClusterTask", "name": "buildah"}},
			{"id": "deploy", "name": "deploy", "taskRef": {"kind": "ClusterTask", "name": "kubectl"}}
		],
		"edges": [
			{"source": "build", "target": "deploy", "type": "when", "whenInput": "$(params.deploy)", "whenOperator": "equals", "whenValues": ["true"]}
		]
	}`

	if _, err := validateTektonPipelineTemplateConfig(nil, nil, content, []string{"SUPER_ADMIN"}); err == nil ||
		!strings.Contains(err.Error(), "When 边") {
		t.Fatalf("incomplete when edge should be rejected, got %v", err)
	}
}

func TestValidateTektonPipelineTemplateConfigRejectsWhenEdgeMissingParam(t *testing.T) {
	content := `{
		"version": "tekton-dag/v1",
		"pipeline": {"name": "build-demo"},
		"nodes": [
			{"id": "build", "name": "build", "taskRef": {"kind": "ClusterTask", "name": "buildah"}},
			{"id": "deploy", "name": "deploy", "taskRef": {"kind": "ClusterTask", "name": "kubectl"}}
		],
		"edges": [
			{"source": "build", "target": "deploy", "type": "when", "whenInput": "$(params.missing)", "whenOperator": "in", "whenValues": ["true"]}
		]
	}`

	if _, err := validateTektonPipelineTemplateConfig(nil, nil, content, []string{"SUPER_ADMIN"}); err == nil ||
		!strings.Contains(err.Error(), "未声明") {
		t.Fatalf("missing when edge param should be rejected, got %v", err)
	}
}

func TestValidateTektonPipelineTemplateConfigPreservesWorkspaceContract(t *testing.T) {
	content := `{
		"version": "tekton-dag/v1",
		"pipeline": {"name": "build-demo"},
		"workspaces": [
			{"name": "source", "description": "代码目录", "bindingType": "persistentVolumeClaim", "allowedBindingTypes": ["persistentVolumeClaim"]}
		],
		"nodes": [
			{
				"id": "build",
				"name": "build",
				"taskRef": {"kind": "ClusterTask", "name": "buildah"},
				"workspaces": [
					{"taskWorkspace": "source", "pipelineWorkspace": "source", "requiredBindingType": "persistentVolumeClaim", "bindingParamCode": "SOURCE_WS"}
				],
				"workspaceRequirements": [
					{"name": "source", "requiredBindingType": "persistentVolumeClaim", "bindingParamCode": "SOURCE_WS"}
				]
			}
		]
	}`

	got, err := validateTektonPipelineTemplateConfig(nil, nil, content, []string{"SUPER_ADMIN"})
	if err != nil {
		t.Fatalf("workspace contract should be accepted: %v", err)
	}
	for _, want := range []string{
		`"description":"代码目录"`,
		`"requiredBindingType":"persistentVolumeClaim"`,
		`"bindingParamCode":"SOURCE_WS"`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("normalized config should preserve %s:\n%s", want, got)
		}
	}
}

func TestValidateTektonPipelineTemplateConfigRejectsWorkspaceBindingMismatch(t *testing.T) {
	content := `{
		"version": "tekton-dag/v1",
		"pipeline": {"name": "build-demo"},
		"workspaces": [
			{"name": "source", "bindingType": "emptyDir", "allowedBindingTypes": ["emptyDir", "persistentVolumeClaim"]}
		],
		"nodes": [
			{
				"id": "build",
				"name": "build",
				"taskRef": {"kind": "ClusterTask", "name": "buildah"},
				"workspaces": [
					{"taskWorkspace": "source", "pipelineWorkspace": "source", "requiredBindingType": "persistentVolumeClaim"}
				],
				"workspaceRequirements": [
					{"name": "source", "requiredBindingType": "persistentVolumeClaim"}
				]
			}
		]
	}`

	if _, err := validateTektonPipelineTemplateConfig(nil, nil, content, []string{"SUPER_ADMIN"}); err == nil ||
		!strings.Contains(err.Error(), "绑定类型不匹配") {
		t.Fatalf("workspace binding mismatch should be rejected, got %v", err)
	}
}

func TestValidateTektonPipelineTemplateConfigRejectsOptionalRequiredWorkspaceWithoutMapping(t *testing.T) {
	content := `{
		"version": "tekton-dag/v1",
		"pipeline": {"name": "build-demo"},
		"workspaces": [
			{"name": "source", "bindingType": "persistentVolumeClaim", "allowedBindingTypes": ["persistentVolumeClaim"]}
		],
		"nodes": [
			{
				"id": "build",
				"name": "build",
				"taskRef": {"kind": "ClusterTask", "name": "buildah"},
				"workspaceRequirements": [
					{"name": "source", "optional": true, "requiredBindingType": "persistentVolumeClaim"}
				]
			}
		]
	}`

	if _, err := validateTektonPipelineTemplateConfig(nil, nil, content, []string{"SUPER_ADMIN"}); err == nil ||
		!strings.Contains(err.Error(), "缺少必填 workspace 映射") {
		t.Fatalf("optional workspace with required binding should require mapping, got %v", err)
	}
}

func TestValidateTektonPipelineTemplateConfigAllowsPlatformOptionalParamRequirement(t *testing.T) {
	content := `{
		"version": "tekton-dag/v1",
		"pipeline": {"name": "build-demo"},
		"nodes": [
			{
				"id": "build",
				"name": "build",
				"taskRef": {"kind": "ClusterTask", "name": "buildah"},
				"paramRequirements": [
					{"name": "maybe-image", "type": "string", "required": false}
				]
			}
		]
	}`

	got, err := validateTektonPipelineTemplateConfig(nil, nil, content, []string{"SUPER_ADMIN"})
	if err != nil {
		t.Fatalf("platform optional param should be accepted: %v", err)
	}
	if !strings.Contains(got, `"required":false`) {
		t.Fatalf("normalized config should preserve platform required=false:\n%s", got)
	}
}

func TestValidateTektonPipelineTemplateConfigRejectsPlatformRequiredParamWithoutMapping(t *testing.T) {
	content := `{
		"version": "tekton-dag/v1",
		"pipeline": {"name": "build-demo"},
		"nodes": [
			{
				"id": "build",
				"name": "build",
				"taskRef": {"kind": "ClusterTask", "name": "buildah"},
				"paramRequirements": [
					{"name": "image-url", "type": "string", "required": true}
				]
			}
		]
	}`

	if _, err := validateTektonPipelineTemplateConfig(nil, nil, content, []string{"SUPER_ADMIN"}); err == nil ||
		!strings.Contains(err.Error(), "缺少") ||
		!strings.Contains(err.Error(), "参数绑定") {
		t.Fatalf("platform required param without mapping should be rejected, got %v", err)
	}
}

func TestKubeNovaTokenBase64RejectsGlobalTokenWhenBindingDisallows(t *testing.T) {
	tokenBase64, scope, message, err := (&DynamicParamOptionsLogic{}).kubeNovaTokenBase64(
		&model.DevopsProjectChannelBinding{AllowUseGlobalCredential: false},
		&model.DevopsChannel{Token: "global-token"},
	)
	if err != nil {
		t.Fatalf("kube-nova token resolve should not fail: %v", err)
	}
	if tokenBase64 != "" {
		t.Fatalf("global token should not be used when binding disallows it: %s", tokenBase64)
	}
	if scope != "none" || !strings.Contains(message, "未绑定项目凭据") {
		t.Fatalf("unexpected credential message: scope=%s message=%s", scope, message)
	}
}

func TestKubeNovaTokenBase64AllowsGlobalTokenWhenBindingAllows(t *testing.T) {
	token := "global-token"
	tokenBase64, scope, message, err := (&DynamicParamOptionsLogic{}).kubeNovaTokenBase64(
		&model.DevopsProjectChannelBinding{AllowUseGlobalCredential: true},
		&model.DevopsChannel{Token: token},
	)
	if err != nil {
		t.Fatalf("kube-nova token resolve should not fail: %v", err)
	}
	want := base64.StdEncoding.EncodeToString([]byte(token))
	if tokenBase64 != want {
		t.Fatalf("global token should be encoded, got %s want %s", tokenBase64, want)
	}
	if scope != "global" || !strings.Contains(message, "全局 token") {
		t.Fatalf("unexpected credential message: scope=%s message=%s", scope, message)
	}
}

func TestChannelCredentialSecretFieldBuildsJSONBase64(t *testing.T) {
	secret := model.CredentialSecret{
		Username: "robot",
		Password: "secret",
	}

	raw, ok := channelCredentialSecretField(secret, "username_password", "jsonData")
	if !ok {
		t.Fatal("username password credential should support jsonData")
	}
	got := decodeCredentialJSON(t, raw)
	if got["username"] != "robot" || got["password"] != "secret" || len(got) != 2 {
		t.Fatalf("unexpected jsonData: %#v", got)
	}

	encoded, ok := channelCredentialSecretField(secret, "username_password", "jsonBase64")
	if !ok {
		t.Fatal("username password credential should support jsonBase64")
	}
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("jsonBase64 should be valid base64: %v", err)
	}
	got = decodeCredentialJSON(t, string(decoded))
	if got["username"] != "robot" || got["password"] != "secret" || len(got) != 2 {
		t.Fatalf("unexpected decoded jsonBase64: %#v", got)
	}
}

func TestChannelCredentialSecretFieldPreservesJSONCredential(t *testing.T) {
	rawJSON := `{"token":"abc"}`
	encoded, ok := channelCredentialSecretField(model.CredentialSecret{JsonData: rawJSON}, "json", "jsonBase64")
	if !ok {
		t.Fatal("json credential should support jsonBase64")
	}
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("jsonBase64 should be valid base64: %v", err)
	}
	if string(decoded) != rawJSON {
		t.Fatalf("json credential should preserve raw json, got %s", decoded)
	}
}

func TestNormalizeChannelCredentialModeSupportsJSONModes(t *testing.T) {
	if got := normalizeChannelCredentialMode("jsonData", ""); got != credentialModeJSONData {
		t.Fatalf("jsonData mode should be kept, got %s", got)
	}
	if got := normalizeChannelCredentialMode("jsonBase64", ""); got != credentialModeJSONBase64 {
		t.Fatalf("jsonBase64 mode should be kept, got %s", got)
	}
	if got := normalizeChannelCredentialMode("", "jsonBase64"); got != credentialModeJSONBase64 {
		t.Fatalf("legacy jsonBase64 mapping field should map to mode, got %s", got)
	}
	if got := normalizeCredentialMode("jsonBase64", ""); got != credentialModeJSONBase64 {
		t.Fatalf("platform credential should keep jsonBase64 mode, got %s", got)
	}
	if got := normalizeCredentialMode("", "jsonData"); got != credentialModeJSONData {
		t.Fatalf("legacy platform credential jsonData mapping field should map to mode, got %s", got)
	}
}

func TestValidateStepParamsAllowsPlatformCredentialJSONModeWithoutMappingField(t *testing.T) {
	params := []model.DevopsStepParam{
		{
			Name:        "平台凭证",
			Code:        "CREDENTIAL_JSON",
			ParamType:   "voucherModel",
			Mode:        "paas",
			RuntimeMode: "env",
			Config: model.DevopsStepParamConfig{
				VoucherModel:   "username_password",
				CredentialMode: credentialModeJSONBase64,
			},
		},
	}

	if err := validateStepParams(nil, nil, "jenkins", "", params); err != nil {
		t.Fatalf("platform credential json mode should not require mapping field: %v", err)
	}
}

func TestChannelMappingFieldsKeepDefaultFields(t *testing.T) {
	schema := channelvars.ParseVariableSchema(`{"fields":[{"field":"config.project","name":"默认项目"},{"field":"dynamic.image","name":"Harbor 镜像","kind":"dynamic","uiControl":"select"}]}`)
	fields := dedupeMappingFields(variableFieldsToPb(channelvars.MergeVariableFields(schema.Fields)))

	got := map[string]string{}
	for _, item := range fields {
		got[item.Field] = item.Name
	}
	if got["endpoint"] != "渠道实例" {
		t.Fatalf("default endpoint field should be preserved, got %#v", got)
	}
	if got["config.project"] != "默认项目" {
		t.Fatalf("custom mapping field should be appended, got %#v", got)
	}
	if got["dynamic.image"] != "Harbor 镜像" {
		t.Fatalf("group dynamic field should be preserved, got %#v", got)
	}
}

func decodeCredentialJSON(t *testing.T, value string) map[string]string {
	t.Helper()
	var got map[string]string
	if err := json.Unmarshal([]byte(value), &got); err != nil {
		t.Fatalf("credential json should be valid: %v, value: %s", err, value)
	}
	return got
}

func TestValidateStepParamsAllowsEnvText(t *testing.T) {
	param := model.DevopsStepParam{
		Name:         "配置文本",
		Code:         "CONFIG_TEXT",
		ParamType:    "text",
		Mode:         "env",
		DefaultValue: "文本1\n文本2\n文本3",
	}

	if err := validateStepParams(nil, nil, "jenkins", "", []model.DevopsStepParam{param}); err != nil {
		t.Fatalf("env text should be allowed, got %v", err)
	}
}

func TestNormalizeStepParamConfigPreservesFullRow(t *testing.T) {
	param := model.DevopsStepParam{
		Name:      "命令",
		Code:      "BASH_COMMAND",
		ParamType: "text",
		Mode:      "env",
		Config: model.DevopsStepParamConfig{
			FullRow: true,
		},
	}

	got := normalizeStepParamConfig(param)
	if !got.FullRow {
		t.Fatal("fullRow should be preserved")
	}
}

func TestValidateStepParamsAllowsProjectDynamicParamTypes(t *testing.T) {
	params := []model.DevopsStepParam{
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
			Config: model.DevopsStepParamConfig{
				ChannelParamCode: "SONAR_URL",
			},
		},
		{
			Name:        "主机组",
			Code:        "HOSTS",
			ParamType:   channelvars.ParamHostGroupTargets,
			Mode:        "paas",
			RuntimeMode: "env",
			Config: model.DevopsStepParamConfig{
				RenderMode: "inventory",
			},
		},
	}

	if err := validateStepParams(nil, nil, "jenkins", "", params); err != nil {
		t.Fatalf("project dynamic param types should be allowed: %v", err)
	}
}

func TestValidateStepParamsAllowsAllDynamicParamTypes(t *testing.T) {
	for _, paramType := range channelvars.DynamicParamTypes() {
		if paramType == channelvars.ParamConfigCenter {
			continue
		}
		t.Run(paramType, func(t *testing.T) {
			param := model.DevopsStepParam{
				Name:        paramType,
				Code:        strings.ToUpper(paramType),
				ParamType:   paramType,
				Mode:        "paas",
				RuntimeMode: "params",
			}
			if err := validateStepParams(nil, nil, "jenkins", "", []model.DevopsStepParam{param}); err != nil {
				t.Fatalf("dynamic param type should be allowed: %v", err)
			}
		})
	}
}

func TestValidateStepParamsAllowsKubeNovaDeployConfig(t *testing.T) {
	params := []model.DevopsStepParam{
		{
			Name:        "kube-nova 部署配置",
			Code:        "KUBE_NOVA_DEPLOY_CONFIG",
			ParamType:   channelvars.ParamKubeNovaDeployConfig,
			Mode:        "paas",
			RuntimeMode: "params",
			Required:    true,
		},
	}

	if err := validateStepParams(nil, nil, "jenkins", "", params); err != nil {
		t.Fatalf("kube-nova deploy config should be allowed: %v", err)
	}

	config := normalizeStepParamConfig(params[0])
	if !config.FullRow {
		t.Fatal("kube-nova deploy config should be full row")
	}
	if config.ChannelGroupCode != channelvars.GroupDeployTarget || config.MappingField != channelvars.FieldAddressDeployConfig {
		t.Fatalf("unexpected kube-nova channel mapping: %#v", config)
	}
}

func TestValidateStepParamsRejectsInvalidHostGroupRenderMode(t *testing.T) {
	params := []model.DevopsStepParam{
		{
			Name:        "主机组",
			Code:        "HOSTS",
			ParamType:   channelvars.ParamHostGroupTargets,
			Mode:        "paas",
			RuntimeMode: "env",
			Config: model.DevopsStepParamConfig{
				RenderMode: "yaml",
			},
		},
	}

	err := validateStepParams(nil, nil, "jenkins", "", params)
	if err == nil || !strings.Contains(err.Error(), "主机组输出模式不支持") {
		t.Fatalf("invalid host group render mode should be rejected, got %v", err)
	}
}

func TestValidateStepParamsAllowsHostGroupExtendedRenderModes(t *testing.T) {
	for _, renderMode := range []string{"json", "jsonBase64", "ansible", "ansibleBase64"} {
		t.Run(renderMode, func(t *testing.T) {
			params := []model.DevopsStepParam{
				{
					Name:        "主机组",
					Code:        "HOSTS",
					ParamType:   channelvars.ParamHostGroupTargets,
					Mode:        "paas",
					RuntimeMode: "env",
					Config: model.DevopsStepParamConfig{
						RenderMode: renderMode,
					},
				},
			}
			if err := validateStepParams(nil, nil, "jenkins", "", params); err != nil {
				t.Fatalf("host group render mode %s should be allowed: %v", renderMode, err)
			}
		})
	}
}

func TestValidateStepParamsAllowsGitBranchDependsOnGitRepositoryURL(t *testing.T) {
	params := []model.DevopsStepParam{
		{
			Name:        "Git 仓库地址",
			Code:        "GIT_REPO_URL",
			ParamType:   channelvars.ParamGitRepositoryURL,
			Mode:        "paas",
			RuntimeMode: "env",
		},
		{
			Name:        "分支",
			Code:        "BRANCH",
			ParamType:   channelvars.ParamGitBranch,
			Mode:        "paas",
			RuntimeMode: "params",
			Config: model.DevopsStepParamConfig{
				ProjectParamCode: "GIT_REPO_URL",
			},
		},
	}

	if err := validateStepParams(nil, nil, "jenkins", "", params); err != nil {
		t.Fatalf("git branch should allow gitRepositoryUrl dependency: %v", err)
	}
}

func TestValidateStepParamsAllowsChannelVariableBranchManualInputWithoutDependencies(t *testing.T) {
	params := []model.DevopsStepParam{
		{
			Name:        "Git 分支",
			Code:        "BRANCH",
			ParamType:   channelvars.ParamChannelVariable,
			Mode:        "paas",
			RuntimeMode: "params",
			Config: model.DevopsStepParamConfig{
				ChannelGroupCode:  channelvars.GroupCodeRepo,
				ChannelTypeFilter: "gitlab",
				MappingField:      "dynamic.branch",
			},
		},
	}

	if err := validateStepParams(nil, nil, "jenkins", "", params); err != nil {
		t.Fatalf("channel variable branch should allow manual input without dependencies: %v", err)
	}
}

func TestValidateStepParamsAllowsChannelVariableBranchDependencies(t *testing.T) {
	params := []model.DevopsStepParam{
		{
			Name:        "GitLab 渠道",
			Code:        "GIT_CHANNEL",
			ParamType:   channelvars.ParamChannelVariable,
			Mode:        "paas",
			RuntimeMode: "env",
			Config: model.DevopsStepParamConfig{
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
			Config: model.DevopsStepParamConfig{
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
			Config: model.DevopsStepParamConfig{
				ChannelGroupCode:  channelvars.GroupCodeRepo,
				ChannelTypeFilter: "gitlab",
				MappingField:      "dynamic.branch",
				ChannelParamCode:  "GIT_CHANNEL",
				DependencyParamCodes: []model.DevopsStepParamDependency{
					{Field: channelvars.FieldDynamicProject, ParamCode: "GIT_PROJECT"},
				},
			},
		},
	}

	if err := validateStepParams(nil, nil, "jenkins", "", params); err != nil {
		t.Fatalf("channel variable branch dependencies should be allowed: %v", err)
	}
}

func TestValidateStepParamsAllowsChannelVariableBranchByRepositoryURL(t *testing.T) {
	params := []model.DevopsStepParam{
		{
			Name:        "Git 仓库地址",
			Code:        "GIT_REPO_URL",
			ParamType:   channelvars.ParamChannelVariable,
			Mode:        "paas",
			RuntimeMode: "env",
			Config: model.DevopsStepParamConfig{
				ChannelGroupCode:  channelvars.GroupCodeRepo,
				ChannelTypeFilter: "gitlab",
				MappingField:      channelvars.FieldAddressProjectURL,
			},
		},
		{
			Name:        "Git 分支",
			Code:        "BRANCH",
			ParamType:   channelvars.ParamChannelVariable,
			Mode:        "paas",
			RuntimeMode: "params",
			Config: model.DevopsStepParamConfig{
				ChannelGroupCode:  channelvars.GroupCodeRepo,
				ChannelTypeFilter: "gitlab",
				MappingField:      channelvars.FieldDynamicBranch,
				DependencyParamCodes: []model.DevopsStepParamDependency{
					{Field: channelvars.FieldDynamicProject, ParamCode: "GIT_REPO_URL"},
				},
			},
		},
	}

	if err := validateStepParams(nil, nil, "jenkins", "", params); err != nil {
		t.Fatalf("channel variable branch should allow repository URL dependency: %v", err)
	}
}

func TestChannelMappingFieldsKeepChannelBaseFields(t *testing.T) {
	schema := channelvars.ParseVariableSchema(`{"fields":[{"field":"config.project","name":"默认项目"},{"field":"dynamic.project","name":"Harbor 项目"}]}`)
	fields := dedupeMappingFields(variableFieldsToPb(channelvars.MergeVariableFields(schema.Fields)))
	got := make(map[string]bool, len(fields))
	for _, item := range fields {
		got[item.Field] = true
	}
	for _, field := range []string{"id", "name", "code", "endpoint", "config.project", "dynamic.project"} {
		if !got[field] {
			t.Fatalf("mapping fields should contain %s, got %#v", field, got)
		}
	}
}

func TestNormalizeChannelVariableMappingFieldKeepsLatestField(t *testing.T) {
	cases := []struct {
		name        string
		channelType string
		field       string
		want        string
	}{
		{name: "nexus repository", channelType: "nexus", field: channelvars.FieldDynamicRepository, want: channelvars.FieldDynamicRepository},
		{name: "nexus repository url", channelType: "nexus", field: channelvars.FieldAddressRepositoryURL, want: channelvars.FieldAddressRepositoryURL},
		{name: "nexus legacy artifact", channelType: "nexus", field: channelvars.FieldDynamicArtifact, want: channelvars.FieldDynamicArtifactName},
		{name: "nexus legacy tag", channelType: "nexus", field: channelvars.FieldDynamicTag, want: channelvars.FieldDynamicArtifactVersion},
		{name: "nexus legacy artifact tag url", channelType: "nexus", field: channelvars.FieldAddressArtifactTagURL, want: channelvars.FieldAddressArtifactVersionURL},
		{name: "jfrog repository", channelType: "jfrog", field: channelvars.FieldDynamicRepository, want: channelvars.FieldDynamicRepository},
		{name: "image registry", channelType: "registry", field: channelvars.FieldAddressRegistryURL, want: channelvars.FieldAddressProjectURL},
		{name: "image legacy repository", channelType: "harbor", field: channelvars.FieldDynamicRepository, want: channelvars.FieldDynamicProject},
		{name: "kubernetes legacy resource", channelType: "kubernetes", field: channelvars.FieldDynamicResource, want: channelvars.FieldDynamicResourceName},
		{name: "kubernetes legacy container", channelType: "kubernetes", field: channelvars.FieldDynamicContainer, want: channelvars.FieldDynamicContainerName},
		{name: "kubernetes legacy deploy config", channelType: "kubernetes", field: channelvars.FieldDynamicDeployConfig, want: channelvars.FieldAddressDeploymentConfig},
		{name: "kube nova legacy deploy config", channelType: "kube-nova", field: channelvars.FieldDynamicDeployConfig, want: channelvars.FieldAddressDeployConfig},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := normalizeChannelVariableMappingField(channelvars.GroupArtifactRepo, tc.channelType, tc.field)
			if got != tc.want {
				t.Fatalf("unexpected mapping field, got %s want %s", got, tc.want)
			}
		})
	}
}

func TestValidateStepParamsAllowsImageRegistryDynamicDependencies(t *testing.T) {
	params := []model.DevopsStepParam{
		imageRegistryStepParam("IMG_CHANNEL", channelvars.FieldEndpoint, nil, ""),
		imageRegistryStepParam("IMG_PROJECT", channelvars.FieldDynamicProject, nil, "IMG_CHANNEL"),
		imageRegistryStepParam("IMG_IMAGE", channelvars.FieldDynamicImage, []model.DevopsStepParamDependency{
			{Field: channelvars.FieldDynamicProject, ParamCode: "IMG_PROJECT"},
		}, "IMG_CHANNEL"),
		imageRegistryStepParam("IMG_TAG", channelvars.FieldDynamicTag, []model.DevopsStepParamDependency{
			{Field: channelvars.FieldDynamicProject, ParamCode: "IMG_PROJECT"},
			{Field: channelvars.FieldDynamicImage, ParamCode: "IMG_IMAGE"},
		}, "IMG_CHANNEL"),
	}

	if err := validateStepParams(nil, nil, "jenkins", "", params); err != nil {
		t.Fatalf("image registry endpoint dependencies should be allowed: %v", err)
	}
}

func TestValidateStepParamsAllowsImageRegistryAddressDependencies(t *testing.T) {
	params := []model.DevopsStepParam{
		imageRegistryStepParam("IMG_PROJECT_URL", channelvars.FieldAddressProjectURL, nil, ""),
		imageRegistryStepParam("IMG_URL", channelvars.FieldAddressImageURL, nil, ""),
		imageRegistryStepParam("IMG_IMAGE", channelvars.FieldDynamicImage, []model.DevopsStepParamDependency{
			{Field: channelvars.FieldAddressProjectURL, ParamCode: "IMG_PROJECT_URL"},
		}, ""),
		imageRegistryStepParam("IMG_TAG", channelvars.FieldDynamicTag, []model.DevopsStepParamDependency{
			{Field: channelvars.FieldAddressImageURL, ParamCode: "IMG_URL"},
		}, ""),
		imageRegistryStepParam("IMG_IMAGE_TAG", channelvars.FieldDynamicImageTag, []model.DevopsStepParamDependency{
			{Field: channelvars.FieldAddressProjectURL, ParamCode: "IMG_PROJECT_URL"},
		}, ""),
	}

	if err := validateStepParams(nil, nil, "jenkins", "", params); err != nil {
		t.Fatalf("image registry address dependencies should be allowed: %v", err)
	}
}

func TestValidateStepParamsAllowsImageRegistryTagManualInputWithProjectURL(t *testing.T) {
	params := []model.DevopsStepParam{
		imageRegistryStepParam("IMG_PROJECT_URL", channelvars.FieldAddressProjectURL, nil, ""),
		imageRegistryStepParam("IMG_TAG", channelvars.FieldDynamicTag, []model.DevopsStepParamDependency{
			{Field: channelvars.FieldAddressProjectURL, ParamCode: "IMG_PROJECT_URL"},
		}, ""),
	}

	if err := validateStepParams(nil, nil, "jenkins", "", params); err != nil {
		t.Fatalf("image tag with project URL should allow manual input without image dependency: %v", err)
	}
}

func TestValidateStepParamsAllowsArtifactVersionWithArtifactURL(t *testing.T) {
	params := []model.DevopsStepParam{
		artifactRepositoryStepParam("ARTIFACT_URL", channelvars.FieldAddressArtifactURL, nil, ""),
		artifactRepositoryStepParam("ARTIFACT_VERSION", channelvars.FieldDynamicArtifactVersion, []model.DevopsStepParamDependency{
			{Field: channelvars.FieldAddressArtifactURL, ParamCode: "ARTIFACT_URL"},
		}, ""),
	}

	if err := validateStepParams(nil, nil, "jenkins", "", params); err != nil {
		t.Fatalf("artifact version should allow artifact URL dependency: %v", err)
	}
}

func imageRegistryStepParam(code, mappingField string, deps []model.DevopsStepParamDependency, channelParamCode string) model.DevopsStepParam {
	return model.DevopsStepParam{
		Name:        code,
		Code:        code,
		ParamType:   channelvars.ParamChannelVariable,
		Mode:        "paas",
		RuntimeMode: "params",
		Config: model.DevopsStepParamConfig{
			ChannelGroupCode:     channelvars.GroupImageRepo,
			ChannelTypeFilter:    "harbor",
			MappingField:         mappingField,
			ChannelParamCode:     channelParamCode,
			DependencyParamCodes: deps,
		},
	}
}

func artifactRepositoryStepParam(code, mappingField string, deps []model.DevopsStepParamDependency, channelParamCode string) model.DevopsStepParam {
	return model.DevopsStepParam{
		Name:        code,
		Code:        code,
		ParamType:   channelvars.ParamChannelVariable,
		Mode:        "paas",
		RuntimeMode: "params",
		Config: model.DevopsStepParamConfig{
			ChannelGroupCode:     channelvars.GroupArtifactRepo,
			ChannelTypeFilter:    "nexus",
			MappingField:         mappingField,
			ChannelParamCode:     channelParamCode,
			DependencyParamCodes: deps,
		},
	}
}

func TestValidateTektonStepParamsRejectsPlatformTypesAsTaskContract(t *testing.T) {
	params := []model.DevopsStepParam{
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
			DefaultValue: "git-basic-auth",
		},
	}

	err := validateStepParams(nil, nil, tektonEngineType, "", params)
	if err == nil || !strings.Contains(err.Error(), "string、array、object") {
		t.Fatalf("tekton task params should reject platform types, got %v", err)
	}
}

func TestValidateTektonStepParamsAllowsOfficialTaskTypes(t *testing.T) {
	params := []model.DevopsStepParam{
		{
			Name:        "字符串参数",
			Code:        "url",
			ParamType:   "string",
			Mode:        "params",
			RuntimeMode: "params",
		},
		{
			Name:        "数组参数",
			Code:        "args",
			ParamType:   "array",
			Mode:        "params",
			RuntimeMode: "params",
		},
		{
			Name:        "对象参数",
			Code:        "config",
			ParamType:   "object",
			Mode:        "params",
			RuntimeMode: "params",
		},
	}

	if err := validateStepParams(nil, nil, tektonEngineType, "", params); err != nil {
		t.Fatalf("tekton official task types should be allowed: %v", err)
	}
}

func TestMergeTektonTaskParamContractsPreservesPlatformRequired(t *testing.T) {
	items := []model.DevopsTektonTaskParam{
		{Name: "image", Type: "string", Required: true},
		{Name: "args", Type: "array", Required: true},
	}
	overrides := []*pb.DevopsTektonTaskParam{
		{Name: "image", Required: false},
	}

	got := mergeTektonTaskParamContracts(items, overrides)
	if got[0].Required {
		t.Fatalf("platform required override should be preserved")
	}
	if !got[1].Required {
		t.Fatalf("unmatched task param required value should be unchanged")
	}
}

func TestMergeTektonTaskParamStepContractsPreservesPlatformRequired(t *testing.T) {
	items := []model.DevopsTektonTaskParam{
		{Name: "image", Type: "string", Required: false},
		{Name: "args", Type: "array", Required: false},
	}
	overrides := []*pb.DevopsStepParam{
		{Code: "image", Required: true},
	}

	got := mergeTektonTaskParamStepContracts(items, overrides)
	if !got[0].Required {
		t.Fatalf("step param required override should be preserved")
	}
	if got[1].Required {
		t.Fatalf("unmatched task param required value should be unchanged")
	}
}

func TestValidateTektonStepParamsRejectsJenkinsCredentialParam(t *testing.T) {
	param := model.DevopsStepParam{
		Name:        "Jenkins 凭证",
		Code:        "CREDENTIAL",
		ParamType:   channelvars.ParamChannelCredential,
		Mode:        "params",
		RuntimeMode: "params",
	}

	err := validateStepParams(nil, nil, tektonEngineType, "", []model.DevopsStepParam{param})
	if err == nil || !strings.Contains(err.Error(), "string、array、object") {
		t.Fatalf("tekton should reject Jenkins credential param, got %v", err)
	}
}

func TestValidateTektonStepParamMappingsRejectsDuplicateSecretWorkspace(t *testing.T) {
	content := `apiVersion: tekton.dev/v1
kind: Task
metadata:
  name: git-clone
spec:
  workspaces:
    - name: basic-auth
  steps:
    - name: clone
      image: alpine/git:latest
      script: git --version
`
	params := []model.DevopsStepParam{
		{
			Name:      "Secret A",
			Code:      "secret-a",
			ParamType: channelvars.ParamKubernetesSecretName,
			Config: model.DevopsStepParamConfig{
				MappingField: "basic-auth",
			},
		},
		{
			Name:      "Secret B",
			Code:      "secret-b",
			ParamType: channelvars.ParamKubernetesSecretName,
			Config: model.DevopsStepParamConfig{
				MappingField: "basic-auth",
			},
		},
	}

	err := validateTektonStepParamMappings(content, params)
	if err == nil || !strings.Contains(err.Error(), "只能映射一个 Secret 参数") {
		t.Fatalf("duplicate secret workspace mapping should be rejected, got %v", err)
	}
}

func TestValidateTektonStepParamMappingsRejectsSecretCodeSameAsTaskParam(t *testing.T) {
	content := `apiVersion: tekton.dev/v1
kind: Task
metadata:
  name: git-clone
spec:
  params:
    - name: url
      type: string
      default: ""
  workspaces:
    - name: basic-auth
  steps:
    - name: clone
      image: alpine/git:latest
      script: git --version
`
	params := []model.DevopsStepParam{
		{
			Name:      "认证 Secret",
			Code:      "url",
			ParamType: channelvars.ParamKubernetesSecretName,
			Config: model.DevopsStepParamConfig{
				MappingField: "basic-auth",
			},
		},
	}

	err := validateTektonStepParamMappings(content, params)
	if err == nil || !strings.Contains(err.Error(), "不能与 Task 参数重名") {
		t.Fatalf("secret param code should not reuse task param name, got %v", err)
	}
}

func TestValidateTektonStepParamMappingsRejectsDuplicateWorkspaceName(t *testing.T) {
	content := `apiVersion: tekton.dev/v1
kind: Task
metadata:
  name: git-clone
spec:
  workspaces:
    - name: output
    - name: output
  steps:
    - name: clone
      image: alpine/git:latest
      script: git --version
`

	err := validateTektonStepParamMappings(content, nil)
	if err == nil || !strings.Contains(err.Error(), "workspace 名称不能重复") {
		t.Fatalf("duplicate workspace name should be rejected, got %v", err)
	}
}

func TestCollectTektonStepImagesFromTemplateResolvesParamDefault(t *testing.T) {
	step := &model.DevopsStepTemplate{
		ID:   bson.NewObjectID(),
		Name: "Kubectl Set Image",
		Code: "kubectl-set-image",
		StageContent: `apiVersion: tekton.dev/v1
kind: Task
metadata:
  name: kubectl-set-image
spec:
  params:
    - name: kubectlImage
      type: string
      default: bitnami/kubectl:1.30
  steps:
    - name: set
      image: $(params.kubectlImage)
      script: kubectl version --client
`,
	}

	items, err := collectTektonStepImagesFromTemplate(step)
	if err != nil {
		t.Fatalf("collectTektonStepImagesFromTemplate() error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("len(items) = %d, want 1", len(items))
	}
	if items[0].Image != "bitnami/kubectl:1.30" {
		t.Fatalf("Image = %q, want bitnami/kubectl:1.30", items[0].Image)
	}
	if items[0].ImageType != tektonStepImageTypeParam || items[0].ParamName != "kubectlImage" {
		t.Fatalf("type=%q param=%q, want param kubectlImage", items[0].ImageType, items[0].ParamName)
	}
}

func TestReplaceTektonStepImagesParamUpdatesDefaultOnly(t *testing.T) {
	content := `apiVersion: tekton.dev/v1
kind: Task
metadata:
  name: kubectl-set-image
spec:
  params:
    - name: kubectlImage
      type: string
      default: bitnami/kubectl:1.30
  steps:
    - name: set
      image: $(params.kubectlImage)
      script: kubectl version --client
`

	got, changed, err := replaceTektonStepImages(content, []tektonStepImageReplacement{
		{
			Image:     "bitnami/kubectl:1.30",
			NewImage:  "bitnami/kubectl:1.31",
			ImageType: tektonStepImageTypeParam,
			ParamName: "kubectlImage",
		},
	})
	if err != nil {
		t.Fatalf("replaceTektonStepImages() error = %v", err)
	}
	if !changed {
		t.Fatalf("changed = false, want true")
	}
	if !strings.Contains(got, "default: bitnami/kubectl:1.31") {
		t.Fatalf("default was not updated:\n%s", got)
	}
	if !strings.Contains(got, "image: $(params.kubectlImage)") {
		t.Fatalf("image field should keep param ref:\n%s", got)
	}
}

func TestValidateTektonPipelineTemplateConfigRejectsMissingParamRefInValue(t *testing.T) {
	content := `{
  "version": "tekton-dag/v1",
  "pipeline": {"name": "demo-pipeline"},
  "params": [{"name": "declared", "type": "string"}],
  "nodes": [{
    "id": "clone",
    "name": "clone",
    "taskRef": {"kind": "ClusterTask", "name": "git-clone"},
    "paramRequirements": [{"name": "url", "required": true}],
    "params": [{"name": "url", "source": "fixed", "value": "$(params.missing)"}]
  }],
  "edges": []
}`

	_, err := validateTektonPipelineTemplateConfig(nil, nil, content, []string{"SUPER_ADMIN"})
	if err == nil || !strings.Contains(err.Error(), "未声明") {
		t.Fatalf("missing Pipeline param ref should be rejected, got %v", err)
	}
}

func TestValidateTektonPipelineTemplateConfigAllowsDeclaredParamRefInValue(t *testing.T) {
	content := `{
  "version": "tekton-dag/v1",
  "pipeline": {"name": "demo-pipeline"},
  "params": [{"name": "declared", "type": "string"}],
  "nodes": [{
    "id": "clone",
    "name": "clone",
    "taskRef": {"kind": "ClusterTask", "name": "git-clone"},
    "paramRequirements": [{"name": "url", "required": true}],
    "params": [{"name": "url", "source": "fixed", "value": "$(params.declared)"}]
  }],
  "edges": []
}`

	if _, err := validateTektonPipelineTemplateConfig(nil, nil, content, []string{"SUPER_ADMIN"}); err != nil {
		t.Fatalf("declared Pipeline param ref should be accepted: %v", err)
	}
}

func TestValidateTektonPipelineTemplateConfigRejectsMissingPipelineResultRef(t *testing.T) {
	content := `{
  "version": "tekton-dag/v1",
  "pipeline": {"name": "demo-pipeline"},
  "results": [{"name": "commit", "type": "string", "value": "$(tasks.clone.results.missing)"}],
  "nodes": [{
    "id": "clone",
    "name": "clone",
    "taskRef": {"kind": "ClusterTask", "name": "git-clone"},
    "results": ["commit"]
  }],
  "edges": []
}`

	_, err := validateTektonPipelineTemplateConfig(nil, nil, content, []string{"SUPER_ADMIN"})
	if err == nil || !strings.Contains(err.Error(), "Result") {
		t.Fatalf("missing Pipeline result ref should be rejected, got %v", err)
	}
}

func TestValidateTektonPipelineTemplateConfigAllowsPipelineResultRef(t *testing.T) {
	content := `{
  "version": "tekton-dag/v1",
  "pipeline": {"name": "demo-pipeline"},
  "results": [{"name": "commit", "type": "string", "value": "$(tasks.clone.results.commit)"}],
  "nodes": [{
    "id": "clone",
    "name": "clone",
    "taskRef": {"kind": "ClusterTask", "name": "git-clone"},
    "results": ["commit"]
  }],
  "edges": []
}`

	if _, err := validateTektonPipelineTemplateConfig(nil, nil, content, []string{"SUPER_ADMIN"}); err != nil {
		t.Fatalf("Pipeline result ref should be accepted: %v", err)
	}
}

func TestValidateTektonPipelineTemplateConfigPreservesOfficialPipelineTaskFields(t *testing.T) {
	content := `{
  "version": "tekton-dag/v1",
  "pipeline": {"name": "demo-pipeline", "displayName": "Demo Pipeline", "description": "demo", "timeout": "1h"},
  "params": [{"name": "branch", "type": "string", "defaultValue": "main", "enum": ["main", "release"]}, {"name": "git", "type": "object", "defaultValue": {"url": "repo"}, "properties": {"url": {"type": "string"}}}],
  "results": [{"name": "image", "type": "object", "properties": {"digest": {"type": "string"}}, "value": {"digest": "$(tasks.build.results.digest)"}}],
  "nodes": [{
    "id": "build",
    "name": "build",
    "displayName": "Build Image",
    "description": "build image",
    "taskRef": {"kind": "ClusterTask", "name": "buildah"},
    "results": ["digest"],
    "when": [{"input": "$(params.branch)", "operator": "in", "values": ["main"]}],
    "retries": 1,
    "timeout": "20m",
    "onError": "continue",
    "matrix": {"params": [{"name": "platform", "values": ["linux", "arm64"]}]}
  }, {
    "id": "deploy",
    "name": "deploy",
    "taskRef": {"kind": "ClusterTask", "name": "kubectl"}
  }],
  "edges": [{
    "source": "build",
    "target": "deploy",
    "type": "when",
    "whenInput": "$(params.branch)",
    "whenOperator": "in",
    "whenValues": ["main"]
  }]
}`

	normalized, err := validateTektonPipelineTemplateConfig(nil, nil, content, []string{"SUPER_ADMIN"})
	if err != nil {
		t.Fatalf("official PipelineTask fields should be accepted: %v", err)
	}
	for _, want := range []string{`"displayName":"Demo Pipeline"`, `"description":"demo"`, `"enum"`, `"properties"`, `"results"`, `"digest"`, `"displayName":"Build Image"`, `"description":"build image"`, `"when"`, `"retries":1`, `"timeout":"20m"`, `"onError":"continue"`, `"matrix"`, `"whenInput"`} {
		if !strings.Contains(normalized, want) {
			t.Fatalf("normalized config should preserve %s, got %s", want, normalized)
		}
	}
}

func TestValidateTektonPipelineTemplateConfigRejectsUnsupportedOfficialTypes(t *testing.T) {
	content := `{
  "version": "tekton-dag/v1",
  "pipeline": {"name": "demo-pipeline"},
  "params": [{"name": "count", "type": "number"}],
  "nodes": [{"id": "build", "name": "build", "taskRef": {"kind": "ClusterTask", "name": "buildah"}}]
}`

	_, err := validateTektonPipelineTemplateConfig(nil, nil, content, []string{"SUPER_ADMIN"})
	if err == nil || !strings.Contains(err.Error(), "参数类型不支持") {
		t.Fatalf("unsupported Tekton param type should be rejected, got %v", err)
	}

	content = `{
  "version": "tekton-dag/v1",
  "pipeline": {"name": "demo-pipeline"},
  "params": [{"name": "meta", "type": "object", "properties": {"count": {"type": "number"}}}],
  "nodes": [{"id": "build", "name": "build", "taskRef": {"kind": "ClusterTask", "name": "buildah"}}]
}`

	_, err = validateTektonPipelineTemplateConfig(nil, nil, content, []string{"SUPER_ADMIN"})
	if err == nil || !strings.Contains(err.Error(), "properties 类型不支持") {
		t.Fatalf("unsupported Tekton property type should be rejected, got %v", err)
	}
}

func TestValidateTektonPipelineTemplateConfigPreservesCustomTaskRefAPIVersion(t *testing.T) {
	content := `{
  "version": "tekton-dag/v1",
  "pipeline": {"name": "demo-pipeline"},
  "nodes": [{
    "id": "custom",
    "name": "custom",
    "taskRef": {"kind": "ExampleTask", "apiVersion": "example.dev/v1alpha1", "name": "custom-task"}
  }]
}`

	normalized, err := validateTektonPipelineTemplateConfig(nil, nil, content, []string{"SUPER_ADMIN"})
	if err != nil {
		t.Fatalf("custom taskRef apiVersion should be accepted: %v", err)
	}
	for _, want := range []string{`"kind":"ExampleTask"`, `"apiVersion":"example.dev/v1alpha1"`} {
		if !strings.Contains(normalized, want) {
			t.Fatalf("normalized config should preserve %s, got %s", want, normalized)
		}
	}
}

func TestValidateTektonPipelineTemplateConfigAllowsNonClusterResolverParams(t *testing.T) {
	content := `{
  "version": "tekton-dag/v1",
  "pipeline": {"name": "demo-pipeline"},
  "nodes": [{
    "id": "git-task",
    "name": "git-task",
    "taskRef": {"resolver": "git", "params": [
      {"name": "url", "value": "https://example.com/tasks.git"},
      {"name": "revision", "value": "main"},
      {"name": "pathInRepo", "value": "tasks/build.yaml"}
    ]}
  }]
}`

	if _, err := validateTektonPipelineTemplateConfig(nil, nil, content, []string{"SUPER_ADMIN"}); err != nil {
		t.Fatalf("non-cluster resolver params should be accepted: %v", err)
	}
}

func TestValidateTektonPipelineTemplateConfigRejectsClusterResolverMissingName(t *testing.T) {
	content := `{
  "version": "tekton-dag/v1",
  "pipeline": {"name": "demo-pipeline"},
  "nodes": [{
    "id": "cluster-task",
    "name": "cluster-task",
    "taskRef": {"resolver": "cluster", "params": [
      {"name": "kind", "value": "task"}
    ]}
  }]
}`

	_, err := validateTektonPipelineTemplateConfig(nil, nil, content, []string{"SUPER_ADMIN"})
	if err == nil || !strings.Contains(err.Error(), "kind 和 name") {
		t.Fatalf("cluster resolver missing name should be rejected, got %v", err)
	}
}

func TestValidateTektonPipelineTemplateConfigSupportsPipelineRef(t *testing.T) {
	content := `{
  "version": "tekton-dag/v1",
  "pipeline": {"name": "demo-pipeline"},
  "nodes": [{
    "id": "child",
    "name": "child",
    "pipelineRef": {"name": "child-pipeline", "apiVersion": "tekton.dev/v1"},
    "results": ["digest"]
  }, {
    "id": "deploy",
    "name": "deploy",
    "taskRef": {"kind": "ClusterTask", "name": "kubectl"}
  }],
  "edges": [{
    "source": "child",
    "target": "deploy",
    "type": "result",
    "resultName": "digest",
    "targetParam": "digest"
  }]
}`

	normalized, err := validateTektonPipelineTemplateConfig(nil, nil, content, []string{"SUPER_ADMIN"})
	if err != nil {
		t.Fatalf("pipelineRef should be accepted: %v", err)
	}
	if !strings.Contains(normalized, `"pipelineRef":{"name":"child-pipeline","apiVersion":"tekton.dev/v1"`) {
		t.Fatalf("normalized config should preserve pipelineRef, got %s", normalized)
	}
}

func TestValidateTektonPipelineTemplateConfigRejectsDeprecatedRefBundle(t *testing.T) {
	content := `{
  "version": "tekton-dag/v1",
  "pipeline": {"name": "demo-pipeline"},
  "nodes": [{
    "id": "child",
    "name": "child",
    "pipelineRef": {"bundle": "gcr.io/example/pipeline:latest"}
  }]
}`

	_, err := validateTektonPipelineTemplateConfig(nil, nil, content, []string{"SUPER_ADMIN"})
	if err == nil || !strings.Contains(err.Error(), "bundle") {
		t.Fatalf("pipelineRef.bundle should be rejected, got %v", err)
	}

	content = `{
  "version": "tekton-dag/v1",
  "pipeline": {"name": "demo-pipeline"},
  "nodes": [{
    "id": "build",
    "name": "build",
    "taskRef": {"kind": "ClusterTask", "name": "buildah", "bundle": "gcr.io/example/task:latest"}
  }]
}`

	_, err = validateTektonPipelineTemplateConfig(nil, nil, content, []string{"SUPER_ADMIN"})
	if err == nil || !strings.Contains(err.Error(), "bundle") {
		t.Fatalf("taskRef.bundle should be rejected, got %v", err)
	}
}

func TestValidateTektonPipelineTemplateConfigRejectsInlinePipelineSpec(t *testing.T) {
	content := `{
  "version": "tekton-dag/v1",
  "pipeline": {"name": "demo-pipeline"},
  "nodes": [{
    "id": "child",
    "name": "child",
    "pipelineSpec": {"tasks": []}
  }]
}`

	_, err := validateTektonPipelineTemplateConfig(nil, nil, content, []string{"SUPER_ADMIN"})
	if err == nil || !strings.Contains(err.Error(), "inline pipelineSpec") {
		t.Fatalf("inline pipelineSpec should be rejected, got %v", err)
	}
}

func TestValidateTektonPipelineTemplateConfigRejectsMixedTaskAndPipelineRef(t *testing.T) {
	content := `{
  "version": "tekton-dag/v1",
  "pipeline": {"name": "demo-pipeline"},
  "nodes": [{
    "id": "child",
    "name": "child",
    "taskRef": {"kind": "ClusterTask", "name": "buildah"},
    "pipelineRef": {"name": "child-pipeline"}
  }]
}`

	_, err := validateTektonPipelineTemplateConfig(nil, nil, content, []string{"SUPER_ADMIN"})
	if err == nil || !strings.Contains(err.Error(), "只能配置一个") {
		t.Fatalf("mixed taskRef and pipelineRef should be rejected, got %v", err)
	}
}

func TestValidateTektonTemplateRunPolicyTaskRunSpecRefsRejectsUnknownTask(t *testing.T) {
	cfg := tektonPipelineTemplateDagConfig{
		Nodes: []tektonPipelineTemplateNode{
			{ID: "build", Name: "build"},
		},
		RunPolicy: tektonPipelineTemplateRunPolicy{
			TaskRunSpecsRaw: "- pipelineTaskName: missing\n  taskServiceAccountName: build-task-sa",
		},
	}

	err := validateTektonTemplateRunPolicyTaskRunSpecRefs(cfg)
	if err == nil || !strings.Contains(err.Error(), "PipelineTask 不存在") {
		t.Fatalf("unknown taskRunSpecs pipelineTaskName should be rejected, got %v", err)
	}
}

func TestValidateTektonPipelineTemplateConfigRejectsPlatformParamTypeMismatch(t *testing.T) {
	content := `{
  "version": "tekton-dag/v1",
  "pipeline": {"name": "demo-pipeline"},
  "params": [{"name": "targets", "type": "string", "paramType": "list", "defaultValue": ["dev"]}],
  "nodes": [{
    "id": "build",
    "name": "build",
    "taskRef": {"kind": "ClusterTask", "name": "buildah"}
  }],
  "edges": []
}`

	_, err := validateTektonPipelineTemplateConfig(nil, nil, content, []string{"SUPER_ADMIN"})
	if err == nil || !strings.Contains(err.Error(), "平台类型") {
		t.Fatalf("platform param type mismatch should be rejected, got %v", err)
	}
}

func TestValidateTektonTemplatePipelineParamAllowsCommaListDefault(t *testing.T) {
	item := model.DevopsStepParam{
		Name:         "目标环境",
		Code:         "targets",
		ParamType:    "list",
		DefaultValue: "dev,prod",
	}

	if err := validateTektonTemplatePipelineParam(item, map[string]model.DevopsStepParam{}); err != nil {
		t.Fatalf("comma list default should be accepted: %v", err)
	}

	item.DefaultValue = "[1, 2]"
	if err := validateTektonTemplatePipelineParam(item, map[string]model.DevopsStepParam{}); err == nil ||
		!strings.Contains(err.Error(), "字符串数组") {
		t.Fatalf("non-string array default should be rejected, got %v", err)
	}
}

func TestValidateTektonPipelineTemplateConfigAllowsChannelCredentialContract(t *testing.T) {
	content := `{
  "version": "tekton-dag/v1",
  "pipeline": {"name": "demo-pipeline"},
  "params": [{
    "name": "git_channel",
    "type": "string",
    "paramType": "channelGroupCode",
    "config": {"channelGroupCode": "code_repo", "channelTypeFilter": "gitlab", "mappingField": "endpoint"}
  }, {
    "name": "git_username",
    "type": "string",
    "paramType": "channelCredential",
    "config": {"credentialSourceParamCode": "git_channel", "credentialMode": "field", "mappingField": "username"}
  }],
  "nodes": [{
    "id": "clone",
    "name": "clone",
    "taskRef": {"kind": "ClusterTask", "name": "git-clone"}
  }],
  "edges": []
}`

	if _, err := validateTektonPipelineTemplateConfig(nil, nil, content, []string{"SUPER_ADMIN"}); err != nil {
		t.Fatalf("channel credential contract should be accepted: %v", err)
	}
}

func TestValidateTektonPipelineTemplateConfigAllowsChannelVariableManualInput(t *testing.T) {
	content := `{
  "version": "tekton-dag/v1",
  "pipeline": {"name": "demo-pipeline"},
  "params": [{
    "name": "git_branch",
    "type": "string",
    "paramType": "channelGroupCode",
    "config": {"channelGroupCode": "code_repo", "channelTypeFilter": "gitlab", "mappingField": "dynamic.branch"}
  }],
  "nodes": [{
    "id": "clone",
    "name": "clone",
    "taskRef": {"kind": "ClusterTask", "name": "git-clone"}
  }],
  "edges": []
}`

	if _, err := validateTektonPipelineTemplateConfig(nil, nil, content, []string{"SUPER_ADMIN"}); err != nil {
		t.Fatalf("channel variable should allow manual input without endpoint dependency: %v", err)
	}
}

func TestValidateTektonPipelineTemplateConfigPreservesPolicyFields(t *testing.T) {
	content := `{
  "version": "tekton-dag/v1",
  "pipeline": {"name": "demo-pipeline"},
  "runPolicy": {
    "serviceAccountName": "tekton-runner",
    "managedBy": "kube-nova-controller",
    "pipelineTimeout": "1h",
    "tasksTimeout": "50m",
    "concurrencyPolicy": "skip",
    "workspaceStrategy": "emptyDir",
    "labels": [{"name": "run", "value": "manual"}],
    "annotations": [{"name": "owner", "value": "devops"}],
    "podTemplateEnv": [{"name": "BUILD_ENV", "value": "prod"}],
    "taskRunSpecsRaw": "- pipelineTaskName: build\n  serviceAccountName: build-sa"
  },
  "triggerConfig": {"enabled": false, "apiVersion": "triggers.tekton.dev/v1beta1"},
  "scheduleConfig": {"enabled": true, "cron": "*/5 * * * *", "timezone": "Asia/Shanghai", "concurrencyPolicy": "skip"},
  "prunerPolicy": {"enabled": true, "resourceTypes": ["PipelineRun", "TaskRun"], "scope": "pipeline", "historyLimit": 10},
  "labels": {"team": "devops"},
  "annotations": {"audit": "enabled"},
  "nodes": [{
    "id": "build",
    "name": "build",
    "taskRef": {"kind": "ClusterTask", "name": "buildah"}
  }],
  "edges": []
}`

	normalized, err := validateTektonPipelineTemplateConfig(nil, nil, content, []string{"SUPER_ADMIN"})
	if err != nil {
		t.Fatalf("policy fields should be accepted: %v", err)
	}
	for _, want := range []string{`"runPolicy"`, `"serviceAccountName":"tekton-runner"`, `"managedBy":"kube-nova-controller"`, `"pipelineTimeout":"1h"`, `"taskRunSpecsRaw"`, `"scheduleConfig"`, `"cron":"*/5 * * * *"`, `"prunerPolicy"`, `"historyLimit":10`, `"team":"devops"`, `"audit":"enabled"`} {
		if !strings.Contains(normalized, want) {
			t.Fatalf("normalized config should preserve %s, got %s", want, normalized)
		}
	}
}

func TestValidateTektonPipelineTemplateConfigPreservesWorkspaceSubPath(t *testing.T) {
	content := `{
  "version": "tekton-dag/v1",
  "pipeline": {"name": "demo-pipeline"},
  "params": [{"name": "module", "type": "string", "defaultValue": "api"}],
  "workspaces": [{"name": "source", "bindingType": "emptyDir"}],
  "nodes": [{
    "id": "build",
    "name": "build",
    "taskRef": {"kind": "ClusterTask", "name": "buildah"},
    "workspaceRequirements": [{"name": "source", "optional": false}],
    "workspaces": [{"taskWorkspace": "source", "pipelineWorkspace": "source", "subPath": "$(params.module)"}]
  }],
  "edges": []
}`

	normalized, err := validateTektonPipelineTemplateConfig(nil, nil, content, []string{"SUPER_ADMIN"})
	if err != nil {
		t.Fatalf("workspace subPath should be accepted: %v", err)
	}
	if !strings.Contains(normalized, `"subPath":"$(params.module)"`) {
		t.Fatalf("normalized config should preserve workspace subPath, got %s", normalized)
	}
}

func TestValidateTektonPipelineTemplateConfigAllowsProjectedWorkspaceContract(t *testing.T) {
	content := `{
  "version": "tekton-dag/v1",
  "pipeline": {"name": "demo-pipeline"},
  "workspaces": [{"name": "config", "bindingType": "emptyDir", "allowedBindingTypes": ["emptyDir", "projected"]}],
  "nodes": [{
    "id": "build",
    "name": "build",
    "taskRef": {"kind": "ClusterTask", "name": "buildah"},
    "workspaceRequirements": [{"name": "config", "optional": true}]
  }],
  "edges": []
}`

	normalized, err := validateTektonPipelineTemplateConfig(nil, nil, content, []string{"SUPER_ADMIN"})
	if err != nil {
		t.Fatalf("projected workspace contract should be accepted: %v", err)
	}
	if !strings.Contains(normalized, `"projected"`) {
		t.Fatalf("normalized config should preserve projected allowed binding, got %s", normalized)
	}
}

func TestValidateTektonPipelineTemplateConfigRejectsWorkspaceSubPathMissingParam(t *testing.T) {
	content := `{
  "version": "tekton-dag/v1",
  "pipeline": {"name": "demo-pipeline"},
  "workspaces": [{"name": "source", "bindingType": "emptyDir"}],
  "nodes": [{
    "id": "build",
    "name": "build",
    "taskRef": {"kind": "ClusterTask", "name": "buildah"},
    "workspaceRequirements": [{"name": "source", "optional": false}],
    "workspaces": [{"taskWorkspace": "source", "pipelineWorkspace": "source", "subPath": "$(params.missing)"}]
  }],
  "edges": []
}`

	_, err := validateTektonPipelineTemplateConfig(nil, nil, content, []string{"SUPER_ADMIN"})
	if err == nil || !strings.Contains(err.Error(), "subPath") {
		t.Fatalf("workspace subPath missing param should be rejected, got %v", err)
	}
}

func TestValidateTektonPipelineTemplateConfigPreservesWhenCEL(t *testing.T) {
	content := `{
  "version": "tekton-dag/v1",
  "pipeline": {"name": "demo-pipeline"},
  "params": [{"name": "branch", "type": "string", "defaultValue": "main"}],
  "nodes": [{
    "id": "deploy",
    "name": "deploy",
    "taskRef": {"kind": "ClusterTask", "name": "kubectl"},
    "when": [{"cel": "'$(params.branch)' == 'main'"}]
  }],
  "edges": []
}`

	normalized, err := validateTektonPipelineTemplateConfig(nil, nil, content, []string{"SUPER_ADMIN"})
	if err != nil {
		t.Fatalf("when cel should be accepted: %v", err)
	}
	if !strings.Contains(normalized, `"cel":"'$(params.branch)' == 'main'"`) {
		t.Fatalf("normalized config should preserve when cel, got %s", normalized)
	}
}

func TestValidateTektonPipelineTemplateConfigRejectsWhenCELMissingParam(t *testing.T) {
	content := `{
  "version": "tekton-dag/v1",
  "pipeline": {"name": "demo-pipeline"},
  "nodes": [{
    "id": "deploy",
    "name": "deploy",
    "taskRef": {"kind": "ClusterTask", "name": "kubectl"},
    "when": [{"cel": "'$(params.missing)' == 'main'"}]
  }],
  "edges": []
}`

	_, err := validateTektonPipelineTemplateConfig(nil, nil, content, []string{"SUPER_ADMIN"})
	if err == nil || !strings.Contains(err.Error(), "CEL") {
		t.Fatalf("when cel missing param should be rejected, got %v", err)
	}
}

func TestValidateTektonPipelineTemplateConfigRejectsWhenInputMissingParam(t *testing.T) {
	content := `{
  "version": "tekton-dag/v1",
  "pipeline": {"name": "demo-pipeline"},
  "nodes": [{
    "id": "deploy",
    "name": "deploy",
    "taskRef": {"kind": "ClusterTask", "name": "kubectl"},
    "when": [{"input": "$(params.missing)", "operator": "in", "values": ["main"]}]
  }],
  "edges": []
}`

	_, err := validateTektonPipelineTemplateConfig(nil, nil, content, []string{"SUPER_ADMIN"})
	if err == nil || !strings.Contains(err.Error(), "未声明") {
		t.Fatalf("when input missing param should be rejected, got %v", err)
	}
}

func TestValidateTektonPipelineTemplateRunPolicyContentRejectsInvalidTimeout(t *testing.T) {
	_, err := validateTektonPipelineTemplateRunPolicyContent(`{"pipelineTimeout":"1hour"}`)
	if err == nil || !strings.Contains(err.Error(), "超时时间") {
		t.Fatalf("invalid run policy timeout should be rejected, got %v", err)
	}
}

func TestValidateTektonPipelineTemplateRunPolicyContentPreservesPipelineRunTemplates(t *testing.T) {
	normalized, err := validateTektonPipelineTemplateRunPolicyContent(`{
  "podTemplateRaw": "nodeSelector:\n  kubernetes.io/os: linux",
  "taskRunTemplateRaw": "serviceAccountName: build-sa\npodTemplate:\n  securityContext: {}"
}`)
	if err != nil {
		t.Fatalf("run policy pod/taskRun template should be accepted: %v", err)
	}
	for _, want := range []string{`"podTemplateRaw"`, `"taskRunTemplateRaw"`, `build-sa`} {
		if !strings.Contains(normalized, want) {
			t.Fatalf("normalized run policy should preserve %s, got %s", want, normalized)
		}
	}
}

func TestValidateTektonPipelineTemplateRunPolicyContentRejectsInvalidTaskRunTemplate(t *testing.T) {
	_, err := validateTektonPipelineTemplateRunPolicyContent(`{"taskRunTemplateRaw": "- bad"}`)
	if err == nil || !strings.Contains(err.Error(), "taskRunTemplate") {
		t.Fatalf("invalid taskRunTemplateRaw should be rejected, got %v", err)
	}
}

func TestValidateTektonPipelineTemplateTriggerConfigContentPreservesSchedule(t *testing.T) {
	normalized, err := validateTektonPipelineTemplateTriggerConfigContent(`{
  "enabled": false,
  "apiVersion": "triggers.tekton.dev/v1beta1",
  "scheduleConfig": {"enabled": true, "cron": "*/10 * * * *", "timezone": "Asia/Shanghai", "concurrencyPolicy": "replace"}
}`)
	if err != nil {
		t.Fatalf("trigger config with schedule should be accepted: %v", err)
	}
	for _, want := range []string{`"apiVersion":"triggers.tekton.dev/v1beta1"`, `"scheduleConfig"`, `"cron":"*/10 * * * *"`, `"concurrencyPolicy":"replace"`} {
		if !strings.Contains(normalized, want) {
			t.Fatalf("normalized trigger config should preserve %s, got %s", want, normalized)
		}
	}
}

func TestValidateTektonPipelineTemplateConfigRejectsInvalidParamProperty(t *testing.T) {
	content := `{
  "version": "tekton-dag/v1",
  "pipeline": {"name": "demo-pipeline"},
  "params": [{"name": "git", "type": "object", "properties": {"bad field": {"type": "string"}}}],
  "nodes": [{
    "id": "build",
    "name": "build",
    "taskRef": {"kind": "ClusterTask", "name": "buildah"}
  }],
  "edges": []
}`

	_, err := validateTektonPipelineTemplateConfig(nil, nil, content, []string{"SUPER_ADMIN"})
	if err == nil || !strings.Contains(err.Error(), "properties") {
		t.Fatalf("invalid param property should be rejected, got %v", err)
	}
}

func TestValidateTektonPipelineTemplateConfigRejectsInvalidMatrix(t *testing.T) {
	content := `{
  "version": "tekton-dag/v1",
  "pipeline": {"name": "demo-pipeline"},
  "nodes": [{
    "id": "build",
    "name": "build",
    "taskRef": {"kind": "ClusterTask", "name": "buildah"},
    "matrix": {"params": [{"name": "platform", "values": []}]}
  }],
  "edges": []
}`

	_, err := validateTektonPipelineTemplateConfig(nil, nil, content, []string{"SUPER_ADMIN"})
	if err == nil || !strings.Contains(err.Error(), "Matrix") {
		t.Fatalf("invalid matrix should be rejected, got %v", err)
	}
}

func TestValidateTektonPipelineTemplateConfigRejectsLongTaskName(t *testing.T) {
	longName := strings.Repeat("a", 64)
	content := `{
  "version": "tekton-dag/v1",
  "pipeline": {"name": "demo-pipeline"},
  "nodes": [{
    "id": "build",
    "name": "` + longName + `",
    "taskRef": {"kind": "ClusterTask", "name": "buildah"}
  }],
  "edges": []
}`

	_, err := validateTektonPipelineTemplateConfig(nil, nil, content, []string{"SUPER_ADMIN"})
	if err == nil || !strings.Contains(err.Error(), "命名规范") {
		t.Fatalf("long task name should be rejected, got %v", err)
	}
}

func TestValidateTektonPipelineTemplateConfigRejectsNonStringWhenAndMatrixValues(t *testing.T) {
	content := `{
  "version": "tekton-dag/v1",
  "pipeline": {"name": "demo-pipeline"},
  "params": [{"name": "branch", "type": "string"}],
  "nodes": [{
    "id": "build",
    "name": "build",
    "taskRef": {"kind": "ClusterTask", "name": "buildah"},
    "when": [{"input": "$(params.branch)", "operator": "in", "values": [1]}]
  }],
  "edges": []
}`

	_, err := validateTektonPipelineTemplateConfig(nil, nil, content, []string{"SUPER_ADMIN"})
	if err == nil || !strings.Contains(err.Error(), "When values") {
		t.Fatalf("non-string when values should be rejected, got %v", err)
	}

	content = `{
  "version": "tekton-dag/v1",
  "pipeline": {"name": "demo-pipeline"},
  "nodes": [{
    "id": "build",
    "name": "build",
    "taskRef": {"kind": "ClusterTask", "name": "buildah"},
    "matrix": {"params": [{"name": "platform", "values": ["linux", 1]}]}
  }],
  "edges": []
}`

	_, err = validateTektonPipelineTemplateConfig(nil, nil, content, []string{"SUPER_ADMIN"})
	if err == nil || !strings.Contains(err.Error(), "Matrix value") {
		t.Fatalf("non-string matrix values should be rejected, got %v", err)
	}
}

func TestValidateTektonPipelineTemplateConfigRejectsInvalidFixedObjectParamValue(t *testing.T) {
	content := `{
  "version": "tekton-dag/v1",
  "pipeline": {"name": "demo-pipeline"},
  "nodes": [{
    "id": "deploy",
    "name": "deploy",
    "taskRef": {"kind": "ClusterTask", "name": "kubectl"},
    "paramRequirements": [{"name": "deployConfig", "type": "object", "required": true}],
    "params": [{"name": "deployConfig", "source": "fixed", "value": "not-object"}]
  }],
  "edges": []
}`

	_, err := validateTektonPipelineTemplateConfig(nil, nil, content, []string{"SUPER_ADMIN"})
	if err == nil || !strings.Contains(err.Error(), "object") {
		t.Fatalf("invalid fixed object param value should be rejected, got %v", err)
	}

	content = `{
  "version": "tekton-dag/v1",
  "pipeline": {"name": "demo-pipeline"},
  "nodes": [{
    "id": "build",
    "name": "build",
    "taskRef": {"kind": "ClusterTask", "name": "buildah"},
    "paramRequirements": [{"name": "platforms", "type": "array", "required": true}],
    "params": [{"name": "platforms", "source": "fixed", "value": [1, 2]}]
  }],
  "edges": []
}`

	_, err = validateTektonPipelineTemplateConfig(nil, nil, content, []string{"SUPER_ADMIN"})
	if err == nil || !strings.Contains(err.Error(), "string array") {
		t.Fatalf("invalid fixed array param value should be rejected, got %v", err)
	}
}

func TestValidateTektonPipelineTemplateConfigMatrixSatisfiesRequiredParam(t *testing.T) {
	content := `{
  "version": "tekton-dag/v1",
  "pipeline": {"name": "demo-pipeline"},
  "nodes": [{
    "id": "build",
    "name": "build",
    "taskRef": {"kind": "ClusterTask", "name": "buildah"},
    "paramRequirements": [{"name": "platform", "required": true}],
    "matrix": {"params": [{"name": "platform", "values": ["linux", "arm64"]}]}
  }],
  "edges": []
}`

	if _, err := validateTektonPipelineTemplateConfig(nil, nil, content, []string{"SUPER_ADMIN"}); err != nil {
		t.Fatalf("matrix param should satisfy required task param: %v", err)
	}
}

func TestValidateTektonPipelineTemplateConfigAllowsEmptyArrayObjectDefaults(t *testing.T) {
	content := `{
  "version": "tekton-dag/v1",
  "pipeline": {"name": "demo-pipeline"},
  "params": [
    {"name": "items", "type": "array", "required": true, "default": []},
    {"name": "meta", "type": "object", "required": true, "default": {}, "properties": {"digest": {"type": "string"}}}
  ],
  "nodes": [{
    "id": "build",
    "name": "build",
    "taskRef": {"kind": "ClusterTask", "name": "buildah"},
    "paramRequirements": [
      {"name": "items", "required": true, "defaultValue": []},
      {"name": "meta", "required": true, "defaultValue": {}}
    ]
  }],
  "edges": []
}`

	if _, err := validateTektonPipelineTemplateConfig(nil, nil, content, []string{"SUPER_ADMIN"}); err != nil {
		t.Fatalf("empty array/object defaults should be accepted: %v", err)
	}
}

func TestValidateTektonPipelineTemplateConfigRejectsObjectParamResultWithoutProperties(t *testing.T) {
	content := `{
  "version": "tekton-dag/v1",
  "pipeline": {"name": "demo-pipeline"},
  "params": [{"name": "git", "type": "object"}],
  "nodes": [{
    "id": "build",
    "name": "build",
    "taskRef": {"kind": "ClusterTask", "name": "buildah"}
  }],
  "edges": []
}`

	_, err := validateTektonPipelineTemplateConfig(nil, nil, content, []string{"SUPER_ADMIN"})
	if err == nil || !strings.Contains(err.Error(), "properties") {
		t.Fatalf("object pipeline param without properties should be rejected, got %v", err)
	}

	content = `{
  "version": "tekton-dag/v1",
  "pipeline": {"name": "demo-pipeline"},
  "results": [{"name": "meta", "type": "object", "value": {"digest": "sha256:abc"}}],
  "nodes": [{
    "id": "build",
    "name": "build",
    "taskRef": {"kind": "ClusterTask", "name": "buildah"}
  }],
  "edges": []
}`

	_, err = validateTektonPipelineTemplateConfig(nil, nil, content, []string{"SUPER_ADMIN"})
	if err == nil || !strings.Contains(err.Error(), "properties") {
		t.Fatalf("object pipeline result without properties should be rejected, got %v", err)
	}
}

func TestValidateTektonPipelineTemplateConfigRejectsObjectParamNameWithDot(t *testing.T) {
	content := `{
  "version": "tekton-dag/v1",
  "pipeline": {"name": "demo-pipeline"},
  "params": [{"name": "git.url", "type": "object", "properties": {"url": {"type": "string"}}}],
  "nodes": [{
    "id": "build",
    "name": "build",
    "taskRef": {"kind": "ClusterTask", "name": "buildah"}
  }],
  "edges": []
}`

	_, err := validateTektonPipelineTemplateConfig(nil, nil, content, []string{"SUPER_ADMIN"})
	if err == nil || !strings.Contains(err.Error(), "点号") {
		t.Fatalf("object pipeline param name with dot should be rejected, got %v", err)
	}
}

func TestValidateTektonTemplateNodeParamsRejectsUnknownMatrixIncludeParam(t *testing.T) {
	node := tektonPipelineTemplateNode{
		ID:   "build",
		Name: "build",
		Matrix: map[string]any{
			"params": []any{
				map[string]any{"name": "platform", "values": []any{"linux"}},
			},
			"include": []any{
				map[string]any{
					"params": []any{
						map[string]any{"name": "unknown", "value": "extra"},
					},
				},
			},
		},
	}
	spec := tektonTemplateTaskSpec{Params: map[string]bool{"platform": false}}

	err := validateTektonTemplateNodeParams(node, spec, nil)
	if err == nil || !strings.Contains(err.Error(), "Matrix 参数不属于 Task spec.params") {
		t.Fatalf("unknown matrix include param should be rejected, got %v", err)
	}
}

func TestNormalizeGitProjectValueFromRepositoryURL(t *testing.T) {
	got := normalizeGitProjectValue("http://git.example.com/ikubeops/test-vue.git")
	if got != "ikubeops/test-vue" {
		t.Fatalf("unexpected project path: %s", got)
	}
}

func TestGitRepositoryURLWithCredential(t *testing.T) {
	got, ok := gitRepositoryURLWithCredential(
		"http://gitlab.example.com/mygroup/myproject.git",
		"zhangsan",
		"glpat-xxxxx-yyyy-zzzz",
	)
	if !ok {
		t.Fatal("expected credential url")
	}
	want := "http://zhangsan:glpat-xxxxx-yyyy-zzzz@gitlab.example.com/mygroup/myproject.git"
	if got != want {
		t.Fatalf("unexpected credential url: %s", got)
	}
}

func TestNormalizePipelineStepGraphDemotesSingleParallelBranch(t *testing.T) {
	steps := []model.PipelineTemplateStep{
		{ID: "clone", ParentNodeID: workflowStartNodeID, BranchType: branchTypeNext, SortOrder: 1},
		{ID: "npm", ParentNodeID: "clone", BranchType: branchTypeParallel, SortOrder: 2},
		{ID: "build", ParentNodeID: "clone", BranchType: branchTypeNext, SortOrder: 3},
	}

	got := normalizePipelineStepGraph(steps)
	for _, item := range got {
		if item.ID == "npm" && item.BranchType != branchTypeNext {
			t.Fatalf("single parallel branch should be demoted to next, got %s", item.BranchType)
		}
	}
}

func TestNormalizePipelineStepGraphKeepsParallelGroupAndLiftsContinuation(t *testing.T) {
	steps := []model.PipelineTemplateStep{
		{ID: "clone", ParentNodeID: workflowStartNodeID, BranchType: branchTypeNext, SortOrder: 1},
		{ID: "sonar", ParentNodeID: "clone", BranchType: branchTypeParallel, SortOrder: 2},
		{ID: "npm", ParentNodeID: "clone", BranchType: branchTypeParallel, SortOrder: 3},
		{ID: "custom", ParentNodeID: "npm", BranchType: branchTypeNext, SortOrder: 4},
	}

	got := normalizePipelineStepGraph(steps)
	byID := make(map[string]model.PipelineTemplateStep, len(got))
	for _, item := range got {
		byID[item.ID] = item
	}
	if byID["sonar"].BranchType != branchTypeParallel || byID["npm"].BranchType != branchTypeParallel {
		t.Fatalf("two parallel branches should keep parallel type: %#v", byID)
	}
	if byID["custom"].ParentNodeID != "clone" || byID["custom"].BranchType != branchTypeNext {
		t.Fatalf("parallel branch continuation should move after group, got %#v", byID["custom"])
	}
}
