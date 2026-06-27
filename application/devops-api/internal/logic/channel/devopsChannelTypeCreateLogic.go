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

type DevopsChannelTypeCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 创建渠道类型
func NewDevopsChannelTypeCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsChannelTypeCreateLogic {
	return &DevopsChannelTypeCreateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsChannelTypeCreateLogic) DevopsChannelTypeCreate(req *types.CreateDevopsChannelTypeRequest) (resp *types.IdResponse, err error) {
	result, err := l.svcCtx.ChannelRpc.ChannelTypeCreate(l.ctx, &channelservice.CreateChannelTypeReq{
		Code:             req.Code,
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
		IsSystem:         req.IsSystem,
		Status:           req.Status,
		CreatedBy:        currentUsername(l.ctx),
	})
	if err != nil {
		return nil, err
	}

	return &types.IdResponse{Id: result.Id}, nil
}
