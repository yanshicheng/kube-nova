package tekton

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/zeromicro/go-zero/core/logx"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/yaml"
)

const (
	tektonTriggersV1Beta1     = "triggers.tekton.dev/v1beta1"
	tektonTriggersV1Alpha1    = "triggers.tekton.dev/v1alpha1"
	tektonTriggerFieldManager = "kube-nova-tekton-trigger"
)

var triggerGVRs = []schema.GroupVersionResource{
	{Group: "triggers.tekton.dev", Version: "v1beta1", Resource: "triggers"},
	{Group: "triggers.tekton.dev", Version: "v1alpha1", Resource: "triggers"},
}

var triggerBindingGVRs = []schema.GroupVersionResource{
	{Group: "triggers.tekton.dev", Version: "v1beta1", Resource: "triggerbindings"},
	{Group: "triggers.tekton.dev", Version: "v1alpha1", Resource: "triggerbindings"},
}

var triggerTemplateGVRs = []schema.GroupVersionResource{
	{Group: "triggers.tekton.dev", Version: "v1beta1", Resource: "triggertemplates"},
	{Group: "triggers.tekton.dev", Version: "v1alpha1", Resource: "triggertemplates"},
}

var eventListenerGVRs = []schema.GroupVersionResource{
	{Group: "triggers.tekton.dev", Version: "v1beta1", Resource: "eventlisteners"},
	{Group: "triggers.tekton.dev", Version: "v1alpha1", Resource: "eventlisteners"},
}

var ingressGVRs = []schema.GroupVersionResource{
	{Group: "networking.k8s.io", Version: "v1", Resource: "ingresses"},
}

type TriggerResourceRef struct {
	APIVersion string
	Kind       string
	Name       string
}

func (c *Client) ApplyTriggerResource(ctx context.Context, namespace, content string) (StepResource, error) {
	namespace = strings.TrimSpace(namespace)
	if namespace == "" {
		return StepResource{}, fmt.Errorf("Tekton Trigger namespace 不能为空")
	}
	resource, err := parseTriggerResource(content)
	if err != nil {
		return StepResource{}, err
	}
	resource.Namespace = namespace
	resource.Object.SetNamespace(namespace)
	return c.applyTriggerResource(ctx, namespace, resource)
}

func (c *Client) ApplyTriggerResources(ctx context.Context, namespace string, contents []string) ([]StepResource, error) {
	resources := make([]StepResource, 0, len(contents))
	for _, content := range contents {
		if strings.TrimSpace(content) == "" {
			continue
		}
		resource, err := c.ApplyTriggerResource(ctx, namespace, content)
		if err != nil {
			return nil, err
		}
		resources = append(resources, resource)
	}
	return resources, nil
}

func (c *Client) ApplyTriggerYAML(ctx context.Context, namespace, content string) ([]StepResource, error) {
	docs := splitYAMLDocuments(content)
	if len(docs) == 0 {
		return nil, nil
	}
	return c.ApplyTriggerResources(ctx, namespace, docs)
}

func (c *Client) DeleteTriggerResource(ctx context.Context, namespace string, ref TriggerResourceRef) error {
	namespace = strings.TrimSpace(namespace)
	ref.APIVersion = strings.TrimSpace(ref.APIVersion)
	ref.Kind = strings.TrimSpace(ref.Kind)
	ref.Name = strings.TrimSpace(ref.Name)
	if namespace == "" || ref.Kind == "" || ref.Name == "" {
		return fmt.Errorf("Tekton Trigger 删除参数不完整")
	}
	var lastErr error
	for _, gvr := range triggerGVRsByKind(ref.Kind, ref.APIVersion) {
		err := c.dynamicClient.Resource(gvr).Namespace(namespace).Delete(ctx, ref.Name, metav1.DeleteOptions{})
		if err == nil || apierrors.IsNotFound(err) {
			return nil
		}
		if meta.IsNoMatchError(err) {
			continue
		}
		lastErr = err
	}
	if lastErr != nil {
		logx.Errorf("删除 Tekton Trigger 资源失败: kind=%s namespace=%s name=%s err=%v", ref.Kind, namespace, ref.Name, lastErr)
		return lastErr
	}
	return nil
}

