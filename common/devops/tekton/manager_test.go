package tekton

import (
	"context"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	kubernetesfake "k8s.io/client-go/kubernetes/fake"
)

func TestParseStepResourceV1Beta1ParamImage(t *testing.T) {
	content := `
apiVersion: tekton.dev/v1beta1
kind: Task
metadata:
  name: git-clone
  labels:
    app.kubernetes.io/version: "0.9"
  annotations:
    tekton.dev/displayName: "git clone"
    tekton.dev/deprecated: "true"
spec:
  workspaces:
    - name: output
  params:
    - name: url
      type: string
    - name: gitInitImage
      type: string
      default: "ghcr.io/tektoncd/github.com/tektoncd/pipeline/cmd/git-init:v0.40.2"
  results:
    - name: commit
  steps:
    - name: clone
      image: "$(params.gitInitImage)"
      env:
        - name: PARAM_URL
          value: $(params.url)
      script: |
        #!/usr/bin/env sh
        set -eu
`
	resource, err := ParseStepResource(content)
	if err != nil {
		t.Fatalf("ParseStepResource() error = %v", err)
	}
	if resource.APIVersion != tektonV1Beta1 {
		t.Fatalf("APIVersion = %q, want %q", resource.APIVersion, tektonV1Beta1)
	}
	if resource.Name != "git-clone" {
		t.Fatalf("Name = %q, want git-clone", resource.Name)
	}
	spec, ok := resource.Object.Object["spec"].(map[string]any)
	if !ok {
		t.Fatalf("spec parse failed")
	}
	steps, ok := spec["steps"].([]any)
	if !ok || len(steps) != 1 {
		t.Fatalf("steps parse failed")
	}
	step, ok := steps[0].(map[string]any)
	if !ok {
		t.Fatalf("step parse failed")
	}
	if step["image"] != "$(params.gitInitImage)" {
		t.Fatalf("image = %v, want $(params.gitInitImage)", step["image"])
	}
}

func TestParseStepResourceAllowsStepRef(t *testing.T) {
	content := `
apiVersion: tekton.dev/v1
kind: Task
metadata:
  name: ref-step
spec:
  steps:
    - name: run-action
      ref:
        name: shared-action
`
	resource, err := ParseStepResource(content)
	if err != nil {
		t.Fatalf("ParseStepResource() step ref error = %v", err)
	}
	if resource.Name != "ref-step" {
		t.Fatalf("Name = %q, want ref-step", resource.Name)
	}
}

func TestParseStepResourceAllowsStepResolverRef(t *testing.T) {
	content := `
apiVersion: tekton.dev/v1
kind: Task
metadata:
  name: resolver-ref-step
spec:
  steps:
    - name: run-action
      ref:
        resolver: git
        params:
          - name: url
            value: https://example.com/actions.git
          - name: pathInRepo
            value: action.yaml
`
	resource, err := ParseStepResource(content)
	if err != nil {
		t.Fatalf("ParseStepResource() resolver ref error = %v", err)
	}
	if resource.Name != "resolver-ref-step" {
		t.Fatalf("Name = %q, want resolver-ref-step", resource.Name)
	}
}

func TestParseStepResourceRejectsStepWithoutImageOrRef(t *testing.T) {
	content := `
apiVersion: tekton.dev/v1
kind: Task
metadata:
  name: invalid-step
spec:
  steps:
    - name: missing-image
      script: echo ok
`
	_, err := ParseStepResource(content)
	if err == nil || !strings.Contains(err.Error(), "image/ref") {
		t.Fatalf("missing image/ref should be rejected, got %v", err)
	}
}

func TestParseStepResourceRejectsInvalidTaskMetadataName(t *testing.T) {
	content := `
apiVersion: tekton.dev/v1
kind: Task
metadata:
  name: Invalid_Task_Name
spec:
  steps:
    - name: run
      image: busybox
`
	_, err := ParseStepResource(content)
	if err == nil || !strings.Contains(err.Error(), "metadata.name") {
		t.Fatalf("invalid task metadata name should be rejected, got %v", err)
	}
}

