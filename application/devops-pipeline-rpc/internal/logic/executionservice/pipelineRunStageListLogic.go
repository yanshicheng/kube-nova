package executionservicelogic

import (
	"context"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/yanshicheng/kube-nova/application/devops-pipeline-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/devops-pipeline-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-pipeline-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/devops/jenkins"

	"github.com/zeromicro/go-zero/core/logx"
)

var (
	generatedParallelGroupNamePattern = regexp.MustCompile(`^并行组-\d+$`)

	runStageSyncMu     sync.Mutex
	runStageSyncStates = make(map[string]runStageSyncState)
)

const (
	runStageSyncMinInterval = 2 * time.Second
	runStageSyncTimeout     = 12 * time.Second
)

type runStageSyncState struct {
	Running bool
	Last    time.Time
}

type PipelineRunStageListLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPipelineRunStageListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PipelineRunStageListLogic {
	return &PipelineRunStageListLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *PipelineRunStageListLogic) PipelineRunStageList(in *pb.ListPipelineRunStageReq) (*pb.ListPipelineRunStageResp, error) {
	run, err := l.svcCtx.PipelineRunModel.FindOne(l.ctx, in.RunId)
	if err != nil {
		l.Errorf("流水线运行阶段查询列表失败: %v", err)
		return nil, err
	}
	if err := ensureProjectAccess(l.ctx, l.svcCtx, run.ProjectID, in.CurrentUserId, in.CurrentRoles); err != nil {
		l.Errorf("流水线运行阶段查询列表失败: %v", err)
		return nil, err
	}
	items, err := l.svcCtx.RunStageModel.ListByRun(l.ctx, in.RunId)
	if err != nil {
		l.Errorf("流水线运行阶段查询列表失败: %v", err)
		return nil, err
	}
	if run.EngineType == engineTekton {
		if err := syncTektonRunSnapshot(l.ctx, l.svcCtx, run, in.CurrentUserId, in.CurrentRoles, run.UpdatedBy); err != nil {
			l.Errorf("同步 Tekton 阶段状态失败: %v", err)
		}
		items, err = l.svcCtx.RunStageModel.ListByRun(l.ctx, in.RunId)
		if err != nil {
			l.Errorf("流水线运行阶段查询列表失败: %v", err)
			return nil, err
		}
	}
	if shouldScheduleRunStageSync(run, items) {
		scheduleRunStageSync(l.svcCtx, in.RunId, in.CurrentUserId, in.CurrentRoles)
	}
	data := make([]*pb.PipelineRunStage, 0, len(items))
	for _, item := range items {
		data = append(data, runStageToPb(item))
	}

	return &pb.ListPipelineRunStageResp{Data: data}, nil
}

func shouldScheduleRunStageSync(run *model.DevopsPipelineRun, items []*model.DevopsPipelineRunStage) bool {
	if run == nil {
		return false
	}
	if isRunLogActive(run.Status) {
		return true
	}
	for _, item := range items {
		if item == nil || isPostRunStage(item) {
			continue
		}
		status := strings.ToLower(strings.TrimSpace(item.Status))
		if status == "" || status == "queued" || status == "running" || status == "paused" {
			return true
		}
		if isFinalRunStatus(status) && item.DurationSeconds <= 0 {
			return true
		}
	}
	return false
}

func scheduleRunStageSync(svcCtx *svc.ServiceContext, runID string, userID uint64, roles []string) {
	runID = strings.TrimSpace(runID)
	if svcCtx == nil || runID == "" || !markRunStageSyncStarted(runID) {
		return
	}
	roleCopy := append([]string(nil), roles...)
	go func() {
		defer finishRunStageSync(runID)
		ctx, cancel := context.WithTimeout(context.Background(), runStageSyncTimeout)
		defer cancel()
		if err := syncRunStageSnapshot(ctx, svcCtx, runID, userID, roleCopy); err != nil {
			logx.Errorf("后台同步流水线阶段状态失败: runId=%s err=%v", runID, err)
		}
	}()
}

func markRunStageSyncStarted(runID string) bool {
	now := time.Now()
	runStageSyncMu.Lock()
	defer runStageSyncMu.Unlock()
	state := runStageSyncStates[runID]
	if state.Running {
		return false
	}
	if !state.Last.IsZero() && now.Sub(state.Last) < runStageSyncMinInterval {
		return false
	}
	state.Running = true
	state.Last = now
	runStageSyncStates[runID] = state
	return true
}

func finishRunStageSync(runID string) {
	now := time.Now()
	runStageSyncMu.Lock()
	defer runStageSyncMu.Unlock()
	state := runStageSyncStates[runID]
	state.Running = false
	state.Last = now
	runStageSyncStates[runID] = state
	for key, item := range runStageSyncStates {
		if !item.Running && now.Sub(item.Last) > time.Hour {
			delete(runStageSyncStates, key)
		}
	}
}

