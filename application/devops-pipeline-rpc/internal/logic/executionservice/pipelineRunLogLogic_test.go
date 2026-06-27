package executionservicelogic

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/yanshicheng/kube-nova/application/devops-pipeline-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/common/devops/jenkins"
)

func TestExtractStageConsoleLog(t *testing.T) {
	fullLog := `[Pipeline] stage
[Pipeline] { (信息展示)
[Pipeline] echo
APP_NAME = demo
[Pipeline] }
[Pipeline] // stage
[Pipeline] stage
[Pipeline] { (发布)
[Pipeline] echo
deploy
`
	content := extractStageConsoleLog(fullLog, "信息展示")
	if !stringsContains(content, "APP_NAME = demo") || stringsContains(content, "deploy") {
		t.Fatalf("stage console log should be sliced by stage name, got:\n%s", content)
	}
}

func TestExtractStageConsoleLogWithParallelInterleaving(t *testing.T) {
	fullLog := `[Pipeline] parallel
[Pipeline] { (Branch: 信息展示)
[Pipeline] { (Branch: 示例步骤)
[Pipeline] stage
[Pipeline] { (信息展示)
[Pipeline] echo
APP_NAME = demo
[Pipeline] }
[Pipeline] // stage
[Pipeline] stage
[Pipeline] { (示例步骤-2)
[Pipeline] echo
hello
[Pipeline] }
[Pipeline] // stage
[Pipeline] }
[Pipeline] // parallel
`
	content := extractStageConsoleLog(fullLog, "信息展示")
	if !stringsContains(content, "APP_NAME = demo") {
		t.Fatalf("parallel stage console log should include selected stage body, got:\n%s", content)
	}
	if stringsContains(content, "示例步骤-2") || stringsContains(content, "hello") {
		t.Fatalf("parallel stage console log should not include sibling stage, got:\n%s", content)
	}
}

func TestExtractExactStageConsoleLogForLastParallelStage(t *testing.T) {
	fullLog := `[Pipeline] parallel
[Pipeline] { (Branch: Sonar扫描前端项目)
[Pipeline] { (Branch: npm编译)
[Pipeline] stage
[Pipeline] { (Sonar扫描前端项目)
[Pipeline] echo
sonar start
[Pipeline] }
[Pipeline] // stage
[Pipeline] stage
[Pipeline] { (npm编译)
[Pipeline] echo
npm build
[Pipeline] }
[Pipeline] // stage
[Pipeline] }
[Pipeline] // parallel
`
	content := extractExactStageConsoleLog(fullLog, "npm编译")
	if !stringsContains(content, "npm build") {
		t.Fatalf("last parallel stage console log should include selected stage body, got:\n%s", content)
	}
	if stringsContains(content, "sonar start") || stringsContains(content, "Sonar扫描前端项目") {
		t.Fatalf("last parallel stage console log should not include sibling stage, got:\n%s", content)
	}
}

func TestExtractExactStageConsoleLogForRunningParallelStageSkipsSiblingBlocks(t *testing.T) {
	fullLog := `[Pipeline] parallel
[Pipeline] { (Branch: Sonar扫描前端项目)
[Pipeline] { (Branch: npm编译)
[Pipeline] stage
[Pipeline] { (Sonar扫描前端项目)
[Pipeline] echo
sonar start
[Pipeline] withEnv
[Pipeline] {
sonar detail
[Pipeline] }
[Pipeline] stage
[Pipeline] { (npm编译)
[Pipeline] echo
npm build
[Pipeline] }
[Pipeline] // stage
[Pipeline] echo
sonar continue
`
	content := extractExactStageConsoleLog(fullLog, "Sonar扫描前端项目")
	if !stringsContains(content, "sonar start") || !stringsContains(content, "sonar continue") {
		t.Fatalf("running parallel stage console log should keep current stage body, got:\n%s", content)
	}
	if stringsContains(content, "npm build") || stringsContains(content, "npm编译") {
		t.Fatalf("running parallel stage console log should exclude sibling stage body, got:\n%s", content)
	}
}

func TestMatchJenkinsStagesByNormalizedName(t *testing.T) {
	items := []*model.DevopsPipelineRunStage{
		{StageName: "测试输出"},
		{StageName: "测试输出-2"},
	}
	stages := []jenkins.StageStatus{
		{ID: "7", Name: "测试输出", Status: "SUCCESS"},
		{ID: "13", Name: "测试输出-2", Status: "FAILED"},
	}
	matches := matchJenkinsStages(items, stages)
	if matches[items[0]].ID != "7" || matches[items[1]].ID != "13" {
		t.Fatalf("stage should match by exact name before normalized fallback, got: %#v", matches)
	}
}

