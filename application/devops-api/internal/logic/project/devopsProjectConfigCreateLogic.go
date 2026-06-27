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

type DevopsProjectConfigCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 创建配置中心项目配置
func NewDevopsProjectConfigCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsProjectConfigCreateLogic {
	return &DevopsProjectConfigCreateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsProjectConfigCreateLogic) DevopsProjectConfigCreate(req *types.CreateDevopsProjectConfigRequest) (resp *types.IdResponse, err error) {
	result, err := l.svcCtx.ProjectRpc.ProjectConfigCreate(l.ctx, &projectservice.CreateProjectConfigReq{
		ProjectId:     req.ProjectId,
		TypeId:        req.TypeId,
		TypeCode:      req.TypeCode,
		Name:          req.Name,
		Code:          req.Code,
		Content:       req.Content,
		Description:   req.Description,
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
