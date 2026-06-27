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

type DevopsProjectConfigUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 更新配置中心项目配置
func NewDevopsProjectConfigUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsProjectConfigUpdateLogic {
	return &DevopsProjectConfigUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsProjectConfigUpdateLogic) DevopsProjectConfigUpdate(req *types.UpdateDevopsProjectConfigRequest) error {
	_, err := l.svcCtx.ProjectRpc.ProjectConfigUpdate(l.ctx, &projectservice.UpdateProjectConfigReq{
		Id:            req.Id,
		TypeId:        req.TypeId,
		TypeCode:      req.TypeCode,
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
