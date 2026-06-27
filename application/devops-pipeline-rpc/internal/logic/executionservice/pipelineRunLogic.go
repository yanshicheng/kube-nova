package executionservicelogic

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/client/pipelineconfigservice"
	"github.com/yanshicheng/kube-nova/application/devops-pipeline-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/devops-pipeline-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-pipeline-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/devops/channelvars"
	"github.com/yanshicheng/kube-nova/common/devops/jenkins"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type PipelineRunLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPipelineRunLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PipelineRunLogic {
	return &PipelineRunLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *PipelineRunLogic) PipelineRun(in *pb.RunPipelineReq) (*pb.IdResp, error) {
	pipeline, err := l.svcCtx.PipelineModel.FindOne(l.ctx, in.Id)
	if err != nil {
		l.Errorf("流水线运行失败: %v", err)
		return nil, err
	}
	if err := ensurePipelineAccess(l.ctx, l.svcCtx, pipeline, in.CurrentUserId, in.CurrentRoles); err != nil {
		l.Errorf("流水线运行失败: %v", err)
		return nil, err
	}
	if pipeline.Status != 1 {
		l.Errorf("流水线已停用，不能运行: pipelineId=%s", pipeline.ID.Hex())
		return nil, errorx.Msg("流水线已停用，不能运行")
	}
	params, err := pipelineParamsMap(pipeline.Params, in.Params)
	if err != nil {
		l.Errorf("流水线运行失败: %v", err)
		return nil, err
	}
	if err := validateRunRequiredParams(pipeline.Params, params); err != nil {
		l.Errorf("流水线运行失败: %v", err)
		return nil, err
	}
	workspaces, err := pipelineStringMap(in.Workspaces, "Workspace 运行配置")
	if err != nil {
		l.Errorf("流水线运行失败: %v", err)
		return nil, err
	}
	if pipeline.EngineType == engineTekton {
		if err := validateTektonPipelineRunCompleteness(pipeline, params, workspaces); err != nil {
			l.Errorf("流水线运行失败: %v", err)
			return nil, err
		}
	}
	if shouldValidateRunDynamicParams(pipeline) {
		if err := validateRunDynamicParams(l.ctx, l.svcCtx, pipeline, params, in.CurrentUserId, in.CurrentRoles); err != nil {
			l.Errorf("流水线运行失败: %v", err)
			return nil, err
		}
	}
	if pipeline.SyncStatus != "synced" {
		if strings.TrimSpace(pipeline.SyncMessage) != "" {
			l.Errorf("%s", pipeline.SyncMessage)
			return nil, errorx.Msg(pipeline.SyncMessage)
		}
		l.Errorf("流水线尚未同步目标引擎，请先保存流水线配置")
		return nil, errorx.Msg("流水线尚未同步目标引擎，请先保存流水线配置")
	}
	if pipeline.EngineType == engineTekton {
		policy, err := tektonRunPolicyFromPipeline(pipeline)
		if err != nil {
			l.Errorf("流水线运行失败: %v", err)
			return nil, err
		}
		if err := applyTektonRunConcurrencyPolicy(l.ctx, l.svcCtx, pipeline.ID.Hex(), policy.ConcurrencyPolicy, "", in.CurrentUsername, in.CurrentUserId, in.CurrentRoles); err != nil {
			l.Errorf("流水线运行失败: %v", err)
			return nil, err
		}
	}
	var buildID int64
	if pipeline.EngineType == engineTekton {
		runtime, runtimeErr := buildRuntimeCached(l.ctx, l.svcCtx, pipeline.ProjectID, pipeline.SystemID, pipeline.EnvironmentID, pipeline.BuildChannelBindingID, in.CurrentUserId, in.CurrentRoles)
		if runtimeErr != nil {
			l.Errorf("解析 Tekton 运行上下文失败: %v", runtimeErr)
			return nil, runtimeErr
		}
		if runtime == nil || runtime.Project == nil || runtime.System == nil || runtime.Environment == nil || runtime.Binding == nil || runtime.Channel == nil {
			l.Errorf("解析 Tekton 运行上下文失败: 流水线运行上下文不完整")
			return nil, errorx.Msg("流水线运行上下文不完整")
		}
		if runtime.Binding.ChannelType != engineTekton || runtime.Channel.ChannelType != engineTekton {
			l.Errorf("解析 Tekton 运行上下文失败: 构建渠道不是 Tekton 类型")
			return nil, errorx.Msg("构建渠道不是 Tekton 类型")
		}
		cfg, cfgErr := parseTektonBindingConfig(runtime.Binding.BindingConfig)
		if cfgErr != nil {
			l.Errorf("解析 Tekton 运行上下文失败: %v", cfgErr)
			return nil, cfgErr
		}
		pipeline.ProjectName = runtime.Project.Name
		pipeline.ProjectCode = runtime.Project.Code
		pipeline.SystemName = runtime.System.Name
		pipeline.SystemCode = runtime.System.Code
		pipeline.EnvironmentName = runtime.Environment.Name
		pipeline.EnvironmentCode = runtime.Environment.Code
		pipeline.BuildChannelName = runtime.Binding.ChannelName
		pipeline.TektonNamespace = cfg.Namespace
		buildID, err = nextTektonPipelineRunBuildID(l.ctx, l.svcCtx, cfg.Namespace, pipeline.SystemCode, pipeline.Code)
		if err != nil {
			l.Errorf("生成 Tekton buildId 失败: %v", err)
			return nil, err
		}
	}
	paramBytes, _ := json.Marshal(params)
	workspaceBytes, _ := json.Marshal(workspaces)
	env := map[string]string{
		"PROJECT_CODE":      pipeline.ProjectCode,
		"SYSTEM_CODE":       pipeline.SystemCode,
		"SYSTEM_NAME":       pipeline.SystemName,
		"PIPELINE_CODE":     pipeline.Code,
		"PIPELINE_NAME":     pipeline.Name,
		"PIPELINE_ENV":      pipeline.EnvironmentCode,
		"PIPELINE_ENV_NAME": pipeline.EnvironmentName,
	}
	if buildID > 0 {
		env["BUILD_ID"] = fmt.Sprint(buildID)
		env["KUBE_NOVA_BUILD_ID"] = fmt.Sprint(buildID)
	}
	envBytes, _ := json.Marshal(env)
	run := &model.DevopsPipelineRun{
		ProjectID:             pipeline.ProjectID,
		ProjectName:           pipeline.ProjectName,
		ProjectCode:           pipeline.ProjectCode,
		SystemID:              pipeline.SystemID,
		SystemName:            pipeline.SystemName,
		SystemCode:            pipeline.SystemCode,
		EnvironmentID:         pipeline.EnvironmentID,
		EnvironmentName:       pipeline.EnvironmentName,
		EnvironmentCode:       pipeline.EnvironmentCode,
		PipelineID:            pipeline.ID.Hex(),
		PipelineName:          pipeline.Name,
		PipelineCode:          pipeline.Code,
		EngineType:            pipeline.EngineType,
		TemplateID:            pipeline.TemplateID,
		BuildChannelBindingID: pipeline.BuildChannelBindingID,
		JenkinsJobName:        pipeline.JobName,
		JenkinsJobFullName:    pipeline.JobFullName,
		BuildID:               buildID,
		TektonNamespace:       pipeline.TektonNamespace,
		TektonPipelineName:    pipeline.TektonPipelineName,
		TriggerType:           in.TriggerType,
		TriggerUserID:         in.CurrentUserId,
		TriggerUsername:       in.CurrentUsername,
		Status:                "preparing",
		ParamsSnapshot:        string(paramBytes),
		WorkspaceSnapshot:     string(workspaceBytes),
		EnvSnapshot:           string(envBytes),
		StartedAt:             time.Now(),
		CreatedBy:             in.CurrentUsername,
		UpdatedBy:             in.CurrentUsername,
	}
	if run.TriggerType == "" {
		run.TriggerType = "manual"
	}
	if err := l.svcCtx.PipelineRunModel.Insert(l.ctx, run); err != nil {
		l.Errorf("流水线运行失败: %v", err)
		return nil, err
	}
	if err := createScanPlanSnapshot(l.ctx, l.svcCtx, run, pipeline, params, in.CurrentUsername); err != nil {
		markPipelineRunTriggerFailed(l.ctx, l.svcCtx, run, pipeline.ID.Hex(), "创建扫描计划快照失败："+err.Error(), in.CurrentUsername)
		return nil, err
	}
	if err := createRunStagesForPipeline(l.ctx, l.svcCtx, run.ID.Hex(), pipeline, nil); err != nil {
		l.Errorf("流水线运行失败: %v", err)
		return nil, err
	}
	pipelineForRun := *pipeline
	schedulePipelineRunTrigger(l.svcCtx, run.ID.Hex(), &pipelineForRun, params, workspaces, in.CurrentUserId, in.CurrentRoles, in.CurrentUsername)
	_ = l.svcCtx.PipelineModel.UpdateRunStatus(l.ctx, pipeline.ID.Hex(), "preparing", in.CurrentUsername)

	return &pb.IdResp{Id: run.ID.Hex()}, nil
}

