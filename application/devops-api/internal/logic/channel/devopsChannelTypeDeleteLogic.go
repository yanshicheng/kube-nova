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

type DevopsChannelTypeDeleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 删除渠道类型
func NewDevopsChannelTypeDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsChannelTypeDeleteLogic {
	return &DevopsChannelTypeDeleteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsChannelTypeDeleteLogic) DevopsChannelTypeDelete(req *types.DefaultStringIdRequest) error {
	_, err := l.svcCtx.ChannelRpc.ChannelTypeDelete(l.ctx, &channelservice.DeleteByIdReq{
		Id:        req.Id,
		UpdatedBy: currentUsername(l.ctx),
	})
	return err
}
