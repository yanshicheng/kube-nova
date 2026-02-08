package workload

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	k8sTypes "github.com/yanshicheng/kube-nova/common/k8smanager/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateSchedulingConfigLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateSchedulingConfigLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateSchedulingConfigLogic {
	return &UpdateSchedulingConfigLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateSchedulingConfigLogic) UpdateSchedulingConfig(req *types.CommUpdateSchedulingConfigRequest) (resp string, err error) {
	client, versionDetail, err := getResourceClusterClient(l.ctx, l.svcCtx, req.Id)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return "", fmt.Errorf("获取集群客户端失败")
	}

	resourceType := strings.ToUpper(versionDetail.ResourceType)

	// 获取原调度配置（k8smanager 返回的是 *SchedulingConfig）
	var oldConfig *k8sTypes.SchedulingConfig
	switch resourceType {
	case "DEPLOYMENT":
		oldConfig, err = client.Deployment().GetSchedulingConfig(versionDetail.Namespace, versionDetail.ResourceName)
	case "STATEFULSET":
		oldConfig, err = client.StatefulSet().GetSchedulingConfig(versionDetail.Namespace, versionDetail.ResourceName)
	case "DAEMONSET":
		oldConfig, err = client.DaemonSet().GetSchedulingConfig(versionDetail.Namespace, versionDetail.ResourceName)
	case "JOB":
		oldConfig, err = client.Job().GetSchedulingConfig(versionDetail.Namespace, versionDetail.ResourceName)
	case "CRONJOB":
		oldConfig, err = client.CronJob().GetSchedulingConfig(versionDetail.Namespace, versionDetail.ResourceName)
	default:
		return "", fmt.Errorf("资源类型 %s 不支持修改调度配置", resourceType)
	}

	if err != nil {
		l.Errorf("获取原调度配置失败: %v", err)
		// 继续执行，oldConfig 可能为 nil
	}

	// 将 API 请求转换为 k8smanager 使用的 UpdateSchedulingConfigRequest
	updateReq := convertCommUpdateSchedulingConfigToK8sRequest(req)

	// 执行更新
	switch resourceType {
	case "DEPLOYMENT":
		err = client.Deployment().UpdateSchedulingConfig(versionDetail.Namespace, versionDetail.ResourceName, updateReq)
	case "STATEFULSET":
		err = client.StatefulSet().UpdateSchedulingConfig(versionDetail.Namespace, versionDetail.ResourceName, updateReq)
	case "DAEMONSET":
		err = client.DaemonSet().UpdateSchedulingConfig(versionDetail.Namespace, versionDetail.ResourceName, updateReq)
	case "JOB":
		err = client.Job().UpdateSchedulingConfig(versionDetail.Namespace, versionDetail.ResourceName, updateReq)
	case "CRONJOB":
		err = client.CronJob().UpdateSchedulingConfig(versionDetail.Namespace, versionDetail.ResourceName, updateReq)
	}

	// 比较并生成变更详情
	changeDetail := compareSchedulingConfigChanges(oldConfig, req)

	// 如果没有变更，不记录日志
	if changeDetail == "" {
		if err != nil {
			l.Errorf("修改调度配置失败: %v", err)
			return "", fmt.Errorf("修改调度配置失败")
		}
		return "修改调度配置成功(无变更)", nil
	}

	if err != nil {
		l.Errorf("修改调度配置失败: %v", err)
		recordAuditLog(l.ctx, l.svcCtx, versionDetail, "修改调度配置",
			fmt.Sprintf("%s %s/%s 修改调度配置失败, %s, 错误: %v", resourceType, versionDetail.Namespace, versionDetail.ResourceName, changeDetail, err), 2)
		return "", fmt.Errorf("修改调度配置失败")
	}

	recordAuditLog(l.ctx, l.svcCtx, versionDetail, "修改调度配置",
		fmt.Sprintf("%s %s/%s 修改调度配置成功, %s", resourceType, versionDetail.Namespace, versionDetail.ResourceName, changeDetail), 1)
	return "修改调度配置成功", nil
}

