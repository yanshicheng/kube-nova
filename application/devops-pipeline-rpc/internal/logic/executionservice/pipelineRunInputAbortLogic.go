package executionservicelogic

import (
	"context"
	"time"

	"github.com/yanshicheng/kube-nova/application/devops-pipeline-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-pipeline-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type PipelineRunInputAbortLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPipelineRunInputAbortLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PipelineRunInputAbortLogic {
	return &PipelineRunInputAbortLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *PipelineRunInputAbortLogic) PipelineRunInputAbort(in *pb.PipelineRunInputActionReq) (*pb.EmptyResp, error) {
	run, manager, err := preparePipelineRunInputAction(l.ctx, l.svcCtx, in.RunId, in.CurrentUserId, in.CurrentRoles, in.Operator)
	if err != nil {
		l.Errorf("流水线人工确认终止失败: %v", err)
		return nil, err
	}
	if err := manager.AbortInput(l.ctx, run.JenkinsJobFullName, run.JenkinsBuildNumber, in.InputId); err != nil {
		l.Errorf("流水线人工确认终止失败: %v", err)
		return nil, err
	}
	run.Status = "aborted"
	run.FinishedAt = time.Now()
	run.DurationSeconds = int64(run.FinishedAt.Sub(run.StartedAt).Seconds())
	run.UpdatedBy = in.Operator
	if err := l.svcCtx.PipelineRunModel.Update(l.ctx, run); err != nil {
		l.Errorf("流水线人工确认终止失败: %v", err)
		return nil, err
	}
	_ = l.svcCtx.PipelineModel.UpdateRunStatus(l.ctx, run.PipelineID, "aborted", in.Operator)
	_ = recordPipelineRunMetric(l.ctx, l.svcCtx, run)

	return &pb.EmptyResp{}, nil
}
