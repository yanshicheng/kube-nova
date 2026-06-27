package executionservicelogic

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/yanshicheng/kube-nova/application/devops-pipeline-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/devops-pipeline-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-pipeline-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/devops/jenkins"
	devopstekton "github.com/yanshicheng/kube-nova/common/devops/tekton"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
	gzredis "github.com/zeromicro/go-zero/core/stores/redis"
)

const (
	pipelineRunLogCacheTTL     = 24 * 60 * 60
	tektonFullLogMaxConcurrent = 4
)

type pipelineLogCache struct {
	Content   string `json:"content"`
	Offset    int64  `json:"offset"`
	Complete  bool   `json:"complete"`
	NodeID    string `json:"nodeId,omitempty"`
	UpdatedAt int64  `json:"updatedAt,omitempty"`
}

type PipelineRunLogLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPipelineRunLogLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PipelineRunLogLogic {
	return &PipelineRunLogLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *PipelineRunLogLogic) PipelineRunLog(in *pb.PipelineRunLogReq) (*pb.PipelineRunLogResp, error) {
	run, err := l.svcCtx.PipelineRunModel.FindOne(l.ctx, in.RunId)
	if err != nil {
		l.Errorf("流水线运行日志失败: %v", err)
		return nil, err
	}
	if err := ensureProjectAccess(l.ctx, l.svcCtx, run.ProjectID, in.CurrentUserId, in.CurrentRoles); err != nil {
		l.Errorf("流水线运行日志失败: %v", err)
		return nil, err
	}
	if run.EngineType == engineTekton {
		return l.tektonRunLog(run, in)
	}
	runtime, err := buildRuntimeCached(l.ctx, l.svcCtx, run.ProjectID, run.SystemID, run.EnvironmentID, run.BuildChannelBindingID, in.CurrentUserId, in.CurrentRoles)
	if err != nil {
		l.Errorf("流水线运行日志失败: %v", err)
		return nil, err
	}
	manager := jenkinsManagerFromRuntime(runtime)
	ready, err := ensureRunBuildNumber(l.ctx, l.svcCtx, run, manager, "")
	if err != nil {
		l.Errorf("流水线运行日志失败: %v", err)
		return nil, err
	}
	if !ready {
		return &pb.PipelineRunLogResp{Content: "构建已提交 Jenkins，等待分配构建号。", Archived: false, Source: "jenkins"}, nil
	}
	if shouldSyncJenkinsRunStatus(run.ID.Hex()) {
		_ = syncRunStatusFromJenkins(l.ctx, l.svcCtx, run, manager, "")
	}

	if strings.TrimSpace(in.StageId) != "" && !in.Full {
		stage, err := l.svcCtx.RunStageModel.FindOne(l.ctx, in.StageId)
		if err != nil {
			l.Errorf("流水线运行日志失败: %v", err)
			return nil, err
		}
		if stage.RunID != in.RunId {
			return nil, model.ErrNotFound
		}
		if strings.TrimSpace(stage.JenkinsNodeID) == "" {
			items, listErr := l.svcCtx.RunStageModel.ListByRun(l.ctx, in.RunId)
			if listErr != nil {
				l.Errorf("%s", listErr)
				return nil, listErr
			}
			if stages, statusErr := manager.StageStatus(l.ctx, run.JenkinsJobFullName, run.JenkinsBuildNumber); statusErr == nil {
				stages = expandJenkinsStageStatusesForRun(l.ctx, manager, run, stages, items, pipelineStepsForRun(l.ctx, l.svcCtx, run))
				if syncErr := syncRunStages(l.ctx, l.svcCtx, items, stages, run.Status, pipelineStepsForRun(l.ctx, l.svcCtx, run)); syncErr != nil {
					l.Errorf("%s", syncErr)
					return nil, syncErr
				}
			}
			stage, err = l.svcCtx.RunStageModel.FindOne(l.ctx, in.StageId)
			if err != nil {
				l.Errorf("流水线运行日志失败: %v", err)
				return nil, err
			}
		}
		if strings.TrimSpace(stage.JenkinsNodeID) == "" {
			if isRunLogActive(run.Status) {
				if content, fullErr := l.fullLogContent(run, manager); fullErr == nil {
					stageContent := extractStageConsoleLog(content, stage.StageName)
					if strings.TrimSpace(stageContent) != "" {
						return &pb.PipelineRunLogResp{Content: stageContent, Archived: false, Source: "jenkins-stage-console"}, nil
					}
				}
			}
			if fullLog, fullErr := manager.ConsoleText(l.ctx, run.JenkinsJobFullName, run.JenkinsBuildNumber); fullErr == nil {
				content := extractStageConsoleLog(fullLog, stage.StageName)
				if strings.TrimSpace(content) != "" {
					return &pb.PipelineRunLogResp{Content: content, Archived: false, Source: "jenkins-stage-console"}, nil
				}
			}
			return &pb.PipelineRunLogResp{Content: "阶段日志暂未就绪，请稍后刷新。", Archived: false, Source: "jenkins-stage"}, nil
		}
		isParallelStage := isParallelRunStageForLog(l.ctx, l.svcCtx, run, stage)
		content, err := l.stageLogContent(run, stage, manager)
		if err != nil {
			l.Errorf("流水线运行日志失败: %v", err)
			return nil, err
		}
		if strings.TrimSpace(content) == "" && isParallelStage && strings.TrimSpace(stage.JenkinsNodeID) == "" {
			content = l.exactStageConsoleLogContent(run, stage, manager)
		}
		if strings.TrimSpace(content) == "" {
			if isRunLogActive(run.Status) && !(isParallelStage && strings.TrimSpace(stage.JenkinsNodeID) != "") {
				if fullLog, fullErr := l.fullLogContent(run, manager); fullErr == nil {
					content = extractStageConsoleLog(fullLog, stage.StageName)
				}
			}
			if strings.TrimSpace(content) == "" {
				if !(isParallelStage && strings.TrimSpace(stage.JenkinsNodeID) != "") {
					if fullLog, fullErr := manager.ConsoleText(l.ctx, run.JenkinsJobFullName, run.JenkinsBuildNumber); fullErr == nil {
						content = extractStageConsoleLog(fullLog, stage.StageName)
					}
				}
			}
		}
		if strings.TrimSpace(content) == "" {
			content = "阶段日志暂未就绪，请稍后刷新。"
		}
		return &pb.PipelineRunLogResp{Content: content, Archived: false, Source: "jenkins-stage"}, nil
	}

	content, err := l.fullLogContent(run, manager)
	if err != nil {
		l.Errorf("流水线运行日志失败: %v", err)
		return nil, err
	}

	return &pb.PipelineRunLogResp{Content: content, Archived: false, Source: "jenkins"}, nil
}

