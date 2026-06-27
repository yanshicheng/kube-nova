package executionservicelogic

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/yanshicheng/kube-nova/application/devops-pipeline-rpc/internal/model"
	devopstekton "github.com/yanshicheng/kube-nova/common/devops/tekton"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestRenderTektonDagPipelineYAML(t *testing.T) {
	pipeline := &model.DevopsPipeline{
		ID:   bson.NewObjectID(),
		Code: "build-deploy",
		TektonDagConfig: `{
			"pipeline": {
				"name": "build-deploy",
				"displayName": "Build Deploy",
				"description": "build and deploy demo",
				"namespace": "devops-demo",
				"workspaces": [{"name": "source"}]
			},
			"params": [{"name": "branch", "type": "string", "default": "main", "enum": ["main", "release"]}, {"name": "git", "type": "object", "default": "{\"url\":\"repo\"}", "properties": {"url": {"type": "string"}}}],
			"results": [
				{"name": "image-digest", "type": "string", "value": "$(tasks.build.results.IMAGE_DIGEST)"},
				{"name": "image-metadata", "type": "object", "properties": {"digest": {"type": "string"}}, "value": {"digest": "$(tasks.build.results.IMAGE_DIGEST)"}}
			],
			"nodes": [
				{
					"id": "git-clone",
					"name": "git-clone",
					"taskRef": {"name": "git-clone", "resolver": "cluster", "namespace": "tekton-tasks"},
					"workspaces": {"output": "source"}
				},
				{
					"id": "build",
					"name": "build",
					"displayName": "Build Image",
					"description": "build container image",
					"taskRef": {"kind": "ClusterTask", "name": "buildah"},
					"params": {"BRANCH": {"source": "pipelineParam", "paramName": "branch"}},
					"matrix": {"params": [{"name": "platform", "value": ["linux", "darwin"]}]},
					"retries": 1,
					"timeout": "20m"
				},
				{
					"id": "deploy",
					"name": "deploy",
					"taskRef": {"name": "kubectl-apply"},
					"when": [{"input": "$(params.branch)", "operator": "in", "values": ["main"]}]
				}
			],
			"edges": [
				{"source": "git-clone", "target": "build", "type": "runAfter"},
				{"source": "build", "target": "deploy", "type": "result", "resultName": "IMAGE_DIGEST", "targetParam": "imageDigest"},
				{"source": "git-clone", "target": "deploy", "type": "when", "whenInput": "$(params.branch)", "whenOperator": "in", "whenValues": ["main", "release"]}
			],
			"finallyNodes": [
				{"id": "notify", "name": "notify", "taskRef": {"name": "send-message"}}
			],
			"runPolicy": {
				"cleanBeforeRun": true,
				"cleanAfterRun": true
			}
		}`,
	}

	content, err := renderTektonDagPipelineYAML("build-deploy", tektonBindingConfig{Namespace: "devops-demo"}, pipeline, nil)
	if err != nil {
		t.Fatalf("renderTektonDagPipelineYAML error: %v", err)
	}
	assertContains(t, content, "kind: Pipeline")
	assertContains(t, content, "displayName: Build Deploy")
	assertContains(t, content, "description: build and deploy demo")
	assertContains(t, content, "enum:")
	assertContains(t, content, "properties:")
	assertContains(t, content, "displayName: Build Image")
	assertContains(t, content, "description: build container image")
	assertContains(t, content, "runAfter:")
	assertContains(t, content, "- git-clone")
	assertContains(t, content, "finally:")
	assertContains(t, content, "results:")
	assertContains(t, content, "image-digest")
	assertContains(t, content, "image-metadata")
	assertContains(t, content, "digest:")
	assertContains(t, content, "matrix:")
	assertContains(t, content, "value:")
	assertContains(t, content, "$(tasks.build.results.IMAGE_DIGEST)")
	assertContains(t, content, "retries: 1")
	assertContains(t, content, "timeout: 20m")
	assertContains(t, content, "kube-nova-clean-before-run")
	assertContains(t, content, "kube-nova-clean-after-run")
	assertContains(t, content, "taskSpec:")
	assertContains(t, content, "$(workspaces.source.path)")
	assertContains(t, content, "release")
}

func TestRenderTektonDagPipelineYAMLAcceptsNameValueMetadata(t *testing.T) {
	pipeline := &model.DevopsPipeline{
		ID:   bson.NewObjectID(),
		Code: "metadata-demo",
		TektonDagConfig: `{
			"pipeline": {
				"name": "metadata-demo",
				"labels": [{"name": "app", "value": "demo"}],
				"annotations": [{"name": "owner", "value": "devops"}]
			},
			"labels": {"team": "platform"},
			"annotations": {"audit": "enabled"},
			"nodes": [{"id": "build", "name": "build", "taskRef": {"name": "buildah"}}]
		}`,
	}

	content, err := renderTektonDagPipelineYAML("metadata-demo", tektonBindingConfig{Namespace: "devops-demo"}, pipeline, nil)
	if err != nil {
		t.Fatalf("renderTektonDagPipelineYAML should accept metadata name-value lists: %v", err)
	}
	assertContains(t, content, "app: demo")
	assertContains(t, content, "team: platform")
	assertContains(t, content, "owner: devops")
	assertContains(t, content, "audit: enabled")
}

func TestRenderTektonDagPipelineYAMLPreservesWorkspaceSubPath(t *testing.T) {
	pipeline := &model.DevopsPipeline{
		ID:   bson.NewObjectID(),
		Code: "workspace-demo",
		TektonDagConfig: `{
			"pipeline": {"name": "workspace-demo"},
			"params": [{"name": "module", "type": "string", "default": "api"}],
			"workspaces": [{"name": "source"}],
			"nodes": [{
				"id": "build",
				"name": "build",
				"taskRef": {"name": "buildah"},
				"workspaces": [{"name": "source", "workspace": "source", "subPath": "$(params.module)"}]
			}]
		}`,
	}

	content, err := renderTektonDagPipelineYAML("workspace-demo", tektonBindingConfig{Namespace: "devops-demo"}, pipeline, nil)
	if err != nil {
		t.Fatalf("renderTektonDagPipelineYAML should preserve workspace subPath: %v", err)
	}
	assertContains(t, content, "subPath: $(params.module)")
}

