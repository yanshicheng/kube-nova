package projectservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type SyncProjectInfoLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSyncProjectInfoLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SyncProjectInfoLogic {
	return &SyncProjectInfoLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *SyncProjectInfoLogic) SyncProjectInfo(in *pb.DevopsSyncProjectInfoReq) (*pb.EmptyResp, error) {
	_, err := l.svcCtx.ProjectModel.FindOneByPortalUuid(l.ctx, in.PortalProjectUuid)
	if err != nil {
		l.Infof("DevOps 项目不存在，跳过同步，portalProjectUuid: %s", in.PortalProjectUuid)
		return &pb.EmptyResp{}, nil
	}

	err = l.svcCtx.ProjectModel.UpdateNameByPortalUuid(l.ctx, in.PortalProjectUuid, in.Name, in.Description, "system")
	if err != nil {
		l.Errorf("同步 DevOps 项目信息失败，portalProjectUuid: %s, 错误: %v", in.PortalProjectUuid, err)
		return nil, err
	}

	l.Infof("同步 DevOps 项目信息成功，portalProjectUuid: %s, Name: %s", in.PortalProjectUuid, in.Name)
	return &pb.EmptyResp{}, nil
}
