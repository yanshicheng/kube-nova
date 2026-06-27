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

type DevopsPipelineRunLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 运行流水线
func NewDevopsPipelineRunLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsPipelineRunLogic {
	return &DevopsPipelineRunLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsPipelineRunLogic) DevopsPipelineRun(req *types.RunDevopsPipelineRequest) (resp *types.IdResponse, err error) {
	result, err := l.svcCtx.PipelineExecRpc.PipelineRun(l.ctx, &executionservice.RunPipelineReq{
		Id:              req.Id,
		Params:          req.Params,
		Workspaces:      req.Workspaces,
		TriggerType:     req.TriggerType,
		CurrentUserId:   currentUserID(l.ctx),
		CurrentUsername: currentUsername(l.ctx),
		CurrentRoles:    currentRoles(l.ctx),
	})
	if err != nil {
		return nil, err
	}

	return &types.IdResponse{Id: result.Id}, nil
}