func TestRenderTektonDagPipelineYAMLUsesTaskParamRequirementType(t *testing.T) {
	pipeline := &model.DevopsPipeline{
		ID:   bson.NewObjectID(),
		Code: "typed-task-param-demo",
		TektonDagConfig: `{
			"pipeline": {"name": "typed-task-param-demo"},
			"nodes": [{
				"id": "build",
				"name": "build",
				"taskRef": {"name": "buildah"},
				"paramRequirements": [{"name": "platforms", "type": "array", "required": true}],
				"params": [{"name": "platforms", "source": "fixed", "value": "linux,arm64"}]
			}]
		}`,
	}

	content, err := renderTektonDagPipelineYAML("typed-task-param-demo", tektonBindingConfig{Namespace: "devops-demo"}, pipeline, nil)
	if err != nil {
		t.Fatalf("renderTektonDagPipelineYAML should use task param requirement type: %v", err)
	}
	assertContains(t, content, "name: platforms")
	assertContains(t, content, "- linux")
	assertContains(t, content, "- arm64")

	pipeline.TektonDagConfig = `{
		"pipeline": {"name": "typed-task-param-demo"},
		"nodes": [{
			"id": "build",
			"name": "build",
			"taskRef": {"name": "buildah"},
			"paramRequirements": [{"name": "platforms", "type": "array", "required": true}],
			"params": [{"name": "platforms", "source": "fixed", "value": "[1, 2]"}]
		}]
	}`
	if _, err := renderTektonDagPipelineYAML("typed-task-param-demo", tektonBindingConfig{Namespace: "devops-demo"}, pipeline, nil); err == nil ||
		!strings.Contains(err.Error(), "string array") {
		t.Fatalf("non-string array param value should be rejected, got %v", err)
	}
}

func TestValidateTektonDagConfigRejectsWorkspaceSubPathMissingParam(t *testing.T) {
	err := validateTektonDagConfig(`{
		"workspaces": [{"name": "source"}],
		"nodes": [{
			"id": "build",
			"name": "build",
			"taskRef": {"name": "buildah"},
			"workspaces": [{"name": "source", "workspace": "source", "subPath": "$(params.missing)"}]
		}]
	}`)
	if err == nil || !strings.Contains(err.Error(), "subPath") {
		t.Fatalf("missing subPath param should be rejected, got %v", err)
	}
}

func TestRenderTektonDagPipelineYAMLPreservesWhenCEL(t *testing.T) {
	pipeline := &model.DevopsPipeline{
		ID:   bson.NewObjectID(),
		Code: "when-cel-demo",
		TektonDagConfig: `{
			"pipeline": {"name": "when-cel-demo"},
			"params": [{"name": "branch", "type": "string", "default": "main"}],
			"nodes": [{
				"id": "deploy",
				"name": "deploy",
				"taskRef": {"name": "kubectl"},
				"when": [{"cel": "'$(params.branch)' == 'main'"}]
			}]
		}`,
	}

	content, err := renderTektonDagPipelineYAML("when-cel-demo", tektonBindingConfig{Namespace: "devops-demo"}, pipeline, nil)
	if err != nil {
		t.Fatalf("renderTektonDagPipelineYAML should preserve when cel: %v", err)
	}
	assertContains(t, content, "cel:")
	assertContains(t, content, "$(params.branch)")
}

func TestValidateTektonDagConfigRejectsWhenCELMissingParam(t *testing.T) {
	err := validateTektonDagConfig(`{
		"nodes": [{
			"id": "deploy",
			"name": "deploy",
			"taskRef": {"name": "kubectl"},
			"when": [{"cel": "'$(params.missing)' == 'main'"}]
		}]
	}`)
	if err == nil || !strings.Contains(err.Error(), "CEL") {
		t.Fatalf("missing when cel param should be rejected, got %v", err)
	}
}

func TestValidateTektonDagConfigRejectsWhenInputMissingParam(t *testing.T) {
	err := validateTektonDagConfig(`{
		"nodes": [{
			"id": "deploy",
			"name": "deploy",
			"taskRef": {"name": "kubectl"},
			"when": [{"input": "$(params.missing)", "operator": "in", "values": ["main"]}]
		}]
	}`)
	if err == nil || !strings.Contains(err.Error(), "未声明") {
		t.Fatalf("missing when input param should be rejected, got %v", err)
	}
}

func TestValidateTektonDagConfigRejectsMissingPipelineResultRef(t *testing.T) {
	content := `{
		"results": [{"name": "commit", "value": "$(tasks.clone.results.missing)"}],
		"nodes": [{"id": "clone", "name": "clone", "taskRef": {"name": "git-clone"}, "results": ["commit"]}]
	}`

	err := validateTektonDagConfig(content)
	if err == nil || !strings.Contains(err.Error(), "Task Result") {
		t.Fatalf("missing Pipeline result ref should be rejected, got %v", err)
	}
}

func TestValidateTektonDagConfigAllowsFinallyDesignEdge(t *testing.T) {
	content := `{
		"nodes": [
			{"id": "build", "name": "build", "taskRef": {"name": "buildah"}},
			{"id": "notify", "type": "finally", "name": "notify", "taskRef": {"name": "send-message"}}
		],
		"edges": [
			{"source": "build", "target": "notify", "type": "finally"}
		]
	}`

	if err := validateTektonDagConfig(content); err != nil {
		t.Fatalf("finally design edge should be accepted: %v", err)
	}
}

func TestTektonDagPipelineRunWorkspaces(t *testing.T) {
	pipeline := &model.DevopsPipeline{
		TektonDagConfig: `{
			"nodes": [
				{"id": "build", "name": "build", "taskRef": {"name": "buildah"}, "workspaces": {"source": "source"}}
			],
			"workspaces": [
				{"name": "source", "strategy": "volumeClaimTemplate", "storage": "5Gi", "storageClassName": "standard"},
				{"name": "docker-config", "strategy": "secret", "secretName": "docker-auth"},
				{"name": "cache", "strategy": "persistentVolumeClaim", "claimName": "build-cache"}
			]
		}`,
	}

	workspaces, err := tektonDagPipelineRunWorkspaces(pipeline, nil)
	if err != nil {
		t.Fatalf("tektonDagPipelineRunWorkspaces error: %v", err)
	}
	if len(workspaces) != 3 {
		t.Fatalf("workspace count = %d, want 3", len(workspaces))
	}
	if _, ok := workspaces[0]["volumeClaimTemplate"]; !ok {
		t.Fatalf("source workspace should use volumeClaimTemplate: %#v", workspaces[0])
	}
	if _, ok := workspaces[1]["secret"]; !ok {
		t.Fatalf("docker-config workspace should use secret: %#v", workspaces[1])
	}
	if _, ok := workspaces[2]["persistentVolumeClaim"]; !ok {
		t.Fatalf("cache workspace should use persistentVolumeClaim: %#v", workspaces[2])
	}
}