func TestParseStepResourceRejectsInvalidTaskObjectNames(t *testing.T) {
	content := `
apiVersion: tekton.dev/v1
kind: Task
metadata:
  name: invalid-step-name
spec:
  steps:
    - name: Build_Image
      image: busybox
`
	_, err := ParseStepResource(content)
	if err == nil || !strings.Contains(err.Error(), "DNS_LABEL") {
		t.Fatalf("invalid step name should be rejected, got %v", err)
	}

	content = `
apiVersion: tekton.dev/v1
kind: Task
metadata:
  name: invalid-sidecar-name
spec:
  sidecars:
    - name: Cache_Sidecar
      image: busybox
  steps:
    - name: run
      image: busybox
`
	_, err = ParseStepResource(content)
	if err == nil || !strings.Contains(err.Error(), "sidecars.name") {
		t.Fatalf("invalid sidecar name should be rejected, got %v", err)
	}

	content = `
apiVersion: tekton.dev/v1
kind: Task
metadata:
  name: invalid-volume-name
spec:
  volumes:
    - name: Source_Volume
      emptyDir: {}
  steps:
    - name: run
      image: busybox
`
	_, err = ParseStepResource(content)
	if err == nil || !strings.Contains(err.Error(), "volumes.name") {
		t.Fatalf("invalid volume name should be rejected, got %v", err)
	}
}

func TestParseStepResourceRejectsStepRefWithImage(t *testing.T) {
	content := `
apiVersion: tekton.dev/v1
kind: Task
metadata:
  name: invalid-ref-step
spec:
  steps:
    - name: run-action
      ref:
        name: shared-action
      image: busybox
`
	_, err := ParseStepResource(content)
	if err == nil || !strings.Contains(err.Error(), "使用 ref") {
		t.Fatalf("ref with image should be rejected, got %v", err)
	}
}

func TestParseStepResourceAllowsOfficialStepAdvancedFields(t *testing.T) {
	content := `
apiVersion: tekton.dev/v1
kind: Task
metadata:
  name: advanced-step
spec:
  steps:
    - name: build
      image: golang:1.25
      workingDir: /workspace/source
      envFrom:
        - configMapRef:
            name: build-env
      computeResources:
        requests:
          cpu: 500m
      securityContext:
        runAsNonRoot: true
      stdoutConfig:
        path: /tekton/results/stdout
      stderrConfig:
        path: /tekton/results/stderr
      params:
        - name: flag
          value: enabled
      results:
        - name: step-result
      when:
        - input: "$(params.flag)"
          operator: in
          values: ["enabled"]
      workspaces:
        - name: source
`
	resource, err := ParseStepResource(content)
	if err != nil {
		t.Fatalf("ParseStepResource() advanced fields error = %v", err)
	}
	spec := resource.Object.Object["spec"].(map[string]any)
	steps := spec["steps"].([]any)
	step := steps[0].(map[string]any)
	if _, ok := step["computeResources"]; !ok {
		t.Fatalf("computeResources should be preserved")
	}
	if _, ok := step["stdoutConfig"]; !ok {
		t.Fatalf("stdoutConfig should be preserved")
	}
	for _, field := range []string{"params", "results", "when", "workspaces"} {
		if _, ok := step[field]; !ok {
			t.Fatalf("%s should be preserved", field)
		}
	}
}

func TestParseStepResourceRejectsStepRefWithAdvancedContainerFields(t *testing.T) {
	content := `
apiVersion: tekton.dev/v1
kind: Task
metadata:
  name: invalid-ref-advanced-step
spec:
  steps:
    - name: run-action
      ref:
        name: shared-action
      computeResources:
        requests:
          cpu: 500m
`
	_, err := ParseStepResource(content)
	if err == nil || !strings.Contains(err.Error(), "computeResources") {
		t.Fatalf("ref with computeResources should be rejected, got %v", err)
	}
}

func TestParseStepResourceRejectsStepRefWithResults(t *testing.T) {
	content := `
apiVersion: tekton.dev/v1
kind: Task
metadata:
  name: invalid-ref-results-step
spec:
  steps:
    - name: run-action
      ref:
        name: shared-action
      results:
        - name: step-result
`
	_, err := ParseStepResource(content)
	if err == nil || !strings.Contains(err.Error(), "results") {
		t.Fatalf("ref with results should be rejected, got %v", err)
	}
}

func TestParseStepResourceRejectsInvalidStepExecutionFields(t *testing.T) {
	content := `
apiVersion: tekton.dev/v1
kind: Task
metadata:
  name: invalid-step-execution
spec:
  steps:
    - name: run
      image: alpine:3.20
      onError: retry
`
	_, err := ParseStepResource(content)
	if err == nil || !strings.Contains(err.Error(), "onError") {
		t.Fatalf("invalid onError should be rejected, got %v", err)
	}

	content = `
apiVersion: tekton.dev/v1
kind: Task
metadata:
  name: invalid-step-timeout
spec:
  steps:
    - name: run
      image: alpine:3.20
      timeout: 1hour
`
	_, err = ParseStepResource(content)
	if err == nil || !strings.Contains(err.Error(), "timeout") {
		t.Fatalf("invalid timeout should be rejected, got %v", err)
	}
}

