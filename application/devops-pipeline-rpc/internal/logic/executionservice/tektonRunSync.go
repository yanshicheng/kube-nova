package executionservicelogic

import (
	"context"
	"strings"
	"time"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/client/pipelineconfigservice"
	"github.com/yanshicheng/kube-nova/application/devops-pipeline-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/devops-pipeline-rpc/internal/svc"
	devopstekton "github.com/yanshicheng/kube-nova/common/devops/tekton"
	"github.com/zeromicro/go-zero/core/logx"
)

func syncTektonRunSnapshot(ctx context.Context, svcCtx *svc.ServiceContext, run *model.DevopsPipelineRun, userID uint64, roles []string, operator string) error {
	if run == nil || run.EngineType != engineTekton {
		return nil
	}
	if strings.TrimSpace(run.TektonNamespace) == "" || strings.TrimSpace(run.TektonPipelineRunName) == "" {
		return nil
	}
	runtime, err := buildRuntimeCached(ctx, svcCtx, run.ProjectID, run.SystemID, run.EnvironmentID, run.BuildChannelBindingID, userID, roles)
	if err != nil {
		return err
	}
	client, err := devopstekton.NewClient(tektonRequestFromRuntime(runtime))
	if err != nil {
		logx.Errorf("创建 Tekton 客户端失败: %v", err)
		return err
	}
	status, err := client.GetPipelineRunStatus(ctx, run.TektonNamespace, run.TektonPipelineRunName)
	if err != nil {
		return err
	}
	if err := syncTektonRunRecord(ctx, svcCtx, run, status, operator); err != nil {
		return err
	}
	taskRuns, err := client.ListTaskRuns(ctx, run.TektonNamespace, run.TektonPipelineRunName)
	if err != nil {
		logx.Errorf("同步 Tekton TaskRun 状态失败: runId=%s err=%v", run.ID.Hex(), err)
		return nil
	}
	return syncTektonRunStages(ctx, svcCtx, run, taskRuns, status.SkippedTasks)
}

func syncTektonRunRecord(ctx context.Context, svcCtx *svc.ServiceContext, run *model.DevopsPipelineRun, status devopstekton.RunStatus, operator string) error {
	nextStatus := strings.TrimSpace(status.Status)
	if nextStatus == "" {
		nextStatus = "running"
	}
	changed := run.Status != nextStatus
	run.Status = nextStatus
	if !status.StartedAt.IsZero() {
		run.StartedAt = status.StartedAt
	}
	if isFinalRunStatus(nextStatus) {
		if !status.FinishedAt.IsZero() {
			run.FinishedAt = status.FinishedAt
		} else if run.FinishedAt.IsZero() {
			run.FinishedAt = time.Now()
		}
		if strings.TrimSpace(status.Message) != "" && nextStatus != "success" {
			run.ErrorMessage = status.Message
		}
	} else {
		run.FinishedAt = time.Time{}
	}
	if !run.StartedAt.IsZero() {
		end := time.Now()
		if !run.FinishedAt.IsZero() {
			end = run.FinishedAt
		}
		run.DurationSeconds = int64(end.Sub(run.StartedAt).Seconds())
		if run.DurationSeconds < 0 {
			run.DurationSeconds = 0
		}
	}
	if strings.TrimSpace(operator) != "" {
		run.UpdatedBy = operator
	}
	if changed || isFinalRunStatus(nextStatus) {
		if err := svcCtx.PipelineRunModel.Update(ctx, run); err != nil {
			logx.Errorf("同步 Tekton PipelineRun 状态失败: runId=%s err=%v", run.ID.Hex(), err)
			return err
		}
		_ = svcCtx.PipelineModel.UpdateRunStatus(ctx, run.PipelineID, nextStatus, run.UpdatedBy)
	}
	if isFinalRunStatus(nextStatus) {
		_ = recordPipelineRunMetric(ctx, svcCtx, run)
	}
	return nil
}