func syncRunStageSnapshot(ctx context.Context, svcCtx *svc.ServiceContext, runID string, userID uint64, roles []string) error {
	run, err := svcCtx.PipelineRunModel.FindOne(ctx, runID)
	if err != nil {
		return err
	}
	if run.EngineType == engineTekton {
		return syncTektonRunSnapshot(ctx, svcCtx, run, userID, roles, run.UpdatedBy)
	}
	items, err := svcCtx.RunStageModel.ListByRun(ctx, runID)
	if err != nil {
		return err
	}
	runtime, err := buildRuntimeCached(ctx, svcCtx, run.ProjectID, run.SystemID, run.EnvironmentID, run.BuildChannelBindingID, userID, roles)
	if err != nil {
		return err
	}
	manager := jenkinsManagerFromRuntime(runtime)
	ready, err := ensureRunBuildNumber(ctx, svcCtx, run, manager, "")
	if err != nil {
		return err
	}
	if !ready {
		return nil
	}
	pipelineSteps := pipelineStepsForRun(ctx, svcCtx, run)
	if shouldSyncJenkinsRunStatus(run.ID.Hex()) {
		if err := syncRunStatusFromJenkins(ctx, svcCtx, run, manager, ""); err != nil {
			logx.Errorf("同步 Jenkins 构建状态失败: runId=%s buildNumber=%d err=%v", runID, run.JenkinsBuildNumber, err)
		}
	}
	stages, err := manager.StageStatus(ctx, run.JenkinsJobFullName, run.JenkinsBuildNumber)
	if err != nil {
		logx.Errorf("同步 Jenkins 阶段状态失败: runId=%s buildNumber=%d err=%v", runID, run.JenkinsBuildNumber, err)
		return syncRunStagesFromConsoleLog(ctx, svcCtx, items, run, manager, pipelineSteps)
	}
	stages = expandJenkinsStageStatusesForRun(ctx, manager, run, stages, items, pipelineSteps)
	if len(stages) > 0 {
		if err := syncRunStages(ctx, svcCtx, items, stages, run.Status, pipelineSteps); err != nil {
			return err
		}
		if hasPausedJenkinsStage(stages) && !isFinalRunStatus(run.Status) && run.Status != "paused" {
			run.Status = "paused"
			if err := svcCtx.PipelineRunModel.Update(ctx, run); err != nil {
				logx.Errorf("同步 Jenkins 人工确认状态失败: %v", err)
				return err
			}
			_ = svcCtx.PipelineModel.UpdateRunStatus(ctx, run.PipelineID, "paused", run.UpdatedBy)
		}
	}
	if len(stages) == 0 || hasStageNeedConsoleFallback(items) {
		return syncRunStagesFromConsoleLog(ctx, svcCtx, items, run, manager, pipelineSteps)
	}
	return nil
}

func hasPausedJenkinsStage(stages []jenkins.StageStatus) bool {
	for _, stage := range stages {
		if normalizeJenkinsStageStatus(stage.Status) == "paused" {
			return true
		}
	}
	return false
}

func syncRunStages(ctx context.Context, svcCtx *svc.ServiceContext, items []*model.DevopsPipelineRunStage, stages []jenkins.StageStatus, runStatus string, pipelineSteps []model.PipelineStep) error {
	if len(items) == 0 || len(stages) == 0 {
		return nil
	}
	now := time.Now()
	snapshots := snapshotRunStages(items)
	matches := matchJenkinsStages(items, stages)
	effectiveStages := buildEffectiveJenkinsStages(stages)
	for _, item := range items {
		stage, ok := matches[item]
		if !ok {
			continue
		}
		if effective, exists := effectiveStages[strings.TrimSpace(stage.ID)]; exists {
			stage = effective
		}
		item.Status = normalizeJenkinsStageStatus(stage.Status)
		item.JenkinsNodeID = stage.ID
		if stage.Duration > 0 {
			item.DurationSeconds = stageDurationSeconds(stage.Duration)
		}
		if stage.Start > 0 {
			item.StartedAt = time.UnixMilli(stage.Start)
			if stage.Duration > 0 && item.Status != "running" {
				item.FinishedAt = time.UnixMilli(stage.Start + stage.Duration)
			}
		}
		if item.Status == "running" || item.Status == "paused" {
			item.FinishedAt = time.Time{}
			if !item.StartedAt.IsZero() {
				duration := int64(now.Sub(item.StartedAt).Seconds())
				if duration < 0 {
					duration = 0
				}
				if duration > item.DurationSeconds {
					item.DurationSeconds = duration
				}
			}
		}
	}
	if isRunLogActive(runStatus) {
		guardActiveRunStageDependencies(items, pipelineSteps)
	}
	for _, item := range items {
		if item == nil || !runStageChanged(item, snapshots[item]) {
			continue
		}
		if err := svcCtx.RunStageModel.Update(ctx, item); err != nil {
			logx.Errorf("查询流水线阶段列表失败: %v", err)
			return err
		}
	}
	return nil
}

func syncQueuedRunStagesByFinalRun(ctx context.Context, svcCtx *svc.ServiceContext, items []*model.DevopsPipelineRunStage, run *model.DevopsPipelineRun) error {
	if run == nil || !isFinalRunStatus(run.Status) {
		return nil
	}
	status := finalRunStageFallbackStatus(run.Status)
	if status == "" {
		return nil
	}
	finishedAt := run.FinishedAt
	if finishedAt.IsZero() {
		finishedAt = time.Now()
	}
	for _, item := range items {
		if item == nil || !isPendingStageStatus(item.Status) {
			continue
		}
		item.Status = status
		if item.StartedAt.IsZero() && !run.StartedAt.IsZero() {
			item.StartedAt = run.StartedAt
		}
		if item.FinishedAt.IsZero() {
			item.FinishedAt = finishedAt
		}
		if item.DurationSeconds <= 0 && !item.StartedAt.IsZero() && !item.FinishedAt.IsZero() {
			item.DurationSeconds = int64(item.FinishedAt.Sub(item.StartedAt).Seconds())
			if item.DurationSeconds < 0 {
				item.DurationSeconds = 0
			}
		}
		if err := svcCtx.RunStageModel.Update(ctx, item); err != nil {
			logx.Errorf("按流水线最终状态同步阶段失败: %v", err)
			return err
		}
	}
	return nil
}

