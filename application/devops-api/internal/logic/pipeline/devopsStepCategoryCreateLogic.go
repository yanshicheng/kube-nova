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

type DevopsStepCategoryCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 创建步骤分类
func NewDevopsStepCategoryCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsStepCategoryCreateLogic {
	return &DevopsStepCategoryCreateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsStepCategoryCreateLogic) DevopsStepCategoryCreate(req *types.CreateDevopsStepCategoryRequest) (resp *types.IdResponse, err error) {
	result, err := l.svcCtx.PipelineRpc.StepCategoryCreate(l.ctx, &pipelineconfigservice.CreateStepCategoryReq{
		Name:        req.Name,
		Code:        req.Code,
		Description: req.Description,
		Icon:        req.Icon,
		IconColor:   req.IconColor,
		SortOrder:   req.SortOrder,
		Status:      req.Status,
		CreatedBy:   currentUsername(l.ctx),
	})
	if err != nil {
		return nil, err
	}

	return &types.IdResponse{Id: result.Id}, nil
}