func TestParseStepResourceRejectsInvalidSidecarsAndVolumes(t *testing.T) {
	content := `
apiVersion: tekton.dev/v1
kind: Task
metadata:
  name: invalid-sidecar
spec:
  sidecars:
    - name: docker
  steps:
    - name: run
      image: alpine:3.20
`
	_, err := ParseStepResource(content)
	if err == nil || !strings.Contains(err.Error(), "sidecars") || !strings.Contains(err.Error(), "image") {
		t.Fatalf("sidecar without image should be rejected, got %v", err)
	}

	content = `
apiVersion: tekton.dev/v1
kind: Task
metadata:
  name: duplicate-sidecar
spec:
  sidecars:
    - name: docker
      image: docker:dind
    - name: docker
      image: docker:dind
  steps:
    - name: run
      image: alpine:3.20
`
	_, err = ParseStepResource(content)
	if err == nil || !strings.Contains(err.Error(), "sidecars.name 不能重复") {
		t.Fatalf("duplicate sidecar names should be rejected, got %v", err)
	}

	content = `
apiVersion: tekton.dev/v1
kind: Task
metadata:
  name: invalid-volume
spec:
  volumes:
    - name: cache
  steps:
    - name: run
      image: alpine:3.20
`
	_, err = ParseStepResource(content)
	if err == nil || !strings.Contains(err.Error(), "volumes") || !strings.Contains(err.Error(), "卷来源") {
		t.Fatalf("volume without source should be rejected, got %v", err)
	}
}

func TestParseStepResourceRejectsObjectParamWithoutProperties(t *testing.T) {
	content := `
apiVersion: tekton.dev/v1
kind: Task
metadata:
  name: object-param
spec:
  params:
    - name: git
      type: object
  results:
    - name: meta
      type: object
  steps:
    - name: run
      image: alpine:3.20
`
	_, err := ParseStepResource(content)
	if err == nil || !strings.Contains(err.Error(), "properties") {
		t.Fatalf("object param without properties should be rejected, got %v", err)
	}

	content = `
apiVersion: tekton.dev/v1
kind: Task
metadata:
  name: object-result
spec:
  params:
    - name: git
      type: object
      properties:
        url:
          type: string
  results:
    - name: meta
      type: object
  steps:
    - name: run
      image: alpine:3.20
`
	_, err = ParseStepResource(content)
	if err == nil || !strings.Contains(err.Error(), "properties") {
		t.Fatalf("object result without properties should be rejected, got %v", err)
	}
}

func TestParseStepResourceRejectsObjectParamNameWithDot(t *testing.T) {
	content := `
apiVersion: tekton.dev/v1
kind: Task
metadata:
  name: object-param-dot
spec:
  params:
    - name: git.url
      type: object
      properties:
        url:
          type: string
  steps:
    - name: run
      image: alpine:3.20
`
	_, err := ParseStepResource(content)
	if err == nil || !strings.Contains(err.Error(), "点号") {
		t.Fatalf("object param name with dot should be rejected, got %v", err)
	}
}

func TestParseStepResourceRejectsInvalidTypedParamValues(t *testing.T) {
	content := `
apiVersion: tekton.dev/v1
kind: Task
metadata:
  name: invalid-array-default
spec:
  params:
    - name: platforms
      type: array
      default:
        - linux
        - 1
  steps:
    - name: run
      image: busybox
`
	_, err := ParseStepResource(content)
	if err == nil || !strings.Contains(err.Error(), "字符串数组") {
		t.Fatalf("array default with non-string item should be rejected, got %v", err)
	}

	content = `
apiVersion: tekton.dev/v1
kind: Task
metadata:
  name: invalid-object-result
spec:
  results:
    - name: meta
      type: object
      properties:
        digest:
          type: string
      value: not-object
  steps:
    - name: run
      image: busybox
`
	_, err = ParseStepResource(content)
	if err == nil || !strings.Contains(err.Error(), "必须是对象") {
		t.Fatalf("object result value should be rejected, got %v", err)
	}
}

func TestParseStepResourceRejectsInvalidObjectPropertyType(t *testing.T) {
	content := `
apiVersion: tekton.dev/v1
kind: Task
metadata:
  name: object-property-type
spec:
  params:
    - name: git
      type: object
      properties:
        url:
          type: number
  steps:
    - name: run
      image: alpine:3.20
`
	_, err := ParseStepResource(content)
	if err == nil || !strings.Contains(err.Error(), "类型不支持") {
		t.Fatalf("invalid object property type should be rejected, got %v", err)
	}

	content = `
apiVersion: tekton.dev/v1
kind: Task
metadata:
  name: invalid-param-type
spec:
  params:
    - name: git
      type: number
  steps:
    - name: run
      image: alpine:3.20
`
	_, err = ParseStepResource(content)
	if err == nil || !strings.Contains(err.Error(), "type 只支持") {
		t.Fatalf("invalid param type should be rejected, got %v", err)
	}

	content = `
apiVersion: tekton.dev/v1
kind: Task
metadata:
  name: invalid-properties-type
spec:
  results:
    - name: meta
      type: string
      properties:
        digest:
          type: string
  steps:
    - name: run
      image: alpine:3.20
`
	_, err = ParseStepResource(content)
	if err == nil || !strings.Contains(err.Error(), "properties") {
		t.Fatalf("properties on non-object result should be rejected, got %v", err)
	}
}

