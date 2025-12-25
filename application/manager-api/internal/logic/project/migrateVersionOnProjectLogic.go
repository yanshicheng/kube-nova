package project

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type MigrateVersionOnProjectLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 同集群迁移版本
func NewMigrateVersionOnProjectLogic(ctx context.Context, svcCtx *svc.ServiceContext) *MigrateVersionOnProjectLogic {
	return &MigrateVersionOnProjectLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *MigrateVersionOnProjectLogic) MigrateVersionOnProject(req *types.MigrateVersionOnProjectRequest) (resp string, err error) {
	// todo: add your logic here and delete this line

	return
}
