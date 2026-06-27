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

type DevopsTektonStepListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 分页查询 Tekton 步骤
func NewDevopsTektonStepListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsTektonStepListLogic {
	return &DevopsTektonStepListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsTektonStepListLogic) DevopsTektonStepList(req *types.ListDevopsStepTemplateRequest) (resp *types.ListDevopsStepTemplateResponse, err error) {
	result, err := l.svcCtx.PipelineRpc.TektonStepList(l.ctx, &pipelineconfigservice.ListStepTemplateReq{
		Page:       req.Page,
		PageSize:   req.PageSize,
		Name:       req.Name,
		Code:       req.Code,
		CategoryId: req.CategoryId,
		EngineType: "tekton",
		Type:       req.Type,
		Status:     req.Status,
	})
	if err != nil {
		return nil, err
	}
	items := make([]types.DevopsStepTemplate, 0, len(result.Data))
	for _, item := range result.Data {
		items = append(items, stepTemplateToType(item))
	}

	return &types.ListDevopsStepTemplateResponse{Items: items, Total: result.Total}, nil
}
