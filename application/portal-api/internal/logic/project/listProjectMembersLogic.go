// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package project

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListProjectMembersLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取项目成员
func NewListProjectMembersLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListProjectMembersLogic {
	return &ListProjectMembersLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListProjectMembersLogic) ListProjectMembers(req *types.PortalProjectMemberReq) (resp *types.PortalListProjectMembersResp, err error) {
	rpcResp, err := l.svcCtx.ProjectRpc.ListProjectMembers(l.ctx, &pb.PortalListProjectMembersReq{
		ProjectId: req.Id,
	})
	if err != nil {
		l.Errorf("获取项目成员失败: %v", err)
		return nil, err
	}

	items := make([]types.PortalProjectMember, 0, len(rpcResp.Data))
	for _, item := range rpcResp.Data {
		platformRoles := make([]types.PortalProjectMemberPlatformRole, 0, len(item.PlatformRoles))
		for _, platformRole := range item.PlatformRoles {
			platformRoles = append(platformRoles, types.PortalProjectMemberPlatformRole{
				PlatformId:   platformRole.PlatformId,
				PlatformCode: platformRole.PlatformCode,
				PlatformName: platformRole.PlatformName,
				Role:         platformRole.Role,
			})
		}
		items = append(items, types.PortalProjectMember{
			Id:            item.Id,
			ProjectId:     item.ProjectId,
			UserId:        item.UserId,
			Username:      item.Username,
			Nickname:      item.Nickname,
			Role:          item.Role,
			CreatedBy:     item.CreatedBy,
			UpdatedBy:     item.UpdatedBy,
			CreatedAt:     item.CreatedAt,
			UpdatedAt:     item.UpdatedAt,
			PlatformRoles: platformRoles,
		})
	}

	return &types.PortalListProjectMembersResp{Data: items}, nil
}
