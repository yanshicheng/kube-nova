package project

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type ProjectOneSyncLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 同步项目相关数据
func NewProjectOneSyncLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ProjectOneSyncLogic {
	return &ProjectOneSyncLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ProjectOneSyncLogic) ProjectOneSync(req *types.SyncRequest) (resp string, err error) {
	// todo: add your logic here and delete this line

	return
}
