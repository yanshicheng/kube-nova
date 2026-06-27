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

type DevopsChannelTypeListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 分页查询渠道类型
func NewDevopsChannelTypeListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsChannelTypeListLogic {
	return &DevopsChannelTypeListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsChannelTypeListLogic) DevopsChannelTypeList(req *types.ListDevopsChannelTypeRequest) (resp *types.ListDevopsChannelTypeResponse, err error) {
	result, err := l.svcCtx.ChannelRpc.ChannelTypeList(l.ctx, &channelservice.ListChannelTypeReq{
		Page:      req.Page,
		PageSize:  req.PageSize,
		Name:      req.Name,
		Code:      req.Code,
		GroupCode: req.GroupCode,
		Status:    req.Status,
	})
	if err != nil {
		return nil, err
	}
	items := make([]types.DevopsChannelType, 0, len(result.Data))
	for _, item := range result.Data {
		items = append(items, channelTypeToType(item))
	}

	return &types.ListDevopsChannelTypeResponse{Items: items, Total: result.Total}, nil
}