func TestMatchJenkinsStagesFallsBackByOrder(t *testing.T) {
	items := []*model.DevopsPipelineRunStage{
		{StageName: "Git代码克隆"},
		{StageName: "npm编译"},
	}
	stages := []jenkins.StageStatus{
		{ID: "11", Name: "Git代码克隆分支-简版", Status: "SUCCESS"},
		{ID: "22", Name: "npm 编译", Status: "SUCCESS"},
	}
	matches := matchJenkinsStages(items, stages)
	if matches[items[0]].ID != "11" || matches[items[1]].ID != "22" {
		t.Fatalf("stage should fallback by execution order, got: %#v", matches)
	}
}

func TestMatchJenkinsStagesSkipsPostStages(t *testing.T) {
	items := []*model.DevopsPipelineRunStage{
		{StageName: "构建", StageType: "custom"},
		{StageName: "后置通知", StageType: "post"},
	}
	stages := []jenkins.StageStatus{
		{ID: "11", Name: "构建", Status: "SUCCESS"},
		{ID: "22", Name: "后置通知", Status: "SUCCESS"},
	}
	matches := matchJenkinsStages(items, stages)
	if matches[items[0]].ID != "11" {
		t.Fatalf("normal stage should match Jenkins stage, got: %#v", matches)
	}
	if _, ok := matches[items[1]]; ok {
		t.Fatalf("post stage should not match Jenkins user stages")
	}
}

func TestExtractConsoleStageNames(t *testing.T) {
	fullLog := `[Pipeline] stage
[Pipeline] { (Git代码克隆分支-简版)
[Pipeline] echo
[Pipeline] }
[Pipeline] stage
[Pipeline] { (并行组-1)
[Pipeline] echo
[Pipeline] stage
[Pipeline] { (npm编译-分支)
[Pipeline] echo
[Pipeline] stage
[Pipeline] { (Declarative: Post Actions)
[Pipeline] echo
[Pipeline] parallel
[Pipeline] { (Branch: npm编译)
[Pipeline] stage
[Pipeline] { (npm编译)
`
	names := extractConsoleStageNames(fullLog)
	if len(names) != 2 || names[0] != "Git代码克隆分支-简版" || names[1] != "npm编译" {
		t.Fatalf("console stage names should only include user stages, got: %#v", names)
	}
}

func TestMatchJenkinsStagesSkipsParallelWrapperStages(t *testing.T) {
	items := []*model.DevopsPipelineRunStage{
		{StageName: "Sonar扫描前端项目"},
		{StageName: "npm编译"},
	}
	stages := []jenkins.StageStatus{
		{ID: "11", Name: "并行组-1", Status: "IN_PROGRESS"},
		{ID: "12", Name: "Sonar扫描前端项目-分支", Status: "SUCCESS"},
		{ID: "13", Name: "Sonar扫描前端项目", Status: "SUCCESS"},
		{ID: "14", Name: "npm编译-分支", Status: "IN_PROGRESS"},
		{ID: "15", Name: "npm编译", Status: "IN_PROGRESS"},
	}
	matches := matchJenkinsStages(items, stages)
	if matches[items[0]].ID != "13" || matches[items[1]].ID != "15" {
		t.Fatalf("parallel stage should prefer leaf stage nodes, got: %#v", matches)
	}
}

