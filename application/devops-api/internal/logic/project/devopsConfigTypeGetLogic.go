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

type DevopsConfigTypeGetLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取配置中心类型
func NewDevopsConfigTypeGetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsConfigTypeGetLogic {
	return &DevopsConfigTypeGetLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsConfigTypeGetLogic) DevopsConfigTypeGet(req *types.DefaultStringIdRequest) (resp *types.DevopsConfigType, err error) {
	result, err := l.svcCtx.ProjectRpc.ConfigTypeGet(l.ctx, &projectservice.GetByIdReq{
		Id:            req.Id,
		CurrentUserId: currentUserID(l.ctx),
		CurrentRoles:  currentRoles(l.ctx),
	})
	if err != nil {
		return nil, err
	}
	data := configTypeToType(result.Data)

	return &data, nil
}
