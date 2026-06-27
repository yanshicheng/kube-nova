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

type DevopsPipelineRunLogLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 查询流水线日志
func NewDevopsPipelineRunLogLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsPipelineRunLogLogic {
	return &DevopsPipelineRunLogLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsPipelineRunLogLogic) DevopsPipelineRunLog(req *types.DevopsPipelineRunLogRequest) (resp *types.DevopsPipelineRunLogResponse, err error) {
	result, err := l.svcCtx.PipelineExecRpc.PipelineRunLog(l.ctx, &executionservice.PipelineRunLogReq{
		RunId:         req.RunId,
		StageId:       req.StageId,
		Full:          req.Full,
		CurrentUserId: currentUserID(l.ctx),
		CurrentRoles:  currentRoles(l.ctx),
		TaskRunName:   req.TaskRunName,
		PodName:       req.PodName,
		ContainerName: req.ContainerName,
	})
	if err != nil {
		return nil, err
	}

	return &types.DevopsPipelineRunLogResponse{
		Content:  result.Content,
		Archived: result.Archived,
		Source:   result.Source,
		Changed:  true,
	}, nil
}
