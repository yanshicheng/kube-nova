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

type DevopsChannelGroupGetLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取渠道分组详情
func NewDevopsChannelGroupGetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsChannelGroupGetLogic {
	return &DevopsChannelGroupGetLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsChannelGroupGetLogic) DevopsChannelGroupGet(req *types.DefaultStringIdRequest) (resp *types.DevopsChannelGroup, err error) {
	result, err := l.svcCtx.ChannelRpc.ChannelGroupGet(l.ctx, &channelservice.GetByIdReq{Id: req.Id})
	if err != nil {
		return nil, err
	}
	data := channelGroupToType(result.Data)

	return &data, nil
}
