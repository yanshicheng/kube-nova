package channelvars

import (
	"strings"
	"testing"
)

func TestIsChannelParamTypeKeepsInternalAliasFallback(t *testing.T) {
	SetChannelParamTypes(nil)

	cases := []string{
		ParamRepositoryChannel,
		ParamNexusChannel,
		ParamHarborChannel,
		ParamJfrogChannel,
		ParamKubeNovaDeployConfig,
		ParamKubernetesDeployConfig,
	}
	for _, paramType := range cases {
		if !IsChannelParamType(paramType) {
			t.Fatalf("%s should be treated as internal channel query alias", paramType)
		}
	}
}

func TestIsCompositeAddressParamTypeIncludesChannelVariableAddressTypes(t *testing.T) {
	cases := []string{
		ParamGitRepositoryURL,
		ParamNexusRepositoryURL,
		ParamNexusArtifactURL,
		ParamNexusArtifactVersionURL,
		ParamHarborProjectURL,
		ParamHarborImageURL,
		ParamHarborImageTagURL,
		ParamRegistryRepositoryURL,
		ParamRegistryImageURL,
		ParamRegistryImageTagURL,
		ParamJfrogRepositoryURL,
		ParamJfrogArtifactURL,
		ParamJfrogArtifactVersionURL,
		ParamSonarProjectURL,
		ParamSonarProjectKeyURL,
		ParamHostConfig,
		ParamHostGroupConfig,
	}
	for _, paramType := range cases {
		if !IsCompositeAddressParamType(paramType) {
			t.Fatalf("%s should be treated as composite address param", paramType)
		}
	}
}

func TestStandardVariableFieldsAllowManualInput(t *testing.T) {
	cases := []struct {
		groupCode   string
		channelType string
	}{
		{GroupCodeRepo, "gitlab"},
		{GroupCodeRepo, "svn"},
		{GroupImageRepo, "harbor"},
		{GroupImageRepo, "registry"},
		{GroupArtifactRepo, "nexus"},
		{GroupArtifactRepo, "jfrog"},
		{GroupCodeScan, "sonarqube"},
		{GroupDeployTarget, "kubernetes"},
		{GroupDeployTarget, "host"},
		{GroupDeployTarget, "host_group"},
		{GroupDeployTarget, "kube-nova"},
	}
	for _, tc := range cases {
		fields := StandardVariableFields(tc.groupCode, tc.channelType)
		if len(fields) == 0 {
			t.Fatalf("%s/%s should have standard fields", tc.groupCode, tc.channelType)
		}
		for _, field := range fields {
			if field.Kind == "default" {
				continue
			}
			if !field.AllowManualInput {
				t.Fatalf("%s/%s field %s should allow manual input", tc.groupCode, tc.channelType, field.Field)
			}
		}
	}
}

func TestDynamicParamTypesIncludeStandardDeployFields(t *testing.T) {
	cases := []string{
		ParamNexusArtifactName,
		ParamNexusArtifactVersion,
		ParamHarborImageTag,
		ParamRegistryRepository,
		ParamRegistryImage,
		ParamRegistryImageTag,
		ParamJfrogArtifactName,
		ParamJfrogArtifactVersion,
		ParamKubeNovaProject,
		ParamKubeNovaCluster,
		ParamKubeNovaWorkspace,
		ParamKubeNovaApplication,
		ParamKubeNovaVersion,
		ParamKubeNovaContainer,
		ParamKubernetesNamespace,
		ParamKubernetesWorkloadType,
		ParamKubernetesResource,
		ParamKubernetesContainer,
		ParamKubernetesImage,
	}
	for _, paramType := range cases {
		if !IsDynamicParamType(paramType) {
			t.Fatalf("%s should be registered as dynamic param type", paramType)
		}
		if _, ok := DynamicSpecFor(paramType); !ok {
			t.Fatalf("%s should have dynamic spec", paramType)
		}
	}
}

