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

type DevopsPipelineTemplateListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 分页查询流水线模板
func NewDevopsPipelineTemplateListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsPipelineTemplateListLogic {
	return &DevopsPipelineTemplateListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsPipelineTemplateListLogic) DevopsPipelineTemplateList(req *types.ListDevopsPipelineTemplateRequest) (resp *types.ListDevopsPipelineTemplateResponse, err error) {
	result, err := l.svcCtx.PipelineRpc.PipelineTemplateList(l.ctx, &pipelineconfigservice.ListPipelineTemplateReq{
		Page:          req.Page,
		PageSize:      req.PageSize,
		Name:          req.Name,
		Code:          req.Code,
		Scope:         req.Scope,
		ProjectId:     req.ProjectId,
		EngineType:    req.EngineType,
		Status:        req.Status,
		CurrentUserId: currentUserID(l.ctx),
		CurrentRoles:  currentRoles(l.ctx),
	})
	if err != nil {
		return nil, err
	}
	items := make([]types.DevopsPipelineTemplate, 0, len(result.Data))
	for _, item := range result.Data {
		items = append(items, pipelineTemplateToType(item))
	}

	return &types.ListDevopsPipelineTemplateResponse{Items: items, Total: result.Total}, nil
}