func (l *PipelineRunLogLogic) tektonRunLog(run *model.DevopsPipelineRun, in *pb.PipelineRunLogReq) (*pb.PipelineRunLogResp, error) {
	client, _, err := tektonClientForRun(l.ctx, l.svcCtx, run, in.CurrentUserId, in.CurrentRoles)
	if err != nil {
		l.Errorf("流水线运行日志失败: %v", err)
		return nil, err
	}
	if err := syncTektonRunSnapshot(l.ctx, l.svcCtx, run, in.CurrentUserId, in.CurrentRoles, run.UpdatedBy); err != nil {
		l.Errorf("同步 Tekton 运行状态失败: %v", err)
	}
	if hasTektonLogTarget(in) {
		return l.tektonTargetLog(client, run, in)
	}
	if strings.TrimSpace(in.StageId) != "" && !in.Full {
		stage, err := l.svcCtx.RunStageModel.FindOne(l.ctx, in.StageId)
		if err != nil {
			l.Errorf("流水线运行日志失败: %v", err)
			return nil, err
		}
		if stage.RunID != in.RunId {
			return nil, model.ErrNotFound
		}
		if strings.TrimSpace(stage.TektonPodName) == "" {
			_ = syncTektonRunSnapshot(l.ctx, l.svcCtx, run, in.CurrentUserId, in.CurrentRoles, run.UpdatedBy)
			stage, _ = l.svcCtx.RunStageModel.FindOne(l.ctx, in.StageId)
		}
		if strings.TrimSpace(stage.TektonPodName) == "" {
			return &pb.PipelineRunLogResp{Content: "Tekton 阶段日志暂未就绪，请稍后刷新。", Archived: false, Source: "tekton-stage"}, nil
		}
		content, err := client.PodLogs(l.ctx, run.TektonNamespace, stage.TektonPodName, strings.TrimSpace(in.ContainerName))
		if err != nil {
			l.Errorf("读取 Tekton 阶段日志失败: %v", err)
			return &pb.PipelineRunLogResp{Content: "Tekton 阶段日志暂未就绪，请稍后刷新。", Archived: false, Source: "tekton-stage"}, nil
		}
		return &pb.PipelineRunLogResp{Content: content, Archived: false, Source: "tekton-stage"}, nil
	}
	taskRuns, err := client.ListTaskRuns(l.ctx, run.TektonNamespace, run.TektonPipelineRunName)
	if err != nil {
		l.Errorf("读取 Tekton TaskRun 列表失败: %v", err)
		return &pb.PipelineRunLogResp{Content: "Tekton 日志暂未就绪，请稍后刷新。", Archived: false, Source: "tekton"}, nil
	}
	content := l.collectTektonTaskRunLogs(client, run.TektonNamespace, taskRuns)
	if content == "" {
		content = "Tekton 日志暂未就绪，请稍后刷新。"
	}
	return &pb.PipelineRunLogResp{Content: content, Archived: false, Source: "tekton"}, nil
}

