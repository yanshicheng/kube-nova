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

type DevopsProjectMavenConfigUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 更新项目 Maven 配置
func NewDevopsProjectMavenConfigUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsProjectMavenConfigUpdateLogic {
	return &DevopsProjectMavenConfigUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsProjectMavenConfigUpdateLogic) DevopsProjectMavenConfigUpdate(req *types.UpdateDevopsProjectMavenConfigRequest) error {
	_, err := l.svcCtx.ProjectRpc.ProjectMavenConfigUpdate(l.ctx, &projectservice.UpdateProjectMavenConfigReq{
		Id:            req.Id,
		Name:          req.Name,
		Content:       req.Content,
		Description:   req.Description,
		Status:        req.Status,
		UpdatedBy:     currentUsername(l.ctx),
		CurrentUserId: currentUserID(l.ctx),
		CurrentRoles:  currentRoles(l.ctx),
	})
	return err
}
