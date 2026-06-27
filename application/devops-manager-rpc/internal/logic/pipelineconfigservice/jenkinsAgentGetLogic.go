package pipelineconfigservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type JenkinsAgentGetLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewJenkinsAgentGetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *JenkinsAgentGetLogic {
	return &JenkinsAgentGetLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *JenkinsAgentGetLogic) JenkinsAgentGet(in *pb.GetByIdReq) (*pb.GetJenkinsAgentResp, error) {
	data, err := l.svcCtx.JenkinsAgentModel.FindOne(l.ctx, in.Id)
	if err != nil {
		l.Errorf("JenkinsAgent查询详情失败: %v", err)
		return nil, err
	}

	return &pb.GetJenkinsAgentResp{Data: jenkinsAgentToPb(l.ctx, l.svcCtx, data)}, nil
}
