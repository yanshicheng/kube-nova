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

type DevopsHostUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 更新主机资产
func NewDevopsHostUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsHostUpdateLogic {
	return &DevopsHostUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsHostUpdateLogic) DevopsHostUpdate(req *types.UpdateDevopsHostRequest) error {
	_, err := l.svcCtx.ChannelRpc.HostUpdate(l.ctx, &channelservice.UpdateHostReq{
		Id:           req.Id,
		Name:         req.Name,
		Ip:           req.Ip,
		Port:         req.Port,
		CredentialId: req.CredentialId,
		Labels:       req.Labels,
		Description:  req.Description,
		Status:       req.Status,
		UpdatedBy:    currentUsername(l.ctx),
	})
	return err
}
