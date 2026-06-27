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

type DevopsProjectChannelUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 更新项目渠道绑定
func NewDevopsProjectChannelUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsProjectChannelUpdateLogic {
	return &DevopsProjectChannelUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsProjectChannelUpdateLogic) DevopsProjectChannelUpdate(req *types.UpdateDevopsProjectChannelBindingRequest) error {
	_, err := l.svcCtx.ProjectRpc.ProjectChannelUpdate(l.ctx, &projectservice.UpdateProjectChannelBindingReq{
		Id:                       req.Id,
		IsDefault:                req.IsDefault,
		AllowUseGlobalCredential: req.AllowUseGlobalCredential,
		ProjectCredentialId:      req.ProjectCredentialId,
		BindingConfig:            req.BindingConfig,
		Status:                   req.Status,
		UpdatedBy:                currentUsername(l.ctx),
	})
	return err
}