func schedulePipelineRunTrigger(svcCtx *svc.ServiceContext, runID string, pipeline *model.DevopsPipeline, runParams, runWorkspaces map[string]string, userID uint64, roles []string, operator string) {
	paramsCopy := make(map[string]string, len(runParams))
	for key, value := range runParams {
		paramsCopy[key] = value
	}
	workspacesCopy := make(map[string]string, len(runWorkspaces))
	for key, value := range runWorkspaces {
		workspacesCopy[key] = value
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		run, err := svcCtx.PipelineRunModel.FindOne(ctx, runID)
		if err != nil {
			logx.Errorf("触发流水线运行失败: runId=%s err=%v", runID, err)
			return
		}
		if run.Status != "preparing" && run.Status != "queued" {
			logx.Errorf("流水线运行状态已变更，跳过触发: runId=%s status=%s", runID, run.Status)
			return
		}
		runtime, err := buildRuntimeCached(ctx, svcCtx, pipeline.ProjectID, pipeline.SystemID, pipeline.EnvironmentID, pipeline.BuildChannelBindingID, userID, roles)
		if err != nil {
			markPipelineRunTriggerFailed(ctx, svcCtx, run, pipeline.ID.Hex(), "解析流水线运行上下文失败："+err.Error(), operator)
			return
		}
		if runtime.Project == nil || runtime.System == nil || runtime.Environment == nil || runtime.Binding == nil || runtime.Channel == nil {
			markPipelineRunTriggerFailed(ctx, svcCtx, run, pipeline.ID.Hex(), "流水线运行上下文不完整", operator)
			return
		}
		if pipeline.EngineType == engineTekton {
			if err := triggerTektonPipelineRun(ctx, svcCtx, run, pipeline, runtime, paramsCopy, workspacesCopy, operator, userID, roles); err != nil {
				markPipelineRunTriggerFailed(ctx, svcCtx, run, pipeline.ID.Hex(), "触发 Tekton PipelineRun 失败："+err.Error(), operator)
			}
			return
		}
		if err := validateJenkinsRuntime(runtime); err != nil {
			markPipelineRunTriggerFailed(ctx, svcCtx, run, pipeline.ID.Hex(), err.Error(), operator)
			return
		}
		pipeline.ProjectName = runtime.Project.Name
		pipeline.ProjectCode = runtime.Project.Code
		pipeline.SystemName = runtime.System.Name
		pipeline.SystemCode = runtime.System.Code
		pipeline.EnvironmentName = runtime.Environment.Name
		pipeline.EnvironmentCode = runtime.Environment.Code
		pipeline.BuildChannelName = runtime.Binding.ChannelName
		pipeline.JobName = uniqueJobName(runtime.System.Code, pipeline.Code, runtime.Environment.Code)
		pipeline.JobFullName = jenkinsFolder(runtime.Project.Name, runtime.Project.Code) + "/" + pipeline.JobName
		pipeline.SyncStatus = "synced"
		pipeline.UpdatedBy = operator
		rendered, err := renderPipelineWithMetadata(ctx, svcCtx, pipeline, pipelineRenderContext{UserID: userID, Roles: roles, Params: paramsCopy})
		if err != nil {
			markPipelineRunTriggerFailed(ctx, svcCtx, run, pipeline.ID.Hex(), "生成 Jenkins Pipeline 失败", operator)
			return
		}
		buildParams := pipelineBuildParamsMapFromRendered(paramsCopy, rendered.Params)
		paramBytes, _ := json.Marshal(buildParams)
		envBytes, _ := json.Marshal(map[string]string{
			"PROJECT_CODE":      pipeline.ProjectCode,
			"SYSTEM_CODE":       pipeline.SystemCode,
			"SYSTEM_NAME":       pipeline.SystemName,
			"PIPELINE_CODE":     pipeline.Code,
			"PIPELINE_NAME":     pipeline.Name,
			"PIPELINE_ENV":      pipeline.EnvironmentCode,
			"PIPELINE_ENV_NAME": pipeline.EnvironmentName,
		})
		run.ProjectName = pipeline.ProjectName
		run.ProjectCode = pipeline.ProjectCode
		run.SystemName = pipeline.SystemName
		run.SystemCode = pipeline.SystemCode
		run.EnvironmentName = pipeline.EnvironmentName
		run.EnvironmentCode = pipeline.EnvironmentCode
		run.ChannelID = runtime.Channel.Id
		run.ChannelName = runtime.Channel.Name
		run.JenkinsFolder = jenkinsFolder(runtime.Project.Name, runtime.Project.Code)
		run.JenkinsJobName = pipeline.JobName
		run.JenkinsJobFullName = pipeline.JobFullName
		run.ParamsSnapshot = string(paramBytes)
		run.EnvSnapshot = string(envBytes)
		run.PipelineSnapshot = rendered.Script
		run.UpdatedBy = operator
		if err := svcCtx.PipelineRunModel.UpdateRuntimeSnapshot(ctx, run); err != nil {
			markPipelineRunTriggerFailed(ctx, svcCtx, run, pipeline.ID.Hex(), "更新流水线运行快照失败："+err.Error(), operator)
			return
		}
		runCredentialIDs := collectPipelineCredentialIDsWithParams(pipeline, buildParams)
		manager := jenkinsManagerFromRuntime(runtime)
		if err := syncPipelineCredentials(ctx, svcCtx, pipeline, runCredentialIDs, userID, roles); err != nil {
			markPipelineRunTriggerFailed(ctx, svcCtx, run, pipeline.ID.Hex(), "同步 Jenkins 凭据失败："+err.Error(), operator)
			return
		}
		build, err := manager.BuildWithParameters(ctx, pipeline.JobFullName, buildParams)
		if err != nil {
			markPipelineRunTriggerFailed(ctx, svcCtx, run, pipeline.ID.Hex(), "触发 Jenkins 构建失败："+err.Error(), operator)
			return
		}
		run.JenkinsQueueID = build.QueueID
		run.Status = "queued"
		run.UpdatedBy = operator
		if err := svcCtx.PipelineRunModel.Update(ctx, run); err != nil {
			logx.Errorf("更新 Jenkins 队列信息失败: runId=%s err=%v", runID, err)
			return
		}
		_ = svcCtx.PipelineModel.UpdateRunStatus(ctx, pipeline.ID.Hex(), "queued", operator)
		scheduleRunBuildNumberSync(svcCtx, runID, manager, operator)
	}()
}

