package executionservicelogic

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/yanshicheng/kube-nova/application/devops-pipeline-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/devops-pipeline-rpc/internal/svc"
	devopstekton "github.com/yanshicheng/kube-nova/common/devops/tekton"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
	"github.com/zeromicro/go-zero/core/logx"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/yaml"
)

type tektonDagConfigDoc struct {
	Version      string                    `json:"version"`
	Pipeline     tektonDagPipeline         `json:"pipeline"`
	Params       []tektonDagPipelineParam  `json:"params"`
	Results      []tektonDagPipelineResult `json:"results"`
	Workspaces   []tektonDagWorkspace      `json:"workspaces"`
	Nodes        []tektonDagNode           `json:"nodes"`
	Edges        []tektonDagEdge           `json:"edges"`
	FinallyNodes []tektonDagNode           `json:"finallyNodes"`
	RunPolicy    tektonDagRunPolicy        `json:"runPolicy"`
	Labels       tektonDagStringMap        `json:"labels"`
	Annotations  tektonDagStringMap        `json:"annotations"`
}

type tektonDagPipeline struct {
	Name               string                    `json:"name"`
	Namespace          string                    `json:"namespace"`
	DisplayName        string                    `json:"displayName"`
	Description        string                    `json:"description"`
	ServiceAccountName string                    `json:"serviceAccountName"`
	Timeout            string                    `json:"timeout"`
	Timeouts           map[string]string         `json:"timeouts"`
	Params             []tektonDagPipelineParam  `json:"params"`
	Results            []tektonDagPipelineResult `json:"results"`
	Workspaces         []tektonDagWorkspace      `json:"workspaces"`
	Labels             tektonDagStringMap        `json:"labels"`
	Annotations        tektonDagStringMap        `json:"annotations"`
}

type tektonDagStringMap map[string]string

func (m *tektonDagStringMap) UnmarshalJSON(data []byte) error {
	text := strings.TrimSpace(string(data))
	if text == "" || text == "null" {
		*m = nil
		return nil
	}
	var obj map[string]string
	if err := json.Unmarshal(data, &obj); err == nil {
		*m = tektonDagStringMap(obj)
		return nil
	}
	var items []tektonDagNameValue
	if err := json.Unmarshal(data, &items); err != nil {
		return err
	}
	result := map[string]string{}
	for _, item := range items {
		name := strings.TrimSpace(item.Name)
		value := strings.TrimSpace(fmt.Sprint(item.Value))
		if name == "" || value == "" {
			continue
		}
		result[name] = value
	}
	*m = tektonDagStringMap(result)
	return nil
}

type tektonDagRunPolicy struct {
	ServiceAccountName string               `json:"serviceAccountName"`
	ManagedBy          string               `json:"managedBy"`
	Timeout            string               `json:"timeout"`
	PipelineTimeout    string               `json:"pipelineTimeout"`
	TasksTimeout       string               `json:"tasksTimeout"`
	FinallyTimeout     string               `json:"finallyTimeout"`
	Timeouts           map[string]string    `json:"timeouts"`
	Workspaces         []tektonDagWorkspace `json:"workspaces"`
	Labels             []tektonDagNameValue `json:"labels"`
	Annotations        []tektonDagNameValue `json:"annotations"`
	PodTemplateEnv     []tektonDagNameValue `json:"podTemplateEnv"`
	PodTemplateRaw     string               `json:"podTemplateRaw"`
	TaskRunTemplateRaw string               `json:"taskRunTemplateRaw"`
	TaskRunSpecs       []map[string]any     `json:"taskRunSpecs"`
	TaskRunSpecsRaw    string               `json:"taskRunSpecsRaw"`
	CleanBeforeRun     bool                 `json:"cleanBeforeRun"`
	CleanAfterRun      bool                 `json:"cleanAfterRun"`
	CleanupImage       string               `json:"cleanupImage"`
	ConcurrencyPolicy  string               `json:"concurrencyPolicy"`
	WorkspaceStrategy  string               `json:"workspaceStrategy"`
}

type tektonDagPipelineParam struct {
	Name          string                            `json:"name"`
	DisplayName   string                            `json:"displayName"`
	Type          string                            `json:"type"`
	Enum          []any                             `json:"enum"`
	Properties    map[string]tektonDagParamProperty `json:"properties"`
	Default       any                               `json:"default"`
	DefaultValue  any                               `json:"defaultValue"`
	Description   string                            `json:"description"`
	Required      bool                              `json:"required"`
	RuntimeConfig bool                              `json:"runtimeConfig"`
	Readonly      bool                              `json:"readonly"`
	ParamType     string                            `json:"paramType"`
	CurrentValue  any                               `json:"currentValue"`
}

type tektonDagParamProperty struct {
	Type string `json:"type"`
}

type tektonDagPipelineResult struct {
	Name        string                            `json:"name"`
	Type        string                            `json:"type"`
	Properties  map[string]tektonDagParamProperty `json:"properties"`
	Description string                            `json:"description"`
	Value       any                               `json:"value"`
}

type tektonDagParamRequirement struct {
	Name          string `json:"name"`
	Type          string `json:"type"`
	DisplayName   string `json:"displayName"`
	Required      bool   `json:"required"`
	DefaultValue  any    `json:"defaultValue"`
	RuntimeConfig bool   `json:"runtimeConfig"`
	Readonly      bool   `json:"readonly"`
}

type tektonDagWorkspaceRequirement struct {
	Name                string `json:"name"`
	Optional            bool   `json:"optional"`
	RequiredBindingType string `json:"requiredBindingType"`
}

type tektonDagWorkspace struct {
	Name                  string         `json:"name"`
	Description           string         `json:"description"`
	Optional              bool           `json:"optional"`
	Type                  string         `json:"type"`
	Strategy              string         `json:"strategy"`
	BindingType           string         `json:"bindingType"`
	AllowedBindingTypes   []string       `json:"allowedBindingTypes"`
	AllowedSourceTypes    []string       `json:"allowedSourceTypes"`
	AllowedPlatformTypes  []string       `json:"allowedPlatformTypes"`
	SourceType            string         `json:"sourceType"`
	PlatformType          string         `json:"platformType"`
	ResourceID            string         `json:"resourceId"`
	ResourceName          string         `json:"resourceName"`
	RuntimeConfig         bool           `json:"runtimeConfig"`
	ClaimName             string         `json:"claimName"`
	SecretName            string         `json:"secretName"`
	ConfigMapName         string         `json:"configMapName"`
	StorageClassName      string         `json:"storageClassName"`
	Storage               string         `json:"storage"`
	AccessMode            string         `json:"accessMode"`
	VolumeMode            string         `json:"volumeMode"`
	SubPath               string         `json:"subPath"`
	Binding               map[string]any `json:"binding"`
	EmptyDir              map[string]any `json:"emptyDir"`
	PersistentVolumeClaim map[string]any `json:"persistentVolumeClaim"`
	VolumeClaimTemplate   map[string]any `json:"volumeClaimTemplate"`
	Secret                map[string]any `json:"secret"`
	ConfigMap             map[string]any `json:"configMap"`
	Projected             map[string]any `json:"projected"`
	Config                map[string]any `json:"config"`
}

type tektonDagNode struct {
	ID                    string                          `json:"id"`
	Type                  string                          `json:"type"`
	Name                  string                          `json:"name"`
	TaskName              string                          `json:"taskName"`
	DisplayName           string                          `json:"displayName"`
	Description           string                          `json:"description"`
	StepCode              string                          `json:"stepCode"`
	TaskRef               tektonDagTaskRef                `json:"taskRef"`
	TaskSpec              map[string]any                  `json:"taskSpec"`
	PipelineRef           tektonDagPipelineRef            `json:"pipelineRef"`
	PipelineSpec          map[string]any                  `json:"pipelineSpec"`
	Params                json.RawMessage                 `json:"params"`
	ParamRequirements     []tektonDagParamRequirement     `json:"paramRequirements"`
	Workspaces            json.RawMessage                 `json:"workspaces"`
	WorkspaceRequirements []tektonDagWorkspaceRequirement `json:"workspaceRequirements"`
	When                  []map[string]any                `json:"when"`
	Retries               *int64                          `json:"retries"`
	Timeout               string                          `json:"timeout"`
	OnError               string                          `json:"onError"`
	Matrix                map[string]any                  `json:"matrix"`
	Results               []string                        `json:"results"`
	Enabled               *bool                           `json:"enabled"`
	Labels                map[string]string               `json:"labels"`
	Annotations           map[string]string               `json:"annotations"`
}

type tektonDagTaskRef struct {
	Kind       string               `json:"kind"`
	Name       string               `json:"name"`
	APIVersion string               `json:"apiVersion"`
	Bundle     string               `json:"bundle"`
	Resolver   string               `json:"resolver"`
	Namespace  string               `json:"namespace"`
	Params     []tektonDagNameValue `json:"params"`
}

type tektonDagPipelineRef struct {
	Name       string               `json:"name"`
	APIVersion string               `json:"apiVersion"`
	Bundle     string               `json:"bundle"`
	Resolver   string               `json:"resolver"`
	Params     []tektonDagNameValue `json:"params"`
}

type tektonDagNameValue struct {
	Name  string `json:"name"`
	Value any    `json:"value"`
}

type tektonDagEdge struct {
	ID              string `json:"id"`
	Source          string `json:"source"`
	Target          string `json:"target"`
	Type            string `json:"type"`
	ResultName      string `json:"resultName"`
	TargetParam     string `json:"targetParam"`
	TargetParamName string `json:"targetParamName"`
	ParamName       string `json:"paramName"`
	Condition       string `json:"condition"`
	WhenInput       string `json:"whenInput"`
	WhenOperator    string `json:"whenOperator"`
	WhenValues      []any  `json:"whenValues"`
}

func hasTektonDagConfig(content string) bool {
	return strings.TrimSpace(content) != ""
}

func validateTektonDagConfig(content string) error {
	cfg, err := parseTektonDagConfig(content)
	if err != nil {
		return err
	}
	return validateTektonDagConfigDoc(cfg)
}

func validateTektonDagConfigDoc(cfg tektonDagConfigDoc) error {
	if len(enabledTektonDagMainNodes(cfg)) == 0 {
		return errorx.Msg("Tekton DAG 至少需要一个启用任务节点")
	}
	if err := validateTektonDagParams(cfg); err != nil {
		return err
	}
	if err := validateTektonDagResults(cfg); err != nil {
		return err
	}
	if err := validateTektonDagWorkspaces(cfg); err != nil {
		return err
	}
	if err := validateTektonDagNodes(cfg); err != nil {
		return err
	}
	if err := validateTektonDagEdges(cfg); err != nil {
		return err
	}
	return validateTektonDagNoCycle(cfg)
}

func validateTektonDagParams(cfg tektonDagConfigDoc) error {
	seen := map[string]struct{}{}
	for _, item := range append(cfg.Pipeline.Params, cfg.Params...) {
		name := strings.TrimSpace(item.Name)
		if name == "" {
			return errorx.Msg("Tekton Pipeline 参数名不能为空")
		}
		if !tektonParamNamePattern.MatchString(name) {
			return errorx.Msg("Tekton Pipeline 参数名不合法：" + name)
		}
		if _, ok := seen[name]; ok {
			return errorx.Msg("Tekton Pipeline 参数重复：" + name)
		}
		seen[name] = struct{}{}
		paramType, ok := tektonOfficialParamType(item.Type)
		if !ok {
			return errorx.Msg("Tekton Pipeline 参数类型不支持：" + name)
		}
		defaultValue := item.Default
		if defaultValue == nil {
			defaultValue = item.DefaultValue
		}
		if defaultValue != nil && (!item.Required || tektonDagHasProvidedValue(defaultValue)) {
			if _, err := tektonParamDefaultValue(paramType, defaultValue, name); err != nil {
				return err
			}
		}
		for _, enumValue := range item.Enum {
			if !tektonDagAnyHasValue(enumValue) {
				return errorx.Msg("Tekton Pipeline 参数 enum 不能为空：" + name)
			}
		}
		if len(item.Properties) > 0 && paramType != "object" {
			return errorx.Msg("Tekton Pipeline 参数 properties 只能用于 object 类型：" + name)
		}
		if paramType == "object" && strings.Contains(name, ".") {
			return errorx.Msg("Tekton Pipeline 参数类型为 object 时名称不能包含点号：" + name)
		}
		if paramType == "object" && len(item.Properties) == 0 {
			return errorx.Msg("Tekton Pipeline 参数类型为 object 时必须配置 properties：" + name)
		}
		for propertyName, property := range item.Properties {
			propertyName = strings.TrimSpace(propertyName)
			if propertyName == "" || !tektonParamNamePattern.MatchString(propertyName) {
				return errorx.Msg("Tekton Pipeline 参数 properties 名称不合法：" + name)
			}
			if _, ok := tektonOfficialParamType(property.Type); !ok {
				return errorx.Msg("Tekton Pipeline 参数 properties 类型不支持：" + name)
			}
		}
	}
	return nil
}

func tektonOfficialParamType(value string) (string, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "string", true
	}
	switch value {
	case "string", "array", "object":
		return value, true
	default:
		return "", false
	}
}

func validateTektonDagResults(cfg tektonDagConfigDoc) error {
	seen := map[string]struct{}{}
	resultByTaskName := map[string]map[string]struct{}{}
	for _, node := range append(enabledTektonDagMainNodes(cfg), enabledTektonDagFinallyNodes(cfg)...) {
		taskName := tektonDagTaskName(node)
		if taskName == "" {
			continue
		}
		results := map[string]struct{}{}
		for _, resultName := range node.Results {
			if strings.TrimSpace(resultName) != "" {
				results[strings.TrimSpace(resultName)] = struct{}{}
			}
		}
		resultByTaskName[taskName] = results
	}
	for _, item := range append(cfg.Pipeline.Results, cfg.Results...) {
		name := strings.TrimSpace(item.Name)
		if name == "" {
			return errorx.Msg("Tekton Pipeline Result 名称不能为空")
		}
		if !tektonParamNamePattern.MatchString(name) {
			return errorx.Msg("Tekton Pipeline Result 名称不合法：" + name)
		}
		if _, ok := seen[name]; ok {
			return errorx.Msg("Tekton Pipeline Result 重复：" + name)
		}
		seen[name] = struct{}{}
		resultType, ok := tektonOfficialParamType(item.Type)
		if !ok {
			return errorx.Msg("Tekton Pipeline Result 类型不支持：" + name)
		}
		if len(item.Properties) > 0 && resultType != "object" {
			return errorx.Msg("Tekton Pipeline Result properties 只能用于 object 类型：" + name)
		}
		if resultType == "object" && len(item.Properties) == 0 {
			return errorx.Msg("Tekton Pipeline Result 类型为 object 时必须配置 properties：" + name)
		}
		for propertyName, property := range item.Properties {
			propertyName = strings.TrimSpace(propertyName)
			if propertyName == "" || !tektonParamNamePattern.MatchString(propertyName) {
				return errorx.Msg("Tekton Pipeline Result properties 名称不合法：" + name)
			}
			if _, ok := tektonOfficialParamType(property.Type); !ok {
				return errorx.Msg("Tekton Pipeline Result properties 类型不支持：" + name)
			}
		}
		value := strings.TrimSpace(fmt.Sprint(item.Value))
		if value == "" || value == "<nil>" {
			return errorx.Msg("Tekton Pipeline Result 必须配置 value：" + name)
		}
		for _, ref := range extractTektonResultRefs(value) {
			results, ok := resultByTaskName[ref.TaskName]
			if !ok {
				return errorx.Msg("Tekton Pipeline Result 引用的 Task 不存在：" + ref.TaskName)
			}
			if _, ok := results[ref.ResultName]; !ok {
				return errorx.Msg("Tekton Pipeline Result 引用的 Task Result 不存在：" + ref.TaskName + "." + ref.ResultName)
			}
		}
	}
	return nil
}

