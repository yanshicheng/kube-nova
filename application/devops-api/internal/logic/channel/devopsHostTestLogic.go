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

type DevopsHostTestLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 测试主机连通性
func NewDevopsHostTestLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsHostTestLogic {
	return &DevopsHostTestLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsHostTestLogic) DevopsHostTest(req *types.DefaultStringIdRequest) (resp *types.TestDevopsChannelResponse, err error) {
	result, err := l.svcCtx.ChannelRpc.HostTest(l.ctx, &channelservice.GetByIdReq{Id: req.Id})
	if err != nil {
		return nil, err
	}

	return &types.TestDevopsChannelResponse{
		Success:      result.Success,
		HealthStatus: result.HealthStatus,
		Message:      result.Message,
		CheckedAt:    result.CheckedAt,
		Metadata:     result.Metadata,
	}, nil
}