func markPipelineRunTriggerFailed(ctx context.Context, svcCtx *svc.ServiceContext, run *model.DevopsPipelineRun, pipelineID, message, operator string) {
	run.Status = "failed"
	run.FinishedAt = time.Now()
	run.DurationSeconds = int64(run.FinishedAt.Sub(run.StartedAt).Seconds())
	run.ErrorMessage = message
	run.UpdatedBy = operator
	_ = svcCtx.PipelineRunModel.Update(ctx, run)
	_ = svcCtx.PipelineModel.UpdateRunStatus(ctx, pipelineID, "failed", operator)
	_ = recordPipelineRunMetric(ctx, svcCtx, run)
	logx.Errorf("%s", message)
}

func pipelineBuildParamsMapFromRendered(base map[string]string, rendered []jenkins.JenkinsRenderParam) map[string]string {
	result := make(map[string]string, len(base))
	for key, value := range base {
		result[key] = value
	}
	for _, item := range rendered {
		if item.RuntimeMode != "params" {
			continue
		}
		value := strings.TrimSpace(item.CurrentValue)
		if value == "" {
			value = strings.TrimSpace(item.DefaultValue)
		}
		if value != "" {
			result[item.Code] = value
		}
	}
	return result
}

func scheduleRunBuildNumberSync(svcCtx *svc.ServiceContext, runID string, manager *jenkins.Manager, operator string) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Hour)
		defer cancel()
		for {
			run, err := svcCtx.PipelineRunModel.FindOne(ctx, runID)
			if err == nil {
				ready, syncErr := ensureRunBuildNumber(ctx, svcCtx, run, manager, operator)
				if syncErr == nil {
					if isFinalRunStatus(run.Status) {
						return
					}
					if ready {
						_ = syncRunStatusFromJenkins(ctx, svcCtx, run, manager, operator)
						if isFinalRunStatus(run.Status) {
							return
						}
					}
				}
				if time.Since(run.StartedAt) > 2*time.Hour {
					_ = markRunFinal(ctx, svcCtx, run, "failed", "流水线运行状态跟踪超时，请检查 Jenkins 构建状态", operator)
					return
				}
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(3 * time.Second):
			}
		}
	}()
}