func TestTektonTaskDescriptionPrefersOfficialDisplayName(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]any{
		"spec": map[string]any{
			"displayName": "Build Image",
			"description": "build container image",
		},
	}}

	if got := tektonTaskDescription(obj); got != "Build Image" {
		t.Fatalf("description = %q, want Build Image", got)
	}
}

func TestPipelineRunParamsAndWorkspacesFromObject(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]any{
		"spec": map[string]any{
			"params": []any{
				map[string]any{"name": "branch", "value": "main"},
				map[string]any{"name": "flags", "value": []any{"a", "b"}},
			},
			"workspaces": []any{
				map[string]any{"name": "source"},
				map[string]any{"name": "cache"},
			},
		},
	}}

	params := pipelineRunParamsFromObject(obj)
	if params["branch"] != "main" {
		t.Fatalf("branch = %q, want main", params["branch"])
	}
	if params["flags"] != `["a","b"]` {
		t.Fatalf("flags = %q, want JSON array", params["flags"])
	}
	workspaces := pipelineRunWorkspacesFromObject(obj)
	if len(workspaces) != 2 || workspaces[0] != "source" || workspaces[1] != "cache" {
		t.Fatalf("workspaces = %#v", workspaces)
	}
}

func TestStatusFromObjectParsesSkippedTasks(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]any{
		"status": map[string]any{
			"skippedTasks": []any{
				map[string]any{
					"name":    "deploy",
					"reason":  "WhenExpressionsSkip",
					"message": "when expression evaluated to false",
				},
			},
		},
	}}

	status := statusFromObject(obj)
	if len(status.SkippedTasks) != 1 {
		t.Fatalf("SkippedTasks = %#v, want one item", status.SkippedTasks)
	}
	if status.SkippedTasks[0].Name != "deploy" {
		t.Fatalf("SkippedTasks[0].Name = %q, want deploy", status.SkippedTasks[0].Name)
	}
	if status.SkippedTasks[0].Reason != "WhenExpressionsSkip" {
		t.Fatalf("SkippedTasks[0].Reason = %q", status.SkippedTasks[0].Reason)
	}
}

func TestRuntimeBindingExistsChecks(t *testing.T) {
	ctx := context.Background()
	client := &Client{kubeClient: kubernetesfake.NewSimpleClientset(
		&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "maven-settings", Namespace: "ci"}},
		&storagev1.StorageClass{ObjectMeta: metav1.ObjectMeta{Name: "standard"}},
	)}

	if err := client.ConfigMapExists(ctx, "ci", "maven-settings"); err != nil {
		t.Fatalf("ConfigMapExists existing error = %v", err)
	}
	if err := client.StorageClassExists(ctx, "standard"); err != nil {
		t.Fatalf("StorageClassExists existing error = %v", err)
	}
	if err := client.ConfigMapExists(ctx, "ci", "missing"); err == nil || !strings.Contains(err.Error(), "ConfigMap 不存在") {
		t.Fatalf("ConfigMapExists missing error = %v", err)
	}
	if err := client.StorageClassExists(ctx, "missing"); err == nil || !strings.Contains(err.Error(), "StorageClass 不存在") {
		t.Fatalf("StorageClassExists missing error = %v", err)
	}
}

