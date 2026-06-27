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

type DevopsProjectChannelAddLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 新增项目渠道绑定
func NewDevopsProjectChannelAddLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsProjectChannelAddLogic {
	return &DevopsProjectChannelAddLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsProjectChannelAddLogic) DevopsProjectChannelAdd(req *types.AddDevopsProjectChannelRequest) error {
	_, err := l.svcCtx.ProjectRpc.ProjectChannelAdd(l.ctx, &projectservice.AddProjectChannelReq{
		ProjectId:                req.Id,
		ChannelId:                req.ChannelId,
		IsDefault:                req.IsDefault,
		AllowUseGlobalCredential: req.AllowUseGlobalCredential,
		ProjectCredentialId:      req.ProjectCredentialId,
		BindingConfig:            req.BindingConfig,
		CreatedBy:                currentUsername(l.ctx),
	})
	return err
}
