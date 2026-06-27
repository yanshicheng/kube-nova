package channelvars

import (
	"encoding/json"
	"errors"
	"strings"
	"sync"
)

const (
	ParamRepositoryChannel       = "repositoryChannel"
	ParamChannelVariable         = "channelGroupCode"
	ParamGitProject              = "gitProject"
	ParamGitRepositoryURL        = "gitRepositoryUrl"
	ParamGitBranch               = "gitBranch"
	ParamGitTag                  = "gitTag"
	ParamNexusChannel            = "nexusChannel"
	ParamNexusRepository         = "nexusRepository"
	ParamNexusArtifactName       = "nexusArtifactName"
	ParamNexusArtifactVersion    = "nexusArtifactVersion"
	ParamNexusRepositoryURL      = "nexusRepositoryUrl"
	ParamNexusArtifactURL        = "nexusArtifactUrl"
	ParamNexusArtifactVersionURL = "nexusArtifactVersionUrl"
	ParamHarborChannel           = "harborChannel"
	ParamHarborProject           = "harborProject"
	ParamHarborProjectURL        = "harborProjectUrl"
	ParamHarborImage             = "harborImage"
	ParamHarborImageTag          = "harborImageTag"
	ParamHarborImageURL          = "harborImageUrl"
	ParamHarborImageTagURL       = "harborImageTagUrl"
	ParamRegistryRepository      = "registryRepository"
	ParamRegistryImage           = "registryImage"
	ParamRegistryImageTag        = "registryImageTag"
	ParamRegistryRepositoryURL   = "registryRepositoryUrl"
	ParamRegistryImageURL        = "registryImageUrl"
	ParamRegistryImageTagURL     = "registryImageTagUrl"
	ParamJfrogChannel            = "jfrogChannel"
	ParamJfrogRepository         = "jfrogRepository"
	ParamJfrogArtifact           = "jfrogArtifact"
	ParamJfrogArtifactName       = "jfrogArtifactName"
	ParamJfrogArtifactVersion    = "jfrogArtifactVersion"
	ParamJfrogRepositoryURL      = "jfrogRepositoryUrl"
	ParamJfrogArtifactURL        = "jfrogArtifactUrl"
	ParamJfrogArtifactVersionURL = "jfrogArtifactVersionUrl"
	ParamNexusArtifact           = "nexusArtifact"
	ParamChannelCredential       = "channelCredential"
	ParamKubernetesSecretName    = "kubernetesSecretName"
	ParamKubernetesPVCName       = "kubernetesPvcName"
	ParamMavenConfig             = "mavenConfig"
	ParamConfigCenter            = "configCenter"
	ParamSonarAddress            = "sonarAddress"
	ParamSonarProjectName        = "sonarProjectName"
	ParamSonarProjectKey         = "sonarProjectKey"
	ParamSonarProjectURL         = "sonarProjectUrl"
	ParamSonarProjectKeyURL      = "sonarProjectKeyUrl"
	ParamHostGroupTargets        = "hostGroupTargets"
	ParamHostConfig              = "hostConfig"
	ParamHostGroupConfig         = "hostGroupConfig"
	ParamKubeNovaDeployConfig    = "kubeNovaDeployConfig"
	ParamKubeNovaProject         = "kubeNovaProject"
	ParamKubeNovaCluster         = "kubeNovaCluster"
	ParamKubeNovaWorkspace       = "kubeNovaWorkspace"
	ParamKubeNovaApplication     = "kubeNovaApplication"
	ParamKubeNovaVersion         = "kubeNovaVersion"
	ParamKubeNovaContainer       = "kubeNovaContainer"
	ParamKubernetesDeployConfig  = "kubernetesDeployConfig"
	ParamKubernetesNamespace     = "kubernetesNamespace"
	ParamKubernetesWorkloadType  = "kubernetesWorkloadType"
	ParamKubernetesResource      = "kubernetesResource"
	ParamKubernetesContainer     = "kubernetesContainer"
	ParamKubernetesImage         = "kubernetesImage"

	DynamicHarborImage    = ParamHarborImage
	DynamicHarborImageTag = ParamHarborImageTag

	FieldEndpoint                  = "endpoint"
	FieldChannelID                 = "id"
	FieldChannelName               = "name"
	FieldChannelCode               = "code"
	FieldChannelType               = "channelType"
	FieldDescription               = "description"
	FieldGlobalCredentialID        = "globalCredentialId"
	FieldCredentialID              = "credentialId"
	FieldConfig                    = "config"
	FieldLabels                    = "labels"
	FieldHealthStatus              = "healthStatus"
	FieldLastCheckAt               = "lastCheckAt"
	FieldLastCheckMessage          = "lastCheckMessage"
	FieldMetadata                  = "metadata"
	FieldMetadataVersion           = "metadata.version"
	FieldStatus                    = "status"
	FieldAuthType                  = "authType"
	FieldInsecureSkipTLS           = "insecureSkipTls"
	FieldIcon                      = "icon"
	FieldIconColor                 = "iconColor"
	FieldCreatedBy                 = "createdBy"
	FieldUpdatedBy                 = "updatedBy"
	FieldCreatedAt                 = "createdAt"
	FieldUpdatedAt                 = "updatedAt"
	FieldDynamicProject            = "dynamic.project"
	FieldDynamicBranch             = "dynamic.branch"
	FieldDynamicTag                = "dynamic.tag"
	FieldDynamicRegistry           = "dynamic.registry"
	FieldDynamicRepository         = "dynamic.repository"
	FieldDynamicImage              = "dynamic.image"
	FieldDynamicImageTag           = "dynamic.imageTag"
	FieldDynamicArtifact           = "dynamic.artifact"
	FieldDynamicArtifactName       = "dynamic.artifactName"
	FieldDynamicArtifactVersion    = "dynamic.artifactVersion"
	FieldDynamicArtifactTag        = "dynamic.artifactTag"
	FieldDynamicProjectName        = "dynamic.projectName"
	FieldDynamicProjectKey         = "dynamic.projectKey"
	FieldDynamicPath               = "dynamic.path"
	FieldDynamicRevision           = "dynamic.revision"
	FieldDynamicCluster            = "dynamic.cluster"
	FieldDynamicWorkspace          = "dynamic.workspace"
	FieldDynamicApplication        = "dynamic.application"
	FieldDynamicVersion            = "dynamic.version"
	FieldDynamicContainer          = "dynamic.container"
	FieldDynamicContainerName      = "dynamic.containerName"
	FieldDynamicImageName          = "dynamic.imageName"
	FieldDynamicDeployConfig       = "dynamic.deployConfig"
	FieldDynamicNamespace          = "dynamic.namespace"
	FieldDynamicWorkloadType       = "dynamic.workloadType"
	FieldDynamicResource           = "dynamic.resource"
	FieldDynamicResourceName       = "dynamic.resourceName"
	FieldDynamicDeployment         = "dynamic.deployment"
	FieldDynamicStatefulSet        = "dynamic.statefulSet"
	FieldDynamicDaemonSet          = "dynamic.daemonSet"
	FieldDynamicCronJob            = "dynamic.cronJob"
	FieldDynamicHostGroup          = "dynamic.hostGroup"
	FieldAddressProjectURL         = "address.projectUrl"
	FieldAddressRegistryURL        = "address.registryUrl"
	FieldAddressImageURL           = "address.imageUrl"
	FieldAddressImageTagURL        = "address.imageTagUrl"
	FieldAddressRepositoryURL      = "address.repositoryUrl"
	FieldAddressArtifactURL        = "address.artifactUrl"
	FieldAddressArtifactVersionURL = "address.artifactVersionUrl"
	FieldAddressArtifactTagURL     = "address.artifactTagUrl"
	FieldAddressProjectKeyURL      = "address.projectKeyUrl"
	FieldAddressDeploymentConfig   = "address.deploymentConfig"
	FieldAddressDeployConfig       = "address.deployConfig"
	FieldAddressHostConfig         = "address.hostConfig"
	FieldAddressGroupConfig        = "address.groupConfig"
	FieldOptionOutputFormat        = "option.outputFormat"
	FieldOutputHostTargets         = "output.hostTargets"
)