func TestTaskAndClusterTaskExists(t *testing.T) {
	task := &unstructured.Unstructured{Object: map[string]any{
		"spec": map[string]any{
			"params": []any{
				map[string]any{"name": "image"},
			},
			"workspaces": []any{
				map[string]any{"name": "source"},
			},
			"results": []any{
				map[string]any{"name": "digest"},
			},
		},
	}}
	task.SetGroupVersionKind(schema.GroupVersionKind{Group: "tekton.dev", Version: "v1", Kind: "Task"})
	task.SetNamespace("ci")
	task.SetName("buildah")
	clusterTask := &unstructured.Unstructured{Object: map[string]any{}}
	clusterTask.SetGroupVersionKind(schema.GroupVersionKind{Group: "tekton.dev", Version: "v1", Kind: "ClusterTask"})
	clusterTask.SetName("git-clone")
	client := &Client{
		dynamicClient: dynamicfake.NewSimpleDynamicClient(runtime.NewScheme(), task, clusterTask),
	}

	if err := client.TaskExists(context.Background(), "ci", "buildah"); err != nil {
		t.Fatalf("TaskExists() error = %v", err)
	}
	if err := client.ClusterTaskExists(context.Background(), "git-clone"); err != nil {
		t.Fatalf("ClusterTaskExists() error = %v", err)
	}
	info, err := client.GetTask(context.Background(), "ci", "buildah")
	if err != nil {
		t.Fatalf("GetTask() error = %v", err)
	}
	if len(info.Params) != 1 || info.Params[0] != "image" {
		t.Fatalf("Task params = %#v, want image", info.Params)
	}
	if len(info.Workspaces) != 1 || info.Workspaces[0] != "source" {
		t.Fatalf("Task workspaces = %#v, want source", info.Workspaces)
	}
	if len(info.Results) != 1 || info.Results[0] != "digest" {
		t.Fatalf("Task results = %#v, want digest", info.Results)
	}
	err = client.TaskExists(context.Background(), "ci", "missing")
	if err == nil || !strings.Contains(err.Error(), "不存在") {
		t.Fatalf("missing Task should be rejected, got %v", err)
	}
}

func TestApplyAndDeletePrunerConfigMap(t *testing.T) {
	ctx := context.Background()
	client := &Client{kubeClient: kubernetesfake.NewSimpleClientset()}
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tekton-pruner-namespace-spec",
			Namespace: "ci",
		},
		Data: map[string]string{"ns-config": "historyLimit: 3\n"},
	}

	if err := client.ApplyPrunerConfigMap(ctx, cm); err != nil {
		t.Fatalf("ApplyPrunerConfigMap(create) error = %v", err)
	}
	cm.Data["ns-config"] = "historyLimit: 5\n"
	if err := client.ApplyPrunerConfigMap(ctx, cm); err != nil {
		t.Fatalf("ApplyPrunerConfigMap(update) error = %v", err)
	}
	got, err := client.kubeClient.CoreV1().ConfigMaps("ci").Get(ctx, "tekton-pruner-namespace-spec", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Get ConfigMap error = %v", err)
	}
	if got.Data["ns-config"] != "historyLimit: 5\n" {
		t.Fatalf("ns-config = %q", got.Data["ns-config"])
	}
	if err := client.DeletePrunerConfigMap(ctx, "ci", "tekton-pruner-namespace-spec"); err != nil {
		t.Fatalf("DeletePrunerConfigMap() error = %v", err)
	}
	if err := client.DeletePrunerConfigMap(ctx, "ci", "tekton-pruner-namespace-spec"); err != nil {
		t.Fatalf("DeletePrunerConfigMap(not found) error = %v", err)
	}
}

