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

type DevopsCredentialGetLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取凭据详情
func NewDevopsCredentialGetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsCredentialGetLogic {
	return &DevopsCredentialGetLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsCredentialGetLogic) DevopsCredentialGet(req *types.DefaultStringIdRequest) (resp *types.DevopsCredential, err error) {
	result, err := l.svcCtx.ChannelRpc.CredentialGet(l.ctx, &channelservice.GetByIdReq{
		Id:            req.Id,
		CurrentUserId: currentUserID(l.ctx),
		CurrentRoles:  currentRoles(l.ctx),
	})
	if err != nil {
		return nil, err
	}
	data := credentialToType(result.Data)

	return &data, nil
}
