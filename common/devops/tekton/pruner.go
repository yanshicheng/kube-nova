package tekton

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/yaml"
)

type PrunerResourceInfo struct {
	Name        string
	Namespace   string
	Labels      map[string]string
	Annotations map[string]string
	Params      map[string]string
	Workspaces  []string
	Status      RunStatus
	CreatedAt   time.Time
	FinishedAt  time.Time
	Yaml        string
}

func (c *Client) ApplyPrunerConfigMap(ctx context.Context, cm *corev1.ConfigMap) error {
	if cm == nil {
		return fmt.Errorf("Tekton Pruner ConfigMap 不能为空")
	}
	namespace := strings.TrimSpace(cm.Namespace)
	name := strings.TrimSpace(cm.Name)
	if namespace == "" || name == "" {
		return fmt.Errorf("Tekton Pruner ConfigMap 参数不完整")
	}
	existing, err := c.kubeClient.CoreV1().ConfigMaps(namespace).Get(ctx, name, metav1.GetOptions{})
	if err == nil {
		cm.ResourceVersion = existing.ResourceVersion
		if _, err := c.kubeClient.CoreV1().ConfigMaps(namespace).Update(ctx, cm, metav1.UpdateOptions{}); err != nil {
			logx.Errorf("更新 Tekton Pruner ConfigMap 失败: namespace=%s name=%s err=%v", namespace, name, err)
			return err
		}
		return nil
	}
	if !apierrors.IsNotFound(err) {
		logx.Errorf("查询 Tekton Pruner ConfigMap 失败: namespace=%s name=%s err=%v", namespace, name, err)
		return err
	}
	if _, err := c.kubeClient.CoreV1().ConfigMaps(namespace).Create(ctx, cm, metav1.CreateOptions{}); err != nil {
		logx.Errorf("创建 Tekton Pruner ConfigMap 失败: namespace=%s name=%s err=%v", namespace, name, err)
		return err
	}
	return nil
}

func (c *Client) DeletePrunerConfigMap(ctx context.Context, namespace, name string) error {
	namespace = strings.TrimSpace(namespace)
	name = strings.TrimSpace(name)
	if namespace == "" || name == "" {
		return fmt.Errorf("Tekton Pruner ConfigMap 删除参数不完整")
	}
	if err := c.kubeClient.CoreV1().ConfigMaps(namespace).Delete(ctx, name, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
		logx.Errorf("删除 Tekton Pruner ConfigMap 失败: namespace=%s name=%s err=%v", namespace, name, err)
		return err
	}
	return nil
}

func (c *Client) ApplyManagedPrunerConfigMap(ctx context.Context, cm *corev1.ConfigMap, ownerLabelKey, ownerLabelValue string) error {
	if cm == nil {
		return fmt.Errorf("Tekton Pruner ConfigMap 不能为空")
	}
	if strings.TrimSpace(ownerLabelKey) == "" || strings.TrimSpace(ownerLabelValue) == "" {
		return fmt.Errorf("Tekton Pruner ConfigMap owner 不完整")
	}
	namespace := strings.TrimSpace(cm.Namespace)
	name := strings.TrimSpace(cm.Name)
	dataKey := prunerConfigMapDataKey(cm)
	if namespace == "" || name == "" || dataKey == "" {
		return fmt.Errorf("Tekton Pruner ConfigMap 参数不完整")
	}
	existing, err := c.kubeClient.CoreV1().ConfigMaps(namespace).Get(ctx, name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		if _, err := c.kubeClient.CoreV1().ConfigMaps(namespace).Create(ctx, cm, metav1.CreateOptions{}); err != nil {
			logx.Errorf("创建 Tekton Pruner ConfigMap 失败: namespace=%s name=%s err=%v", namespace, name, err)
			return err
		}
		return nil
	}
	if err != nil {
		logx.Errorf("查询 Tekton Pruner ConfigMap 失败: namespace=%s name=%s err=%v", namespace, name, err)
		return err
	}
	merged, err := mergeManagedPrunerConfigMap(existing.DeepCopy(), cm, dataKey, ownerLabelKey, ownerLabelValue)
	if err != nil {
		return err
	}
	if _, err := c.kubeClient.CoreV1().ConfigMaps(namespace).Update(ctx, merged, metav1.UpdateOptions{}); err != nil {
		logx.Errorf("更新 Tekton Pruner ConfigMap 失败: namespace=%s name=%s err=%v", namespace, name, err)
		return err
	}
	return nil
}

