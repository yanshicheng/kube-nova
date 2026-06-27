package executionservicelogic

import (
	"strings"
	"testing"
	"time"

	"github.com/yanshicheng/kube-nova/application/devops-pipeline-rpc/internal/model"
	devopstekton "github.com/yanshicheng/kube-nova/common/devops/tekton"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestApplyTektonSkippedTaskToStage(t *testing.T) {
	startedAt := time.Now().Add(-2 * time.Minute).Truncate(time.Second)
	finishedAt := time.Now().Truncate(time.Second)
	stage := &model.DevopsPipelineRunStage{Status: "queued"}
	run := &model.DevopsPipelineRun{StartedAt: startedAt, FinishedAt: finishedAt}

	applyTektonSkippedTaskToStage(stage, devopstekton.SkippedTaskInfo{
		Name:    "deploy",
		Reason:  "WhenExpressionsSkip",
		Message: "when expression evaluated to false",
	}, run)

	if stage.Status != "skipped" {
		t.Fatalf("Status = %q, want skipped", stage.Status)
	}
	if stage.ErrorMessage != "when expression evaluated to false" {
		t.Fatalf("ErrorMessage = %q", stage.ErrorMessage)
	}
	if !stage.StartedAt.Equal(startedAt) {
		t.Fatalf("StartedAt = %v, want %v", stage.StartedAt, startedAt)
	}
	if !stage.FinishedAt.Equal(finishedAt) {
		t.Fatalf("FinishedAt = %v, want %v", stage.FinishedAt, finishedAt)
	}
	if stage.DurationSeconds <= 0 {
		t.Fatalf("DurationSeconds = %d, want positive", stage.DurationSeconds)
	}
}

func TestTektonPipelineRunNameUsesSystemPipelineCodeAndBuildID(t *testing.T) {
	name, err := tektonPipelineRunName("app1", "test-xx", 1)
	if err != nil {
		t.Fatalf("tektonPipelineRunName error: %v", err)
	}
	if name != "app1-test-xx-1" {
		t.Fatalf("name = %q, want app1-test-xx-1", name)
	}

	longName, err := tektonPipelineRunName(strings.Repeat("app", 20), strings.Repeat("pipe", 20), 12345)
	if err != nil {
		t.Fatalf("tektonPipelineRunName long error: %v", err)
	}
	if len(longName) > 63 {
		t.Fatalf("longName length = %d, want <= 63", len(longName))
	}
	if !strings.HasSuffix(longName, "-12345") {
		t.Fatalf("longName = %q, want suffix -12345", longName)
	}
}

func TestRenderTektonPipelineRunYAMLInjectsBuildIDEnv(t *testing.T) {
	pipeline := &model.DevopsPipeline{ID: bson.NewObjectID(), SystemCode: "app1", Code: "test-xx"}
	run := &model.DevopsPipelineRun{ID: bson.NewObjectID(), BuildID: 12}
	content, err := renderTektonPipelineRunYAML(
		"app1-test-xx-12",
		"test-xx",
		tektonBindingConfig{Namespace: "devops-app1"},
		pipeline,
		nil,
		nil,
		run,
	)
	if err != nil {
		t.Fatalf("renderTektonPipelineRunYAML error: %v", err)
	}
	for _, want := range []string{
		"name: BUILD_ID",
		"value: \"12\"",
		"name: buildId",
		"name: KUBE_NOVA_BUILD_ID",
		"name: KUBE_NOVA_RUN_ID",
		"name: KUBE_NOVA_PIPELINE_RUN_NAME",
		"value: app1-test-xx-12",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("content missing %q:\n%s", want, content)
		}
	}
}

func TestApplyTektonTaskRunsToStageAggregatesMatrixTaskRuns(t *testing.T) {
	startedAt := time.Now().Add(-5 * time.Minute).Truncate(time.Second)
	finishedAt := time.Now().Add(-1 * time.Minute).Truncate(time.Second)
	stage := &model.DevopsPipelineRunStage{Status: "queued"}

	applyTektonTaskRunsToStage(stage, []devopstekton.TaskRunInfo{
		{
			Name:             "build-0",
			PipelineTaskName: "build",
			PodName:          "build-0-pod",
			ContainerName:    "step-build",
			ContainerNames:   []string{"step-build"},
			Status: devopstekton.RunStatus{
				Status:     "success",
				StartedAt:  startedAt,
				FinishedAt: startedAt.Add(2 * time.Minute),
			},
		},
		{
			Name:             "build-1",
			PipelineTaskName: "build",
			PodName:          "build-1-pod",
			ContainerName:    "step-build",
			ContainerNames:   []string{"step-build", "sidecar-cache"},
			Status: devopstekton.RunStatus{
				Status:     "failed",
				Message:    "matrix item failed",
				StartedAt:  startedAt.Add(30 * time.Second),
				FinishedAt: finishedAt,
			},
		},
	})

	if stage.Status != "failed" {
		t.Fatalf("Status = %q, want failed", stage.Status)
	}
	if stage.TektonTaskRunName != "build-1" {
		t.Fatalf("TektonTaskRunName = %q, want failed representative", stage.TektonTaskRunName)
	}
	if stage.TektonPodName != "build-1-pod" {
		t.Fatalf("TektonPodName = %q, want build-1-pod", stage.TektonPodName)
	}
	if len(stage.TektonTaskRuns) != 2 {
		t.Fatalf("TektonTaskRuns len = %d, want 2", len(stage.TektonTaskRuns))
	}
	if !stage.StartedAt.Equal(startedAt) {
		t.Fatalf("StartedAt = %v, want earliest %v", stage.StartedAt, startedAt)
	}
	if !stage.FinishedAt.Equal(finishedAt) {
		t.Fatalf("FinishedAt = %v, want latest %v", stage.FinishedAt, finishedAt)
	}
	if stage.ErrorMessage != "matrix item failed" {
		t.Fatalf("ErrorMessage = %q, want matrix item failed", stage.ErrorMessage)
	}
}

func TestApplyTektonTaskRunsToStageKeepsRunningWhenAnyTaskRunActive(t *testing.T) {
	startedAt := time.Now().Add(-2 * time.Minute).Truncate(time.Second)
	stage := &model.DevopsPipelineRunStage{Status: "queued"}

	applyTektonTaskRunsToStage(stage, []devopstekton.TaskRunInfo{
		{
			Name:             "test-0",
			PipelineTaskName: "test",
			Status: devopstekton.RunStatus{
				Status:     "success",
				StartedAt:  startedAt,
				FinishedAt: startedAt.Add(time.Minute),
			},
		},
		{
			Name:             "test-1",
			PipelineTaskName: "test",
			PodName:          "test-1-pod",
			Status: devopstekton.RunStatus{
				Status:    "running",
				StartedAt: startedAt.Add(30 * time.Second),
			},
		},
	})

	if stage.Status != "running" {
		t.Fatalf("Status = %q, want running", stage.Status)
	}
	if !stage.FinishedAt.IsZero() {
		t.Fatalf("FinishedAt = %v, want zero while running", stage.FinishedAt)
	}
	if stage.TektonTaskRunName != "test-1" {
		t.Fatalf("TektonTaskRunName = %q, want active representative", stage.TektonTaskRunName)
	}
}