// convertCommUpdateSchedulingConfigToK8sRequest 将 API 请求转换为 k8smanager 的 UpdateSchedulingConfigRequest
func convertCommUpdateSchedulingConfigToK8sRequest(req *types.CommUpdateSchedulingConfigRequest) *k8sTypes.UpdateSchedulingConfigRequest {
	if req == nil {
		return nil
	}

	result := &k8sTypes.UpdateSchedulingConfigRequest{
		NodeSelector: req.NodeSelector,
	}

	// 处理 NodeName（使用指针）
	if req.NodeName != "" {
		result.NodeName = &req.NodeName
	}

	// 转换 Affinity
	if req.Affinity != nil {
		result.Affinity = convertCommAffinityToK8sAffinityConfig(req.Affinity)
	}

	// 转换 Tolerations
	if len(req.Tolerations) > 0 {
		result.Tolerations = convertCommTolerationsToK8sTolerationConfigs(req.Tolerations)
	}

	// 转换 TopologySpreadConstraints
	if len(req.TopologySpreadConstraints) > 0 {
		result.TopologySpreadConstraints = convertCommTopologySpreadToK8sTopologySpreadConfigs(req.TopologySpreadConstraints)
	}

	return result
}

// convertCommAffinityToK8sAffinityConfig 转换亲和性配置
func convertCommAffinityToK8sAffinityConfig(affinity *types.CommAffinityConfig) *k8sTypes.AffinityConfig {
	if affinity == nil {
		return nil
	}

	result := &k8sTypes.AffinityConfig{}

	// 转换 NodeAffinity
	if affinity.NodeAffinity != nil {
		result.NodeAffinity = &k8sTypes.NodeAffinityConfig{}

		if affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution != nil {
			result.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution = convertCommNodeSelectorToK8sNodeSelectorConfig(
				affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution,
			)
		}

		if len(affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution) > 0 {
			result.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution = convertCommPreferredSchedulingTermsToK8s(
				affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution,
			)
		}
	}

	// 转换 PodAffinity
	if affinity.PodAffinity != nil {
		result.PodAffinity = convertCommPodAffinityToK8sPodAffinityConfig(affinity.PodAffinity)
	}

	// 转换 PodAntiAffinity
	if affinity.PodAntiAffinity != nil {
		result.PodAntiAffinity = convertCommPodAntiAffinityToK8sPodAntiAffinityConfig(affinity.PodAntiAffinity)
	}

	return result
}

// convertCommNodeSelectorToK8sNodeSelectorConfig 转换节点选择器
func convertCommNodeSelectorToK8sNodeSelectorConfig(selector *types.CommNodeSelector) *k8sTypes.NodeSelectorConfig {
	if selector == nil {
		return nil
	}

	result := &k8sTypes.NodeSelectorConfig{
		NodeSelectorTerms: make([]k8sTypes.NodeSelectorTermConfig, 0, len(selector.NodeSelectorTerms)),
	}

	for _, term := range selector.NodeSelectorTerms {
		result.NodeSelectorTerms = append(result.NodeSelectorTerms, k8sTypes.NodeSelectorTermConfig{
			MatchExpressions: convertCommNodeSelectorRequirementsToK8s(term.MatchExpressions),
			MatchFields:      convertCommNodeSelectorRequirementsToK8s(term.MatchFields),
		})
	}

	return result
}

// convertCommNodeSelectorRequirementsToK8s 转换节点选择器要求
func convertCommNodeSelectorRequirementsToK8s(reqs []types.CommNodeSelectorRequirement) []k8sTypes.NodeSelectorRequirementConfig {
	result := make([]k8sTypes.NodeSelectorRequirementConfig, 0, len(reqs))
	for _, req := range reqs {
		result = append(result, k8sTypes.NodeSelectorRequirementConfig{
			Key:      req.Key,
			Operator: req.Operator,
			Values:   req.Values,
		})
	}
	return result
}

