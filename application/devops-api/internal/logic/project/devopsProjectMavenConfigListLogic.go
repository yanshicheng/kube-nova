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

type DevopsProjectMavenConfigListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 分页查询项目 Maven 配置
func NewDevopsProjectMavenConfigListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsProjectMavenConfigListLogic {
	return &DevopsProjectMavenConfigListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsProjectMavenConfigListLogic) DevopsProjectMavenConfigList(req *types.ListDevopsProjectMavenConfigRequest) (resp *types.ListDevopsProjectMavenConfigResponse, err error) {
	result, err := l.svcCtx.ProjectRpc.ProjectMavenConfigList(l.ctx, &projectservice.ListProjectMavenConfigReq{
		Page:          req.Page,
		PageSize:      req.PageSize,
		ProjectId:     req.ProjectId,
		Name:          req.Name,
		Code:          req.Code,
		Status:        req.Status,
		CurrentUserId: currentUserID(l.ctx),
		CurrentRoles:  currentRoles(l.ctx),
	})
	if err != nil {
		return nil, err
	}
	items := make([]types.DevopsProjectMavenConfig, 0, len(result.Data))
	for _, item := range result.Data {
		items = append(items, projectMavenConfigToType(item))
	}

	return &types.ListDevopsProjectMavenConfigResponse{Items: items, Total: result.Total}, nil
}