const (
	GroupCodeRepo     = "code_repo"
	GroupArtifactRepo = "artifact_repo"
	GroupImageRepo    = "image_registry"
	GroupCodeScan     = "code_scan"
	GroupDeployTarget = "deploy_target"
	GroupBuildEngine  = "build_engine"
)

// StepChannelParamSpec 步骤参数到渠道分组的映射规格。
type StepChannelParamSpec struct {
	ParamType         string
	GroupCode         string
	ChannelTypeFilter string
}

// StepChannelParamLookup 动态查询接口，由 RPC 服务在启动时注入。
type StepChannelParamLookup interface {
	LookupGroupCode(paramType string) (groupCode string, channelTypeFilter string, found bool)
}

var (
	lookupMu    sync.RWMutex
	paramLookup StepChannelParamLookup
	// channelParamTypes 步骤参数类型到渠道分组的映射，用于 IsChannelParamType。
	// key 为 paramType，value 为 groupCode。
	channelParamTypes map[string]string
)

// SetStepChannelParamLookup 注册动态查询实现。由 RPC 服务在初始化时调用。
func SetStepChannelParamLookup(lookup StepChannelParamLookup) {
	lookupMu.Lock()
	defer lookupMu.Unlock()
	paramLookup = lookup
}

// SetChannelParamTypes 批量注册步骤参数类型映射，用于 IsChannelParamType 判断。
func SetChannelParamTypes(paramTypes []StepChannelParamSpec) {
	lookupMu.Lock()
	defer lookupMu.Unlock()
	channelParamTypes = make(map[string]string, len(paramTypes))
	for _, spec := range paramTypes {
		channelParamTypes[spec.ParamType] = spec.GroupCode
	}
}

