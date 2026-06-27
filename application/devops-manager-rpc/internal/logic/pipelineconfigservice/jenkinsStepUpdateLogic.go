package pipelineconfigservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type JenkinsStepUpdateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewJenkinsStepUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *JenkinsStepUpdateLogic {
	return &JenkinsStepUpdateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *JenkinsStepUpdateLogic) JenkinsStepUpdate(in *pb.UpdateStepTemplateReq) (*pb.EmptyResp, error) {
	exist, err := l.svcCtx.StepTemplateModel.FindOne(l.ctx, in.Id)
	if err != nil {
		l.Errorf("Jenkins步骤更新失败: %v", err)
		return nil, err
	}
	if exist.EngineType != jenkinsEngineType {
		l.Errorf("只能编辑 Jenkins 步骤")
		return nil, errorx.Msg("只能编辑 Jenkins 步骤")
	}
	category, err := l.svcCtx.StepCategoryModel.FindOne(l.ctx, in.CategoryId)
	if err != nil {
		l.Errorf("Jenkins步骤更新失败: %v", err)
		return nil, err
	}
	if category.Status != 1 {
		l.Errorf("步骤分类已停用")
		return nil, errorx.Msg("步骤分类已停用")
	}
	stageContent, err := validateJenkinsStepContent(in.StageContent, category.Code == stepCategoryCodePost)
	if err != nil {
		l.Errorf("Jenkins步骤更新失败: %v", err)
		return nil, err
	}
	stepType := normalizeStepType(in.Type)
	if err := validateStepType(stepType); err != nil {
		l.Errorf("Jenkins步骤更新失败: %v", err)
		return nil, err
	}
	params := stepParamsFromPb(in.Params)
	if err := validateStepParams(l.ctx, l.svcCtx, jenkinsEngineType, in.Id, params); err != nil {
		l.Errorf("Jenkins步骤更新失败: %v", err)
		return nil, err
	}
	artifactConfig := artifactConfigFromPb(in.ArtifactConfig)
	if err := validateStepArtifactConfig(artifactConfig); err != nil {
		l.Errorf("Jenkins步骤更新失败: %v", err)
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
		EngineType:             jenkinsEngineType,
		EngineChannelGroupCode: jenkinsEngineChannelGroupCode,
		EngineChannelType:      jenkinsEngineChannelType,
		StageContent:           stageContent,
		Params:                 params,
		ArtifactConfig:         artifactConfig,
		Status:                 in.Status,
		UpdatedBy:              in.UpdatedBy,
	}
	if err := l.svcCtx.StepTemplateModel.Update(l.ctx, data); err != nil {
		l.Errorf("Jenkins步骤更新失败: %v", err)
		return nil, err
	}

	return &pb.EmptyResp{}, nil
}