func TestTektonDagPipelineRunWorkspacesProjected(t *testing.T) {
	pipeline := &model.DevopsPipeline{
		TektonDagConfig: `{
			"nodes": [
				{"id": "build", "name": "build", "taskRef": {"name": "buildah"}, "workspaces": {"config": "config"}}
			],
			"workspaces": [
				{"name": "config", "bindingType": "projected", "projected": {"sources": [{"secret": {"name": "docker-auth"}}]}}
			]
		}`,
	}

	workspaces, err := tektonDagPipelineRunWorkspaces(pipeline, nil)
	if err != nil {
		t.Fatalf("tektonDagPipelineRunWorkspaces projected error: %v", err)
	}
	if len(workspaces) != 1 {
		t.Fatalf("workspace count = %d, want 1", len(workspaces))
	}
	if _, ok := workspaces[0]["projected"]; !ok {
		t.Fatalf("workspace should use projected binding: %#v", workspaces[0])
	}
}

func TestValidateTektonDagConfigRejectsProjectedWorkspaceWithoutSpec(t *testing.T) {
	err := validateTektonDagConfig(`{
		"nodes": [
			{"id": "build", "name": "build", "taskRef": {"name": "buildah"}, "workspaces": {"config": "config"}}
		],
		"workspaces": [
			{"name": "config", "bindingType": "projected"}
		]
	}`)
	if err == nil || !strings.Contains(err.Error(), "Projected") {
		t.Fatalf("projected workspace without spec should be rejected, got %v", err)
	}
}

func TestTektonRunPolicyOverridesDagPipelineRunSpec(t *testing.T) {
	pipeline := &model.DevopsPipeline{
		TektonDagConfig: `{
			"pipeline": {"serviceAccountName": "dag-sa"},
			"runPolicy": {
				"managedBy": "dag-controller",
				"pipelineTimeout": "1h",
				"tasksTimeout": "30m",
				"cleanBeforeRun": true
			}
		}`,
		TektonRunPolicy: `{
			"serviceAccountName": "policy-sa",
			"managedBy": "policy-controller",
			"pipelineTimeout": "2h",
			"finallyTimeout": "10m"
		}`,
	}
	spec := map[string]any{}
	policy, err := tektonRunPolicyFromPipeline(pipeline)
	if err != nil {
		t.Fatalf("tektonRunPolicyFromPipeline error: %v", err)
	}

	if err := applyTektonDagPipelineRunSpec(spec, pipeline, tektonBindingConfig{ServiceAccountName: "binding-sa"}, policy); err != nil {
		t.Fatalf("applyTektonDagPipelineRunSpec error: %v", err)
	}

	if spec["serviceAccountName"] != nil {
		t.Fatalf("serviceAccountName must not be rendered at PipelineRun spec top level: %#v", spec["serviceAccountName"])
	}
	taskRunTemplate, ok := spec["taskRunTemplate"].(map[string]any)
	if !ok {
		t.Fatalf("taskRunTemplate should exist: %#v", spec["taskRunTemplate"])
	}
	if taskRunTemplate["serviceAccountName"] != "policy-sa" {
		t.Fatalf("taskRunTemplate.serviceAccountName = %#v, want policy-sa", taskRunTemplate["serviceAccountName"])
	}
	if spec["managedBy"] != "policy-controller" {
		t.Fatalf("managedBy = %#v, want policy-controller", spec["managedBy"])
	}
	timeouts, ok := spec["timeouts"].(map[string]any)
	if !ok {
		t.Fatalf("timeouts should exist: %#v", spec["timeouts"])
	}
	if timeouts["pipeline"] != "2h" || timeouts["tasks"] != "30m" || timeouts["finally"] != "10m" {
		t.Fatalf("unexpected timeouts: %#v", timeouts)
	}
	if !policy.CleanBeforeRun {
		t.Fatalf("dag cleanBeforeRun should be preserved when override omits it")
	}
}

func TestTektonWorkspaceStrategyAppliesToDefaultWorkspace(t *testing.T) {
	pipeline := &model.DevopsPipeline{
		TektonDagConfig: `{
			"nodes": [
				{"id": "build", "name": "build", "taskRef": {"name": "buildah"}, "workspaces": {"source": "source"}}
			],
			"workspaces": [
				{"name": "source", "bindingType": "emptyDir", "storage": "5Gi"}
			],
			"runPolicy": {"workspaceStrategy": "volumeClaimTemplate"}
		}`,
	}

	workspaces, err := tektonDagPipelineRunWorkspaces(pipeline, nil)
	if err != nil {
		t.Fatalf("tektonDagPipelineRunWorkspaces error: %v", err)
	}
	if len(workspaces) != 1 {
		t.Fatalf("workspace count = %d, want 1", len(workspaces))
	}
	if _, ok := workspaces[0]["volumeClaimTemplate"]; !ok {
		t.Fatalf("workspaceStrategy should render dynamic PVC: %#v", workspaces[0])
	}
}

func TestRenderTektonDagPipelineYAMLParsesArrayAndObjectDefaults(t *testing.T) {
	pipeline := &model.DevopsPipeline{
		ID:   bson.NewObjectID(),
		Code: "typed-params",
		TektonDagConfig: `{
			"params": [
				{"name": "platforms", "type": "array", "defaultValue": "[linux, arm64]"},
				{"name": "deployConfig", "type": "object", "defaultValue": "{\"replicas\": 2}", "properties": {"replicas": {"type": "string"}}}
			],
			"nodes": [
				{"id": "build", "name": "build", "taskRef": {"name": "buildah"}}
			]
		}`,
	}

	content, err := renderTektonDagPipelineYAML("typed-params", tektonBindingConfig{Namespace: "devops-demo"}, pipeline, nil)
	if err != nil {
		t.Fatalf("render typed params: %v", err)
	}
	assertContains(t, content, "type: array")
	assertContains(t, content, "- linux")
	assertContains(t, content, "type: object")
	assertContains(t, content, "replicas: 2")
}

func TestValidateTektonDagPipelineParamBindingsRejectsMissingParam(t *testing.T) {
	err := validateTektonDagPipelineParamBindings(`{
		"params": [{"name": "branch", "type": "string"}],
		"nodes": [{"id": "build", "name": "build", "taskRef": {"name": "buildah"}}]
	}`, []model.PipelineParam{})
	if err == nil || !strings.Contains(err.Error(), "参数未配置") {
		t.Fatalf("missing project param should be rejected, got %v", err)
	}
}

