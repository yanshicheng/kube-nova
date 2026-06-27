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

type DevopsCredentialUsageLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 查询凭据使用资源
func NewDevopsCredentialUsageLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsCredentialUsageLogic {
	return &DevopsCredentialUsageLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsCredentialUsageLogic) DevopsCredentialUsage(req *types.DefaultStringIdRequest) (resp *types.DevopsCredentialUsageResponse, err error) {
	result, err := l.svcCtx.ChannelRpc.CredentialUsage(l.ctx, &channelservice.GetByIdReq{
		Id:            req.Id,
		CurrentUserId: currentUserID(l.ctx),
		CurrentRoles:  currentRoles(l.ctx),
	})
	if err != nil {
		return nil, err
	}
	items := make([]types.DevopsCredentialUsage, 0, len(result.Data))
	for _, item := range result.Data {
		items = append(items, credentialUsageToType(item))
	}

	return &types.DevopsCredentialUsageResponse{Items: items, Total: result.Total}, nil
}