func (c *Client) DeleteManagedPrunerConfigMap(ctx context.Context, namespace, name, dataKey, ownerLabelKey, ownerLabelValue string) error {
	namespace = strings.TrimSpace(namespace)
	name = strings.TrimSpace(name)
	dataKey = strings.TrimSpace(dataKey)
	if namespace == "" || name == "" || dataKey == "" {
		return fmt.Errorf("Tekton Pruner ConfigMap 删除参数不完整")
	}
	if strings.TrimSpace(ownerLabelKey) == "" || strings.TrimSpace(ownerLabelValue) == "" {
		return fmt.Errorf("Tekton Pruner ConfigMap owner 不完整")
	}
	existing, err := c.kubeClient.CoreV1().ConfigMaps(namespace).Get(ctx, name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		logx.Errorf("查询 Tekton Pruner ConfigMap 失败: namespace=%s name=%s err=%v", namespace, name, err)
		return err
	}
	updated, empty, err := removeManagedPrunerConfigMapOwner(existing.DeepCopy(), dataKey, ownerLabelKey, ownerLabelValue)
	if err != nil {
		return err
	}
	if empty {
		return c.DeletePrunerConfigMap(ctx, namespace, name)
	}
	if _, err := c.kubeClient.CoreV1().ConfigMaps(namespace).Update(ctx, updated, metav1.UpdateOptions{}); err != nil {
		logx.Errorf("更新 Tekton Pruner ConfigMap 失败: namespace=%s name=%s err=%v", namespace, name, err)
		return err
	}
	return nil
}

func prunerConfigMapDataKey(cm *corev1.ConfigMap) string {
	if cm == nil {
		return ""
	}
	if _, ok := cm.Data["global-config"]; ok {
		return "global-config"
	}
	if _, ok := cm.Data["ns-config"]; ok {
		return "ns-config"
	}
	if len(cm.Data) == 1 {
		for key := range cm.Data {
			return key
		}
	}
	return ""
}

func mergeManagedPrunerConfigMap(existing, desired *corev1.ConfigMap, dataKey, ownerLabelKey, ownerLabelValue string) (*corev1.ConfigMap, error) {
	existingSpec, err := parsePrunerConfigMapSpec(existing.Data[dataKey])
	if err != nil {
		return nil, err
	}
	desiredSpec, err := parsePrunerConfigMapSpec(desired.Data[dataKey])
	if err != nil {
		return nil, err
	}
	if existing.Labels == nil {
		existing.Labels = map[string]string{}
	}
	for key, value := range desired.Labels {
		existing.Labels[key] = value
	}
	if existing.Data == nil {
		existing.Data = map[string]string{}
	}
	for key, value := range desiredSpec {
		if key != "pipelineRuns" && key != "taskRuns" {
			existingSpec[key] = value
		}
	}
	mergeManagedPrunerGroups(existingSpec, desiredSpec, ownerLabelKey, ownerLabelValue)
	raw, err := yaml.Marshal(existingSpec)
	if err != nil {
		return nil, err
	}
	existing.Data[dataKey] = string(raw)
	return existing, nil
}

func removeManagedPrunerConfigMapOwner(cm *corev1.ConfigMap, dataKey, ownerLabelKey, ownerLabelValue string) (*corev1.ConfigMap, bool, error) {
	spec, err := parsePrunerConfigMapSpec(cm.Data[dataKey])
	if err != nil {
		return nil, false, err
	}
	changed := false
	for _, resourceKey := range []string{"pipelineRuns", "taskRuns"} {
		groups := prunerConfigMapGroups(spec[resourceKey])
		next := removeOwnedPrunerGroups(groups, ownerLabelKey, ownerLabelValue)
		if len(next) != len(groups) {
			changed = true
		}
		if len(next) == 0 {
			delete(spec, resourceKey)
		} else {
			spec[resourceKey] = next
		}
	}
	if !changed {
		return cm, false, nil
	}
	if !hasPrunerConfigMapGroups(spec) && cm.Labels["kube-nova.io/managed-by"] == "kube-nova" {
		return cm, true, nil
	}
	raw, err := yaml.Marshal(spec)
	if err != nil {
		return nil, false, err
	}
	if cm.Data == nil {
		cm.Data = map[string]string{}
	}
	cm.Data[dataKey] = string(raw)
	return cm, false, nil
}

