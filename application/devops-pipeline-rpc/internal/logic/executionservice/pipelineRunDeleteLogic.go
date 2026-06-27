package executionservicelogic

import (
	"context"
	"strings"

	"github.com/yanshicheng/kube-nova/application/devops-pipeline-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-pipeline-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type PipelineRunDeleteLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPipelineRunDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PipelineRunDeleteLogic {
	return &PipelineRunDeleteLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *PipelineRunDeleteLogic) PipelineRunDelete(in *pb.DeletePipelineRunReq) (*pb.EmptyResp, error) {
	run, err := l.svcCtx.PipelineRunModel.FindOne(l.ctx, in.RunId)
	if err != nil {
		l.Errorf("流水线运行记录删除失败: %v", err)
		return nil, err
	}
	if err := ensureProjectAccess(l.ctx, l.svcCtx, run.ProjectID, in.CurrentUserId, in.CurrentRoles); err != nil {
		l.Errorf("流水线运行记录删除失败: %v", err)
		return nil, err
	}
	if !isFinalRunStatus(run.Status) {
		l.Errorf("流水线未执行完成，不能删除")
		return nil, errorx.Msg("流水线未执行完成，不能删除")
	}
	if run.EngineType == engineTekton {
		if strings.TrimSpace(run.TektonNamespace) != "" && strings.TrimSpace(run.TektonPipelineRunName) != "" {
			client, _, err := tektonClientForRun(l.ctx, l.svcCtx, run, in.CurrentUserId, in.CurrentRoles)
			if err != nil {
				l.Errorf("流水线运行记录删除失败: %v", err)
				return nil, err
			}
			if err := client.DeletePipelineRun(l.ctx, run.TektonNamespace, run.TektonPipelineRunName); err != nil {
				l.Errorf("删除 Tekton PipelineRun 失败: runId=%s name=%s err=%v", run.ID.Hex(), run.TektonPipelineRunName, err)
				return nil, err
			}
		}
	} else {
		runtime, err := buildRuntime(l.ctx, l.svcCtx, run.ProjectID, run.SystemID, run.EnvironmentID, run.BuildChannelBindingID, in.CurrentUserId, in.CurrentRoles)
		if err != nil {
			l.Errorf("流水线运行记录删除失败: %v", err)
			return nil, err
		}
		manager := jenkinsManagerFromRuntime(runtime)
		if manager != nil && strings.TrimSpace(run.JenkinsJobFullName) != "" && run.JenkinsBuildNumber > 0 {
			if err := manager.DeleteBuild(l.ctx, run.JenkinsJobFullName, run.JenkinsBuildNumber); err != nil {
				l.Errorf("删除 Jenkins 构建记录失败: runId=%s buildNumber=%d err=%v", run.ID.Hex(), run.JenkinsBuildNumber, err)
				return nil, err
			}
		}
	}
	stages, _ := l.svcCtx.RunStageModel.ListByRun(l.ctx, in.RunId)
	if err := l.svcCtx.RunStageModel.DeleteByRun(l.ctx, in.RunId); err != nil {
		l.Errorf("流水线运行记录删除失败: %v", err)
		return nil, err
	}
	if l.svcCtx.ArtifactModel != nil {
		if err := l.svcCtx.ArtifactModel.DeleteByRun(l.ctx, in.RunId); err != nil {
			l.Errorf("流水线运行记录删除失败: %v", err)
			return nil, err
		}
	}
	if err := l.svcCtx.PipelineRunModel.DeleteSoft(l.ctx, in.RunId, in.Operator); err != nil {
		l.Errorf("流水线运行记录删除失败: %v", err)
		return nil, err
	}
	removePipelineRunLogCache(l.ctx, l.svcCtx, in.RunId, stages)
	refreshPipelineLastRunStatus(l.ctx, l.svcCtx, run.PipelineID, in.Operator)

	return &pb.EmptyResp{}, nil
}
