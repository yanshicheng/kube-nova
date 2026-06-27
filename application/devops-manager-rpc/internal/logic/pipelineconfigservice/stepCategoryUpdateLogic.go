package pipelineconfigservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type StepCategoryUpdateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewStepCategoryUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *StepCategoryUpdateLogic {
	return &StepCategoryUpdateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *StepCategoryUpdateLogic) StepCategoryUpdate(in *pb.UpdateStepCategoryReq) (*pb.EmptyResp, error) {
	exist, err := l.svcCtx.StepCategoryModel.FindOne(l.ctx, in.Id)
	if err != nil {
		l.Errorf("步骤分类更新失败: %v", err)
		return nil, err
	}
	data := &model.DevopsStepCategory{
		ID:          exist.ID,
		Name:        in.Name,
		Code:        exist.Code,
		Description: in.Description,
		Icon:        in.Icon,
		IconColor:   in.IconColor,
		SortOrder:   in.SortOrder,
		Status:      in.Status,
		UpdatedBy:   in.UpdatedBy,
	}
	if err := l.svcCtx.StepCategoryModel.Update(l.ctx, data); err != nil {
		l.Errorf("步骤分类更新失败: %v", err)
		return nil, err
	}

	return &pb.EmptyResp{}, nil
}