func hasPrunerConfigMapGroups(spec map[string]any) bool {
	return len(prunerConfigMapGroups(spec["pipelineRuns"])) > 0 || len(prunerConfigMapGroups(spec["taskRuns"])) > 0
}

func parsePrunerConfigMapSpec(content string) (map[string]any, error) {
	spec := map[string]any{}
	if strings.TrimSpace(content) == "" {
		return spec, nil
	}
	if err := yaml.Unmarshal([]byte(content), &spec); err != nil {
		return nil, err
	}
	if spec == nil {
		spec = map[string]any{}
	}
	return spec, nil
}

func mergeManagedPrunerGroups(existingSpec, desiredSpec map[string]any, ownerLabelKey, ownerLabelValue string) {
	for _, resourceKey := range []string{"pipelineRuns", "taskRuns"} {
		existingGroups := removeOwnedPrunerGroups(prunerConfigMapGroups(existingSpec[resourceKey]), ownerLabelKey, ownerLabelValue)
		desiredGroups := prunerConfigMapGroups(desiredSpec[resourceKey])
		next := append(existingGroups, desiredGroups...)
		if len(next) == 0 {
			delete(existingSpec, resourceKey)
		} else {
			existingSpec[resourceKey] = next
		}
	}
}

func prunerConfigMapGroups(value any) []any {
	switch typed := value.(type) {
	case []any:
		return typed
	case []map[string]any:
		result := make([]any, 0, len(typed))
		for _, item := range typed {
			result = append(result, item)
		}
		return result
	default:
		return nil
	}
}

func removeOwnedPrunerGroups(groups []any, ownerLabelKey, ownerLabelValue string) []any {
	result := make([]any, 0, len(groups))
	for _, group := range groups {
		if prunerGroupHasOwner(group, ownerLabelKey, ownerLabelValue) {
			continue
		}
		result = append(result, group)
	}
	return result
}

func prunerGroupHasOwner(group any, ownerLabelKey, ownerLabelValue string) bool {
	groupMap, ok := group.(map[string]any)
	if !ok {
		return false
	}
	for _, selector := range prunerConfigMapGroups(groupMap["selector"]) {
		selectorMap, ok := selector.(map[string]any)
		if !ok {
			continue
		}
		labels, ok := selectorMap["matchLabels"].(map[string]any)
		if !ok {
			continue
		}
		if strings.TrimSpace(fmt.Sprint(labels[ownerLabelKey])) == ownerLabelValue {
			return true
		}
	}
	return false
}

func (c *Client) ListPipelineRunResources(ctx context.Context, namespace, labelSelector string) ([]PrunerResourceInfo, error) {
	return c.listPrunerResources(ctx, namespace, pipelineRunGVRs, labelSelector, "PipelineRun")
}

func (c *Client) ListTaskRunResources(ctx context.Context, namespace, pipelineRunName, labelSelector string) ([]PrunerResourceInfo, error) {
	pipelineRunName = strings.TrimSpace(pipelineRunName)
	if pipelineRunName != "" {
		selector := "tekton.dev/pipelineRun=" + pipelineRunName
		if strings.TrimSpace(labelSelector) != "" {
			selector += "," + strings.TrimSpace(labelSelector)
		}
		labelSelector = selector
	}
	return c.listPrunerResources(ctx, namespace, taskRunGVRs, labelSelector, "TaskRun")
}

