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

type DevopsChannelGroupCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 创建渠道分组
func NewDevopsChannelGroupCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsChannelGroupCreateLogic {
	return &DevopsChannelGroupCreateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsChannelGroupCreateLogic) DevopsChannelGroupCreate(req *types.CreateDevopsChannelGroupRequest) (resp *types.IdResponse, err error) {
	result, err := l.svcCtx.ChannelRpc.ChannelGroupCreate(l.ctx, &channelservice.CreateChannelGroupReq{
		Name:                req.Name,
		Code:                req.Code,
		Description:         req.Description,
		SortOrder:           req.SortOrder,
		IsSystem:            req.IsSystem,
		Status:              req.Status,
		GroupType:           req.GroupType,
		AllowedChannelTypes: req.AllowedChannelTypes,
		Icon:                req.Icon,
		IconColor:           req.IconColor,
		CreatedBy:           currentUsername(l.ctx),
	})
	if err != nil {
		return nil, err
	}

	return &types.IdResponse{Id: result.Id}, nil
}
