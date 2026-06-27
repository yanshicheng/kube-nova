package executionservicelogic

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/client/pipelineconfigservice"
	"github.com/yanshicheng/kube-nova/application/devops-pipeline-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/devops-pipeline-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/common/devops/channelvars"
	devopstekton "github.com/yanshicheng/kube-nova/common/devops/tekton"
	devopstypes "github.com/yanshicheng/kube-nova/common/devops/types"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
	"github.com/zeromicro/go-zero/core/logx"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

type tektonBindingConfig struct {
	Namespace          string `json:"namespace"`
	ServiceAccountName string `json:"serviceAccountName"`
	PipelinePrefix     string `json:"pipelinePrefix"`
}

func syncTektonPipeline(ctx context.Context, svcCtx *svc.ServiceContext, data *model.DevopsPipeline, runtime *pipelineconfigservice.ResolvePipelineRuntimeResp, runtimeParams map[string]string, renderCtx pipelineRenderContext) (string, error) {
	if runtime == nil || runtime.Binding == nil || runtime.Channel == nil {
		return "", errorx.Msg("Tekton 运行时配置不完整")
	}
	if runtime.Binding.ChannelType != engineTekton || runtime.Channel.ChannelType != engineTekton {
		return "", errorx.Msg("Tekton 流水线必须选择 Tekton 构建渠道")
	}
	cfg, err := parseTektonBindingConfig(runtime.Binding.BindingConfig)
	if err != nil {
		return "", err
	}
	projectCode := strings.TrimSpace(data.ProjectCode)
	if runtime.Project != nil && strings.TrimSpace(runtime.Project.Code) != "" {
		projectCode = runtime.Project.Code
	}
	if expected := devopstekton.ProjectNamespace(projectCode); cfg.Namespace != expected {
		return "", errorx.Msg("Tekton 绑定 namespace 必须是 " + expected)
	}
	client, err := devopstekton.NewClient(tektonRequestFromRuntime(runtime))
	if err != nil {
		logx.Errorf("创建 Tekton 客户端失败: %v", err)
		return "", err
	}
	if err := client.NamespaceExists(ctx, cfg.Namespace); err != nil {
		return "", errorx.Msg(err.Error())
	}
	if err := client.ServiceAccountExists(ctx, cfg.Namespace, cfg.ServiceAccountName); err != nil {
		return "", errorx.Msg(err.Error())
	}
	if err := ensureTektonWorkspaceBindings(ctx, svcCtx, client, cfg.Namespace, data, runtimeParams, renderCtx); err != nil {
		return "", err
	}
	taskCfg, err := parseTektonChannelTaskConfig(runtime.Channel.Config)
	if err != nil {
		return "", errorx.Msg(err.Error())
	}
	if err := client.NamespaceExists(ctx, taskCfg.TaskNamespace); err != nil {
		return "", errorx.Msg(err.Error())
	}

	pipelineName := tektonPipelineName(data.Code)
	content, err := renderTektonPipelineContent(ctx, client, data, cfg, taskCfg.TaskNamespace, runtimeParams)
	if err != nil {
		logx.Errorf("渲染 Tekton Pipeline 失败: %v", err)
		return "", err
	}
	if _, err := client.ApplyPipeline(ctx, cfg.Namespace, content); err != nil {
		logx.Errorf("同步 Tekton Pipeline 失败: namespace=%s name=%s err=%v", cfg.Namespace, pipelineName, err)
		return "", err
	}
	data.JobName = pipelineName
	data.JobFullName = cfg.Namespace + "/" + pipelineName
	data.TektonNamespace = cfg.Namespace
	data.TektonPipelineName = pipelineName
	data.TektonPipelineYamlSnapshot = content
	return content, nil
}

func deleteTektonPipeline(ctx context.Context, data *model.DevopsPipeline, runtime *pipelineconfigservice.ResolvePipelineRuntimeResp) error {
	if runtime == nil || runtime.Binding == nil || runtime.Channel == nil {
		return errorx.Msg("Tekton 运行时配置不完整")
	}
	cfg, err := parseTektonBindingConfig(runtime.Binding.BindingConfig)
	if err != nil {
		return err
	}
	client, err := devopstekton.NewClient(tektonRequestFromRuntime(runtime))
	if err != nil {
		logx.Errorf("创建 Tekton 客户端失败: %v", err)
		return err
	}
	pipelineName := strings.TrimSpace(data.TektonPipelineName)
	if pipelineName == "" {
		pipelineName = strings.TrimSpace(data.JobName)
	}
	if pipelineName == "" {
		pipelineName = tektonPipelineName(data.Code)
	}
	if err := client.DeletePipeline(ctx, cfg.Namespace, pipelineName); err != nil {
		logx.Errorf("删除 Tekton Pipeline 失败: namespace=%s name=%s err=%v", cfg.Namespace, pipelineName, err)
		return err
	}
	return nil
}