func TestExpandJenkinsStageStatusesForRunRecursesParallelUserStageChildren(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/job/demo/63/execution/node/11/wfapi/describe":
			_, _ = w.Write([]byte(`{"stageFlowNodes":[{"id":"12","name":"Sonar扫描前端项目","status":"SUCCESS"},{"id":"13","name":"npm编译","status":"IN_PROGRESS"}]}`))
		case "/job/demo/63/execution/node/12/wfapi/describe":
			_, _ = w.Write([]byte(`{"stageFlowNodes":[{"id":"121","name":"Sonar扫描前端项目","status":"SUCCESS"}]}`))
		case "/job/demo/63/execution/node/13/wfapi/describe":
			_, _ = w.Write([]byte(`{"stageFlowNodes":[{"id":"131","name":"npm编译","status":"IN_PROGRESS"}]}`))
		default:
			_, _ = w.Write([]byte(`{"stageFlowNodes":[]}`))
		}
	}))
	defer server.Close()

	manager := jenkins.NewManager(jenkins.ClientConfig{Endpoint: server.URL})
	run := &model.DevopsPipelineRun{
		JenkinsJobFullName: "demo",
		JenkinsBuildNumber: 63,
	}
	items := []*model.DevopsPipelineRunStage{
		{StageName: "Sonar扫描前端项目"},
		{StageName: "npm编译"},
	}
	steps := []model.PipelineStep{
		{ID: "git", ParentNodeID: workflowStartNodeID, BranchType: "next", SortOrder: 1},
		{ID: "sonar", ParentNodeID: "git", BranchType: "parallel", SortOrder: 2},
		{ID: "npm", ParentNodeID: "git", BranchType: "parallel", SortOrder: 3},
	}

	expanded := expandJenkinsStageStatusesForRun(
		context.Background(),
		manager,
		run,
		[]jenkins.StageStatus{{ID: "11", Name: "并行组-1", Status: "IN_PROGRESS"}},
		items,
		steps,
	)

	byID := make(map[string]jenkins.StageStatus, len(expanded))
	for _, stage := range expanded {
		byID[stage.ID] = stage
	}
	if byID["121"].Name != "Sonar扫描前端项目" || byID["121"].ParentID != "12" {
		t.Fatalf("parallel sonar leaf stage should be expanded, got: %#v", byID["121"])
	}
	if byID["131"].Name != "npm编译" || byID["131"].ParentID != "13" {
		t.Fatalf("parallel npm leaf stage should be expanded, got: %#v", byID["131"])
	}
}

func TestExpandJenkinsStageStatusesForRunExpandsTopLevelParallelUserStages(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/job/demo/64/execution/node/12/wfapi/describe":
			_, _ = w.Write([]byte(`{"stageFlowNodes":[{"id":"121","name":"Sonar扫描前端项目","status":"SUCCESS","durationMillis":1200,"startTimeMillis":1710000000000}]}`))
		case "/job/demo/64/execution/node/13/wfapi/describe":
			_, _ = w.Write([]byte(`{"stageFlowNodes":[{"id":"131","name":"npm编译","status":"IN_PROGRESS","startTimeMillis":1710000002000}]}`))
		default:
			_, _ = w.Write([]byte(`{"stageFlowNodes":[]}`))
		}
	}))
	defer server.Close()

	manager := jenkins.NewManager(jenkins.ClientConfig{Endpoint: server.URL})
	run := &model.DevopsPipelineRun{
		JenkinsJobFullName: "demo",
		JenkinsBuildNumber: 64,
	}
	items := []*model.DevopsPipelineRunStage{
		{StageName: "Sonar扫描前端项目"},
		{StageName: "npm编译"},
	}
	steps := []model.PipelineStep{
		{ID: "git", ParentNodeID: workflowStartNodeID, BranchType: "next", SortOrder: 1},
		{ID: "sonar", ParentNodeID: "git", BranchType: "parallel", SortOrder: 2},
		{ID: "npm", ParentNodeID: "git", BranchType: "parallel", SortOrder: 3},
	}

	expanded := expandJenkinsStageStatusesForRun(
		context.Background(),
		manager,
		run,
		[]jenkins.StageStatus{
			{ID: "12", Name: "Sonar扫描前端项目", Status: "SUCCESS"},
			{ID: "13", Name: "npm编译", Status: "SUCCESS"},
		},
		items,
		steps,
	)

	byID := make(map[string]jenkins.StageStatus, len(expanded))
	for _, stage := range expanded {
		byID[stage.ID] = stage
	}
	if byID["121"].Name != "Sonar扫描前端项目" || byID["121"].Duration != 1200 {
		t.Fatalf("top-level parallel sonar stage should expand to leaf node, got: %#v", byID["121"])
	}
	if byID["131"].Name != "npm编译" || byID["131"].Start != 1710000002000 {
		t.Fatalf("top-level parallel npm stage should expand to leaf node, got: %#v", byID["131"])
	}
}

