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

type DevopsProjectConfigDeleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 删除配置中心项目配置
func NewDevopsProjectConfigDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsProjectConfigDeleteLogic {
	return &DevopsProjectConfigDeleteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsProjectConfigDeleteLogic) DevopsProjectConfigDelete(req *types.DefaultStringIdRequest) error {
	_, err := l.svcCtx.ProjectRpc.ProjectConfigDelete(l.ctx, &projectservice.DeleteByIdReq{
		Id:            req.Id,
		UpdatedBy:     currentUsername(l.ctx),
		CurrentUserId: currentUserID(l.ctx),
		CurrentRoles:  currentRoles(l.ctx),
	})
	return err
}