func syncActiveRunStagesFromConsoleLog(ctx context.Context, svcCtx *svc.ServiceContext, items []*model.DevopsPipelineRunStage, stageNames []string, fullLog string, pipelineSteps []model.PipelineStep) error {
	if len(items) == 0 || len(stageNames) == 0 {
		return nil
	}
	now := time.Now()
	snapshots := snapshotRunStages(items)
	stages := buildActiveConsoleStageStatuses(stageNames, fullLog, pipelineSteps)
	matches := matchJenkinsStages(items, stages)
	for _, item := range items {
		if item == nil || strings.TrimSpace(item.JenkinsNodeID) != "" {
			continue
		}
		stage, ok := matches[item]
		if !ok {
			continue
		}
		status := normalizeJenkinsStageStatus(stage.Status)
		if isFinalRunStatus(item.Status) && !isFinalRunStatus(status) {
			continue
		}
		item.Status = status
		if item.StartedAt.IsZero() {
			item.StartedAt = now
		}
		if status == "success" {
			if item.FinishedAt.IsZero() {
				item.FinishedAt = now
			}
			duration := int64(item.FinishedAt.Sub(item.StartedAt).Seconds())
			if duration <= 0 && item.DurationSeconds <= 0 {
				duration = 1
			}
			if duration > item.DurationSeconds {
				item.DurationSeconds = duration
			}
		}
		if status == "running" {
			duration := int64(now.Sub(item.StartedAt).Seconds())
			if duration < 0 {
				duration = 0
			}
			if duration > item.DurationSeconds {
				item.DurationSeconds = duration
			}
		}
		if status == "paused" {
			duration := int64(now.Sub(item.StartedAt).Seconds())
			if duration < 0 {
				duration = 0
			}
			if duration > item.DurationSeconds {
				item.DurationSeconds = duration
			}
		}
	}
	guardActiveRunStageDependencies(items, pipelineSteps)
	for _, item := range items {
		if item == nil || !runStageChanged(item, snapshots[item]) {
			continue
		}
		if err := svcCtx.RunStageModel.Update(ctx, item); err != nil {
			logx.Errorf("从 Jenkins 实时日志同步阶段状态失败: %v", err)
			return err
		}
	}
	return nil
}

func buildActiveConsoleStageStatuses(stageNames []string, fullLog string, pipelineSteps []model.PipelineStep) []jenkins.StageStatus {
	if !hasParallelPipelineBranches(pipelineSteps) {
		stages := make([]jenkins.StageStatus, 0, len(stageNames))
		for index, name := range stageNames {
			status := "SUCCESS"
			if index == len(stageNames)-1 {
				status = "IN_PROGRESS"
				if consoleStageWaitingInput(fullLog, name) {
					status = "PAUSED_PENDING_INPUT"
				}
			}
			stages = append(stages, jenkins.StageStatus{Name: name, Status: status})
		}
		return stages
	}
	states := extractConsoleStageRuntimeStates(fullLog)
	stages := make([]jenkins.StageStatus, 0, len(stageNames))
	for _, name := range stageNames {
		name = strings.TrimSpace(name)
		state, ok := states[name]
		if !ok || !state.Opened {
			continue
		}
		status := "IN_PROGRESS"
		if state.Closed {
			status = "SUCCESS"
		}
		if state.WaitingInput {
			status = "PAUSED_PENDING_INPUT"
		}
		stages = append(stages, jenkins.StageStatus{Name: name, Status: status})
	}
	return stages
}

type consoleStageRuntimeState struct {
	Opened       bool
	Closed       bool
	WaitingInput bool
}

func extractConsoleStageRuntimeStates(fullLog string) map[string]consoleStageRuntimeState {
	result := make(map[string]consoleStageRuntimeState)
	lines := strings.Split(strings.ReplaceAll(fullLog, "\r\n", "\n"), "\n")
	activeStages := make([]string, 0)
	for _, line := range lines {
		if name, ok := extractConsoleStageName(line); ok && isMatchableJenkinsStageName(name) {
			state := result[name]
			state.Opened = true
			result[name] = state
			activeStages = append(activeStages, name)
			continue
		}
		if len(activeStages) == 0 {
			continue
		}
		name := activeStages[len(activeStages)-1]
		state := result[name]
		lower := strings.ToLower(line)
		if strings.Contains(lower, "[pipeline] input") ||
			strings.Contains(lower, "paused for input") ||
			strings.Contains(lower, "input requested") {
			state.WaitingInput = true
		}
		if strings.Contains(line, "[Pipeline] // stage") {
			state.Closed = true
			activeStages = activeStages[:len(activeStages)-1]
		}
		result[name] = state
	}
	return result
}

func consoleStageWaitingInput(fullLog, stageName string) bool {
	content := strings.ToLower(extractStageConsoleLog(fullLog, stageName))
	if strings.TrimSpace(content) == "" {
		return false
	}
	return strings.Contains(content, "[pipeline] input") ||
		strings.Contains(content, "paused for input") ||
		strings.Contains(content, "input requested")
}

func isPendingStageStatus(status string) bool {
	status = strings.ToLower(strings.TrimSpace(status))
	return status == "" || status == "queued"
}

type runStageSnapshot struct {
	Status          string
	StartedAt       time.Time
	FinishedAt      time.Time
	DurationSeconds int64
	JenkinsNodeID   string
}

func snapshotRunStages(items []*model.DevopsPipelineRunStage) map[*model.DevopsPipelineRunStage]runStageSnapshot {
	result := make(map[*model.DevopsPipelineRunStage]runStageSnapshot, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		result[item] = runStageSnapshot{
			Status:          item.Status,
			StartedAt:       item.StartedAt,
			FinishedAt:      item.FinishedAt,
			DurationSeconds: item.DurationSeconds,
			JenkinsNodeID:   item.JenkinsNodeID,
		}
	}
	return result
}

