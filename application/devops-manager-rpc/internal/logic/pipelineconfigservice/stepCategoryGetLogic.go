package pipelineconfigservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type StepCategoryGetLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewStepCategoryGetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *StepCategoryGetLogic {
	return &StepCategoryGetLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *StepCategoryGetLogic) StepCategoryGet(in *pb.GetByIdReq) (*pb.GetStepCategoryResp, error) {
	data, err := l.svcCtx.StepCategoryModel.FindOne(l.ctx, in.Id)
	if err != nil {
		l.Errorf("步骤分类查询详情失败: %v", err)
		return nil, err
	}

	return &pb.GetStepCategoryResp{Data: stepCategoryToPb(data)}, nil
}