// IsChannelParamType 判断参数类型是否为渠道参数类型。
func IsChannelParamType(paramType string) bool {
	if strings.TrimSpace(paramType) == ParamChannelVariable {
		return true
	}
	lookupMu.RLock()
	types := channelParamTypes
	lookupMu.RUnlock()
	if types != nil {
		if _, ok := types[strings.TrimSpace(paramType)]; ok {
			return true
		}
	}
	// 内部查询别名回退：页面入口已收敛为渠道变量，但运行期查询仍复用这些 provider 别名。
	switch strings.TrimSpace(paramType) {
	case ParamRepositoryChannel, ParamNexusChannel, ParamHarborChannel, ParamJfrogChannel, ParamKubeNovaDeployConfig, ParamKubernetesDeployConfig:
		return true
	default:
		return false
	}
}

// ChannelGroupCode 返回参数类型对应的渠道分组编码。
func ChannelGroupCode(paramType string) string {
	if strings.TrimSpace(paramType) == ParamChannelVariable {
		return ""
	}
	lookupMu.RLock()
	lookup := paramLookup
	lookupMu.RUnlock()
	if lookup != nil {
		groupCode, _, found := lookup.LookupGroupCode(strings.TrimSpace(paramType))
		if found {
			return groupCode
		}
	}
	// 硬编码回退。
	switch strings.TrimSpace(paramType) {
	case ParamRepositoryChannel:
		return GroupCodeRepo
	case ParamNexusChannel, ParamJfrogChannel:
		return GroupArtifactRepo
	case ParamHarborChannel:
		return GroupImageRepo
	case ParamKubeNovaDeployConfig, ParamKubernetesDeployConfig:
		return GroupDeployTarget
	default:
		return ""
	}
}

// ChannelTypeFilter 返回参数类型对应的渠道类型过滤。
func ChannelTypeFilter(paramType string) string {
	if strings.TrimSpace(paramType) == ParamChannelVariable {
		return ""
	}
	lookupMu.RLock()
	lookup := paramLookup
	lookupMu.RUnlock()
	if lookup != nil {
		_, filter, found := lookup.LookupGroupCode(strings.TrimSpace(paramType))
		if found {
			return filter
		}
	}
	// 硬编码回退。
	switch strings.TrimSpace(paramType) {
	case ParamNexusChannel:
		return "nexus"
	case ParamHarborChannel:
		return "harbor"
	case ParamJfrogChannel:
		return "jfrog"
	case ParamKubeNovaDeployConfig:
		return "kube-nova"
	case ParamKubernetesDeployConfig:
		return "kubernetes"
	default:
		return ""
	}
}