func validateTektonDagWorkspaces(cfg tektonDagConfigDoc) error {
	seen := map[string]struct{}{}
	for _, item := range append(append([]tektonDagWorkspace{}, cfg.Pipeline.Workspaces...), cfg.Workspaces...) {
		name := strings.TrimSpace(item.Name)
		if name == "" {
			return errorx.Msg("Tekton Workspace 名称不能为空")
		}
		if !kubernetesSecretNamePattern.MatchString(name) {
			return errorx.Msg("Tekton Workspace 名称不符合 Kubernetes 命名规范：" + name)
		}
		if _, ok := seen[name]; ok {
			return errorx.Msg("Tekton Workspace 重复：" + name)
		}
		seen[name] = struct{}{}
		bindingType := firstNotBlank(item.BindingType, item.Strategy, item.Type, "emptyDir")
		if !isSupportedTektonWorkspaceBindingType(bindingType) {
			return errorx.Msg("Workspace 绑定类型不支持：" + bindingType)
		}
		allowedBindings := normalizeTektonWorkspaceBindingTypes(item.AllowedBindingTypes)
		for _, allowedBinding := range allowedBindings {
			if !isSupportedTektonWorkspaceBindingType(allowedBinding) {
				return errorx.Msg("Workspace 允许绑定类型不支持：" + allowedBinding)
			}
		}
		if len(allowedBindings) > 0 && !stringInList(allowedBindings, bindingType) {
			return errorx.Msg("Workspace 绑定类型不在模板允许范围内：" + name)
		}
		for _, sourceType := range item.AllowedSourceTypes {
			if !isSupportedTektonWorkspaceSourceType(sourceType) {
				return errorx.Msg("Workspace 平台来源类型不支持：" + sourceType)
			}
			if stringInList(allowedBindings, "generatedSecret") && strings.TrimSpace(sourceType) == "kubernetesResource" {
				return errorx.Msg("Workspace 生成 Secret 只能来自平台凭证或配置中心")
			}
		}
		if item.SourceType != "" && len(item.AllowedSourceTypes) > 0 && !stringInList(item.AllowedSourceTypes, item.SourceType) {
			return errorx.Msg("Workspace 平台来源不在模板允许范围内：" + name)
		}
		if bindingType == "generatedSecret" && strings.TrimSpace(item.SourceType) == "kubernetesResource" {
			return errorx.Msg("Workspace 生成 Secret 只能来自平台凭证或配置中心：" + name)
		}
		if item.PlatformType != "" && len(item.AllowedPlatformTypes) > 0 && !stringInList(item.AllowedPlatformTypes, item.PlatformType) {
			return errorx.Msg("Workspace 平台类型不在模板允许范围内：" + name)
		}
		if bindingType == "volumeClaimTemplate" || bindingType == "dynamicPVC" {
			if _, err := tektonDagVolumeClaimTemplate(item); err != nil {
				return err
			}
		}
		if bindingType == "projected" && len(item.Projected) == 0 {
			if _, ok := item.Config["projected"].(map[string]any); !ok && !item.Optional {
				return errorx.Msg("Projected Workspace 绑定不能为空：" + name)
			}
		}
		if strings.TrimSpace(item.VolumeMode) != "" && item.VolumeMode != "Filesystem" && item.VolumeMode != "Block" {
			return errorx.Msg("动态 PVC volumeMode 不支持：" + name)
		}
	}
	return nil
}