func TestRenderTektonPipelineRunYAMLUsesDeclaredDagParams(t *testing.T) {
	pipeline := &model.DevopsPipeline{
		ID:   bson.NewObjectID(),
		Code: "declared-params",
		TektonDagConfig: `{
			"params": [{"name": "branch", "type": "string"}],
			"nodes": [{"id": "build", "name": "build", "taskRef": {"name": "buildah"}}]
		}`,
		Params: []model.PipelineParam{
			{Code: "branch", ParamType: "string", CurrentValue: "main"},
			{Code: "extra", ParamType: "string", CurrentValue: "ignored"},
		},
	}

	content, err := renderTektonPipelineRunYAML("declared-params-1", "declared-params", tektonBindingConfig{Namespace: "devops-demo"}, pipeline, map[string]string{"branch": "release"}, nil, &model.DevopsPipelineRun{BuildID: 1})
	if err != nil {
		t.Fatalf("render pipelineRun: %v", err)
	}
	assertContains(t, content, "name: branch")
	assertContains(t, content, "value: release")
	if strings.Contains(content, "extra") {
		t.Fatalf("PipelineRun should not include params not declared in DAG:\n%s", content)
	}
}

func TestRenderTektonPipelineRunYAMLParsesTypedParamValues(t *testing.T) {
	pipeline := &model.DevopsPipeline{
		ID:   bson.NewObjectID(),
		Code: "typed-run-params",
		TektonDagConfig: `{
			"params": [
				{"name": "platforms", "type": "array"},
				{"name": "deployConfig", "type": "object", "properties": {"replicas": {"type": "string"}}}
			],
			"nodes": [{"id": "build", "name": "build", "taskRef": {"name": "buildah"}}]
		}`,
		Params: []model.PipelineParam{
			{Code: "platforms", ParamType: "list", CurrentValue: "linux,arm64"},
			{Code: "deployConfig", ParamType: "objectList", CurrentValue: `{"replicas": 2}`},
		},
	}

	content, err := renderTektonPipelineRunYAML("typed-run-params-1", "typed-run-params", tektonBindingConfig{Namespace: "devops-demo"}, pipeline, nil, nil, &model.DevopsPipelineRun{BuildID: 1})
	if err != nil {
		t.Fatalf("render typed pipelineRun params: %v", err)
	}
	assertContains(t, content, "name: platforms")
	assertContains(t, content, "- linux")
	assertContains(t, content, "- arm64")
	assertContains(t, content, "name: deployConfig")
	assertContains(t, content, "replicas: 2")
}

func TestRenderTektonDagPipelineYAMLParsesTypedTaskParamValues(t *testing.T) {
	pipeline := &model.DevopsPipeline{
		ID:   bson.NewObjectID(),
		Code: "typed-task-params",
		TektonDagConfig: `{
			"nodes": [{
				"id": "build",
				"name": "build",
				"taskRef": {"name": "buildah"},
				"paramRequirements": [
					{"name": "platforms", "type": "array"},
					{"name": "deployConfig", "type": "object", "properties": {"replicas": {"type": "string"}}}
				],
				"params": {
					"platforms": {"value": "linux,arm64"},
					"deployConfig": {"value": "{\"replicas\": 2}"}
				}
			}]
		}`,
	}

	content, err := renderTektonDagPipelineYAML("typed-task-params", tektonBindingConfig{Namespace: "devops-demo"}, pipeline, nil)
	if err != nil {
		t.Fatalf("render typed task params: %v", err)
	}
	assertContains(t, content, "name: platforms")
	assertContains(t, content, "- linux")
	assertContains(t, content, "- arm64")
	assertContains(t, content, "name: deployConfig")
	assertContains(t, content, "replicas: 2")
}

func TestRenderTektonPipelineRunYAMLPreservesRunPolicyTemplates(t *testing.T) {
	pipeline := &model.DevopsPipeline{
		ID:   bson.NewObjectID(),
		Code: "run-template",
		TektonDagConfig: `{
			"nodes": [{"id": "build", "name": "build", "taskRef": {"name": "buildah"}}],
			"runPolicy": {
				"podTemplateRaw": "nodeSelector:\n  kubernetes.io/os: linux",
				"taskRunTemplateRaw": "serviceAccountName: build-sa\npodTemplate:\n  securityContext: {}",
				"taskRunSpecsRaw": "- pipelineTaskName: build\n  taskServiceAccountName: build-task-sa",
				"podTemplateEnv": [{"name": "HTTP_PROXY", "value": "http://proxy"}]
			}
		}`,
	}
	policy, err := tektonRunPolicyFromPipeline(pipeline)
	if err != nil {
		t.Fatalf("parse run policy: %v", err)
	}
	if len(policy.PodTemplateEnv) != 1 {
		t.Fatalf("podTemplateEnv count = %d, want 1", len(policy.PodTemplateEnv))
	}

	content, err := renderTektonPipelineRunYAML("run-template-1", "run-template", tektonBindingConfig{Namespace: "devops-demo"}, pipeline, nil, nil, &model.DevopsPipelineRun{BuildID: 1})
	if err != nil {
		t.Fatalf("render pipelineRun templates: %v", err)
	}
	assertContains(t, content, "podTemplate:")
	assertContains(t, content, "nodeSelector:")
	assertContains(t, content, "HTTP_PROXY")
	assertContains(t, content, "taskRunTemplate:")
	assertContains(t, content, "serviceAccountName: build-sa")
	assertContains(t, content, "taskRunSpecs:")
	assertContains(t, content, "pipelineTaskName: build")
	assertContains(t, content, "serviceAccountName: build-task-sa")
	assertNotContains(t, content, "taskServiceAccountName")
}

func TestRenderTektonPipelineRunYAMLRejectsUnknownTaskRunSpecTask(t *testing.T) {
	pipeline := &model.DevopsPipeline{
		ID:   bson.NewObjectID(),
		Code: "bad-task-run-spec",
		TektonDagConfig: `{
			"nodes": [{"id": "build", "name": "build", "taskRef": {"name": "buildah"}}],
			"runPolicy": {
				"taskRunSpecsRaw": "- pipelineTaskName: missing\n  taskServiceAccountName: build-task-sa"
			}
		}`,
	}

	_, err := renderTektonPipelineRunYAML("bad-task-run-spec-1", "bad-task-run-spec", tektonBindingConfig{Namespace: "devops-demo"}, pipeline, nil, nil, &model.DevopsPipelineRun{BuildID: 1})
	if err == nil || !strings.Contains(err.Error(), "PipelineTask 不存在") {
		t.Fatalf("unknown taskRunSpecs pipelineTaskName should be rejected, got %v", err)
	}
}

