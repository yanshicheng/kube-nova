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

type DevopsProjectCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 创建 DevOps 项目
func NewDevopsProjectCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsProjectCreateLogic {
	return &DevopsProjectCreateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsProjectCreateLogic) DevopsProjectCreate(req *types.CreateDevopsProjectRequest) (resp *types.IdResponse, err error) {
	result, err := l.svcCtx.ProjectRpc.ProjectCreate(l.ctx, &projectservice.CreateProjectReq{
		Name:                   req.Name,
		Description:            req.Description,
		DefaultEngineChannelId: req.DefaultEngineChannelId,
		BuildChannelIds:        req.BuildChannelIds,
		Status:                 req.Status,
		ExtraConfig:            req.ExtraConfig,
		CreatedBy:              currentUsername(l.ctx),
	})
	if err != nil {
		return nil, err
	}
	if userID := currentUserID(l.ctx); userID > 0 {
		_, err = l.svcCtx.ProjectRpc.ProjectMemberSet(l.ctx, &projectservice.SetProjectMembersReq{
			ProjectId: result.Id,
			Members: []*projectservice.DevopsProjectMemberInput{
				{
					UserId:   userID,
					Username: currentUsername(l.ctx),
					Role:     "owner",
				},
			},
			UpdatedBy: currentUsername(l.ctx),
		})
		if err != nil {
			return nil, err
		}
	}

	return &types.IdResponse{Id: result.Id}, nil
}
