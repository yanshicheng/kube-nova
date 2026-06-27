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

type PipelineCreateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPipelineCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PipelineCreateLogic {
	return &PipelineCreateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *PipelineCreateLogic) PipelineCreate(in *pb.CreatePipelineReq) (*pb.IdResp, error) {
	if strings.TrimSpace(in.EngineType) == "" {
		in.EngineType = engineJenkins
	}
	if in.EngineType != engineJenkins && in.EngineType != engineTekton {
		l.Errorf("流水线引擎不支持")
		return nil, errorx.Msg("流水线引擎不支持")
	}
	if err := validateCode(in.Code, "流水线编码"); err != nil {
		l.Errorf("流水线创建失败: %v", err)
		return nil, err
	}
	if strings.TrimSpace(in.Name) == "" {
		l.Errorf("流水线名称不能为空")
		return nil, errorx.Msg("流水线名称不能为空")
	}
	tektonTemplateSnapshot := strings.TrimSpace(in.TektonTemplateSnapshot)
	if in.EngineType == engineTekton {
		tektonTemplateID := strings.TrimSpace(in.TektonTemplateId)
		if tektonTemplateID == "" {
			tektonTemplateID = strings.TrimSpace(in.TemplateId)
		}
		template, snapshot, err := resolveTektonProjectPipelineTemplate(l.ctx, l.svcCtx, tektonTemplateID, in.ProjectId, in.CurrentUserId, in.CurrentRoles)
		if err != nil {
			l.Errorf("流水线创建失败: %v", err)
			return nil, err
		}
		in.TemplateId = tektonTemplateID
		in.TektonTemplateId = tektonTemplateID
		tektonDagConfig, err := applyTektonWorkspaceBindingsToDag(template.TektonDagConfig, in.TektonWorkspaceBindings)
		if err != nil {
			l.Errorf("流水线创建失败: %v", err)
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
	steps, params, err := preparePipelineSnapshotForEngine(l.ctx, l.svcCtx, in.EngineType, in.Steps, in.Params, in.TemplateId, in.TektonDagConfig, in.ProjectId, in.CurrentUserId, in.CurrentRoles)
	if err != nil {
		l.Errorf("流水线创建失败: %v", err)
		return nil, err
	}
	runtime, err := buildRuntimeCached(l.ctx, l.svcCtx, in.ProjectId, in.SystemId, in.EnvironmentId, in.BuildChannelBindingId, in.CurrentUserId, in.CurrentRoles)
	if err != nil {
		l.Errorf("流水线创建失败: %v", err)
		return nil, err
	}
	if in.EngineType == engineJenkins {
		if err := validateJenkinsRuntime(runtime); err != nil {
			l.Errorf("流水线创建失败: %v", err)
			return nil, err
		}
	}
	if _, err := l.svcCtx.PipelineModel.FindOneByUnique(l.ctx, runtime.Project.Id, runtime.System.Id, strings.TrimSpace(in.Code), runtime.Environment.Id); err == nil {
		l.Errorf("同系统同环境下流水线编码已存在")
		return nil, errorx.Msg("同系统同环境下流水线编码已存在")
	} else if !errors.Is(err, model.ErrNotFound) {
		l.Errorf("流水线创建失败: %v", err)
		return nil, err
	}
	jobName := uniqueJobName(runtime.System.Code, strings.TrimSpace(in.Code), runtime.Environment.Code)
	folder := jenkinsFolder(runtime.Project.Name, runtime.Project.Code)
	jobFullName := folder + "/" + jobName
	triggers := triggersFromPb(in.Triggers)
	data := &model.DevopsPipeline{
		ProjectID:               runtime.Project.Id,
		ProjectName:             runtime.Project.Name,
		ProjectCode:             runtime.Project.Code,
		SystemID:                runtime.System.Id,
		SystemName:              runtime.System.Name,
		SystemCode:              runtime.System.Code,
		EnvironmentID:           runtime.Environment.Id,
		EnvironmentName:         runtime.Environment.Name,
		EnvironmentCode:         runtime.Environment.Code,
		Name:                    strings.TrimSpace(in.Name),
		Code:                    strings.TrimSpace(in.Code),
		EngineType:              in.EngineType,
		TemplateID:              in.TemplateId,
		BuildChannelBindingID:   in.BuildChannelBindingId,
		BuildChannelName:        runtime.Binding.ChannelName,
		Steps:                   steps,
		Params:                  params,
		Agent:                   agentFromPb(in.Agent),
		Tools:                   toolsFromPb(in.Tools),
		Options:                 optionsFromPb(in.Options),
		Triggers:                triggers,
		JobName:                 jobName,
		JobFullName:             jobFullName,
		TriggerMode:             normalizeTriggerMode(in.TriggerMode, triggers),
		ScanEnabled:             in.ScanEnabled,
		ScanMode:                strings.TrimSpace(in.ScanMode),
		RejectUnexpectedReports: in.RejectUnexpectedReports,
		Enforce:                 in.Enforce,
		ScanItems:               scanItemsFromPb(in.ScanItems),
		TektonDagConfig:         strings.TrimSpace(in.TektonDagConfig),
		TektonRunPolicy:         strings.TrimSpace(in.TektonRunPolicy),
		TektonTriggerConfig:     strings.TrimSpace(in.TektonTriggerConfig),
		TektonPrunerPolicyRef:   strings.TrimSpace(in.TektonPrunerPolicyRef),
		TektonTemplateID:        strings.TrimSpace(in.TektonTemplateId),
		TektonTemplateSnapshot:  tektonTemplateSnapshot,
		TektonParamBindings:     strings.TrimSpace(in.TektonParamBindings),
		TektonWorkspaceBindings: strings.TrimSpace(in.TektonWorkspaceBindings),
		TektonResourceBindings:  strings.TrimSpace(in.TektonResourceBindings),
		SyncStatus:              "pending",
		LastRunStatus:           "never",
		Status:                  in.Status,
		CreatedBy:               in.CreatedBy,
		UpdatedBy:               in.CreatedBy,
	}
	if data.Agent.Type == "" {
		data.Agent.Type = defaultPipelineAgent
	}
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
		l.Errorf("流水线创建失败: %v", err)
		return nil, err
	}
	if err := validatePipelineScanItems(data.ScanEnabled, data.ScanItems); err != nil {
		l.Errorf("流水线创建失败: %v", err)
		return nil, err
	}
	if data.Status == 0 {
		data.Status = 1
	}
	if err := l.svcCtx.PipelineModel.Insert(l.ctx, data); err != nil {
		if model.IsDuplicateKey(err) {
			l.Errorf("同系统同环境下流水线编码已存在")
			return nil, errorx.Msg("同系统同环境下流水线编码已存在")
		}
		l.Errorf("流水线创建失败: %v", err)
		return nil, err
	}
	if data.EngineType == engineTekton {
		if _, err := syncTektonPipeline(l.ctx, l.svcCtx, data, runtime, nil, pipelineRenderContext{UserID: in.CurrentUserId, Roles: in.CurrentRoles}); err != nil {
			message := "同步 Tekton Pipeline 失败：" + err.Error()
			if deleteErr := l.svcCtx.PipelineModel.DeleteSoft(l.ctx, data.ID.Hex(), in.CreatedBy); deleteErr != nil {
				l.Errorf("Tekton 流水线创建失败后回滚数据库失败: pipelineId=%s err=%v", data.ID.Hex(), deleteErr)
			}
			l.Errorf("%s", message)
			return nil, errorx.Msg(message)
		}
		if err := syncTektonTrigger(l.ctx, data, runtime); err != nil {
			message := "同步 Tekton Trigger 失败：" + err.Error()
			if deleteErr := deleteTektonPipeline(l.ctx, data, runtime); deleteErr != nil {
				l.Errorf("Tekton Trigger 同步失败后回滚远端 Pipeline 失败: pipelineId=%s err=%v", data.ID.Hex(), deleteErr)
			}
			if deleteErr := l.svcCtx.PipelineModel.DeleteSoft(l.ctx, data.ID.Hex(), in.CreatedBy); deleteErr != nil {
				l.Errorf("Tekton 流水线创建失败后回滚数据库失败: pipelineId=%s err=%v", data.ID.Hex(), deleteErr)
			}
			l.Errorf("%s", message)
			return nil, errorx.Msg(message)
		}
		if err := syncTektonNativePrunerConfigWithRuntime(l.ctx, data, parseTektonPrunerPolicy(data), runtime); err != nil {
			message := "同步 Tekton Pruner ConfigMap 失败：" + err.Error()
			if deleteErr := deleteTektonTrigger(l.ctx, data, runtime); deleteErr != nil {
				l.Errorf("Tekton Pruner 同步失败后回滚 Trigger 失败: pipelineId=%s err=%v", data.ID.Hex(), deleteErr)
			}
			if deleteErr := deleteTektonPipeline(l.ctx, data, runtime); deleteErr != nil {
				l.Errorf("Tekton Pruner 同步失败后回滚远端 Pipeline 失败: pipelineId=%s err=%v", data.ID.Hex(), deleteErr)
			}
			if deleteErr := l.svcCtx.PipelineModel.DeleteSoft(l.ctx, data.ID.Hex(), in.CreatedBy); deleteErr != nil {
				l.Errorf("Tekton 流水线创建失败后回滚数据库失败: pipelineId=%s err=%v", data.ID.Hex(), deleteErr)
			}
			l.Errorf("%s", message)
			return nil, errorx.Msg(message)
		}
		data.SyncStatus = "synced"
		data.SyncMessage = ""
		if err := l.svcCtx.PipelineModel.Update(l.ctx, data); err != nil {
			l.Errorf("流水线创建失败: %v", err)
			return nil, err
		}
		return &pb.IdResp{Id: data.ID.Hex()}, nil
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
	if err := manager.CreatePipelineJobWithParams(l.ctx, folder, jobName, displayName, pipelineScript, rendered.Params); err != nil {
		message := "同步 Jenkins Job 失败：" + err.Error()
		l.Errorf("同步 Jenkins Job 失败: pipelineName=%s folder=%s jobName=%s jobFullName=%s err=%v", data.Name, folder, jobName, jobFullName, err)
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
		if model.IsDuplicateKey(err) {
			l.Errorf("同系统同环境下流水线编码已存在")
			return nil, errorx.Msg("同系统同环境下流水线编码已存在")
		}
		l.Errorf("流水线创建失败: %v", err)
		return nil, err
	}

	return &pb.IdResp{Id: data.ID.Hex()}, nil
}
