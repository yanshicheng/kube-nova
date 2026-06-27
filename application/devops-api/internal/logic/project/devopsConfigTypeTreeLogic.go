// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package project

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/client/projectservice"

	"github.com/zeromicro/go-zero/core/logx"
)

type DevopsConfigTypeTreeLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 查询配置中心类型树
func NewDevopsConfigTypeTreeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsConfigTypeTreeLogic {
	return &DevopsConfigTypeTreeLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsConfigTypeTreeLogic) DevopsConfigTypeTree(req *types.DevopsConfigTypeTreeRequest) (resp *types.DevopsConfigTypeTreeResponse, err error) {
	result, err := l.svcCtx.ProjectRpc.ConfigTypeTree(l.ctx, &projectservice.ConfigTypeTreeReq{
		Status: req.Status,
	})
	if err != nil {
		return nil, err
	}
	items := make([]types.DevopsConfigType, 0, len(result.Data))
	for _, item := range result.Data {
		items = append(items, configTypeToType(item))
	}

	return &types.DevopsConfigTypeTreeResponse{Items: items}, nil
}