func normalizeTektonWorkspaceBindingTypes(items []string) []string {
	result := make([]string, 0, len(items))
	seen := map[string]struct{}{}
	for _, item := range items {
		value := strings.TrimSpace(item)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func isSupportedTektonWorkspaceBindingType(value string) bool {
	switch strings.TrimSpace(value) {
	case "emptyDir", "persistentVolumeClaim", "pvc", "sharedPVC", "volumeClaimTemplate", "dynamicPVC", "secret", "generatedSecret", "configMap", "projected":
		return true
	default:
		return false
	}
}

func isSupportedTektonWorkspaceSourceType(value string) bool {
	switch strings.TrimSpace(value) {
	case "kubernetesResource", "platformCredential", "configCenter":
		return true
	default:
		return false
	}
}

func stringInList(items []string, target string) bool {
	target = strings.TrimSpace(target)
	for _, item := range items {
		if strings.TrimSpace(item) == target {
			return true
		}
	}
	return false
}

func validateTektonDagNodes(cfg tektonDagConfigDoc) error {
	ids := map[string]struct{}{}
	names := map[string]struct{}{}
	workspaceNames := map[string]struct{}{}
	paramNames := tektonDagPipelineParamNameSet(cfg)
	for _, item := range append(append([]tektonDagWorkspace{}, cfg.Pipeline.Workspaces...), cfg.Workspaces...) {
		if name := strings.TrimSpace(item.Name); name != "" {
			workspaceNames[name] = struct{}{}
		}
	}
	for _, node := range append(enabledTektonDagMainNodes(cfg), enabledTektonDagFinallyNodes(cfg)...) {
		id := strings.TrimSpace(node.ID)
		if id == "" {
			return errorx.Msg("Tekton DAG 节点 ID 不能为空")
		}
		if _, ok := ids[id]; ok {
			return errorx.Msg("Tekton DAG 节点 ID 重复：" + id)
		}
		ids[id] = struct{}{}
		name := tektonDagTaskName(node)
		if _, ok := names[name]; ok {
			return errorx.Msg("Tekton DAG Task 名称重复：" + name)
		}
		names[name] = struct{}{}
		if err := validateTektonDagTaskRefShape(node, paramNames); err != nil {
			return err
		}
		if hasTektonDagTaskRef(node) {
			if _, err := buildTektonDagTaskRef(node); err != nil {
				return err
			}
		} else if hasTektonDagPipelineRef(node.PipelineRef) {
			if _, err := buildTektonDagPipelineRef(node.PipelineRef); err != nil {
				return err
			}
		}
		if err := validateTektonDagNodeExecution(node); err != nil {
			return err
		}
		if err := validateTektonDagNodeWhen(node, paramNames); err != nil {
			return err
		}
		if err := validateTektonDagNodeMatrix(node); err != nil {
			return err
		}
		if err := validateTektonDagNodeParamBindings(node, paramNames); err != nil {
			return err
		}
		if err := validateTektonDagWorkspaceMappings(node, workspaceNames, paramNames); err != nil {
			return err
		}
	}
	return nil
}

func validateTektonDagNodeExecution(node tektonDagNode) error {
	name := tektonDagTaskName(node)
	if node.Retries != nil && *node.Retries < 0 {
		return errorx.Msg("Tekton PipelineTask retries 不能小于 0：" + name)
	}
	if timeout := strings.TrimSpace(node.Timeout); timeout != "" && !tektonDurationPattern.MatchString(timeout) {
		return errorx.Msg("Tekton PipelineTask timeout 格式不合法：" + name)
	}
	if onError := strings.TrimSpace(node.OnError); onError != "" && onError != "continue" && onError != "stopAndFail" {
		return errorx.Msg("Tekton PipelineTask onError 只支持 continue 或 stopAndFail：" + name)
	}
	return nil
}

func tektonDagPipelineParamNameSet(cfg tektonDagConfigDoc) map[string]tektonDagPipelineParam {
	result := map[string]tektonDagPipelineParam{}
	for _, item := range append(cfg.Pipeline.Params, cfg.Params...) {
		name := strings.TrimSpace(item.Name)
		if name != "" {
			result[name] = item
		}
	}
	return result
}

func validateTektonDagTaskRefShape(node tektonDagNode, pipelineParams map[string]tektonDagPipelineParam) error {
	refCount := 0
	if hasTektonDagTaskRef(node) {
		refCount++
	}
	if hasTektonDagPipelineRef(node.PipelineRef) {
		refCount++
	}
	if len(node.TaskSpec) > 0 {
		refCount++
	}
	if len(node.PipelineSpec) > 0 {
		refCount++
	}
	if refCount == 0 {
		return errorx.Msg("Tekton DAG 任务引用不能为空")
	}
	if refCount > 1 {
		return errorx.Msg("Tekton DAG taskRef、pipelineRef、taskSpec、pipelineSpec 只能配置一个")
	}
	if len(node.TaskSpec) > 0 {
		return errorx.Msg("平台默认禁止 Tekton inline taskSpec")
	}
	if len(node.PipelineSpec) > 0 {
		return errorx.Msg("平台默认禁止 Tekton inline pipelineSpec")
	}
	if hasTektonDagPipelineRef(node.PipelineRef) {
		return validateTektonDagPipelineRefShape(node.PipelineRef, pipelineParams)
	}
	ref := normalizedTektonDagTaskRef(node)
	if strings.TrimSpace(ref.Bundle) != "" {
		return errorx.Msg("Tekton taskRef.bundle 已废弃，请使用 resolver")
	}
	resolver := strings.TrimSpace(ref.Resolver)
	isResolverRef := resolver != "" || strings.TrimSpace(ref.Namespace) != "" || len(ref.Params) > 0
	if strings.TrimSpace(ref.Name) == "" && (!isResolverRef || len(ref.Params) == 0) {
		return errorx.Msg("Tekton DAG 任务引用不能为空")
	}
	kind, err := normalizeTektonDagTaskKind(ref.Kind, ref.APIVersion)
	if err != nil {
		return err
	}
	if apiVersion := strings.TrimSpace(ref.APIVersion); apiVersion != "" && !tektonAPIVersionPattern.MatchString(apiVersion) {
		return errorx.Msg("Tekton taskRef.apiVersion 不合法")
	}
	if kind != "Task" && kind != "ClusterTask" && strings.TrimSpace(ref.APIVersion) == "" {
		return errorx.Msg("Tekton Custom Task 必须配置 apiVersion")
	}
	if strings.TrimSpace(ref.Name) != "" && !kubernetesSecretNamePattern.MatchString(ref.Name) {
		return errorx.Msg("Tekton DAG 任务引用名称不合法：" + ref.Name)
	}
	if !isResolverRef {
		return nil
	}
	if resolver == "" {
		resolver = "cluster"
	}
	if !kubernetesSecretNamePattern.MatchString(resolver) {
		return errorx.Msg("Tekton resolver 名称不合法：" + resolver)
	}
	paramMap := map[string]string{}
	for _, item := range ref.Params {
		name := strings.TrimSpace(item.Name)
		value := strings.TrimSpace(fmt.Sprint(item.Value))
		if name == "" || value == "" {
			return errorx.Msg("Tekton resolver params 必须包含 name 和 value")
		}
		if !tektonParamNamePattern.MatchString(name) {
			return errorx.Msg("Tekton resolver params 名称不合法：" + name)
		}
		for _, refName := range extractTektonParamRefs(value) {
			if _, ok := pipelineParams[refName]; !ok {
				return errorx.Msg("Tekton resolver params 引用的 Pipeline 参数未声明：" + refName)
			}
		}
		paramMap[name] = value
	}
	if resolver == "cluster" && len(ref.Params) > 0 {
		resolvedKind := strings.ToLower(strings.TrimSpace(paramMap["kind"]))
		resolvedName := strings.TrimSpace(paramMap["name"])
		if resolvedKind == "" || resolvedName == "" {
			return errorx.Msg("Tekton cluster resolver 必须包含 kind 和 name")
		}
		if resolvedKind != "task" && resolvedKind != "clustertask" {
			return errorx.Msg("Tekton cluster resolver kind 只支持 task 或 clustertask")
		}
		if !kubernetesSecretNamePattern.MatchString(resolvedName) {
			return errorx.Msg("Tekton cluster resolver name 不合法：" + resolvedName)
		}
		if namespace := strings.TrimSpace(paramMap["namespace"]); namespace != "" && !kubernetesSecretNamePattern.MatchString(namespace) {
			return errorx.Msg("Tekton cluster resolver namespace 不合法：" + namespace)
		}
		return nil
	}
	if resolver == "cluster" && kind != "Task" && kind != "ClusterTask" {
		return errorx.Msg("Tekton cluster resolver kind 只支持 Task 或 ClusterTask")
	}
	return nil
}

func hasTektonDagTaskRef(node tektonDagNode) bool {
	ref := node.TaskRef
	kind := strings.TrimSpace(ref.Kind)
	return strings.TrimSpace(ref.Name) != "" ||
		strings.TrimSpace(ref.APIVersion) != "" ||
		strings.TrimSpace(ref.Bundle) != "" ||
		strings.TrimSpace(ref.Resolver) != "" ||
		strings.TrimSpace(ref.Namespace) != "" ||
		len(ref.Params) > 0 ||
		(kind != "" && kind != "Task")
}

func hasTektonDagPipelineRef(ref tektonDagPipelineRef) bool {
	return strings.TrimSpace(ref.Name) != "" ||
		strings.TrimSpace(ref.APIVersion) != "" ||
		strings.TrimSpace(ref.Bundle) != "" ||
		strings.TrimSpace(ref.Resolver) != "" ||
		len(ref.Params) > 0
}

func validateTektonDagPipelineRefShape(ref tektonDagPipelineRef, pipelineParams map[string]tektonDagPipelineParam) error {
	if strings.TrimSpace(ref.Bundle) != "" {
		return errorx.Msg("Tekton pipelineRef.bundle 已废弃，请使用 resolver")
	}
	if apiVersion := strings.TrimSpace(ref.APIVersion); apiVersion != "" && !tektonAPIVersionPattern.MatchString(apiVersion) {
		return errorx.Msg("Tekton pipelineRef.apiVersion 不合法")
	}
	resolver := strings.TrimSpace(ref.Resolver)
	isResolverRef := resolver != "" || len(ref.Params) > 0
	if !isResolverRef {
		name := strings.TrimSpace(ref.Name)
		if name == "" {
			return errorx.Msg("Tekton pipelineRef.name 不能为空")
		}
		if !kubernetesSecretNamePattern.MatchString(name) {
			return errorx.Msg("Tekton pipelineRef.name 不合法：" + name)
		}
		return nil
	}
	if resolver == "" {
		resolver = "cluster"
	}
	if !kubernetesSecretNamePattern.MatchString(resolver) {
		return errorx.Msg("Tekton pipelineRef resolver 名称不合法：" + resolver)
	}
	if len(ref.Params) == 0 {
		return errorx.Msg("Tekton pipelineRef resolver params 不能为空")
	}
	for _, item := range ref.Params {
		name := strings.TrimSpace(item.Name)
		value := strings.TrimSpace(fmt.Sprint(item.Value))
		if name == "" || value == "" {
			return errorx.Msg("Tekton pipelineRef resolver params 必须包含 name 和 value")
		}
		if !tektonParamNamePattern.MatchString(name) {
			return errorx.Msg("Tekton pipelineRef resolver params 名称不合法：" + name)
		}
		for _, refName := range extractTektonParamRefs(value) {
			if _, ok := pipelineParams[refName]; !ok {
				return errorx.Msg("Tekton pipelineRef 参数引用的 Pipeline 参数未声明：" + refName)
			}
		}
	}
	return nil
}

func normalizedTektonDagTaskRef(node tektonDagNode) tektonDagTaskRef {
	ref := node.TaskRef
	ref.Name = firstNotBlank(ref.Name, node.TaskName, node.StepCode)
	ref.Kind = firstNotBlank(ref.Kind, "Task")
	return ref
}

func normalizeTektonDagTaskKind(kind, apiVersion string) (string, error) {
	raw := strings.TrimSpace(firstNotBlank(kind, "Task"))
	switch strings.ToLower(raw) {
	case "task":
		return "Task", nil
	case "clustertask":
		return "ClusterTask", nil
	default:
		if strings.TrimSpace(apiVersion) != "" && tektonTaskKindPattern.MatchString(raw) {
			return raw, nil
		}
		return "", errorx.Msg("Tekton taskRef kind 只支持 Task、ClusterTask 或带 apiVersion 的 Custom Task")
	}
}

func validateTektonDagNodeWhen(node tektonDagNode, pipelineParams map[string]tektonDagPipelineParam) error {
	for _, item := range node.When {
		cel := strings.TrimSpace(fmt.Sprint(item["cel"]))
		if cel != "" && cel != "<nil>" {
			for _, ref := range extractTektonParamRefs(cel) {
				if _, ok := pipelineParams[ref]; !ok {
					return errorx.Msg("Tekton DAG When CEL 引用的 Pipeline 参数未声明：" + ref)
				}
			}
			continue
		}
		input := strings.TrimSpace(fmt.Sprint(item["input"]))
		operator := strings.TrimSpace(fmt.Sprint(item["operator"]))
		if operator == "" {
			operator = "in"
		}
		values := anySlice(item["values"])
		if input == "" || len(values) == 0 {
			return errorx.Msg("Tekton DAG When 条件必须配置 input 和 values：" + tektonDagStageName(node))
		}
		if !tektonDagAllNonEmptyStrings(values) {
			return errorx.Msg("Tekton DAG When values 必须是非空字符串：" + tektonDagStageName(node))
		}
		if operator != "in" && operator != "notin" {
			return errorx.Msg("Tekton DAG When operator 只支持 in/notin：" + tektonDagStageName(node))
		}
		for _, ref := range extractTektonParamRefs(input) {
			if _, ok := pipelineParams[ref]; !ok {
				return errorx.Msg("Tekton DAG When 引用的 Pipeline 参数未声明：" + ref)
			}
		}
	}
	return nil
}

func validateTektonDagNodeMatrix(node tektonDagNode) error {
	if len(node.Matrix) == 0 {
		return nil
	}
	if _, err := tektonDagMatrix(node.Matrix, tektonDagStageName(node)); err != nil {
		return err
	}
	return nil
}

func validateTektonDagNodeParamBindings(node tektonDagNode, pipelineParams map[string]tektonDagPipelineParam) error {
	taskParams := tektonDagTaskParamsFromRawPreserveSource(node.Params)
	taskParamByName := map[string]map[string]any{}
	matrixParamNames := map[string]struct{}{}
	for _, name := range tektonDagMatrixParamNames(node.Matrix) {
		matrixParamNames[name] = struct{}{}
	}
	for _, item := range taskParams {
		name := firstNotBlank(tektonDagString(item["name"]), tektonDagString(item["code"]))
		if name == "" {
			continue
		}
		taskParamByName[name] = item
		for _, ref := range tektonDagParamRefsFromValue(item) {
			if _, ok := pipelineParams[ref]; !ok {
				return errorx.Msg("Tekton Pipeline 参数未声明：" + ref)
			}
		}
	}
	for _, requirement := range node.ParamRequirements {
		name := strings.TrimSpace(requirement.Name)
		if name == "" {
			continue
		}
		item, ok := taskParamByName[name]
		if !ok {
			if _, matrixBound := matrixParamNames[name]; matrixBound {
				continue
			}
			if requirement.Required && !tektonDagRequirementHasDefault(requirement) {
				return errorx.Msg("节点「" + tektonDagStageName(node) + "」缺少必填 Task 参数映射：" + tektonDagRequirementDisplayName(requirement))
			}
			continue
		}
		if err := validateTektonDagRequiredTaskParamValue(node, requirement, item, pipelineParams); err != nil {
			return err
		}
	}
	return nil
}

func validateTektonDagRequiredTaskParamValue(node tektonDagNode, requirement tektonDagParamRequirement, item map[string]any, pipelineParams map[string]tektonDagPipelineParam) error {
	if !requirement.Required {
		return nil
	}
	if tektonDagTaskParamHasConcreteValue(item) || tektonDagRequirementHasDefault(requirement) {
		return nil
	}
	source := strings.TrimSpace(fmt.Sprint(item["source"]))
	if source == "pipelineParam" || source == "runtime" || source == "param" {
		paramName := firstNotBlank(
			tektonDagString(item["pipelineParam"]),
			tektonDagString(item["paramName"]),
			tektonDagString(item["sourceParam"]),
			tektonDagString(item["name"]),
		)
		pipelineParam, ok := pipelineParams[paramName]
		if !ok {
			return errorx.Msg("Tekton Pipeline 参数未声明：" + paramName)
		}
		if tektonDagPipelineParamHasValue(pipelineParam) || pipelineParam.RuntimeConfig {
			return nil
		}
		return errorx.Msg("Pipeline 参数「" + paramName + "」被必填 Task 参数「" + tektonDagRequirementDisplayName(requirement) + "」引用，必须配置默认值或设为运行时输入")
	}
	return errorx.Msg("节点「" + tektonDagStageName(node) + "」必填参数「" + tektonDagRequirementDisplayName(requirement) + "」未配置")
}

func tektonDagTaskParamsFromRawPreserveSource(raw json.RawMessage) []map[string]any {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var list []map[string]any
	if err := json.Unmarshal(raw, &list); err == nil {
		return list
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil
	}
	result := make([]map[string]any, 0, len(obj))
	keys := sortedMapKeys(obj)
	for _, key := range keys {
		if data, ok := obj[key].(map[string]any); ok {
			data["name"] = key
			result = append(result, data)
			continue
		}
		result = append(result, map[string]any{"name": key, "value": obj[key]})
	}
	return result
}

func tektonDagParamRefsFromValue(item map[string]any) []string {
	result := make([]string, 0)
	for _, key := range []string{"value", "default", "input"} {
		result = append(result, extractTektonParamRefs(fmt.Sprint(item[key]))...)
	}
	source := strings.TrimSpace(fmt.Sprint(item["source"]))
	if source == "pipelineParam" || source == "runtime" || source == "param" {
		paramName := firstNotBlank(
			tektonDagString(item["pipelineParam"]),
			tektonDagString(item["paramName"]),
			tektonDagString(item["sourceParam"]),
		)
		if paramName != "" {
			result = append(result, paramName)
		}
	}
	return uniqueStrings(result)
}

func extractTektonParamRefs(value string) []string {
	result := make([]string, 0)
	const prefix = "$(params."
	for {
		index := strings.Index(value, prefix)
		if index < 0 {
			return result
		}
		rest := value[index+len(prefix):]
		end := strings.Index(rest, ")")
		if end < 0 {
			return result
		}
		name := strings.TrimSpace(rest[:end])
		if name != "" {
			result = append(result, name)
		}
		value = rest[end+1:]
	}
}

type tektonResultRef struct {
	TaskName   string
	ResultName string
}

func extractTektonResultRefs(value string) []tektonResultRef {
	result := make([]tektonResultRef, 0)
	for _, prefix := range []string{"$(tasks.", "$(finally."} {
		rest := value
		for {
			index := strings.Index(rest, prefix)
			if index < 0 {
				break
			}
			part := rest[index+len(prefix):]
			taskEnd := strings.Index(part, ".results.")
			if taskEnd < 0 {
				rest = part
				continue
			}
			taskName := strings.TrimSpace(part[:taskEnd])
			resultPart := part[taskEnd+len(".results."):]
			resultEnd := strings.Index(resultPart, ")")
			if resultEnd < 0 {
				break
			}
			resultName := strings.TrimSpace(resultPart[:resultEnd])
			if taskName != "" && resultName != "" {
				result = append(result, tektonResultRef{TaskName: taskName, ResultName: resultName})
			}
			rest = resultPart[resultEnd+1:]
		}
	}
	return result
}

func uniqueStrings(items []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
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

func tektonDagTaskParamHasConcreteValue(item map[string]any) bool {
	if value, ok := item["value"]; ok && tektonDagHasProvidedValue(value) {
		return true
	}
	if value, ok := item["default"]; ok && tektonDagHasProvidedValue(value) {
		return true
	}
	return false
}

func tektonDagRequirementHasDefault(item tektonDagParamRequirement) bool {
	return tektonDagHasProvidedValue(item.DefaultValue)
}

func tektonDagPipelineParamHasValue(item tektonDagPipelineParam) bool {
	return tektonDagHasProvidedValue(item.CurrentValue) || tektonDagHasProvidedValue(item.DefaultValue) || tektonDagHasProvidedValue(item.Default)
}

func tektonDagHasProvidedValue(value any) bool {
	switch data := value.(type) {
	case nil:
		return false
	case string:
		return strings.TrimSpace(data) != ""
	default:
		return true
	}
}

func tektonDagAnyHasValue(value any) bool {
	switch data := value.(type) {
	case nil:
		return false
	case string:
		return strings.TrimSpace(data) != ""
	case []any:
		return len(data) > 0
	case map[string]any:
		return len(data) > 0
	default:
		return true
	}
}

func tektonDagRequirementDisplayName(item tektonDagParamRequirement) string {
	if name := strings.TrimSpace(item.DisplayName); name != "" {
		return name
	}
	return strings.TrimSpace(item.Name)
}

func validateTektonDagWorkspaceMappings(node tektonDagNode, workspaceNames map[string]struct{}, pipelineParams map[string]tektonDagPipelineParam) error {
	if len(workspaceNames) == 0 {
		return nil
	}
	seenTaskWorkspaces := map[string]struct{}{}
	for _, item := range tektonDagTaskWorkspaces(node.Workspaces) {
		taskWorkspace := strings.TrimSpace(fmt.Sprint(item["name"]))
		if taskWorkspace == "" {
			return errorx.Msg("Tekton DAG Workspace 映射不完整：" + tektonDagStageName(node))
		}
		if _, ok := seenTaskWorkspaces[taskWorkspace]; ok {
			return errorx.Msg("Tekton DAG Workspace 重复映射：" + taskWorkspace)
		}
		seenTaskWorkspaces[taskWorkspace] = struct{}{}
		workspace := strings.TrimSpace(fmt.Sprint(item["workspace"]))
		if workspace == "" {
			return errorx.Msg("Tekton DAG Workspace 映射不完整：" + tektonDagStageName(node))
		}
		if _, ok := workspaceNames[workspace]; !ok {
			return errorx.Msg("Tekton DAG Workspace 未声明：" + workspace)
		}
		for _, ref := range extractTektonParamRefs(strings.TrimSpace(fmt.Sprint(item["subPath"]))) {
			if _, ok := pipelineParams[ref]; !ok {
				return errorx.Msg("Tekton DAG Workspace subPath 引用的 Pipeline 参数未声明：" + ref)
			}
		}
	}
	return nil
}

func validateTektonDagEdges(cfg tektonDagConfigDoc) error {
	nodeIDs := map[string]struct{}{}
	mainNodeIDs := map[string]struct{}{}
	pipelineParams := tektonDagPipelineParamNameSet(cfg)
	for _, node := range append(enabledTektonDagMainNodes(cfg), enabledTektonDagFinallyNodes(cfg)...) {
		nodeIDs[tektonDagNodeID(node)] = struct{}{}
	}
	for _, node := range enabledTektonDagMainNodes(cfg) {
		mainNodeIDs[tektonDagNodeID(node)] = struct{}{}
	}
	for _, edge := range cfg.Edges {
		edgeType := strings.TrimSpace(edge.Type)
		if edgeType == "" {
			edgeType = "runAfter"
		}
		if edgeType != "runAfter" && edgeType != "result" && edgeType != "when" && edgeType != "finally" {
			return errorx.Msg("Tekton DAG 连线类型不支持：" + edgeType)
		}
		sourceID := strings.TrimSpace(edge.Source)
		targetID := strings.TrimSpace(edge.Target)
		if sourceID == "" || targetID == "" {
			return errorx.Msg("Tekton DAG 连线 source/target 不能为空")
		}
		if sourceID == targetID {
			return errorx.Msg("Tekton DAG 连线不能指向自身")
		}
		if _, ok := nodeIDs[sourceID]; !ok {
			return errorx.Msg("Tekton DAG 连线 source 不存在：" + sourceID)
		}
		if _, ok := nodeIDs[targetID]; !ok {
			return errorx.Msg("Tekton DAG 连线 target 不存在：" + targetID)
		}
		if edgeType == "finally" {
			continue
		}
		if _, ok := mainNodeIDs[sourceID]; !ok {
			return errorx.Msg("普通 Task 不能依赖 Finally Task")
		}
		if edgeType == "result" && (strings.TrimSpace(edge.ResultName) == "" || strings.TrimSpace(firstNotBlank(edge.TargetParam, edge.TargetParamName, edge.ParamName)) == "") {
			return errorx.Msg("Tekton DAG Result 连线必须配置 Result 和目标参数")
		}
		if edgeType == "when" {
			operator := firstNotBlank(edge.WhenOperator, "in")
			if operator != "in" && operator != "notin" {
				return errorx.Msg("Tekton DAG When operator 只支持 in/notin")
			}
			if strings.TrimSpace(firstNotBlank(edge.WhenInput, edge.Condition)) == "" || len(edge.WhenValues) == 0 {
				return errorx.Msg("Tekton DAG When 连线必须配置 input 和 values")
			}
			if !tektonDagAllNonEmptyStrings(edge.WhenValues) {
				return errorx.Msg("Tekton DAG When 连线 values 必须是非空字符串")
			}
			for _, ref := range extractTektonParamRefs(firstNotBlank(edge.WhenInput, edge.Condition)) {
				if _, ok := pipelineParams[ref]; !ok {
					return errorx.Msg("Tekton DAG When 连线引用的 Pipeline 参数未声明：" + ref)
				}
			}
		}
	}
	return nil
}

func validateTektonDagNoCycle(cfg tektonDagConfigDoc) error {
	graph := map[string][]string{}
	for _, node := range enabledTektonDagMainNodes(cfg) {
		graph[tektonDagNodeID(node)] = []string{}
	}
	for _, edge := range cfg.Edges {
		edgeType := strings.TrimSpace(edge.Type)
		if edgeType != "" && edgeType != "runAfter" && edgeType != "result" && edgeType != "when" {
			continue
		}
		sourceID := strings.TrimSpace(edge.Source)
		targetID := strings.TrimSpace(edge.Target)
		if _, ok := graph[sourceID]; !ok {
			continue
		}
		if _, ok := graph[targetID]; !ok {
			continue
		}
		graph[sourceID] = append(graph[sourceID], targetID)
	}
	visiting := map[string]bool{}
	visited := map[string]bool{}
	var walk func(string) bool
	walk = func(nodeID string) bool {
		if visiting[nodeID] {
			return true
		}
		if visited[nodeID] {
			return false
		}
		visiting[nodeID] = true
		for _, next := range graph[nodeID] {
			if walk(next) {
				return true
			}
		}
		visiting[nodeID] = false
		visited[nodeID] = true
		return false
	}
	for nodeID := range graph {
		if walk(nodeID) {
			return errorx.Msg("Tekton DAG 存在循环依赖")
		}
	}
	return nil
}

func parseTektonDagConfig(content string) (tektonDagConfigDoc, error) {
	var cfg tektonDagConfigDoc
	if err := json.Unmarshal([]byte(strings.TrimSpace(content)), &cfg); err != nil {
		return cfg, errorx.Msg("Tekton DAG 配置必须是 JSON 对象")
	}
	return cfg, nil
}

func applyTektonWorkspaceBindingsToDag(content, bindings string) (string, error) {
	if strings.TrimSpace(bindings) == "" {
		return content, nil
	}
	cfg, err := parseTektonDagConfig(content)
	if err != nil {
		return "", err
	}
	var items []tektonDagWorkspace
	if err := json.Unmarshal([]byte(strings.TrimSpace(bindings)), &items); err != nil {
		return "", errorx.Msg("Tekton Workspace 绑定必须是 JSON 数组")
	}
	bindingByName := map[string]tektonDagWorkspace{}
	for _, item := range items {
		name := strings.TrimSpace(item.Name)
		if name == "" {
			continue
		}
		bindingByName[name] = item
	}
	apply := func(workspaces []tektonDagWorkspace) ([]tektonDagWorkspace, error) {
		for index := range workspaces {
			binding, ok := bindingByName[strings.TrimSpace(workspaces[index].Name)]
			if !ok {
				continue
			}
			base := workspaces[index]
			if err := validateTektonWorkspaceBindingAgainstTemplate(base, binding); err != nil {
				return nil, err
			}
			binding.Name = workspaces[index].Name
			binding.Description = firstNotBlank(binding.Description, workspaces[index].Description)
			binding.Optional = workspaces[index].Optional
			binding.AllowedBindingTypes = workspaces[index].AllowedBindingTypes
			binding.AllowedSourceTypes = workspaces[index].AllowedSourceTypes
			binding.AllowedPlatformTypes = workspaces[index].AllowedPlatformTypes
			if !workspaces[index].RuntimeConfig {
				binding.RuntimeConfig = false
			}
			workspaces[index] = binding
		}
		return workspaces, nil
	}
	cfg.Workspaces, err = apply(cfg.Workspaces)
	if err != nil {
		return "", err
	}
	cfg.Pipeline.Workspaces, err = apply(cfg.Pipeline.Workspaces)
	if err != nil {
		return "", err
	}
	out, err := json.Marshal(cfg)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func validateTektonDagPipelineParamBindings(content string, params []model.PipelineParam) error {
	cfg, err := parseTektonDagConfig(content)
	if err != nil {
		return err
	}
	paramByCode := make(map[string]model.PipelineParam, len(params))
	for _, item := range params {
		code := strings.TrimSpace(item.Code)
		if code == "" {
			continue
		}
		paramByCode[code] = item
	}
	for _, item := range tektonDagDeclaredPipelineParams(cfg) {
		name := strings.TrimSpace(item.Name)
		if name == "" {
			continue
		}
		param, ok := paramByCode[name]
		if !ok {
			return errorx.Msg("Tekton Pipeline 参数未配置：" + name)
		}
		expectedType := tektonParamType(firstNotBlank(item.Type, "string"))
		actualType := tektonParamType(firstNotBlank(param.ParamType, "string"))
		if expectedType != actualType {
			return errorx.Msg("Tekton Pipeline 参数类型不一致：" + name)
		}
	}
	return nil
}

func validateTektonPipelineRunCompleteness(pipeline *model.DevopsPipeline, params, workspaces map[string]string) error {
	if pipeline == nil || !hasTektonDagConfig(pipeline.TektonDagConfig) {
		return nil
	}
	if err := validateTektonDagPipelineParamBindings(pipeline.TektonDagConfig, pipeline.Params); err != nil {
		return err
	}
	cfg, err := parseTektonDagConfig(pipeline.TektonDagConfig)
	if err != nil {
		return err
	}
	runtimeWorkspaceNames := map[string]struct{}{}
	items := append([]tektonDagWorkspace{}, cfg.Pipeline.Workspaces...)
	items = append(items, cfg.Workspaces...)
	policy, err := tektonRunPolicyFromPipeline(pipeline)
	if err != nil {
		return err
	}
	items = append(items, policy.Workspaces...)
	for _, item := range items {
		name := strings.TrimSpace(item.Name)
		if name == "" || !item.RuntimeConfig {
			continue
		}
		runtimeWorkspaceNames[name] = struct{}{}
	}
	for name := range workspaces {
		if _, ok := runtimeWorkspaceNames[strings.TrimSpace(name)]; !ok {
			return errorx.Msg("Workspace 不允许运行时覆盖：" + name)
		}
	}
	_, err = tektonDagPipelineRunWorkspaces(pipeline, mergeRuntimeWorkspaceValues(params, workspaces))
	return err
}

func tektonPipelineRunParams(pipeline *model.DevopsPipeline, params map[string]string) ([]map[string]any, error) {
	if pipeline == nil {
		return nil, nil
	}
	if hasTektonDagConfig(pipeline.TektonDagConfig) {
		cfg, err := parseTektonDagConfig(pipeline.TektonDagConfig)
		if err != nil {
			return nil, err
		}
		paramByCode := make(map[string]model.PipelineParam, len(pipeline.Params))
		for _, item := range pipeline.Params {
			code := strings.TrimSpace(item.Code)
			if code != "" {
				paramByCode[code] = item
			}
		}
		result := make([]map[string]any, 0, len(tektonDagDeclaredPipelineParams(cfg)))
		for _, item := range tektonDagDeclaredPipelineParams(cfg) {
			name := strings.TrimSpace(item.Name)
			if name == "" {
				continue
			}
			param, ok := paramByCode[name]
			if !ok {
				return nil, errorx.Msg("Tekton Pipeline 参数未配置：" + name)
			}
			value, err := tektonParamDefaultValue(tektonParamType(firstNotBlank(item.Type, "string")), tektonPipelineParamValue(param, params), name)
			if err != nil {
				return nil, err
			}
			result = append(result, map[string]any{
				"name":  name,
				"value": value,
			})
		}
		return result, nil
	}
	runParams := make([]map[string]any, 0, len(pipeline.Params))
	for _, item := range pipeline.Params {
		code := strings.TrimSpace(item.Code)
		if code == "" {
			continue
		}
		value, err := tektonParamDefaultValue(tektonParamType(item.ParamType), tektonPipelineParamValue(item, params), code)
		if err != nil {
			return nil, err
		}
		runParams = append(runParams, map[string]any{
			"name":  code,
			"value": value,
		})
	}
	return runParams, nil
}

func validateTektonWorkspaceBindingAgainstTemplate(template, binding tektonDagWorkspace) error {
	name := strings.TrimSpace(template.Name)
	bindingType := firstNotBlank(binding.BindingType, binding.Strategy, binding.Type, "emptyDir")
	allowedBindings := normalizeTektonWorkspaceBindingTypes(template.AllowedBindingTypes)
	if len(allowedBindings) > 0 && !stringInList(allowedBindings, bindingType) {
		return errorx.Msg("Workspace 绑定类型不在模板允许范围内：" + name)
	}
	if binding.RuntimeConfig && !template.RuntimeConfig {
		return errorx.Msg("Workspace 不允许运行时覆盖：" + name)
	}
	sourceType := strings.TrimSpace(binding.SourceType)
	if sourceType != "" && len(template.AllowedSourceTypes) > 0 && !stringInList(template.AllowedSourceTypes, sourceType) {
		return errorx.Msg("Workspace 平台来源不在模板允许范围内：" + name)
	}
	platformType := strings.TrimSpace(binding.PlatformType)
	if platformType != "" && len(template.AllowedPlatformTypes) > 0 && !stringInList(template.AllowedPlatformTypes, platformType) {
		return errorx.Msg("Workspace 平台类型不在模板允许范围内：" + name)
	}
	return nil
}

func tektonRunPolicyFromPipeline(pipeline *model.DevopsPipeline) (tektonDagRunPolicy, error) {
	var policy tektonDagRunPolicy
	if pipeline == nil {
		return policy, nil
	}
	if hasTektonDagConfig(pipeline.TektonDagConfig) {
		cfg, err := parseTektonDagConfig(pipeline.TektonDagConfig)
		if err != nil {
			return policy, err
		}
		policy = cfg.RunPolicy
	}
	if strings.TrimSpace(pipeline.TektonRunPolicy) != "" {
		var override tektonDagRunPolicy
		var raw map[string]json.RawMessage
		if err := json.Unmarshal([]byte(strings.TrimSpace(pipeline.TektonRunPolicy)), &override); err != nil {
			return policy, errorx.Msg("Tekton 运行策略必须是结构化 JSON")
		}
		_ = json.Unmarshal([]byte(strings.TrimSpace(pipeline.TektonRunPolicy)), &raw)
		policy = mergeTektonRunPolicy(policy, override)
		if _, ok := raw["cleanBeforeRun"]; ok {
			policy.CleanBeforeRun = override.CleanBeforeRun
		}
		if _, ok := raw["cleanAfterRun"]; ok {
			policy.CleanAfterRun = override.CleanAfterRun
		}
		if _, ok := raw["managedBy"]; ok {
			policy.ManagedBy = override.ManagedBy
		}
		if _, ok := raw["labels"]; ok {
			policy.Labels = override.Labels
		}
		if _, ok := raw["annotations"]; ok {
			policy.Annotations = override.Annotations
		}
		if _, ok := raw["podTemplateEnv"]; ok {
			policy.PodTemplateEnv = override.PodTemplateEnv
		}
		if _, ok := raw["podTemplateRaw"]; ok {
			policy.PodTemplateRaw = override.PodTemplateRaw
		}
		if _, ok := raw["taskRunTemplateRaw"]; ok {
			policy.TaskRunTemplateRaw = override.TaskRunTemplateRaw
		}
		if _, ok := raw["taskRunSpecs"]; ok {
			policy.TaskRunSpecs = override.TaskRunSpecs
		}
		if _, ok := raw["taskRunSpecsRaw"]; ok {
			policy.TaskRunSpecsRaw = override.TaskRunSpecsRaw
		}
	}
	policy.ConcurrencyPolicy = normalizeTektonRunConcurrency(policy.ConcurrencyPolicy)
	return policy, nil
}

func mergeTektonRunPolicy(base, override tektonDagRunPolicy) tektonDagRunPolicy {
	if strings.TrimSpace(override.ServiceAccountName) != "" {
		base.ServiceAccountName = override.ServiceAccountName
	}
	if strings.TrimSpace(override.ManagedBy) != "" {
		base.ManagedBy = override.ManagedBy
	}
	if strings.TrimSpace(override.Timeout) != "" {
		base.Timeout = override.Timeout
	}
	if strings.TrimSpace(override.PipelineTimeout) != "" {
		base.PipelineTimeout = override.PipelineTimeout
	}
	if strings.TrimSpace(override.TasksTimeout) != "" {
		base.TasksTimeout = override.TasksTimeout
	}
	if strings.TrimSpace(override.FinallyTimeout) != "" {
		base.FinallyTimeout = override.FinallyTimeout
	}
	if len(override.Timeouts) > 0 {
		if base.Timeouts == nil {
			base.Timeouts = map[string]string{}
		}
		for key, value := range override.Timeouts {
			base.Timeouts[key] = value
		}
	}
	if len(override.Workspaces) > 0 {
		base.Workspaces = override.Workspaces
	}
	if len(override.Labels) > 0 {
		base.Labels = override.Labels
	}
	if len(override.Annotations) > 0 {
		base.Annotations = override.Annotations
	}
	if len(override.PodTemplateEnv) > 0 {
		base.PodTemplateEnv = override.PodTemplateEnv
	}
	if strings.TrimSpace(override.PodTemplateRaw) != "" {
		base.PodTemplateRaw = override.PodTemplateRaw
	}
	if strings.TrimSpace(override.TaskRunTemplateRaw) != "" {
		base.TaskRunTemplateRaw = override.TaskRunTemplateRaw
	}
	if len(override.TaskRunSpecs) > 0 {
		base.TaskRunSpecs = override.TaskRunSpecs
	}
	if strings.TrimSpace(override.TaskRunSpecsRaw) != "" {
		base.TaskRunSpecsRaw = override.TaskRunSpecsRaw
	}
	if strings.TrimSpace(override.CleanupImage) != "" {
		base.CleanupImage = override.CleanupImage
	}
	if strings.TrimSpace(override.ConcurrencyPolicy) != "" {
		base.ConcurrencyPolicy = override.ConcurrencyPolicy
	}
	if strings.TrimSpace(override.WorkspaceStrategy) != "" {
		base.WorkspaceStrategy = override.WorkspaceStrategy
	}
	return base
}

func renderTektonDagPipelineYAML(name string, binding tektonBindingConfig, pipeline *model.DevopsPipeline, runtimeParams map[string]string) (string, error) {
	cfg, err := parseTektonDagConfig(pipeline.TektonDagConfig)
	if err != nil {
		return "", err
	}
	tasks, finallyTasks, err := tektonDagTasks(cfg)
	if err != nil {
		return "", err
	}
	spec := map[string]any{"tasks": tasks}
	if displayName := strings.TrimSpace(cfg.Pipeline.DisplayName); displayName != "" {
		spec["displayName"] = displayName
	}
	if description := strings.TrimSpace(cfg.Pipeline.Description); description != "" {
		spec["description"] = description
	}
	if len(finallyTasks) > 0 {
		spec["finally"] = finallyTasks
	}
	params, err := tektonDagPipelineParams(cfg, pipeline.Params)
	if err != nil {
		return "", err
	}
	if len(params) > 0 {
		spec["params"] = params
	}
	if results := tektonDagPipelineResults(cfg); len(results) > 0 {
		spec["results"] = results
	}
	workspaces := tektonDagPipelineWorkspaces(cfg, tasks, finallyTasks)
	if len(workspaces) > 0 {
		spec["workspaces"] = workspaces
	}
	metadata := map[string]any{
		"name":      name,
		"namespace": binding.Namespace,
		"labels": map[string]any{
			"kube-nova.io/pipeline-id": pipeline.ID.Hex(),
			"kube-nova.io/engine":      engineTekton,
		},
	}
	mergeStringMap(metadata["labels"].(map[string]any), cfg.Labels)
	mergeStringMap(metadata["labels"].(map[string]any), cfg.Pipeline.Labels)
	annotations := map[string]any{}
	mergeStringMap(annotations, cfg.Annotations)
	mergeStringMap(annotations, cfg.Pipeline.Annotations)
	if len(annotations) > 0 {
		metadata["annotations"] = annotations
	}
	obj := map[string]any{
		"apiVersion": "tekton.dev/v1",
		"kind":       "Pipeline",
		"metadata":   metadata,
		"spec":       spec,
	}
	out, err := yaml.Marshal(obj)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func tektonDagTasks(cfg tektonDagConfigDoc) ([]map[string]any, []map[string]any, error) {
	nameByID := tektonDagNodeNameMap(cfg)
	runAfter := tektonDagRunAfterMap(cfg.Edges, nameByID)
	mainNodes := enabledTektonDagMainNodes(cfg)
	finallyNodes := enabledTektonDagFinallyNodes(cfg)
	tasks := make([]map[string]any, 0, len(mainNodes))
	for _, node := range mainNodes {
		task, err := tektonDagTask(node, runAfter[node.ID], cfg.Edges, nameByID)
		if err != nil {
			return nil, nil, err
		}
		tasks = append(tasks, task)
	}
	finallyTasks := make([]map[string]any, 0, len(finallyNodes))
	for _, node := range finallyNodes {
		task, err := tektonDagTask(node, nil, cfg.Edges, nameByID)
		if err != nil {
			return nil, nil, err
		}
		delete(task, "runAfter")
		finallyTasks = append(finallyTasks, task)
	}
	cleanupWorkspaces := tektonDagCleanupWorkspaceMappings(cfg, tasks, finallyTasks)
	if cfg.RunPolicy.CleanBeforeRun && len(cleanupWorkspaces) > 0 {
		cleanupName := uniqueTektonDagGeneratedTaskName("kube-nova-clean-before-run", tasks, finallyTasks)
		tasks = prependTektonDagCleanupTask(tasks, cleanupName, cleanupWorkspaces, cfg.RunPolicy.CleanupImage)
		addTektonDagRunAfter(tasks[1:], cleanupName)
	}
	if cfg.RunPolicy.CleanAfterRun && len(cleanupWorkspaces) > 0 {
		cleanupName := uniqueTektonDagGeneratedTaskName("kube-nova-clean-after-run", tasks, finallyTasks)
		finallyTasks = append(finallyTasks, tektonDagCleanupTask(cleanupName, cleanupWorkspaces, cfg.RunPolicy.CleanupImage))
	}
	return tasks, finallyTasks, nil
}

func tektonDagTask(node tektonDagNode, runAfter []string, edges []tektonDagEdge, nameByID map[string]string) (map[string]any, error) {
	name := tektonDagTaskName(node)
	if name == "" {
		return nil, errorx.Msg("Tekton DAG 任务名称不能为空")
	}
	task := map[string]any{
		"name": name,
	}
	if len(node.PipelineSpec) > 0 {
		return nil, errorx.Msg("平台默认禁止 Tekton inline pipelineSpec")
	}
	if hasTektonDagPipelineRef(node.PipelineRef) {
		pipelineRef, err := buildTektonDagPipelineRef(node.PipelineRef)
		if err != nil {
			return nil, err
		}
		task["pipelineRef"] = pipelineRef
	} else {
		if len(node.TaskSpec) > 0 {
			return nil, errorx.Msg("平台默认禁止 Tekton inline taskSpec")
		}
		taskRef, err := buildTektonDagTaskRef(node)
		if err != nil {
			return nil, err
		}
		task["taskRef"] = taskRef
	}
	if displayName := strings.TrimSpace(node.DisplayName); displayName != "" {
		task["displayName"] = displayName
	}
	if description := strings.TrimSpace(node.Description); description != "" {
		task["description"] = description
	}
	if len(runAfter) > 0 {
		task["runAfter"] = uniqueSortedStrings(runAfter)
	}
	params, err := tektonDagTaskParams(node.Params, edges, node.ID, nameByID, node.ParamRequirements, name)
	if err != nil {
		return nil, err
	}
	if len(params) > 0 {
		task["params"] = params
	}
	workspaces := tektonDagTaskWorkspaces(node.Workspaces)
	if len(workspaces) > 0 {
		task["workspaces"] = workspaces
	}
	when := append([]map[string]any{}, node.When...)
	when = append(when, tektonDagWhenFromEdges(edges, node.ID)...)
	if len(when) > 0 {
		task["when"] = when
	}
	if node.Retries != nil && *node.Retries >= 0 {
		task["retries"] = *node.Retries
	}
	if timeout := strings.TrimSpace(node.Timeout); timeout != "" {
		task["timeout"] = timeout
	}
	if onError := strings.TrimSpace(node.OnError); onError != "" {
		task["onError"] = onError
	}
	if len(node.Matrix) > 0 {
		matrix, err := tektonDagMatrix(node.Matrix, name)
		if err != nil {
			return nil, err
		}
		task["matrix"] = matrix
	}
	return task, nil
}

func tektonDagMatrix(raw map[string]any, taskName string) (map[string]any, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	matrix := map[string]any{}
	params, err := tektonDagMatrixParams(raw["params"], taskName)
	if err != nil {
		return nil, err
	}
	if len(params) == 0 {
		return nil, errorx.Msg("Tekton DAG Matrix 必须配置 params：" + taskName)
	}
	matrix["params"] = params
	if includeRaw, ok := raw["include"]; ok {
		include, err := tektonDagMatrixInclude(includeRaw, taskName)
		if err != nil {
			return nil, err
		}
		if len(include) > 0 {
			matrix["include"] = include
		}
	}
	return matrix, nil
}

func tektonDagMatrixParams(raw any, taskName string) ([]map[string]any, error) {
	items := anySlice(raw)
	if len(items) == 0 {
		return nil, nil
	}
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		param, ok := item.(map[string]any)
		if !ok {
			return nil, errorx.Msg("Tekton DAG Matrix params 格式不正确：" + taskName)
		}
		name := strings.TrimSpace(fmt.Sprint(param["name"]))
		values := anySlice(firstNonNil(param["value"], param["values"]))
		if name == "" || len(values) == 0 {
			return nil, errorx.Msg("Tekton DAG Matrix 参数必须配置 name 和 value：" + taskName)
		}
		if !tektonDagAllNonEmptyStrings(values) {
			return nil, errorx.Msg("Tekton DAG Matrix value 必须是非空字符串数组：" + taskName)
		}
		result = append(result, map[string]any{"name": name, "value": values})
	}
	return result, nil
}

func tektonDagMatrixInclude(raw any, taskName string) ([]map[string]any, error) {
	items := anySlice(raw)
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		include, ok := item.(map[string]any)
		if !ok {
			return nil, errorx.Msg("Tekton DAG Matrix include 格式不正确：" + taskName)
		}
		params := anySlice(include["params"])
		if len(params) == 0 {
			return nil, errorx.Msg("Tekton DAG Matrix include 必须配置 params：" + taskName)
		}
		next := map[string]any{}
		if name := strings.TrimSpace(fmt.Sprint(include["name"])); name != "" {
			next["name"] = name
		}
		nextParams := make([]map[string]any, 0, len(params))
		for _, paramItem := range params {
			param, ok := paramItem.(map[string]any)
			if !ok {
				return nil, errorx.Msg("Tekton DAG Matrix include params 格式不正确：" + taskName)
			}
			name := strings.TrimSpace(fmt.Sprint(param["name"]))
			value := firstNonNil(param["value"], param["values"])
			if name == "" || !tektonDagNonEmptyString(value) {
				return nil, errorx.Msg("Tekton DAG Matrix include 参数必须配置 name 和 value：" + taskName)
			}
			nextParams = append(nextParams, map[string]any{"name": name, "value": value})
		}
		next["params"] = nextParams
		result = append(result, next)
	}
	return result, nil
}

func buildTektonDagPipelineRef(ref tektonDagPipelineRef) (map[string]any, error) {
	if err := validateTektonDagPipelineRefShape(ref, nil); err != nil {
		return nil, err
	}
	resolver := strings.TrimSpace(ref.Resolver)
	if resolver != "" || len(ref.Params) > 0 {
		if resolver == "" {
			resolver = "cluster"
		}
		params := make([]map[string]any, 0, len(ref.Params))
		for _, item := range ref.Params {
			if strings.TrimSpace(item.Name) == "" {
				continue
			}
			params = append(params, map[string]any{"name": strings.TrimSpace(item.Name), "value": item.Value})
		}
		out := map[string]any{"resolver": resolver, "params": params}
		if apiVersion := strings.TrimSpace(ref.APIVersion); apiVersion != "" {
			out["apiVersion"] = apiVersion
		}
		return out, nil
	}
	out := map[string]any{"name": strings.TrimSpace(ref.Name)}
	if apiVersion := strings.TrimSpace(ref.APIVersion); apiVersion != "" {
		out["apiVersion"] = apiVersion
	}
	return out, nil
}

func anySlice(value any) []any {
	switch items := value.(type) {
	case []any:
		return items
	case []map[string]any:
		result := make([]any, 0, len(items))
		for _, item := range items {
			result = append(result, item)
		}
		return result
	case []map[string]string:
		result := make([]any, 0, len(items))
		for _, item := range items {
			next := make(map[string]any, len(item))
			for key, value := range item {
				next[key] = value
			}
			result = append(result, next)
		}
		return result
	case []string:
		result := make([]any, 0, len(items))
		for _, item := range items {
			result = append(result, item)
		}
		return result
	default:
		return nil
	}
}

func tektonDagNonEmptyString(value any) bool {
	text, ok := value.(string)
	return ok && strings.TrimSpace(text) != ""
}

func tektonDagAllNonEmptyStrings(values []any) bool {
	if len(values) == 0 {
		return false
	}
	for _, value := range values {
		if !tektonDagNonEmptyString(value) {
			return false
		}
	}
	return true
}

func firstNonNil(values ...any) any {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func buildTektonDagTaskRef(node tektonDagNode) (map[string]any, error) {
	ref := normalizedTektonDagTaskRef(node)
	if strings.TrimSpace(ref.Bundle) != "" {
		return nil, errorx.Msg("Tekton taskRef.bundle 已废弃，请使用 resolver")
	}
	kind, err := normalizeTektonDagTaskKind(ref.Kind, ref.APIVersion)
	if err != nil {
		return nil, err
	}
	ref.Kind = kind
	isResolverRef := strings.TrimSpace(ref.Resolver) != "" || strings.TrimSpace(ref.Namespace) != "" || len(ref.Params) > 0
	if strings.TrimSpace(ref.Name) == "" && (!isResolverRef || len(ref.Params) == 0) {
		return nil, errorx.Msg("Tekton DAG 任务引用不能为空")
	}
	if resolver := strings.TrimSpace(ref.Resolver); resolver != "" || strings.TrimSpace(ref.Namespace) != "" || len(ref.Params) > 0 {
		if resolver == "" {
			resolver = "cluster"
		}
		params := make([]map[string]any, 0, len(ref.Params)+3)
		if len(ref.Params) == 0 {
			params = append(params,
				map[string]any{"name": "kind", "value": strings.ToLower(ref.Kind)},
				map[string]any{"name": "name", "value": ref.Name},
			)
			if strings.TrimSpace(ref.Namespace) != "" {
				params = append(params, map[string]any{"name": "namespace", "value": strings.TrimSpace(ref.Namespace)})
			}
		} else {
			for _, item := range ref.Params {
				if strings.TrimSpace(item.Name) == "" {
					continue
				}
				params = append(params, map[string]any{"name": strings.TrimSpace(item.Name), "value": item.Value})
			}
		}
		out := map[string]any{"resolver": resolver, "params": params}
		if apiVersion := strings.TrimSpace(ref.APIVersion); apiVersion != "" {
			out["apiVersion"] = apiVersion
		}
		return out, nil
	}
	out := map[string]any{"name": ref.Name}
	if ref.Kind != "" && (ref.Kind != "Task" || strings.TrimSpace(ref.APIVersion) != "") {
		out["kind"] = ref.Kind
	}
	if apiVersion := strings.TrimSpace(ref.APIVersion); apiVersion != "" {
		out["apiVersion"] = apiVersion
	}
	return out, nil
}

func validateTektonDagTaskRefReality(ctx context.Context, client *devopstekton.Client, pipeline *model.DevopsPipeline, pipelineNamespace string) error {
	if client == nil || pipeline == nil || !hasTektonDagConfig(pipeline.TektonDagConfig) {
		return nil
	}
	cfg, err := parseTektonDagConfig(pipeline.TektonDagConfig)
	if err != nil {
		return err
	}
	pipelineNamespace = strings.TrimSpace(pipelineNamespace)
	taskInfoByNode := make(map[string]devopstekton.TaskInfo)
	for _, node := range append(enabledTektonDagMainNodes(cfg), enabledTektonDagFinallyNodes(cfg)...) {
		taskInfo, err := tektonDagNodeTaskInfo(ctx, client, node, pipelineNamespace)
		if err != nil {
			return err
		}
		taskInfoByNode[strings.TrimSpace(node.ID)] = taskInfo
	}
	return validateTektonDagTaskSpecMappings(cfg, taskInfoByNode)
}

func tektonDagNodeTaskInfo(ctx context.Context, client *devopstekton.Client, node tektonDagNode, pipelineNamespace string) (devopstekton.TaskInfo, error) {
	if hasTektonDagPipelineRef(node.PipelineRef) || len(node.PipelineSpec) > 0 || len(node.TaskSpec) > 0 {
		return devopstekton.TaskInfo{}, nil
	}
	ref := normalizedTektonDagTaskRef(node)
	kind, err := normalizeTektonDagTaskKind(ref.Kind, ref.APIVersion)
	if err != nil {
		return devopstekton.TaskInfo{}, err
	}
	resolver := strings.TrimSpace(ref.Resolver)
	if resolver != "" || strings.TrimSpace(ref.Namespace) != "" {
		if resolver == "" {
			resolver = "cluster"
		}
		if resolver != "cluster" {
			return devopstekton.TaskInfo{}, nil
		}
		resolvedKind := strings.ToLower(kind)
		resolvedName := ref.Name
		resolvedNamespace := firstNotBlank(ref.Namespace, pipelineNamespace)
		if len(ref.Params) > 0 {
			params := tektonDagTaskRefParamMap(ref.Params)
			resolvedKind = strings.ToLower(firstNotBlank(params["kind"], resolvedKind))
			resolvedName = firstNotBlank(params["name"], resolvedName)
			resolvedNamespace = firstNotBlank(params["namespace"], resolvedNamespace)
		}
		if resolvedKind == "clustertask" {
			return client.GetClusterTask(ctx, resolvedName)
		}
		return client.GetTask(ctx, resolvedNamespace, resolvedName)
	}
	if kind == "ClusterTask" {
		return client.GetClusterTask(ctx, ref.Name)
	}
	if strings.TrimSpace(ref.APIVersion) != "" {
		return devopstekton.TaskInfo{}, nil
	}
	if kind != "Task" {
		return devopstekton.TaskInfo{}, nil
	}
	return client.GetTask(ctx, pipelineNamespace, ref.Name)
}

func tektonDagTaskRefParamMap(params []tektonDagNameValue) map[string]string {
	result := make(map[string]string, len(params))
	for _, item := range params {
		name := strings.TrimSpace(item.Name)
		if name == "" {
			continue
		}
		result[name] = strings.TrimSpace(fmt.Sprint(item.Value))
	}
	return result
}

func validateTektonDagTaskSpecMappings(cfg tektonDagConfigDoc, taskInfoByNode map[string]devopstekton.TaskInfo) error {
	nameByID := tektonDagNodeNameMap(cfg)
	taskInfoByTaskName := map[string]devopstekton.TaskInfo{}
	for _, node := range append(enabledTektonDagMainNodes(cfg), enabledTektonDagFinallyNodes(cfg)...) {
		taskInfo, ok := taskInfoByNode[strings.TrimSpace(node.ID)]
		if !ok || strings.TrimSpace(taskInfo.Name) == "" {
			continue
		}
		taskInfoByTaskName[tektonDagTaskName(node)] = taskInfo
		if err := validateTektonDagTaskParamsExist(node, taskInfo, cfg.Edges, nameByID); err != nil {
			return err
		}
		if err := validateTektonDagTaskWorkspacesExist(node, taskInfo); err != nil {
			return err
		}
	}
	for _, edge := range cfg.Edges {
		if strings.TrimSpace(edge.Type) != "result" {
			continue
		}
		sourceID := strings.TrimSpace(edge.Source)
		taskInfo, ok := taskInfoByNode[sourceID]
		if !ok || strings.TrimSpace(taskInfo.Name) == "" {
			continue
		}
		if !stringListContains(taskInfo.Results, edge.ResultName) {
			return errorx.Msg("Tekton Result 不存在：" + nameByID[sourceID] + "." + strings.TrimSpace(edge.ResultName))
		}
	}
	for _, result := range append(cfg.Pipeline.Results, cfg.Results...) {
		for _, ref := range extractTektonResultRefs(strings.TrimSpace(fmt.Sprint(result.Value))) {
			taskInfo, ok := taskInfoByTaskName[ref.TaskName]
			if !ok {
				return errorx.Msg("Tekton Pipeline Result 引用的 Task 不存在：" + ref.TaskName)
			}
			if !stringListContains(taskInfo.Results, ref.ResultName) {
				return errorx.Msg("Tekton Pipeline Result 引用的 Task Result 不存在：" + ref.TaskName + "." + ref.ResultName)
			}
		}
	}
	return nil
}

func validateTektonDagTaskParamsExist(node tektonDagNode, taskInfo devopstekton.TaskInfo, edges []tektonDagEdge, nameByID map[string]string) error {
	params, err := tektonDagTaskParams(node.Params, edges, node.ID, nameByID, node.ParamRequirements, tektonDagTaskName(node))
	if err != nil {
		return err
	}
	for _, item := range params {
		name := strings.TrimSpace(fmt.Sprint(item["name"]))
		if name == "" {
			continue
		}
		if !stringListContains(taskInfo.Params, name) {
			return errorx.Msg("Tekton Task 参数不存在：" + tektonDagTaskName(node) + "." + name)
		}
	}
	matrixParams := tektonDagMatrixAllParamNames(node.Matrix)
	for _, name := range matrixParams {
		if !stringListContains(taskInfo.Params, name) {
			return errorx.Msg("Tekton Matrix 参数不存在：" + tektonDagTaskName(node) + "." + name)
		}
	}
	return nil
}

func validateTektonDagTaskWorkspacesExist(node tektonDagNode, taskInfo devopstekton.TaskInfo) error {
	for _, item := range tektonDagTaskWorkspaces(node.Workspaces) {
		name := strings.TrimSpace(fmt.Sprint(item["name"]))
		if name == "" {
			continue
		}
		if !stringListContains(taskInfo.Workspaces, name) {
			return errorx.Msg("Tekton Task Workspace 不存在：" + tektonDagTaskName(node) + "." + name)
		}
	}
	return nil
}

func tektonDagMatrixParamNames(matrix map[string]any) []string {
	result := make([]string, 0)
	params := anySlice(firstNonNil(matrix["params"], matrix["Params"]))
	for _, item := range params {
		data, ok := item.(map[string]any)
		if !ok {
			continue
		}
		name := strings.TrimSpace(fmt.Sprint(data["name"]))
		if name != "" {
			result = append(result, name)
		}
	}
	return result
}

func tektonDagMatrixAllParamNames(matrix map[string]any) []string {
	seen := map[string]struct{}{}
	add := func(name string, result *[]string) {
		name = strings.TrimSpace(name)
		if name == "" {
			return
		}
		if _, ok := seen[name]; ok {
			return
		}
		seen[name] = struct{}{}
		*result = append(*result, name)
	}
	result := make([]string, 0)
	for _, name := range tektonDagMatrixParamNames(matrix) {
		add(name, &result)
	}
	includes := anySlice(firstNonNil(matrix["include"], matrix["Include"]))
	for _, item := range includes {
		include, ok := item.(map[string]any)
		if !ok {
			continue
		}
		params := anySlice(firstNonNil(include["params"], include["Params"]))
		for _, rawParam := range params {
			param, ok := rawParam.(map[string]any)
			if !ok {
				continue
			}
			add(fmt.Sprint(param["name"]), &result)
		}
	}
	return result
}

func stringListContains(items []string, value string) bool {
	value = strings.TrimSpace(value)
	for _, item := range items {
		if strings.TrimSpace(item) == value {
			return true
		}
	}
	return false
}

func tektonDagTaskParams(raw json.RawMessage, edges []tektonDagEdge, nodeID string, nameByID map[string]string, requirements []tektonDagParamRequirement, taskName string) ([]map[string]any, error) {
	result := make([]map[string]any, 0)
	result = append(result, tektonDagTaskParamsFromRaw(raw)...)
	for _, edge := range edges {
		edgeType := strings.TrimSpace(edge.Type)
		if edgeType != "result" || strings.TrimSpace(edge.Target) != nodeID {
			continue
		}
		paramName := firstNotBlank(edge.TargetParam, edge.TargetParamName, edge.ParamName)
		resultName := strings.TrimSpace(edge.ResultName)
		sourceTask := nameByID[strings.TrimSpace(edge.Source)]
		if paramName == "" || resultName == "" || sourceTask == "" {
			continue
		}
		value := fmt.Sprintf("$(tasks.%s.results.%s)", sourceTask, resultName)
		replaced := false
		for index := range result {
			if strings.TrimSpace(fmt.Sprint(result[index]["name"])) == paramName {
				result[index]["value"] = value
				replaced = true
				break
			}
		}
		if !replaced {
			result = append(result, map[string]any{"name": paramName, "value": value})
		}
	}
	if err := applyTektonDagTaskParamTypes(result, requirements, taskName); err != nil {
		return nil, err
	}
	return result, nil
}

func applyTektonDagTaskParamTypes(params []map[string]any, requirements []tektonDagParamRequirement, taskName string) error {
	typeByName := map[string]string{}
	for _, item := range requirements {
		name := strings.TrimSpace(item.Name)
		if name != "" {
			typeByName[name] = tektonParamType(firstNotBlank(item.Type, "string"))
		}
	}
	for _, item := range params {
		name := firstNotBlank(tektonDagString(item["name"]), tektonDagString(item["code"]))
		paramType := typeByName[name]
		if name == "" || paramType == "" || paramType == "string" {
			continue
		}
		value, ok := item["value"]
		if !ok || value == nil {
			continue
		}
		if text, ok := value.(string); ok && strings.Contains(text, "$(") {
			continue
		}
		parsed, err := tektonParamDefaultValue(paramType, value, taskName+"."+name)
		if err != nil {
			return err
		}
		item["value"] = parsed
	}
	return nil
}

func tektonDagWhenFromEdges(edges []tektonDagEdge, nodeID string) []map[string]any {
	result := make([]map[string]any, 0)
	for _, edge := range edges {
		if strings.TrimSpace(edge.Type) != "when" || strings.TrimSpace(edge.Target) != nodeID {
			continue
		}
		input := firstNotBlank(edge.WhenInput, edge.Condition)
		if input == "" || len(edge.WhenValues) == 0 {
			continue
		}
		operator := firstNotBlank(edge.WhenOperator, "in")
		if operator != "in" && operator != "notin" {
			operator = "in"
		}
		values := make([]any, 0, len(edge.WhenValues))
		for _, item := range edge.WhenValues {
			if value := strings.TrimSpace(fmt.Sprint(item)); value != "" {
				values = append(values, value)
			}
		}
		if len(values) == 0 {
			continue
		}
		result = append(result, map[string]any{
			"input":    input,
			"operator": operator,
			"values":   values,
		})
	}
	return result
}

func tektonDagTaskParamsFromRaw(raw json.RawMessage) []map[string]any {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var list []map[string]any
	if err := json.Unmarshal(raw, &list); err == nil {
		result := make([]map[string]any, 0, len(list))
		for _, item := range list {
			if param := tektonDagParamMap(item); param != nil {
				result = append(result, param)
			}
		}
		return result
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil
	}
	result := make([]map[string]any, 0, len(obj))
	keys := sortedMapKeys(obj)
	for _, key := range keys {
		value := obj[key]
		if data, ok := value.(map[string]any); ok {
			data["name"] = key
			if param := tektonDagParamMap(data); param != nil {
				result = append(result, param)
			}
			continue
		}
		result = append(result, map[string]any{"name": key, "value": value})
	}
	return result
}

func tektonDagParamMap(item map[string]any) map[string]any {
	name := firstNotBlank(fmt.Sprint(item["name"]), fmt.Sprint(item["code"]))
	if name == "" || name == "<nil>" {
		return nil
	}
	value := item["value"]
	if value == nil {
		value = item["default"]
	}
	if value == nil {
		value = tektonDagResultValue(item["resultRef"])
	}
	if value == nil {
		if source := strings.TrimSpace(fmt.Sprint(item["source"])); source == "pipelineParam" || source == "runtime" || source == "param" {
			paramName := firstNotBlank(
				tektonDagString(item["pipelineParam"]),
				tektonDagString(item["paramName"]),
				tektonDagString(item["sourceParam"]),
				name,
			)
			value = fmt.Sprintf("$(params.%s)", paramName)
		}
	}
	if value == nil {
		value = ""
	}
	return map[string]any{"name": name, "value": value}
}

func tektonDagString(value any) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func tektonDagResultValue(raw any) any {
	switch data := raw.(type) {
	case nil:
		return nil
	case string:
		parts := strings.Split(strings.TrimSpace(data), ".")
		if len(parts) >= 2 {
			return fmt.Sprintf("$(tasks.%s.results.%s)", parts[0], parts[1])
		}
	case map[string]any:
		taskName := firstNotBlank(fmt.Sprint(data["taskName"]), fmt.Sprint(data["sourceTask"]), fmt.Sprint(data["task"]))
		resultName := firstNotBlank(fmt.Sprint(data["resultName"]), fmt.Sprint(data["result"]))
		if taskName != "" && resultName != "" {
			return fmt.Sprintf("$(tasks.%s.results.%s)", taskName, resultName)
		}
	}
	return nil
}

func tektonDagTaskWorkspaces(raw json.RawMessage) []map[string]any {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var list []map[string]any
	if err := json.Unmarshal(raw, &list); err == nil {
		return cleanTektonDagWorkspaces(list)
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil
	}
	result := make([]map[string]any, 0, len(obj))
	keys := sortedMapKeys(obj)
	for _, key := range keys {
		value := obj[key]
		if data, ok := value.(map[string]any); ok {
			data["name"] = key
			result = append(result, data)
			continue
		}
		result = append(result, map[string]any{"name": key, "workspace": fmt.Sprint(value)})
	}
	return cleanTektonDagWorkspaces(result)
}

func cleanTektonDagWorkspaces(items []map[string]any) []map[string]any {
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		name := strings.TrimSpace(fmt.Sprint(item["name"]))
		workspace := firstNotBlank(fmt.Sprint(item["workspace"]), fmt.Sprint(item["pipelineWorkspace"]), fmt.Sprint(item["workspaceName"]))
		if name == "" || name == "<nil>" || workspace == "" || workspace == "<nil>" {
			continue
		}
		out := map[string]any{"name": name, "workspace": workspace}
		if subPath := strings.TrimSpace(fmt.Sprint(item["subPath"])); subPath != "" && subPath != "<nil>" {
			out["subPath"] = subPath
		}
		result = append(result, out)
	}
	return result
}

func tektonDagPipelineParams(cfg tektonDagConfigDoc, params []model.PipelineParam) ([]map[string]any, error) {
	items := tektonDagDeclaredPipelineParams(cfg)
	if len(items) > 0 {
		result := make([]map[string]any, 0, len(items))
		for _, item := range items {
			name := strings.TrimSpace(item.Name)
			if name == "" {
				continue
			}
			paramType := tektonParamType(firstNotBlank(item.Type, "string"))
			param := map[string]any{"name": name, "type": paramType}
			defaultValue := item.Default
			if defaultValue == nil {
				defaultValue = item.DefaultValue
			}
			if defaultValue != nil && (!item.Required || tektonDagHasProvidedValue(defaultValue)) {
				parsedDefault, err := tektonParamDefaultValue(paramType, defaultValue, name)
				if err != nil {
					return nil, err
				}
				param["default"] = parsedDefault
			}
			if strings.TrimSpace(item.Description) != "" {
				param["description"] = item.Description
			}
			if len(item.Enum) > 0 {
				param["enum"] = item.Enum
			}
			if len(item.Properties) > 0 && paramType == "object" {
				properties := map[string]any{}
				for propertyName, property := range item.Properties {
					properties[propertyName] = map[string]any{
						"type": tektonParamType(property.Type),
					}
				}
				param["properties"] = properties
			}
			result = append(result, param)
		}
		return result, nil
	}
	return tektonPipelineParams(params)
}

func tektonDagDeclaredPipelineParams(cfg tektonDagConfigDoc) []tektonDagPipelineParam {
	items := append([]tektonDagPipelineParam{}, cfg.Pipeline.Params...)
	items = append(items, cfg.Params...)
	result := make([]tektonDagPipelineParam, 0, len(items))
	seen := map[string]struct{}{}
	for _, item := range items {
		name := strings.TrimSpace(item.Name)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		item.Name = name
		result = append(result, item)
	}
	return result
}

func tektonDagPipelineResults(cfg tektonDagConfigDoc) []map[string]any {
	items := append([]tektonDagPipelineResult{}, cfg.Pipeline.Results...)
	items = append(items, cfg.Results...)
	result := make([]map[string]any, 0, len(items))
	seen := map[string]struct{}{}
	for _, item := range items {
		name := strings.TrimSpace(item.Name)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		value := strings.TrimSpace(fmt.Sprint(item.Value))
		if value == "" || value == "<nil>" {
			continue
		}
		out := map[string]any{
			"name":  name,
			"type":  tektonParamType(firstNotBlank(item.Type, "string")),
			"value": item.Value,
		}
		if len(item.Properties) > 0 && tektonParamType(firstNotBlank(item.Type, "string")) == "object" {
			properties := map[string]any{}
			for propertyName, property := range item.Properties {
				properties[propertyName] = map[string]any{"type": tektonParamType(property.Type)}
			}
			out["properties"] = properties
		}
		if description := strings.TrimSpace(item.Description); description != "" {
			out["description"] = description
		}
		result = append(result, out)
	}
	return result
}

func tektonDagPipelineWorkspaces(cfg tektonDagConfigDoc, tasks, finallyTasks []map[string]any) []map[string]any {
	items := append([]tektonDagWorkspace{}, cfg.Pipeline.Workspaces...)
	items = append(items, cfg.Workspaces...)
	if len(items) > 0 {
		result := make([]map[string]any, 0, len(items))
		seen := map[string]struct{}{}
		for _, item := range items {
			name := strings.TrimSpace(item.Name)
			if name == "" {
				continue
			}
			if _, ok := seen[name]; ok {
				continue
			}
			seen[name] = struct{}{}
			workspace := map[string]any{"name": name}
			if strings.TrimSpace(item.Description) != "" {
				workspace["description"] = item.Description
			}
			if item.Optional {
				workspace["optional"] = true
			}
			result = append(result, workspace)
		}
		return result
	}
	return tektonPipelineWorkspaceDeclarations(append(tasks, finallyTasks...))
}

func tektonDagPipelineRunWorkspaces(pipeline *model.DevopsPipeline, runtimeParams map[string]string) ([]map[string]any, error) {
	cfg, err := parseTektonDagConfig(pipeline.TektonDagConfig)
	if err != nil {
		return nil, err
	}
	items := append([]tektonDagWorkspace{}, cfg.Pipeline.Workspaces...)
	items = append(items, cfg.Workspaces...)
	policy, err := tektonRunPolicyFromPipeline(pipeline)
	if err != nil {
		return nil, err
	}
	items = append(items, policy.Workspaces...)
	if len(items) == 0 {
		_, finallyTasks, err := tektonDagTasks(cfg)
		if err != nil {
			return nil, err
		}
		tasks, _, err := tektonDagTasks(cfg)
		if err != nil {
			return nil, err
		}
		for _, workspace := range tektonPipelineWorkspaceDeclarations(append(tasks, finallyTasks...)) {
			items = append(items, tektonDagWorkspace{Name: strings.TrimSpace(fmt.Sprint(workspace["name"]))})
		}
	}
	result := make([]map[string]any, 0, len(items))
	seen := map[string]struct{}{}
	for _, item := range items {
		applyTektonWorkspaceStrategy(&item, policy.WorkspaceStrategy)
		workspace, err := tektonDagPipelineRunWorkspace(pipeline, item, runtimeParams)
		if err != nil {
			return nil, err
		}
		if workspace == nil {
			continue
		}
		name := strings.TrimSpace(fmt.Sprint(workspace["name"]))
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		result = append(result, workspace)
	}
	return result, nil
}

func applyTektonWorkspaceStrategy(item *tektonDagWorkspace, strategy string) {
	if item == nil {
		return
	}
	strategy = strings.TrimSpace(strategy)
	if strategy == "" || strategy == "emptyDir" {
		return
	}
	bindingType := strings.TrimSpace(firstNotBlank(item.BindingType, item.Strategy, item.Type))
	if bindingType == "" || bindingType == "emptyDir" {
		item.BindingType = strategy
	}
}

func tektonDagPipelineRunWorkspace(pipeline *model.DevopsPipeline, item tektonDagWorkspace, runtimeParams map[string]string) (map[string]any, error) {
	name := strings.TrimSpace(item.Name)
	if name == "" {
		return nil, nil
	}
	if len(item.Binding) > 0 {
		out := map[string]any{"name": name}
		for key, value := range item.Binding {
			out[key] = value
		}
		addTektonDagSubPath(out, item)
		return out, nil
	}
	if len(item.EmptyDir) > 0 {
		out := map[string]any{"name": name, "emptyDir": item.EmptyDir}
		addTektonDagSubPath(out, item)
		return out, nil
	}
	if len(item.PersistentVolumeClaim) > 0 {
		out := map[string]any{"name": name, "persistentVolumeClaim": item.PersistentVolumeClaim}
		addTektonDagSubPath(out, item)
		return out, nil
	}
	if len(item.VolumeClaimTemplate) > 0 {
		out := map[string]any{"name": name, "volumeClaimTemplate": item.VolumeClaimTemplate}
		addTektonDagSubPath(out, item)
		return out, nil
	}
	if len(item.Secret) > 0 {
		out := map[string]any{"name": name, "secret": item.Secret}
		addTektonDagSubPath(out, item)
		return out, nil
	}
	if len(item.ConfigMap) > 0 {
		out := map[string]any{"name": name, "configMap": item.ConfigMap}
		addTektonDagSubPath(out, item)
		return out, nil
	}
	if len(item.Projected) > 0 {
		out := map[string]any{"name": name, "projected": item.Projected}
		addTektonDagSubPath(out, item)
		return out, nil
	}
	bindingType := firstNotBlank(item.BindingType, item.Strategy, item.Type)
	switch bindingType {
	case "", "emptyDir":
		out := map[string]any{"name": name, "emptyDir": map[string]any{}}
		addTektonDagSubPath(out, item)
		return out, nil
	case "persistentVolumeClaim", "pvc", "sharedPVC":
		claimName := item.ClaimName
		if item.RuntimeConfig {
			claimName = firstNotBlank(runtimeParams[name], item.ClaimName)
		}
		if claimName == "" {
			if item.Optional {
				return nil, nil
			}
			return nil, errorx.Msg("PVC Workspace 绑定不能为空：" + name)
		}
		out := map[string]any{"name": name, "persistentVolumeClaim": map[string]any{"claimName": claimName}}
		addTektonDagSubPath(out, item)
		return out, nil
	case "volumeClaimTemplate", "dynamicPVC":
		claim, err := tektonDagVolumeClaimTemplate(item)
		if err != nil {
			return nil, err
		}
		out := map[string]any{"name": name, "volumeClaimTemplate": claim}
		addTektonDagSubPath(out, item)
		return out, nil
	case "secret":
		secretName := item.SecretName
		if item.RuntimeConfig {
			secretName = firstNotBlank(runtimeParams[name], item.SecretName)
		}
		if secretName == "" {
			if item.Optional {
				return nil, nil
			}
			return nil, errorx.Msg("Secret Workspace 绑定不能为空：" + name)
		}
		out := map[string]any{"name": name, "secret": map[string]any{"secretName": secretName}}
		addTektonDagSubPath(out, item)
		return out, nil
	case "generatedSecret":
		secretName := firstNotBlank(item.SecretName, tektonGeneratedWorkspaceSecretName(pipeline, item))
		if secretName == "" {
			if item.Optional {
				return nil, nil
			}
			return nil, errorx.Msg("生成 Secret Workspace 名称不能为空：" + name)
		}
		out := map[string]any{"name": name, "secret": map[string]any{"secretName": secretName}}
		addTektonDagSubPath(out, item)
		return out, nil
	case "configMap":
		configMapName := item.ConfigMapName
		if item.RuntimeConfig {
			configMapName = firstNotBlank(runtimeParams[name], item.ConfigMapName)
		}
		if configMapName == "" {
			if item.Optional {
				return nil, nil
			}
			return nil, errorx.Msg("ConfigMap Workspace 绑定不能为空：" + name)
		}
		out := map[string]any{"name": name, "configMap": map[string]any{"name": configMapName}}
		addTektonDagSubPath(out, item)
		return out, nil
	case "projected":
		projected := item.Projected
		if len(projected) == 0 {
			if raw, ok := item.Config["projected"].(map[string]any); ok {
				projected = raw
			}
		}
		if len(projected) == 0 {
			if item.Optional {
				return nil, nil
			}
			return nil, errorx.Msg("Projected Workspace 绑定不能为空：" + name)
		}
		out := map[string]any{"name": name, "projected": projected}
		addTektonDagSubPath(out, item)
		return out, nil
	default:
		return nil, errorx.Msg("Workspace 绑定类型不支持：" + bindingType)
	}
}

func tektonDagVolumeClaimTemplate(item tektonDagWorkspace) (map[string]any, error) {
	storage := firstNotBlank(item.Storage, "10Gi")
	if _, err := resource.ParseQuantity(storage); err != nil {
		return nil, errorx.Msg("动态 PVC 存储容量不合法：" + item.Name)
	}
	accessMode := firstNotBlank(item.AccessMode, "ReadWriteOnce")
	if !isSupportedTektonAccessMode(accessMode) {
		return nil, errorx.Msg("动态 PVC accessMode 不支持：" + item.Name)
	}
	spec := map[string]any{
		"accessModes": []string{accessMode},
		"resources": map[string]any{
			"requests": map[string]any{"storage": storage},
		},
	}
	if strings.TrimSpace(item.StorageClassName) != "" {
		spec["storageClassName"] = strings.TrimSpace(item.StorageClassName)
	}
	if strings.TrimSpace(item.VolumeMode) != "" {
		spec["volumeMode"] = strings.TrimSpace(item.VolumeMode)
	}
	return map[string]any{"spec": spec}, nil
}

func addTektonDagSubPath(out map[string]any, item tektonDagWorkspace) {
	if subPath := strings.TrimSpace(item.SubPath); subPath != "" {
		out["subPath"] = subPath
	}
}

func tektonDagCleanupWorkspaceMappings(cfg tektonDagConfigDoc, tasks, finallyTasks []map[string]any) []map[string]string {
	declared := tektonDagPipelineWorkspaces(cfg, tasks, finallyTasks)
	if len(declared) == 0 {
		return nil
	}
	writable, hasConfiguredWorkspaces := tektonDagWritableWorkspaceNames(cfg)
	result := make([]map[string]string, 0, len(declared))
	for _, item := range declared {
		name := strings.TrimSpace(fmt.Sprint(item["name"]))
		if name == "" {
			continue
		}
		if hasConfiguredWorkspaces {
			if _, ok := writable[name]; !ok {
				continue
			}
		}
		result = append(result, map[string]string{"name": name, "workspace": name})
	}
	return result
}

func tektonDagWritableWorkspaceNames(cfg tektonDagConfigDoc) (map[string]struct{}, bool) {
	items := append([]tektonDagWorkspace{}, cfg.Pipeline.Workspaces...)
	items = append(items, cfg.Workspaces...)
	items = append(items, cfg.RunPolicy.Workspaces...)
	result := map[string]struct{}{}
	for _, item := range items {
		name := strings.TrimSpace(item.Name)
		if name == "" || !tektonDagWorkspaceCanCleanup(item) {
			continue
		}
		result[name] = struct{}{}
	}
	return result, len(items) > 0
}

func tektonDagWorkspaceCanCleanup(item tektonDagWorkspace) bool {
	if len(item.Secret) > 0 || len(item.ConfigMap) > 0 || len(item.Projected) > 0 {
		return false
	}
	bindingType := strings.ToLower(firstNotBlank(item.BindingType, item.Strategy, item.Type))
	return bindingType != "secret" && bindingType != "configmap" && bindingType != "projected"
}

func prependTektonDagCleanupTask(tasks []map[string]any, name string, workspaces []map[string]string, image string) []map[string]any {
	result := make([]map[string]any, 0, len(tasks)+1)
	result = append(result, tektonDagCleanupTask(name, workspaces, image))
	result = append(result, tasks...)
	return result
}

func tektonDagCleanupTask(name string, workspaces []map[string]string, image string) map[string]any {
	taskWorkspaces := make([]map[string]any, 0, len(workspaces))
	taskSpecWorkspaces := make([]map[string]any, 0, len(workspaces))
	steps := make([]map[string]any, 0, len(workspaces))
	for _, item := range workspaces {
		workspaceName := strings.TrimSpace(item["name"])
		pipelineWorkspace := strings.TrimSpace(item["workspace"])
		if workspaceName == "" || pipelineWorkspace == "" {
			continue
		}
		taskWorkspaces = append(taskWorkspaces, map[string]any{"name": workspaceName, "workspace": pipelineWorkspace})
		taskSpecWorkspaces = append(taskSpecWorkspaces, map[string]any{"name": workspaceName})
		steps = append(steps, map[string]any{
			"name":   tektonResourceName("clean", workspaceName),
			"image":  firstNotBlank(image, "busybox:1.36"),
			"script": tektonDagCleanupScript(workspaceName),
		})
	}
	return map[string]any{
		"name":       name,
		"workspaces": taskWorkspaces,
		"taskSpec": map[string]any{
			"workspaces": taskSpecWorkspaces,
			"steps":      steps,
		},
	}
}

func tektonDagCleanupScript(workspaceName string) string {
	return fmt.Sprintf("#!/bin/sh\nset -eu\nTARGET=\"$(workspaces.%s.path)\"\nif [ -n \"$TARGET\" ] && [ -d \"$TARGET\" ]; then\n  rm -rf \"$TARGET\"/* \"$TARGET\"/.[!.]* \"$TARGET\"/..?* 2>/dev/null || true\nfi\n", workspaceName)
}

func addTektonDagRunAfter(tasks []map[string]any, dependency string) {
	for _, task := range tasks {
		runAfter := tektonDagRunAfterValues(task["runAfter"])
		runAfter = append(runAfter, dependency)
		task["runAfter"] = uniqueSortedStrings(runAfter)
	}
}

func tektonDagRunAfterValues(value any) []string {
	switch data := value.(type) {
	case nil:
		return nil
	case []string:
		return append([]string{}, data...)
	case []any:
		result := make([]string, 0, len(data))
		for _, item := range data {
			result = append(result, fmt.Sprint(item))
		}
		return result
	case []map[string]any:
		return nil
	default:
		return []string{fmt.Sprint(data)}
	}
}

func uniqueTektonDagGeneratedTaskName(base string, groups ...[]map[string]any) string {
	used := map[string]struct{}{}
	for _, group := range groups {
		for _, task := range group {
			name := strings.TrimSpace(fmt.Sprint(task["name"]))
			if name != "" {
				used[name] = struct{}{}
			}
		}
	}
	name := tektonResourceName(base)
	if _, ok := used[name]; !ok {
		return name
	}
	for index := 1; index < 20; index++ {
		candidate := tektonResourceName(base, fmt.Sprintf("%d", index))
		if _, ok := used[candidate]; !ok {
			return candidate
		}
	}
	return tektonResourceName(base, "task")
}

func applyTektonDagPipelineRunSpec(spec map[string]any, pipeline *model.DevopsPipeline, binding tektonBindingConfig, policy tektonDagRunPolicy) error {
	cfg, err := parseTektonDagConfig(pipeline.TektonDagConfig)
	if err != nil {
		return err
	}
	if managedBy := strings.TrimSpace(policy.ManagedBy); managedBy != "" {
		spec["managedBy"] = managedBy
	}
	timeouts := map[string]string{}
	for key, value := range cfg.Pipeline.Timeouts {
		if strings.TrimSpace(value) != "" {
			timeouts[key] = strings.TrimSpace(value)
		}
	}
	for key, value := range policy.Timeouts {
		if strings.TrimSpace(value) != "" {
			timeouts[key] = strings.TrimSpace(value)
		}
	}
	if timeout := firstNotBlank(policy.Timeout, cfg.Pipeline.Timeout); timeout != "" {
		timeouts["pipeline"] = timeout
	}
	if timeout := strings.TrimSpace(policy.PipelineTimeout); timeout != "" {
		timeouts["pipeline"] = timeout
	}
	if timeout := strings.TrimSpace(policy.TasksTimeout); timeout != "" {
		timeouts["tasks"] = timeout
	}
	if timeout := strings.TrimSpace(policy.FinallyTimeout); timeout != "" {
		timeouts["finally"] = timeout
	}
	if len(timeouts) > 0 {
		spec["timeouts"] = stringMapToAny(timeouts)
	}
	if taskRunTemplate, err := tektonDagRunPolicyRawObject(policy.TaskRunTemplateRaw, "taskRunTemplate"); err != nil {
		return err
	} else if len(taskRunTemplate) > 0 {
		spec["taskRunTemplate"] = taskRunTemplate
	}
	if sa := firstNotBlank(policy.ServiceAccountName, cfg.Pipeline.ServiceAccountName, binding.ServiceAccountName); sa != "" {
		applyTektonPipelineRunServiceAccount(spec, sa)
	}
	if podTemplate, err := tektonDagRunPolicyRawObject(policy.PodTemplateRaw, "podTemplate"); err != nil {
		return err
	} else if len(podTemplate) > 0 {
		mergeTektonPipelineRunPodTemplate(spec, podTemplate)
	}
	if env := tektonDagRunPolicyEnv(policy.PodTemplateEnv); len(env) > 0 {
		podTemplate := ensureTektonPipelineRunPodTemplate(spec)
		podTemplate["env"] = appendTektonPodEnv(podTemplate["env"], env)
	}
	taskRunSpecs, err := tektonDagRunPolicyTaskRunSpecs(policy)
	if err != nil {
		return err
	}
	if err := validateTektonDagTaskRunSpecRefs(taskRunSpecs, cfg); err != nil {
		return err
	}
	if len(taskRunSpecs) > 0 {
		spec["taskRunSpecs"] = taskRunSpecs
	}
	return nil
}

func tektonDagRunPolicyRawObject(raw, name string) (map[string]any, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	var item map[string]any
	if err := yaml.Unmarshal([]byte(raw), &item); err != nil || len(item) == 0 {
		return nil, errorx.Msg("PipelineRun " + name + " 配置不是合法 YAML 对象")
	}
	return item, nil
}

func tektonDagRunPolicyEnv(items []tektonDagNameValue) []map[string]any {
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		name := strings.TrimSpace(item.Name)
		value := strings.TrimSpace(fmt.Sprint(item.Value))
		if name == "" || value == "" {
			continue
		}
		result = append(result, map[string]any{"name": name, "value": value})
	}
	return result
}

func tektonDagRunPolicyTaskRunSpecs(policy tektonDagRunPolicy) ([]map[string]any, error) {
	if len(policy.TaskRunSpecs) > 0 {
		specs := normalizeTektonDagTaskRunSpecs(policy.TaskRunSpecs)
		if err := validateTektonDagTaskRunSpecs(specs); err != nil {
			return nil, err
		}
		return specs, nil
	}
	raw := strings.TrimSpace(policy.TaskRunSpecsRaw)
	if raw == "" {
		return nil, nil
	}
	var specs []map[string]any
	if err := yaml.Unmarshal([]byte(raw), &specs); err == nil {
		specs = normalizeTektonDagTaskRunSpecs(specs)
		if err := validateTektonDagTaskRunSpecs(specs); err != nil {
			return nil, err
		}
		return specs, nil
	}
	var spec map[string]any
	if err := yaml.Unmarshal([]byte(raw), &spec); err != nil || len(spec) == 0 {
		return nil, errorx.Msg("PipelineRun taskRunSpecs 配置不是合法 YAML")
	}
	specs = []map[string]any{spec}
	specs = normalizeTektonDagTaskRunSpecs(specs)
	if err := validateTektonDagTaskRunSpecs(specs); err != nil {
		return nil, err
	}
	return specs, nil
}

func normalizeTektonDagTaskRunSpecs(specs []map[string]any) []map[string]any {
	result := make([]map[string]any, 0, len(specs))
	for _, item := range specs {
		next := make(map[string]any, len(item))
		for key, value := range item {
			next[key] = value
		}
		aliases := map[string]string{
			"taskServiceAccountName": "serviceAccountName",
			"taskPodTemplate":        "podTemplate",
			"stepOverrides":          "stepSpecs",
			"sidecarOverrides":       "sidecarSpecs",
		}
		for legacyKey, v1Key := range aliases {
			if hasTektonDagTaskRunSpecValue(next[legacyKey]) && !hasTektonDagTaskRunSpecValue(next[v1Key]) {
				next[v1Key] = next[legacyKey]
			}
			delete(next, legacyKey)
		}
		result = append(result, next)
	}
	return result
}

func hasTektonDagTaskRunSpecValue(value any) bool {
	switch data := value.(type) {
	case nil:
		return false
	case string:
		return strings.TrimSpace(data) != ""
	case []any:
		return len(data) > 0
	case map[string]any:
		return len(data) > 0
	default:
		return true
	}
}

func validateTektonDagTaskRunSpecs(specs []map[string]any) error {
	for _, item := range specs {
		if strings.TrimSpace(fmt.Sprint(item["pipelineTaskName"])) == "" || fmt.Sprint(item["pipelineTaskName"]) == "<nil>" {
			return errorx.Msg("PipelineRun taskRunSpecs 每一项必须配置 pipelineTaskName")
		}
	}
	return nil
}

func validateTektonDagTaskRunSpecRefs(specs []map[string]any, cfg tektonDagConfigDoc) error {
	if len(specs) == 0 {
		return nil
	}
	taskNames := map[string]struct{}{}
	for _, node := range append(enabledTektonDagMainNodes(cfg), enabledTektonDagFinallyNodes(cfg)...) {
		if name := tektonDagTaskName(node); name != "" {
			taskNames[name] = struct{}{}
		}
	}
	for _, item := range specs {
		name := strings.TrimSpace(fmt.Sprint(item["pipelineTaskName"]))
		if _, ok := taskNames[name]; !ok {
			return errorx.Msg("PipelineRun taskRunSpecs 引用的 PipelineTask 不存在：" + name)
		}
	}
	return nil
}

func enabledTektonDagMainNodes(cfg tektonDagConfigDoc) []tektonDagNode {
	result := make([]tektonDagNode, 0, len(cfg.Nodes))
	for _, node := range cfg.Nodes {
		if !tektonDagNodeEnabled(node) || tektonDagNodeIsFinally(node) || tektonDagNodeIsNonRuntime(node) {
			continue
		}
		result = append(result, node)
	}
	return result
}

func enabledTektonDagFinallyNodes(cfg tektonDagConfigDoc) []tektonDagNode {
	result := make([]tektonDagNode, 0, len(cfg.FinallyNodes))
	for _, node := range cfg.FinallyNodes {
		if !tektonDagNodeEnabled(node) || tektonDagNodeIsNonRuntime(node) {
			continue
		}
		result = append(result, node)
	}
	for _, node := range cfg.Nodes {
		if !tektonDagNodeEnabled(node) || !tektonDagNodeIsFinally(node) || tektonDagNodeIsNonRuntime(node) {
			continue
		}
		result = append(result, node)
	}
	return result
}

func tektonDagNodeEnabled(node tektonDagNode) bool {
	return node.Enabled == nil || *node.Enabled
}

func tektonDagNodeIsFinally(node tektonDagNode) bool {
	nodeType := strings.ToLower(strings.TrimSpace(node.Type))
	return nodeType == "finally" || nodeType == "finallytask"
}

func tektonDagNodeIsNonRuntime(node tektonDagNode) bool {
	nodeType := strings.ToLower(strings.TrimSpace(node.Type))
	return nodeType == "start" || nodeType == "note" || nodeType == "manual"
}

func tektonDagNodeNameMap(cfg tektonDagConfigDoc) map[string]string {
	result := map[string]string{}
	for _, node := range append(cfg.Nodes, cfg.FinallyNodes...) {
		if !tektonDagNodeEnabled(node) {
			continue
		}
		id := strings.TrimSpace(node.ID)
		if id == "" {
			continue
		}
		result[id] = tektonDagTaskName(node)
	}
	return result
}

func tektonDagTaskName(node tektonDagNode) string {
	return tektonResourceName(firstNotBlank(node.Name, node.ID, node.TaskName, node.StepCode))
}

func tektonDagRunAfterMap(edges []tektonDagEdge, nameByID map[string]string) map[string][]string {
	result := map[string][]string{}
	for _, edge := range edges {
		sourceID := strings.TrimSpace(edge.Source)
		targetID := strings.TrimSpace(edge.Target)
		if sourceID == "" || targetID == "" {
			continue
		}
		edgeType := strings.TrimSpace(edge.Type)
		if edgeType != "" && edgeType != "runAfter" && edgeType != "result" && edgeType != "when" {
			continue
		}
		sourceName := nameByID[sourceID]
		if sourceName == "" {
			continue
		}
		result[targetID] = append(result[targetID], sourceName)
	}
	return result
}

func uniqueSortedStrings(items []string) []string {
	seen := make(map[string]struct{}, len(items))
	result := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		result = append(result, item)
	}
	sort.Strings(result)
	return result
}

func sortedMapKeys(items map[string]any) []string {
	keys := make([]string, 0, len(items))
	for key := range items {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func mergeStringMap(target map[string]any, source map[string]string) {
	for key, value := range source {
		if strings.TrimSpace(key) == "" || strings.TrimSpace(value) == "" {
			continue
		}
		target[key] = value
	}
}

func stringMapToAny(items map[string]string) map[string]any {
	result := make(map[string]any, len(items))
	for key, value := range items {
		if strings.TrimSpace(key) == "" || strings.TrimSpace(value) == "" {
			continue
		}
		result[key] = value
	}
	return result
}

func createRunStagesForPipeline(ctx context.Context, svcCtx *svc.ServiceContext, runID string, pipeline *model.DevopsPipeline, stageNames map[string]string) error {
	if pipeline != nil && pipeline.EngineType == engineTekton && hasTektonDagConfig(pipeline.TektonDagConfig) {
		return createTektonDagRunStages(ctx, svcCtx, runID, pipeline)
	}
	if pipeline == nil {
		return nil
	}
	return createRunStages(ctx, svcCtx, runID, pipeline.ID.Hex(), pipeline.Steps, stageNames)
}

func createTektonDagRunStages(ctx context.Context, svcCtx *svc.ServiceContext, runID string, pipeline *model.DevopsPipeline) error {
	cfg, err := parseTektonDagConfig(pipeline.TektonDagConfig)
	if err != nil {
		return err
	}
	nodes := append(enabledTektonDagMainNodes(cfg), enabledTektonDagFinallyNodes(cfg)...)
	stageCount := make(map[string]int)
	stages := make([]*model.DevopsPipelineRunStage, 0, len(nodes))
	for _, node := range nodes {
		nodeID := tektonDagNodeID(node)
		if nodeID == "" {
			continue
		}
		stageName := uniqueRunStageName(tektonDagStageName(node), stageCount)
		stageType := firstNotBlank(strings.TrimSpace(node.Type), "task")
		if tektonDagNodeIsFinally(node) {
			stageType = "finally"
		}
		stages = append(stages, &model.DevopsPipelineRunStage{
			RunID:      runID,
			PipelineID: pipeline.ID.Hex(),
			NodeID:     nodeID,
			StageName:  stageName,
			StageType:  stageType,
			Status:     "queued",
		})
	}
	if err := svcCtx.RunStageModel.InsertMany(ctx, stages); err != nil {
		logx.Errorf("创建 Tekton DAG 阶段快照失败: %v", err)
		return err
	}
	return nil
}

func tektonTaskNameToNodeMapForRun(ctx context.Context, svcCtx *svc.ServiceContext, run *model.DevopsPipelineRun) map[string]string {
	if run == nil || strings.TrimSpace(run.PipelineID) == "" {
		return nil
	}
	pipeline, err := svcCtx.PipelineModel.FindOne(ctx, run.PipelineID)
	if err != nil {
		logx.Errorf("读取 Tekton DAG 阶段拓扑失败: %v", err)
		return nil
	}
	return tektonTaskNameToNodeMap(pipeline)
}

func tektonTaskNameToNodeMap(pipeline *model.DevopsPipeline) map[string]string {
	if pipeline == nil || !hasTektonDagConfig(pipeline.TektonDagConfig) {
		return nil
	}
	cfg, err := parseTektonDagConfig(pipeline.TektonDagConfig)
	if err != nil {
		return nil
	}
	nodes := append(enabledTektonDagMainNodes(cfg), enabledTektonDagFinallyNodes(cfg)...)
	result := make(map[string]string, len(nodes))
	for _, node := range nodes {
		taskName := tektonDagTaskName(node)
		nodeID := tektonDagNodeID(node)
		if taskName == "" || nodeID == "" {
			continue
		}
		result[taskName] = nodeID
	}
	return result
}

func tektonDagNodeID(node tektonDagNode) string {
	return firstNotBlank(node.ID, tektonDagTaskName(node))
}

func tektonDagStageName(node tektonDagNode) string {
	return firstNotBlank(node.Name, node.TaskName, node.StepCode, node.ID, "未命名任务")
}