func runStageChanged(item *model.DevopsPipelineRunStage, snapshot runStageSnapshot) bool {
	if item == nil {
		return false
	}
	return item.Status != snapshot.Status ||
		!item.StartedAt.Equal(snapshot.StartedAt) ||
		!item.FinishedAt.Equal(snapshot.FinishedAt) ||
		item.DurationSeconds != snapshot.DurationSeconds ||
		item.JenkinsNodeID != snapshot.JenkinsNodeID
}

func pipelineStepsForRun(ctx context.Context, svcCtx *svc.ServiceContext, run *model.DevopsPipelineRun) []model.PipelineStep {
	if run == nil || strings.TrimSpace(run.PipelineID) == "" {
		return nil
	}
	pipeline, err := svcCtx.PipelineModel.FindOne(ctx, run.PipelineID)
	if err != nil {
		logx.Errorf("读取流水线步骤拓扑失败: %v", err)
		return nil
	}
	return pipeline.Steps
}

func guardActiveRunStageDependencies(items []*model.DevopsPipelineRunStage, steps []model.PipelineStep) {
	if len(items) == 0 || len(steps) == 0 {
		return
	}
	dependencies := buildRunStageDependencyMap(items, steps)
	if len(dependencies) == 0 {
		return
	}
	stageByNodeID := make(map[string]*model.DevopsPipelineRunStage, len(items))
	for _, item := range items {
		if item == nil || strings.TrimSpace(item.NodeID) == "" {
			continue
		}
		stageByNodeID[item.NodeID] = item
	}
	for pass := 0; pass < len(items); pass++ {
		changed := false
		for _, item := range items {
			if item == nil || isPostRunStage(item) {
				continue
			}
			deps := dependencies[item.NodeID]
			if len(deps) == 0 || runStageDependenciesReady(stageByNodeID, deps) {
				continue
			}
			if isPendingStageStatus(item.Status) && item.StartedAt.IsZero() && item.FinishedAt.IsZero() && item.DurationSeconds == 0 && strings.TrimSpace(item.JenkinsNodeID) == "" {
				continue
			}
			item.Status = "queued"
			item.StartedAt = time.Time{}
			item.FinishedAt = time.Time{}
			item.DurationSeconds = 0
			item.JenkinsNodeID = ""
			changed = true
		}
		if !changed {
			break
		}
	}
}

func buildRunStageDependencyMap(items []*model.DevopsPipelineRunStage, steps []model.PipelineStep) map[string][]string {
	stageByNodeID := make(map[string]*model.DevopsPipelineRunStage, len(items))
	for _, item := range items {
		if item == nil || strings.TrimSpace(item.NodeID) == "" || isPostRunStage(item) {
			continue
		}
		stageByNodeID[item.NodeID] = item
	}
	if len(stageByNodeID) == 0 {
		return nil
	}

	normalized := make([]model.PipelineStep, 0, len(steps))
	for _, step := range steps {
		if strings.TrimSpace(step.ID) == "" {
			continue
		}
		if _, ok := stageByNodeID[step.ID]; !ok {
			continue
		}
		normalized = append(normalized, step)
	}
	normalizePipelineStepBranches(normalized)

	childrenMap := make(map[string][]model.PipelineStep)
	for _, step := range normalized {
		parentID := strings.TrimSpace(step.ParentNodeID)
		if parentID == "" {
			parentID = workflowStartNodeID
		}
		childrenMap[parentID] = append(childrenMap[parentID], step)
	}
	for parentID := range childrenMap {
		sort.SliceStable(childrenMap[parentID], func(i, j int) bool {
			return compareRunStageStep(childrenMap[parentID][i], childrenMap[parentID][j])
		})
	}

	dependencies := make(map[string][]string)
	placed := make(map[string]struct{})
	var walk func(parentID string, sources []string, stack map[string]struct{}) []string
	placeStep := func(step model.PipelineStep, sources []string) []string {
		if strings.TrimSpace(step.ID) == "" {
			return sources
		}
		if _, ok := placed[step.ID]; ok {
			return sources
		}
		placed[step.ID] = struct{}{}
		dependencies[step.ID] = uniqueStageNodeIDs(sources)
		return []string{step.ID}
	}
	walk = func(parentID string, sources []string, stack map[string]struct{}) []string {
		if _, ok := stack[parentID]; ok {
			return sources
		}
		nextStack := make(map[string]struct{}, len(stack)+1)
		for key := range stack {
			nextStack[key] = struct{}{}
		}
		nextStack[parentID] = struct{}{}

		children := childrenMap[parentID]
		if len(children) == 0 {
			return sources
		}
		parallelChildren := make([]model.PipelineStep, 0)
		nextChildren := make([]model.PipelineStep, 0)
		for _, child := range children {
			if strings.TrimSpace(child.BranchType) == "parallel" {
				parallelChildren = append(parallelChildren, child)
			} else {
				nextChildren = append(nextChildren, child)
			}
		}

		currentSources := sources
		if len(parallelChildren) >= 2 {
			parallelEndSources := make([]string, 0, len(parallelChildren))
			for _, child := range parallelChildren {
				childSources := placeStep(child, sources)
				parallelEndSources = append(parallelEndSources, walk(child.ID, childSources, nextStack)...)
			}
			currentSources = uniqueStageNodeIDs(parallelEndSources)
		}
		for _, child := range nextChildren {
			childSources := placeStep(child, currentSources)
			currentSources = walk(child.ID, childSources, nextStack)
		}
		return currentSources
	}

	walk(workflowStartNodeID, nil, map[string]struct{}{})
	return dependencies
}

