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

type DevopsChannelGroupUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 更新渠道分组
func NewDevopsChannelGroupUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsChannelGroupUpdateLogic {
	return &DevopsChannelGroupUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsChannelGroupUpdateLogic) DevopsChannelGroupUpdate(req *types.UpdateDevopsChannelGroupRequest) error {
	_, err := l.svcCtx.ChannelRpc.ChannelGroupUpdate(l.ctx, &channelservice.UpdateChannelGroupReq{
		Id:                  req.Id,
		Name:                req.Name,
		Description:         req.Description,
		SortOrder:           req.SortOrder,
		Status:              req.Status,
		GroupType:           req.GroupType,
		AllowedChannelTypes: req.AllowedChannelTypes,
		Icon:                req.Icon,
		IconColor:           req.IconColor,
		UpdatedBy:           currentUsername(l.ctx),
	})
	return err
}
