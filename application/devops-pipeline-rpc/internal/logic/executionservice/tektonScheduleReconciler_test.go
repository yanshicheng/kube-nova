package executionservicelogic

import (
	"testing"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/yanshicheng/kube-nova/application/devops-pipeline-rpc/internal/model"
)

func TestLatestTektonScheduleDueTime(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatalf("load location error: %v", err)
	}
	schedule, err := cron.ParseStandard("CRON_TZ=Asia/Shanghai */5 * * * *")
	if err != nil {
		t.Fatalf("parse cron error: %v", err)
	}
	now := time.Date(2026, 5, 24, 10, 12, 0, 0, loc)

	dueAt, ok := latestTektonScheduleDueTime(schedule, now, time.Hour)
	if !ok {
		t.Fatalf("latest due time should exist")
	}
	want := time.Date(2026, 5, 24, 10, 10, 0, 0, loc)
	if !dueAt.Equal(want) {
		t.Fatalf("dueAt = %s, want %s", dueAt, want)
	}
}

func TestShouldTriggerTektonMissedSchedule(t *testing.T) {
	dueAt := time.Date(2026, 5, 24, 10, 10, 0, 0, time.UTC)
	pipeline := &model.DevopsPipeline{UpdateAt: dueAt.Add(-time.Minute)}

	if !shouldTriggerTektonMissedSchedule(pipeline, nil, dueAt) {
		t.Fatalf("missing scheduled run should trigger recovery")
	}
	if !shouldTriggerTektonMissedSchedule(pipeline, &model.DevopsPipelineRun{CreateAt: dueAt.Add(-time.Second)}, dueAt) {
		t.Fatalf("old scheduled run should trigger recovery")
	}
	if shouldTriggerTektonMissedSchedule(pipeline, &model.DevopsPipelineRun{CreateAt: dueAt.Add(time.Second)}, dueAt) {
		t.Fatalf("scheduled run created after due time should not trigger recovery")
	}
	pipeline.UpdateAt = dueAt.Add(time.Second)
	if shouldTriggerTektonMissedSchedule(pipeline, nil, dueAt) {
		t.Fatalf("pipeline updated after due time should not recover old schedule")
	}
}

func TestIsRunnableTektonScheduledPipeline(t *testing.T) {
	if !isRunnableTektonScheduledPipeline(&model.DevopsPipeline{
		EngineType:      engineTekton,
		Status:          1,
		SyncStatus:      "synced",
		TektonDagConfig: `{"scheduleConfig":{"enabled":true,"cron":"*/5 * * * *","timezone":"Asia/Shanghai"}}`,
	}) {
		t.Fatalf("enabled synced Tekton pipeline should be runnable")
	}
	for name, pipeline := range map[string]*model.DevopsPipeline{
		"disabled": {EngineType: engineTekton, Status: 0, SyncStatus: "synced"},
		"pending":  {EngineType: engineTekton, Status: 1, SyncStatus: "pending"},
		"jenkins":  {EngineType: engineJenkins, Status: 1, SyncStatus: "synced"},
	} {
		if isRunnableTektonScheduledPipeline(pipeline) {
			t.Fatalf("%s pipeline should not be runnable", name)
		}
	}
}
