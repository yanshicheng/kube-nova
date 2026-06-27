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

type DevopsChannelGetLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取渠道详情
func NewDevopsChannelGetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsChannelGetLogic {
	return &DevopsChannelGetLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsChannelGetLogic) DevopsChannelGet(req *types.DefaultStringIdRequest) (resp *types.DevopsChannel, err error) {
	result, err := l.svcCtx.ChannelRpc.ChannelGet(l.ctx, &channelservice.GetByIdReq{Id: req.Id})
	if err != nil {
		return nil, err
	}
	data := channelToType(result.Data)

	return &data, nil
}
