// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package pipeline

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/client/pipelineconfigservice"

	"github.com/zeromicro/go-zero/core/logx"
)

type DevopsSystemCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 创建系统
func NewDevopsSystemCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsSystemCreateLogic {
	return &DevopsSystemCreateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsSystemCreateLogic) DevopsSystemCreate(req *types.CreateDevopsSystemRequest) (resp *types.IdResponse, err error) {
	result, err := l.svcCtx.PipelineRpc.SystemCreate(l.ctx, &pipelineconfigservice.CreateSystemReq{
		ProjectId:     req.ProjectId,
		Name:          req.Name,
		Code:          req.Code,
		Description:   req.Description,
		OwnerUserIds:  req.OwnerUserIds,
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