// convertCommPreferredSchedulingTermsToK8s 转换偏好调度条件
func convertCommPreferredSchedulingTermsToK8s(terms []types.CommPreferredSchedulingTerm) []k8sTypes.PreferredSchedulingTermConfig {
	result := make([]k8sTypes.PreferredSchedulingTermConfig, 0, len(terms))
	for _, term := range terms {
		result = append(result, k8sTypes.PreferredSchedulingTermConfig{
			Weight: term.Weight,
			Preference: k8sTypes.NodeSelectorTermConfig{
				MatchExpressions: convertCommNodeSelectorRequirementsToK8s(term.Preference.MatchExpressions),
				MatchFields:      convertCommNodeSelectorRequirementsToK8s(term.Preference.MatchFields),
			},
		})
	}
	return result
}

// convertCommPodAffinityToK8sPodAffinityConfig 转换 Pod 亲和性
func convertCommPodAffinityToK8sPodAffinityConfig(affinity *types.CommPodAffinity) *k8sTypes.PodAffinityConfig {
	if affinity == nil {
		return nil
	}

	result := &k8sTypes.PodAffinityConfig{}

	if len(affinity.RequiredDuringSchedulingIgnoredDuringExecution) > 0 {
		result.RequiredDuringSchedulingIgnoredDuringExecution = convertCommPodAffinityTermsToK8s(
			affinity.RequiredDuringSchedulingIgnoredDuringExecution,
		)
	}

	if len(affinity.PreferredDuringSchedulingIgnoredDuringExecution) > 0 {
		result.PreferredDuringSchedulingIgnoredDuringExecution = convertCommWeightedPodAffinityTermsToK8s(
			affinity.PreferredDuringSchedulingIgnoredDuringExecution,
		)
	}

	return result
}

// convertCommPodAntiAffinityToK8sPodAntiAffinityConfig 转换 Pod 反亲和性
func convertCommPodAntiAffinityToK8sPodAntiAffinityConfig(affinity *types.CommPodAntiAffinity) *k8sTypes.PodAntiAffinityConfig {
	if affinity == nil {
		return nil
	}

	result := &k8sTypes.PodAntiAffinityConfig{}

	if len(affinity.RequiredDuringSchedulingIgnoredDuringExecution) > 0 {
		result.RequiredDuringSchedulingIgnoredDuringExecution = convertCommPodAffinityTermsToK8s(
			affinity.RequiredDuringSchedulingIgnoredDuringExecution,
		)
	}

	if len(affinity.PreferredDuringSchedulingIgnoredDuringExecution) > 0 {
		result.PreferredDuringSchedulingIgnoredDuringExecution = convertCommWeightedPodAffinityTermsToK8s(
			affinity.PreferredDuringSchedulingIgnoredDuringExecution,
		)
	}

	return result
}

// convertCommPodAffinityTermsToK8s 转换 Pod 亲和性条件
func convertCommPodAffinityTermsToK8s(terms []types.CommPodAffinityTerm) []k8sTypes.PodAffinityTermConfig {
	result := make([]k8sTypes.PodAffinityTermConfig, 0, len(terms))
	for _, term := range terms {
		converted := k8sTypes.PodAffinityTermConfig{
			Namespaces:  term.Namespaces,
			TopologyKey: term.TopologyKey,
		}
		if term.LabelSelector != nil {
			converted.LabelSelector = convertCommLabelSelectorToK8sLabelSelectorConfig(term.LabelSelector)
		}
		if term.NamespaceSelector != nil {
			converted.NamespaceSelector = convertCommLabelSelectorToK8sLabelSelectorConfig(term.NamespaceSelector)
		}
		result = append(result, converted)
	}
	return result
}

// convertCommWeightedPodAffinityTermsToK8s 转换带权重的 Pod 亲和性条件
func convertCommWeightedPodAffinityTermsToK8s(terms []types.CommWeightedPodAffinityTerm) []k8sTypes.WeightedPodAffinityTermConfig {
	result := make([]k8sTypes.WeightedPodAffinityTermConfig, 0, len(terms))
	for _, term := range terms {
		converted := k8sTypes.WeightedPodAffinityTermConfig{
			Weight: term.Weight,
			PodAffinityTerm: k8sTypes.PodAffinityTermConfig{
				Namespaces:  term.PodAffinityTerm.Namespaces,
				TopologyKey: term.PodAffinityTerm.TopologyKey,
			},
		}
		if term.PodAffinityTerm.LabelSelector != nil {
			converted.PodAffinityTerm.LabelSelector = convertCommLabelSelectorToK8sLabelSelectorConfig(term.PodAffinityTerm.LabelSelector)
		}
		if term.PodAffinityTerm.NamespaceSelector != nil {
			converted.PodAffinityTerm.NamespaceSelector = convertCommLabelSelectorToK8sLabelSelectorConfig(term.PodAffinityTerm.NamespaceSelector)
		}
		result = append(result, converted)
	}
	return result
}

