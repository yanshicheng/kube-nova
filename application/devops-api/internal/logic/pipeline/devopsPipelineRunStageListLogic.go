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

type DevopsPipelineRunStageListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 查询流水线阶段状态
func NewDevopsPipelineRunStageListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsPipelineRunStageListLogic {
	return &DevopsPipelineRunStageListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsPipelineRunStageListLogic) DevopsPipelineRunStageList(req *types.ListDevopsPipelineRunStageRequest) (resp *types.ListDevopsPipelineRunStageResponse, err error) {
	result, err := l.svcCtx.PipelineExecRpc.PipelineRunStageList(l.ctx, &executionservice.ListPipelineRunStageReq{
		RunId:         req.RunId,
		CurrentUserId: currentUserID(l.ctx),
		CurrentRoles:  currentRoles(l.ctx),
	})
	if err != nil {
		return nil, err
	}
	items := make([]types.DevopsPipelineRunStage, 0, len(result.Data))
	for _, item := range result.Data {
		items = append(items, pipelineRunStageToType(item))
	}

	return &types.ListDevopsPipelineRunStageResponse{Items: items}, nil
}
