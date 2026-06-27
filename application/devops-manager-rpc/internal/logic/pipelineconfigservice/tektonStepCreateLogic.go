package pipelineconfigservicelogic

import (
	"context"
	"errors"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/tektonsync"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"
	devopstypes "github.com/yanshicheng/kube-nova/common/devops/types"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type TektonStepCreateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewTektonStepCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TektonStepCreateLogic {
	return &TektonStepCreateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *TektonStepCreateLogic) TektonStepCreate(in *pb.CreateStepTemplateReq) (*pb.IdResp, error) {
	if err := validatePipelineCode(in.Code, "步骤编码"); err != nil {
		l.Errorf("Tekton步骤创建失败: %v", err)
		return nil, err
	}
	if _, err := l.svcCtx.StepTemplateModel.FindOneByCode(l.ctx, tektonEngineType, in.Code); err == nil {
		l.Errorf("Tekton 步骤编码已存在")
		return nil, errorx.Msg("Tekton 步骤编码已存在")
	} else if !errors.Is(err, model.ErrNotFound) {
		l.Errorf("Tekton步骤创建失败: %v", err)
		return nil, err
	}
	category, err := l.svcCtx.StepCategoryModel.FindOne(l.ctx, in.CategoryId)
	if err != nil {
		l.Errorf("Tekton步骤创建失败: %v", err)
		return nil, err
	}
	if category.Status != 1 {
		l.Errorf("步骤分类已停用")
		return nil, errorx.Msg("步骤分类已停用")
	}
	stageContent, err := validateTektonStepContent(in.StageContent, in.Code)
	if err != nil {
		l.Errorf("Tekton步骤创建失败: %v", err)
		return nil, err
	}
	stepType := normalizeStepType(in.Type)
	if stepType != stepTypeTask {
		l.Errorf("Tekton 步骤只支持任务类型")
		return nil, errorx.Msg("Tekton 步骤只支持任务类型")
	}
	taskParams, taskResults, taskWorkspaces, err := tektonTaskContractsFromContent(stageContent)
	if err != nil {
		l.Errorf("Tekton步骤创建失败: %v", err)
		return nil, err
	}
	taskParams = mergeTektonTaskParamStepContracts(taskParams, in.Params)
	taskParams = mergeTektonTaskParamContracts(taskParams, in.TaskParams)
	params := stepParamsFromTektonTaskParams(taskParams, stepParamsFromPb(in.Params))
	if err := validateStepParams(l.ctx, l.svcCtx, tektonEngineType, "", params); err != nil {
		l.Errorf("Tekton步骤创建失败: %v", err)
		return nil, err
	}
	if err := validateTektonStepParamMappings(stageContent, params); err != nil {
		l.Errorf("Tekton步骤创建失败: %v", err)
		return nil, err
	}
	artifactConfig := artifactConfigFromPb(in.ArtifactConfig)
	if err := validateStepArtifactConfig(artifactConfig); err != nil {
		l.Errorf("Tekton步骤创建失败: %v", err)
		return nil, err
	}
	data := &model.DevopsStepTemplate{
		Name:                   in.Name,
		Code:                   in.Code,
		Icon:                   in.Icon,
		IconColor:              in.IconColor,
		Description:            in.Description,
		Type:                   stepType,
		CategoryID:             in.CategoryId,
		EngineType:             tektonEngineType,
		EngineChannelGroupCode: tektonEngineChannelGroupCode,
		EngineChannelType:      tektonEngineChannelType,
		StageContent:           stageContent,
		Params:                 params,
		TaskParams:             taskParams,
		TaskResults:            taskResults,
		TaskWorkspaces:         taskWorkspaces,
		ArtifactConfig:         artifactConfig,
		Status:                 in.Status,
		CreatedBy:              in.CreatedBy,
		UpdatedBy:              in.CreatedBy,
	}
	if data.Status == 0 {
		data.Status = 1
	}
	if err := l.svcCtx.StepTemplateModel.Insert(l.ctx, data); err != nil {
		l.Errorf("Tekton步骤创建失败: %v", err)
		return nil, err
	}
	if err := tektonsync.NewSyncer(l.svcCtx).SyncStepToAllChannels(l.ctx, data, in.CreatedBy); err != nil {
		l.Errorf("Tekton步骤已保存但同步失败: %v", err)
		if cleanErr := tektonsync.NewSyncer(l.svcCtx).DeleteStepFromAllChannels(l.ctx, data, in.CreatedBy); cleanErr != nil {
			l.Errorf("Tekton步骤同步失败后远端清理失败: %v", cleanErr)
		}
		if deleteErr := l.svcCtx.StepTemplateModel.DeleteSoft(l.ctx, data.ID.Hex(), in.CreatedBy); deleteErr != nil {
			l.Errorf("Tekton步骤同步失败后回滚失败: %v", deleteErr)
		}
		return nil, errorx.Msg("Tekton 步骤同步失败：" + devopstypes.TrimMessage(err.Error()))
	}

	return &pb.IdResp{Id: data.ID.Hex()}, nil
}