func validateRunRequiredParams(items []model.PipelineParam, values map[string]string) error {
	for _, item := range items {
		if item.ParamType == channelvars.ParamChannelCredential {
			continue
		}
		if effectiveParamRuntimeMode(item) != "params" || !item.Required {
			continue
		}
		if strings.TrimSpace(values[item.Code]) != "" {
			continue
		}
		name := strings.TrimSpace(item.Name)
		if name == "" {
			name = item.Code
		}
		logx.Errorf("%s", "请填写运行参数："+name)
		return errorx.Msg("请填写运行参数：" + name)
	}
	return nil
}

func shouldValidateRunDynamicParams(pipeline *model.DevopsPipeline) bool {
	return pipeline != nil && pipeline.EngineType == engineTekton
}

func validateRunDynamicParams(ctx context.Context, svcCtx *svc.ServiceContext, pipeline *model.DevopsPipeline, values map[string]string, userID uint64, roles []string) error {
	if pipeline == nil {
		return nil
	}
	paramByCode := make(map[string]model.PipelineParam, len(pipeline.Params))
	for _, item := range pipeline.Params {
		paramByCode[item.Code] = item
	}
	for _, item := range pipeline.Params {
		effectiveType := pipelineDynamicParamType(item)
		if !isDynamicPipelineParam(item) {
			continue
		}
		value := strings.TrimSpace(values[item.Code])
		if value == "" {
			value = pipelineParamValue(item)
		}
		if value == "" || !normalizePipelineParamConfig(item).ValidateRemote {
			continue
		}
		config := normalizePipelineParamConfig(item)
		channelBindingID := strings.TrimSpace(config.ChannelBindingID)
		if channelBindingID == "" && config.ChannelParamCode != "" {
			channelParam, ok := paramByCode[config.ChannelParamCode]
			if !ok {
				logx.Errorf("%s", "动态参数缺少渠道依赖："+displayPipelineParamName(item))
				return errorx.Msg("动态参数缺少渠道依赖：" + displayPipelineParamName(item))
			}
			channelBindingID = pipelineParamValue(channelParam)
		}
		projectValue := ""
		projectParamCode := dynamicProjectParamCode(effectiveType, config)
		if projectParamCode != "" {
			projectParam, ok := paramByCode[projectParamCode]
			if !ok {
				logx.Errorf("%s", "动态参数缺少上级依赖："+displayPipelineParamName(item))
				return errorx.Msg("动态参数缺少上级依赖：" + displayPipelineParamName(item))
			}
			projectValue = strings.TrimSpace(values[projectParam.Code])
			if projectValue == "" {
				projectValue = pipelineParamValue(projectParam)
			}
		}
		if channelBindingID == "" && !canValidateDynamicParamWithRepositoryURLType(effectiveType, projectValue) {
			logx.Errorf("%s", "动态参数缺少渠道绑定："+displayPipelineParamName(item))
			return errorx.Msg("动态参数缺少渠道绑定：" + displayPipelineParamName(item))
		}
		componentValue := ""
		if item.ParamType == "voucherModel" {
			componentValue = config.VoucherModel
		}
		if config.ComponentParamCode != "" {
			componentParam, ok := paramByCode[config.ComponentParamCode]
			if !ok {
				logx.Errorf("%s", "动态参数缺少组件依赖："+displayPipelineParamName(item))
				return errorx.Msg("动态参数缺少组件依赖：" + displayPipelineParamName(item))
			}
			componentValue = strings.TrimSpace(values[componentParam.Code])
			if componentValue == "" {
				componentValue = pipelineParamValue(componentParam)
			}
		}
		dependencyValues := runDependencyValues(config, paramByCode, values)
		resp, err := svcCtx.PipelineConfigRpc.DynamicParamOptions(ctx, &pipelineconfigservice.DynamicParamOptionsReq{
			ProjectId:        pipeline.ProjectID,
			SystemId:         pipeline.SystemID,
			EnvironmentId:    pipeline.EnvironmentID,
			ParamType:        effectiveType,
			ChannelBindingId: channelBindingID,
			ProjectValue:     projectValue,
			ComponentValue:   componentValue,
			ConfigTypeCode:   config.ConfigTypeCode,
			MappingField:     config.MappingField,
			DependencyValues: dependencyValues,
			Keyword:          value,
			Page:             1,
			PageSize:         50,
			CurrentUserId:    userID,
			CurrentRoles:     roles,
		})
		if err != nil {
			logx.Errorf("校验动态参数失败: param=%s err=%v", displayPipelineParamName(item), err)
			return errorx.Msg("校验动态参数失败：" + displayPipelineParamName(item))
		}
		if !runDynamicParamAllowCustom(item) && !dynamicOptionContains(resp.Data, value) {
			logx.Errorf("%s", "动态参数值无效："+displayPipelineParamName(item))
			return errorx.Msg("动态参数值无效：" + displayPipelineParamName(item))
		}
	}
	return nil
}