func TestManagedTektonResourceCRUD(t *testing.T) {
	ctx := context.Background()
	client := &Client{kubeClient: kubernetesfake.NewSimpleClientset()}
	const (
		namespace = "ci"
		projectID = "project-1"
		bindingID = "binding-1"
	)

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "demo-config"},
		Data:       map[string]string{"key": "value"},
	}
	if err := client.ApplyConfigMap(ctx, namespace, cm); err != nil {
		t.Fatalf("ApplyConfigMap(create) error = %v", err)
	}
	cm.Data["key"] = "updated"
	if err := client.ApplyConfigMap(ctx, namespace, cm); err != nil {
		t.Fatalf("ApplyConfigMap(update) error = %v", err)
	}
	gotCM, err := client.GetConfigMap(ctx, namespace, "demo-config")
	if err != nil {
		t.Fatalf("GetConfigMap() error = %v", err)
	}
	if gotCM.Data["key"] != "updated" {
		t.Fatalf("ConfigMap data = %q", gotCM.Data["key"])
	}
	configMaps, err := client.ListConfigMaps(ctx, namespace, "demo")
	if err != nil || len(configMaps) != 1 {
		t.Fatalf("ListConfigMaps() items = %d err = %v", len(configMaps), err)
	}
	if err := client.DeleteConfigMap(ctx, namespace, "demo-config"); err != nil {
		t.Fatalf("DeleteConfigMap() error = %v", err)
	}

	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: "demo-pvc"},
	}
	if err := client.ApplyPersistentVolumeClaim(ctx, namespace, pvc); err != nil {
		t.Fatalf("ApplyPersistentVolumeClaim(create) error = %v", err)
	}
	if _, err := client.GetPersistentVolumeClaim(ctx, namespace, "demo-pvc"); err != nil {
		t.Fatalf("GetPersistentVolumeClaim() error = %v", err)
	}
	pvcs, err := client.ListPersistentVolumeClaims(ctx, namespace, "demo")
	if err != nil || len(pvcs) != 1 {
		t.Fatalf("ListPersistentVolumeClaims() items = %d err = %v", len(pvcs), err)
	}
	if err := client.DeletePersistentVolumeClaim(ctx, namespace, "demo-pvc"); err != nil {
		t.Fatalf("DeletePersistentVolumeClaim() error = %v", err)
	}

	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{Name: "demo-sa"},
	}
	if err := client.ApplyServiceAccount(ctx, namespace, sa); err != nil {
		t.Fatalf("ApplyServiceAccount(create) error = %v", err)
	}
	if _, err := client.GetServiceAccount(ctx, namespace, "demo-sa"); err != nil {
		t.Fatalf("GetServiceAccount() error = %v", err)
	}
	serviceAccounts, err := client.ListServiceAccounts(ctx, namespace, "demo")
	if err != nil || len(serviceAccounts) != 1 {
		t.Fatalf("ListServiceAccounts() items = %d err = %v", len(serviceAccounts), err)
	}
	if err := client.DeleteServiceAccount(ctx, namespace, "demo-sa"); err != nil {
		t.Fatalf("DeleteServiceAccount() error = %v", err)
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "demo-secret",
			Labels: map[string]string{
				"kube-nova.io/devops-project-id": projectID,
				"kube-nova.io/devops-binding-id": bindingID,
			},
		},
		Data: map[string][]byte{"token": []byte("abc")},
	}
	if err := client.ApplyProjectSecret(ctx, namespace, secret); err != nil {
		t.Fatalf("ApplyProjectSecret(create) error = %v", err)
	}
	secret.Data["token"] = []byte("updated")
	if err := client.ApplyProjectSecret(ctx, namespace, secret); err != nil {
		t.Fatalf("ApplyProjectSecret(update) error = %v", err)
	}
	gotSecret, err := client.GetProjectSecret(ctx, namespace, "demo-secret", projectID, bindingID)
	if err != nil {
		t.Fatalf("GetProjectSecret() error = %v", err)
	}
	if string(gotSecret.Data["token"]) != "updated" {
		t.Fatalf("Secret data = %q", string(gotSecret.Data["token"]))
	}
	secrets, err := client.ListProjectSecrets(ctx, namespace, "demo", projectID, bindingID)
	if err != nil || len(secrets) != 1 {
		t.Fatalf("ListProjectSecrets() items = %d err = %v", len(secrets), err)
	}
	if err := client.DeleteProjectSecret(ctx, namespace, "demo-secret", projectID, bindingID); err != nil {
		t.Fatalf("DeleteProjectSecret() error = %v", err)
	}
}

func TestManagedTektonResourceDescribe(t *testing.T) {
	ctx := context.Background()
	const (
		namespace = "ci"
		projectID = "project-1"
		bindingID = "binding-1"
	)
	client := &Client{kubeClient: kubernetesfake.NewSimpleClientset(
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: "demo-config", Namespace: namespace},
			Data:       map[string]string{"key": "value"},
		},
		&corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{Name: "demo-pvc", Namespace: namespace},
			Status:     corev1.PersistentVolumeClaimStatus{Phase: corev1.ClaimBound},
		},
		&corev1.ServiceAccount{
			ObjectMeta:       metav1.ObjectMeta{Name: "demo-sa", Namespace: namespace},
			ImagePullSecrets: []corev1.LocalObjectReference{{Name: "pull-secret"}},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "demo-secret",
				Namespace: namespace,
				Labels: map[string]string{
					"kube-nova.io/managed-by":        "kube-nova",
					"kube-nova.io/devops-project-id": projectID,
					"kube-nova.io/devops-binding-id": bindingID,
				},
			},
			Type: corev1.SecretTypeOpaque,
			Data: map[string][]byte{"token": []byte("abc")},
		},
	)}

	cases := []struct {
		name string
		run  func() (string, error)
		want string
	}{
		{
			name: "configMap",
			run:  func() (string, error) { return client.DescribeConfigMap(ctx, namespace, "demo-config") },
			want: "Data Keys: key",
		},
		{
			name: "pvc",
			run:  func() (string, error) { return client.DescribePersistentVolumeClaim(ctx, namespace, "demo-pvc") },
			want: "Status: Bound",
		},
		{
			name: "serviceAccount",
			run:  func() (string, error) { return client.DescribeServiceAccount(ctx, namespace, "demo-sa") },
			want: "ImagePullSecrets: pull-secret",
		},
		{
			name: "secret",
			run: func() (string, error) {
				return client.DescribeProjectSecret(ctx, namespace, "demo-secret", projectID, bindingID)
			},
			want: "Data Keys: token",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			content, err := tc.run()
			if err != nil {
				t.Fatalf("Describe %s error = %v", tc.name, err)
			}
			if !strings.Contains(content, tc.want) {
				t.Fatalf("Describe %s content = %q, want %q", tc.name, content, tc.want)
			}
		})
	}
}