// DynamicParamTypes 返回所有动态参数类型列表。
func DynamicParamTypes() []string {
	return []string{
		ParamGitProject,
		ParamGitBranch,
		ParamGitTag,
		ParamNexusRepository,
		ParamNexusArtifactName,
		ParamNexusArtifactVersion,
		ParamNexusArtifact,
		ParamHarborProject,
		ParamHarborImage,
		ParamHarborImageTag,
		ParamRegistryRepository,
		ParamRegistryImage,
		ParamRegistryImageTag,
		ParamJfrogRepository,
		ParamJfrogArtifactName,
		ParamJfrogArtifactVersion,
		ParamJfrogArtifact,
		ParamSonarProjectName,
		ParamSonarProjectKey,
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
		ParamConfigCenter,
	}
}

// IsDynamicParamType 判断参数类型是否需要动态加载可选值。
func IsDynamicParamType(paramType string) bool {
	for _, item := range DynamicParamTypes() {
		if item == strings.TrimSpace(paramType) {
			return true
		}
	}
	return false
}

// IsCompositeAddressParamType 判断参数类型是否为复合地址类型。
func IsCompositeAddressParamType(paramType string) bool {
	switch strings.TrimSpace(paramType) {
	case ParamGitRepositoryURL,
		ParamNexusRepositoryURL, ParamNexusArtifactURL, ParamNexusArtifactVersionURL,
		ParamHarborProjectURL, ParamHarborImageURL, ParamHarborImageTagURL,
		ParamRegistryRepositoryURL, ParamRegistryImageURL, ParamRegistryImageTagURL,
		ParamJfrogRepositoryURL, ParamJfrogArtifactURL, ParamJfrogArtifactVersionURL,
		ParamSonarAddress, ParamSonarProjectURL, ParamSonarProjectKeyURL,
		ParamHostConfig, ParamHostGroupConfig:
		return true
	default:
		return false
	}
}

// IsCredentialSourceParamType 判断参数类型是否可以作为凭据来源。
func IsCredentialSourceParamType(paramType string) bool {
	return IsChannelParamType(paramType) || IsCompositeAddressParamType(paramType)
}

// IsChannelCredentialMappingField 判断字段是否为凭据映射字段。
func IsChannelCredentialMappingField(field string) bool {
	switch strings.TrimSpace(field) {
	case "username", "password", "token", "secretText", "privateKey", "passphrase", "kubeconfig", "certificate":
		return true
	default:
		return false
	}
}

type DynamicSpec struct {
	GroupCode          string
	ChannelParamType   string
	ProjectParamType   string
	RequireProject     bool
	AllowedChannelType string
}

type VariableSchema struct {
	Version int             `json:"version,omitempty"`
	Fields  []VariableField `json:"fields"`
}

var ErrLegacyVariableField = errors.New("legacy variable field")

type VariableField struct {
	GroupCode             string               `json:"groupCode,omitempty"`
	Field                 string               `json:"field"`
	Name                  string               `json:"name"`
	Kind                  string               `json:"kind,omitempty"`
	UIControl             string               `json:"uiControl,omitempty"`
	Provider              string               `json:"provider,omitempty"`
	Dependencies          []VariableDependency `json:"dependencies,omitempty"`
	AllOf                 []VariableDependency `json:"allOf,omitempty"`
	AnyOf                 []VariableDependency `json:"anyOf,omitempty"`
	AllowManualInput      bool                 `json:"allowManualInput,omitempty"`
	AddressBuilder        string               `json:"addressBuilder,omitempty"`
	Required              bool                 `json:"required,omitempty"`
	ValueType             string               `json:"valueType,omitempty"`
	OutputMode            string               `json:"outputMode,omitempty"`
	OutputTemplate        string               `json:"outputTemplate,omitempty"`
	CredentialPolicy      string               `json:"credentialPolicy,omitempty"`
	FallbackAddressFields []string             `json:"fallbackAddressFields,omitempty"`
	ResolveEndpoint       bool                 `json:"resolveEndpoint,omitempty"`
	ChannelTypes          []string             `json:"channelTypes,omitempty"`
	Options               []VariableOption     `json:"options,omitempty"`
	SortOrder             int64                `json:"sortOrder,omitempty"`
}

type VariableDependency struct {
	Field    string `json:"field"`
	Source   string `json:"source"`
	Name     string `json:"name,omitempty"`
	Required bool   `json:"required,omitempty"`
}

type VariableOption struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

type StepParamDependency struct {
	Field     string
	ParamCode string
}