// convertCommLabelSelectorToK8sLabelSelectorConfig 转换标签选择器
func convertCommLabelSelectorToK8sLabelSelectorConfig(selector *types.CommLabelSelectorConfig) *k8sTypes.LabelSelectorConfig {
	if selector == nil {
		return nil
	}

	result := &k8sTypes.LabelSelectorConfig{
		MatchLabels: selector.MatchLabels,
	}

	if len(selector.MatchExpressions) > 0 {
		result.MatchExpressions = make([]k8sTypes.LabelSelectorRequirementConfig, 0, len(selector.MatchExpressions))
		for _, expr := range selector.MatchExpressions {
			result.MatchExpressions = append(result.MatchExpressions, k8sTypes.LabelSelectorRequirementConfig{
				Key:      expr.Key,
				Operator: expr.Operator,
				Values:   expr.Values,
			})
		}
	}

	return result
}

// convertCommTolerationsToK8sTolerationConfigs 转换容忍配置
func convertCommTolerationsToK8sTolerationConfigs(tolerations []types.CommToleration) []k8sTypes.TolerationConfig {
	result := make([]k8sTypes.TolerationConfig, 0, len(tolerations))
	for _, t := range tolerations {
		toleration := k8sTypes.TolerationConfig{
			Key:      t.Key,
			Operator: t.Operator,
			Value:    t.Value,
			Effect:   t.Effect,
		}
		if t.TolerationSeconds != 0 {
			tolerationSeconds := t.TolerationSeconds
			toleration.TolerationSeconds = &tolerationSeconds
		}
		result = append(result, toleration)
	}
	return result
}

// convertCommTopologySpreadToK8sTopologySpreadConfigs 转换拓扑分布约束
func convertCommTopologySpreadToK8sTopologySpreadConfigs(constraints []types.CommTopologySpreadConstraint) []k8sTypes.TopologySpreadConstraintConfig {
	result := make([]k8sTypes.TopologySpreadConstraintConfig, 0, len(constraints))
	for _, c := range constraints {
		converted := k8sTypes.TopologySpreadConstraintConfig{
			MaxSkew:           c.MaxSkew,
			TopologyKey:       c.TopologyKey,
			WhenUnsatisfiable: c.WhenUnsatisfiable,
		}

		if c.MinDomains != 0 {
			minDomains := c.MinDomains
			converted.MinDomains = &minDomains
		}

		if c.NodeAffinityPolicy != "" {
			nodeAffinityPolicy := c.NodeAffinityPolicy
			converted.NodeAffinityPolicy = &nodeAffinityPolicy
		}

		if c.NodeTaintsPolicy != "" {
			nodeTaintsPolicy := c.NodeTaintsPolicy
			converted.NodeTaintsPolicy = &nodeTaintsPolicy
		}

		if c.LabelSelector != nil {
			converted.LabelSelector = convertCommLabelSelectorToK8sLabelSelectorConfig(c.LabelSelector)
		}

		result = append(result, converted)
	}
	return result
}

// ==================== 变更比较函数 ====================

