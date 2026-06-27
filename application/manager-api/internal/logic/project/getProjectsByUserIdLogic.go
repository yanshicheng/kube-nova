package project

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetProjectsByUserIdLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 根据用户ID查询项目列表
func NewGetProjectsByUserIdLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetProjectsByUserIdLogic {
	return &GetProjectsByUserIdLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetProjectsByUserIdLogic) GetProjectsByUserId(req *types.GetProjectsByUserIdRequest) (resp []types.Project, err error) {
	userId := uint64(0)
	platformId := currentPlatformID(l.ctx)
	if !isSuperAdmin(currentRoles(l.ctx)) {
		userId = currentUserID(l.ctx)
		if userId == 0 {
			return []types.Project{}, nil
		}
		if platformId == 0 {
			return []types.Project{}, nil
		}
	}
	if isSuperAdmin(currentRoles(l.ctx)) && platformId == 0 {
		return []types.Project{}, nil
	}

	projects, err := l.svcCtx.ProjectRpc.ListProjects(l.ctx, &pb.PortalListProjectsReq{
		Page:       1,
		PageSize:   1000,
		Name:       req.Name,
		IsSystem:   0,
		UserId:     userId,
		PlatformId: platformId,
	})
	if err != nil {
		l.Errorf("查询用户 [ID:%d] 的项目列表失败: %v", userId, err)
		return nil, err
	}

	return projectsToType(projects.Data), nil
}