func TestExpandJenkinsStageStatusesForRunDoesNotRecurseInternalNodes(t *testing.T) {
	var internalNodeRequests int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/job/demo/65/execution/node/12/wfapi/describe":
			_, _ = w.Write([]byte(`{"stageFlowNodes":[{"id":"121","name":"script","status":"IN_PROGRESS"}]}`))
		case "/job/demo/65/execution/node/121/wfapi/describe":
			atomic.AddInt32(&internalNodeRequests, 1)
			_, _ = w.Write([]byte(`{"stageFlowNodes":[{"id":"1211","name":"sh","status":"IN_PROGRESS"}]}`))
		default:
			_, _ = w.Write([]byte(`{"stageFlowNodes":[]}`))
		}
	}))
	defer server.Close()

	manager := jenkins.NewManager(jenkins.ClientConfig{Endpoint: server.URL})
	run := &model.DevopsPipelineRun{
		JenkinsJobFullName: "demo",
		JenkinsBuildNumber: 65,
	}
	items := []*model.DevopsPipelineRunStage{{StageName: "Sonar扫描前端项目"}}
	steps := []model.PipelineStep{
		{ID: "sonar", ParentNodeID: workflowStartNodeID, BranchType: "parallel", SortOrder: 1},
		{ID: "npm", ParentNodeID: workflowStartNodeID, BranchType: "parallel", SortOrder: 2},
	}

	expanded := expandJenkinsStageStatusesForRun(
		context.Background(),
		manager,
		run,
		[]jenkins.StageStatus{{ID: "12", Name: "Sonar扫描前端项目", Status: "IN_PROGRESS"}},
		items,
		steps,
	)

	if atomic.LoadInt32(&internalNodeRequests) != 0 {
		t.Fatalf("internal Jenkins nodes should not be recursively described")
	}
	if len(expanded) != 2 || expanded[1].Name != "script" {
		t.Fatalf("direct child should still be included, got: %#v", expanded)
	}
}

func TestBuildEffectiveJenkinsStagesKeepsParallelStageRunningWhenChildStillRunning(t *testing.T) {
	stages := []jenkins.StageStatus{
		{ID: "13", Name: "Sonar扫描前端项目", Status: "SUCCESS", Start: 1710000000000, Duration: 1000},
		{ID: "131", ParentID: "13", Name: "script", Status: "IN_PROGRESS", Start: 1710000000500},
	}

	effective := buildEffectiveJenkinsStages(stages)
	stage := effective["13"]
	if normalizeJenkinsStageStatus(stage.Status) != "running" {
		t.Fatalf("parallel stage should stay running while child is running, got %#v", stage)
	}
	if stage.Start != 1710000000000 {
		t.Fatalf("parallel stage should keep earliest start time, got %#v", stage)
	}
}

func TestMatchJenkinsStagesPrefersLeafNodeWhenParallelStagesShareSameName(t *testing.T) {
	items := []*model.DevopsPipelineRunStage{
		{StageName: "Sonar扫描前端项目"},
		{StageName: "npm编译"},
	}
	stages := []jenkins.StageStatus{
		{ID: "11", Name: "并行组-1", Status: "IN_PROGRESS"},
		{ID: "12", Name: "Sonar扫描前端项目", Status: "SUCCESS", ParentID: "11"},
		{ID: "121", Name: "Sonar扫描前端项目", Status: "SUCCESS", ParentID: "12"},
		{ID: "13", Name: "npm编译", Status: "IN_PROGRESS", ParentID: "11"},
		{ID: "131", Name: "npm编译", Status: "IN_PROGRESS", ParentID: "13"},
	}

	matches := matchJenkinsStages(items, stages)
	if matches[items[0]].ID != "121" {
		t.Fatalf("sonar stage should prefer leaf Jenkins node, got: %#v", matches[items[0]])
	}
	if matches[items[1]].ID != "131" {
		t.Fatalf("npm stage should prefer leaf Jenkins node, got: %#v", matches[items[1]])
	}
}

func TestHasUsableLogCacheRejectsEmptyCompleteCache(t *testing.T) {
	if hasUsableLogCache(&pipelineLogCache{Complete: true, Content: ""}) {
		t.Fatalf("empty complete cache should not be treated as usable")
	}
	if !hasUsableLogCache(&pipelineLogCache{Complete: true, Content: "sonar log"}) {
		t.Fatalf("non-empty complete cache should be treated as usable")
	}
}

func TestPipelineLogLooksLikeScaffold(t *testing.T) {
	scaffold := `[Pipeline] { (Sonar扫描前端项目)
[Pipeline] stage
[Pipeline] {
[Pipeline] }
`
	if !pipelineLogLooksLikeScaffold(scaffold) {
		t.Fatalf("pipeline scaffold log should be detected")
	}
	if pipelineLogLooksLikeScaffold("17:13:23 INFO real output") {
		t.Fatalf("real output should not be treated as scaffold")
	}
}

