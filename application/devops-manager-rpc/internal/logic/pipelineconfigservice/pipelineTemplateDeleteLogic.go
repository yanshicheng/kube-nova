package pipelineconfigservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type PipelineTemplateDeleteLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPipelineTemplateDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PipelineTemplateDeleteLogic {
	return &PipelineTemplateDeleteLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *PipelineTemplateDeleteLogic) PipelineTemplateDelete(in *pb.DeleteByIdReq) (*pb.EmptyResp, error) {
	exist, err := l.svcCtx.PipelineTemplateModel.FindOne(l.ctx, in.Id)
	if err != nil {
		l.Errorf("流水线模板删除失败: %v", err)
		return nil, err
	}
	if err := ensureTemplateWriteAccess(l.ctx, l.svcCtx, exist, in.CurrentUserId, in.CurrentRoles); err != nil {
		l.Errorf("流水线模板删除失败: %v", err)
		return nil, err
	}
	if err := l.svcCtx.PipelineTemplateModel.DeleteSoft(l.ctx, in.Id, in.UpdatedBy); err != nil {
		l.Errorf("流水线模板删除失败: %v", err)
		return nil, err
	}

	return &pb.EmptyResp{}, nil
}
