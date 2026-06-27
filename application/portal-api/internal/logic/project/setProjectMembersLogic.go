// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package project

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type SetProjectMembersLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 设置项目成员
func NewSetProjectMembersLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SetProjectMembersLogic {
	return &SetProjectMembersLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SetProjectMembersLogic) SetProjectMembers(req *types.PortalSetProjectMembersReq) error {
	if !isSuperAdmin(currentRoles(l.ctx)) {
		return errorx.Msg("无项目成员维护权限")
	}
	members := make([]*pb.PortalProjectMemberInput, 0, len(req.Members))
	for _, item := range req.Members {
		platformRoles := make([]*pb.PortalProjectMemberPlatformRoleInput, 0, len(item.PlatformRoles))
		for _, platformRole := range item.PlatformRoles {
			platformRoles = append(platformRoles, &pb.PortalProjectMemberPlatformRoleInput{
				PlatformId: platformRole.PlatformId,
				Role:       platformRole.Role,
			})
		}
		members = append(members, &pb.PortalProjectMemberInput{
			UserId:        item.UserId,
			Role:          item.Role,
			PlatformRoles: platformRoles,
		})
	}
	_, err := l.svcCtx.ProjectRpc.SetProjectMembers(l.ctx, &pb.PortalSetProjectMembersReq{
		ProjectId: req.Id,
		Members:   members,
		UpdatedBy: currentUsername(l.ctx),
	})
	if err != nil {
		l.Errorf("设置项目成员失败: %v", err)
		return err
	}

	return nil
}
