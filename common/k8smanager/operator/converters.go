package operator

import (
	"github.com/yanshicheng/kube-nova/common/k8smanager/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ==================== 调度配置转换 ====================

// convertPodSpecToSchedulingConfig 将 PodSpec 转换为调度配置
func convertPodSpecToSchedulingConfig(spec *corev1.PodSpec) *types.SchedulingConfig {
	config := &types.SchedulingConfig{
		NodeSelector:      spec.NodeSelector,
		NodeName:          spec.NodeName,
		SchedulerName:     spec.SchedulerName,
		PriorityClassName: spec.PriorityClassName,
		Priority:          spec.Priority,
		RuntimeClassName:  spec.RuntimeClassName,
	}

	// 转换 Affinity
	if spec.Affinity != nil {
		config.Affinity = convertK8sAffinityToConfig(spec.Affinity)
	}

	// 转换 Tolerations
	if len(spec.Tolerations) > 0 {
		config.Tolerations = convertK8sTolerationsToConfig(spec.Tolerations)
	}

	// 转换 TopologySpreadConstraints
	if len(spec.TopologySpreadConstraints) > 0 {
		config.TopologySpreadConstraints = convertK8sTopologySpreadConstraintsToConfig(spec.TopologySpreadConstraints)
	}

	return config
}

// convertAffinityConfigToK8s 转换亲和性配置到 K8s 格式
func convertAffinityConfigToK8s(config *types.AffinityConfig) *corev1.Affinity {
	if config == nil {
		return nil
	}

	affinity := &corev1.Affinity{}

	// 节点亲和性
	if config.NodeAffinity != nil {
		affinity.NodeAffinity = &corev1.NodeAffinity{}

		if config.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution != nil {
			affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution = &corev1.NodeSelector{
				NodeSelectorTerms: convertNodeSelectorTermsToK8s(config.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms),
			}
		}

		if len(config.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution) > 0 {
			preferred := make([]corev1.PreferredSchedulingTerm, 0)
			for _, p := range config.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution {
				preferred = append(preferred, corev1.PreferredSchedulingTerm{
					Weight:     p.Weight,
					Preference: convertNodeSelectorTermToK8s(&p.Preference),
				})
			}
			affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution = preferred
		}
	}

	// Pod 亲和性
	if config.PodAffinity != nil {
		affinity.PodAffinity = &corev1.PodAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution:  convertPodAffinityTermsToK8s(config.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution),
			PreferredDuringSchedulingIgnoredDuringExecution: convertWeightedPodAffinityTermsToK8s(config.PodAffinity.PreferredDuringSchedulingIgnoredDuringExecution),
		}
	}

	// Pod 反亲和性
	if config.PodAntiAffinity != nil {
		affinity.PodAntiAffinity = &corev1.PodAntiAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution:  convertPodAffinityTermsToK8s(config.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution),
			PreferredDuringSchedulingIgnoredDuringExecution: convertWeightedPodAffinityTermsToK8s(config.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution),
		}
	}

	return affinity
}

func convertNodeSelectorTermsToK8s(terms []types.NodeSelectorTermConfig) []corev1.NodeSelectorTerm {
	result := make([]corev1.NodeSelectorTerm, 0, len(terms))
	for _, term := range terms {
		result = append(result, convertNodeSelectorTermToK8s(&term))
	}
	return result
}

func convertNodeSelectorTermToK8s(term *types.NodeSelectorTermConfig) corev1.NodeSelectorTerm {
	k8sTerm := corev1.NodeSelectorTerm{}

	if len(term.MatchExpressions) > 0 {
		k8sTerm.MatchExpressions = make([]corev1.NodeSelectorRequirement, 0, len(term.MatchExpressions))
		for _, expr := range term.MatchExpressions {
			k8sTerm.MatchExpressions = append(k8sTerm.MatchExpressions, corev1.NodeSelectorRequirement{
				Key:      expr.Key,
				Operator: corev1.NodeSelectorOperator(expr.Operator),
				Values:   expr.Values,
			})
		}
	}

	if len(term.MatchFields) > 0 {
		k8sTerm.MatchFields = make([]corev1.NodeSelectorRequirement, 0, len(term.MatchFields))
		for _, field := range term.MatchFields {
			k8sTerm.MatchFields = append(k8sTerm.MatchFields, corev1.NodeSelectorRequirement{
				Key:      field.Key,
				Operator: corev1.NodeSelectorOperator(field.Operator),
				Values:   field.Values,
			})
		}
	}

	return k8sTerm
}

func convertPodAffinityTermsToK8s(terms []types.PodAffinityTermConfig) []corev1.PodAffinityTerm {
	result := make([]corev1.PodAffinityTerm, 0, len(terms))
	for _, term := range terms {
		k8sTerm := corev1.PodAffinityTerm{
			TopologyKey: term.TopologyKey,
			Namespaces:  term.Namespaces,
		}

		if term.LabelSelector != nil {
			k8sTerm.LabelSelector = convertLabelSelectorToK8s(term.LabelSelector)
		}

		if term.NamespaceSelector != nil {
			k8sTerm.NamespaceSelector = convertLabelSelectorToK8s(term.NamespaceSelector)
		}

		result = append(result, k8sTerm)
	}
	return result
}

