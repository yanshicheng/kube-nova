package pipelineconfigservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type SyncJenkinsCredentialsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSyncJenkinsCredentialsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SyncJenkinsCredentialsLogic {
	return &SyncJenkinsCredentialsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *SyncJenkinsCredentialsLogic) SyncJenkinsCredentials(in *pb.SyncJenkinsCredentialsReq) (*pb.SyncJenkinsCredentialsResp, error) {
	if err := ensureProjectAccess(l.ctx, l.svcCtx, in.ProjectId, in.CurrentUserId, in.CurrentRoles); err != nil {
		l.Errorf("同步 Jenkins 凭证失败: %v", err)
		return nil, err
	}
	data, err := syncJenkinsCredentials(l.ctx, l.svcCtx, in)
	if err != nil {
		l.Errorf("同步 Jenkins 凭证失败: %v", err)
		return nil, err
	}
	return &pb.SyncJenkinsCredentialsResp{Data: data}, nil
}