func TestValidateTektonPipelineRunCompletenessRejectsNonRuntimeWorkspaceOverride(t *testing.T) {
	pipeline := &model.DevopsPipeline{
		ID:   bson.NewObjectID(),
		Code: "workspace-override",
		TektonDagConfig: `{
			"nodes": [{"id": "build", "name": "build", "taskRef": {"name": "buildah"}, "workspaces": {"source": "source"}}],
			"workspaces": [{"name": "source", "bindingType": "persistentVolumeClaim", "claimName": "cache", "runtimeConfig": false}]
		}`,
	}

	err := validateTektonPipelineRunCompleteness(pipeline, nil, map[string]string{"source": "other-cache"})
	if err == nil || !strings.Contains(err.Error(), "不允许运行时覆盖") {
		t.Fatalf("non-runtime workspace override should be rejected, got %v", err)
	}
}

func TestValidateTektonPipelineRunCompletenessAllowsRuntimeWorkspaceOverride(t *testing.T) {
	pipeline := &model.DevopsPipeline{
		ID:   bson.NewObjectID(),
		Code: "workspace-override",
		TektonDagConfig: `{
			"nodes": [{"id": "build", "name": "build", "taskRef": {"name": "buildah"}, "workspaces": {"source": "source"}}],
			"workspaces": [{"name": "source", "bindingType": "persistentVolumeClaim", "claimName": "cache", "runtimeConfig": true}]
		}`,
	}

	if err := validateTektonPipelineRunCompleteness(pipeline, nil, map[string]string{"source": "other-cache"}); err != nil {
		t.Fatalf("runtime workspace override should pass, got %v", err)
	}
}

func TestValidateTektonDagConfigRejectsCycle(t *testing.T) {
	err := validateTektonDagConfig(`{
		"nodes": [
			{"id": "a", "name": "a", "taskRef": {"name": "task-a"}},
			{"id": "b", "name": "b", "taskRef": {"name": "task-b"}}
		],
		"edges": [
			{"source": "a", "target": "b", "type": "runAfter"},
			{"source": "b", "target": "a", "type": "runAfter"}
		]
	}`)
	if err == nil || !strings.Contains(err.Error(), "循环依赖") {
		t.Fatalf("cycle should be rejected, got %v", err)
	}
}

func TestValidateTektonDagConfigRejectsInvalidResultEdge(t *testing.T) {
	err := validateTektonDagConfig(`{
		"nodes": [
			{"id": "a", "name": "a", "taskRef": {"name": "task-a"}},
			{"id": "b", "name": "b", "taskRef": {"name": "task-b"}}
		],
		"edges": [
			{"source": "a", "target": "b", "type": "result", "targetParam": "digest"}
		]
	}`)
	if err == nil || !strings.Contains(err.Error(), "Result") {
		t.Fatalf("invalid result edge should be rejected, got %v", err)
	}
}

func TestValidateTektonDagConfigRejectsMissingWorkspace(t *testing.T) {
	err := validateTektonDagConfig(`{
		"workspaces": [{"name": "source"}],
		"nodes": [
			{"id": "build", "name": "build", "taskRef": {"name": "buildah"}, "workspaces": {"source": "missing"}}
		]
	}`)
	if err == nil || !strings.Contains(err.Error(), "Workspace 未声明") {
		t.Fatalf("missing workspace should be rejected, got %v", err)
	}
}

func TestValidateTektonDagConfigRejectsInvalidNodeWhen(t *testing.T) {
	err := validateTektonDagConfig(`{
		"nodes": [
			{"id": "deploy", "name": "deploy", "taskRef": {"name": "kubectl"}, "when": [{"input": "$(params.branch)", "operator": "exists", "values": ["main"]}]}
		]
	}`)
	if err == nil || !strings.Contains(err.Error(), "When operator") {
		t.Fatalf("invalid node when should be rejected, got %v", err)
	}
}

func TestValidateTektonDagConfigRejectsWhenEdgeMissingParam(t *testing.T) {
	err := validateTektonDagConfig(`{
		"nodes": [
			{"id": "build", "name": "build", "taskRef": {"name": "buildah"}},
			{"id": "deploy", "name": "deploy", "taskRef": {"name": "kubectl"}}
		],
		"edges": [
			{"source": "build", "target": "deploy", "type": "when", "whenInput": "$(params.missing)", "whenValues": ["main"]}
		]
	}`)
	if err == nil || !strings.Contains(err.Error(), "未声明") {
		t.Fatalf("missing when edge param should be rejected, got %v", err)
	}
}

func TestValidateTektonDagConfigRejectsNonStringWhenValues(t *testing.T) {
	err := validateTektonDagConfig(`{
		"params": [{"name": "branch", "type": "string"}],
		"nodes": [
			{"id": "build", "name": "build", "taskRef": {"name": "buildah"}, "when": [{"input": "$(params.branch)", "operator": "in", "values": [1]}]}
		]
	}`)
	if err == nil || !strings.Contains(err.Error(), "When values") {
		t.Fatalf("non-string node when values should be rejected, got %v", err)
	}

	err = validateTektonDagConfig(`{
		"params": [{"name": "branch", "type": "string"}],
		"nodes": [
			{"id": "build", "name": "build", "taskRef": {"name": "buildah"}},
			{"id": "deploy", "name": "deploy", "taskRef": {"name": "deploy"}}
		],
		"edges": [
			{"source": "build", "target": "deploy", "type": "when", "whenInput": "$(params.branch)", "whenValues": [1]}
		]
	}`)
	if err == nil || !strings.Contains(err.Error(), "When 连线 values") {
		t.Fatalf("non-string edge when values should be rejected, got %v", err)
	}
}

func TestRenderTektonDagPipelineYAMLNormalizesMatrixValues(t *testing.T) {
	pipeline := &model.DevopsPipeline{
		ID:   bson.NewObjectID(),
		Code: "matrix-values",
		TektonDagConfig: `{
			"nodes": [
				{"id": "build", "name": "build", "taskRef": {"name": "buildah"}, "matrix": {"params": [{"name": "platform", "values": ["linux", "arm64"]}]}}
			]
		}`,
	}
	content, err := renderTektonDagPipelineYAML("matrix-values", tektonBindingConfig{Namespace: "devops-demo"}, pipeline, nil)
	if err != nil {
		t.Fatalf("render matrix values: %v", err)
	}
	assertContains(t, content, "matrix:")
	assertContains(t, content, "value:")
	if strings.Contains(content, "values:") {
		t.Fatalf("matrix should render Tekton value, not frontend values:\n%s", content)
	}
}

func TestValidateTektonDagConfigRejectsInvalidMatrix(t *testing.T) {
	err := validateTektonDagConfig(`{
		"nodes": [
			{"id": "build", "name": "build", "taskRef": {"name": "buildah"}, "matrix": {"params": [{"name": "platform", "values": []}]}}
		]
	}`)
	if err == nil || !strings.Contains(err.Error(), "Matrix") {
		t.Fatalf("invalid matrix should be rejected, got %v", err)
	}
}