func (l *PipelineRunLogLogic) collectTektonTaskRunLogs(client *devopstekton.Client, namespace string, taskRuns []devopstekton.TaskRunInfo) string {
	type logResult struct {
		title   string
		content string
	}
	results := make([]logResult, len(taskRuns))
	limit := tektonFullLogMaxConcurrent
	if len(taskRuns) < limit {
		limit = len(taskRuns)
	}
	if limit <= 0 {
		return ""
	}
	sem := make(chan struct{}, limit)
	var wg sync.WaitGroup
	for index, taskRun := range taskRuns {
		podName := strings.TrimSpace(taskRun.PodName)
		if podName == "" {
			continue
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(index int, taskRun devopstekton.TaskRunInfo, podName string) {
			defer wg.Done()
			defer func() { <-sem }()
			content, err := client.PodLogs(l.ctx, namespace, podName, "")
			if err != nil || strings.TrimSpace(content) == "" {
				return
			}
			results[index] = logResult{
				title:   firstNotBlank(taskRun.PipelineTaskName, taskRun.Name),
				content: content,
			}
		}(index, taskRun, podName)
	}
	wg.Wait()

	var builder strings.Builder
	for _, item := range results {
		if strings.TrimSpace(item.content) == "" {
			continue
		}
		if builder.Len() > 0 {
			builder.WriteString("\n\n")
		}
		builder.WriteString("===== ")
		builder.WriteString(item.title)
		builder.WriteString(" =====\n")
		builder.WriteString(item.content)
	}
	return strings.TrimSpace(builder.String())
}

func hasTektonLogTarget(in *pb.PipelineRunLogReq) bool {
	if in == nil {
		return false
	}
	return strings.TrimSpace(in.TaskRunName) != "" || strings.TrimSpace(in.PodName) != ""
}

func (l *PipelineRunLogLogic) tektonTargetLog(client *devopstekton.Client, run *model.DevopsPipelineRun, in *pb.PipelineRunLogReq) (*pb.PipelineRunLogResp, error) {
	taskRuns, err := client.ListTaskRuns(l.ctx, run.TektonNamespace, run.TektonPipelineRunName)
	if err != nil {
		l.Errorf("读取 Tekton TaskRun 列表失败: %v", err)
		return nil, err
	}
	taskRunName := strings.TrimSpace(in.TaskRunName)
	podName := strings.TrimSpace(in.PodName)
	containerName := strings.TrimSpace(in.ContainerName)
	for _, taskRun := range taskRuns {
		if taskRunName != "" && taskRun.Name != taskRunName {
			continue
		}
		if podName != "" && taskRun.PodName != podName {
			continue
		}
		targetPod := firstNotBlank(podName, taskRun.PodName)
		if targetPod == "" {
			return &pb.PipelineRunLogResp{Content: "Tekton 日志暂未就绪，请稍后刷新。", Archived: false, Source: "tekton-taskrun"}, nil
		}
		content, err := client.PodLogs(l.ctx, run.TektonNamespace, targetPod, containerName)
		if err != nil {
			l.Errorf("读取 Tekton 指定日志失败: %v", err)
			return &pb.PipelineRunLogResp{Content: "Tekton 日志暂未就绪，请稍后刷新。", Archived: false, Source: "tekton-taskrun"}, nil
		}
		return &pb.PipelineRunLogResp{Content: content, Archived: false, Source: "tekton-taskrun"}, nil
	}
	return nil, errorx.Msg("Tekton 日志目标不属于当前 PipelineRun")
}

func (l *PipelineRunLogLogic) fullLogContent(run *model.DevopsPipelineRun, manager *jenkins.Manager) (string, error) {
	cacheKey := pipelineRunFullLogCacheKey(run.ID.Hex())
	cache, _ := l.loadPipelineLogCache(cacheKey)
	if isRunLogActive(run.Status) {
		return l.collectProgressiveLog(cacheKey, cache, func(start int64) (string, int64, bool, error) {
			return manager.ProgressiveLog(l.ctx, run.JenkinsJobFullName, run.JenkinsBuildNumber, start)
		})
	}
	if hasUsableLogCache(cache) {
		return cache.Content, nil
	}
	content, err := l.collectCompletedFullLog(run, manager, cache)
	if err != nil {
		l.Errorf("读取完整日志内容失败: %v", err)
		return "", err
	}
	return content, nil
}

func (l *PipelineRunLogLogic) stageLogContent(run *model.DevopsPipelineRun, stage *model.DevopsPipelineRunStage, manager *jenkins.Manager) (string, error) {
	cacheKey := pipelineRunStageLogCacheKey(run.ID.Hex(), stage.ID.Hex())
	cache, _ := l.loadPipelineLogCache(cacheKey)
	if cache != nil && cache.NodeID != "" && cache.NodeID != stage.JenkinsNodeID {
		cache = nil
	}
	if isRunLogActive(run.Status) && cache != nil {
		cache.Complete = false
	}
	if isParallelRunStageForLog(l.ctx, l.svcCtx, run, stage) {
		return l.collectParallelStageSnapshotLog(cacheKey, cache, run, stage, manager)
	}
	if isStageLogActive(stage.Status) {
		return l.collectProgressiveLog(cacheKey, cache, func(start int64) (string, int64, bool, error) {
			return manager.NodeProgressiveLog(l.ctx, run.JenkinsJobFullName, run.JenkinsBuildNumber, stage.JenkinsNodeID, start)
		}, stage.JenkinsNodeID)
	}
	if hasUsableLogCache(cache) {
		return cache.Content, nil
	}
	content, err := l.collectCompletedStageLog(run, stage, manager, cache)
	if err != nil {
		l.Errorf("读取阶段日志内容失败: %v", err)
		return "", err
	}
	return content, nil
}

func (l *PipelineRunLogLogic) collectParallelStageSnapshotLog(
	cacheKey string,
	cache *pipelineLogCache,
	run *model.DevopsPipelineRun,
	stage *model.DevopsPipelineRunStage,
	manager *jenkins.Manager,
) (string, error) {
	if cache != nil && pipelineLogLooksLikeScaffold(cache.Content) {
		cache = nil
	}
	content, err := manager.StageLog(l.ctx, run.JenkinsJobFullName, run.JenkinsBuildNumber, stage.JenkinsNodeID)
	if err != nil {
		if cache != nil && strings.TrimSpace(cache.Content) != "" {
			return cache.Content, nil
		}
		l.Errorf("读取并行阶段节点日志失败: %v", err)
		return "", err
	}
	if pipelineLogLooksLikeScaffold(content) {
		content = ""
	}
	if strings.TrimSpace(content) == "" {
		if cache != nil && strings.TrimSpace(cache.Content) != "" {
			return cache.Content, nil
		}
		return "", nil
	}
	if cache == nil {
		cache = &pipelineLogCache{}
	}
	if cache.Content != "" && len(cache.Content) > len(content) {
		content = cache.Content
	}
	cache.Content = content
	cache.Offset = int64(len([]byte(content)))
	cache.Complete = !isRunLogActive(run.Status) && !isStageLogActive(stage.Status)
	cache.NodeID = stage.JenkinsNodeID
	cache.UpdatedAt = time.Now().Unix()
	_ = l.savePipelineLogCache(cacheKey, cache)
	return content, nil
}

func (l *PipelineRunLogLogic) exactStageConsoleLogContent(run *model.DevopsPipelineRun, stage *model.DevopsPipelineRunStage, manager *jenkins.Manager) string {
	if run == nil || stage == nil || manager == nil {
		return ""
	}
	if content, err := l.fullLogContent(run, manager); err == nil {
		if stageContent := extractExactStageConsoleLog(content, stage.StageName); strings.TrimSpace(stageContent) != "" {
			return stageContent
		}
	}
	if fullLog, err := manager.ConsoleText(l.ctx, run.JenkinsJobFullName, run.JenkinsBuildNumber); err == nil {
		return extractExactStageConsoleLog(fullLog, stage.StageName)
	}
	return ""
}

func (l *PipelineRunLogLogic) collectCompletedFullLog(run *model.DevopsPipelineRun, manager *jenkins.Manager, cache *pipelineLogCache) (string, error) {
	cacheKey := pipelineRunFullLogCacheKey(run.ID.Hex())
	if cache == nil {
		cache = &pipelineLogCache{}
	}
	content := cache.Content
	offset := cache.Offset
	for i := 0; i < 20; i++ {
		chunk, next, more, err := manager.ProgressiveLog(l.ctx, run.JenkinsJobFullName, run.JenkinsBuildNumber, offset)
		if err != nil {
			break
		}
		if chunk != "" {
			content += chunk
		}
		if next <= offset {
			offset = next
			break
		}
		offset = next
		if !more {
			cache.Content = content
			cache.Offset = offset
			cache.Complete = true
			cache.UpdatedAt = time.Now().Unix()
			_ = l.savePipelineLogCache(cacheKey, cache)
			return content, nil
		}
	}
	if strings.TrimSpace(content) == "" {
		fullLog, err := manager.ConsoleText(l.ctx, run.JenkinsJobFullName, run.JenkinsBuildNumber)
		if err != nil {
			l.Errorf("收集已完成完整日志失败: %v", err)
			return "", err
		}
		content = fullLog
		offset = int64(len([]byte(fullLog)))
	}
	cache.Content = content
	cache.Offset = offset
	cache.Complete = true
	cache.UpdatedAt = time.Now().Unix()
	_ = l.savePipelineLogCache(cacheKey, cache)
	return content, nil
}

func (l *PipelineRunLogLogic) collectCompletedStageLog(run *model.DevopsPipelineRun, stage *model.DevopsPipelineRunStage, manager *jenkins.Manager, cache *pipelineLogCache) (string, error) {
	cacheKey := pipelineRunStageLogCacheKey(run.ID.Hex(), stage.ID.Hex())
	if cache == nil {
		cache = &pipelineLogCache{NodeID: stage.JenkinsNodeID}
	}
	content := cache.Content
	offset := cache.Offset
	for i := 0; i < 20; i++ {
		chunk, next, more, err := manager.NodeProgressiveLog(l.ctx, run.JenkinsJobFullName, run.JenkinsBuildNumber, stage.JenkinsNodeID, offset)
		if err != nil {
			break
		}
		if chunk != "" {
			content += chunk
		}
		if next <= offset {
			offset = next
			break
		}
		offset = next
		if !more {
			cache.Content = content
			cache.Offset = offset
			cache.Complete = !isRunLogActive(run.Status)
			cache.NodeID = stage.JenkinsNodeID
			cache.UpdatedAt = time.Now().Unix()
			_ = l.savePipelineLogCache(cacheKey, cache)
			return content, nil
		}
	}
	if strings.TrimSpace(content) == "" {
		fullStageLog, err := manager.StageLog(l.ctx, run.JenkinsJobFullName, run.JenkinsBuildNumber, stage.JenkinsNodeID)
		if err != nil {
			l.Errorf("收集已完成阶段日志失败: %v", err)
			return "", err
		}
		content = fullStageLog
		offset = int64(len([]byte(fullStageLog)))
	}
	cache.Content = content
	cache.Offset = offset
	cache.Complete = strings.TrimSpace(content) != "" && !isRunLogActive(run.Status)
	cache.NodeID = stage.JenkinsNodeID
	cache.UpdatedAt = time.Now().Unix()
	_ = l.savePipelineLogCache(cacheKey, cache)
	return content, nil
}

func (l *PipelineRunLogLogic) collectProgressiveLog(
	cacheKey string,
	cache *pipelineLogCache,
	fetch func(start int64) (string, int64, bool, error),
	nodeIDs ...string,
) (string, error) {
	if cache == nil {
		cache = &pipelineLogCache{}
	}
	if len(nodeIDs) > 0 {
		cache.NodeID = nodeIDs[0]
	}
	chunk, next, more, err := fetch(cache.Offset)
	if err != nil {
		if cache.Content != "" {
			return cache.Content, nil
		}
		logx.Errorf("读取流水线运行日志失败: %v", err)
		return "", err
	}
	if chunk != "" {
		cache.Content += chunk
	}
	if next > cache.Offset {
		cache.Offset = next
	}
	cache.Complete = !more
	cache.UpdatedAt = time.Now().Unix()
	_ = l.savePipelineLogCache(cacheKey, cache)
	return cache.Content, nil
}

func hasUsableLogCache(cache *pipelineLogCache) bool {
	return cache != nil && cache.Complete && strings.TrimSpace(cache.Content) != ""
}

func (l *PipelineRunLogLogic) loadPipelineLogCache(key string) (*pipelineLogCache, error) {
	value, err := l.svcCtx.Cache.GetCtx(l.ctx, key)
	if err != nil {
		if err == gzredis.Nil {
			return nil, nil
		}
		l.Errorf("加载流水线日志缓存失败: %v", err)
		return nil, err
	}
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	var data pipelineLogCache
	if err := json.Unmarshal([]byte(value), &data); err != nil {
		l.Errorf("加载流水线日志缓存失败: %v", err)
		return nil, err
	}
	data.Content = jenkins.CleanLogText(data.Content)
	return &data, nil
}

func (l *PipelineRunLogLogic) savePipelineLogCache(key string, data *pipelineLogCache) error {
	if data == nil {
		return nil
	}
	data.Content = jenkins.CleanLogText(data.Content)
	raw, err := json.Marshal(data)
	if err != nil {
		l.Errorf("保存流水线日志缓存失败: %v", err)
		return err
	}
	return l.svcCtx.Cache.SetexCtx(l.ctx, key, string(raw), pipelineRunLogCacheTTL)
}

func pipelineRunFullLogCacheKey(runID string) string {
	return "devops:pipeline:runlog:full:" + strings.TrimSpace(runID)
}

func pipelineRunStageLogCacheKey(runID, stageID string) string {
	return "devops:pipeline:runlog:stage:" + strings.TrimSpace(runID) + ":" + strings.TrimSpace(stageID)
}

func isRunLogActive(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "preparing", "queued", "running", "paused":
		return true
	default:
		return false
	}
}

