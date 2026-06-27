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

type DevopsPipelineRunStopLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 停止流水线执行
func NewDevopsPipelineRunStopLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsPipelineRunStopLogic {
	return &DevopsPipelineRunStopLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsPipelineRunStopLogic) DevopsPipelineRunStop(req *types.DefaultStringIdRequest) error {
	_, err := l.svcCtx.PipelineExecRpc.PipelineRunStop(l.ctx, &executionservice.StopPipelineRunReq{
		RunId:         req.Id,
		Operator:      currentUsername(l.ctx),
		CurrentUserId: currentUserID(l.ctx),
		CurrentRoles:  currentRoles(l.ctx),
	})
	return err
}
