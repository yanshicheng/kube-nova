// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package pipeline

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/devops-pipeline-rpc/client/executionservice"

	"github.com/zeromicro/go-zero/core/logx"
)

type DevopsPipelineRunListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 查询流水线执行记录
func NewDevopsPipelineRunListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsPipelineRunListLogic {
	return &DevopsPipelineRunListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsPipelineRunListLogic) DevopsPipelineRunList(req *types.ListDevopsPipelineRunRequest) (resp *types.ListDevopsPipelineRunResponse, err error) {
	result, err := l.svcCtx.PipelineExecRpc.PipelineRunList(l.ctx, &executionservice.ListPipelineRunReq{
		Page:          req.Page,
		PageSize:      req.PageSize,
		ProjectId:     req.ProjectId,
		PipelineId:    req.PipelineId,
		Status:        req.Status,
		CurrentUserId: currentUserID(l.ctx),
		CurrentRoles:  currentRoles(l.ctx),
	})
	if err != nil {
		return nil, err
	}
	items := make([]types.DevopsPipelineRun, 0, len(result.Data))
	for _, item := range result.Data {
		items = append(items, pipelineRunToType(item))
	}

	return &types.ListDevopsPipelineRunResponse{Items: items, Total: result.Total}, nil
}