func TestExtractConsoleStageTimes(t *testing.T) {
	fullLog := `[2026-05-11T05:11:20.100Z] [Pipeline] stage
[2026-05-11T05:11:20.100Z] [Pipeline] { (Git代码克隆)
[2026-05-11T05:11:22.900Z] [Pipeline] echo
[2026-05-11T05:11:23.400Z] [Pipeline] }
[2026-05-11T05:11:23.500Z] [Pipeline] stage
[2026-05-11T05:11:23.500Z] [Pipeline] { (npm编译)
[2026-05-11T05:11:29.600Z] [Pipeline] echo
`
	times := extractConsoleStageTimes(fullLog)
	if int64(times["Git代码克隆"].End.Sub(times["Git代码克隆"].Start).Seconds()) != 3 {
		t.Fatalf("git stage duration should be parsed from console timestamps, got: %#v", times["Git代码克隆"])
	}
	if int64(times["npm编译"].End.Sub(times["npm编译"].Start).Seconds()) != 6 {
		t.Fatalf("npm stage duration should be parsed from console timestamps, got: %#v", times["npm编译"])
	}
}

func TestFinalRunStageFallbackStatus(t *testing.T) {
	if finalRunStageFallbackStatus("failed") != "failed" {
		t.Fatalf("failed run should fallback failed stage status")
	}
	if finalRunStageFallbackStatus("success") != "success" {
		t.Fatalf("success run should fallback success stage status")
	}
	if finalRunStageFallbackStatus("running") != "" {
		t.Fatalf("active run should not fallback stage status")
	}
}

func TestMatchActiveConsoleStagesMarksPreviousSuccessAndCurrentRunning(t *testing.T) {
	items := []*model.DevopsPipelineRunStage{
		{StageName: "Git代码克隆", Status: "queued"},
		{StageName: "npm编译", Status: "queued"},
		{StageName: "自定义命令", Status: "queued"},
	}
	stages := []jenkins.StageStatus{
		{Name: "Git代码克隆", Status: "SUCCESS"},
		{Name: "npm编译", Status: "IN_PROGRESS"},
	}
	matches := matchJenkinsStages(items, stages)
	if normalizeJenkinsStageStatus(matches[items[0]].Status) != "success" {
		t.Fatalf("previous console stage should be success, got: %#v", matches[items[0]])
	}
	if normalizeJenkinsStageStatus(matches[items[1]].Status) != "running" {
		t.Fatalf("current console stage should be running, got: %#v", matches[items[1]])
	}
	if _, ok := matches[items[2]]; ok {
		t.Fatalf("future stage should keep queued")
	}
}

func TestBuildActiveConsoleStageStatusesKeepsParallelBranchesRunning(t *testing.T) {
	fullLog := `[Pipeline] stage
[Pipeline] { (Git代码克隆)
[Pipeline] echo
clone
[Pipeline] }
[Pipeline] // stage
[Pipeline] parallel
[Pipeline] { (Branch: Sonar扫描前端项目)
[Pipeline] { (Branch: npm编译)
[Pipeline] stage
[Pipeline] { (Sonar扫描前端项目)
[Pipeline] echo
sonar start
[Pipeline] stage
[Pipeline] { (npm编译)
[Pipeline] echo
npm start
`
	stages := buildActiveConsoleStageStatuses(
		[]string{"Git代码克隆", "Sonar扫描前端项目", "npm编译"},
		fullLog,
		[]model.PipelineStep{
			{ID: "git", ParentNodeID: workflowStartNodeID, BranchType: "next", SortOrder: 1},
			{ID: "sonar", ParentNodeID: "git", BranchType: "parallel", SortOrder: 2},
			{ID: "npm", ParentNodeID: "git", BranchType: "parallel", SortOrder: 3},
		},
	)
	byName := consoleStageStatusByName(stages)
	if normalizeJenkinsStageStatus(byName["Git代码克隆"]) != "success" {
		t.Fatalf("closed serial stage should be success, got %#v", byName["Git代码克隆"])
	}
	if normalizeJenkinsStageStatus(byName["Sonar扫描前端项目"]) != "running" {
		t.Fatalf("open parallel branch should keep running, got %#v", byName["Sonar扫描前端项目"])
	}
	if normalizeJenkinsStageStatus(byName["npm编译"]) != "running" {
		t.Fatalf("second open parallel branch should be running, got %#v", byName["npm编译"])
	}
}

