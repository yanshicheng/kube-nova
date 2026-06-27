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

type DevopsSystemListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 查询系统
func NewDevopsSystemListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsSystemListLogic {
	return &DevopsSystemListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsSystemListLogic) DevopsSystemList(req *types.ListDevopsSystemRequest) (resp *types.ListDevopsSystemResponse, err error) {
	result, err := l.svcCtx.PipelineRpc.SystemList(l.ctx, &pipelineconfigservice.ListSystemReq{
		Page:          req.Page,
		PageSize:      req.PageSize,
		ProjectId:     req.ProjectId,
		Name:          req.Name,
		Code:          req.Code,
		Status:        req.Status,
		CurrentUserId: currentUserID(l.ctx),
		CurrentRoles:  currentRoles(l.ctx),
	})
	if err != nil {
		return nil, err
	}
	items := make([]types.DevopsSystem, 0, len(result.Data))
	for _, item := range result.Data {
		items = append(items, systemToType(item))
	}

	return &types.ListDevopsSystemResponse{Items: items, Total: result.Total}, nil
}
