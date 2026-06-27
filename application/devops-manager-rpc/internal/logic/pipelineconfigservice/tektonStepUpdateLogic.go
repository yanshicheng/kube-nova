package pipelineconfigservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/tektonsync"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"
	devopstekton "github.com/yanshicheng/kube-nova/common/devops/tekton"
	devopstypes "github.com/yanshicheng/kube-nova/common/devops/types"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type TektonStepUpdateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewTektonStepUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TektonStepUpdateLogic {
	return &TektonStepUpdateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *TektonStepUpdateLogic) TektonStepUpdate(in *pb.UpdateStepTemplateReq) (*pb.EmptyResp, error) {
	exist, err := l.svcCtx.StepTemplateModel.FindOne(l.ctx, in.Id)
	if err != nil {
		l.Errorf("Tekton步骤更新失败: %v", err)
		return nil, err
	}
	if exist.EngineType != tektonEngineType {
		l.Errorf("只能编辑 Tekton 步骤")
		return nil, errorx.Msg("只能编辑 Tekton 步骤")
	}
	category, err := l.svcCtx.StepCategoryModel.FindOne(l.ctx, in.CategoryId)
	if err != nil {
		l.Errorf("Tekton步骤更新失败: %v", err)
		return nil, err
	}
	if category.Status != 1 {
		l.Errorf("步骤分类已停用")
		return nil, errorx.Msg("步骤分类已停用")
	}
	stageContent, err := validateTektonStepContent(in.StageContent, exist.Code)
	if err != nil {
		l.Errorf("Tekton步骤更新失败: %v", err)
		return nil, err
	}
	stepType := normalizeStepType(in.Type)
	if stepType != stepTypeTask {
		l.Errorf("Tekton 步骤只支持任务类型")
		return nil, errorx.Msg("Tekton 步骤只支持任务类型")
	}
	taskParams, taskResults, taskWorkspaces, err := tektonTaskContractsFromContent(stageContent)
	if err != nil {
		l.Errorf("Tekton步骤更新失败: %v", err)
		return nil, err
	}
	taskParams = mergeTektonTaskParamStepContracts(taskParams, in.Params)
	taskParams = mergeTektonTaskParamContracts(taskParams, in.TaskParams)
	params := stepParamsFromTektonTaskParams(taskParams, stepParamsFromPb(in.Params))
	if err := validateStepParams(l.ctx, l.svcCtx, tektonEngineType, in.Id, params); err != nil {
		l.Errorf("Tekton步骤更新失败: %v", err)
		return nil, err
	}
	if err := validateTektonStepParamMappings(stageContent, params); err != nil {
		l.Errorf("Tekton步骤更新失败: %v", err)
		return nil, err
	}
	artifactConfig := artifactConfigFromPb(in.ArtifactConfig)
	if err := validateStepArtifactConfig(artifactConfig); err != nil {
		l.Errorf("Tekton步骤更新失败: %v", err)
		return nil, err
	}
	data := &model.DevopsStepTemplate{
		ID:                     exist.ID,
		Name:                   in.Name,
		Code:                   exist.Code,
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
		UpdatedBy:              in.UpdatedBy,
	}
	syncer := tektonsync.NewSyncer(l.svcCtx)
	if data.Status == 1 {
		if err := syncer.SyncStepToAllChannels(l.ctx, data, in.UpdatedBy); err != nil {
			l.Errorf("Tekton步骤同步失败: %v", err)
			if exist.Status == 1 {
				if restoreErr := syncer.SyncStepToAllChannels(l.ctx, exist, in.UpdatedBy); restoreErr != nil {
					l.Errorf("Tekton步骤同步失败后恢复旧资源失败: %v", restoreErr)
				}
			} else if cleanErr := syncer.DeleteStepFromAllChannels(l.ctx, data, in.UpdatedBy); cleanErr != nil {
				l.Errorf("Tekton步骤同步失败后远端清理失败: %v", cleanErr)
			}
			return nil, errorx.Msg("Tekton 步骤同步失败：" + devopstypes.TrimMessage(err.Error()))
		}
		if tektonStepResourceNameChanged(exist.StageContent, stageContent) {
			if err := syncer.DeleteStepFromAllChannels(l.ctx, exist, in.UpdatedBy); err != nil {
				l.Errorf("Tekton步骤重命名后清理旧资源失败: %v", err)
				return nil, errorx.Msg("Tekton 步骤旧资源清理失败：" + devopstypes.TrimMessage(err.Error()))
			}
		}
	} else {
		if err := syncer.DeleteStepFromAllChannels(l.ctx, exist, in.UpdatedBy); err != nil {
			l.Errorf("Tekton步骤停用清理失败: %v", err)
			return nil, errorx.Msg("Tekton 步骤远端清理失败：" + devopstypes.TrimMessage(err.Error()))
		}
	}
	if err := l.svcCtx.StepTemplateModel.Update(l.ctx, data); err != nil {
		l.Errorf("Tekton步骤更新失败: %v", err)
		if exist.Status == 1 {
			if restoreErr := syncer.SyncStepToAllChannels(l.ctx, exist, in.UpdatedBy); restoreErr != nil {
				l.Errorf("Tekton步骤数据库更新失败后恢复旧资源失败: %v", restoreErr)
			}
		} else if cleanErr := syncer.DeleteStepFromAllChannels(l.ctx, data, in.UpdatedBy); cleanErr != nil {
			l.Errorf("Tekton步骤数据库更新失败后远端清理失败: %v", cleanErr)
		}
		return nil, err
	}

	return &pb.EmptyResp{}, nil
}

func tektonStepResourceNameChanged(oldContent, newContent string) bool {
	oldResource, oldErr := devopstekton.ParseStepResource(oldContent)
	newResource, newErr := devopstekton.ParseStepResource(newContent)
	return oldErr == nil && newErr == nil && oldResource.Name != "" && oldResource.Name != newResource.Name
}