// compareSchedulingConfigChanges 比较调度配置变更，返回变更详情
// 如果没有变更返回空字符串
func compareSchedulingConfigChanges(oldConfig *k8sTypes.SchedulingConfig, newReq *types.CommUpdateSchedulingConfigRequest) string {
	if newReq == nil {
		return ""
	}

	var changes []string

	// 比较 NodeSelector
	oldNodeSelector := make(map[string]string)
	if oldConfig != nil && oldConfig.NodeSelector != nil {
		oldNodeSelector = oldConfig.NodeSelector
	}
	newNodeSelector := make(map[string]string)
	if newReq.NodeSelector != nil {
		newNodeSelector = newReq.NodeSelector
	}
	if !reflect.DeepEqual(oldNodeSelector, newNodeSelector) {
		changes = append(changes, fmt.Sprintf("NodeSelector: %s -> %s", formatMapForLog(oldNodeSelector), formatMapForLog(newNodeSelector)))
	}

	// 比较 NodeName
	oldNodeName := ""
	if oldConfig != nil {
		oldNodeName = oldConfig.NodeName
	}
	if oldNodeName != newReq.NodeName {
		changes = append(changes, fmt.Sprintf("NodeName: %s -> %s", defaultIfEmptyStr(oldNodeName, "未设置"), defaultIfEmptyStr(newReq.NodeName, "未设置")))
	}

	// 比较 Tolerations
	oldTolerationCount := 0
	if oldConfig != nil && oldConfig.Tolerations != nil {
		oldTolerationCount = len(oldConfig.Tolerations)
	}
	newTolerationCount := len(newReq.Tolerations)
	if oldTolerationCount != newTolerationCount || !compareTolerationsEqual(oldConfig, newReq) {
		oldTolerationDesc := describeOldTolerations(oldConfig)
		newTolerationDesc := describeNewTolerations(newReq.Tolerations)
		changes = append(changes, fmt.Sprintf("Tolerations: %s -> %s", oldTolerationDesc, newTolerationDesc))
	}

	// 比较 Affinity
	oldHasAffinity := oldConfig != nil && oldConfig.Affinity != nil
	newHasAffinity := newReq.Affinity != nil
	if oldHasAffinity != newHasAffinity || (oldHasAffinity && newHasAffinity && !compareAffinityEqual(oldConfig.Affinity, newReq.Affinity)) {
		oldAffinityDesc := "未配置"
		if oldHasAffinity {
			oldAffinityDesc = describeOldAffinity(oldConfig.Affinity)
		}
		newAffinityDesc := "未配置"
		if newHasAffinity {
			newAffinityDesc = describeNewAffinity(newReq.Affinity)
		}
		changes = append(changes, fmt.Sprintf("Affinity: %s -> %s", oldAffinityDesc, newAffinityDesc))
	}

	// 比较 TopologySpreadConstraints
	oldTopologyCount := 0
	if oldConfig != nil && oldConfig.TopologySpreadConstraints != nil {
		oldTopologyCount = len(oldConfig.TopologySpreadConstraints)
	}
	newTopologyCount := len(newReq.TopologySpreadConstraints)
	if oldTopologyCount != newTopologyCount || !compareTopologySpreadEqual(oldConfig, newReq) {
		changes = append(changes, fmt.Sprintf("TopologySpreadConstraints: %d条 -> %d条", oldTopologyCount, newTopologyCount))
	}

	if len(changes) == 0 {
		return ""
	}

	return "调度配置变更: " + strings.Join(changes, "; ")
}

