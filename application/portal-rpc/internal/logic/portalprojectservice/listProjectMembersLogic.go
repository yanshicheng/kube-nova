package portalprojectservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListProjectMembersLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListProjectMembersLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListProjectMembersLogic {
	return &ListProjectMembersLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListProjectMembersLogic) ListProjectMembers(in *pb.PortalListProjectMembersReq) (*pb.PortalListProjectMembersResp, error) {
	if in.ProjectId == 0 {
		return nil, errorx.Msg("项目ID不能为空")
	}
	if _, err := l.svcCtx.OnecProjectModel.FindOne(l.ctx, in.ProjectId); err != nil {
		l.Errorf("查询项目成员失败，项目不存在: %v", err)
		return nil, errorx.Msg("项目不存在")
	}

	members, err := l.svcCtx.ProjectMemberBindingModel.ListByProject(l.ctx, in.ProjectId)
	if err != nil {
		l.Errorf("查询项目成员失败: %v", err)
		return nil, errorx.Msg("查询项目成员失败")
	}
	platformRoles, err := l.svcCtx.ProjectMemberPlatformRole.ListByProject(l.ctx, in.ProjectId)
	if err != nil {
		l.Errorf("查询项目成员平台授权失败: %v", err)
		return nil, errorx.Msg("查询项目成员失败")
	}
	roleMap := make(map[uint64][]*pb.PortalProjectMemberPlatformRole)
	for _, item := range platformRoles {
		roleMap[item.UserId] = append(roleMap[item.UserId], &pb.PortalProjectMemberPlatformRole{
			PlatformId:   item.PlatformId,
			PlatformCode: item.PlatformCode,
			PlatformName: item.PlatformName,
			Role:         item.Role,
		})
	}

	data := make([]*pb.PortalProjectMember, 0, len(members))
	for _, member := range members {
		data = append(data, &pb.PortalProjectMember{
			Id:            member.Id,
			ProjectId:     member.ProjectId,
			UserId:        member.UserId,
			Username:      member.Username,
			Nickname:      member.Nickname,
			Role:          member.Role,
			CreatedBy:     member.CreatedBy,
			UpdatedBy:     member.UpdatedBy,
			CreatedAt:     member.CreatedAt.Unix(),
			UpdatedAt:     member.UpdatedAt.Unix(),
			PlatformRoles: roleMap[member.UserId],
		})
	}

	return &pb.PortalListProjectMembersResp{Data: data}, nil
}