func buildEffectiveJenkinsStages(stages []jenkins.StageStatus) map[string]jenkins.StageStatus {
	byID := make(map[string]jenkins.StageStatus, len(stages))
	childrenByParent := make(map[string][]jenkins.StageStatus, len(stages))
	for _, stage := range stages {
		stage.ID = strings.TrimSpace(stage.ID)
		stage.ParentID = strings.TrimSpace(stage.ParentID)
		if stage.ID == "" {
			continue
		}
		byID[stage.ID] = stage
		if stage.ParentID != "" {
			childrenByParent[stage.ParentID] = append(childrenByParent[stage.ParentID], stage)
		}
	}
	cache := make(map[string]jenkins.StageStatus, len(byID))
	var resolve func(id string) jenkins.StageStatus
	resolve = func(id string) jenkins.StageStatus {
		if stage, ok := cache[id]; ok {
			return stage
		}
		current := byID[id]
		children := childrenByParent[id]
		if len(children) == 0 {
			cache[id] = current
			return current
		}
		effective := current
		statuses := make([]string, 0, len(children)+1)
		statuses = append(statuses, normalizeJenkinsStageStatus(current.Status))
		earliestStart := current.Start
		latestEnd := stageEndMillis(current)
		maxDuration := current.Duration
		for _, child := range children {
			childEffective := resolve(child.ID)
			statuses = append(statuses, normalizeJenkinsStageStatus(childEffective.Status))
			earliestStart = minPositiveStageMillis(earliestStart, childEffective.Start)
			latestEnd = maxStageMillis(latestEnd, stageEndMillis(childEffective))
			if childEffective.Duration > maxDuration {
				maxDuration = childEffective.Duration
			}
		}
		effective.Status = aggregateJenkinsStageStatuses(statuses, current.Status)
		effective.Start = earliestStart
		if earliestStart > 0 && latestEnd > earliestStart {
			effective.Duration = latestEnd - earliestStart
		} else {
			effective.Duration = maxDuration
		}
		cache[id] = effective
		return effective
	}
	for id := range byID {
		cache[id] = resolve(id)
	}
	return cache
}

func aggregateJenkinsStageStatuses(statuses []string, fallback string) string {
	has := func(target string) bool {
		for _, status := range statuses {
			if status == target {
				return true
			}
		}
		return false
	}
	switch {
	case has("paused"):
		return "PAUSED_PENDING_INPUT"
	case has("running"):
		return "IN_PROGRESS"
	case has("failed"):
		return "FAILED"
	case has("aborted"):
		return "ABORTED"
	case has("unstable"):
		return "UNSTABLE"
	case has("queued"):
		return "NOT_EXECUTED"
	case has("success"):
		return "SUCCESS"
	case has("skipped"):
		return "SKIPPED"
	case has("finished"):
		return "SUCCESS"
	default:
		return fallback
	}
}

func stageEndMillis(stage jenkins.StageStatus) int64 {
	if stage.Start <= 0 || stage.Duration <= 0 {
		return 0
	}
	return stage.Start + stage.Duration
}

func stageDurationSeconds(durationMillis int64) int64 {
	if durationMillis <= 0 {
		return 0
	}
	seconds := durationMillis / 1000
	if seconds <= 0 {
		return 1
	}
	return seconds
}

func minPositiveStageMillis(current, next int64) int64 {
	switch {
	case current <= 0:
		return next
	case next <= 0:
		return current
	case next < current:
		return next
	default:
		return current
	}
}

func maxStageMillis(current, next int64) int64 {
	if next > current {
		return next
	}
	return current
}

func runStageDependenciesReady(stageByNodeID map[string]*model.DevopsPipelineRunStage, dependencies []string) bool {
	for _, nodeID := range dependencies {
		stage := stageByNodeID[nodeID]
		if stage == nil || !runStageDependencyReady(stage.Status) {
			return false
		}
	}
	return true
}

func runStageDependencyReady(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "success", "unstable", "skipped", "finished":
		return true
	default:
		return false
	}
}

func compareRunStageStep(left, right model.PipelineStep) bool {
	leftOrder := left.SortOrder
	rightOrder := right.SortOrder
	if leftOrder <= 0 {
		leftOrder = 1<<63 - 1
	}
	if rightOrder <= 0 {
		rightOrder = 1<<63 - 1
	}
	if leftOrder != rightOrder {
		return leftOrder < rightOrder
	}
	if left.X != right.X {
		return left.X < right.X
	}
	if left.Y != right.Y {
		return left.Y < right.Y
	}
	return left.ID < right.ID
}

func uniqueStageNodeIDs(items []string) []string {
	seen := make(map[string]struct{}, len(items))
	result := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		result = append(result, item)
	}
	return result
}

func finalRunStageFallbackStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "success":
		return "success"
	case "failed", "failure":
		return "failed"
	case "aborted":
		return "aborted"
	case "unstable":
		return "unstable"
	case "skipped":
		return "skipped"
	case "finished":
		return "finished"
	default:
		return ""
	}
}

func hasQueuedRunStages(items []*model.DevopsPipelineRunStage) bool {
	for _, item := range items {
		if item == nil {
			continue
		}
		if strings.TrimSpace(item.Status) == "" || strings.EqualFold(item.Status, "queued") {
			return true
		}
	}
	return false
}

