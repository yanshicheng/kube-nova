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

type DevopsHostConfigTestLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 测试主机配置连通性
func NewDevopsHostConfigTestLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsHostConfigTestLogic {
	return &DevopsHostConfigTestLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsHostConfigTestLogic) DevopsHostConfigTest(req *types.TestDevopsHostRequest) (resp *types.TestDevopsChannelResponse, err error) {
	result, err := l.svcCtx.ChannelRpc.HostTestConfig(l.ctx, &channelservice.TestHostReq{
		Id:           req.Id,
		Name:         req.Name,
		Ip:           req.Ip,
		Port:         req.Port,
		CredentialId: req.CredentialId,
	})
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
