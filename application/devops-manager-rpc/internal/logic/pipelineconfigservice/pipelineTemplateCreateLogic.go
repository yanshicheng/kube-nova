package pipelineconfigservicelogic

import (
	"context"
	"errors"
	"strings"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type PipelineTemplateCreateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPipelineTemplateCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PipelineTemplateCreateLogic {
	return &PipelineTemplateCreateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *PipelineTemplateCreateLogic) PipelineTemplateCreate(in *pb.CreatePipelineTemplateReq) (*pb.IdResp, error) {
	if err := validatePipelineCode(in.Code, "模板编码"); err != nil {
		l.Errorf("流水线模板创建失败: %v", err)
		return nil, err
	}
	if _, err := l.svcCtx.PipelineTemplateModel.FindOneByCode(l.ctx, in.Code); err == nil {
		l.Errorf("流水线模板编码已存在")
		return nil, errorx.Msg("流水线模板编码已存在")
	} else if !errors.Is(err, model.ErrNotFound) {
		l.Errorf("流水线模板创建失败: %v", err)
		return nil, err
	}
	scope := normalizeTemplateScope(in.Scope)
	if err := ensureTemplateTargetAccess(l.ctx, l.svcCtx, scope, in.ProjectId, in.CurrentUserId, in.CurrentRoles); err != nil {
		l.Errorf("流水线模板创建失败: %v", err)
		return nil, err
	}
	engineType := normalizeTemplateEngineType(in.EngineType)
	if err := validateTemplateEngineType(engineType); err != nil {
		l.Errorf("流水线模板创建失败: %v", err)
		return nil, err
	}
	steps := make([]model.PipelineTemplateStep, 0)
	tektonDagConfig := ""
	tektonRunPolicy := ""
	tektonTriggerConfig := ""
	tektonPrunerPolicyRef := ""
	tektonPipelineYamlSnapshot := ""
	if engineType == tektonEngineType {
		var err error
		tektonDagConfig, err = validateTektonPipelineTemplateConfig(l.ctx, l.svcCtx, in.TektonDagConfig, in.CurrentRoles)
		if err != nil {
			l.Errorf("流水线模板创建失败: %v", err)
			return nil, err
		}
		tektonRunPolicy, err = validateTektonPipelineTemplateRunPolicyContent(in.TektonRunPolicy)
		if err != nil {
			l.Errorf("流水线模板创建失败: %v", err)
			return nil, err
		}
		tektonTriggerConfig, err = validateTektonPipelineTemplateTriggerConfigContent(in.TektonTriggerConfig)
		if err != nil {
			l.Errorf("流水线模板创建失败: %v", err)
			return nil, err
		}
		tektonPrunerPolicyRef, err = validateTektonPipelineTemplatePrunerPolicyContent(in.TektonPrunerPolicyRef)
		if err != nil {
			l.Errorf("流水线模板创建失败: %v", err)
			return nil, err
		}
		tektonPipelineYamlSnapshot = strings.TrimSpace(in.TektonPipelineYamlSnapshot)
	} else {
		var err error
		steps, err = validateTemplateSteps(l.ctx, l.svcCtx, engineType, pipelineStepsFromPb(in.Steps))
		if err != nil {
			l.Errorf("流水线模板创建失败: %v", err)
			return nil, err
		}
	}
	data := &model.DevopsPipelineTemplate{
		Name:                       in.Name,
		Code:                       in.Code,
		Icon:                       in.Icon,
		IconColor:                  in.IconColor,
		Description:                in.Description,
		Scope:                      scope,
		ProjectID:                  in.ProjectId,
		EngineType:                 engineType,
		Steps:                      steps,
		TektonDagConfig:            tektonDagConfig,
		TektonRunPolicy:            tektonRunPolicy,
		TektonTriggerConfig:        tektonTriggerConfig,
		TektonPrunerPolicyRef:      tektonPrunerPolicyRef,
		TektonPipelineYamlSnapshot: tektonPipelineYamlSnapshot,
		Status:                     in.Status,
		CreatedBy:                  in.CreatedBy,
		UpdatedBy:                  in.CreatedBy,
	}
	if data.Status == 0 {
		data.Status = 1
	}
	if data.Scope == "system" {
		data.ProjectID = ""
	}
	if err := l.svcCtx.PipelineTemplateModel.Insert(l.ctx, data); err != nil {
		l.Errorf("流水线模板创建失败: %v", err)
		return nil, err
	}

	return &pb.IdResp{Id: data.ID.Hex()}, nil
}
