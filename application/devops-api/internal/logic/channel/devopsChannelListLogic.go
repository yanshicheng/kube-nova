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

type DevopsChannelListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 分页查询渠道
func NewDevopsChannelListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsChannelListLogic {
	return &DevopsChannelListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsChannelListLogic) DevopsChannelList(req *types.ListDevopsChannelRequest) (resp *types.ListDevopsChannelResponse, err error) {
	result, err := l.svcCtx.ChannelRpc.ChannelList(l.ctx, &channelservice.ListChannelReq{
		Page:        req.Page,
		PageSize:    req.PageSize,
		Name:        req.Name,
		Code:        req.Code,
		GroupId:     req.GroupId,
		ChannelType: req.ChannelType,
		Status:      req.Status,
	})
	if err != nil {
		return nil, err
	}
	items := make([]types.DevopsChannel, 0, len(result.Data))
	for _, item := range result.Data {
		items = append(items, channelToType(item))
	}

	return &types.ListDevopsChannelResponse{Items: items, Total: result.Total}, nil
}