func TestRuntimeTektonResourceDescribeAndDelete(t *testing.T) {
	ctx := context.Background()
	pipeline := &unstructured.Unstructured{Object: map[string]any{
		"spec": map[string]any{
			"params":     []any{map[string]any{"name": "image"}},
			"workspaces": []any{map[string]any{"name": "source"}},
			"results":    []any{map[string]any{"name": "digest"}},
		},
	}}
	pipeline.SetGroupVersionKind(schema.GroupVersionKind{Group: "tekton.dev", Version: "v1", Kind: "Pipeline"})
	pipeline.SetNamespace("ci")
	pipeline.SetName("demo-pipeline")
	pipelineRun := &unstructured.Unstructured{Object: map[string]any{
		"status": map[string]any{
			"conditions": []any{
				map[string]any{"type": "Succeeded", "status": "True", "reason": "Succeeded"},
			},
		},
	}}
	pipelineRun.SetGroupVersionKind(schema.GroupVersionKind{Group: "tekton.dev", Version: "v1", Kind: "PipelineRun"})
	pipelineRun.SetNamespace("ci")
	pipelineRun.SetName("demo-run")
	client := &Client{
		kubeClient: kubernetesfake.NewSimpleClientset(&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "demo-pod", Namespace: "ci"},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{Name: "step-build"}},
			},
			Status: corev1.PodStatus{Phase: corev1.PodRunning, PodIP: "10.0.0.1"},
		}),
		dynamicClient: dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), map[schema.GroupVersionResource]string{
			{Group: "tekton.dev", Version: "v1", Resource: "pipelines"}:    "PipelineList",
			{Group: "tekton.dev", Version: "v1", Resource: "pipelineruns"}: "PipelineRunList",
			{Group: "tekton.dev", Version: "v1", Resource: "taskruns"}:     "TaskRunList",
		}, pipeline, pipelineRun),
	}

	podDesc, err := client.DescribePod(ctx, "ci", "demo-pod")
	if err != nil || !strings.Contains(podDesc, "Containers: step-build") {
		t.Fatalf("DescribePod() content = %q err = %v", podDesc, err)
	}
	pipelineDesc, err := client.DescribePipeline(ctx, "ci", "demo-pipeline")
	if err != nil || !strings.Contains(pipelineDesc, "Params: image") {
		t.Fatalf("DescribePipeline() content = %q err = %v", pipelineDesc, err)
	}
	runDesc, err := client.DescribePipelineRun(ctx, "ci", "demo-run")
	if err != nil || !strings.Contains(runDesc, "Status: success") {
		t.Fatalf("DescribePipelineRun() content = %q err = %v", runDesc, err)
	}
	pipelines, err := client.ListPipelines(ctx, "ci", "demo")
	if err != nil || len(pipelines) != 1 {
		t.Fatalf("ListPipelines() items = %d err = %v", len(pipelines), err)
	}
	pipelineYaml, err := client.GetPipelineYaml(ctx, "ci", "demo-pipeline")
	if err != nil || !strings.Contains(pipelineYaml, "kind: Pipeline") {
		t.Fatalf("GetPipelineYaml() content = %q err = %v", pipelineYaml, err)
	}
	pipelineRuns, err := client.ListPipelineRuns(ctx, "ci", "demo")
	if err != nil || len(pipelineRuns) != 1 {
		t.Fatalf("ListPipelineRuns() items = %d err = %v", len(pipelineRuns), err)
	}
	pipelineRunYaml, err := client.GetPipelineRunYaml(ctx, "ci", "demo-run")
	if err != nil || !strings.Contains(pipelineRunYaml, "kind: PipelineRun") {
		t.Fatalf("GetPipelineRunYaml() content = %q err = %v", pipelineRunYaml, err)
	}
	if err := client.CancelPipelineRun(ctx, "ci", "demo-run"); err != nil {
		t.Fatalf("CancelPipelineRun() error = %v", err)
	}
	cancelled, err := client.getDynamicResource(ctx, pipelineRunGVRs, "ci", "demo-run", "Tekton PipelineRun")
	if err != nil {
		t.Fatalf("get cancelled PipelineRun error = %v", err)
	}
	status, _, _ := unstructured.NestedString(cancelled.Object, "spec", "status")
	if status != "CancelledRunFinally" {
		t.Fatalf("PipelineRun spec.status = %q, want CancelledRunFinally", status)
	}
	if err := client.DeletePod(ctx, "ci", "demo-pod"); err != nil {
		t.Fatalf("DeletePod() error = %v", err)
	}
	if err := client.DeletePipeline(ctx, "ci", "demo-pipeline"); err != nil {
		t.Fatalf("DeletePipeline() error = %v", err)
	}
	if err := client.DeletePipelineRun(ctx, "ci", "demo-run"); err != nil {
		t.Fatalf("DeletePipelineRun() error = %v", err)
	}
}

