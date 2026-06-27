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

type DevopsProjectMemberSetLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 设置项目成员
func NewDevopsProjectMemberSetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsProjectMemberSetLogic {
	return &DevopsProjectMemberSetLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsProjectMemberSetLogic) DevopsProjectMemberSet(req *types.SetDevopsProjectMembersRequest) error {
	members := make([]*projectservice.DevopsProjectMemberInput, 0, len(req.Members))
	for _, item := range req.Members {
		members = append(members, &projectservice.DevopsProjectMemberInput{
			UserId:   item.UserId,
			Username: item.Username,
			Nickname: item.Nickname,
			Role:     item.Role,
		})
	}
	_, err := l.svcCtx.ProjectRpc.ProjectMemberSet(l.ctx, &projectservice.SetProjectMembersReq{
		ProjectId: req.Id,
		Members:   members,
		UpdatedBy: currentUsername(l.ctx),
	})

	return err
}