func TestValidateTektonDagConfigRejectsNonStringMatrixValues(t *testing.T) {
	err := validateTektonDagConfig(`{
		"nodes": [
			{"id": "build", "name": "build", "taskRef": {"name": "buildah"}, "matrix": {"params": [{"name": "platform", "values": ["linux", 1]}]}}
		]
	}`)
	if err == nil || !strings.Contains(err.Error(), "Matrix value") {
		t.Fatalf("non-string matrix values should be rejected, got %v", err)
	}

	err = validateTektonDagConfig(`{
		"nodes": [
			{"id": "build", "name": "build", "taskRef": {"name": "buildah"}, "matrix": {"params": [{"name": "platform", "values": ["linux"]}], "include": [{"params": [{"name": "platform", "value": ["linux"]}]}]}}
		]
	}`)
	if err == nil || !strings.Contains(err.Error(), "Matrix include") {
		t.Fatalf("non-string matrix include value should be rejected, got %v", err)
	}
}

func TestValidateTektonDagConfigRejectsObjectParamResultWithoutProperties(t *testing.T) {
	err := validateTektonDagConfig(`{
		"params": [{"name": "git", "type": "object"}],
		"nodes": [{"id": "build", "name": "build", "taskRef": {"name": "buildah"}}]
	}`)
	if err == nil || !strings.Contains(err.Error(), "properties") {
		t.Fatalf("object pipeline param without properties should be rejected, got %v", err)
	}

	err = validateTektonDagConfig(`{
		"results": [{"name": "meta", "type": "object", "value": {"digest": "sha256:abc"}}],
		"nodes": [{"id": "build", "name": "build", "taskRef": {"name": "buildah"}}]
	}`)
	if err == nil || !strings.Contains(err.Error(), "properties") {
		t.Fatalf("object pipeline result without properties should be rejected, got %v", err)
	}
}

func TestValidateTektonDagConfigRejectsObjectParamNameWithDot(t *testing.T) {
	err := validateTektonDagConfig(`{
		"params": [{"name": "git.url", "type": "object", "properties": {"url": {"type": "string"}}}],
		"nodes": [{"id": "build", "name": "build", "taskRef": {"name": "buildah"}}]
	}`)
	if err == nil || !strings.Contains(err.Error(), "点号") {
		t.Fatalf("object pipeline param name with dot should be rejected, got %v", err)
	}
}

func TestValidateTektonDagConfigRejectsInvalidPipelineParamShape(t *testing.T) {
	err := validateTektonDagConfig(`{
		"params": [{"name": "bad param", "type": "string"}],
		"nodes": [{"id": "build", "name": "build", "taskRef": {"name": "buildah"}}]
	}`)
	if err == nil || !strings.Contains(err.Error(), "参数名不合法") {
		t.Fatalf("invalid pipeline param name should be rejected, got %v", err)
	}

	err = validateTektonDagConfig(`{
		"params": [{"name": "count", "type": "number"}],
		"nodes": [{"id": "build", "name": "build", "taskRef": {"name": "buildah"}}]
	}`)
	if err == nil || !strings.Contains(err.Error(), "参数类型不支持") {
		t.Fatalf("unsupported pipeline param type should be rejected, got %v", err)
	}

	err = validateTektonDagConfig(`{
		"params": [{"name": "meta", "type": "object", "properties": {"count": {"type": "number"}}}],
		"nodes": [{"id": "build", "name": "build", "taskRef": {"name": "buildah"}}]
	}`)
	if err == nil || !strings.Contains(err.Error(), "properties 类型不支持") {
		t.Fatalf("unsupported pipeline param property type should be rejected, got %v", err)
	}
}

func TestValidateTektonDagConfigRejectsInvalidWorkspaceAndResultNames(t *testing.T) {
	err := validateTektonDagConfig(`{
		"workspaces": [{"name": "BadWorkspace"}],
		"nodes": [{"id": "build", "name": "build", "taskRef": {"name": "buildah"}}]
	}`)
	if err == nil || !strings.Contains(err.Error(), "Workspace 名称") {
		t.Fatalf("invalid workspace name should be rejected, got %v", err)
	}

	err = validateTektonDagConfig(`{
		"results": [{"name": "bad result", "value": "$(tasks.build.results.digest)"}],
		"nodes": [{"id": "build", "name": "build", "taskRef": {"name": "buildah"}, "results": ["digest"]}]
	}`)
	if err == nil || !strings.Contains(err.Error(), "Result 名称不合法") {
		t.Fatalf("invalid pipeline result name should be rejected, got %v", err)
	}
}

func TestValidateTektonDagConfigMatrixSatisfiesRequiredParam(t *testing.T) {
	err := validateTektonDagConfig(`{
		"nodes": [{
			"id": "build",
			"name": "build",
			"taskRef": {"name": "buildah"},
			"paramRequirements": [{"name": "platform", "required": true}],
			"matrix": {"params": [{"name": "platform", "values": ["linux", "arm64"]}]}
		}]
	}`)
	if err != nil {
		t.Fatalf("matrix param should satisfy required task param, got %v", err)
	}
}

func TestValidateTektonDagConfigAllowsEmptyArrayObjectDefaults(t *testing.T) {
	err := validateTektonDagConfig(`{
		"params": [
			{"name": "items", "type": "array", "required": true, "default": []},
			{"name": "meta", "type": "object", "required": true, "default": {}, "properties": {"digest": {"type": "string"}}}
		],
		"nodes": [{
			"id": "build",
			"name": "build",
			"taskRef": {"name": "buildah"},
			"paramRequirements": [
				{"name": "items", "required": true, "defaultValue": []},
				{"name": "meta", "required": true, "defaultValue": {}}
			]
		}]
	}`)
	if err != nil {
		t.Fatalf("empty array/object defaults should be accepted, got %v", err)
	}
}

func TestValidateTektonDagTaskParamsExistRejectsUnknownMatrixIncludeParam(t *testing.T) {
	node := tektonDagNode{
		ID:     "build",
		Name:   "build",
		Params: json.RawMessage(`[]`),
		Matrix: map[string]any{
			"params": []any{
				map[string]any{"name": "platform", "values": []any{"linux"}},
			},
			"include": []any{
				map[string]any{
					"params": []any{
						map[string]any{"name": "unknown", "value": "extra"},
					},
				},
			},
		},
	}
	taskInfo := devopstekton.TaskInfo{Params: []string{"platform"}}

	err := validateTektonDagTaskParamsExist(node, taskInfo, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "Tekton Matrix 参数不存在") {
		t.Fatalf("unknown matrix include param should be rejected, got %v", err)
	}
}