func hasActiveOrQueuedRunStages(items []*model.DevopsPipelineRunStage) bool {
	for _, item := range items {
		if item == nil {
			continue
		}
		status := strings.ToLower(strings.TrimSpace(item.Status))
		if status == "" || status == "queued" || status == "running" || status == "paused" {
			return true
		}
	}
	return false
}

func hasStageNeedConsoleFallback(items []*model.DevopsPipelineRunStage) bool {
	for _, item := range items {
		if item == nil || isPostRunStage(item) || strings.TrimSpace(item.JenkinsNodeID) != "" {
			continue
		}
		status := strings.ToLower(strings.TrimSpace(item.Status))
		if status == "" || status == "queued" || status == "running" || status == "paused" {
			return true
		}
		if isFinalRunStatus(status) && item.DurationSeconds <= 0 {
			return true
		}
	}
	return false
}

func syncRunStagesFromConsoleLog(ctx context.Context, svcCtx *svc.ServiceContext, items []*model.DevopsPipelineRunStage, run *model.DevopsPipelineRun, manager *jenkins.Manager, pipelineSteps []model.PipelineStep) error {
	if run == nil {
		return nil
	}
	fullLog, err := readJenkinsConsoleSnapshot(ctx, run, manager)
	if err != nil {
		logx.Errorf("从 Jenkins 全量日志同步阶段状态失败: %v", err)
		fullLog, err = manager.ConsoleText(ctx, run.JenkinsJobFullName, run.JenkinsBuildNumber)
		if err != nil {
			logx.Errorf("从 Jenkins 全量日志同步阶段状态失败: %v", err)
			return nil
		}
	}
	stageNames := extractConsoleStageNames(fullLog)
	if len(stageNames) == 0 {
		return syncQueuedRunStagesByFinalRun(ctx, svcCtx, items, run)
	}
	if !isFinalRunStatus(run.Status) {
		return syncActiveRunStagesFromConsoleLog(ctx, svcCtx, items, stageNames, fullLog, pipelineSteps)
	}
	fallbackStatus := consoleFallbackStageStatus(run.Status)
	if fallbackStatus == "" {
		return syncQueuedRunStagesByFinalRun(ctx, svcCtx, items, run)
	}
	stages := make([]jenkins.StageStatus, 0, len(stageNames))
	for _, name := range stageNames {
		stages = append(stages, jenkins.StageStatus{Name: name, Status: fallbackStatus})
	}
	matches := matchJenkinsStages(items, stages)
	times := extractConsoleStageTimes(fullLog)
	for _, item := range items {
		stage, ok := matches[item]
		if !ok {
			continue
		}
		item.Status = normalizeJenkinsStageStatus(stage.Status)
		applyStageTimeFallback(item, run, times[strings.TrimSpace(stage.Name)], time.Now())
		if err := svcCtx.RunStageModel.Update(ctx, item); err != nil {
			logx.Errorf("从 Jenkins 全量日志同步阶段状态失败: %v", err)
			return err
		}
	}
	if hasQueuedRunStages(items) {
		if err := syncQueuedRunStagesByFinalRun(ctx, svcCtx, items, run); err != nil {
			logx.Errorf("从 Jenkins 全量日志同步阶段状态失败: %v", err)
			return err
		}
	}
	return nil
}

func expandJenkinsStageStatusesForRun(
	ctx context.Context,
	manager *jenkins.Manager,
	run *model.DevopsPipelineRun,
	stages []jenkins.StageStatus,
	items []*model.DevopsPipelineRunStage,
	pipelineSteps []model.PipelineStep,
) []jenkins.StageStatus {
	if manager == nil || run == nil || len(stages) == 0 || !hasParallelPipelineBranches(pipelineSteps) {
		return stages
	}
	stageNames := runStageNameSet(items)
	seenIDs := make(map[string]struct{}, len(stages))
	result := make([]jenkins.StageStatus, 0, len(stages))
	queue := make([]jenkins.StageStatus, 0)
	for _, stage := range stages {
		stage.ID = strings.TrimSpace(stage.ID)
		stage.Name = strings.TrimSpace(stage.Name)
		if stage.ID != "" {
			seenIDs[stage.ID] = struct{}{}
		}
		result = append(result, stage)
		if shouldExpandJenkinsStageChildren(stage, stageNames) {
			queue = append(queue, stage)
		}
	}
	for len(queue) > 0 {
		stage := queue[0]
		queue = queue[1:]
		if strings.TrimSpace(stage.ID) == "" {
			continue
		}
		children, err := manager.StageChildStatuses(ctx, run.JenkinsJobFullName, run.JenkinsBuildNumber, stage.ID)
		if err != nil {
			continue
		}
		for _, child := range children {
			child.ID = strings.TrimSpace(child.ID)
			child.ParentID = stage.ID
			child.Name = strings.TrimSpace(child.Name)
			if child.ID == "" {
				continue
			}
			if _, ok := seenIDs[child.ID]; ok {
				continue
			}
			seenIDs[child.ID] = struct{}{}
			result = append(result, child)
			if shouldExpandJenkinsStageChildren(child, stageNames) {
				queue = append(queue, child)
			}
		}
	}
	return result
}

func hasParallelPipelineBranches(steps []model.PipelineStep) bool {
	for _, step := range steps {
		if strings.EqualFold(strings.TrimSpace(step.BranchType), "parallel") {
			return true
		}
	}
	return false
}

func runStageNameSet(items []*model.DevopsPipelineRunStage) map[string]struct{} {
	result := make(map[string]struct{}, len(items))
	for _, item := range items {
		if item == nil || isPostRunStage(item) {
			continue
		}
		name := strings.TrimSpace(item.StageName)
		if name == "" {
			continue
		}
		result[name] = struct{}{}
	}
	return result
}

