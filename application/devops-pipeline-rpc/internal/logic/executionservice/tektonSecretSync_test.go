package executionservicelogic

import (
	"context"
	"strings"
	"testing"

	"github.com/yanshicheng/kube-nova/application/devops-pipeline-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/common/devops/channelvars"
)

func TestTektonEnabledStepIDsUsesNodeID(t *testing.T) {
	steps := []model.PipelineStep{
		{ID: "node-a", Enabled: true},
		{ID: "node-b", StepID: "template-b", Enabled: true},
		{ID: "node-c", Enabled: false},
		{StepID: "template-d", Enabled: true},
	}

	result := tektonEnabledStepIDs(steps)
	if _, ok := result["node-a"]; !ok {
		t.Fatalf("expected enabled node without stepId to be included")
	}
	if _, ok := result["node-b"]; !ok {
		t.Fatalf("expected enabled node with stepId to be included")
	}
	if _, ok := result["node-c"]; ok {
		t.Fatalf("disabled node should not be included")
	}
	if _, ok := result[""]; ok {
		t.Fatalf("empty node id should not be included")
	}
}

func TestEnsureTektonWorkspaceBindingsAllowsRuntimeSelectionOnSave(t *testing.T) {
	taskYaml := `apiVersion: tekton.dev/v1
kind: Task
metadata:
  name: build
spec:
  workspaces:
    - name: cache
  steps:
    - name: build
      image: alpine:latest
      script: echo ok
`
	pipeline := &model.DevopsPipeline{
		Code: "demo",
		Steps: []model.PipelineStep{
			{ID: "build", StepCode: "build", Enabled: true, StageContent: taskYaml},
		},
		Params: []model.PipelineParam{
			{
				Name:        "缓存 PVC",
				Code:        "CACHE_PVC",
				ParamType:   channelvars.ParamKubernetesPVCName,
				Mode:        "params",
				RuntimeMode: "params",
				StepNodeID:  "build",
				Config: model.StepParamConfig{
					MappingField: "cache",
				},
			},
		},
	}
	if err := ensureTektonPVCWorkspaces(context.Background(), nil, "devops-demo", pipeline, nil); err != nil {
		t.Fatalf("save should allow runtime pvc selection: %v", err)
	}
	err := ensureTektonPVCWorkspaces(context.Background(), nil, "devops-demo", pipeline, map[string]string{})
	if err == nil || !strings.Contains(err.Error(), "Kubernetes PVC 参数不能为空") {
		t.Fatalf("run should require pvc selection, got %v", err)
	}
}

func TestValidateTektonVolumeClaimTemplateAllowsRuntimeSelectionOnSave(t *testing.T) {
	taskYaml := `apiVersion: tekton.dev/v1
kind: Task
metadata:
  name: build
spec:
  workspaces:
    - name: cache
  steps:
    - name: build
      image: alpine:latest
      script: echo ok
`
	param := model.PipelineParam{
		Name:        "缓存容量",
		Code:        "CACHE_STORAGE",
		ParamType:   "string",
		Mode:        "params",
		RuntimeMode: "params",
		StepNodeID:  "build",
		Config: model.StepParamConfig{
			MappingField: "cache",
			Provider:     "volumeClaimTemplate.storage",
		},
	}
	pipeline := &model.DevopsPipeline{
		Code: "demo",
		Steps: []model.PipelineStep{
			{ID: "build", StepCode: "build", Enabled: true, StageContent: taskYaml},
		},
	}
	if err := validateTektonVolumeClaimTemplateParam(pipeline, param, nil, false); err != nil {
		t.Fatalf("save should allow runtime storage selection: %v", err)
	}
	err := validateTektonVolumeClaimTemplateParam(pipeline, param, map[string]string{}, true)
	if err == nil || !strings.Contains(err.Error(), "动态 PVC 存储容量不能为空") {
		t.Fatalf("run should require storage selection, got %v", err)
	}
}