func TestValidateTektonDagConfigRejectsInvalidTaskRefKind(t *testing.T) {
	err := validateTektonDagConfig(`{
		"nodes": [
			{"id": "build", "name": "build", "taskRef": {"kind": "Job", "name": "buildah"}}
		]
	}`)
	if err == nil || !strings.Contains(err.Error(), "taskRef kind") {
		t.Fatalf("invalid taskRef kind should be rejected, got %v", err)
	}
}

func TestValidateTektonDagConfigRejectsInvalidNodeExecutionFields(t *testing.T) {
	err := validateTektonDagConfig(`{
		"nodes": [
			{"id": "build", "name": "build", "taskRef": {"name": "buildah"}, "onError": "retry"}
		]
	}`)
	if err == nil || !strings.Contains(err.Error(), "onError") {
		t.Fatalf("invalid onError should be rejected, got %v", err)
	}

	err = validateTektonDagConfig(`{
		"nodes": [
			{"id": "build", "name": "build", "taskRef": {"name": "buildah"}, "timeout": "1hour"}
		]
	}`)
	if err == nil || !strings.Contains(err.Error(), "timeout") {
		t.Fatalf("invalid timeout should be rejected, got %v", err)
	}

	err = validateTektonDagConfig(`{
		"nodes": [
			{"id": "build", "name": "build", "taskRef": {"name": "buildah"}, "retries": -1}
		]
	}`)
	if err == nil || !strings.Contains(err.Error(), "retries") {
		t.Fatalf("invalid retries should be rejected, got %v", err)
	}
}

func TestValidateTektonDagConfigRejectsInvalidResolverParams(t *testing.T) {
	err := validateTektonDagConfig(`{
		"nodes": [
			{"id": "build", "name": "build", "taskRef": {"resolver": "cluster", "params": [{"name": "kind", "value": "task"}]}}
		]
	}`)
	if err == nil || !strings.Contains(err.Error(), "kind 和 name") {
		t.Fatalf("invalid resolver params should be rejected, got %v", err)
	}

	err = validateTektonDagConfig(`{
		"nodes": [
			{"id": "build", "name": "build", "taskRef": {"resolver": "git", "params": [{"name": "bad param", "value": "main"}]}}
		]
	}`)
	if err == nil || !strings.Contains(err.Error(), "params 名称不合法") {
		t.Fatalf("invalid resolver param name should be rejected, got %v", err)
	}

	err = validateTektonDagConfig(`{
		"nodes": [
			{"id": "build", "name": "build", "taskRef": {"resolver": "git", "params": [{"name": "revision", "value": "$(params.missing)"}]}}
		]
	}`)
	if err == nil || !strings.Contains(err.Error(), "Pipeline 参数未声明") {
		t.Fatalf("resolver param missing pipeline ref should be rejected, got %v", err)
	}
}

func TestValidateTektonDagConfigAcceptsClusterResolverParams(t *testing.T) {
	err := validateTektonDagConfig(`{
		"nodes": [
			{"id": "build", "name": "build", "taskRef": {"resolver": "cluster", "params": [{"name": "kind", "value": "task"}, {"name": "name", "value": "buildah"}, {"name": "namespace", "value": "tekton-tasks"}]}}
		]
	}`)
	if err != nil {
		t.Fatalf("cluster resolver params should pass, got %v", err)
	}
}

func TestRenderTektonDagPipelineYAMLDefaultsResolverWhenParamsProvided(t *testing.T) {
	pipeline := &model.DevopsPipeline{
		ID:   bson.NewObjectID(),
		Code: "resolver-default",
		TektonDagConfig: `{
			"nodes": [
				{"id": "build", "name": "build", "taskRef": {"params": [{"name": "kind", "value": "task"}, {"name": "name", "value": "buildah"}]}}
			]
		}`,
	}
	content, err := renderTektonDagPipelineYAML("resolver-default", tektonBindingConfig{Namespace: "devops-demo"}, pipeline, nil)
	if err != nil {
		t.Fatalf("resolver params without resolver should render: %v", err)
	}
	assertContains(t, content, "resolver: cluster")
	assertContains(t, content, "name: kind")
	assertContains(t, content, "value: buildah")
}

func TestRenderTektonDagPipelineYAMLPreservesResolverRefAPIVersion(t *testing.T) {
	pipeline := &model.DevopsPipeline{
		ID:   bson.NewObjectID(),
		Code: "resolver-api-version",
		TektonDagConfig: `{
			"nodes": [
				{"id": "build", "name": "build", "taskRef": {"apiVersion": "tekton.dev/v1", "resolver": "git", "params": [{"name": "url", "value": "https://example.com/tasks.git"}, {"name": "revision", "value": "main"}, {"name": "pathInRepo", "value": "task.yaml"}]}},
				{"id": "child", "name": "child", "pipelineRef": {"apiVersion": "tekton.dev/v1", "resolver": "git", "params": [{"name": "url", "value": "https://example.com/pipelines.git"}, {"name": "revision", "value": "main"}, {"name": "pathInRepo", "value": "pipeline.yaml"}]}}
			]
		}`,
	}

	content, err := renderTektonDagPipelineYAML("resolver-api-version", tektonBindingConfig{Namespace: "devops-demo"}, pipeline, nil)
	if err != nil {
		t.Fatalf("resolver apiVersion should render: %v", err)
	}
	if count := strings.Count(content, "apiVersion: tekton.dev/v1"); count != 3 {
		t.Fatalf("resolver apiVersion should render on pipeline, taskRef and pipelineRef, got %d:\n%s", count, content)
	}
}

func TestValidateTektonDagTaskSpecMappingsRejectsMissingParamWorkspaceAndResult(t *testing.T) {
	cfg, err := parseTektonDagConfig(`{
		"nodes": [
			{"id": "build", "name": "build", "taskRef": {"name": "buildah"}, "params": {"missing": "value"}, "workspaces": {"source": "source"}}
		]
	}`)
	if err != nil {
		t.Fatalf("parse config: %v", err)
	}
	err = validateTektonDagTaskSpecMappings(cfg, map[string]devopstekton.TaskInfo{
		"build": {Name: "buildah", Params: []string{"image"}, Workspaces: []string{"source"}, Results: []string{"digest"}},
	})
	if err == nil || !strings.Contains(err.Error(), "参数不存在") {
		t.Fatalf("missing param should be rejected, got %v", err)
	}

	cfg.Nodes[0].Params = json.RawMessage(`{"image": "value"}`)
	cfg.Nodes[0].Workspaces = json.RawMessage(`{"missing": "source"}`)
	err = validateTektonDagTaskSpecMappings(cfg, map[string]devopstekton.TaskInfo{
		"build": {Name: "buildah", Params: []string{"image"}, Workspaces: []string{"source"}, Results: []string{"digest"}},
	})
	if err == nil || !strings.Contains(err.Error(), "Workspace 不存在") {
		t.Fatalf("missing workspace should be rejected, got %v", err)
	}

	cfg, err = parseTektonDagConfig(`{
		"nodes": [
			{"id": "build", "name": "build", "taskRef": {"name": "buildah"}},
			{"id": "deploy", "name": "deploy", "taskRef": {"name": "deploy"}, "params": {"digest": "placeholder"}}
		],
		"edges": [{"source": "build", "target": "deploy", "type": "result", "resultName": "missing", "targetParam": "digest"}]
	}`)
	if err != nil {
		t.Fatalf("parse result config: %v", err)
	}
	err = validateTektonDagTaskSpecMappings(cfg, map[string]devopstekton.TaskInfo{
		"build":  {Name: "buildah", Results: []string{"digest"}},
		"deploy": {Name: "deploy", Params: []string{"digest"}},
	})
	if err == nil || !strings.Contains(err.Error(), "Result 不存在") {
		t.Fatalf("missing result should be rejected, got %v", err)
	}
}

