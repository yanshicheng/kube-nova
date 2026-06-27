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

type DevopsChannelGroupListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 分页查询渠道分组
func NewDevopsChannelGroupListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsChannelGroupListLogic {
	return &DevopsChannelGroupListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsChannelGroupListLogic) DevopsChannelGroupList(req *types.ListDevopsChannelGroupRequest) (resp *types.ListDevopsChannelGroupResponse, err error) {
	result, err := l.svcCtx.ChannelRpc.ChannelGroupList(l.ctx, &channelservice.ListChannelGroupReq{
		Page:      req.Page,
		PageSize:  req.PageSize,
		Name:      req.Name,
		Code:      req.Code,
		Status:    req.Status,
		GroupType: req.GroupType,
	})
	if err != nil {
		return nil, err
	}
	items := make([]types.DevopsChannelGroup, 0, len(result.Data))
	for _, item := range result.Data {
		items = append(items, channelGroupToType(item))
	}

	return &types.ListDevopsChannelGroupResponse{Items: items, Total: result.Total}, nil
}
