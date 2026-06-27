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

type DevopsHostListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 分页查询主机资产
func NewDevopsHostListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsHostListLogic {
	return &DevopsHostListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsHostListLogic) DevopsHostList(req *types.ListDevopsHostRequest) (resp *types.ListDevopsHostResponse, err error) {
	result, err := l.svcCtx.ChannelRpc.HostList(l.ctx, &channelservice.ListHostReq{
		Page:     req.Page,
		PageSize: req.PageSize,
		Name:     req.Name,
		Ip:       req.Ip,
		Status:   req.Status,
	})
	if err != nil {
		return nil, err
	}
	items := make([]types.DevopsHost, 0, len(result.Data))
	for _, item := range result.Data {
		items = append(items, hostToType(item))
	}

	return &types.ListDevopsHostResponse{Items: items, Total: result.Total}, nil
}
