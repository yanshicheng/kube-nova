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

type DevopsHostGetLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取主机资产详情
func NewDevopsHostGetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsHostGetLogic {
	return &DevopsHostGetLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsHostGetLogic) DevopsHostGet(req *types.DefaultStringIdRequest) (resp *types.DevopsHost, err error) {
	result, err := l.svcCtx.ChannelRpc.HostGet(l.ctx, &channelservice.GetByIdReq{Id: req.Id})
	if err != nil {
		return nil, err
	}
	data := hostToType(result.Data)

	return &data, nil
}
