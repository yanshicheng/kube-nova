package project

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type ClusterOneSyncLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewClusterOneSyncLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ClusterOneSyncLogic {
	return &ClusterOneSyncLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ClusterOneSyncLogic) ClusterOneSync(req *types.SyncClusterRequest) (resp string, err error) {
	// todo: add your logic here and delete this line

	return
}
