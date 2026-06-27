package pipelineconfigservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type PipelineEnvironmentGetLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPipelineEnvironmentGetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PipelineEnvironmentGetLogic {
	return &PipelineEnvironmentGetLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *PipelineEnvironmentGetLogic) PipelineEnvironmentGet(in *pb.GetByIdReq) (*pb.GetPipelineEnvironmentResp, error) {
	data, err := l.svcCtx.PipelineEnvModel.FindOne(l.ctx, in.Id)
	if err != nil {
		l.Errorf("流水线环境查询详情失败: %v", err)
		return nil, err
	}
	if err := ensureEnvironmentAccess(l.ctx, l.svcCtx, data, in.CurrentUserId, in.CurrentRoles); err != nil {
		l.Errorf("流水线环境查询详情失败: %v", err)
		return nil, err
	}

	return &pb.GetPipelineEnvironmentResp{Data: pipelineEnvironmentToPb(data)}, nil
}