func TestBuildActiveConsoleStageStatusesMarksClosedParallelBranchSuccess(t *testing.T) {
	fullLog := `[Pipeline] parallel
[Pipeline] { (Branch: Sonar扫描前端项目)
[Pipeline] { (Branch: npm编译)
[Pipeline] stage
[Pipeline] { (Sonar扫描前端项目)
[Pipeline] echo
sonar start
[Pipeline] }
[Pipeline] // stage
[Pipeline] stage
[Pipeline] { (npm编译)
[Pipeline] echo
npm start
`
	stages := buildActiveConsoleStageStatuses(
		[]string{"Sonar扫描前端项目", "npm编译"},
		fullLog,
		[]model.PipelineStep{
			{ID: "sonar", ParentNodeID: workflowStartNodeID, BranchType: "parallel", SortOrder: 1},
			{ID: "npm", ParentNodeID: workflowStartNodeID, BranchType: "parallel", SortOrder: 2},
		},
	)
	byName := consoleStageStatusByName(stages)
	if normalizeJenkinsStageStatus(byName["Sonar扫描前端项目"]) != "success" {
		t.Fatalf("closed parallel branch should be success, got %#v", byName["Sonar扫描前端项目"])
	}
	if normalizeJenkinsStageStatus(byName["npm编译"]) != "running" {
		t.Fatalf("open parallel branch should be running, got %#v", byName["npm编译"])
	}
}

func TestGuardActiveRunStageDependenciesBlocksNextAfterRunningParallel(t *testing.T) {
	items := []*model.DevopsPipelineRunStage{
		{NodeID: "git", StageName: "Git代码克隆", Status: "success"},
		{NodeID: "sonar", StageName: "Sonar扫描前端项目", Status: "success"},
		{NodeID: "npm", StageName: "npm编译", Status: "running"},
		{
			NodeID:          "custom",
			StageName:       "自定义命令",
			Status:          "success",
			DurationSeconds: 1,
			JenkinsNodeID:   "bad-node",
		},
	}
	steps := []model.PipelineStep{
		{ID: "git", ParentNodeID: workflowStartNodeID, BranchType: "next", SortOrder: 1},
		{ID: "sonar", ParentNodeID: "git", BranchType: "parallel", SortOrder: 2},
		{ID: "npm", ParentNodeID: "git", BranchType: "parallel", SortOrder: 3},
		{ID: "custom", ParentNodeID: "npm", BranchType: "next", SortOrder: 4},
	}

	guardActiveRunStageDependencies(items, steps)
	if items[3].Status != "queued" {
		t.Fatalf("next stage should wait for all parallel branches, got %s", items[3].Status)
	}
	if items[3].DurationSeconds != 0 || items[3].JenkinsNodeID != "" {
		t.Fatalf("blocked next stage runtime data should be reset, got %#v", items[3])
	}
}

func TestGuardActiveRunStageDependenciesKeepsNextAfterParallelComplete(t *testing.T) {
	items := []*model.DevopsPipelineRunStage{
		{NodeID: "git", StageName: "Git代码克隆", Status: "success"},
		{NodeID: "sonar", StageName: "Sonar扫描前端项目", Status: "success"},
		{NodeID: "npm", StageName: "npm编译", Status: "success"},
		{NodeID: "custom", StageName: "自定义命令", Status: "success", DurationSeconds: 1},
	}
	steps := []model.PipelineStep{
		{ID: "git", ParentNodeID: workflowStartNodeID, BranchType: "next", SortOrder: 1},
		{ID: "sonar", ParentNodeID: "git", BranchType: "parallel", SortOrder: 2},
		{ID: "npm", ParentNodeID: "git", BranchType: "parallel", SortOrder: 3},
		{ID: "custom", ParentNodeID: "npm", BranchType: "next", SortOrder: 4},
	}

	guardActiveRunStageDependencies(items, steps)
	if items[3].Status != "success" {
		t.Fatalf("next stage should keep success after parallel complete, got %s", items[3].Status)
	}
}

func stringsContains(value, part string) bool {
	for idx := 0; idx+len(part) <= len(value); idx++ {
		if value[idx:idx+len(part)] == part {
			return true
		}
	}
	return false
}

func consoleStageStatusByName(stages []jenkins.StageStatus) map[string]string {
	result := make(map[string]string, len(stages))
	for _, stage := range stages {
		result[stage.Name] = stage.Status
	}
	return result
}
