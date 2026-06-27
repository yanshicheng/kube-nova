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

type DevopsChannelExistingTestLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 测试已有渠道连通性
func NewDevopsChannelExistingTestLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsChannelExistingTestLogic {
	return &DevopsChannelExistingTestLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsChannelExistingTestLogic) DevopsChannelExistingTest(req *types.DefaultStringIdRequest) (resp *types.TestDevopsChannelResponse, err error) {
	result, err := l.svcCtx.ChannelRpc.ChannelTest(l.ctx, &channelservice.TestChannelReq{Id: req.Id})
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
