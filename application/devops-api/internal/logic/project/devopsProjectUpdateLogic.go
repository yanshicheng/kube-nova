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

type DevopsProjectUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 更新 DevOps 项目
func NewDevopsProjectUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsProjectUpdateLogic {
	return &DevopsProjectUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsProjectUpdateLogic) DevopsProjectUpdate(req *types.UpdateDevopsProjectRequest) error {
	_, err := l.svcCtx.ProjectRpc.ProjectUpdate(l.ctx, &projectservice.UpdateProjectReq{
		Id:                     req.Id,
		Name:                   req.Name,
		Description:            req.Description,
		DefaultEngineChannelId: req.DefaultEngineChannelId,
		BuildChannelIds:        req.BuildChannelIds,
		Status:                 req.Status,
		ExtraConfig:            req.ExtraConfig,
		UpdatedBy:              currentUsername(l.ctx),
	})
	return err
}
