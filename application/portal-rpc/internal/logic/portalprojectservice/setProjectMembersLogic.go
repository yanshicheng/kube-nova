package portalprojectservicelogic

import (
	"context"
	"strings"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type SetProjectMembersLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSetProjectMembersLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SetProjectMembersLogic {
	return &SetProjectMembersLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *SetProjectMembersLogic) SetProjectMembers(in *pb.PortalSetProjectMembersReq) (*pb.PortalSetProjectMembersResp, error) {
	if in.ProjectId == 0 {
		return nil, errorx.Msg("项目ID不能为空")
	}
	project, err := l.svcCtx.OnecProjectModel.FindOne(l.ctx, in.ProjectId)
	if err != nil {
		l.Errorf("设置项目成员失败，项目不存在: %v", err)
		return nil, errorx.Msg("项目不存在")
	}
	if project.IsSystem == 1 {
		return nil, errorx.Msg("平台项目不允许维护成员")
	}

	seen := make(map[uint64]struct{}, len(in.Members))
	members := make([]*model.ProjectMemberBinding, 0, len(in.Members))
	memberPlatformRoles := make([]*model.ProjectMemberPlatformRole, 0, len(in.Members))
	for _, item := range in.Members {
		if item.UserId == 0 {
			return nil, errorx.Msg("项目成员用户不能为空")
		}
		if _, ok := seen[item.UserId]; ok {
			continue
		}
		if _, err := l.svcCtx.SysUser.FindOne(l.ctx, item.UserId); err != nil {
			l.Errorf("设置项目成员失败，用户不存在: userId=%d, err=%v", item.UserId, err)
			return nil, errorx.Msg("项目成员用户不存在")
		}
		role := strings.TrimSpace(item.Role)
		if role == "" {
			role = "member"
		}
		if !isAllowedPortalProjectRole(role) {
			return nil, errorx.Msg("项目成员角色不合法")
		}
		seen[item.UserId] = struct{}{}
		members = append(members, &model.ProjectMemberBinding{
			ProjectId: in.ProjectId,
			UserId:    item.UserId,
			Role:      role,
			CreatedBy: in.UpdatedBy,
			UpdatedBy: in.UpdatedBy,
		})
		platformRoles, err := l.buildMemberPlatformRoles(in.ProjectId, item.UserId, role, item.PlatformRoles)
		if err != nil {
			return nil, err
		}
		memberPlatformRoles = append(memberPlatformRoles, platformRoles...)
	}

	if err := l.svcCtx.ProjectMemberPlatformRole.ReplaceProjectMembersAndRoles(l.ctx, in.ProjectId, members, memberPlatformRoles, in.UpdatedBy); err != nil {
		l.Errorf("设置项目成员失败: %v", err)
		return nil, errorx.Msg("设置项目成员失败")
	}
	if isProjectBoundToDevops(l.ctx, l.svcCtx, in.ProjectId) {
		if syncErr := syncDevopsProjectMembers(l.ctx, l.svcCtx, project, in.UpdatedBy); syncErr != nil {
			l.Errorf("同步 DevOps 项目成员失败，portalProjectUuid: %s, 错误: %v", project.Uuid, syncErr)
			return nil, errorx.Msg("同步 DevOps 项目成员失败")
		}
	}

	return &pb.PortalSetProjectMembersResp{}, nil
}

func (l *SetProjectMembersLogic) buildMemberPlatformRoles(projectId, userId uint64, projectRole string, inputs []*pb.PortalProjectMemberPlatformRoleInput) ([]*model.ProjectMemberPlatformRole, error) {
	portalPlatform, err := getPortalPlatform(l.ctx, l.svcCtx)
	if err != nil {
		l.Errorf("查询门户平台失败: %v", err)
		return nil, err
	}
	if len(inputs) == 0 {
		bindings, err := l.svcCtx.ProjectPlatformBindingModel.SearchNoPage(l.ctx, "id", true, "`project_id` = ?", projectId)
		if err != nil {
			l.Errorf("查询项目平台失败: %v", err)
			return nil, errorx.Msg("查询项目平台失败")
		}
		roles := make([]*model.ProjectMemberPlatformRole, 0, len(bindings))
		for _, binding := range bindings {
			roles = append(roles, &model.ProjectMemberPlatformRole{
				ProjectId:  projectId,
				UserId:     userId,
				PlatformId: binding.PlatformId,
				Role:       projectRole,
			})
		}
		if len(roles) == 0 {
			roles = append(roles, &model.ProjectMemberPlatformRole{
				ProjectId:  projectId,
				UserId:     userId,
				PlatformId: portalPlatform.Id,
				Role:       projectRole,
			})
		}
		return roles, nil
	}

	seen := make(map[uint64]struct{}, len(inputs))
	roles := make([]*model.ProjectMemberPlatformRole, 0, len(inputs)+1)
	for _, item := range inputs {
		if item == nil || item.PlatformId == 0 {
			return nil, errorx.Msg("项目成员平台不能为空")
		}
		if _, ok := seen[item.PlatformId]; ok {
			continue
		}
		bound, err := l.svcCtx.ProjectPlatformBindingModel.IsProjectBoundToPlatform(l.ctx, projectId, item.PlatformId)
		if err != nil {
			l.Errorf("校验项目平台失败: %v", err)
			return nil, errorx.Msg("校验项目平台失败")
		}
		if !bound {
			return nil, errorx.Msg("项目成员平台未绑定")
		}
		role := strings.TrimSpace(item.Role)
		if role == "" {
			role = projectRole
		}
		if !isAllowedPortalProjectRole(role) {
			return nil, errorx.Msg("项目成员平台角色不合法")
		}
		seen[item.PlatformId] = struct{}{}
		roles = append(roles, &model.ProjectMemberPlatformRole{
			ProjectId:  projectId,
			UserId:     userId,
			PlatformId: item.PlatformId,
			Role:       role,
		})
	}
	if _, ok := seen[portalPlatform.Id]; !ok {
		roles = append(roles, &model.ProjectMemberPlatformRole{
			ProjectId:  projectId,
			UserId:     userId,
			PlatformId: portalPlatform.Id,
			Role:       projectRole,
		})
	}
	return roles, nil
}

func isAllowedPortalProjectRole(role string) bool {
	switch role {
	case "owner", "admin", "member":
		return true
	default:
		return false
	}
}
