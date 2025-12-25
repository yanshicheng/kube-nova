package project

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/zeromicro/go-zero/core/logx"
)

type ClusterAllSyncLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewClusterAllSyncLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ClusterAllSyncLogic {
	return &ClusterAllSyncLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ClusterAllSyncLogic) ClusterAllSync() (resp string, err error) {
	// todo: add your logic here and delete this line

	return
}
