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

type DevopsProjectMavenConfigDeleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 删除项目 Maven 配置
func NewDevopsProjectMavenConfigDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsProjectMavenConfigDeleteLogic {
	return &DevopsProjectMavenConfigDeleteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsProjectMavenConfigDeleteLogic) DevopsProjectMavenConfigDelete(req *types.DefaultStringIdRequest) error {
	_, err := l.svcCtx.ProjectRpc.ProjectMavenConfigDelete(l.ctx, &projectservice.DeleteByIdReq{
		Id:            req.Id,
		UpdatedBy:     currentUsername(l.ctx),
		CurrentUserId: currentUserID(l.ctx),
		CurrentRoles:  currentRoles(l.ctx),
	})
	return err
}
