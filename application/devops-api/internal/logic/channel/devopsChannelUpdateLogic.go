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

type DevopsChannelUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 更新渠道
func NewDevopsChannelUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsChannelUpdateLogic {
	return &DevopsChannelUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsChannelUpdateLogic) DevopsChannelUpdate(req *types.UpdateDevopsChannelRequest) error {
	_, err := l.svcCtx.ChannelRpc.ChannelUpdate(l.ctx, &channelservice.UpdateChannelReq{
		Id:                 req.Id,
		GroupId:            req.GroupId,
		Name:               req.Name,
		ChannelType:        req.ChannelType,
		Endpoint:           req.Endpoint,
		Description:        req.Description,
		GlobalCredentialId: req.GlobalCredentialId,
		CredentialId:       req.CredentialId,
		Config:             req.Config,
		Labels:             req.Labels,
		AuthType:           req.AuthType,
		Username:           req.Username,
		Password:           req.Password,
		Token:              req.Token,
		InsecureSkipTls:    req.InsecureSkipTls,
		Icon:               req.Icon,
		IconColor:          req.IconColor,
		Status:             req.Status,
		UpdatedBy:          currentUsername(l.ctx),
	})
	return err
}