func shouldExpandJenkinsStageChildren(stage jenkins.StageStatus, stageNames map[string]struct{}) bool {
	name := strings.TrimSpace(stage.Name)
	if name == "" {
		return false
	}
	lower := strings.ToLower(name)
	return strings.HasPrefix(lower, "branch:") ||
		strings.HasSuffix(name, "-分支") ||
		generatedParallelGroupNamePattern.MatchString(name) ||
		!isUserJenkinsStage(name) ||
		shouldExpandMatchedUserStage(stage, stageNames)
}

func shouldExpandMatchedUserStage(stage jenkins.StageStatus, stageNames map[string]struct{}) bool {
	name := strings.TrimSpace(stage.Name)
	if name == "" {
		return false
	}
	if _, ok := stageNames[name]; !ok {
		return false
	}
	status := normalizeJenkinsStageStatus(stage.Status)
	if status != "running" && status != "paused" && status != "success" {
		return false
	}
	return true
}

type consoleStageTime struct {
	Start time.Time
	End   time.Time
}

func applyStageTimeFallback(item *model.DevopsPipelineRunStage, run *model.DevopsPipelineRun, stageTime consoleStageTime, now time.Time) {
	if item == nil {
		return
	}
	if item.StartedAt.IsZero() && !stageTime.Start.IsZero() {
		item.StartedAt = stageTime.Start
	}
	if item.FinishedAt.IsZero() && !stageTime.End.IsZero() {
		item.FinishedAt = stageTime.End
	}
	if item.DurationSeconds <= 0 && !item.StartedAt.IsZero() && !item.FinishedAt.IsZero() {
		item.DurationSeconds = int64(item.FinishedAt.Sub(item.StartedAt).Seconds())
	}
	if item.DurationSeconds <= 0 && isFinalRunStatus(item.Status) && !run.StartedAt.IsZero() {
		if item.StartedAt.IsZero() {
			item.StartedAt = run.StartedAt
		}
		if item.FinishedAt.IsZero() {
			if !run.FinishedAt.IsZero() {
				item.FinishedAt = run.FinishedAt
			} else {
				item.FinishedAt = now
			}
		}
		item.DurationSeconds = int64(item.FinishedAt.Sub(item.StartedAt).Seconds())
	}
	if item.DurationSeconds <= 0 && isFinalRunStatus(item.Status) {
		item.DurationSeconds = 1
	}
	if item.DurationSeconds < 0 {
		item.DurationSeconds = 0
	}
}

func readJenkinsConsoleSnapshot(ctx context.Context, run *model.DevopsPipelineRun, manager *jenkins.Manager) (string, error) {
	if run == nil {
		return "", nil
	}
	var builder strings.Builder
	var offset int64
	for i := 0; i < 20; i++ {
		chunk, next, more, err := manager.ProgressiveLog(ctx, run.JenkinsJobFullName, run.JenkinsBuildNumber, offset)
		if err != nil {
			if builder.Len() > 0 {
				return builder.String(), nil
			}
			return manager.ConsoleText(ctx, run.JenkinsJobFullName, run.JenkinsBuildNumber)
		}
		if chunk != "" {
			builder.WriteString(chunk)
		}
		if !more || next <= offset {
			break
		}
		offset = next
	}
	if strings.TrimSpace(builder.String()) != "" {
		return builder.String(), nil
	}
	return manager.ConsoleText(ctx, run.JenkinsJobFullName, run.JenkinsBuildNumber)
}

var consoleTimestampPattern = regexp.MustCompile(`^\[(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?Z?)\]`)

func extractConsoleStageTimes(fullLog string) map[string]consoleStageTime {
	result := make(map[string]consoleStageTime)
	lines := strings.Split(strings.ReplaceAll(fullLog, "\r\n", "\n"), "\n")
	current := ""
	for _, line := range lines {
		tm := parseConsoleLineTime(line)
		if match := consoleStageMarkerPattern.FindStringSubmatch(line); len(match) >= 2 {
			name := strings.TrimSpace(match[1])
			if !isMatchableJenkinsStageName(name) {
				continue
			}
			current = name
			stageTime := result[current]
			if stageTime.Start.IsZero() && !tm.IsZero() {
				stageTime.Start = tm
			}
			if !tm.IsZero() {
				stageTime.End = tm
			}
			result[current] = stageTime
			continue
		}
		if current == "" || tm.IsZero() {
			continue
		}
		stageTime := result[current]
		if stageTime.Start.IsZero() {
			stageTime.Start = tm
		}
		stageTime.End = tm
		result[current] = stageTime
	}
	return result
}

func parseConsoleLineTime(line string) time.Time {
	match := consoleTimestampPattern.FindStringSubmatch(strings.TrimSpace(line))
	if len(match) < 2 {
		return time.Time{}
	}
	value := match[1]
	formats := []string{
		"2006-01-02T15:04:05.000Z",
		"2006-01-02T15:04:05.999Z",
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05.000",
		"2006-01-02T15:04:05.999",
		"2006-01-02T15:04:05",
	}
	for _, layout := range formats {
		if tm, err := time.Parse(layout, value); err == nil {
			return tm
		}
	}
	return time.Time{}
}

func consoleFallbackStageStatus(runStatus string) string {
	switch strings.ToLower(strings.TrimSpace(runStatus)) {
	case "success":
		return "SUCCESS"
	case "failed", "failure":
		return "FAILED"
	case "aborted":
		return "ABORTED"
	case "unstable":
		return "UNSTABLE"
	default:
		return ""
	}
}

