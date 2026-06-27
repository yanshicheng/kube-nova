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

type DevopsChannelTypeUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 更新渠道类型
func NewDevopsChannelTypeUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsChannelTypeUpdateLogic {
	return &DevopsChannelTypeUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsChannelTypeUpdateLogic) DevopsChannelTypeUpdate(req *types.UpdateDevopsChannelTypeRequest) error {
	_, err := l.svcCtx.ChannelRpc.ChannelTypeUpdate(l.ctx, &channelservice.UpdateChannelTypeReq{
		Id:               req.Id,
		Name:             req.Name,
		GroupCode:        req.GroupCode,
		CredentialTypes:  req.CredentialTypes,
		ConfigSchema:     req.ConfigSchema,
		MappingFields:    req.MappingFields,
		TestStrategy:     req.TestStrategy,
		Icon:             req.Icon,
		IconColor:        req.IconColor,
		ConnectionMode:   req.ConnectionMode,
		MetadataStrategy: req.MetadataStrategy,
		Status:           req.Status,
		UpdatedBy:        currentUsername(l.ctx),
	})
	return err
}
