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

type DevopsProjectMemberListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 分页查询项目成员
func NewDevopsProjectMemberListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsProjectMemberListLogic {
	return &DevopsProjectMemberListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsProjectMemberListLogic) DevopsProjectMemberList(req *types.ListDevopsProjectMemberRequest) (resp *types.ListDevopsProjectMemberResponse, err error) {
	result, err := l.svcCtx.ProjectRpc.ProjectMemberList(l.ctx, &projectservice.ListProjectMemberReq{
		Page:      req.Page,
		PageSize:  req.PageSize,
		ProjectId: req.ProjectId,
		UserId:    req.UserId,
		Role:      req.Role,
		Status:    req.Status,
	})
	if err != nil {
		return nil, err
	}
	items := make([]types.DevopsProjectMember, 0, len(result.Data))
	for _, item := range result.Data {
		items = append(items, projectMemberToType(item))
	}

	return &types.ListDevopsProjectMemberResponse{Items: items, Total: result.Total}, nil
}
