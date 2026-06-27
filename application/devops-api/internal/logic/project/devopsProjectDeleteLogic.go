// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package project

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/client/projectservice"

	"github.com/zeromicro/go-zero/core/logx"
)

type DevopsProjectDeleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 删除 DevOps 项目
func NewDevopsProjectDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsProjectDeleteLogic {
	return &DevopsProjectDeleteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsProjectDeleteLogic) DevopsProjectDelete(req *types.DefaultStringIdRequest) error {
	_, err := l.svcCtx.ProjectRpc.ProjectDelete(l.ctx, &projectservice.DeleteByIdReq{
		Id:        req.Id,
		UpdatedBy: currentUsername(l.ctx),
	})
	return err
}