func TestStandardVariableFieldsIncludeDefaultsAndEndpointDependency(t *testing.T) {
	cases := []struct {
		groupCode   string
		channelType string
	}{
		{GroupCodeRepo, "gitlab"},
		{GroupCodeRepo, "svn"},
		{GroupImageRepo, "harbor"},
		{GroupImageRepo, "registry"},
		{GroupArtifactRepo, "nexus"},
		{GroupArtifactRepo, "jfrog"},
		{GroupCodeScan, "sonarqube"},
		{GroupDeployTarget, "kubernetes"},
		{GroupDeployTarget, "host"},
		{GroupDeployTarget, "host_group"},
		{GroupDeployTarget, "kube-nova"},
	}
	for _, tc := range cases {
		fields := MergeVariableFields(StandardVariableFields(tc.groupCode, tc.channelType))
		for _, field := range []string{FieldChannelID, FieldChannelName, FieldChannelCode, FieldEndpoint} {
			if _, ok := FindVariableFieldInList(fields, field); !ok {
				t.Fatalf("%s/%s should contain default field %s", tc.groupCode, tc.channelType, field)
			}
		}
		for _, field := range fields {
			if field.Kind != "dynamic" && field.Kind != "address" && field.Kind != "composite" && field.Kind != "output" {
				continue
			}
			if field.Provider == "" {
				continue
			}
			if !hasEndpointDependency(field.Dependencies) {
				t.Fatalf("%s/%s field %s should depend on endpoint", tc.groupCode, tc.channelType, field.Field)
			}
		}
	}
}

func TestDefaultVariableFieldsOnlyExposeCommonCodeNameEndpoint(t *testing.T) {
	fields := DefaultVariableFields()
	got := make([]string, 0, len(fields))
	for _, field := range fields {
		got = append(got, field.Field)
	}
	want := []string{FieldChannelID, FieldChannelName, FieldChannelCode, FieldEndpoint}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("default fields = %v, want %v", got, want)
	}
}

func TestAddressVariableFieldsOnlyDependOnEndpoint(t *testing.T) {
	cases := []struct {
		groupCode   string
		channelType string
	}{
		{GroupCodeRepo, "gitlab"},
		{GroupCodeRepo, "svn"},
		{GroupImageRepo, "harbor"},
		{GroupImageRepo, "registry"},
		{GroupArtifactRepo, "nexus"},
		{GroupArtifactRepo, "jfrog"},
		{GroupCodeScan, "sonarqube"},
		{GroupDeployTarget, "host"},
		{GroupDeployTarget, "host_group"},
	}
	for _, tc := range cases {
		fields := StandardVariableFields(tc.groupCode, tc.channelType)
		for _, field := range fields {
			if !IsAddressVariableField(field) {
				continue
			}
			if len(RequiredParamDependencies(field)) != 0 {
				t.Fatalf("%s/%s address field %s should not expose param dependencies", tc.groupCode, tc.channelType, field.Field)
			}
			if len(DependencyAllOf(field)) != 0 {
				t.Fatalf("%s/%s address field %s should not require external params", tc.groupCode, tc.channelType, field.Field)
			}
			if !hasEndpointDependency(field.Dependencies) {
				t.Fatalf("%s/%s address field %s should depend on endpoint", tc.groupCode, tc.channelType, field.Field)
			}
		}
	}
}

func TestStandardVariableFieldsIncludeKubernetesDeployConfig(t *testing.T) {
	fields := StandardVariableFields(GroupDeployTarget, "kubernetes")
	field, ok := FindVariableFieldInList(fields, FieldAddressDeploymentConfig)
	if !ok {
		t.Fatalf("kubernetes standard fields should contain %s", FieldAddressDeploymentConfig)
	}
	if field.UIControl != "deployConfig" {
		t.Fatalf("kubernetes deployConfig should use deployConfig control, got %s", field.UIControl)
	}
	if field.Provider != "kubernetes.deploymentConfig" {
		t.Fatalf("kubernetes deployConfig provider = %s", field.Provider)
	}
	if !hasEndpointDependency(field.Dependencies) {
		t.Fatalf("kubernetes deployConfig should depend on endpoint")
	}
}

func TestStandardVariableFieldsIncludeHostConfigs(t *testing.T) {
	cases := []struct {
		channelType string
		field       string
		provider    string
	}{
		{"host", FieldAddressHostConfig, "host.hostConfig"},
		{"host_group", FieldAddressGroupConfig, "hostGroup.groupConfig"},
	}
	for _, tc := range cases {
		fields := StandardVariableFields(GroupDeployTarget, tc.channelType)
		field, ok := FindVariableFieldInList(fields, tc.field)
		if !ok {
			t.Fatalf("%s standard fields should contain %s", tc.channelType, tc.field)
		}
		if field.Kind != "address" || field.UIControl != "cascadePicker" {
			t.Fatalf("%s should be address cascadePicker, got kind=%s control=%s", tc.field, field.Kind, field.UIControl)
		}
		if field.Provider != tc.provider {
			t.Fatalf("%s provider = %s, want %s", tc.field, field.Provider, tc.provider)
		}
		if !field.AllowManualInput {
			t.Fatalf("%s should allow manual input", tc.field)
		}
		if !hasEndpointDependency(field.Dependencies) {
			t.Fatalf("%s should depend on endpoint", tc.field)
		}
	}
}

