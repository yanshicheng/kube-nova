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

type DevopsProjectMavenConfigGetLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取项目 Maven 配置
func NewDevopsProjectMavenConfigGetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsProjectMavenConfigGetLogic {
	return &DevopsProjectMavenConfigGetLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsProjectMavenConfigGetLogic) DevopsProjectMavenConfigGet(req *types.DefaultStringIdRequest) (resp *types.DevopsProjectMavenConfig, err error) {
	result, err := l.svcCtx.ProjectRpc.ProjectMavenConfigGet(l.ctx, &projectservice.GetByIdReq{
		Id:            req.Id,
		CurrentUserId: currentUserID(l.ctx),
		CurrentRoles:  currentRoles(l.ctx),
	})
	if err != nil {
		return nil, err
	}
	data := projectMavenConfigToType(result.Data)

	return &data, nil
}
