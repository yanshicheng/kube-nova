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

type DevopsPipelineRunInputListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 查询流水线人工确认
func NewDevopsPipelineRunInputListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsPipelineRunInputListLogic {
	return &DevopsPipelineRunInputListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsPipelineRunInputListLogic) DevopsPipelineRunInputList(req *types.ListDevopsPipelineRunInputRequest) (resp *types.ListDevopsPipelineRunInputResponse, err error) {
	result, err := l.svcCtx.PipelineExecRpc.PipelineRunInputList(l.ctx, &executionservice.PipelineRunInputListReq{
		RunId:         req.RunId,
		CurrentUserId: currentUserID(l.ctx),
		CurrentRoles:  currentRoles(l.ctx),
	})
	if err != nil {
		return nil, err
	}
	items := make([]types.DevopsPipelineRunInput, 0, len(result.Data))
	for _, item := range result.Data {
		items = append(items, pipelineRunInputToType(item))
	}
	return &types.ListDevopsPipelineRunInputResponse{Items: items}, nil
}
