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

type DevopsConfigTypeCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 创建配置中心类型
func NewDevopsConfigTypeCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsConfigTypeCreateLogic {
	return &DevopsConfigTypeCreateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsConfigTypeCreateLogic) DevopsConfigTypeCreate(req *types.CreateDevopsConfigTypeRequest) (resp *types.IdResponse, err error) {
	result, err := l.svcCtx.ProjectRpc.ConfigTypeCreate(l.ctx, &projectservice.CreateConfigTypeReq{
		ParentId:      req.ParentId,
		Name:          req.Name,
		Code:          req.Code,
		StorageType:   req.StorageType,
		Description:   req.Description,
		SortOrder:     req.SortOrder,
		Status:        req.Status,
		CreatedBy:     currentUsername(l.ctx),
		CurrentUserId: currentUserID(l.ctx),
		CurrentRoles:  currentRoles(l.ctx),
	})
	if err != nil {
		return nil, err
	}

	return &types.IdResponse{Id: result.Id}, nil
}
