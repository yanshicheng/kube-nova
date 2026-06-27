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

type StepCategoryCreateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewStepCategoryCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *StepCategoryCreateLogic {
	return &StepCategoryCreateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *StepCategoryCreateLogic) StepCategoryCreate(in *pb.CreateStepCategoryReq) (*pb.IdResp, error) {
	if err := validatePipelineCode(in.Code, "分类编码"); err != nil {
		l.Errorf("步骤分类创建失败: %v", err)
		return nil, err
	}
	if _, err := l.svcCtx.StepCategoryModel.FindOneByCode(l.ctx, in.Code); err == nil {
		l.Errorf("步骤分类编码已存在")
		return nil, errorx.Msg("步骤分类编码已存在")
	} else if !errors.Is(err, model.ErrNotFound) {
		l.Errorf("步骤分类创建失败: %v", err)
		return nil, err
	}
	data := &model.DevopsStepCategory{
		Name:        in.Name,
		Code:        in.Code,
		Description: in.Description,
		Icon:        in.Icon,
		IconColor:   in.IconColor,
		SortOrder:   in.SortOrder,
		Status:      in.Status,
		CreatedBy:   in.CreatedBy,
		UpdatedBy:   in.CreatedBy,
	}
	if data.Status == 0 {
		data.Status = 1
	}
	if err := l.svcCtx.StepCategoryModel.Insert(l.ctx, data); err != nil {
		l.Errorf("步骤分类创建失败: %v", err)
		return nil, err
	}

	return &pb.IdResp{Id: data.ID.Hex()}, nil
}
