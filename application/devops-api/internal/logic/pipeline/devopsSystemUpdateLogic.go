// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package pipeline

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/client/pipelineconfigservice"

	"github.com/zeromicro/go-zero/core/logx"
)

type DevopsSystemUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 更新系统
func NewDevopsSystemUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsSystemUpdateLogic {
	return &DevopsSystemUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsSystemUpdateLogic) DevopsSystemUpdate(req *types.UpdateDevopsSystemRequest) error {
	_, err := l.svcCtx.PipelineRpc.SystemUpdate(l.ctx, &pipelineconfigservice.UpdateSystemReq{
		Id:            req.Id,
		Name:          req.Name,
		Description:   req.Description,
		OwnerUserIds:  req.OwnerUserIds,
		Status:        req.Status,
		UpdatedBy:     currentUsername(l.ctx),
		CurrentUserId: currentUserID(l.ctx),
		CurrentRoles:  currentRoles(l.ctx),
	})
	return err
}
