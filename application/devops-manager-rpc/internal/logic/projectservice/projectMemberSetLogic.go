package projectservicelogic

import (
	"context"
	"strings"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type ProjectMemberSetLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewProjectMemberSetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ProjectMemberSetLogic {
	return &ProjectMemberSetLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ProjectMemberSetLogic) ProjectMemberSet(in *pb.SetProjectMembersReq) (*pb.EmptyResp, error) {
	if _, err := l.svcCtx.ProjectModel.FindOne(l.ctx, in.ProjectId); err != nil {
		l.Errorf("项目成员设置失败: %v", err)
		return nil, err
	}
	seen := make(map[uint64]struct{}, len(in.Members))
	members := make([]*model.DevopsProjectMember, 0, len(in.Members))
	for _, item := range in.Members {
		if item.UserId == 0 {
			l.Errorf("项目成员用户不能为空")
			return nil, errorx.Msg("项目成员用户不能为空")
		}
		if _, ok := seen[item.UserId]; ok {
			continue
		}
		role := strings.TrimSpace(item.Role)
		if role == "" {
			role = "developer"
		}
		if !isAllowedProjectRole(role) {
			l.Errorf("项目成员角色不合法")
			return nil, errorx.Msg("项目成员角色不合法")
		}
		seen[item.UserId] = struct{}{}
		members = append(members, &model.DevopsProjectMember{
			ProjectID: in.ProjectId,
			UserID:    item.UserId,
			Username:  strings.TrimSpace(item.Username),
			Nickname:  strings.TrimSpace(item.Nickname),
			Role:      role,
			Status:    1,
			CreatedBy: in.UpdatedBy,
			UpdatedBy: in.UpdatedBy,
		})
	}
	if err := l.svcCtx.ProjectMemberModel.DeleteSoftByProject(l.ctx, in.ProjectId, in.UpdatedBy); err != nil {
		l.Errorf("项目成员设置失败: %v", err)
		return nil, err
	}
	for _, member := range members {
		if err := l.svcCtx.ProjectMemberModel.Insert(l.ctx, member); err != nil {
			l.Errorf("项目成员设置失败: %v", err)
			return nil, err
		}
	}

	return &pb.EmptyResp{}, nil
}

func isAllowedProjectRole(role string) bool {
	switch role {
	case "owner", "maintainer", "developer", "viewer":
		return true
	default:
		return false
	}
}
