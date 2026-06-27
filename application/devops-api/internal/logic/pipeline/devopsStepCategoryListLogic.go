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

type DevopsStepCategoryListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 分页查询步骤分类
func NewDevopsStepCategoryListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsStepCategoryListLogic {
	return &DevopsStepCategoryListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsStepCategoryListLogic) DevopsStepCategoryList(req *types.ListDevopsStepCategoryRequest) (resp *types.ListDevopsStepCategoryResponse, err error) {
	result, err := l.svcCtx.PipelineRpc.StepCategoryList(l.ctx, &pipelineconfigservice.ListStepCategoryReq{
		Page:     req.Page,
		PageSize: req.PageSize,
		Name:     req.Name,
		Code:     req.Code,
		Status:   req.Status,
	})
	if err != nil {
		return nil, err
	}
	items := make([]types.DevopsStepCategory, 0, len(result.Data))
	for _, item := range result.Data {
		items = append(items, stepCategoryToType(item))
	}

	return &types.ListDevopsStepCategoryResponse{Items: items, Total: result.Total}, nil
}
