package operator

import (
	"context"
	"strings"

	flaggerv1 "github.com/fluxcd/flagger/pkg/apis/flagger/v1beta1"
	"github.com/yanshicheng/kube-nova/common/k8smanager/cluster"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// FlaggerHelper Flagger 辅助工具
type FlaggerHelper struct {
	client       cluster.Client
	flaggerCRDOK bool                           // Flagger CRD 是否可用
	strategies   map[string]*CanaryStrategyInfo // key: targetKind/targetName, value: 策略信息
}

// NewFlaggerHelper 创建 Flagger 辅助工具
func NewFlaggerHelper(client cluster.Client) *FlaggerHelper {
	return &FlaggerHelper{
		client:       client,
		flaggerCRDOK: false,
		strategies:   make(map[string]*CanaryStrategyInfo),
	}
}

// CheckFlaggerCRD 检测集群是否安装了 Flagger CRD
func (h *FlaggerHelper) CheckFlaggerCRD(ctx context.Context) bool {
	if h.client == nil {
		return false
	}

	// 使用 Discovery API 检测 CRD 是否存在
	discoveryClient := h.client.GetClientSet().Discovery()

	// 获取 API 资源列表
	_, apiResourceList, err := discoveryClient.ServerGroupsAndResources()
	if err != nil {
		// 部分错误是可以接受的（某些 API 组可能不可用）
		if apiResourceList == nil {
			h.flaggerCRDOK = false
			return false
		}
	}

	// 查找 flagger.app/v1beta1 API 组
	for _, resourceList := range apiResourceList {
		if resourceList.GroupVersion == "flagger.app/v1beta1" {
			for _, resource := range resourceList.APIResources {
				if resource.Kind == "Canary" {
					h.flaggerCRDOK = true
					return true
				}
			}
		}
	}

	h.flaggerCRDOK = false
	return false
}

// IsFlaggerAvailable 返回 Flagger 是否可用
func (h *FlaggerHelper) IsFlaggerAvailable() bool {
	return h.flaggerCRDOK
}

// LoadNamespaceCanaries 加载指定命名空间的所有 Canary 资源并解析策略
func (h *FlaggerHelper) LoadNamespaceCanaries(ctx context.Context, namespace string) error {
	if !h.flaggerCRDOK {
		return nil
	}

	// 清空之前的策略缓存
	h.strategies = make(map[string]*CanaryStrategyInfo)

	// 获取 Flagger Operator
	flaggerOp := h.client.Flagger()
	if flaggerOp == nil {
		return nil
	}

	// 列出该命名空间的所有 Canary
	canaryList, err := flaggerOp.List(namespace, "", "")
	if err != nil {
		return err
	}

	if canaryList == nil || len(canaryList.Items) == 0 {
		return nil
	}

	// 解析每个 Canary 的策略类型
	for _, canaryInfo := range canaryList.Items {
		// 获取完整的 Canary 对象以分析策略
		canary, err := flaggerOp.Get(namespace, canaryInfo.Name)
		if err != nil || canary == nil {
			continue
		}

		strategyType := h.determineStrategyType(canary)

		// 构建策略信息
		strategy := &CanaryStrategyInfo{
			Name:         canary.Name,
			TargetName:   canary.Spec.TargetRef.Name,
			TargetKind:   canary.Spec.TargetRef.Kind,
			StrategyType: strategyType,
		}

		// 以 Kind/Name 作为 key 存储
		key := strings.ToLower(strategy.TargetKind) + "/" + strategy.TargetName
		h.strategies[key] = strategy
	}

	return nil
}

// determineStrategyType 根据 Canary spec 判断策略类型
// 参考 Flagger 官方文档: https://docs.flagger.app/usage/deployment-strategies
func (h *FlaggerHelper) determineStrategyType(canary *flaggerv1.Canary) string {
	if canary == nil || canary.Spec.Analysis == nil {
		return VersionRoleStable
	}

	analysis := canary.Spec.Analysis

	// 1. 检查是否是镜像策略 (Mirror)
	// mirror = true 或 mirrorWeight > 0
	if analysis.Mirror {
		return VersionRoleMirroring
	}
	if analysis.MirrorWeight > 0 {
		return VersionRoleMirroring
	}

	// 2. 检查是否是 A/B 测试策略
	// 有 match 规则（HTTP 头匹配）
	if len(analysis.Match) > 0 {
		return VersionRoleABTest
	}

	// 3. 检查是否是蓝绿策略 (Blue/Green)
	// iterations > 0 且没有 stepWeight（或 stepWeight = 0）
	if analysis.Iterations > 0 && analysis.StepWeight == 0 {
		return VersionRoleBlueGreen
	}

	// 4. 检查是否是金丝雀策略 (Canary)
	// 有 stepWeight 和 maxWeight
	if analysis.StepWeight > 0 || analysis.MaxWeight > 0 {
		return VersionRoleCanary
	}

	// 5. 如果有 iterations 但也有 stepWeight，仍然是金丝雀
	if analysis.Iterations > 0 && analysis.StepWeight > 0 {
		return VersionRoleCanary
	}

	// 默认返回金丝雀策略
	return VersionRoleCanary
}

// GetStrategyForTarget 获取指定目标资源的策略信息
func (h *FlaggerHelper) GetStrategyForTarget(kind, name string) *CanaryStrategyInfo {
	key := strings.ToLower(kind) + "/" + name
	return h.strategies[key]
}

// IdentifyResource 识别资源的 Flagger 信息
func (h *FlaggerHelper) IdentifyResource(
	resourceName string,
	resourceKind string,
	labels map[string]string,
	annotations map[string]string,
) *FlaggerResourceInfo {
	info := &FlaggerResourceInfo{
		IsFlaggerManaged: false,
		OriginalAppName:  resourceName,
		VersionRole:      VersionRoleStable,
		CanaryName:       "",
	}

	// 如果 Flagger 不可用，直接返回普通资源
	if !h.flaggerCRDOK {
		return info
	}

	// 方法1：通过 Label 识别 Flagger 管理的资源
	if labels != nil {
		if managedBy, ok := labels[FlaggerManagedByLabel]; ok && managedBy == "flagger" {
			info.IsFlaggerManaged = true
			if appName, ok := labels[FlaggerAppNameLabel]; ok && appName != "" {
				info.OriginalAppName = appName
			}
		}
	}

	// 方法2：通过命名规则推断
	originalName := resourceName
	isFlaggerSuffix := false

	if strings.HasSuffix(resourceName, "-primary") {
		originalName = strings.TrimSuffix(resourceName, "-primary")
		isFlaggerSuffix = true
	} else if strings.HasSuffix(resourceName, "-canary") {
		originalName = strings.TrimSuffix(resourceName, "-canary")
		isFlaggerSuffix = true
	}

	// 如果有 Flagger 后缀，查找对应的策略
	if isFlaggerSuffix {
		info.IsFlaggerManaged = true
		info.OriginalAppName = originalName

		// 查找对应的 Canary 策略
		strategy := h.GetStrategyForTarget(resourceKind, originalName)
		if strategy != nil {
			info.VersionRole = strategy.StrategyType
			info.CanaryName = strategy.Name
		} else {
			// 有后缀但找不到 Canary，默认金丝雀策略
			info.VersionRole = VersionRoleCanary
		}
		return info
	}

	// 方法3：检查是否是被 Canary targetRef 引用的原始资源
	strategy := h.GetStrategyForTarget(resourceKind, resourceName)
	if strategy != nil {
		// 这是被 Flagger 管理的原始资源，保持 stable
		info.IsFlaggerManaged = true
		info.CanaryName = strategy.Name
		info.VersionRole = VersionRoleStable
		return info
	}

	// 如果之前通过 Label 识别为 Flagger 管理，但没找到策略
	if info.IsFlaggerManaged {
		// 尝试通过原始名称查找策略
		strategy := h.GetStrategyForTarget(resourceKind, info.OriginalAppName)
		if strategy != nil {
			info.VersionRole = strategy.StrategyType
			info.CanaryName = strategy.Name
		}
	}

	return info
}

// LoadNamespaceCanariesByDynamicClient 使用 DynamicClient 加载 Canary（备用方法）
func (h *FlaggerHelper) LoadNamespaceCanariesByDynamicClient(ctx context.Context, namespace string) error {
	if !h.flaggerCRDOK {
		return nil
	}

	h.strategies = make(map[string]*CanaryStrategyInfo)

	dynamicClient := h.client.GetDynamicClient()
	if dynamicClient == nil {
		return nil
	}

	// 定义 Canary GVR
	canaryGVR := schema.GroupVersionResource{
		Group:    FlaggerCRDGroup,
		Version:  FlaggerCRDVersion,
		Resource: FlaggerCRDPlural,
	}

	// 列出 Canary 资源
	unstructuredList, err := dynamicClient.Resource(canaryGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, item := range unstructuredList.Items {
		// 解析基本信息
		name := item.GetName()

		// 获取 spec.targetRef
		spec, found, _ := unstructuredNestedMap(item.Object, "spec")
		if !found {
			continue
		}

		targetRef, found, _ := unstructuredNestedMap(spec, "targetRef")
		if !found {
			continue
		}

		targetName, _, _ := unstructuredNestedString(targetRef, "name")
		targetKind, _, _ := unstructuredNestedString(targetRef, "kind")

		if targetName == "" || targetKind == "" {
			continue
		}

		// 解析 analysis 来判断策略类型
		strategyType := h.determineStrategyTypeFromUnstructured(spec)

		strategy := &CanaryStrategyInfo{
			Name:         name,
			TargetName:   targetName,
			TargetKind:   targetKind,
			StrategyType: strategyType,
		}

		key := strings.ToLower(targetKind) + "/" + targetName
		h.strategies[key] = strategy
	}

	return nil
}

// determineStrategyTypeFromUnstructured 从非结构化数据判断策略类型
func (h *FlaggerHelper) determineStrategyTypeFromUnstructured(spec map[string]interface{}) string {
	analysis, found, _ := unstructuredNestedMap(spec, "analysis")
	if !found {
		return VersionRoleCanary
	}

	// 检查镜像策略
	mirror, _, _ := unstructuredNestedBool(analysis, "mirror")
	if mirror {
		return VersionRoleMirroring
	}

	mirrorWeight, _, _ := unstructuredNestedInt64(analysis, "mirrorWeight")
	if mirrorWeight > 0 {
		return VersionRoleMirroring
	}

	// 检查 A/B 测试
	match, found, _ := unstructuredNestedSlice(analysis, "match")
	if found && len(match) > 0 {
		return VersionRoleABTest
	}

	// 检查蓝绿策略
	iterations, _, _ := unstructuredNestedInt64(analysis, "iterations")
	stepWeight, _, _ := unstructuredNestedInt64(analysis, "stepWeight")

	if iterations > 0 && stepWeight == 0 {
		return VersionRoleBlueGreen
	}

	// 默认金丝雀
	return VersionRoleCanary
}

// 辅助函数：从 unstructured 中获取嵌套 map
func unstructuredNestedMap(obj map[string]interface{}, fields ...string) (map[string]interface{}, bool, error) {
	var current interface{} = obj
	for _, field := range fields {
		if m, ok := current.(map[string]interface{}); ok {
			if val, exists := m[field]; exists {
				current = val
			} else {
				return nil, false, nil
			}
		} else {
			return nil, false, nil
		}
	}
	if result, ok := current.(map[string]interface{}); ok {
		return result, true, nil
	}
	return nil, false, nil
}

// 辅助函数：从 unstructured 中获取嵌套 string
func unstructuredNestedString(obj map[string]interface{}, fields ...string) (string, bool, error) {
	var current interface{} = obj
	for _, field := range fields {
		if m, ok := current.(map[string]interface{}); ok {
			if val, exists := m[field]; exists {
				current = val
			} else {
				return "", false, nil
			}
		} else {
			return "", false, nil
		}
	}
	if result, ok := current.(string); ok {
		return result, true, nil
	}
	return "", false, nil
}

// 辅助函数：从 unstructured 中获取嵌套 bool
func unstructuredNestedBool(obj map[string]interface{}, fields ...string) (bool, bool, error) {
	var current interface{} = obj
	for _, field := range fields {
		if m, ok := current.(map[string]interface{}); ok {
			if val, exists := m[field]; exists {
				current = val
			} else {
				return false, false, nil
			}
		} else {
			return false, false, nil
		}
	}
	if result, ok := current.(bool); ok {
		return result, true, nil
	}
	return false, false, nil
}

// 辅助函数：从 unstructured 中获取嵌套 int64
func unstructuredNestedInt64(obj map[string]interface{}, fields ...string) (int64, bool, error) {
	var current interface{} = obj
	for _, field := range fields {
		if m, ok := current.(map[string]interface{}); ok {
			if val, exists := m[field]; exists {
				current = val
			} else {
				return 0, false, nil
			}
		} else {
			return 0, false, nil
		}
	}
	switch v := current.(type) {
	case int64:
		return v, true, nil
	case int32:
		return int64(v), true, nil
	case int:
		return int64(v), true, nil
	case float64:
		return int64(v), true, nil
	}
	return 0, false, nil
}

// 辅助函数：从 unstructured 中获取嵌套 slice
func unstructuredNestedSlice(obj map[string]interface{}, fields ...string) ([]interface{}, bool, error) {
	var current interface{} = obj
	for _, field := range fields {
		if m, ok := current.(map[string]interface{}); ok {
			if val, exists := m[field]; exists {
				current = val
			} else {
				return nil, false, nil
			}
		} else {
			return nil, false, nil
		}
	}
	if result, ok := current.([]interface{}); ok {
		return result, true, nil
	}
	return nil, false, nil
}
