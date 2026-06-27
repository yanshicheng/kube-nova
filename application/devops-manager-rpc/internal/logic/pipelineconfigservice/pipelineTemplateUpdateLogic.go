package pipelineconfigservicelogic

import (
	"context"
	"strings"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type PipelineTemplateUpdateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPipelineTemplateUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PipelineTemplateUpdateLogic {
	return &PipelineTemplateUpdateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *PipelineTemplateUpdateLogic) PipelineTemplateUpdate(in *pb.UpdatePipelineTemplateReq) (*pb.EmptyResp, error) {
	exist, err := l.svcCtx.PipelineTemplateModel.FindOne(l.ctx, in.Id)
	if err != nil {
		l.Errorf("流水线模板更新失败: %v", err)
		return nil, err
	}
	if err := ensureTemplateWriteAccess(l.ctx, l.svcCtx, exist, in.CurrentUserId, in.CurrentRoles); err != nil {
		l.Errorf("流水线模板更新失败: %v", err)
		return nil, err
	}
	scope := normalizeTemplateScope(in.Scope)
	if err := ensureTemplateTargetAccess(l.ctx, l.svcCtx, scope, in.ProjectId, in.CurrentUserId, in.CurrentRoles); err != nil {
		l.Errorf("流水线模板更新失败: %v", err)
		return nil, err
	}
	engineType := normalizeTemplateEngineType(in.EngineType)
	if err := validateTemplateEngineType(engineType); err != nil {
		l.Errorf("流水线模板更新失败: %v", err)
		return nil, err
	}
	steps := make([]model.PipelineTemplateStep, 0)
	tektonDagConfig := ""
	tektonRunPolicy := ""
	tektonTriggerConfig := ""
	tektonPrunerPolicyRef := ""
	tektonPipelineYamlSnapshot := ""
	if engineType == tektonEngineType {
		tektonDagConfig, err = validateTektonPipelineTemplateConfig(l.ctx, l.svcCtx, in.TektonDagConfig, in.CurrentRoles)
		if err != nil {
			l.Errorf("流水线模板更新失败: %v", err)
			return nil, err
		}
		tektonRunPolicy, err = validateTektonPipelineTemplateRunPolicyContent(in.TektonRunPolicy)
		if err != nil {
			l.Errorf("流水线模板更新失败: %v", err)
			return nil, err
		}
		tektonTriggerConfig, err = validateTektonPipelineTemplateTriggerConfigContent(in.TektonTriggerConfig)
		if err != nil {
			l.Errorf("流水线模板更新失败: %v", err)
			return nil, err
		}
		tektonPrunerPolicyRef, err = validateTektonPipelineTemplatePrunerPolicyContent(in.TektonPrunerPolicyRef)
		if err != nil {
			l.Errorf("流水线模板更新失败: %v", err)
			return nil, err
		}
		tektonPipelineYamlSnapshot = strings.TrimSpace(in.TektonPipelineYamlSnapshot)
	} else {
		steps, err = validateTemplateSteps(l.ctx, l.svcCtx, engineType, pipelineStepsFromPb(in.Steps))
		if err != nil {
			l.Errorf("流水线模板更新失败: %v", err)
			return nil, err
		}
	}
	projectID := in.ProjectId
	if scope == "system" {
		projectID = ""
	}
	data := &model.DevopsPipelineTemplate{
		ID:                         exist.ID,
		Name:                       in.Name,
		Code:                       exist.Code,
		Icon:                       in.Icon,
		IconColor:                  in.IconColor,
		Description:                in.Description,
		Scope:                      scope,
		ProjectID:                  projectID,
		EngineType:                 engineType,
		Steps:                      steps,
		TektonDagConfig:            tektonDagConfig,
		TektonRunPolicy:            tektonRunPolicy,
		TektonTriggerConfig:        tektonTriggerConfig,
		TektonPrunerPolicyRef:      tektonPrunerPolicyRef,
		TektonPipelineYamlSnapshot: tektonPipelineYamlSnapshot,
		Status:                     in.Status,
		UpdatedBy:                  in.UpdatedBy,
	}
	if err := l.svcCtx.PipelineTemplateModel.Update(l.ctx, data); err != nil {
		l.Errorf("流水线模板更新失败: %v", err)
		return nil, err
	}

	return &pb.EmptyResp{}, nil
}