func triggerTektonPipelineRun(ctx context.Context, svcCtx *svc.ServiceContext, run *model.DevopsPipelineRun, pipeline *model.DevopsPipeline, runtime *pipelineconfigservice.ResolvePipelineRuntimeResp, params, workspaceOverrides map[string]string, operator string, userID uint64, roles []string) error {
	tektonParams, err := pipelineBuildParamsMap(ctx, svcCtx, pipeline, params, userID, roles)
	if err != nil {
		return err
	}
	workspaceParams := mergeRuntimeWorkspaceValues(tektonParams, workspaceOverrides)
	pipelineYAML, err := syncTektonPipeline(ctx, svcCtx, pipeline, runtime, workspaceParams, pipelineRenderContext{UserID: userID, Roles: roles, Params: params})
	if err != nil {
		return err
	}
	cfg, err := parseTektonBindingConfig(runtime.Binding.BindingConfig)
	if err != nil {
		return err
	}
	client, err := devopstekton.NewClient(tektonRequestFromRuntime(runtime))
	if err != nil {
		logx.Errorf("创建 Tekton 客户端失败: %v", err)
		return err
	}
	pipelineName := strings.TrimSpace(pipeline.TektonPipelineName)
	if pipelineName == "" {
		pipelineName = strings.TrimSpace(pipeline.JobName)
	}
	if pipelineName == "" {
		pipelineName = tektonPipelineName(pipeline.Code)
	}
	if run.BuildID <= 0 {
		buildID, err := nextTektonPipelineRunBuildID(ctx, svcCtx, cfg.Namespace, pipeline.SystemCode, pipeline.Code)
		if err != nil {
			return err
		}
		run.BuildID = buildID
	}
	runName, err := tektonPipelineRunName(pipeline.SystemCode, pipeline.Code, run.BuildID)
	if err != nil {
		return err
	}
	content, err := renderTektonPipelineRunYAML(runName, pipelineName, cfg, pipeline, tektonParams, workspaceOverrides, run)
	if err != nil {
		return err
	}
	applyTektonRunSnapshot(run, runtime, cfg, pipelineName, runName, pipelineYAML, content, operator)
	if err := svcCtx.PipelineRunModel.UpdateRuntimeSnapshot(ctx, run); err != nil {
		return err
	}
	if _, err := client.CreatePipelineRun(ctx, cfg.Namespace, content); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return err
		}
		buildID, buildErr := nextTektonPipelineRunBuildID(ctx, svcCtx, cfg.Namespace, pipeline.SystemCode, pipeline.Code)
		if buildErr != nil {
			return buildErr
		}
		run.BuildID = buildID
		runName, err = tektonPipelineRunName(pipeline.SystemCode, pipeline.Code, run.BuildID)
		if err != nil {
			return err
		}
		content, err = renderTektonPipelineRunYAML(runName, pipelineName, cfg, pipeline, tektonParams, workspaceOverrides, run)
		if err != nil {
			return err
		}
		applyTektonRunSnapshot(run, runtime, cfg, pipelineName, runName, pipelineYAML, content, operator)
		if updateErr := svcCtx.PipelineRunModel.UpdateRuntimeSnapshot(ctx, run); updateErr != nil {
			return updateErr
		}
		if _, err = client.CreatePipelineRun(ctx, cfg.Namespace, content); err != nil {
			return err
		}
	}
	run.Status = "running"
	run.UpdatedBy = operator
	if err := svcCtx.PipelineRunModel.Update(ctx, run); err != nil {
		if deleteErr := client.DeletePipelineRun(ctx, cfg.Namespace, runName); deleteErr != nil {
			logx.Errorf("Tekton PipelineRun 本地状态更新失败后远端清理失败: namespace=%s name=%s err=%v", cfg.Namespace, runName, deleteErr)
		}
		return err
	}
	_ = svcCtx.PipelineModel.UpdateRunStatus(ctx, pipeline.ID.Hex(), "running", operator)
	return nil
}

func applyTektonRunSnapshot(run *model.DevopsPipelineRun, runtime *pipelineconfigservice.ResolvePipelineRuntimeResp, cfg tektonBindingConfig, pipelineName, runName, pipelineYAML, content, operator string) {
	run.ChannelID = runtime.Channel.Id
	run.ChannelName = runtime.Channel.Name
	run.JenkinsJobName = pipelineName
	run.JenkinsJobFullName = cfg.Namespace + "/" + pipelineName
	run.TektonNamespace = cfg.Namespace
	run.TektonPipelineName = pipelineName
	run.TektonPipelineRunName = runName
	run.EnvSnapshot = tektonRunEnvSnapshot(run.EnvSnapshot, run.ID.Hex(), runName, run.BuildID)
	run.PipelineSnapshot = content
	run.TektonPipelineYamlSnapshot = pipelineYAML
	run.TektonPipelineRunYamlSnapshot = content
	run.UpdatedBy = operator
}

func nextTektonPipelineRunBuildID(ctx context.Context, svcCtx *svc.ServiceContext, namespace, systemCode, pipelineCode string) (int64, error) {
	namespace = strings.TrimSpace(namespace)
	systemCode = strings.TrimSpace(systemCode)
	pipelineCode = strings.TrimSpace(pipelineCode)
	if namespace == "" {
		return 0, errorx.Msg("Tekton namespace 不能为空")
	}
	if systemCode == "" {
		return 0, errorx.Msg("应用编码不能为空")
	}
	if pipelineCode == "" {
		return 0, errorx.Msg("流水线编码不能为空")
	}
	return svcCtx.RunCounterModel.NextTektonBuildID(ctx, namespace, tektonResourceName(systemCode), tektonResourceName(pipelineCode))
}