func ParseVariableSchema(raw string) VariableSchema {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return VariableSchema{Version: 2}
	}
	var schema VariableSchema
	if err := json.Unmarshal([]byte(raw), &schema); err != nil {
		return VariableSchema{Version: 2}
	}
	if schema.Version == 0 {
		schema.Version = 2
	}
	for i := range schema.Fields {
		schema.Fields[i] = normalizeVariableField(schema.Fields[i])
	}
	return schema
}

func FindVariableField(raw, field string) (VariableField, bool) {
	return FindVariableFieldInList(ParseVariableSchema(raw).Fields, field)
}

func FindVariableFieldInList(fields []VariableField, field string) (VariableField, bool) {
	field = strings.TrimSpace(field)
	for _, item := range fields {
		if strings.TrimSpace(item.Field) == field {
			return normalizeVariableField(item), true
		}
	}
	return VariableField{}, false
}

func DefaultVariableFields() []VariableField {
	return []VariableField{
		{Field: FieldChannelID, Name: "渠道 ID", Kind: "default", UIControl: "input", AllowManualInput: true},
		{Field: FieldChannelName, Name: "渠道名称", Kind: "default", UIControl: "input", AllowManualInput: true},
		{Field: FieldChannelCode, Name: "渠道编码", Kind: "default", UIControl: "input", AllowManualInput: true},
		{Field: FieldEndpoint, Name: "渠道实例", Kind: "default", UIControl: "channel", AllowManualInput: true},
	}
}

func MergeVariableFields(custom []VariableField) []VariableField {
	result := make([]VariableField, 0, len(custom)+4)
	seen := map[string]struct{}{}
	for _, item := range DefaultVariableFields() {
		item = normalizeVariableField(item)
		result = append(result, item)
		seen[item.Field] = struct{}{}
	}
	for _, item := range custom {
		item = normalizeVariableField(item)
		if strings.TrimSpace(item.Field) == "" {
			continue
		}
		if _, ok := seen[item.Field]; ok {
			continue
		}
		result = append(result, item)
		seen[item.Field] = struct{}{}
	}
	return result
}