func TestPipelineRunLogFallback(t *testing.T) {
	ctx := context.Background()
	client := &Client{
		kubeClient: kubernetesfake.NewSimpleClientset(),
		dynamicClient: dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), map[schema.GroupVersionResource]string{
			{Group: "tekton.dev", Version: "v1", Resource: "taskruns"}: "TaskRunList",
		}),
	}

	content, containers, err := client.GetPipelineRunLog(ctx, "ci", "demo-run", "", 500)
	if err != nil {
		t.Fatalf("GetPipelineRunLog() error = %v", err)
	}
	if len(containers) != 0 {
		t.Fatalf("containers = %#v, want empty", containers)
	}
	if !strings.Contains(content, "未找到可用的 TaskRun Pod 日志") {
		t.Fatalf("content = %q", content)
	}
}

func TestDefaultPodLogContainerPrefersRegularContainer(t *testing.T) {
	pod := &corev1.Pod{
		Spec: corev1.PodSpec{
			InitContainers: []corev1.Container{{Name: "init"}},
			Containers:     []corev1.Container{{Name: "app"}},
		},
	}
	if got := defaultPodLogContainer(pod); got != "app" {
		t.Fatalf("defaultPodLogContainer() = %q, want app", got)
	}
}

func TestApplyManagedPrunerConfigMapMergesPipelineGroups(t *testing.T) {
	ctx := context.Background()
	client := &Client{kubeClient: kubernetesfake.NewSimpleClientset()}
	first := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tekton-pruner-namespace-spec",
			Namespace: "ci",
			Labels: map[string]string{
				"kube-nova.io/managed-by": "kube-nova",
			},
		},
		Data: map[string]string{"ns-config": `
pipelineRuns:
- selector:
  - matchLabels:
      kube-nova.io/pipeline-id: pipeline-a
      tekton.dev/pipeline: app-a
`},
	}
	second := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tekton-pruner-namespace-spec",
			Namespace: "ci",
			Labels: map[string]string{
				"kube-nova.io/managed-by": "kube-nova",
			},
		},
		Data: map[string]string{"ns-config": `
pipelineRuns:
- selector:
  - matchLabels:
      kube-nova.io/pipeline-id: pipeline-b
      tekton.dev/pipeline: app-b
`},
	}

	if err := client.ApplyManagedPrunerConfigMap(ctx, first, "kube-nova.io/pipeline-id", "pipeline-a"); err != nil {
		t.Fatalf("ApplyManagedPrunerConfigMap(first) error = %v", err)
	}
	if err := client.ApplyManagedPrunerConfigMap(ctx, second, "kube-nova.io/pipeline-id", "pipeline-b"); err != nil {
		t.Fatalf("ApplyManagedPrunerConfigMap(second) error = %v", err)
	}
	got, err := client.kubeClient.CoreV1().ConfigMaps("ci").Get(ctx, "tekton-pruner-namespace-spec", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Get ConfigMap error = %v", err)
	}
	content := got.Data["ns-config"]
	for _, want := range []string{"pipeline-a", "pipeline-b", "app-a", "app-b"} {
		if !strings.Contains(content, want) {
			t.Fatalf("ns-config missing %q:\n%s", want, content)
		}
	}

	if err := client.DeleteManagedPrunerConfigMap(ctx, "ci", "tekton-pruner-namespace-spec", "ns-config", "kube-nova.io/pipeline-id", "pipeline-a"); err != nil {
		t.Fatalf("DeleteManagedPrunerConfigMap(pipeline-a) error = %v", err)
	}
	got, err = client.kubeClient.CoreV1().ConfigMaps("ci").Get(ctx, "tekton-pruner-namespace-spec", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Get ConfigMap after partial delete error = %v", err)
	}
	content = got.Data["ns-config"]
	if strings.Contains(content, "pipeline-a") || !strings.Contains(content, "pipeline-b") {
		t.Fatalf("partial delete content mismatch:\n%s", content)
	}
	if err := client.DeleteManagedPrunerConfigMap(ctx, "ci", "tekton-pruner-namespace-spec", "ns-config", "kube-nova.io/pipeline-id", "pipeline-b"); err != nil {
		t.Fatalf("DeleteManagedPrunerConfigMap(pipeline-b) error = %v", err)
	}
	if _, err := client.kubeClient.CoreV1().ConfigMaps("ci").Get(ctx, "tekton-pruner-namespace-spec", metav1.GetOptions{}); err == nil {
		t.Fatalf("ConfigMap should be deleted when last managed group is removed")
	}
}
