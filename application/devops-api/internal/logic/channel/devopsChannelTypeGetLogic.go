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

type DevopsChannelTypeGetLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取渠道类型详情
func NewDevopsChannelTypeGetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsChannelTypeGetLogic {
	return &DevopsChannelTypeGetLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsChannelTypeGetLogic) DevopsChannelTypeGet(req *types.DefaultStringIdRequest) (resp *types.DevopsChannelType, err error) {
	result, err := l.svcCtx.ChannelRpc.ChannelTypeGet(l.ctx, &channelservice.GetByIdReq{Id: req.Id})
	if err != nil {
		return nil, err
	}
	data := channelTypeToType(result.Data)

	return &data, nil
}
