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

type DevopsConfigTypeUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 更新配置中心类型
func NewDevopsConfigTypeUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsConfigTypeUpdateLogic {
	return &DevopsConfigTypeUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsConfigTypeUpdateLogic) DevopsConfigTypeUpdate(req *types.UpdateDevopsConfigTypeRequest) error {
	_, err := l.svcCtx.ProjectRpc.ConfigTypeUpdate(l.ctx, &projectservice.UpdateConfigTypeReq{
		Id:            req.Id,
		ParentId:      req.ParentId,
		Name:          req.Name,
		StorageType:   req.StorageType,
		Description:   req.Description,
		SortOrder:     req.SortOrder,
		Status:        req.Status,
		UpdatedBy:     currentUsername(l.ctx),
		CurrentUserId: currentUserID(l.ctx),
		CurrentRoles:  currentRoles(l.ctx),
	})
	return err
}