func syncTektonRunStages(ctx context.Context, svcCtx *svc.ServiceContext, run *model.DevopsPipelineRun, taskRuns []devopstekton.TaskRunInfo, skippedTasks []devopstekton.SkippedTaskInfo) error {
	if len(taskRuns) == 0 && len(skippedTasks) == 0 {
		return nil
	}
	items, err := svcCtx.RunStageModel.ListByRun(ctx, run.ID.Hex())
	if err != nil {
		return err
	}
	if len(items) == 0 {
		return nil
	}
	taskNameToNode := tektonTaskNameToNodeMapForRun(ctx, svcCtx, run)
	if len(taskNameToNode) == 0 {
		steps := pipelineStepsForRun(ctx, svcCtx, run)
		taskNameToNode = make(map[string]string, len(steps))
		for _, step := range steps {
			taskNameToNode[tektonPipelineTaskName(step)] = step.ID
		}
	}
	stageByNode := make(map[string]*model.DevopsPipelineRunStage, len(items))
	for _, item := range items {
		if item != nil && strings.TrimSpace(item.NodeID) != "" {
			stageByNode[item.NodeID] = item
		}
	}
	taskRunsByNode := make(map[string][]devopstekton.TaskRunInfo)
	for _, taskRun := range taskRuns {
		nodeID := taskNameToNode[taskRun.PipelineTaskName]
		if strings.TrimSpace(nodeID) == "" {
			nodeID = taskNameToNode[strings.TrimSpace(taskRun.PipelineTaskName)]
		}
		if strings.TrimSpace(nodeID) == "" || stageByNode[nodeID] == nil {
			continue
		}
		taskRunsByNode[nodeID] = append(taskRunsByNode[nodeID], taskRun)
	}
	for nodeID, group := range taskRunsByNode {
		stage := stageByNode[nodeID]
		applyTektonTaskRunsToStage(stage, group)
		if err := svcCtx.RunStageModel.Update(ctx, stage); err != nil {
			logx.Errorf("同步 Tekton 阶段状态失败: runId=%s stageId=%s err=%v", run.ID.Hex(), stage.ID.Hex(), err)
			return err
		}
	}
	for _, skipped := range skippedTasks {
		nodeID := taskNameToNode[skipped.Name]
		if strings.TrimSpace(nodeID) == "" {
			nodeID = taskNameToNode[strings.TrimSpace(skipped.Name)]
		}
		stage := stageByNode[nodeID]
		if stage == nil {
			continue
		}
		applyTektonSkippedTaskToStage(stage, skipped, run)
		if err := svcCtx.RunStageModel.Update(ctx, stage); err != nil {
			logx.Errorf("同步 Tekton 跳过阶段状态失败: runId=%s stageId=%s err=%v", run.ID.Hex(), stage.ID.Hex(), err)
			return err
		}
	}
	return nil
}

func applyTektonTaskRunToStage(stage *model.DevopsPipelineRunStage, taskRun devopstekton.TaskRunInfo) {
	applyTektonTaskRunsToStage(stage, []devopstekton.TaskRunInfo{taskRun})
}

func applyTektonTaskRunsToStage(stage *model.DevopsPipelineRunStage, taskRuns []devopstekton.TaskRunInfo) {
	if stage == nil || len(taskRuns) == 0 {
		return
	}
	snapshots := make([]model.TektonTaskRunSnapshot, 0, len(taskRuns))
	for _, taskRun := range taskRuns {
		snapshots = append(snapshots, tektonTaskRunSnapshot(taskRun))
	}
	stage.TektonTaskRuns = snapshots
	representative := tektonTaskRunRepresentative(snapshots)
	stage.TektonTaskRunName = representative.Name
	stage.TektonPodName = representative.PodName
	stage.ContainerName = representative.ContainerName
	stage.ContainerNames = representative.ContainerNames
	stage.Status = aggregateTektonTaskRunStatus(snapshots)
	stage.ErrorMessage = aggregateTektonTaskRunMessage(snapshots, stage.Status)

	startedAt := earliestTektonTaskRunStart(snapshots)
	if !startedAt.IsZero() {
		stage.StartedAt = startedAt
	}
	if isFinalRunStatus(stage.Status) {
		finishedAt := latestTektonTaskRunFinish(snapshots)
		if !finishedAt.IsZero() {
			stage.FinishedAt = finishedAt
		} else if stage.FinishedAt.IsZero() {
			stage.FinishedAt = time.Now()
		}
	} else {
		stage.FinishedAt = time.Time{}
	}
	if !stage.StartedAt.IsZero() {
		end := time.Now()
		if !stage.FinishedAt.IsZero() {
			end = stage.FinishedAt
		}
		stage.DurationSeconds = int64(end.Sub(stage.StartedAt).Seconds())
		if stage.DurationSeconds < 0 {
			stage.DurationSeconds = 0
		}
	}
}

func tektonTaskRunSnapshot(taskRun devopstekton.TaskRunInfo) model.TektonTaskRunSnapshot {
	status := strings.TrimSpace(taskRun.Status.Status)
	if status == "" {
		status = "running"
	}
	snapshot := model.TektonTaskRunSnapshot{
		Name:             taskRun.Name,
		PipelineTaskName: taskRun.PipelineTaskName,
		PodName:          taskRun.PodName,
		ContainerName:    taskRun.ContainerName,
		ContainerNames:   taskRun.ContainerNames,
		Status:           status,
		StartedAt:        taskRun.Status.StartedAt,
		FinishedAt:       taskRun.Status.FinishedAt,
	}
	if strings.TrimSpace(taskRun.Status.Message) != "" && status != "success" {
		snapshot.ErrorMessage = taskRun.Status.Message
	}
	if !snapshot.StartedAt.IsZero() {
		end := time.Now()
		if !snapshot.FinishedAt.IsZero() {
			end = snapshot.FinishedAt
		}
		snapshot.DurationSeconds = int64(end.Sub(snapshot.StartedAt).Seconds())
		if snapshot.DurationSeconds < 0 {
			snapshot.DurationSeconds = 0
		}
	}
	return snapshot
}