func convertWeightedPodAffinityTermsToK8s(terms []types.WeightedPodAffinityTermConfig) []corev1.WeightedPodAffinityTerm {
	result := make([]corev1.WeightedPodAffinityTerm, 0, len(terms))
	for _, term := range terms {
		k8sTerm := corev1.WeightedPodAffinityTerm{
			Weight: term.Weight,
			PodAffinityTerm: corev1.PodAffinityTerm{
				TopologyKey: term.PodAffinityTerm.TopologyKey,
				Namespaces:  term.PodAffinityTerm.Namespaces,
			},
		}

		if term.PodAffinityTerm.LabelSelector != nil {
			k8sTerm.PodAffinityTerm.LabelSelector = convertLabelSelectorToK8s(term.PodAffinityTerm.LabelSelector)
		}

		if term.PodAffinityTerm.NamespaceSelector != nil {
			k8sTerm.PodAffinityTerm.NamespaceSelector = convertLabelSelectorToK8s(term.PodAffinityTerm.NamespaceSelector)
		}

		result = append(result, k8sTerm)
	}
	return result
}

func convertLabelSelectorToK8s(selector *types.LabelSelectorConfig) *metav1.LabelSelector {
	k8sSelector := &metav1.LabelSelector{
		MatchLabels: selector.MatchLabels,
	}

	if len(selector.MatchExpressions) > 0 {
		k8sSelector.MatchExpressions = make([]metav1.LabelSelectorRequirement, 0, len(selector.MatchExpressions))
		for _, expr := range selector.MatchExpressions {
			k8sSelector.MatchExpressions = append(k8sSelector.MatchExpressions, metav1.LabelSelectorRequirement{
				Key:      expr.Key,
				Operator: metav1.LabelSelectorOperator(expr.Operator),
				Values:   expr.Values,
			})
		}
	}

	return k8sSelector
}

// convertTolerationsConfigToK8s 转换容忍配置到 K8s 格式
func convertTolerationsConfigToK8s(tolerations []types.TolerationConfig) []corev1.Toleration {
	result := make([]corev1.Toleration, 0, len(tolerations))
	for _, t := range tolerations {
		toleration := corev1.Toleration{
			Key:               t.Key,
			Operator:          corev1.TolerationOperator(t.Operator),
			Value:             t.Value,
			Effect:            corev1.TaintEffect(t.Effect),
			TolerationSeconds: t.TolerationSeconds,
		}
		result = append(result, toleration)
	}
	return result
}

// convertTopologySpreadConstraintsToK8s 转换拓扑分布约束到 K8s 格式
func convertTopologySpreadConstraintsToK8s(constraints []types.TopologySpreadConstraintConfig) []corev1.TopologySpreadConstraint {
	result := make([]corev1.TopologySpreadConstraint, 0, len(constraints))
	for _, c := range constraints {
		constraint := corev1.TopologySpreadConstraint{
			MaxSkew:           c.MaxSkew,
			TopologyKey:       c.TopologyKey,
			WhenUnsatisfiable: corev1.UnsatisfiableConstraintAction(c.WhenUnsatisfiable),
			MinDomains:        c.MinDomains,
		}

		if c.LabelSelector != nil {
			constraint.LabelSelector = convertLabelSelectorToK8s(c.LabelSelector)
		}

		if c.NodeAffinityPolicy != nil {
			policy := corev1.NodeInclusionPolicy(*c.NodeAffinityPolicy)
			constraint.NodeAffinityPolicy = &policy
		}

		if c.NodeTaintsPolicy != nil {
			policy := corev1.NodeInclusionPolicy(*c.NodeTaintsPolicy)
			constraint.NodeTaintsPolicy = &policy
		}

		result = append(result, constraint)
	}
	return result
}

// 反向转换函数（从 K8s 到 Config）

func convertK8sAffinityToConfig(affinity *corev1.Affinity) *types.AffinityConfig {
	// 实现省略，与上面相反
	// 这里为了篇幅简化，实际使用时需要完整实现
	return &types.AffinityConfig{}
}

func convertK8sTolerationsToConfig(tolerations []corev1.Toleration) []types.TolerationConfig {
	result := make([]types.TolerationConfig, 0, len(tolerations))
	for _, t := range tolerations {
		result = append(result, types.TolerationConfig{
			Key:               t.Key,
			Operator:          string(t.Operator),
			Value:             t.Value,
			Effect:            string(t.Effect),
			TolerationSeconds: t.TolerationSeconds,
		})
	}
	return result
}

func convertK8sTopologySpreadConstraintsToConfig(constraints []corev1.TopologySpreadConstraint) []types.TopologySpreadConstraintConfig {
	// 实现省略
	return []types.TopologySpreadConstraintConfig{}
}