func matchJenkinsStages(items []*model.DevopsPipelineRunStage, stages []jenkins.StageStatus) map[*model.DevopsPipelineRunStage]jenkins.StageStatus {
	result := make(map[*model.DevopsPipelineRunStage]jenkins.StageStatus)
	used := make([]bool, len(stages))
	for _, item := range items {
		if item == nil || isPostRunStage(item) {
			continue
		}
		if index := findJenkinsStageIndex(stages, used, item.StageName, false); index >= 0 {
			result[item] = stages[index]
			used[index] = true
		}
	}
	for _, item := range items {
		if item == nil || isPostRunStage(item) {
			continue
		}
		if _, ok := result[item]; ok {
			continue
		}
		if index := findJenkinsStageIndex(stages, used, item.StageName, true); index >= 0 {
			result[item] = stages[index]
			used[index] = true
		}
	}
	for _, item := range items {
		if item == nil || isPostRunStage(item) {
			continue
		}
		if _, ok := result[item]; ok {
			continue
		}
		for index, stage := range stages {
			if used[index] || !isMatchableJenkinsStageName(stage.Name) {
				continue
			}
			result[item] = stage
			used[index] = true
			break
		}
	}
	return result
}

func isPostRunStage(item *model.DevopsPipelineRunStage) bool {
	return item != nil && strings.EqualFold(strings.TrimSpace(item.StageType), "post")
}

func findJenkinsStageIndex(stages []jenkins.StageStatus, used []bool, name string, fuzzy bool) int {
	name = strings.TrimSpace(name)
	if name == "" {
		return -1
	}
	normalizedName := normalizeStageMatchName(name)
	bestIndex := -1
	bestScore := -1
	for index, stage := range stages {
		if used[index] {
			continue
		}
		stageName := strings.TrimSpace(stage.Name)
		if !isMatchableJenkinsStageName(stageName) {
			continue
		}
		matchScore := stageNameMatchScore(stageName, name, normalizedName, fuzzy)
		if matchScore < 0 {
			continue
		}
		score := matchScore*100 + jenkinsStageStructureScore(stages, stage)
		if score > bestScore {
			bestScore = score
			bestIndex = index
		}
	}
	return bestIndex
}

func stageNameMatchScore(stageName, name, normalizedName string, fuzzy bool) int {
	stageName = strings.TrimSpace(stageName)
	if stageName == "" {
		return -1
	}
	normalizedStageName := normalizeStageMatchName(stageName)
	switch {
	case stageName == name:
		return 3
	case normalizedStageName == normalizedName:
		return 2
	case fuzzy && (strings.Contains(normalizedStageName, normalizedName) || strings.Contains(normalizedName, normalizedStageName)):
		return 1
	default:
		return -1
	}
}

func jenkinsStageStructureScore(stages []jenkins.StageStatus, stage jenkins.StageStatus) int {
	score := 0
	if !jenkinsStageHasChildren(stages, stage.ID) {
		score += 10
	}
	score += jenkinsStageDepth(stages, stage.ID)
	return score
}

func jenkinsStageHasChildren(stages []jenkins.StageStatus, id string) bool {
	id = strings.TrimSpace(id)
	if id == "" {
		return false
	}
	for _, stage := range stages {
		if strings.TrimSpace(stage.ParentID) == id {
			return true
		}
	}
	return false
}

func jenkinsStageDepth(stages []jenkins.StageStatus, id string) int {
	id = strings.TrimSpace(id)
	if id == "" {
		return 0
	}
	parentByID := make(map[string]string, len(stages))
	for _, stage := range stages {
		stageID := strings.TrimSpace(stage.ID)
		if stageID == "" {
			continue
		}
		parentByID[stageID] = strings.TrimSpace(stage.ParentID)
	}
	depth := 0
	seen := make(map[string]struct{})
	current := id
	for current != "" {
		parent := parentByID[current]
		if parent == "" {
			break
		}
		if _, ok := seen[parent]; ok {
			break
		}
		seen[parent] = struct{}{}
		depth++
		current = parent
	}
	return depth
}

var stageMatchSuffixPattern = regexp.MustCompile(`-\d+$`)

func normalizeStageMatchName(name string) string {
	name = strings.TrimSpace(strings.ToLower(name))
	name = strings.Join(strings.Fields(name), "")
	return stageMatchSuffixPattern.ReplaceAllString(name, "")
}

func isUserJenkinsStage(name string) bool {
	name = strings.TrimSpace(name)
	if name == "" {
		return false
	}
	return !strings.HasPrefix(strings.ToLower(name), "declarative:")
}

func isMatchableJenkinsStageName(name string) bool {
	name = strings.TrimSpace(name)
	if !isUserJenkinsStage(name) {
		return false
	}
	lower := strings.ToLower(name)
	if strings.HasPrefix(lower, "branch:") {
		return false
	}
	if strings.HasSuffix(name, "-分支") {
		return false
	}
	return !generatedParallelGroupNamePattern.MatchString(name)
}

func normalizeJenkinsStageStatus(status string) string {
	switch strings.ToUpper(strings.TrimSpace(status)) {
	case "SUCCESS":
		return "success"
	case "FAILED", "FAILURE":
		return "failed"
	case "IN_PROGRESS":
		return "running"
	case "PAUSED", "PAUSED_PENDING_INPUT":
		return "paused"
	case "NOT_EXECUTED", "SKIPPED":
		return "skipped"
	case "ABORTED":
		return "aborted"
	case "UNSTABLE":
		return "unstable"
	default:
		if strings.TrimSpace(status) == "" {
			return "queued"
		}
		return strings.ToLower(strings.TrimSpace(status))
	}
}