func tektonTaskRunRepresentative(items []model.TektonTaskRunSnapshot) model.TektonTaskRunSnapshot {
	if len(items) == 0 {
		return model.TektonTaskRunSnapshot{}
	}
	result := items[0]
	for _, item := range items {
		if tektonTaskRunStatusPriority(item.Status) > tektonTaskRunStatusPriority(result.Status) {
			result = item
		}
	}
	return result
}

func aggregateTektonTaskRunStatus(items []model.TektonTaskRunSnapshot) string {
	if len(items) == 0 {
		return "running"
	}
	result := strings.TrimSpace(items[0].Status)
	if result == "" {
		result = "running"
	}
	for _, item := range items {
		status := strings.TrimSpace(item.Status)
		if status == "" {
			status = "running"
		}
		if tektonTaskRunStatusPriority(status) > tektonTaskRunStatusPriority(result) {
			result = status
		}
	}
	return result
}

func tektonTaskRunStatusPriority(status string) int {
	switch strings.TrimSpace(status) {
	case "failed":
		return 100
	case "aborted":
		return 95
	case "unstable":
		return 90
	case "running":
		return 80
	case "preparing":
		return 70
	case "queued":
		return 60
	case "success":
		return 10
	case "skipped":
		return 5
	case "finished":
		return 10
	default:
		return 80
	}
}

func aggregateTektonTaskRunMessage(items []model.TektonTaskRunSnapshot, status string) string {
	if status == "success" {
		return ""
	}
	for _, item := range items {
		if strings.TrimSpace(item.Status) == strings.TrimSpace(status) && strings.TrimSpace(item.ErrorMessage) != "" {
			return item.ErrorMessage
		}
	}
	for _, item := range items {
		if strings.TrimSpace(item.ErrorMessage) != "" {
			return item.ErrorMessage
		}
	}
	return ""
}

func earliestTektonTaskRunStart(items []model.TektonTaskRunSnapshot) time.Time {
	var result time.Time
	for _, item := range items {
		if item.StartedAt.IsZero() {
			continue
		}
		if result.IsZero() || item.StartedAt.Before(result) {
			result = item.StartedAt
		}
	}
	return result
}

func latestTektonTaskRunFinish(items []model.TektonTaskRunSnapshot) time.Time {
	var result time.Time
	for _, item := range items {
		if item.FinishedAt.IsZero() {
			continue
		}
		if result.IsZero() || item.FinishedAt.After(result) {
			result = item.FinishedAt
		}
	}
	return result
}

func applyTektonSkippedTaskToStage(stage *model.DevopsPipelineRunStage, skipped devopstekton.SkippedTaskInfo, run *model.DevopsPipelineRun) {
	stage.Status = "skipped"
	if strings.TrimSpace(skipped.Message) != "" {
		stage.ErrorMessage = skipped.Message
	} else if strings.TrimSpace(skipped.Reason) != "" {
		stage.ErrorMessage = skipped.Reason
	}
	if stage.StartedAt.IsZero() && run != nil && !run.StartedAt.IsZero() {
		stage.StartedAt = run.StartedAt
	}
	if run != nil && !run.FinishedAt.IsZero() {
		stage.FinishedAt = run.FinishedAt
	} else if stage.FinishedAt.IsZero() {
		stage.FinishedAt = time.Now()
	}
	if !stage.StartedAt.IsZero() {
		stage.DurationSeconds = int64(stage.FinishedAt.Sub(stage.StartedAt).Seconds())
		if stage.DurationSeconds < 0 {
			stage.DurationSeconds = 0
		}
	}
}

func tektonClientForRun(ctx context.Context, svcCtx *svc.ServiceContext, run *model.DevopsPipelineRun, userID uint64, roles []string) (*devopstekton.Client, *pipelineconfigservice.ResolvePipelineRuntimeResp, error) {
	runtime, err := buildRuntimeCached(ctx, svcCtx, run.ProjectID, run.SystemID, run.EnvironmentID, run.BuildChannelBindingID, userID, roles)
	if err != nil {
		return nil, nil, err
	}
	client, err := devopstekton.NewClient(tektonRequestFromRuntime(runtime))
	if err != nil {
		return nil, nil, err
	}
	return client, runtime, nil
}