func TestValidateTektonDagTaskSpecMappingsAcceptsKnownParamWorkspaceAndResult(t *testing.T) {
	cfg, err := parseTektonDagConfig(`{
		"nodes": [
			{"id": "build", "name": "build", "taskRef": {"name": "buildah"}, "params": {"image": "value"}, "workspaces": {"source": "source"}},
			{"id": "deploy", "name": "deploy", "taskRef": {"name": "deploy"}, "params": {"digest": "placeholder"}}
		],
		"edges": [{"source": "build", "target": "deploy", "type": "result", "resultName": "digest", "targetParam": "digest"}]
	}`)
	if err != nil {
		t.Fatalf("parse config: %v", err)
	}
	err = validateTektonDagTaskSpecMappings(cfg, map[string]devopstekton.TaskInfo{
		"build":  {Name: "buildah", Params: []string{"image"}, Workspaces: []string{"source"}, Results: []string{"digest"}},
		"deploy": {Name: "deploy", Params: []string{"digest"}},
	})
	if err != nil {
		t.Fatalf("valid spec mapping should pass, got %v", err)
	}
}

func TestRenderTektonDagPipelineYAMLPreservesCustomTaskRefAPIVersion(t *testing.T) {
	pipeline := &model.DevopsPipeline{
		ID:   bson.NewObjectID(),
		Code: "custom-task",
		TektonDagConfig: `{
			"pipeline": {"name": "custom-task", "namespace": "devops-demo"},
			"nodes": [{
				"id": "custom",
				"name": "custom",
				"taskRef": {"kind": "ExampleTask", "apiVersion": "example.dev/v1alpha1", "name": "custom-task"}
			}]
		}`,
	}

	content, err := renderTektonDagPipelineYAML("custom-task", tektonBindingConfig{Namespace: "devops-demo"}, pipeline, nil)
	if err != nil {
		t.Fatalf("custom taskRef should render: %v", err)
	}
	assertContains(t, content, "apiVersion: example.dev/v1alpha1")
	assertContains(t, content, "kind: ExampleTask")
}

func TestRenderTektonDagPipelineYAMLSupportsPipelineRef(t *testing.T) {
	pipeline := &model.DevopsPipeline{
		ID:   bson.NewObjectID(),
		Code: "pipeline-ref",
		TektonDagConfig: `{
			"pipeline": {"name": "pipeline-ref", "namespace": "devops-demo"},
			"nodes": [{
				"id": "child",
				"name": "child",
				"pipelineRef": {"name": "child-pipeline", "apiVersion": "tekton.dev/v1"}
			}]
		}`,
	}

	content, err := renderTektonDagPipelineYAML("pipeline-ref", tektonBindingConfig{Namespace: "devops-demo"}, pipeline, nil)
	if err != nil {
		t.Fatalf("pipelineRef should render: %v", err)
	}
	assertContains(t, content, "pipelineRef:")
	assertContains(t, content, "name: child-pipeline")
	assertContains(t, content, "apiVersion: tekton.dev/v1")
	assertNotContains(t, content, "taskRef:")
}

func TestValidateTektonDagConfigAcceptsPipelineRef(t *testing.T) {
	err := validateTektonDagConfig(`{
		"nodes": [{
			"id": "child",
			"name": "child",
			"pipelineRef": {"name": "child-pipeline"}
		}]
	}`)
	if err != nil {
		t.Fatalf("pipelineRef should pass DAG validation: %v", err)
	}
}

func TestValidateTektonDagConfigRejectsDeprecatedRefBundle(t *testing.T) {
	err := validateTektonDagConfig(`{
		"nodes": [{
			"id": "child",
			"name": "child",
			"pipelineRef": {"bundle": "gcr.io/example/pipeline:latest"}
		}]
	}`)
	if err == nil || !strings.Contains(err.Error(), "bundle") {
		t.Fatalf("pipelineRef.bundle should be rejected, got %v", err)
	}

	err = validateTektonDagConfig(`{
		"nodes": [{
			"id": "build",
			"name": "build",
			"taskRef": {"name": "buildah", "bundle": "gcr.io/example/task:latest"}
		}]
	}`)
	if err == nil || !strings.Contains(err.Error(), "bundle") {
		t.Fatalf("taskRef.bundle should be rejected, got %v", err)
	}
}

func TestValidateTektonDagConfigRejectsMixedTaskAndPipelineRef(t *testing.T) {
	err := validateTektonDagConfig(`{
		"nodes": [{
			"id": "child",
			"name": "child",
			"taskRef": {"name": "buildah"},
			"pipelineRef": {"name": "child-pipeline"}
		}]
	}`)
	if err == nil || !strings.Contains(err.Error(), "只能配置一个") {
		t.Fatalf("mixed taskRef and pipelineRef should be rejected, got %v", err)
	}
}

func TestValidateTektonDagConfigRejectsInlinePipelineSpec(t *testing.T) {
	err := validateTektonDagConfig(`{
		"nodes": [{
			"id": "child",
			"name": "child",
			"pipelineSpec": {"tasks": []}
		}]
	}`)
	if err == nil || !strings.Contains(err.Error(), "inline pipelineSpec") {
		t.Fatalf("inline pipelineSpec should be rejected, got %v", err)
	}
}

func assertContains(t *testing.T, content, want string) {
	t.Helper()
	if !strings.Contains(content, want) {
		t.Fatalf("content should contain %q:\n%s", want, content)
	}
}

func assertNotContains(t *testing.T, content, want string) {
	t.Helper()
	if strings.Contains(content, want) {
		t.Fatalf("content should not contain %q:\n%s", want, content)
	}
}
