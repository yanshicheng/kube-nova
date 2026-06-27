package executionservicelogic

import (
	"context"
	"errors"
	"strings"

	"github.com/yanshicheng/kube-nova/application/devops-pipeline-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/devops-pipeline-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-pipeline-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type PipelineUpdateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPipelineUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PipelineUpdateLogic {
	return &PipelineUpdateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *PipelineUpdateLogic) PipelineUpdate(in *pb.UpdatePipelineReq) (*pb.EmptyResp, error) {
	data, err := l.svcCtx.PipelineModel.FindOne(l.ctx, in.Id)
	if err != nil {
		l.Errorf("流水线更新失败: %v", err)
		return nil, err
	}
	if err := ensurePipelineAccess(l.ctx, l.svcCtx, data, in.CurrentUserId, in.CurrentRoles); err != nil {
		l.Errorf("流水线更新失败: %v", err)
		return nil, err
	}
	previous := *data
	currentEngineType := strings.TrimSpace(data.EngineType)
	if currentEngineType == "" {
		currentEngineType = engineJenkins
	}
	engineType := currentEngineType
	if strings.TrimSpace(in.EngineType) != "" {
		engineType = strings.TrimSpace(in.EngineType)
	}
	if engineType != currentEngineType {
		l.Errorf("流水线引擎类型不允许修改")
		return nil, errorx.Msg("流水线引擎类型不允许修改")
	}
	if engineType != engineJenkins && engineType != engineTekton {
		l.Errorf("流水线引擎不支持")
		return nil, errorx.Msg("流水线引擎不支持")
	}
	tektonTemplateSnapshot := strings.TrimSpace(in.TektonTemplateSnapshot)
	if engineType == engineTekton {
		tektonTemplateID := strings.TrimSpace(in.TektonTemplateId)
		if tektonTemplateID == "" {
			tektonTemplateID = strings.TrimSpace(in.TemplateId)
		}
		if tektonTemplateID == "" {
			tektonTemplateID = strings.TrimSpace(data.TektonTemplateID)
		}
		template, snapshot, err := resolveTektonProjectPipelineTemplate(l.ctx, l.svcCtx, tektonTemplateID, data.ProjectID, in.CurrentUserId, in.CurrentRoles)
		if err != nil {
			l.Errorf("流水线更新失败: %v", err)
			return nil, err
		}
		in.TemplateId = tektonTemplateID
		in.TektonTemplateId = tektonTemplateID
		tektonDagConfig, err := applyTektonWorkspaceBindingsToDag(template.TektonDagConfig, in.TektonWorkspaceBindings)
		if err != nil {
			l.Errorf("流水线更新失败: %v", err)
			return nil, err
		}
		in.TektonDagConfig = tektonDagConfig
		if strings.TrimSpace(in.TektonRunPolicy) == "" {
			in.TektonRunPolicy = template.TektonRunPolicy
		}
		if strings.TrimSpace(in.TektonTriggerConfig) == "" {
			in.TektonTriggerConfig = template.TektonTriggerConfig
		}
		if strings.TrimSpace(in.TektonPrunerPolicyRef) == "" {
			in.TektonPrunerPolicyRef = template.TektonPrunerPolicyRef
		}
		if tektonTemplateSnapshot == "" {
			tektonTemplateSnapshot = snapshot
		}
	}
	steps, params, err := preparePipelineSnapshotForEngine(l.ctx, l.svcCtx, engineType, in.Steps, in.Params, in.TemplateId, in.TektonDagConfig, data.ProjectID, in.CurrentUserId, in.CurrentRoles)
	if err != nil {
		l.Errorf("流水线更新失败: %v", err)
		return nil, err
	}
	if err := validateReadonlyPipelineParams(data.Params, params); err != nil {
		l.Errorf("流水线更新失败: %v", err)
		return nil, err
	}
	systemID := data.SystemID
	envID := data.EnvironmentID
	bindingID := strings.TrimSpace(in.BuildChannelBindingId)
	if bindingID == "" {
		bindingID = data.BuildChannelBindingID
	}
	runtime, err := buildRuntimeCached(l.ctx, l.svcCtx, data.ProjectID, systemID, envID, bindingID, in.CurrentUserId, in.CurrentRoles)
	if err != nil {
		l.Errorf("流水线更新失败: %v", err)
		return nil, err
	}
	if engineType == engineJenkins {
		if err := validateJenkinsRuntime(runtime); err != nil {
			l.Errorf("流水线更新失败: %v", err)
			return nil, err
		}
	}
	if existing, err := l.svcCtx.PipelineModel.FindOneByUnique(l.ctx, data.ProjectID, runtime.System.Id, data.Code, runtime.Environment.Id); err == nil {
		if existing.ID != data.ID {
			l.Errorf("同系统同环境下流水线编码已存在")
			return nil, errorx.Msg("同系统同环境下流水线编码已存在")
		}
	} else if !errors.Is(err, model.ErrNotFound) {
		l.Errorf("流水线更新失败: %v", err)
		return nil, err
	}
	data.SystemID = runtime.System.Id
	data.SystemName = runtime.System.Name
	data.SystemCode = runtime.System.Code
	data.EnvironmentID = runtime.Environment.Id
	data.EnvironmentName = runtime.Environment.Name
	data.EnvironmentCode = runtime.Environment.Code
	data.EngineType = engineType
	data.TemplateID = in.TemplateId
	data.BuildChannelBindingID = bindingID
	data.BuildChannelName = runtime.Binding.ChannelName
	data.Steps = steps
	data.Params = params
	data.Agent = agentFromPb(in.Agent)
	if data.Agent.Type == "" {
		data.Agent.Type = defaultPipelineAgent
	}
	data.Tools = toolsFromPb(in.Tools)
	data.Options = optionsFromPb(in.Options)
	data.Triggers = triggersFromPb(in.Triggers)
	data.TriggerMode = normalizeTriggerMode(in.TriggerMode, data.Triggers)
	data.ScanEnabled = in.ScanEnabled
	data.ScanMode = strings.TrimSpace(in.ScanMode)
	data.RejectUnexpectedReports = in.RejectUnexpectedReports
	data.Enforce = in.Enforce
	data.ScanItems = scanItemsFromPb(in.ScanItems)
	data.TektonDagConfig = strings.TrimSpace(in.TektonDagConfig)
	data.TektonRunPolicy = strings.TrimSpace(in.TektonRunPolicy)
	data.TektonTriggerConfig = strings.TrimSpace(in.TektonTriggerConfig)
	data.TektonPrunerPolicyRef = strings.TrimSpace(in.TektonPrunerPolicyRef)
	data.TektonTemplateID = strings.TrimSpace(in.TektonTemplateId)
	data.TektonTemplateSnapshot = tektonTemplateSnapshot
	data.TektonParamBindings = strings.TrimSpace(in.TektonParamBindings)
	data.TektonWorkspaceBindings = strings.TrimSpace(in.TektonWorkspaceBindings)
	data.TektonResourceBindings = strings.TrimSpace(in.TektonResourceBindings)
	if data.EngineType == engineJenkins {
		data.TektonDagConfig = ""
		data.TektonRunPolicy = ""
		data.TektonTriggerConfig = ""
		data.TektonPrunerPolicyRef = ""
		data.TektonTemplateID = ""
		data.TektonTemplateSnapshot = ""
		data.TektonParamBindings = ""
		data.TektonWorkspaceBindings = ""
		data.TektonResourceBindings = ""
	}
	if err := validateTektonPolicyConfigs(data); err != nil {
		l.Errorf("流水线更新失败: %v", err)
		return nil, err
	}
	if err := validatePipelineScanItems(data.ScanEnabled, data.ScanItems); err != nil {
		l.Errorf("流水线更新失败: %v", err)
		return nil, err
	}
	data.JobName = uniqueJobName(runtime.System.Code, data.Code, runtime.Environment.Code)
	folder := jenkinsFolder(runtime.Project.Name, runtime.Project.Code)
	data.JobFullName = folder + "/" + data.JobName
	data.SyncStatus = "pending"
	data.SyncMessage = ""
	data.Status = in.Status
	data.UpdatedBy = in.UpdatedBy
	if data.EngineType == engineTekton {
		if _, err := syncTektonPipeline(l.ctx, l.svcCtx, data, runtime, nil, pipelineRenderContext{UserID: in.CurrentUserId, Roles: in.CurrentRoles}); err != nil {
			message := "同步 Tekton Pipeline 失败：" + err.Error()
			data.SyncStatus = "failed"
			data.SyncMessage = message
			if updateErr := l.svcCtx.PipelineModel.Update(l.ctx, data); updateErr != nil {
				l.Errorf("记录流水线同步失败状态失败: pipelineId=%s err=%v", data.ID.Hex(), updateErr)
			}
			l.Errorf("%s", message)
			return nil, errorx.Msg(message)
		}
		var triggerErr error
		if previous.EngineType == engineTekton && samePipelineRuntimeTarget(&previous, data) {
			triggerErr = syncTektonTriggerWithPrevious(l.ctx, data, &previous, runtime)
		} else {
			triggerErr = syncTektonTrigger(l.ctx, data, runtime)
		}
		if triggerErr != nil {
			message := "同步 Tekton Trigger 失败：" + triggerErr.Error()
			data.SyncStatus = "failed"
			data.SyncMessage = message
			if updateErr := l.svcCtx.PipelineModel.Update(l.ctx, data); updateErr != nil {
				l.Errorf("记录流水线同步失败状态失败: pipelineId=%s err=%v", data.ID.Hex(), updateErr)
			}
			l.Errorf("%s", message)
			return nil, errorx.Msg(message)
		}
		if previous.EngineType == engineTekton && samePipelineRuntimeTarget(&previous, data) {
			if err := deleteTektonNativePrunerConfig(l.ctx, &previous, runtime); err != nil {
				message := "清理旧 Tekton Pruner ConfigMap 失败：" + err.Error()
				data.SyncStatus = "failed"
				data.SyncMessage = message
				if updateErr := l.svcCtx.PipelineModel.Update(l.ctx, data); updateErr != nil {
					l.Errorf("记录流水线同步失败状态失败: pipelineId=%s err=%v", data.ID.Hex(), updateErr)
				}
				l.Errorf("%s", message)
				return nil, errorx.Msg(message)
			}
		}
		if err := syncTektonNativePrunerConfigWithRuntime(l.ctx, data, parseTektonPrunerPolicy(data), runtime); err != nil {
			message := "同步 Tekton Pruner ConfigMap 失败：" + err.Error()
			data.SyncStatus = "failed"
			data.SyncMessage = message
			if updateErr := l.svcCtx.PipelineModel.Update(l.ctx, data); updateErr != nil {
				l.Errorf("记录流水线同步失败状态失败: pipelineId=%s err=%v", data.ID.Hex(), updateErr)
			}
			l.Errorf("%s", message)
			return nil, errorx.Msg(message)
		}
		data.SyncStatus = "synced"
		data.SyncMessage = ""
		if err := l.svcCtx.PipelineModel.Update(l.ctx, data); err != nil {
			l.Errorf("流水线更新失败: %v", err)
			return nil, err
		}
		cleanupPreviousPipelineRuntime(l.ctx, l.svcCtx, &previous, data, in.CurrentUserId, in.CurrentRoles)
		return &pb.EmptyResp{}, nil
	}
	manager := jenkinsManagerFromRuntime(runtime)
	displayName := runtime.System.Name + "-" + data.Name + "-" + runtime.Environment.Name
	if err := syncPipelineCredentials(l.ctx, l.svcCtx, data, nil, in.CurrentUserId, in.CurrentRoles); err != nil {
		message := "同步 Jenkins 凭证失败：" + err.Error()
		data.SyncStatus = "failed"
		data.SyncMessage = message
		if updateErr := l.svcCtx.PipelineModel.Update(l.ctx, data); updateErr != nil {
			l.Errorf("记录流水线同步失败状态失败: pipelineId=%s err=%v", data.ID.Hex(), updateErr)
		}
		l.Errorf("%s", message)
		return nil, errorx.Msg(message)
	}
	rendered, err := renderPipelineWithMetadata(l.ctx, l.svcCtx, data, pipelineRenderContext{UserID: in.CurrentUserId, Roles: in.CurrentRoles})
	if err != nil {
		message := "生成 Jenkins Pipeline 失败：" + err.Error()
		data.SyncStatus = "failed"
		data.SyncMessage = message
		if updateErr := l.svcCtx.PipelineModel.Update(l.ctx, data); updateErr != nil {
			l.Errorf("记录流水线同步失败状态失败: pipelineId=%s err=%v", data.ID.Hex(), updateErr)
		}
		l.Errorf("%s", message)
		return nil, errorx.Msg(message)
	}
	pipelineScript := rendered.Script
	if err := manager.UpsertPipelineJobWithParams(l.ctx, folder, data.JobName, displayName, pipelineScript, rendered.Params); err != nil {
		message := "同步 Jenkins Job 失败：" + err.Error()
		data.SyncStatus = "failed"
		data.SyncMessage = message
		l.Errorf("同步 Jenkins Job 失败: pipelineId=%s folder=%s jobName=%s jobFullName=%s err=%v", data.ID.Hex(), folder, data.JobName, data.JobFullName, err)
		if updateErr := l.svcCtx.PipelineModel.Update(l.ctx, data); updateErr != nil {
			l.Errorf("记录流水线同步失败状态失败: pipelineId=%s err=%v", data.ID.Hex(), updateErr)
		}
		l.Errorf("%s", message)
		return nil, errorx.Msg(message)
	}
	data.SyncStatus = "synced"
	data.SyncMessage = ""
	if err := l.svcCtx.PipelineModel.Update(l.ctx, data); err != nil {
		if model.IsDuplicateKey(err) {
			l.Errorf("同系统同环境下流水线编码已存在")
			return nil, errorx.Msg("同系统同环境下流水线编码已存在")
		}
		l.Errorf("流水线更新失败: %v", err)
		return nil, err
	}
	cleanupPreviousPipelineRuntime(l.ctx, l.svcCtx, &previous, data, in.CurrentUserId, in.CurrentRoles)

	return &pb.EmptyResp{}, nil
}