func tektonRunEnvSnapshot(raw, runID, runName string, buildID int64) string {
	env := map[string]string{}
	if strings.TrimSpace(raw) != "" {
		_ = json.Unmarshal([]byte(raw), &env)
	}
	if buildID > 0 {
		value := strconv.FormatInt(buildID, 10)
		env["BUILD_ID"] = value
		env["buildId"] = value
		env["KUBE_NOVA_BUILD_ID"] = value
	}
	if strings.TrimSpace(runID) != "" {
		env["KUBE_NOVA_RUN_ID"] = strings.TrimSpace(runID)
	}
	if strings.TrimSpace(runName) != "" {
		env["KUBE_NOVA_PIPELINE_RUN_NAME"] = strings.TrimSpace(runName)
	}
	out, _ := json.Marshal(env)
	return string(out)
}

func tektonPipelineTasks(ctx context.Context, client *devopstekton.Client, data *model.DevopsPipeline, taskNamespace string, runtimeParams map[string]string) ([]map[string]any, error) {
	result := make([]map[string]any, 0, len(data.Steps))
	paramsByNode := pipelineParamsByNode(data.Params)
	previousTaskName := ""
	for _, step := range orderedTektonPipelineSteps(data.Steps) {
		if !step.Enabled || strings.TrimSpace(step.ID) == "" {
			continue
		}
		resource, err := devopstekton.ParseStepResource(step.StageContent)
		if err != nil {
			logx.Errorf("解析 Tekton 步骤资源失败: step=%s err=%v", step.StepCode, err)
			return nil, err
		}
		resource.Namespace = taskNamespace
		if client != nil {
			if err := client.ValidateStepSynced(ctx, taskNamespace, resource); err != nil {
				logx.Errorf("Tekton 步骤未同步到目标实例: step=%s namespace=%s task=%s err=%v", step.StepCode, taskNamespace, resource.Name, err)
				return nil, errorx.Msg(err.Error())
			}
		}
		taskName := tektonPipelineTaskName(step)
		task := map[string]any{
			"name": taskName,
			"taskRef": map[string]any{
				"resolver": "cluster",
				"params": []map[string]any{
					{"name": "kind", "value": "task"},
					{"name": "name", "value": resource.Name},
					{"name": "namespace", "value": resource.Namespace},
				},
			},
		}
		if previousTaskName != "" {
			task["runAfter"] = []string{previousTaskName}
		}
		taskParams := tektonTaskParams(paramsByNode[step.ID])
		if len(taskParams) > 0 {
			task["params"] = taskParams
		}
		workspaces := tektonTaskWorkspaces(resource.Object, taskName, data, paramsByNode[step.ID], runtimeParams)
		if len(workspaces) > 0 {
			task["workspaces"] = workspaces
		}
		result = append(result, task)
		previousTaskName = taskName
	}
	if len(result) == 0 {
		return nil, errorx.Msg("Tekton 流水线至少需要一个启用步骤")
	}
	return result, nil
}

