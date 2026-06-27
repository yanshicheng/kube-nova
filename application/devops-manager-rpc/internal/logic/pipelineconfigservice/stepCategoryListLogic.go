package pipelineconfigservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type StepCategoryListLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewStepCategoryListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *StepCategoryListLogic {
	return &StepCategoryListLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *StepCategoryListLogic) StepCategoryList(in *pb.ListStepCategoryReq) (*pb.ListStepCategoryResp, error) {
	data, total, err := l.svcCtx.StepCategoryModel.List(l.ctx, model.DevopsStepCategoryListFilter{
		Page:     in.Page,
		PageSize: in.PageSize,
		Name:     in.Name,
		Code:     in.Code,
		Status:   in.Status,
	})
	if err != nil {
		l.Errorf("步骤分类查询列表失败: %v", err)
		return nil, err
	}
	items := make([]*pb.DevopsStepCategory, 0, len(data))
	for _, item := range data {
		items = append(items, stepCategoryToPb(item))
	}

	return &pb.ListStepCategoryResp{Data: items, Total: total}, nil
}
