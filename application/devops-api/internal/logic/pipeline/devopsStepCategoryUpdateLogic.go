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

type DevopsStepCategoryUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 更新步骤分类
func NewDevopsStepCategoryUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsStepCategoryUpdateLogic {
	return &DevopsStepCategoryUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsStepCategoryUpdateLogic) DevopsStepCategoryUpdate(req *types.UpdateDevopsStepCategoryRequest) error {
	_, err := l.svcCtx.PipelineRpc.StepCategoryUpdate(l.ctx, &pipelineconfigservice.UpdateStepCategoryReq{
		Id:          req.Id,
		Name:        req.Name,
		Description: req.Description,
		Icon:        req.Icon,
		IconColor:   req.IconColor,
		SortOrder:   req.SortOrder,
		Status:      req.Status,
		UpdatedBy:   currentUsername(l.ctx),
	})
	return err
}
