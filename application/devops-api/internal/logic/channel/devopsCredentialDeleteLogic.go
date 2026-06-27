// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package channel

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/client/channelservice"

	"github.com/zeromicro/go-zero/core/logx"
)

type DevopsCredentialDeleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 删除凭据
func NewDevopsCredentialDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsCredentialDeleteLogic {
	return &DevopsCredentialDeleteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsCredentialDeleteLogic) DevopsCredentialDelete(req *types.DefaultStringIdRequest) error {
	_, err := l.svcCtx.ChannelRpc.CredentialDelete(l.ctx, &channelservice.DeleteByIdReq{
		Id:            req.Id,
		UpdatedBy:     currentUsername(l.ctx),
		CurrentUserId: currentUserID(l.ctx),
		CurrentRoles:  currentRoles(l.ctx),
	})
	return err
}