func dynamicProjectParamCode(paramType string, config model.StepParamConfig) string {
	if strings.TrimSpace(config.ProjectParamCode) != "" {
		return strings.TrimSpace(config.ProjectParamCode)
	}
	switch strings.TrimSpace(paramType) {
	case channelvars.ParamGitBranch, channelvars.ParamGitTag:
		for _, dep := range config.DependencyParamCodes {
			field := strings.TrimSpace(dep.Field)
			if field == channelvars.FieldDynamicProject || field == channelvars.FieldAddressProjectURL {
				return strings.TrimSpace(dep.ParamCode)
			}
		}
	}
	return ""
}

func runDependencyValues(config model.StepParamConfig, paramByCode map[string]model.PipelineParam, values map[string]string) string {
	if len(config.DependencyParamCodes) == 0 {
		return ""
	}
	data := make(map[string]string, len(config.DependencyParamCodes))
	for _, dep := range config.DependencyParamCodes {
		field := strings.TrimSpace(dep.Field)
		paramCode := strings.TrimSpace(dep.ParamCode)
		if field == "" || paramCode == "" {
			continue
		}
		param, ok := paramByCode[paramCode]
		if !ok {
			continue
		}
		value := strings.TrimSpace(values[param.Code])
		if value == "" {
			value = pipelineParamValue(param)
		}
		if value != "" {
			data[field] = value
		}
	}
	if len(data) == 0 {
		return ""
	}
	raw, err := json.Marshal(data)
	if err != nil {
		return ""
	}
	return string(raw)
}

