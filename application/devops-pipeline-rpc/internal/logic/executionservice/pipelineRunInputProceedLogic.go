package executionservicelogic

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/yanshicheng/kube-nova/application/devops-pipeline-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/devops-pipeline-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-pipeline-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/devops/jenkins"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type PipelineRunInputProceedLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPipelineRunInputProceedLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PipelineRunInputProceedLogic {
	return &PipelineRunInputProceedLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *PipelineRunInputProceedLogic) PipelineRunInputProceed(in *pb.PipelineRunInputActionReq) (*pb.EmptyResp, error) {
	params := map[string]string{}
	if in.Params != "" {
		if err := json.Unmarshal([]byte(in.Params), &params); err != nil {
			l.Errorf("流水线人工确认继续失败: %v", err)
			return nil, err
		}
	}
	run, manager, err := preparePipelineRunInputAction(l.ctx, l.svcCtx, in.RunId, in.CurrentUserId, in.CurrentRoles, in.Operator)
	if err != nil {
		l.Errorf("流水线人工确认继续失败: %v", err)
		return nil, err
	}
	if err := manager.ProceedInput(l.ctx, run.JenkinsJobFullName, run.JenkinsBuildNumber, in.InputId, params); err != nil {
		l.Errorf("流水线人工确认继续失败: %v", err)
		return nil, err
	}
	run.Status = "running"
	run.UpdatedBy = in.Operator
	if err := l.svcCtx.PipelineRunModel.Update(l.ctx, run); err != nil {
		l.Errorf("流水线人工确认继续失败: %v", err)
		return nil, err
	}
	_ = l.svcCtx.PipelineModel.UpdateRunStatus(l.ctx, run.PipelineID, "running", in.Operator)

	return &pb.EmptyResp{}, nil
}

func preparePipelineRunInputAction(ctx context.Context, svcCtx *svc.ServiceContext, runID string, userID uint64, roles []string, operator string) (*model.DevopsPipelineRun, *jenkins.Manager, error) {
	run, err := svcCtx.PipelineRunModel.FindOneForInput(ctx, runID)
	if err != nil {
		logx.Errorf("流水线人工确认处理失败: %v", err)
		return nil, nil, err
	}
	if err := ensureProjectAccess(ctx, svcCtx, run.ProjectID, userID, roles); err != nil {
		logx.Errorf("流水线人工确认处理失败: %v", err)
		return nil, nil, err
	}
	runtime, err := buildRuntime(ctx, svcCtx, run.ProjectID, run.SystemID, run.EnvironmentID, run.BuildChannelBindingID, userID, roles)
	if err != nil {
		logx.Errorf("流水线人工确认处理失败: %v", err)
		return nil, nil, err
	}
	manager := jenkinsManagerFromRuntime(runtime)
	ready, err := ensureRunBuildNumber(ctx, svcCtx, run, manager, operator)
	if err != nil {
		logx.Errorf("流水线人工确认处理失败: %v", err)
		return nil, nil, err
	}
	if !ready {
		logx.Errorf("Jenkins 构建号未就绪")
		return nil, nil, errorx.Msg("Jenkins 构建号未就绪")
	}
	if !strings.EqualFold(run.Status, "paused") && !strings.EqualFold(run.Status, "running") {
		logx.Errorf("流水线未等待人工确认")
		return nil, nil, errorx.Msg("流水线未等待人工确认")
	}
	return run, manager, nil
}