func isStageLogActive(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "queued", "running", "paused":
		return true
	default:
		return false
	}
}

func extractStageConsoleLog(fullLog, stageName string) string {
	stageName = strings.TrimSpace(stageName)
	if strings.TrimSpace(fullLog) == "" || stageName == "" {
		return ""
	}
	if content := extractExactStageConsoleLog(fullLog, stageName); strings.TrimSpace(content) != "" {
		return content
	}
	lines := strings.Split(strings.ReplaceAll(fullLog, "\r\n", "\n"), "\n")
	extracted := extractNamedBranchConsoleLog(lines, stageName)
	if strings.TrimSpace(extracted) != "" {
		return extracted
	}
	return ""
}

func extractExactStageConsoleLog(fullLog, stageName string) string {
	stageName = strings.TrimSpace(stageName)
	if strings.TrimSpace(fullLog) == "" || stageName == "" {
		return ""
	}
	lines := strings.Split(strings.ReplaceAll(fullLog, "\r\n", "\n"), "\n")
	stageStart := -1
	stageMarker := "{ (" + stageName + ")"
	for index := 0; index < len(lines); index++ {
		if strings.Contains(lines[index], stageMarker) {
			stageStart = index
			break
		}
	}
	if stageStart < 0 {
		return ""
	}
	captured := []string{lines[stageStart]}
	stageDepth := 1
	skipDepth := 0
	for index := stageStart + 1; index < len(lines); index++ {
		line := lines[index]
		if skipDepth > 0 {
			if isPipelineBlockOpenLine(line) {
				skipDepth++
			}
			if isPipelineBlockCloseLine(line) && skipDepth > 0 {
				skipDepth--
			}
			continue
		}
		if otherStageName, ok := extractConsoleStageName(line); ok && otherStageName != stageName && isUserJenkinsStage(otherStageName) {
			skipDepth = 1
			continue
		}
		captured = append(captured, line)
		if isPipelineBlockOpenLine(line) {
			stageDepth++
			continue
		}
		if isPipelineBlockCloseLine(line) && stageDepth > 0 {
			stageDepth--
			if stageDepth == 0 {
				return strings.TrimSpace(strings.Join(captured, "\n")) + "\n"
			}
		}
	}
	return strings.TrimSpace(strings.Join(captured, "\n")) + "\n"
}