func canValidateDynamicParamWithRepositoryURL(param any, projectValue string) bool {
	if item, ok := param.(model.PipelineParam); ok {
		return canValidateDynamicParamWithRepositoryURLType(pipelineDynamicParamType(item), projectValue)
	}
	return canValidateDynamicParamWithRepositoryURLType(fmt.Sprint(param), projectValue)
}

func canValidateDynamicParamWithRepositoryURLType(paramType, projectValue string) bool {
	switch strings.TrimSpace(paramType) {
	case channelvars.ParamGitBranch, channelvars.ParamGitTag:
		return strings.HasPrefix(strings.TrimSpace(projectValue), "http://") ||
			strings.HasPrefix(strings.TrimSpace(projectValue), "https://")
	default:
		return false
	}
}

func isDynamicPipelineParam(item model.PipelineParam) bool {
	if strings.TrimSpace(item.ParamType) == channelvars.ParamChannelVariable {
		dynamicType := channelDynamicParamType(item)
		if dynamicType == channelvars.ParamKubeNovaDeployConfig || dynamicType == channelvars.ParamKubernetesDeployConfig {
			return false
		}
		return dynamicType != "" && !channelvars.IsCompositeAddressParamType(dynamicType)
	}
	if item.Mode != "paas" {
		return false
	}
	if dynamicType := channelDynamicParamType(item); dynamicType != "" {
		return !channelvars.IsCompositeAddressParamType(dynamicType)
	}
	return channelvars.IsDynamicParamType(item.ParamType) || item.ParamType == "voucherModel"
}

func pipelineParamValue(item model.PipelineParam) string {
	value := strings.TrimSpace(item.CurrentValue)
	if value == "" {
		value = strings.TrimSpace(item.DefaultValue)
	}
	return value
}

func displayPipelineParamName(item model.PipelineParam) string {
	name := strings.TrimSpace(item.Name)
	if name != "" {
		return name
	}
	return item.Code
}

func dynamicOptionContains(items []*pipelineconfigservice.DynamicParamOption, value string) bool {
	value = strings.TrimSpace(value)
	for _, item := range items {
		if item == nil {
			continue
		}
		if strings.TrimSpace(item.Value) == value {
			return true
		}
	}
	return false
}

func runDynamicParamAllowCustom(item model.PipelineParam) bool {
	if item.AllowCustom {
		return true
	}
	paramType := pipelineDynamicParamType(item)
	return channelvars.IsDynamicParamType(paramType) || strings.TrimSpace(item.ParamType) == channelvars.ParamChannelVariable
}
