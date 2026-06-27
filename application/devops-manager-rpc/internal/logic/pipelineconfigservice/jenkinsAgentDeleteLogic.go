package pipelineconfigservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type JenkinsAgentDeleteLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewJenkinsAgentDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *JenkinsAgentDeleteLogic {
	return &JenkinsAgentDeleteLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *JenkinsAgentDeleteLogic) JenkinsAgentDelete(in *pb.DeleteByIdReq) (*pb.EmptyResp, error) {
	if err := l.svcCtx.JenkinsAgentModel.DeleteSoft(l.ctx, in.Id, in.UpdatedBy); err != nil {
		l.Errorf("JenkinsAgent删除失败: %v", err)
		return nil, err
	}

	return &pb.EmptyResp{}, nil
}