func renderTektonPipelineYAML(name string, cfg tektonBindingConfig, data *model.DevopsPipeline, tasks []map[string]any) (string, error) {
	spec := map[string]any{
		"tasks": tasks,
	}
	params, err := tektonPipelineParams(data.Params)
	if err != nil {
		return "", err
	}
	if len(params) > 0 {
		spec["params"] = params
	}
	if workspaces := tektonPipelineWorkspaceDeclarations(tasks); len(workspaces) > 0 {
		spec["workspaces"] = workspaces
	}
	stripTektonWorkspaceInternalFields(tasks)
	obj := map[string]any{
		"apiVersion": "tekton.dev/v1",
		"kind":       "Pipeline",
		"metadata": map[string]any{
			"name":      name,
			"namespace": cfg.Namespace,
			"labels": map[string]any{
				"kube-nova.io/pipeline-id": data.ID.Hex(),
				"kube-nova.io/engine":      engineTekton,
			},
		},
		"spec": spec,
	}
	out, err := yaml.Marshal(obj)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func renderTektonPipelineRunYAML(runName, pipelineName string, cfg tektonBindingConfig, pipeline *model.DevopsPipeline, params, workspaceOverrides map[string]string, run *model.DevopsPipelineRun) (string, error) {
	spec := map[string]any{
		"pipelineRef": map[string]any{"name": pipelineName},
	}
	runParams, err := tektonPipelineRunParams(pipeline, params)
	if err != nil {
		return "", err
	}
	if len(runParams) > 0 {
		spec["params"] = runParams
	}
	hasDagConfig := hasTektonDagConfig(pipeline.TektonDagConfig)
	var dagRunPolicy *tektonDagRunPolicy
	if hasDagConfig {
		policy, err := tektonRunPolicyFromPipeline(pipeline)
		if err != nil {
			return "", err
		}
		dagRunPolicy = &policy
		if err := applyTektonDagPipelineRunSpec(spec, pipeline, cfg, policy); err != nil {
			return "", err
		}
	} else if strings.TrimSpace(cfg.ServiceAccountName) != "" {
		applyTektonPipelineRunServiceAccount(spec, cfg.ServiceAccountName)
	}
	var workspaces []map[string]any
	workspaceParams := mergeRuntimeWorkspaceValues(params, workspaceOverrides)
	if hasDagConfig {
		workspaces, err = tektonDagPipelineRunWorkspaces(pipeline, workspaceParams)
	} else {
		workspaces, err = tektonPipelineRunWorkspaces(pipeline, workspaceParams)
	}
	if err != nil {
		return "", err
	}
	if len(workspaces) > 0 {
		spec["workspaces"] = workspaces
	}
	applyTektonBuildIDPodEnv(spec, run, runName)
	metadata := map[string]any{
		"name":      runName,
		"namespace": cfg.Namespace,
		"labels": map[string]any{
			"kube-nova.io/pipeline-id": pipeline.ID.Hex(),
			"kube-nova.io/engine":      engineTekton,
			"kube-nova.io/build-id":    strconv.FormatInt(run.BuildID, 10),
		},
	}
	if dagRunPolicy != nil {
		mergeTektonNameValueMap(metadata["labels"].(map[string]any), dagRunPolicy.Labels)
		annotations := map[string]any{}
		mergeTektonNameValueMap(annotations, dagRunPolicy.Annotations)
		if len(annotations) > 0 {
			metadata["annotations"] = annotations
		}
	}
	obj := map[string]any{
		"apiVersion": "tekton.dev/v1",
		"kind":       "PipelineRun",
		"metadata":   metadata,
		"spec":       spec,
	}
	out, err := yaml.Marshal(obj)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func applyTektonBuildIDPodEnv(spec map[string]any, run *model.DevopsPipelineRun, runName string) {
	if spec == nil || run == nil || run.BuildID <= 0 {
		return
	}
	buildID := strconv.FormatInt(run.BuildID, 10)
	env := []map[string]any{
		{"name": "BUILD_ID", "value": buildID},
		{"name": "buildId", "value": buildID},
		{"name": "KUBE_NOVA_BUILD_ID", "value": buildID},
		{"name": "KUBE_NOVA_RUN_ID", "value": run.ID.Hex()},
		{"name": "KUBE_NOVA_PIPELINE_RUN_NAME", "value": runName},
	}
	podTemplate := ensureTektonPipelineRunPodTemplate(spec)
	podTemplate["env"] = appendTektonPodEnv(podTemplate["env"], env)
}

func ensureTektonPipelineRunTaskRunTemplate(spec map[string]any) map[string]any {
	taskRunTemplate, ok := spec["taskRunTemplate"].(map[string]any)
	if !ok || taskRunTemplate == nil {
		taskRunTemplate = map[string]any{}
		spec["taskRunTemplate"] = taskRunTemplate
	}
	return taskRunTemplate
}

func ensureTektonPipelineRunPodTemplate(spec map[string]any) map[string]any {
	taskRunTemplate := ensureTektonPipelineRunTaskRunTemplate(spec)
	podTemplate, ok := taskRunTemplate["podTemplate"].(map[string]any)
	if !ok || podTemplate == nil {
		podTemplate = map[string]any{}
		taskRunTemplate["podTemplate"] = podTemplate
	}
	return podTemplate
}

func applyTektonPipelineRunServiceAccount(spec map[string]any, serviceAccountName string) {
	serviceAccountName = strings.TrimSpace(serviceAccountName)
	if serviceAccountName == "" {
		return
	}
	taskRunTemplate := ensureTektonPipelineRunTaskRunTemplate(spec)
	taskRunTemplate["serviceAccountName"] = serviceAccountName
}

func mergeTektonPipelineRunPodTemplate(spec map[string]any, podTemplate map[string]any) {
	if len(podTemplate) == 0 {
		return
	}
	target := ensureTektonPipelineRunPodTemplate(spec)
	for key, value := range podTemplate {
		target[key] = value
	}
}

func appendTektonPodEnv(raw any, additions []map[string]any) []map[string]any {
	result := make([]map[string]any, 0, len(additions))
	override := make(map[string]struct{}, len(additions))
	seen := make(map[string]struct{}, len(additions))
	for _, item := range additions {
		name := strings.TrimSpace(fmt.Sprint(item["name"]))
		if name != "" {
			override[name] = struct{}{}
		}
	}
	for _, item := range anySlice(raw) {
		envItem, ok := item.(map[string]any)
		if !ok {
			continue
		}
		name := strings.TrimSpace(fmt.Sprint(envItem["name"]))
		if name == "" {
			continue
		}
		if _, ok := override[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		result = append(result, envItem)
	}
	for _, item := range additions {
		name := strings.TrimSpace(fmt.Sprint(item["name"]))
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		result = append(result, item)
	}
	return result
}

func mergeTektonNameValueMap(target map[string]any, source []tektonDagNameValue) {
	for _, item := range source {
		key := strings.TrimSpace(item.Name)
		value := strings.TrimSpace(fmt.Sprint(item.Value))
		if key == "" || value == "" {
			continue
		}
		target[key] = value
	}
}

func tektonRequestFromRuntime(runtime *pipelineconfigservice.ResolvePipelineRuntimeResp) devopstypes.Request {
	req := devopstypes.Request{
		Channel: devopstypes.Channel{
			ID:              runtime.Channel.Id,
			Name:            runtime.Channel.Name,
			Code:            runtime.Channel.Code,
			Type:            runtime.Channel.ChannelType,
			Endpoint:        runtime.Channel.Endpoint,
			Config:          runtime.Channel.Config,
			AuthType:        runtime.Channel.AuthType,
			Username:        runtime.Channel.Username,
			Password:        runtime.Channel.Password,
			Token:           runtime.Channel.Token,
			InsecureSkipTLS: runtime.Channel.InsecureSkipTls,
		},
	}
	if runtime.Credential != nil {
		req.Credential = &devopstypes.Credential{
			Type:        runtime.Credential.CredentialType,
			Username:    runtime.Credential.Username,
			Password:    runtime.Credential.Password,
			Token:       runtime.Credential.Token,
			PrivateKey:  runtime.Credential.PrivateKey,
			Passphrase:  runtime.Credential.Passphrase,
			Kubeconfig:  runtime.Credential.Kubeconfig,
			SecretText:  runtime.Credential.SecretText,
			Certificate: runtime.Credential.Certificate,
			JsonData:    runtime.Credential.JsonData,
		}
	}
	return req
}

func parseTektonBindingConfig(content string) (tektonBindingConfig, error) {
	var cfg tektonBindingConfig
	if err := json.Unmarshal([]byte(strings.TrimSpace(content)), &cfg); err != nil {
		logx.Errorf("解析 Tekton 绑定配置失败: %v", err)
		return cfg, errorx.Msg("Tekton 绑定配置错误")
	}
	cfg.Namespace = strings.TrimSpace(cfg.Namespace)
	cfg.ServiceAccountName = strings.TrimSpace(cfg.ServiceAccountName)
	cfg.PipelinePrefix = strings.TrimSpace(cfg.PipelinePrefix)
	if cfg.Namespace == "" {
		return cfg, errorx.Msg("Tekton namespace 不能为空")
	}
	return cfg, nil
}

func parseTektonChannelTaskConfig(content string) (devopstekton.ChannelConfig, error) {
	return devopstekton.ParseChannelConfig(content)
}

func renderTektonPipelineContent(ctx context.Context, client *devopstekton.Client, data *model.DevopsPipeline, cfg tektonBindingConfig, taskNamespace string, runtimeParams map[string]string) (string, error) {
	pipelineName := tektonPipelineName(data.Code)
	if hasTektonDagConfig(data.TektonDagConfig) {
		if err := validateTektonDagTaskRefReality(ctx, client, data, cfg.Namespace); err != nil {
			return "", err
		}
		return renderTektonDagPipelineYAML(pipelineName, cfg, data, runtimeParams)
	}
	tasks, err := tektonPipelineTasks(ctx, client, data, taskNamespace, runtimeParams)
	if err != nil {
		return "", err
	}
	return renderTektonPipelineYAML(pipelineName, cfg, data, tasks)
}

func pipelineParamsByNode(params []model.PipelineParam) map[string][]model.PipelineParam {
	result := make(map[string][]model.PipelineParam)
	for _, item := range params {
		nodeID := strings.TrimSpace(item.StepNodeID)
		if nodeID == "" {
			continue
		}
		result[nodeID] = append(result[nodeID], item)
	}
	return result
}

func tektonPipelineParams(params []model.PipelineParam) ([]map[string]any, error) {
	result := make([]map[string]any, 0, len(params))
	seen := make(map[string]struct{})
	for _, item := range params {
		code := strings.TrimSpace(item.Code)
		if code == "" {
			continue
		}
		if _, ok := seen[code]; ok {
			continue
		}
		seen[code] = struct{}{}
		paramType := tektonParamType(item.ParamType)
		param := map[string]any{"name": code, "type": paramType}
		if value := firstNotBlank(item.CurrentValue, item.DefaultValue); value != "" {
			defaultValue, err := tektonParamDefaultValue(paramType, value, displayPipelineParamName(item))
			if err != nil {
				return nil, err
			}
			param["default"] = defaultValue
		}
		result = append(result, param)
	}
	return result, nil
}

func tektonParamType(paramType string) string {
	switch strings.TrimSpace(paramType) {
	case "array", "list":
		return "array"
	case "object":
		return "object"
	default:
		return "string"
	}
}

func tektonParamDefaultValue(paramType string, raw any, name string) (any, error) {
	switch strings.TrimSpace(paramType) {
	case "array":
		switch data := raw.(type) {
		case []any:
			if !tektonParamAllStrings(data) {
				return nil, errorx.Msg("Tekton 参数默认值不是合法 string array：" + name)
			}
			return data, nil
		case []string:
			result := make([]any, 0, len(data))
			for _, item := range data {
				result = append(result, item)
			}
			return result, nil
		}
		value := strings.TrimSpace(fmt.Sprint(raw))
		if value == "" {
			return []any{}, nil
		}
		var parsed any
		_ = yaml.Unmarshal([]byte(value), &parsed)
		if list, ok := parsed.([]any); ok {
			if !tektonParamAllStrings(list) {
				return nil, errorx.Msg("Tekton 参数默认值不是合法 string array：" + name)
			}
			return list, nil
		}
		parts := strings.Split(value, ",")
		result := make([]any, 0, len(parts))
		for _, part := range parts {
			if text := strings.TrimSpace(part); text != "" {
				result = append(result, text)
			}
		}
		if len(result) > 0 {
			return result, nil
		}
		return nil, errorx.Msg("Tekton 参数默认值不是合法 array：" + name)
	case "object":
		switch data := raw.(type) {
		case map[string]any:
			return data, nil
		case map[string]string:
			result := make(map[string]any, len(data))
			for key, value := range data {
				result[key] = value
			}
			return result, nil
		}
		value := strings.TrimSpace(fmt.Sprint(raw))
		if value == "" {
			return map[string]any{}, nil
		}
		var parsed any
		if err := yaml.Unmarshal([]byte(value), &parsed); err != nil {
			return nil, errorx.Msg("Tekton 参数默认值不是合法 object：" + name)
		}
		if object, ok := parsed.(map[string]any); ok {
			return object, nil
		}
		return nil, errorx.Msg("Tekton 参数默认值不是合法 object：" + name)
	default:
		return strings.TrimSpace(fmt.Sprint(raw)), nil
	}
}

func tektonParamAllStrings(items []any) bool {
	for _, item := range items {
		if _, ok := item.(string); !ok {
			return false
		}
	}
	return true
}

func tektonTaskParams(params []model.PipelineParam) []map[string]any {
	result := make([]map[string]any, 0, len(params))
	for _, item := range params {
		sourceCode := pipelineParamSourceCode(item)
		code := strings.TrimSpace(item.Code)
		if sourceCode == "" || code == "" {
			continue
		}
		if isTektonWorkspaceBindingParam(item) {
			continue
		}
		result = append(result, map[string]any{
			"name":  sourceCode,
			"value": fmt.Sprintf("$(params.%s)", code),
		})
	}
	return result
}

func tektonTaskWorkspaces(obj *unstructured.Unstructured, taskName string, pipeline *model.DevopsPipeline, params []model.PipelineParam, runtimeParams map[string]string) []map[string]any {
	items, ok, err := unstructured.NestedSlice(obj.Object, "spec", "workspaces")
	if err != nil || !ok || len(items) == 0 {
		return nil
	}
	result := make([]map[string]any, 0, len(items))
	workspaceBindings := tektonWorkspaceParams(taskName, params)
	for _, item := range items {
		data, ok := item.(map[string]any)
		if !ok {
			continue
		}
		name := strings.TrimSpace(fmt.Sprint(data["name"]))
		if name == "" {
			continue
		}
		if workspace := strings.TrimSpace(workspaceBindings[name]); workspace != "" {
			optional, _ := data["optional"].(bool)
			result = append(result, map[string]any{"name": name, "workspace": workspace, "_optional": optional})
			continue
		}
		if optional, _ := data["optional"].(bool); optional {
			continue
		}
		result = append(result, map[string]any{"name": name, "workspace": tektonDefaultPipelineWorkspaceName(taskName, name), "_optional": false})
	}
	return result
}

func tektonDefaultPipelineWorkspaceName(taskName, workspaceName string) string {
	name := strings.TrimSpace(workspaceName)
	if name == "source" || name == "output" {
		return "source"
	}
	return tektonResourceName("workspace", taskName, name)
}

func tektonWorkspaceParams(taskName string, params []model.PipelineParam) map[string]string {
	result := make(map[string]string)
	for _, item := range params {
		config := normalizePipelineParamConfig(item)
		workspaceName := strings.TrimSpace(config.MappingField)
		if workspaceName == "" {
			continue
		}
		switch tektonWorkspaceBindingKind(item, config) {
		case "secret":
			result[workspaceName] = tektonSecretWorkspaceName(taskName, workspaceName, item.Code)
		case "persistentVolumeClaim":
			result[workspaceName] = tektonPVCWorkspaceName(taskName, workspaceName, item.Code)
		case "volumeClaimTemplate":
			result[workspaceName] = tektonVolumeClaimTemplateWorkspaceName(taskName, workspaceName)
		}
	}
	return result
}

func isTektonWorkspaceBindingParam(item model.PipelineParam) bool {
	config := normalizePipelineParamConfig(item)
	return tektonWorkspaceBindingKind(item, config) != ""
}

func tektonWorkspaceBindingKind(item model.PipelineParam, config model.StepParamConfig) string {
	if strings.TrimSpace(config.MappingField) == "" {
		return ""
	}
	switch item.ParamType {
	case channelvars.ParamKubernetesSecretName:
		return "secret"
	case channelvars.ParamKubernetesPVCName:
		return "persistentVolumeClaim"
	}
	if strings.HasPrefix(strings.TrimSpace(config.Provider), "volumeClaimTemplate.") {
		return "volumeClaimTemplate"
	}
	return ""
}

func tektonSecretWorkspaceValue(pipelineWorkspaceName, taskName string, pipeline *model.DevopsPipeline, params []model.PipelineParam, runtimeParams map[string]string) string {
	for _, item := range params {
		config := normalizePipelineParamConfig(item)
		if tektonWorkspaceBindingKind(item, config) != "secret" {
			continue
		}
		workspaceName := strings.TrimSpace(config.MappingField)
		if workspaceName == "" {
			continue
		}
		if tektonSecretWorkspaceName(taskName, workspaceName, item.Code) != pipelineWorkspaceName {
			continue
		}
		return tektonWorkspaceSecretName(pipeline, item, runtimeParams)
	}
	return ""
}

func tektonPVCWorkspaceValue(pipelineWorkspaceName, taskName string, params []model.PipelineParam, runtimeParams map[string]string) string {
	for _, item := range params {
		config := normalizePipelineParamConfig(item)
		if tektonWorkspaceBindingKind(item, config) != "persistentVolumeClaim" {
			continue
		}
		workspaceName := strings.TrimSpace(config.MappingField)
		if workspaceName == "" {
			continue
		}
		if tektonPVCWorkspaceName(taskName, workspaceName, item.Code) != pipelineWorkspaceName {
			continue
		}
		return tektonPipelineParamValue(item, runtimeParams)
	}
	return ""
}

func tektonVolumeClaimTemplateWorkspaceValue(pipelineWorkspaceName, taskName string, params []model.PipelineParam, runtimeParams map[string]string, optional bool) (map[string]any, error) {
	values := map[string]string{}
	hasBinding := false
	for _, item := range params {
		config := normalizePipelineParamConfig(item)
		if tektonWorkspaceBindingKind(item, config) != "volumeClaimTemplate" {
			continue
		}
		if tektonVolumeClaimTemplateWorkspaceName(taskName, config.MappingField) != pipelineWorkspaceName {
			continue
		}
		hasBinding = true
		field, ok := tektonVolumeClaimTemplateField(config.Provider)
		if !ok {
			continue
		}
		if value := strings.TrimSpace(tektonPipelineParamValue(item, runtimeParams)); value != "" {
			values[field] = value
		}
	}
	if !hasBinding {
		return nil, nil
	}
	storage := strings.TrimSpace(values["storage"])
	if storage == "" {
		if optional {
			return nil, nil
		}
		return nil, errorx.Msg("动态 PVC 存储容量不能为空")
	}
	if _, err := resource.ParseQuantity(storage); err != nil {
		logx.Errorf("动态 PVC 存储容量不合法: storage=%s err=%v", storage, err)
		return nil, errorx.Msg("动态 PVC 存储容量不合法")
	}
	accessMode := firstNotBlank(values["accessMode"], "ReadWriteOnce")
	if !isSupportedTektonAccessMode(accessMode) {
		return nil, errorx.Msg("动态 PVC accessMode 不支持")
	}
	volumeMode := strings.TrimSpace(values["volumeMode"])
	if volumeMode != "" && volumeMode != "Filesystem" && volumeMode != "Block" {
		return nil, errorx.Msg("动态 PVC volumeMode 不支持")
	}
	spec := map[string]any{
		"accessModes": []string{accessMode},
		"resources": map[string]any{
			"requests": map[string]any{"storage": storage},
		},
	}
	if storageClassName := strings.TrimSpace(values["storageClassName"]); storageClassName != "" {
		spec["storageClassName"] = storageClassName
	}
	if volumeMode != "" {
		spec["volumeMode"] = volumeMode
	}
	return map[string]any{
		"name": pipelineWorkspaceName,
		"volumeClaimTemplate": map[string]any{
			"spec": spec,
		},
	}, nil
}

func tektonVolumeClaimTemplateField(provider string) (string, bool) {
	field := strings.TrimPrefix(strings.TrimSpace(provider), "volumeClaimTemplate.")
	switch field {
	case "storage", "storageClassName", "accessMode", "volumeMode":
		return field, true
	default:
		return "", false
	}
}

func isSupportedTektonAccessMode(value string) bool {
	switch strings.TrimSpace(value) {
	case "ReadWriteOnce", "ReadOnlyMany", "ReadWriteMany", "ReadWriteOncePod":
		return true
	default:
		return false
	}
}

func tektonPipelineParamValue(item model.PipelineParam, runtimeParams map[string]string) string {
	code := strings.TrimSpace(item.Code)
	if runtimeParams != nil {
		if value, ok := runtimeParams[code]; ok {
			return strings.TrimSpace(value)
		}
	}
	return firstNotBlank(item.CurrentValue, item.DefaultValue)
}

func tektonSecretWorkspaceName(taskName, workspaceName, paramCode string) string {
	return tektonResourceName("secret", taskName, workspaceName, paramCode)
}

func tektonPVCWorkspaceName(taskName, workspaceName, paramCode string) string {
	return tektonResourceName("pvc", taskName, workspaceName, paramCode)
}

func tektonVolumeClaimTemplateWorkspaceName(taskName, workspaceName string) string {
	return tektonResourceName("claim", taskName, workspaceName)
}

func tektonPipelineWorkspaceBindingKind(pipelineWorkspaceName, taskName string, params []model.PipelineParam) string {
	name := strings.TrimSpace(pipelineWorkspaceName)
	for _, item := range params {
		config := normalizePipelineParamConfig(item)
		workspaceName := strings.TrimSpace(config.MappingField)
		if workspaceName == "" {
			continue
		}
		switch tektonWorkspaceBindingKind(item, config) {
		case "secret":
			if tektonSecretWorkspaceName(taskName, workspaceName, item.Code) == name {
				return "secret"
			}
		case "persistentVolumeClaim":
			if tektonPVCWorkspaceName(taskName, workspaceName, item.Code) == name {
				return "persistentVolumeClaim"
			}
		case "volumeClaimTemplate":
			if tektonVolumeClaimTemplateWorkspaceName(taskName, workspaceName) == name {
				return "volumeClaimTemplate"
			}
		}
	}
	return ""
}

func tektonPipelineWorkspaceDeclarations(tasks []map[string]any) []map[string]any {
	seen := make(map[string]struct{})
	optionalByName := make(map[string]bool)
	names := make([]string, 0)
	for _, item := range tasks {
		workspaces, ok := item["workspaces"].([]map[string]any)
		if !ok {
			continue
		}
		for _, workspace := range workspaces {
			name := strings.TrimSpace(fmt.Sprint(workspace["workspace"]))
			if name == "" {
				continue
			}
			optional, _ := workspace["_optional"].(bool)
			if _, ok := optionalByName[name]; !ok {
				optionalByName[name] = optional
			} else {
				optionalByName[name] = optionalByName[name] && optional
			}
			if _, ok := seen[name]; ok {
				continue
			}
			seen[name] = struct{}{}
			names = append(names, name)
		}
	}
	result := make([]map[string]any, 0, len(names))
	for _, name := range names {
		workspace := map[string]any{"name": name}
		if optionalByName[name] {
			workspace["optional"] = true
		}
		result = append(result, workspace)
	}
	return result
}

func stripTektonWorkspaceInternalFields(tasks []map[string]any) {
	for _, item := range tasks {
		workspaces, ok := item["workspaces"].([]map[string]any)
		if !ok {
			continue
		}
		for _, workspace := range workspaces {
			delete(workspace, "_optional")
		}
	}
}

func tektonPipelineRunWorkspaces(pipeline *model.DevopsPipeline, runtimeParams map[string]string) ([]map[string]any, error) {
	result := make([]map[string]any, 0)
	seen := make(map[string]struct{})
	paramsByNode := pipelineParamsByNode(pipeline.Params)
	for _, step := range pipeline.Steps {
		if !step.Enabled || strings.TrimSpace(step.ID) == "" {
			continue
		}
		resource, err := devopstekton.ParseStepResource(step.StageContent)
		if err != nil {
			continue
		}
		taskName := tektonPipelineTaskName(step)
		for _, workspace := range tektonTaskWorkspaces(resource.Object, taskName, pipeline, paramsByNode[step.ID], runtimeParams) {
			name := strings.TrimSpace(fmt.Sprint(workspace["workspace"]))
			if name == "" {
				continue
			}
			if _, ok := seen[name]; ok {
				continue
			}
			if name == "source" {
				seen[name] = struct{}{}
				result = append(result, map[string]any{"name": name, "emptyDir": map[string]any{}})
				continue
			}
			optional, _ := workspace["_optional"].(bool)
			if secretName := tektonSecretWorkspaceValue(name, taskName, pipeline, paramsByNode[step.ID], runtimeParams); secretName != "" {
				seen[name] = struct{}{}
				result = append(result, map[string]any{"name": name, "secret": map[string]any{"secretName": secretName}})
				continue
			}
			if pvcName := tektonPVCWorkspaceValue(name, taskName, paramsByNode[step.ID], runtimeParams); pvcName != "" {
				seen[name] = struct{}{}
				result = append(result, map[string]any{"name": name, "persistentVolumeClaim": map[string]any{"claimName": pvcName}})
				continue
			}
			bindingKind := tektonPipelineWorkspaceBindingKind(name, taskName, paramsByNode[step.ID])
			if (bindingKind == "secret" || bindingKind == "persistentVolumeClaim") && !optional {
				return nil, errorx.Msg("必填 Workspace 绑定值不能为空")
			}
			if bindingKind != "" && optional {
				continue
			}
			workspace, err := tektonVolumeClaimTemplateWorkspaceValue(name, taskName, paramsByNode[step.ID], runtimeParams, optional)
			if err != nil {
				return nil, err
			}
			if workspace != nil {
				seen[name] = struct{}{}
				result = append(result, workspace)
				continue
			}
			if optional {
				continue
			}
			seen[name] = struct{}{}
			result = append(result, map[string]any{"name": name, "emptyDir": map[string]any{}})
		}
	}
	return result, nil
}

func orderedTektonPipelineSteps(steps []model.PipelineStep) []model.PipelineStep {
	result := append([]model.PipelineStep(nil), steps...)
	sort.SliceStable(result, func(i, j int) bool {
		left := result[i].SortOrder
		right := result[j].SortOrder
		if left <= 0 {
			left = int64(i + 1)
		}
		if right <= 0 {
			right = int64(j + 1)
		}
		return left < right
	})
	return result
}

func tektonPipelineTaskName(step model.PipelineStep) string {
	return tektonResourceName(step.ID, step.StepCode)
}

func tektonPipelineName(pipelineCode string) string {
	return tektonResourceName(pipelineCode)
}

func tektonResourceName(values ...string) string {
	return devopstekton.ResourceName(values...)
}

func tektonPipelineRunName(systemCode, pipelineCode string, buildID int64) (string, error) {
	if buildID <= 0 {
		return "", errorx.Msg("Tekton buildId 不合法")
	}
	if strings.TrimSpace(systemCode) == "" {
		return "", errorx.Msg("应用编码不能为空")
	}
	if strings.TrimSpace(pipelineCode) == "" {
		return "", errorx.Msg("流水线编码不能为空")
	}
	suffix := strconv.FormatInt(buildID, 10)
	prefix := strings.Trim(strings.Join([]string{
		tektonResourceName(systemCode),
		tektonResourceName(pipelineCode),
	}, "-"), "-")
	maxPrefixLen := 63 - len(suffix) - 1
	if maxPrefixLen < 1 {
		maxPrefixLen = 1
	}
	if len(prefix) > maxPrefixLen {
		prefix = strings.Trim(prefix[:maxPrefixLen], "-")
	}
	if prefix == "" {
		prefix = "run"
	}
	return tektonResourceName(prefix, suffix), nil
}

func firstNotBlank(values ...string) string {
	for _, item := range values {
		if value := strings.TrimSpace(item); value != "" {
			return value
		}
	}
	return ""
}
