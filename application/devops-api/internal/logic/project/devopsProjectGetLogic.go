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

type DevopsProjectGetLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 DevOps 项目详情
func NewDevopsProjectGetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsProjectGetLogic {
	return &DevopsProjectGetLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsProjectGetLogic) DevopsProjectGet(req *types.DefaultStringIdRequest) (resp *types.DevopsProject, err error) {
	result, err := l.svcCtx.ProjectRpc.ProjectGet(l.ctx, &projectservice.GetByIdReq{
		Id:            req.Id,
		CurrentUserId: currentUserID(l.ctx),
		CurrentRoles:  currentRoles(l.ctx),
	})
	if err != nil {
		return nil, err
	}
	data := projectToType(result.Data)

	return &data, nil
}