// formatMapForLog 格式化 map 用于日志输出
func formatMapForLog(m map[string]string) string {
	if len(m) == 0 {
		return "未设置"
	}
	var parts []string
	for k, v := range m {
		parts = append(parts, fmt.Sprintf("%s=%s", k, v))
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

// defaultIfEmptyStr 如果字符串为空返回默认值
func defaultIfEmptyStr(s, defaultVal string) string {
	if s == "" {
		return defaultVal
	}
	return s
}

// compareTolerationsEqual 比较容忍配置是否相同
func compareTolerationsEqual(oldConfig *k8sTypes.SchedulingConfig, newReq *types.CommUpdateSchedulingConfigRequest) bool {
	if oldConfig == nil || oldConfig.Tolerations == nil {
		return len(newReq.Tolerations) == 0
	}
	if len(oldConfig.Tolerations) != len(newReq.Tolerations) {
		return false
	}

	for i, oldT := range oldConfig.Tolerations {
		newT := newReq.Tolerations[i]
		if oldT.Key != newT.Key || oldT.Operator != newT.Operator ||
			oldT.Value != newT.Value || oldT.Effect != newT.Effect {
			return false
		}
		oldSeconds := int64(0)
		if oldT.TolerationSeconds != nil {
			oldSeconds = *oldT.TolerationSeconds
		}
		if oldSeconds != newT.TolerationSeconds {
			return false
		}
	}
	return true
}

// describeOldTolerations 描述旧的容忍配置
func describeOldTolerations(oldConfig *k8sTypes.SchedulingConfig) string {
	if oldConfig == nil || len(oldConfig.Tolerations) == 0 {
		return "无"
	}
	var parts []string
	for _, t := range oldConfig.Tolerations {
		parts = append(parts, fmt.Sprintf("%s:%s:%s", t.Key, t.Operator, t.Effect))
	}
	return fmt.Sprintf("[%s]", strings.Join(parts, ", "))
}

// describeNewTolerations 描述新的容忍配置
func describeNewTolerations(tolerations []types.CommToleration) string {
	if len(tolerations) == 0 {
		return "无"
	}
	var parts []string
	for _, t := range tolerations {
		parts = append(parts, fmt.Sprintf("%s:%s:%s", t.Key, t.Operator, t.Effect))
	}
	return fmt.Sprintf("[%s]", strings.Join(parts, ", "))
}

// compareAffinityEqual 比较亲和性配置是否相同
func compareAffinityEqual(old *k8sTypes.AffinityConfig, new *types.CommAffinityConfig) bool {
	if old == nil && new == nil {
		return true
	}
	if old == nil || new == nil {
		return false
	}

	// 比较 NodeAffinity
	oldHasNodeAffinity := old.NodeAffinity != nil
	newHasNodeAffinity := new.NodeAffinity != nil
	if oldHasNodeAffinity != newHasNodeAffinity {
		return false
	}

	// 比较 PodAffinity
	oldHasPodAffinity := old.PodAffinity != nil
	newHasPodAffinity := new.PodAffinity != nil
	if oldHasPodAffinity != newHasPodAffinity {
		return false
	}

	// 比较 PodAntiAffinity
	oldHasPodAntiAffinity := old.PodAntiAffinity != nil
	newHasPodAntiAffinity := new.PodAntiAffinity != nil
	if oldHasPodAntiAffinity != newHasPodAntiAffinity {
		return false
	}

	return true
}

// describeOldAffinity 描述旧的亲和性配置
func describeOldAffinity(affinity *k8sTypes.AffinityConfig) string {
	if affinity == nil {
		return "未配置"
	}
	var parts []string
	if affinity.NodeAffinity != nil {
		parts = append(parts, "NodeAffinity")
	}
	if affinity.PodAffinity != nil {
		parts = append(parts, "PodAffinity")
	}
	if affinity.PodAntiAffinity != nil {
		parts = append(parts, "PodAntiAffinity")
	}
	if len(parts) == 0 {
		return "未配置"
	}
	return strings.Join(parts, "+")
}

// describeNewAffinity 描述新的亲和性配置
func describeNewAffinity(affinity *types.CommAffinityConfig) string {
	if affinity == nil {
		return "未配置"
	}
	var parts []string
	if affinity.NodeAffinity != nil {
		parts = append(parts, "NodeAffinity")
	}
	if affinity.PodAffinity != nil {
		parts = append(parts, "PodAffinity")
	}
	if affinity.PodAntiAffinity != nil {
		parts = append(parts, "PodAntiAffinity")
	}
	if len(parts) == 0 {
		return "未配置"
	}
	return strings.Join(parts, "+")
}

// compareTopologySpreadEqual 比较拓扑分布约束是否相同
func compareTopologySpreadEqual(oldConfig *k8sTypes.SchedulingConfig, newReq *types.CommUpdateSchedulingConfigRequest) bool {
	if oldConfig == nil || oldConfig.TopologySpreadConstraints == nil {
		return len(newReq.TopologySpreadConstraints) == 0
	}
	if len(oldConfig.TopologySpreadConstraints) != len(newReq.TopologySpreadConstraints) {
		return false
	}

	for i, oldC := range oldConfig.TopologySpreadConstraints {
		newC := newReq.TopologySpreadConstraints[i]
		if oldC.MaxSkew != newC.MaxSkew || oldC.TopologyKey != newC.TopologyKey ||
			oldC.WhenUnsatisfiable != newC.WhenUnsatisfiable {
			return false
		}
	}
	return true
}