func NormalizeVariableSchema(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return variableSchemaJSON(DefaultVariableFields())
	}
	var schema VariableSchema
	if err := json.Unmarshal([]byte(raw), &schema); err != nil {
		return "", err
	}
	if schema.Version == 0 {
		schema.Version = 2
	}
	for _, field := range schema.Fields {
		if IsLegacyVariableField(field.Field) {
			return "", ErrLegacyVariableField
		}
	}
	for i := range schema.Fields {
		schema.Fields[i] = normalizeVariableField(schema.Fields[i])
	}
	schema.Fields = MergeVariableFields(schema.Fields)
	data, err := json.Marshal(schema)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func variableSchemaJSON(fields []VariableField) (string, error) {
	data, err := json.Marshal(VariableSchema{Version: 2, Fields: MergeVariableFields(fields)})
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func VariableSchemaJSON(fields []VariableField) string {
	data, err := json.Marshal(VariableSchema{Version: 2, Fields: MergeVariableFields(fields)})
	if err != nil {
		return `{"version":2,"fields":[]}`
	}
	return string(data)
}

func IsLegacyVariableField(field string) bool {
	field = strings.TrimSpace(field)
	return strings.HasPrefix(field, "dynamic.") && strings.HasSuffix(field, "Url")
}

func IsAddressVariableField(field VariableField) bool {
	field = normalizeVariableField(field)
	return field.Kind == "address" || field.UIControl == "addressPicker" || field.UIControl == "cascadePicker"
}

func RequiredParamDependencies(field VariableField) []VariableDependency {
	field = normalizeVariableField(field)
	result := make([]VariableDependency, 0, len(field.Dependencies))
	for _, item := range field.Dependencies {
		if strings.TrimSpace(item.Source) != "param" {
			continue
		}
		if strings.TrimSpace(item.Field) == "" {
			continue
		}
		if !item.Required && IsAddressVariableField(field) {
			continue
		}
		result = append(result, item)
	}
	return result
}

func DependencyAllOf(field VariableField) []VariableDependency {
	field = normalizeVariableField(field)
	if len(field.AllOf) > 0 {
		return field.AllOf
	}
	result := make([]VariableDependency, 0, len(field.Dependencies))
	for _, dep := range field.Dependencies {
		if strings.TrimSpace(dep.Field) == "" || !dep.Required {
			continue
		}
		if strings.TrimSpace(dep.Source) == "channel" && strings.TrimSpace(dep.Field) == FieldEndpoint {
			continue
		}
		result = append(result, dep)
	}
	return result
}

func DependencyAnyOf(field VariableField) []VariableDependency {
	field = normalizeVariableField(field)
	if len(field.AnyOf) > 0 {
		return field.AnyOf
	}
	result := make([]VariableDependency, 0, len(field.FallbackAddressFields)+1)
	for _, dep := range field.Dependencies {
		if strings.TrimSpace(dep.Source) == "channel" && strings.TrimSpace(dep.Field) == FieldEndpoint {
			result = append(result, dep)
			break
		}
	}
	for _, item := range field.FallbackAddressFields {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		result = append(result, VariableDependency{Field: item, Source: "address", Name: item, Required: true})
	}
	return result
}

func normalizeVariableField(field VariableField) VariableField {
	field.Field = strings.TrimSpace(field.Field)
	field.Name = strings.TrimSpace(field.Name)
	if field.Name == "" {
		field.Name = field.Field
	}
	field.Kind = strings.TrimSpace(field.Kind)
	field.UIControl = strings.TrimSpace(field.UIControl)
	field.Provider = strings.TrimSpace(field.Provider)
	field.AddressBuilder = strings.TrimSpace(field.AddressBuilder)
	field.ValueType = strings.TrimSpace(field.ValueType)
	field.OutputMode = strings.TrimSpace(field.OutputMode)
	field.OutputTemplate = strings.TrimSpace(field.OutputTemplate)
	if field.Kind == "" {
		switch {
		case strings.HasPrefix(field.Field, "dynamic."):
			field.Kind = "dynamic"
		case strings.HasPrefix(field.Field, "address."):
			field.Kind = "address"
		case strings.HasPrefix(field.Field, "credential."):
			field.Kind = "credential"
		case strings.HasPrefix(field.Field, "output."):
			field.Kind = "output"
		case strings.HasPrefix(field.Field, "option."):
			field.Kind = "option"
		default:
			field.Kind = "default"
		}
	}
	if field.UIControl == "" {
		switch field.Kind {
		case "address":
			field.UIControl = "cascadePicker"
		case "dynamic":
			field.UIControl = "select"
		case "option":
			field.UIControl = "radio"
		case "output":
			field.UIControl = "output"
		case "composite":
			field.UIControl = "deployConfig"
		default:
			field.UIControl = "readonly"
		}
	}
	if field.ValueType == "" {
		field.ValueType = "string"
	}
	if field.Kind == "address" && field.OutputMode == "" {
		field.OutputMode = "provider"
	}
	if field.Kind == "output" && field.OutputMode == "" {
		field.OutputMode = "provider"
	}
	for i := range field.Dependencies {
		field.Dependencies[i].Field = strings.TrimSpace(field.Dependencies[i].Field)
		field.Dependencies[i].Source = strings.TrimSpace(field.Dependencies[i].Source)
		field.Dependencies[i].Name = strings.TrimSpace(field.Dependencies[i].Name)
	}
	for i := range field.AllOf {
		field.AllOf[i].Field = strings.TrimSpace(field.AllOf[i].Field)
		field.AllOf[i].Source = strings.TrimSpace(field.AllOf[i].Source)
		field.AllOf[i].Name = strings.TrimSpace(field.AllOf[i].Name)
	}
	for i := range field.AnyOf {
		field.AnyOf[i].Field = strings.TrimSpace(field.AnyOf[i].Field)
		field.AnyOf[i].Source = strings.TrimSpace(field.AnyOf[i].Source)
		field.AnyOf[i].Name = strings.TrimSpace(field.AnyOf[i].Name)
	}
	if isAddressVariableKind(field) {
		field.Dependencies = addressEndpointDependencies(field.Dependencies)
		field.AllOf = nil
		field.AnyOf = nil
		field.ResolveEndpoint = true
	}
	switch field.Kind {
	case "dynamic", "address", "credential", "output", "option", "composite":
		field.AllowManualInput = true
	}
	for i := range field.FallbackAddressFields {
		field.FallbackAddressFields[i] = strings.TrimSpace(field.FallbackAddressFields[i])
	}
	for i := range field.ChannelTypes {
		field.ChannelTypes[i] = strings.TrimSpace(field.ChannelTypes[i])
	}
	return field
}

func isAddressVariableKind(field VariableField) bool {
	return field.Kind == "address" || field.UIControl == "addressPicker" || field.UIControl == "cascadePicker"
}

func addressEndpointDependencies(deps []VariableDependency) []VariableDependency {
	result := make([]VariableDependency, 0, 1)
	for _, dep := range deps {
		if strings.TrimSpace(dep.Source) == "channel" && strings.TrimSpace(dep.Field) == FieldEndpoint {
			result = append(result, dep)
			break
		}
	}
	if len(result) == 0 {
		result = append(result, VariableDependency{Field: FieldEndpoint, Source: "channel", Name: "渠道实例", Required: true})
	}
	return result
}

// DynamicSpecFor 返回动态参数类型的规格信息。
func DynamicSpecFor(paramType string) (DynamicSpec, bool) {
	switch strings.TrimSpace(paramType) {
	case ParamGitProject:
		return DynamicSpec{GroupCode: GroupCodeRepo, ChannelParamType: ParamRepositoryChannel}, true
	case ParamGitBranch, ParamGitTag:
		return DynamicSpec{
			GroupCode:        GroupCodeRepo,
			ChannelParamType: ParamRepositoryChannel,
			ProjectParamType: ParamGitProject,
			RequireProject:   true,
		}, true
	case ParamNexusRepository:
		return DynamicSpec{
			GroupCode:          GroupArtifactRepo,
			ChannelParamType:   ParamNexusChannel,
			AllowedChannelType: "nexus",
		}, true
	case ParamNexusArtifactName, ParamNexusArtifact:
		return DynamicSpec{
			GroupCode:          GroupArtifactRepo,
			ChannelParamType:   ParamNexusChannel,
			ProjectParamType:   ParamNexusRepository,
			RequireProject:     true,
			AllowedChannelType: "nexus",
		}, true
	case ParamNexusArtifactVersion:
		return DynamicSpec{
			GroupCode:          GroupArtifactRepo,
			ChannelParamType:   ParamNexusChannel,
			ProjectParamType:   ParamNexusRepository,
			RequireProject:     true,
			AllowedChannelType: "nexus",
		}, true
	case ParamHarborProject:
		return DynamicSpec{
			GroupCode:          GroupImageRepo,
			ChannelParamType:   ParamHarborChannel,
			AllowedChannelType: "harbor",
		}, true
	case ParamHarborImage:
		return DynamicSpec{
			GroupCode:          GroupImageRepo,
			ChannelParamType:   ParamHarborChannel,
			ProjectParamType:   ParamHarborProject,
			RequireProject:     true,
			AllowedChannelType: "harbor",
		}, true
	case ParamHarborImageTag:
		return DynamicSpec{
			GroupCode:          GroupImageRepo,
			ChannelParamType:   ParamHarborChannel,
			ProjectParamType:   ParamHarborProject,
			RequireProject:     true,
			AllowedChannelType: "harbor",
		}, true
	case ParamRegistryRepository:
		return DynamicSpec{
			GroupCode:          GroupImageRepo,
			ChannelParamType:   ParamChannelVariable,
			AllowedChannelType: "registry",
		}, true
	case ParamRegistryImage:
		return DynamicSpec{
			GroupCode:          GroupImageRepo,
			ChannelParamType:   ParamChannelVariable,
			ProjectParamType:   ParamRegistryRepository,
			RequireProject:     true,
			AllowedChannelType: "registry",
		}, true
	case ParamRegistryImageTag:
		return DynamicSpec{
			GroupCode:          GroupImageRepo,
			ChannelParamType:   ParamChannelVariable,
			ProjectParamType:   ParamRegistryRepository,
			RequireProject:     true,
			AllowedChannelType: "registry",
		}, true
	case ParamJfrogRepository:
		return DynamicSpec{
			GroupCode:          GroupArtifactRepo,
			ChannelParamType:   ParamJfrogChannel,
			AllowedChannelType: "jfrog",
		}, true
	case ParamJfrogArtifactName, ParamJfrogArtifact:
		return DynamicSpec{
			GroupCode:          GroupArtifactRepo,
			ChannelParamType:   ParamJfrogChannel,
			ProjectParamType:   ParamJfrogRepository,
			RequireProject:     true,
			AllowedChannelType: "jfrog",
		}, true
	case ParamJfrogArtifactVersion:
		return DynamicSpec{
			GroupCode:          GroupArtifactRepo,
			ChannelParamType:   ParamJfrogChannel,
			ProjectParamType:   ParamJfrogRepository,
			RequireProject:     true,
			AllowedChannelType: "jfrog",
		}, true
	case ParamSonarProjectName, ParamSonarProjectKey:
		return DynamicSpec{
			GroupCode:          GroupCodeScan,
			ChannelParamType:   ParamSonarAddress,
			AllowedChannelType: "sonarqube",
		}, true
	case ParamKubeNovaProject:
		return DynamicSpec{
			GroupCode:          GroupDeployTarget,
			ChannelParamType:   ParamKubeNovaDeployConfig,
			AllowedChannelType: "kube-nova",
		}, true
	case ParamKubeNovaCluster:
		return DynamicSpec{
			GroupCode:          GroupDeployTarget,
			ChannelParamType:   ParamKubeNovaDeployConfig,
			ProjectParamType:   ParamKubeNovaProject,
			RequireProject:     true,
			AllowedChannelType: "kube-nova",
		}, true
	case ParamKubeNovaWorkspace:
		return DynamicSpec{
			GroupCode:          GroupDeployTarget,
			ChannelParamType:   ParamKubeNovaDeployConfig,
			ProjectParamType:   ParamKubeNovaCluster,
			RequireProject:     true,
			AllowedChannelType: "kube-nova",
		}, true
	case ParamKubeNovaApplication:
		return DynamicSpec{
			GroupCode:          GroupDeployTarget,
			ChannelParamType:   ParamKubeNovaDeployConfig,
			ProjectParamType:   ParamKubeNovaWorkspace,
			RequireProject:     true,
			AllowedChannelType: "kube-nova",
		}, true
	case ParamKubeNovaVersion:
		return DynamicSpec{
			GroupCode:          GroupDeployTarget,
			ChannelParamType:   ParamKubeNovaDeployConfig,
			ProjectParamType:   ParamKubeNovaApplication,
			RequireProject:     true,
			AllowedChannelType: "kube-nova",
		}, true
	case ParamKubeNovaContainer:
		return DynamicSpec{
			GroupCode:          GroupDeployTarget,
			ChannelParamType:   ParamKubeNovaDeployConfig,
			ProjectParamType:   ParamKubeNovaVersion,
			RequireProject:     true,
			AllowedChannelType: "kube-nova",
		}, true
	case ParamKubernetesNamespace:
		return DynamicSpec{
			GroupCode:          GroupDeployTarget,
			ChannelParamType:   ParamKubernetesDeployConfig,
			AllowedChannelType: "kubernetes",
		}, true
	case ParamKubernetesWorkloadType:
		return DynamicSpec{
			GroupCode:          GroupDeployTarget,
			ChannelParamType:   ParamKubernetesDeployConfig,
			AllowedChannelType: "kubernetes",
		}, true
	case ParamKubernetesResource:
		return DynamicSpec{
			GroupCode:          GroupDeployTarget,
			ChannelParamType:   ParamKubernetesDeployConfig,
			ProjectParamType:   ParamKubernetesNamespace,
			RequireProject:     true,
			AllowedChannelType: "kubernetes",
		}, true
	case ParamKubernetesContainer:
		return DynamicSpec{
			GroupCode:          GroupDeployTarget,
			ChannelParamType:   ParamKubernetesDeployConfig,
			ProjectParamType:   ParamKubernetesResource,
			RequireProject:     true,
			AllowedChannelType: "kubernetes",
		}, true
	case ParamKubernetesImage:
		return DynamicSpec{
			GroupCode:          GroupDeployTarget,
			ChannelParamType:   ParamKubernetesDeployConfig,
			ProjectParamType:   ParamKubernetesContainer,
			RequireProject:     true,
			AllowedChannelType: "kubernetes",
		}, true
	default:
		return DynamicSpec{}, false
	}
}
