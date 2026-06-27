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

type DevopsPipelineListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 查询流水线
func NewDevopsPipelineListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsPipelineListLogic {
	return &DevopsPipelineListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsPipelineListLogic) DevopsPipelineList(req *types.ListDevopsPipelineRequest) (resp *types.ListDevopsPipelineResponse, err error) {
	result, err := l.svcCtx.PipelineExecRpc.PipelineList(l.ctx, &executionservice.ListPipelineReq{
		Page:          req.Page,
		PageSize:      req.PageSize,
		ProjectId:     req.ProjectId,
		SystemId:      req.SystemId,
		EnvironmentId: req.EnvironmentId,
		Name:          req.Name,
		Code:          req.Code,
		EngineType:    req.EngineType,
		SyncStatus:    req.SyncStatus,
		LastRunStatus: req.LastRunStatus,
		TriggerMode:   req.TriggerMode,
		Status:        req.Status,
		CurrentUserId: currentUserID(l.ctx),
		CurrentRoles:  currentRoles(l.ctx),
	})
	if err != nil {
		return nil, err
	}
	items := make([]types.DevopsPipeline, 0, len(result.Data))
	for _, item := range result.Data {
		items = append(items, pipelineToType(item))
	}

	return &types.ListDevopsPipelineResponse{Items: items, Total: result.Total}, nil
}
