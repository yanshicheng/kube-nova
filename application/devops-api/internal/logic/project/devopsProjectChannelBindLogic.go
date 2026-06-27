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

type DevopsProjectChannelBindLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 绑定项目构建渠道
func NewDevopsProjectChannelBindLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsProjectChannelBindLogic {
	return &DevopsProjectChannelBindLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsProjectChannelBindLogic) DevopsProjectChannelBind(req *types.BindDevopsProjectChannelsRequest) error {
	_, err := l.svcCtx.ProjectRpc.ProjectChannelBind(l.ctx, &projectservice.BindProjectChannelsReq{
		ProjectId:                req.Id,
		ChannelGroupCode:         req.ChannelGroupCode,
		ChannelIds:               req.ChannelIds,
		DefaultChannelId:         req.DefaultChannelId,
		UsageScope:               req.UsageScope,
		AllowUseGlobalCredential: req.AllowUseGlobalCredential,
		ProjectCredentialId:      req.ProjectCredentialId,
		BindingConfig:            req.BindingConfig,
		UpdatedBy:                currentUsername(l.ctx),
	})

	return err
}