func (c *Client) DeleteTriggerResources(ctx context.Context, namespace string, refs []TriggerResourceRef) error {
	for _, ref := range refs {
		if err := c.DeleteTriggerResource(ctx, namespace, ref); err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) applyTriggerResource(ctx context.Context, namespace string, resource StepResource) (StepResource, error) {
	data, err := json.Marshal(resource.Object.Object)
	if err != nil {
		return StepResource{}, fmt.Errorf("Tekton Trigger YAML 解析失败: %w", err)
	}
	patchOptions := metav1.PatchOptions{
		FieldManager: tektonTriggerFieldManager,
		Force:        boolPtr(true),
	}
	var lastErr error
	for _, gvr := range triggerGVRsByKind(resource.Kind, resource.APIVersion) {
		applied, err := c.dynamicClient.Resource(gvr).Namespace(namespace).Patch(ctx, resource.Name, types.ApplyPatchType, data, patchOptions)
		if err == nil {
			resource.Object = applied
			return resource, nil
		}
		if meta.IsNoMatchError(err) {
			continue
		}
		lastErr = err
	}
	if lastErr != nil {
		logx.Errorf("同步 Tekton Trigger 资源失败: kind=%s namespace=%s name=%s err=%v", resource.Kind, namespace, resource.Name, lastErr)
		return StepResource{}, lastErr
	}
	return StepResource{}, fmt.Errorf("未找到可用的 Tekton Trigger API: %s", resource.Kind)
}

func parseTriggerResource(content string) (StepResource, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return StepResource{}, fmt.Errorf("Tekton Trigger YAML 不能为空")
	}
	jsonData, err := yaml.YAMLToJSON([]byte(content))
	if err != nil {
		return StepResource{}, fmt.Errorf("Tekton Trigger YAML 解析失败: %w", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(jsonData, &raw); err != nil {
		return StepResource{}, fmt.Errorf("Tekton Trigger YAML 解析失败: %w", err)
	}
	obj := &unstructured.Unstructured{Object: raw}
	apiVersion := strings.TrimSpace(obj.GetAPIVersion())
	kind := strings.TrimSpace(obj.GetKind())
	name := strings.TrimSpace(obj.GetName())
	if len(triggerGVRsByKind(kind, apiVersion)) == 0 {
		return StepResource{}, fmt.Errorf("Tekton Trigger kind 不支持: %s", kind)
	}
	if name == "" {
		return StepResource{}, fmt.Errorf("Tekton Trigger metadata.name 不能为空")
	}
	return StepResource{
		APIVersion: apiVersion,
		Kind:       kind,
		Name:       name,
		Namespace:  obj.GetNamespace(),
		Hash:       contentHash(content),
		Object:     obj,
	}, nil
}

func triggerGVRsByKind(kind, apiVersion string) []schema.GroupVersionResource {
	var gvrs []schema.GroupVersionResource
	switch kind {
	case "EventListener":
		gvrs = eventListenerGVRs
	case "Trigger":
		gvrs = triggerGVRs
	case "TriggerBinding":
		gvrs = triggerBindingGVRs
	case "TriggerTemplate":
		gvrs = triggerTemplateGVRs
	case "Ingress":
		if apiVersion != "networking.k8s.io/v1" {
			return nil
		}
		return ingressGVRs
	default:
		return nil
	}
	if apiVersion != tektonTriggersV1Beta1 && apiVersion != tektonTriggersV1Alpha1 {
		return nil
	}
	if apiVersion == tektonTriggersV1Alpha1 && len(gvrs) > 1 {
		return []schema.GroupVersionResource{gvrs[1], gvrs[0]}
	}
	return gvrs
}

func splitYAMLDocuments(content string) []string {
	lines := bytes.Split([]byte(content), []byte("\n"))
	docs := make([]string, 0)
	current := bytes.Buffer{}
	for _, line := range lines {
		if strings.TrimSpace(string(line)) == "---" {
			if doc := strings.TrimSpace(current.String()); doc != "" {
				docs = append(docs, doc)
			}
			current.Reset()
			continue
		}
		current.Write(line)
		current.WriteByte('\n')
	}
	if doc := strings.TrimSpace(current.String()); doc != "" {
		docs = append(docs, doc)
	}
	return docs
}

func boolPtr(value bool) *bool {
	return &value
}