func (c *Client) DeleteTaskRun(ctx context.Context, namespace, name string) error {
	namespace = strings.TrimSpace(namespace)
	name = strings.TrimSpace(name)
	if namespace == "" || name == "" {
		return fmt.Errorf("Tekton TaskRun 参数不完整")
	}
	var lastErr error
	for _, gvr := range taskRunGVRs {
		err := c.dynamicClient.Resource(gvr).Namespace(namespace).Delete(ctx, name, metav1.DeleteOptions{})
		if err == nil || apierrors.IsNotFound(err) {
			return nil
		}
		if meta.IsNoMatchError(err) {
			continue
		}
		lastErr = err
	}
	if lastErr != nil {
		logx.Errorf("删除 Tekton TaskRun 失败: namespace=%s name=%s err=%v", namespace, name, lastErr)
		return lastErr
	}
	return fmt.Errorf("Tekton TaskRun 不存在")
}

func (c *Client) listPrunerResources(ctx context.Context, namespace string, gvrs []schema.GroupVersionResource, labelSelector, kind string) ([]PrunerResourceInfo, error) {
	namespace = strings.TrimSpace(namespace)
	if namespace == "" {
		return nil, fmt.Errorf("Tekton namespace 不能为空")
	}
	var lastErr error
	for _, gvr := range gvrs {
		list, err := c.dynamicClient.Resource(gvr).Namespace(namespace).List(ctx, metav1.ListOptions{LabelSelector: strings.TrimSpace(labelSelector)})
		if err != nil {
			if apierrors.IsNotFound(err) || meta.IsNoMatchError(err) {
				continue
			}
			lastErr = err
			continue
		}
		items := make([]PrunerResourceInfo, 0, len(list.Items))
		for index := range list.Items {
			items = append(items, prunerResourceInfoFromObject(&list.Items[index]))
		}
		sort.SliceStable(items, func(i, j int) bool {
			return items[i].CreatedAt.After(items[j].CreatedAt)
		})
		return items, nil
	}
	if lastErr != nil {
		logx.Errorf("查询 Tekton %s 列表失败: namespace=%s selector=%s err=%v", kind, namespace, labelSelector, lastErr)
		return nil, lastErr
	}
	return nil, fmt.Errorf("未找到可用的 Tekton %s API", kind)
}

func prunerResourceInfoFromObject(obj *unstructured.Unstructured) PrunerResourceInfo {
	status := statusFromObject(obj)
	return PrunerResourceInfo{
		Name:        obj.GetName(),
		Namespace:   obj.GetNamespace(),
		Labels:      obj.GetLabels(),
		Annotations: obj.GetAnnotations(),
		Params:      pipelineRunParamsFromObject(obj),
		Workspaces:  pipelineRunWorkspacesFromObject(obj),
		Status:      status,
		CreatedAt:   obj.GetCreationTimestamp().Time,
		FinishedAt:  status.FinishedAt,
		Yaml:        tektonResourceYaml(obj),
	}
}

func tektonResourceYaml(obj *unstructured.Unstructured) string {
	if obj == nil {
		return ""
	}
	data, err := yaml.Marshal(obj.Object)
	if err != nil {
		return ""
	}
	return string(data)
}

func pipelineRunParamsFromObject(obj *unstructured.Unstructured) map[string]string {
	params, ok, err := unstructured.NestedSlice(obj.Object, "spec", "params")
	if err != nil || !ok {
		return nil
	}
	result := make(map[string]string, len(params))
	for _, item := range params {
		param, ok := item.(map[string]any)
		if !ok {
			continue
		}
		name := strings.TrimSpace(fmt.Sprint(param["name"]))
		if name == "" {
			continue
		}
		result[name] = pipelineRunParamValueString(param["value"])
	}
	return result
}

func pipelineRunParamValueString(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	default:
		bytes, err := json.Marshal(typed)
		if err != nil {
			return strings.TrimSpace(fmt.Sprint(typed))
		}
		return string(bytes)
	}
}

func pipelineRunWorkspacesFromObject(obj *unstructured.Unstructured) []string {
	workspaces, ok, err := unstructured.NestedSlice(obj.Object, "spec", "workspaces")
	if err != nil || !ok {
		return nil
	}
	result := make([]string, 0, len(workspaces))
	for _, item := range workspaces {
		workspace, ok := item.(map[string]any)
		if !ok {
			continue
		}
		name := strings.TrimSpace(fmt.Sprint(workspace["name"]))
		if name != "" {
			result = append(result, name)
		}
	}
	return result
}