var consoleStageMarkerPattern = regexp.MustCompile(`\[Pipeline\]\s+\{\s+\(([^)]+)\)`)

func extractConsoleStageNames(fullLog string) []string {
	lines := strings.Split(strings.ReplaceAll(fullLog, "\r\n", "\n"), "\n")
	result := make([]string, 0)
	seen := map[string]struct{}{}
	for _, line := range lines {
		match := consoleStageMarkerPattern.FindStringSubmatch(line)
		if len(match) < 2 {
			continue
		}
		name := strings.TrimSpace(match[1])
		if !isMatchableJenkinsStageName(name) {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		result = append(result, name)
	}
	return result
}

func extractNamedBranchConsoleLog(lines []string, stageName string) string {
	branchMarker := fmt.Sprintf("(Branch: %s)", stageName)
	start := -1
	depth := 0
	for index, line := range lines {
		if start < 0 {
			if strings.Contains(line, branchMarker) {
				start = index
				depth = 1
			}
			continue
		}
	}
	if start < 0 {
		return ""
	}
	captured := []string{lines[start]}
	skipDepth := 0
	for index := start + 1; index < len(lines); index++ {
		line := lines[index]
		if skipDepth > 0 {
			if isPipelineBlockOpenLine(line) {
				skipDepth++
			}
			if isPipelineBlockCloseLine(line) && skipDepth > 0 {
				skipDepth--
			}
			continue
		}
		if otherStageName, ok := extractConsoleStageName(line); ok && otherStageName != stageName && isUserJenkinsStage(otherStageName) {
			skipDepth = 1
			continue
		}
		captured = append(captured, line)
		if isPipelineBlockOpenLine(line) {
			depth++
			continue
		}
		if isPipelineBlockCloseLine(line) && depth > 0 {
			depth--
			if depth == 0 {
				return strings.TrimSpace(strings.Join(captured, "\n")) + "\n"
			}
		}
	}
	return strings.TrimSpace(strings.Join(captured, "\n")) + "\n"
}

func extractConsoleStageName(line string) (string, bool) {
	match := consoleStageMarkerPattern.FindStringSubmatch(line)
	if len(match) < 2 {
		return "", false
	}
	name := strings.TrimSpace(match[1])
	if name == "" {
		return "", false
	}
	return name, true
}

func isPipelineBlockOpenLine(line string) bool {
	return strings.Contains(line, "[Pipeline] {")
}

func isPipelineBlockCloseLine(line string) bool {
	return strings.Contains(line, "[Pipeline] }")
}

func pipelineLogLooksLikeScaffold(text string) bool {
	lines := strings.Split(strings.ReplaceAll(strings.TrimSpace(text), "\r\n", "\n"), "\n")
	meaningful := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		meaningful = append(meaningful, line)
		if len(meaningful) > 6 {
			return false
		}
	}
	if len(meaningful) == 0 {
		return false
	}
	for _, line := range meaningful {
		if !strings.Contains(line, "[Pipeline]") {
			return false
		}
	}
	return true
}

func isParallelRunStageForLog(ctx context.Context, svcCtx *svc.ServiceContext, run *model.DevopsPipelineRun, stage *model.DevopsPipelineRunStage) bool {
	if run == nil || stage == nil || strings.TrimSpace(stage.NodeID) == "" {
		return false
	}
	steps := pipelineStepsForRun(ctx, svcCtx, run)
	if len(steps) == 0 {
		return false
	}
	normalizePipelineStepBranches(steps)
	for _, step := range steps {
		if step.ID == stage.NodeID && strings.TrimSpace(step.BranchType) == "parallel" {
			return true
		}
	}
	return false
}
