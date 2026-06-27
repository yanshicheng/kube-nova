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

type DevopsProjectConfigListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 分页查询配置中心项目配置
func NewDevopsProjectConfigListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsProjectConfigListLogic {
	return &DevopsProjectConfigListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsProjectConfigListLogic) DevopsProjectConfigList(req *types.ListDevopsProjectConfigRequest) (resp *types.ListDevopsProjectConfigResponse, err error) {
	result, err := l.svcCtx.ProjectRpc.ProjectConfigList(l.ctx, &projectservice.ListProjectConfigReq{
		Page:          req.Page,
		PageSize:      req.PageSize,
		ProjectId:     req.ProjectId,
		TypeId:        req.TypeId,
		TypeCode:      req.TypeCode,
		Name:          req.Name,
		Code:          req.Code,
		Status:        req.Status,
		CurrentUserId: currentUserID(l.ctx),
		CurrentRoles:  currentRoles(l.ctx),
	})
	if err != nil {
		return nil, err
	}
	items := make([]types.DevopsProjectConfig, 0, len(result.Data))
	for _, item := range result.Data {
		items = append(items, projectConfigToType(item))
	}

	return &types.ListDevopsProjectConfigResponse{Items: items, Total: result.Total}, nil
}
