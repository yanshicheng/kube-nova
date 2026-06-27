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

type DevopsConfigTypeListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 分页查询配置中心类型
func NewDevopsConfigTypeListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsConfigTypeListLogic {
	return &DevopsConfigTypeListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsConfigTypeListLogic) DevopsConfigTypeList(req *types.ListDevopsConfigTypeRequest) (resp *types.DevopsConfigTypeListResponse, err error) {
	result, err := l.svcCtx.ProjectRpc.ConfigTypeList(l.ctx, &projectservice.ListConfigTypeReq{
		Page:     req.Page,
		PageSize: req.PageSize,
		ParentId: req.ParentId,
		Name:     req.Name,
		Code:     req.Code,
		Status:   req.Status,
	})
	if err != nil {
		return nil, err
	}
	items := make([]types.DevopsConfigType, 0, len(result.Data))
	for _, item := range result.Data {
		items = append(items, configTypeToType(item))
	}

	return &types.DevopsConfigTypeListResponse{Items: items, Total: result.Total}, nil
}
