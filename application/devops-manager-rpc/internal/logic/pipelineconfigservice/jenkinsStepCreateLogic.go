package pipelineconfigservicelogic

import (
	"context"
	"errors"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type JenkinsStepCreateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewJenkinsStepCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *JenkinsStepCreateLogic {
	return &JenkinsStepCreateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *JenkinsStepCreateLogic) JenkinsStepCreate(in *pb.CreateStepTemplateReq) (*pb.IdResp, error) {
	if err := validatePipelineCode(in.Code, "步骤编码"); err != nil {
		l.Errorf("Jenkins步骤创建失败: %v", err)
		return nil, err
	}
	if _, err := l.svcCtx.StepTemplateModel.FindOneByCode(l.ctx, jenkinsEngineType, in.Code); err == nil {
		l.Errorf("Jenkins 步骤编码已存在")
		return nil, errorx.Msg("Jenkins 步骤编码已存在")
	} else if !errors.Is(err, model.ErrNotFound) {
		l.Errorf("Jenkins步骤创建失败: %v", err)
		return nil, err
	}
	category, err := l.svcCtx.StepCategoryModel.FindOne(l.ctx, in.CategoryId)
	if err != nil {
		l.Errorf("Jenkins步骤创建失败: %v", err)
		return nil, err
	}
	if category.Status != 1 {
		l.Errorf("步骤分类已停用")
		return nil, errorx.Msg("步骤分类已停用")
	}
	stageContent, err := validateJenkinsStepContent(in.StageContent, category.Code == stepCategoryCodePost)
	if err != nil {
		l.Errorf("Jenkins步骤创建失败: %v", err)
		return nil, err
	}
	stepType := normalizeStepType(in.Type)
	if err := validateStepType(stepType); err != nil {
		l.Errorf("Jenkins步骤创建失败: %v", err)
		return nil, err
	}
	params := stepParamsFromPb(in.Params)
	if err := validateStepParams(l.ctx, l.svcCtx, jenkinsEngineType, "", params); err != nil {
		l.Errorf("Jenkins步骤创建失败: %v", err)
		return nil, err
	}
	artifactConfig := artifactConfigFromPb(in.ArtifactConfig)
	if err := validateStepArtifactConfig(artifactConfig); err != nil {
		l.Errorf("Jenkins步骤创建失败: %v", err)
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
		EngineType:             jenkinsEngineType,
		EngineChannelGroupCode: jenkinsEngineChannelGroupCode,
		EngineChannelType:      jenkinsEngineChannelType,
		StageContent:           stageContent,
		Params:                 params,
		ArtifactConfig:         artifactConfig,
		Status:                 in.Status,
		CreatedBy:              in.CreatedBy,
		UpdatedBy:              in.CreatedBy,
	}
	if data.Status == 0 {
		data.Status = 1
	}
	if err := l.svcCtx.StepTemplateModel.Insert(l.ctx, data); err != nil {
		l.Errorf("Jenkins步骤创建失败: %v", err)
		return nil, err
	}

	return &pb.IdResp{Id: data.ID.Hex()}, nil
}
