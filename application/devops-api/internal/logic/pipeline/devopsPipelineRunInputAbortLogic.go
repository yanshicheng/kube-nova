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

type DevopsPipelineRunInputAbortLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 终止流水线人工确认
func NewDevopsPipelineRunInputAbortLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsPipelineRunInputAbortLogic {
	return &DevopsPipelineRunInputAbortLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsPipelineRunInputAbortLogic) DevopsPipelineRunInputAbort(req *types.DevopsPipelineRunInputActionRequest) error {
	_, err := l.svcCtx.PipelineExecRpc.PipelineRunInputAbort(l.ctx, &executionservice.PipelineRunInputActionReq{
		RunId:         req.RunId,
		InputId:       req.InputId,
		Params:        req.Params,
		Operator:      currentUsername(l.ctx),
		CurrentUserId: currentUserID(l.ctx),
		CurrentRoles:  currentRoles(l.ctx),
	})
	return err
}
