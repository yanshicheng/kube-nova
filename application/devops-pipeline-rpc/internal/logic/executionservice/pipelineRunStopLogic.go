package executionservicelogic

import (
	"context"
	"strings"
	"time"

	"github.com/yanshicheng/kube-nova/application/devops-pipeline-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-pipeline-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type PipelineRunStopLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPipelineRunStopLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PipelineRunStopLogic {
	return &PipelineRunStopLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *PipelineRunStopLogic) PipelineRunStop(in *pb.StopPipelineRunReq) (*pb.EmptyResp, error) {
	run, err := l.svcCtx.PipelineRunModel.FindOne(l.ctx, in.RunId)
	if err != nil {
		l.Errorf("流水线运行停止失败: %v", err)
		return nil, err
	}
	if err := ensureProjectAccess(l.ctx, l.svcCtx, run.ProjectID, in.CurrentUserId, in.CurrentRoles); err != nil {
		l.Errorf("流水线运行停止失败: %v", err)
		return nil, err
	}
	if isFinalRunStatus(run.Status) {
		return nil, errorx.Msg("流水线运行已结束，不能停止")
	}
	if run.EngineType == engineTekton {
		if strings.TrimSpace(run.TektonNamespace) == "" || strings.TrimSpace(run.TektonPipelineRunName) == "" {
			run.Status = "aborted"
			run.FinishedAt = time.Now()
			run.DurationSeconds = int64(run.FinishedAt.Sub(run.StartedAt).Seconds())
			run.UpdatedBy = in.Operator
			if err := l.svcCtx.PipelineRunModel.Update(l.ctx, run); err != nil {
				l.Errorf("流水线运行停止失败: %v", err)
				return nil, err
			}
			_ = l.svcCtx.PipelineModel.UpdateRunStatus(l.ctx, run.PipelineID, "aborted", in.Operator)
			_ = recordPipelineRunMetric(l.ctx, l.svcCtx, run)
			return &pb.EmptyResp{}, nil
		}
		client, _, err := tektonClientForRun(l.ctx, l.svcCtx, run, in.CurrentUserId, in.CurrentRoles)
		if err != nil {
			l.Errorf("流水线运行停止失败: %v", err)
			return nil, err
		}
		if err := client.CancelPipelineRun(l.ctx, run.TektonNamespace, run.TektonPipelineRunName); err != nil {
			l.Errorf("流水线运行停止失败: %v", err)
			return nil, err
		}
		run.Status = "aborted"
		run.FinishedAt = time.Now()
		run.DurationSeconds = int64(run.FinishedAt.Sub(run.StartedAt).Seconds())
		run.UpdatedBy = in.Operator
		if err := l.svcCtx.PipelineRunModel.Update(l.ctx, run); err != nil {
			l.Errorf("流水线运行停止失败: %v", err)
			return nil, err
		}
		_ = l.svcCtx.PipelineModel.UpdateRunStatus(l.ctx, run.PipelineID, "aborted", in.Operator)
		_ = recordPipelineRunMetric(l.ctx, l.svcCtx, run)
		return &pb.EmptyResp{}, nil
	}
	runtime, err := buildRuntime(l.ctx, l.svcCtx, run.ProjectID, run.SystemID, run.EnvironmentID, run.BuildChannelBindingID, in.CurrentUserId, in.CurrentRoles)
	if err != nil {
		l.Errorf("流水线运行停止失败: %v", err)
		return nil, err
	}
	manager := jenkinsManagerFromRuntime(runtime)
	ready, err := ensureRunBuildNumber(l.ctx, l.svcCtx, run, manager, in.Operator)
	if err != nil {
		l.Errorf("流水线运行停止失败: %v", err)
		return nil, err
	}
	if ready {
		if err := manager.StopBuild(l.ctx, run.JenkinsJobFullName, run.JenkinsBuildNumber); err != nil {
			l.Errorf("流水线运行停止失败: %v", err)
			return nil, err
		}
	}
	run.Status = "aborted"
	run.FinishedAt = time.Now()
	run.DurationSeconds = int64(run.FinishedAt.Sub(run.StartedAt).Seconds())
	run.UpdatedBy = in.Operator
	if err := l.svcCtx.PipelineRunModel.Update(l.ctx, run); err != nil {
		l.Errorf("流水线运行停止失败: %v", err)
		return nil, err
	}
	_ = l.svcCtx.PipelineModel.UpdateRunStatus(l.ctx, run.PipelineID, "aborted", in.Operator)
	_ = recordPipelineRunMetric(l.ctx, l.svcCtx, run)

	return &pb.EmptyResp{}, nil
}
