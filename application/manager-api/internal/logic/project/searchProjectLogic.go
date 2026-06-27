package project

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type SearchProjectLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 搜索项目列表，支持分页和条件筛选
func NewSearchProjectLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SearchProjectLogic {
	return &SearchProjectLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SearchProjectLogic) SearchProject(req *types.SearchProjectRequest) (resp *types.SearchProjectResponse, err error) {
	// 设置默认分页参数
	if req.Page == 0 {
		req.Page = 1
	}
	if req.PageSize == 0 {
		req.PageSize = 10
	}

	userId := uint64(0)
	platformId := currentPlatformID(l.ctx)
	if !isSuperAdmin(currentRoles(l.ctx)) {
		userId = currentUserID(l.ctx)
		if userId == 0 {
			return &types.SearchProjectResponse{Items: []types.Project{}, Total: 0}, nil
		}
		if platformId == 0 {
			return &types.SearchProjectResponse{Items: []types.Project{}, Total: 0}, nil
		}
	}
	if isSuperAdmin(currentRoles(l.ctx)) && platformId == 0 {
		return &types.SearchProjectResponse{Items: []types.Project{}, Total: 0}, nil
	}

	result, err := l.svcCtx.ProjectRpc.ListProjects(l.ctx, &pb.PortalListProjectsReq{
		Page:       req.Page,
		PageSize:   req.PageSize,
		Name:       req.Name,
		Uuid:       req.Uuid,
		IsSystem:   0,
		UserId:     userId,
		PlatformId: platformId,
	})

	if err != nil {
		l.Errorf("搜索项目失败: %v", err)
		return nil, err
	}

	resp = &types.SearchProjectResponse{
		Items: projectsToType(result.Data),
		Total: result.Total,
	}

	return resp, nil
}
