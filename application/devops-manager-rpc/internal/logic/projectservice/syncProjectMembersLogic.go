package projectservicelogic

import (
	"context"
	"strings"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type SyncProjectMembersLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSyncProjectMembersLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SyncProjectMembersLogic {
	return &SyncProjectMembersLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *SyncProjectMembersLogic) SyncProjectMembers(in *pb.DevopsSyncProjectMembersReq) (*pb.EmptyResp, error) {
	project, err := l.svcCtx.ProjectModel.FindOneByPortalUuid(l.ctx, in.PortalProjectUuid)
	if err != nil {
		l.Infof("DevOps 项目不存在，跳过成员同步，portalProjectUuid: %s", in.PortalProjectUuid)
		return &pb.EmptyResp{}, nil
	}

	updatedBy := in.UpdatedBy
	if updatedBy == "" {
		updatedBy = "system"
	}
	projectId := project.ID.Hex()
	if err := l.svcCtx.ProjectMemberModel.DeleteSoftByProject(l.ctx, projectId, updatedBy); err != nil {
		l.Errorf("同步 DevOps 项目成员失败，portalProjectUuid: %s, 错误: %v", in.PortalProjectUuid, err)
		return nil, err
	}

	seen := make(map[uint64]struct{}, len(in.Members))
	for _, item := range in.Members {
		if item.UserId == 0 {
			continue
		}
		if _, ok := seen[item.UserId]; ok {
			continue
		}
		role := normalizeDevopsProjectRole(item.Role)
		member := &model.DevopsProjectMember{
			ProjectID: projectId,
			UserID:    item.UserId,
			Username:  strings.TrimSpace(item.Username),
			Nickname:  strings.TrimSpace(item.Nickname),
			Role:      role,
			Status:    1,
			CreatedBy: updatedBy,
			UpdatedBy: updatedBy,
		}
		if err := l.svcCtx.ProjectMemberModel.Insert(l.ctx, member); err != nil {
			l.Errorf("同步 DevOps 项目成员失败，portalProjectUuid: %s, 错误: %v", in.PortalProjectUuid, err)
			return nil, err
		}
		seen[item.UserId] = struct{}{}
	}

	return &pb.EmptyResp{}, nil
}

func normalizeDevopsProjectRole(role string) string {
	switch strings.TrimSpace(role) {
	case "owner":
		return "owner"
	case "admin":
		return "maintainer"
	case "member":
		return "developer"
	default:
		return "developer"
	}
}
