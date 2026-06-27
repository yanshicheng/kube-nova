package executionservicelogic

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/yanshicheng/kube-nova/application/devops-pipeline-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
)

func validateTektonPolicyConfigs(pipeline *model.DevopsPipeline) error {
	if pipeline == nil || pipeline.EngineType != engineTekton {
		return nil
	}
	if err := validateTektonRunPolicyConfig(pipeline); err != nil {
		return err
	}
	if err := validateTektonTriggerPolicyConfig(pipeline); err != nil {
		return err
	}
	for _, content := range []string{pipeline.TektonTriggerConfig, pipeline.TektonDagConfig} {
		if err := validateTektonSchedulePolicyConfig(content); err != nil {
			return err
		}
	}
	if strings.TrimSpace(pipeline.TektonPrunerPolicyRef) != "" {
		if err := validateTektonPrunerPolicyConfig(pipeline.TektonPrunerPolicyRef); err != nil {
			return err
		}
	}
	if strings.TrimSpace(pipeline.TektonPrunerPolicyRef) == "" && strings.TrimSpace(pipeline.TektonDagConfig) != "" {
		var doc tektonPrunerDagConfig
		if err := json.Unmarshal([]byte(strings.TrimSpace(pipeline.TektonDagConfig)), &doc); err != nil {
			return errorx.Msg("Tekton Pruner 配置必须是结构化 JSON")
		}
		if doc.PrunerPolicy.Enabled {
			if err := validateTektonPrunerPolicy(doc.PrunerPolicy); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateTektonRunPolicyConfig(pipeline *model.DevopsPipeline) error {
	policy, err := tektonRunPolicyFromPipeline(pipeline)
	if err != nil {
		return err
	}
	if serviceAccount := strings.TrimSpace(policy.ServiceAccountName); serviceAccount != "" && !kubernetesSecretNamePattern.MatchString(serviceAccount) {
		return errorx.Msg("Tekton 运行策略 ServiceAccount 名称不符合 Kubernetes 命名规范")
	}
	for _, item := range []struct {
		name  string
		value string
	}{
		{"timeout", policy.Timeout},
		{"pipelineTimeout", policy.PipelineTimeout},
		{"tasksTimeout", policy.TasksTimeout},
		{"finallyTimeout", policy.FinallyTimeout},
	} {
		if strings.TrimSpace(item.value) != "" && !tektonDurationPattern.MatchString(strings.TrimSpace(item.value)) {
			return errorx.Msg("Tekton 运行策略超时时间格式不合法：" + item.name)
		}
	}
	for key, value := range policy.Timeouts {
		switch strings.TrimSpace(key) {
		case "pipeline", "tasks", "finally":
		default:
			return errorx.Msg("Tekton 运行策略 timeouts 只支持 pipeline、tasks、finally")
		}
		if strings.TrimSpace(value) != "" && !tektonDurationPattern.MatchString(strings.TrimSpace(value)) {
			return errorx.Msg("Tekton 运行策略 timeouts 格式不合法：" + key)
		}
	}
	switch strings.TrimSpace(policy.ConcurrencyPolicy) {
	case "", "allow", "skip", "replace":
	default:
		return errorx.Msg("Tekton 运行策略并发策略不支持")
	}
	switch strings.TrimSpace(policy.WorkspaceStrategy) {
	case "", "emptyDir", "persistentVolumeClaim", "volumeClaimTemplate":
	default:
		return errorx.Msg("Tekton 运行策略 Workspace 策略不支持")
	}
	if err := validateTektonRunPolicyNameValues(policy.Labels, "PipelineRun 元数据"); err != nil {
		return err
	}
	if err := validateTektonRunPolicyNameValues(policy.Annotations, "PipelineRun 元数据"); err != nil {
		return err
	}
	if err := validateTektonRunPolicyNameValues(policy.PodTemplateEnv, "podTemplate 环境变量"); err != nil {
		return err
	}
	if _, err := tektonDagRunPolicyRawObject(policy.PodTemplateRaw, "podTemplate"); err != nil {
		return err
	}
	if _, err := tektonDagRunPolicyRawObject(policy.TaskRunTemplateRaw, "taskRunTemplate"); err != nil {
		return err
	}
	taskRunSpecs, err := tektonDagRunPolicyTaskRunSpecs(policy)
	if err != nil {
		return err
	}
	if hasTektonDagConfig(pipeline.TektonDagConfig) {
		cfg, err := parseTektonDagConfig(pipeline.TektonDagConfig)
		if err != nil {
			return err
		}
		if err := validateTektonDagTaskRunSpecRefs(taskRunSpecs, cfg); err != nil {
			return err
		}
	}
	for _, workspace := range policy.Workspaces {
		if strings.TrimSpace(workspace.Name) == "" {
			return errorx.Msg("Tekton 运行策略 Workspace 名称不能为空")
		}
		bindingType := firstNotBlank(workspace.BindingType, workspace.Strategy, workspace.Type, "emptyDir")
		if !isSupportedTektonWorkspaceBindingType(bindingType) {
			return errorx.Msg("Tekton 运行策略 Workspace 绑定类型不支持：" + bindingType)
		}
		if bindingType == "volumeClaimTemplate" || bindingType == "dynamicPVC" {
			if _, err := tektonDagVolumeClaimTemplate(workspace); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateTektonRunPolicyNameValues(items []tektonDagNameValue, label string) error {
	for _, item := range items {
		if strings.TrimSpace(item.Name) == "" || item.Value == nil || strings.TrimSpace(fmt.Sprint(item.Value)) == "" {
			return errorx.Msg(label + "必须同时填写名称和值")
		}
	}
	return nil
}

func validateTektonTriggerPolicyConfig(pipeline *model.DevopsPipeline) error {
	if !hasTektonTriggerConfig(pipeline) {
		return nil
	}
	policy, err := parseTektonTriggerConfig(pipeline)
	if err != nil {
		return err
	}
	if !policy.Enabled {
		return nil
	}
	required := []struct {
		name  string
		value string
	}{
		{"EventListener", policy.EventListenerName},
		{"Trigger", policy.TriggerName},
		{"TriggerBinding", policy.TriggerBindingName},
		{"TriggerTemplate", policy.TriggerTemplateName},
	}
	for _, item := range required {
		if strings.TrimSpace(item.value) == "" {
			return errorx.Msg("Tekton Trigger 必须配置 " + item.name)
		}
		if !kubernetesSecretNamePattern.MatchString(strings.TrimSpace(item.value)) {
			return errorx.Msg("Tekton Trigger 资源名称不合法：" + item.name)
		}
	}
	switch policy.APIVersion {
	case "", "triggers.tekton.dev/v1beta1", "triggers.tekton.dev/v1alpha1":
	default:
		return errorx.Msg("Tekton Trigger apiVersion 不支持")
	}
	switch firstNotBlank(policy.ServiceType, "ClusterIP") {
	case "ClusterIP", "NodePort", "LoadBalancer":
	default:
		return errorx.Msg("Tekton EventListener Service 类型不支持")
	}
	switch tektonTriggerInterceptorType(policy) {
	case "", "none", "github", "gitlab":
	case "cel":
		if strings.TrimSpace(policy.CELFilter) == "" {
			return errorx.Msg("Tekton CEL interceptor 必须配置过滤表达式")
		}
	default:
		return errorx.Msg("Tekton Trigger interceptor 不支持")
	}
	for _, item := range policy.BindingParams {
		if strings.TrimSpace(item.Name) == "" || strings.TrimSpace(item.Value) == "" {
			return errorx.Msg("Tekton TriggerBinding 参数不完整")
		}
	}
	return nil
}

func validateTektonSchedulePolicyConfig(content string) error {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil
	}
	var doc struct {
		ScheduleConfig tektonScheduleConfig `json:"scheduleConfig"`
	}
	if err := json.Unmarshal([]byte(content), &doc); err != nil {
		return errorx.Msg("Tekton 定时配置必须是结构化 JSON")
	}
	if !doc.ScheduleConfig.Enabled {
		return nil
	}
	cfg := doc.ScheduleConfig
	if strings.TrimSpace(cfg.Cron) == "" {
		return errorx.Msg("Tekton 定时 Cron 不能为空")
	}
	if strings.TrimSpace(cfg.Timezone) == "" {
		return errorx.Msg("Tekton 定时时区不能为空")
	}
	if _, err := time.LoadLocation(strings.TrimSpace(cfg.Timezone)); err != nil {
		return errorx.Msg("Tekton 定时时区不合法")
	}
	if _, err := cron.ParseStandard("CRON_TZ=" + strings.TrimSpace(cfg.Timezone) + " " + strings.TrimSpace(cfg.Cron)); err != nil {
		return errorx.Msg("Tekton 定时 Cron 不合法")
	}
	switch strings.ToLower(strings.TrimSpace(cfg.ConcurrencyPolicy)) {
	case "", "allow", "skip", "replace":
	default:
		return errorx.Msg("Tekton 定时并发策略不支持")
	}
	return nil
}

func validateTektonPrunerPolicyConfig(content string) error {
	var policy tektonPrunerPolicy
	if err := json.Unmarshal([]byte(strings.TrimSpace(content)), &policy); err != nil {
		return errorx.Msg("Tekton Pruner 配置必须是结构化 JSON")
	}
	if !policy.Enabled {
		return nil
	}
	return validateTektonPrunerPolicy(policy)
}

func validateTektonPrunerPolicy(policy tektonPrunerPolicy) error {
	switch firstNotBlank(policy.Scope, "pipeline") {
	case "pipeline", "namespace":
	default:
		return errorx.Msg("Tekton Pruner scope 不支持")
	}
	switch firstNotBlank(policy.NativePrunerMode, "disabled") {
	case "disabled", "namespaceConfigMap", "globalConfigMap":
	default:
		return errorx.Msg("Tekton 原生 Pruner 模式不支持")
	}
	if namespace := strings.TrimSpace(policy.NativePrunerTargetNamespace); namespace != "" && !kubernetesSecretNamePattern.MatchString(namespace) {
		return errorx.Msg("Tekton 原生 Pruner Namespace 不合法")
	}
	switch firstNotBlank(policy.NativePrunerConfigLevel, "namespace") {
	case "namespace", "global":
	default:
		return errorx.Msg("Tekton 原生 Pruner 配置级别不支持")
	}
	if len(policy.ResourceTypes) == 0 {
		return errorx.Msg("Tekton Pruner 必须选择清理资源类型")
	}
	for _, item := range policy.ResourceTypes {
		switch item {
		case "PipelineRun", "TaskRun":
		default:
			return errorx.Msg("Tekton Pruner 资源类型不支持：" + item)
		}
	}
	for _, item := range []struct {
		name  string
		value *int
	}{
		{"historyLimit", policy.HistoryLimit},
		{"successfulHistoryLimit", policy.SuccessfulHistoryLimit},
		{"failedHistoryLimit", policy.FailedHistoryLimit},
		{"ttlSecondsAfterFinished", policy.TTLSecondsAfterFinished},
		{"platformRecordRetentionDays", policy.PlatformRecordRetentionDays},
		{"platformRecordLimit", policy.PlatformRecordLimit},
	} {
		if item.value != nil && *item.value < 0 {
			return errorx.Msg("Tekton Pruner 数值不能小于 0：" + item.name)
		}
	}
	if err := validateTektonNameValues(policy.SelectorLabels, "Label selector"); err != nil {
		return err
	}
	return validateTektonNameValues(policy.SelectorAnnotations, "Annotation selector")
}

func validateTektonNameValues(items []tektonNameValue, label string) error {
	for _, item := range items {
		if strings.TrimSpace(item.Name) == "" || strings.TrimSpace(item.Value) == "" {
			return errorx.Msg("Tekton Pruner " + label + " 不完整")
		}
	}
	return nil
}
