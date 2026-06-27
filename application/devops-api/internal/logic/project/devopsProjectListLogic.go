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

type DevopsProjectListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 分页查询 DevOps 项目
func NewDevopsProjectListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsProjectListLogic {
	return &DevopsProjectListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsProjectListLogic) DevopsProjectList(req *types.ListDevopsProjectRequest) (resp *types.ListDevopsProjectResponse, err error) {
	result, err := l.svcCtx.ProjectRpc.ProjectList(l.ctx, &projectservice.ListProjectReq{
		Page:          req.Page,
		PageSize:      req.PageSize,
		Name:          req.Name,
		Code:          req.Code,
		Status:        req.Status,
		CurrentUserId: currentUserID(l.ctx),
		CurrentRoles:  currentRoles(l.ctx),
	})
	if err != nil {
		return nil, err
	}
	items := make([]types.DevopsProject, 0, len(result.Data))
	for _, item := range result.Data {
		items = append(items, projectToType(item))
	}

	return &types.ListDevopsProjectResponse{Items: items, Total: result.Total}, nil
}
