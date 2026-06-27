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

type DevopsCredentialListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 分页查询凭据
func NewDevopsCredentialListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsCredentialListLogic {
	return &DevopsCredentialListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsCredentialListLogic) DevopsCredentialList(req *types.ListDevopsCredentialRequest) (resp *types.ListDevopsCredentialResponse, err error) {
	result, err := l.svcCtx.ChannelRpc.CredentialList(l.ctx, &channelservice.ListCredentialReq{
		Page:                  req.Page,
		PageSize:              req.PageSize,
		Name:                  req.Name,
		Code:                  req.Code,
		CredentialType:        req.CredentialType,
		Status:                req.Status,
		Scope:                 req.Scope,
		ProjectId:             req.ProjectId,
		BuildChannelBindingId: req.BuildChannelBindingId,
		ChannelGroupCode:      req.ChannelGroupCode,
		ChannelType:           req.ChannelType,
		CurrentUserId:         currentUserID(l.ctx),
		CurrentRoles:          currentRoles(l.ctx),
	})
	if err != nil {
		return nil, err
	}
	items := make([]types.DevopsCredential, 0, len(result.Data))
	for _, item := range result.Data {
		items = append(items, credentialToType(item))
	}

	return &types.ListDevopsCredentialResponse{Items: items, Total: result.Total}, nil
}
