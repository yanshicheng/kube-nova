package pipelineconfigservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type StepCategoryDeleteLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewStepCategoryDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *StepCategoryDeleteLogic {
	return &StepCategoryDeleteLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *StepCategoryDeleteLogic) StepCategoryDelete(in *pb.DeleteByIdReq) (*pb.EmptyResp, error) {
	total, err := l.svcCtx.StepTemplateModel.CountByCategory(l.ctx, in.Id)
	if err != nil {
		l.Errorf("步骤分类删除失败: %v", err)
		return nil, err
	}
	if total > 0 {
		l.Errorf("步骤分类已被步骤引用，不能删除")
		return nil, errorx.Msg("步骤分类已被步骤引用，不能删除")
	}
	if err := l.svcCtx.StepCategoryModel.DeleteSoft(l.ctx, in.Id, in.UpdatedBy); err != nil {
		l.Errorf("步骤分类删除失败: %v", err)
		return nil, err
	}

	return &pb.EmptyResp{}, nil
}
