package pipelineconfigservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type PipelineEnvironmentDeleteLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPipelineEnvironmentDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PipelineEnvironmentDeleteLogic {
	return &PipelineEnvironmentDeleteLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *PipelineEnvironmentDeleteLogic) PipelineEnvironmentDelete(in *pb.DeleteByIdReq) (*pb.EmptyResp, error) {
	data, err := l.svcCtx.PipelineEnvModel.FindOne(l.ctx, in.Id)
	if err != nil {
		l.Errorf("流水线环境删除失败: %v", err)
		return nil, err
	}
	if err := ensureEnvironmentAccess(l.ctx, l.svcCtx, data, in.CurrentUserId, in.CurrentRoles); err != nil {
		l.Errorf("流水线环境删除失败: %v", err)
		return nil, err
	}
	if err := l.svcCtx.PipelineEnvModel.DeleteSoft(l.ctx, in.Id, in.UpdatedBy); err != nil {
		l.Errorf("流水线环境删除失败: %v", err)
		return nil, err
	}

	return &pb.EmptyResp{}, nil
}
