package executionservicelogic

import (
	"context"
	"time"

	"github.com/yanshicheng/kube-nova/application/devops-pipeline-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/devops-pipeline-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-pipeline-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type PipelineRunListLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPipelineRunListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PipelineRunListLogic {
	return &PipelineRunListLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *PipelineRunListLogic) PipelineRunList(in *pb.ListPipelineRunReq) (*pb.ListPipelineRunResp, error) {
	projectIDs, restricted, err := accessibleProjectIDs(l.ctx, l.svcCtx, in.CurrentUserId, in.CurrentRoles)
	if err != nil {
		l.Errorf("流水线运行查询列表失败: %v", err)
		return nil, err
	}
	items, total, err := l.svcCtx.PipelineRunModel.List(l.ctx, model.DevopsPipelineRunListFilter{
		ProjectID:  in.ProjectId,
		ProjectIDs: projectIDs,
		Restricted: restricted,
		PipelineID: in.PipelineId,
		Status:     in.Status,
		Page:       in.Page,
		PageSize:   in.PageSize,
	})
	if err != nil {
		l.Errorf("流水线运行查询列表失败: %v", err)
		return nil, err
	}
	scheduleRunListStatusRefresh(l.svcCtx, items, in.CurrentUserId, in.CurrentRoles)
	data := make([]*pb.DevopsPipelineRun, 0, len(items))
	for _, item := range items {
		data = append(data, pipelineRunToPb(item))
	}

	return &pb.ListPipelineRunResp{Data: data, Total: total}, nil
}

func scheduleRunListStatusRefresh(svcCtx *svc.ServiceContext, items []*model.DevopsPipelineRun, userID uint64, roles []string) {
	for _, item := range items {
		if item == nil || (item.Status != "queued" && item.Status != "running" && item.Status != "paused") {
			continue
		}
		runID := item.ID.Hex()
		go func(runID string) {
			ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
			defer cancel()
			run, err := svcCtx.PipelineRunModel.FindOne(ctx, runID)
			if err != nil {
				logx.Errorf("刷新 Jenkins 执行状态失败: runId=%s err=%v", runID, err)
				return
			}
			if run.EngineType == engineTekton {
				if err := syncTektonRunSnapshot(ctx, svcCtx, run, userID, roles, run.UpdatedBy); err != nil {
					logx.Errorf("同步 Tekton 执行状态失败: runId=%s err=%v", runID, err)
				}
				return
			}
			runtime, err := buildRuntimeCached(ctx, svcCtx, run.ProjectID, run.SystemID, run.EnvironmentID, run.BuildChannelBindingID, userID, roles)
			if err != nil {
				logx.Errorf("刷新 Jenkins 执行状态失败: runId=%s err=%v", runID, err)
				return
			}
			manager := jenkinsManagerFromRuntime(runtime)
			ready, err := ensureRunBuildNumber(ctx, svcCtx, run, manager, "")
			if err != nil {
				logx.Errorf("同步 Jenkins 构建号失败: runId=%s queueId=%s err=%v", runID, run.JenkinsQueueID, err)
				return
			}
			if ready && shouldSyncJenkinsRunStatus(runID) {
				if err := syncRunStatusFromJenkins(ctx, svcCtx, run, manager, ""); err != nil {
					logx.Errorf("同步 Jenkins 构建状态失败: runId=%s buildNumber=%d err=%v", runID, run.JenkinsBuildNumber, err)
				}
			}
		}(runID)
	}
}
