package pipelineconfigservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type PipelineTemplateStatusLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPipelineTemplateStatusLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PipelineTemplateStatusLogic {
	return &PipelineTemplateStatusLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *PipelineTemplateStatusLogic) PipelineTemplateStatus(in *pb.UpdateStatusReq) (*pb.EmptyResp, error) {
	if in.Status != 0 && in.Status != 1 {
		l.Errorf("状态不支持")
		return nil, errorx.Msg("状态不支持")
	}
	exist, err := l.svcCtx.PipelineTemplateModel.FindOne(l.ctx, in.Id)
	if err != nil {
		l.Errorf("流水线模板更新状态失败: %v", err)
		return nil, err
	}
	if err := ensureTemplateWriteAccess(l.ctx, l.svcCtx, exist, in.CurrentUserId, in.CurrentRoles); err != nil {
		l.Errorf("流水线模板更新状态失败: %v", err)
		return nil, err
	}
	if err := l.svcCtx.PipelineTemplateModel.UpdateStatus(l.ctx, in.Id, in.Status, in.UpdatedBy); err != nil {
		l.Errorf("流水线模板更新状态失败: %v", err)
		return nil, err
	}

	return &pb.EmptyResp{}, nil
}