func samePipelineRuntimeTarget(previous, current *model.DevopsPipeline) bool {
	if previous == nil || current == nil {
		return false
	}
	return previous.EngineType == current.EngineType &&
		previous.SystemID == current.SystemID &&
		previous.EnvironmentID == current.EnvironmentID &&
		previous.BuildChannelBindingID == current.BuildChannelBindingID
}

func cleanupPreviousPipelineRuntime(ctx context.Context, svcCtx *svc.ServiceContext, previous, current *model.DevopsPipeline, userID uint64, roles []string) {
	if previous == nil || current == nil {
		return
	}
	if samePipelineRuntimeTarget(previous, current) && strings.TrimSpace(previous.JobFullName) == strings.TrimSpace(current.JobFullName) {
		return
	}
	if strings.TrimSpace(previous.BuildChannelBindingID) == "" {
		return
	}
	runtime, err := buildRuntimeCached(ctx, svcCtx, previous.ProjectID, previous.SystemID, previous.EnvironmentID, previous.BuildChannelBindingID, userID, roles)
	if err != nil {
		logx.WithContext(ctx).Errorf("清理旧流水线运行时失败: pipelineId=%s err=%v", previous.ID.Hex(), err)
		return
	}
	switch previous.EngineType {
	case engineTekton:
		if err := deleteTektonNativePrunerConfig(ctx, previous, runtime); err != nil {
			logx.WithContext(ctx).Errorf("清理旧 Tekton Pruner ConfigMap 失败: pipelineId=%s err=%v", previous.ID.Hex(), err)
		}
		if err := deleteTektonTrigger(ctx, previous, runtime); err != nil {
			logx.WithContext(ctx).Errorf("清理旧 Tekton Trigger 失败: pipelineId=%s err=%v", previous.ID.Hex(), err)
		}
		if err := deleteTektonPipeline(ctx, previous, runtime); err != nil {
			logx.WithContext(ctx).Errorf("清理旧 Tekton Pipeline 失败: pipelineId=%s err=%v", previous.ID.Hex(), err)
		}
	case engineJenkins:
		if !shouldDisablePreviousJenkinsJob(previous, current) {
			return
		}
		manager := jenkinsManagerFromRuntime(runtime)
		if manager == nil {
			return
		}
		if err := manager.DisableJob(ctx, previous.JobFullName); err != nil {
			logx.WithContext(ctx).Errorf("停用旧 Jenkins Job 失败: pipelineId=%s jobFullName=%s err=%v", previous.ID.Hex(), previous.JobFullName, err)
		}
	}
}

func shouldDisablePreviousJenkinsJob(previous, current *model.DevopsPipeline) bool {
	if previous == nil || current == nil || previous.EngineType != engineJenkins {
		return false
	}
	if strings.TrimSpace(previous.JobFullName) == "" {
		return false
	}
	if current.EngineType != engineJenkins {
		return true
	}
	return strings.TrimSpace(previous.JobFullName) != strings.TrimSpace(current.JobFullName) ||
		previous.SystemID != current.SystemID ||
		previous.EnvironmentID != current.EnvironmentID ||
		previous.BuildChannelBindingID != current.BuildChannelBindingID
}
