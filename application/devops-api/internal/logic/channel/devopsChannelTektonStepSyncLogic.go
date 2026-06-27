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

type DevopsChannelTektonStepSyncLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 同步 Tekton 步骤到渠道实例
func NewDevopsChannelTektonStepSyncLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsChannelTektonStepSyncLogic {
	return &DevopsChannelTektonStepSyncLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsChannelTektonStepSyncLogic) DevopsChannelTektonStepSync(req *types.DefaultStringIdRequest) error {
	_, err := l.svcCtx.ChannelRpc.ChannelTektonStepSync(l.ctx, &channelservice.DeleteByIdReq{
		Id:        req.Id,
		UpdatedBy: currentUsername(l.ctx),
	})
	return err
}