func TestStandardVariableFieldsDoNotExposeLegacyHostGroupFields(t *testing.T) {
	fields := StandardVariableFields(GroupDeployTarget, "host_group")
	for _, field := range []string{FieldDynamicHostGroup, FieldOptionOutputFormat, FieldOutputHostTargets, "config.hostIds"} {
		if _, ok := FindVariableFieldInList(fields, field); ok {
			t.Fatalf("host_group should not expose legacy field %s", field)
		}
	}
}

func TestStandardVariableFieldsUseAddressPrefix(t *testing.T) {
	fields := StandardVariableFields(GroupImageRepo, "harbor")
	got := map[string]bool{}
	for _, field := range fields {
		got[field.Field] = true
	}
	for _, field := range []string{FieldAddressProjectURL, FieldAddressImageURL, FieldAddressImageTagURL} {
		if !got[field] {
			t.Fatalf("standard image registry fields should contain %s", field)
		}
	}
	for _, field := range fields {
		if strings.HasPrefix(field.Field, "dynamic.") && strings.HasSuffix(field.Field, "Url") {
			t.Fatalf("standard image registry fields should not contain legacy url field %s", field.Field)
		}
	}
}

func TestNormalizeVariableSchemaRejectsLegacyURLFields(t *testing.T) {
	_, err := NormalizeVariableSchema(`{"fields":[{"field":"dynamic.imageUrl","name":"镜像地址","kind":"address"}]}`)
	if err != ErrLegacyVariableField {
		t.Fatalf("NormalizeVariableSchema error = %v, want ErrLegacyVariableField", err)
	}
}

func TestNormalizeVariableSchemaForcesManualInput(t *testing.T) {
	raw := `{"fields":[{"field":"dynamic.image","name":"镜像","kind":"dynamic","allowManualInput":false}]}`
	normalized, err := NormalizeVariableSchema(raw)
	if err != nil {
		t.Fatalf("NormalizeVariableSchema error: %v", err)
	}
	field, ok := FindVariableField(normalized, FieldDynamicImage)
	if !ok {
		t.Fatalf("normalized schema should contain %s", FieldDynamicImage)
	}
	if !field.AllowManualInput {
		t.Fatalf("normalized dynamic field should allow manual input")
	}
}

func TestNormalizeVariableSchemaStripsAddressParamDependencies(t *testing.T) {
	raw := `{"fields":[{"field":"address.projectUrl","name":"仓库地址","kind":"address","dependencies":[{"field":"endpoint","source":"channel","name":"渠道实例","required":true},{"field":"dynamic.project","source":"param","name":"项目","required":true}],"allOf":[{"field":"dynamic.project","source":"param","name":"项目","required":true}]}]}`
	normalized, err := NormalizeVariableSchema(raw)
	if err != nil {
		t.Fatalf("NormalizeVariableSchema error: %v", err)
	}
	field, ok := FindVariableField(normalized, FieldAddressProjectURL)
	if !ok {
		t.Fatalf("normalized schema should contain %s", FieldAddressProjectURL)
	}
	if len(RequiredParamDependencies(field)) != 0 {
		t.Fatalf("normalized address field should not expose param dependencies")
	}
	if len(DependencyAllOf(field)) != 0 {
		t.Fatalf("normalized address field should not require external params")
	}
	if !hasEndpointDependency(field.Dependencies) {
		t.Fatalf("normalized address field should depend on endpoint")
	}
}

func TestNormalizeVariableSchemaStoresDefaultFields(t *testing.T) {
	normalized, err := NormalizeVariableSchema(`{"fields":[{"field":"config.project","name":"默认项目"}]}`)
	if err != nil {
		t.Fatalf("NormalizeVariableSchema error: %v", err)
	}
	for _, field := range []string{FieldChannelID, FieldChannelName, FieldChannelCode, FieldEndpoint, "config.project"} {
		if _, ok := FindVariableField(normalized, field); !ok {
			t.Fatalf("normalized schema should contain %s", field)
		}
	}
}
