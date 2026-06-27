package projectservicelogic

import (
	"context"

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
	return nil, errorx.Msg("项目成员请在门户项目管理中维护")
}

func isAllowedProjectRole(role string) bool {
	switch role {
	case "owner", "maintainer", "developer", "viewer":
		return true
	default:
		return false
	}
}
