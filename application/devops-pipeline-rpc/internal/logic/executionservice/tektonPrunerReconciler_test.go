package executionservicelogic

import (
	"strings"
	"testing"
	"time"

	"github.com/yanshicheng/kube-nova/application/devops-pipeline-rpc/internal/model"
	devopstekton "github.com/yanshicheng/kube-nova/common/devops/tekton"
)

func TestSelectTektonTaskRunPrunerCandidatesSkipsDeletedTaskRuns(t *testing.T) {
	limit := 1
	now := time.Now()
	items := []devopstekton.PrunerResourceInfo{
		{
			Name:       "taskrun-new",
			Namespace:  "ci",
			Status:     devopstekton.RunStatus{Status: "success"},
			FinishedAt: now.Add(-1 * time.Hour),
		},
		{
			Name:       "taskrun-mid",
			Namespace:  "ci",
			Status:     devopstekton.RunStatus{Status: "success"},
			FinishedAt: now.Add(-2 * time.Hour),
		},
		{
			Name:       "taskrun-old",
			Namespace:  "ci",
			Status:     devopstekton.RunStatus{Status: "success"},
			FinishedAt: now.Add(-3 * time.Hour),
		},
	}

	candidates := selectTektonTaskRunPrunerCandidates(items, tektonPrunerPolicy{
		HistoryLimit: &limit,
	}, map[string]struct{}{
		prunerResourceKey("ci", "taskrun-mid"): struct{}{},
	})

	if len(candidates) != 1 {
		t.Fatalf("candidates len = %d, want 1", len(candidates))
	}
	if candidates[0].Name != "taskrun-old" {
		t.Fatalf("candidate = %s, want taskrun-old", candidates[0].Name)
	}
}

func TestSelectTektonPrunerResourcesSupportsTaskRuns(t *testing.T) {
	successLimit := 1
	now := time.Now()
	items := []devopstekton.PrunerResourceInfo{
		{
			Name:       "taskrun-keep",
			Namespace:  "ci",
			Status:     devopstekton.RunStatus{Status: "success"},
			FinishedAt: now.Add(-1 * time.Minute),
		},
		{
			Name:       "taskrun-delete",
			Namespace:  "ci",
			Status:     devopstekton.RunStatus{Status: "success"},
			FinishedAt: now.Add(-2 * time.Minute),
		},
	}

	candidates := selectTektonPrunerResources(items, tektonPrunerPolicy{
		SuccessfulHistoryLimit: &successLimit,
	})

	if len(candidates) != 1 {
		t.Fatalf("candidates len = %d, want 1", len(candidates))
	}
	if candidates[0].Name != "taskrun-delete" {
		t.Fatalf("candidate = %s, want taskrun-delete", candidates[0].Name)
	}
}

func TestTektonNativePrunerConfigMapNamespaceMode(t *testing.T) {
	historyLimit := 3
	ttl := 3600
	pipeline := &model.DevopsPipeline{
		TektonNamespace:    "ci",
		TektonPipelineName: "app-pipeline",
	}

	cm, err := tektonNativePrunerConfigMap(pipeline, tektonPrunerPolicy{
		NativePrunerMode:        "namespaceConfigMap",
		ResourceTypes:           []string{"PipelineRun", "TaskRun"},
		Scope:                   "pipeline",
		HistoryLimit:            &historyLimit,
		TTLSecondsAfterFinished: &ttl,
	})
	if err != nil {
		t.Fatalf("tektonNativePrunerConfigMap() error = %v", err)
	}
	if cm.Namespace != "ci" || cm.Name != tektonNativePrunerNamespaceConfigMapName {
		t.Fatalf("ConfigMap target = %s/%s", cm.Namespace, cm.Name)
	}
	if cm.Labels["pruner.tekton.dev/config-type"] != "namespace" {
		t.Fatalf("config-type label = %q", cm.Labels["pruner.tekton.dev/config-type"])
	}
	content := cm.Data["ns-config"]
	for _, want := range []string{
		"pipelineRuns:",
		"taskRuns:",
		"historyLimit: 3",
		"ttlSecondsAfterFinished: 3600",
		"tekton.dev/pipeline: app-pipeline",
		"kube-nova.io/pipeline-id:",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("ns-config missing %q:\n%s", want, content)
		}
	}
}

func TestTektonNativePrunerConfigMapKeepsZeroLimits(t *testing.T) {
	zero := 0
	pipeline := &model.DevopsPipeline{
		TektonNamespace:    "ci",
		TektonPipelineName: "app-pipeline",
	}

	cm, err := tektonNativePrunerConfigMap(pipeline, tektonPrunerPolicy{
		NativePrunerMode:        "namespaceConfigMap",
		ResourceTypes:           []string{"PipelineRun"},
		Scope:                   "pipeline",
		HistoryLimit:            &zero,
		SuccessfulHistoryLimit:  &zero,
		FailedHistoryLimit:      &zero,
		TTLSecondsAfterFinished: &zero,
	})
	if err != nil {
		t.Fatalf("tektonNativePrunerConfigMap() error = %v", err)
	}
	content := cm.Data["ns-config"]
	for _, want := range []string{
		"historyLimit: 0",
		"successfulHistoryLimit: 0",
		"failedHistoryLimit: 0",
		"ttlSecondsAfterFinished: 0",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("ns-config missing %q:\n%s", want, content)
		}
	}
}

func TestTektonNativePrunerConfigMapGlobalMode(t *testing.T) {
	successLimit := 5
	cm, err := tektonNativePrunerConfigMap(&model.DevopsPipeline{}, tektonPrunerPolicy{
		NativePrunerMode:            "globalConfigMap",
		NativePrunerTargetNamespace: "tekton-pipelines",
		NativePrunerConfigLevel:     "global",
		ResourceTypes:               []string{"PipelineRun"},
		SelectorLabels: []tektonNameValue{
			{Name: "kube-nova.io/engine", Value: "tekton"},
		},
		SuccessfulHistoryLimit: &successLimit,
	})
	if err != nil {
		t.Fatalf("tektonNativePrunerConfigMap() error = %v", err)
	}
	if cm.Namespace != "tekton-pipelines" || cm.Name != tektonNativePrunerGlobalConfigMapName {
		t.Fatalf("ConfigMap target = %s/%s", cm.Namespace, cm.Name)
	}
	if cm.Labels["pruner.tekton.dev/config-type"] != "global" {
		t.Fatalf("config-type label = %q", cm.Labels["pruner.tekton.dev/config-type"])
	}
	content := cm.Data["global-config"]
	for _, want := range []string{
		"enforcedConfigLevel: global",
		"pipelineRuns:",
		"successfulHistoryLimit: 5",
		"kube-nova.io/engine: tekton",
		"kube-nova.io/pipeline-id:",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("global-config missing %q:\n%s", want, content)
		}
	}
	if strings.Contains(content, "taskRuns:") {
		t.Fatalf("global-config should not include taskRuns:\n%s", content)
	}
}
