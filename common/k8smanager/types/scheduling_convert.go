package types

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ==================== PodSpec -> SchedulingConfig 转换 ====================

// ConvertPodSpecToSchedulingConfig 将 PodSpec 转换为调度配置
func ConvertPodSpecToSchedulingConfig(spec *corev1.PodSpec) *SchedulingConfig {
	if spec == nil {
		return &SchedulingConfig{}
	}

	config := &SchedulingConfig{
		NodeSelector:      spec.NodeSelector,
		NodeName:          spec.NodeName,
		SchedulerName:     spec.SchedulerName,
		PriorityClassName: spec.PriorityClassName,
		Priority:          spec.Priority,
		RuntimeClassName:  spec.RuntimeClassName,
	}

	if spec.Affinity != nil {
		config.Affinity = convertK8sAffinityToConfig(spec.Affinity)
	}

	if len(spec.Tolerations) > 0 {
		config.Tolerations = convertK8sTolerationsToConfig(spec.Tolerations)
	}

	if len(spec.TopologySpreadConstraints) > 0 {
		config.TopologySpreadConstraints = convertK8sTopologySpreadToConfig(spec.TopologySpreadConstraints)
	}

	return config
}

// convertK8sAffinityToConfig 将 K8s Affinity 转换为配置
func convertK8sAffinityToConfig(affinity *corev1.Affinity) *AffinityConfig {
	if affinity == nil {
		return nil
	}

	result := &AffinityConfig{}

	if affinity.NodeAffinity != nil {
		result.NodeAffinity = convertK8sNodeAffinityToConfig(affinity.NodeAffinity)
	}

	if affinity.PodAffinity != nil {
		result.PodAffinity = convertK8sPodAffinityToConfig(affinity.PodAffinity)
	}

	if affinity.PodAntiAffinity != nil {
		result.PodAntiAffinity = convertK8sPodAntiAffinityToConfig(affinity.PodAntiAffinity)
	}

	return result
}

func convertK8sNodeAffinityToConfig(na *corev1.NodeAffinity) *NodeAffinityConfig {
	if na == nil {
		return nil
	}

	result := &NodeAffinityConfig{}

	if na.RequiredDuringSchedulingIgnoredDuringExecution != nil {
		result.RequiredDuringSchedulingIgnoredDuringExecution = &NodeSelectorConfig{
			NodeSelectorTerms: make([]NodeSelectorTermConfig, 0, len(na.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms)),
		}
		for _, term := range na.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms {
			result.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms = append(
				result.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms,
				convertK8sNodeSelectorTermToConfig(term),
			)
		}
	}

	if len(na.PreferredDuringSchedulingIgnoredDuringExecution) > 0 {
		result.PreferredDuringSchedulingIgnoredDuringExecution = make([]PreferredSchedulingTermConfig, 0, len(na.PreferredDuringSchedulingIgnoredDuringExecution))
		for _, term := range na.PreferredDuringSchedulingIgnoredDuringExecution {
			result.PreferredDuringSchedulingIgnoredDuringExecution = append(
				result.PreferredDuringSchedulingIgnoredDuringExecution,
				PreferredSchedulingTermConfig{
					Weight:     term.Weight,
					Preference: convertK8sNodeSelectorTermToConfig(term.Preference),
				},
			)
		}
	}

	return result
}

func convertK8sNodeSelectorTermToConfig(term corev1.NodeSelectorTerm) NodeSelectorTermConfig {
	return NodeSelectorTermConfig{
		MatchExpressions: convertK8sNodeSelectorReqsToConfig(term.MatchExpressions),
		MatchFields:      convertK8sNodeSelectorReqsToConfig(term.MatchFields),
	}
}

func convertK8sNodeSelectorReqsToConfig(reqs []corev1.NodeSelectorRequirement) []NodeSelectorRequirementConfig {
	if len(reqs) == 0 {
		return nil
	}
	result := make([]NodeSelectorRequirementConfig, 0, len(reqs))
	for _, req := range reqs {
		result = append(result, NodeSelectorRequirementConfig{
			Key:      req.Key,
			Operator: string(req.Operator),
			Values:   req.Values,
		})
	}
	return result
}

func convertK8sPodAffinityToConfig(pa *corev1.PodAffinity) *PodAffinityConfig {
	if pa == nil {
		return nil
	}

	result := &PodAffinityConfig{}

	if len(pa.RequiredDuringSchedulingIgnoredDuringExecution) > 0 {
		result.RequiredDuringSchedulingIgnoredDuringExecution = convertK8sPodAffinityTermsToConfig(pa.RequiredDuringSchedulingIgnoredDuringExecution)
	}

	if len(pa.PreferredDuringSchedulingIgnoredDuringExecution) > 0 {
		result.PreferredDuringSchedulingIgnoredDuringExecution = make([]WeightedPodAffinityTermConfig, 0, len(pa.PreferredDuringSchedulingIgnoredDuringExecution))
		for _, term := range pa.PreferredDuringSchedulingIgnoredDuringExecution {
			result.PreferredDuringSchedulingIgnoredDuringExecution = append(
				result.PreferredDuringSchedulingIgnoredDuringExecution,
				WeightedPodAffinityTermConfig{
					Weight:          term.Weight,
					PodAffinityTerm: convertK8sPodAffinityTermToConfig(term.PodAffinityTerm),
				},
			)
		}
	}

	return result
}

func convertK8sPodAntiAffinityToConfig(paa *corev1.PodAntiAffinity) *PodAntiAffinityConfig {
	if paa == nil {
		return nil
	}

	result := &PodAntiAffinityConfig{}

	if len(paa.RequiredDuringSchedulingIgnoredDuringExecution) > 0 {
		result.RequiredDuringSchedulingIgnoredDuringExecution = convertK8sPodAffinityTermsToConfig(paa.RequiredDuringSchedulingIgnoredDuringExecution)
	}

	if len(paa.PreferredDuringSchedulingIgnoredDuringExecution) > 0 {
		result.PreferredDuringSchedulingIgnoredDuringExecution = make([]WeightedPodAffinityTermConfig, 0, len(paa.PreferredDuringSchedulingIgnoredDuringExecution))
		for _, term := range paa.PreferredDuringSchedulingIgnoredDuringExecution {
			result.PreferredDuringSchedulingIgnoredDuringExecution = append(
				result.PreferredDuringSchedulingIgnoredDuringExecution,
				WeightedPodAffinityTermConfig{
					Weight:          term.Weight,
					PodAffinityTerm: convertK8sPodAffinityTermToConfig(term.PodAffinityTerm),
				},
			)
		}
	}

	return result
}

func convertK8sPodAffinityTermsToConfig(terms []corev1.PodAffinityTerm) []PodAffinityTermConfig {
	result := make([]PodAffinityTermConfig, 0, len(terms))
	for _, term := range terms {
		result = append(result, convertK8sPodAffinityTermToConfig(term))
	}
	return result
}

func convertK8sPodAffinityTermToConfig(term corev1.PodAffinityTerm) PodAffinityTermConfig {
	return PodAffinityTermConfig{
		LabelSelector:     convertK8sLabelSelectorToConfig(term.LabelSelector),
		Namespaces:        term.Namespaces,
		TopologyKey:       term.TopologyKey,
		NamespaceSelector: convertK8sLabelSelectorToConfig(term.NamespaceSelector),
	}
}

func convertK8sLabelSelectorToConfig(selector *metav1.LabelSelector) *LabelSelectorConfig {
	if selector == nil {
		return nil
	}

	result := &LabelSelectorConfig{
		MatchLabels: selector.MatchLabels,
	}

	if len(selector.MatchExpressions) > 0 {
		result.MatchExpressions = make([]LabelSelectorRequirementConfig, 0, len(selector.MatchExpressions))
		for _, expr := range selector.MatchExpressions {
			result.MatchExpressions = append(result.MatchExpressions, LabelSelectorRequirementConfig{
				Key:      expr.Key,
				Operator: string(expr.Operator),
				Values:   expr.Values,
			})
		}
	}

	return result
}

func convertK8sTolerationsToConfig(tolerations []corev1.Toleration) []TolerationConfig {
	result := make([]TolerationConfig, 0, len(tolerations))
	for _, t := range tolerations {
		tc := TolerationConfig{
			Key:      t.Key,
			Operator: string(t.Operator),
			Value:    t.Value,
			Effect:   string(t.Effect),
		}
		if t.TolerationSeconds != nil {
			tc.TolerationSeconds = t.TolerationSeconds
		}
		result = append(result, tc)
	}
	return result
}

func convertK8sTopologySpreadToConfig(constraints []corev1.TopologySpreadConstraint) []TopologySpreadConstraintConfig {
	result := make([]TopologySpreadConstraintConfig, 0, len(constraints))
	for _, c := range constraints {
		tsc := TopologySpreadConstraintConfig{
			MaxSkew:           c.MaxSkew,
			TopologyKey:       c.TopologyKey,
			WhenUnsatisfiable: string(c.WhenUnsatisfiable),
			LabelSelector:     convertK8sLabelSelectorToConfig(c.LabelSelector),
			MinDomains:        c.MinDomains,
			MatchLabelKeys:    c.MatchLabelKeys,
		}
		if c.NodeAffinityPolicy != nil {
			policy := string(*c.NodeAffinityPolicy)
			tsc.NodeAffinityPolicy = &policy
		}
		if c.NodeTaintsPolicy != nil {
			policy := string(*c.NodeTaintsPolicy)
			tsc.NodeTaintsPolicy = &policy
		}
		result = append(result, tsc)
	}
	return result
}

// ==================== SchedulingConfig -> K8s 转换 ====================

// ApplySchedulingConfigToPodSpec 将更新请求应用到 PodSpec
func ApplySchedulingConfigToPodSpec(spec *corev1.PodSpec, req *UpdateSchedulingConfigRequest) {
	if spec == nil || req == nil {
		return
	}

	// 处理 NodeSelector
	// map 类型：nil 表示清空，非 nil 直接赋值（包括空 map）
	if req.NodeSelector != nil {
		spec.NodeSelector = req.NodeSelector
	} else {
		spec.NodeSelector = nil
	}

	// 处理 NodeName
	// 指针类型：非 nil 且非空字符串才设置，否则清空
	if req.NodeName != nil && *req.NodeName != "" {
		spec.NodeName = *req.NodeName
	} else {
		spec.NodeName = ""
	}

	// 处理 SchedulerName
	// 指针类型：非 nil 且非空字符串才设置，否则清空
	if req.SchedulerName != nil && *req.SchedulerName != "" {
		spec.SchedulerName = *req.SchedulerName
	} else {
		spec.SchedulerName = ""
	}

	// 处理 PriorityClassName
	// 指针类型：非 nil 且非空字符串才设置，否则清空
	if req.PriorityClassName != nil && *req.PriorityClassName != "" {
		spec.PriorityClassName = *req.PriorityClassName
	} else {
		spec.PriorityClassName = ""
	}

	// 处理 Priority
	// 指针类型：非 nil 就设置，nil 就清空
	if req.Priority != nil {
		spec.Priority = req.Priority
	} else {
		spec.Priority = nil
	}

	// 处理 RuntimeClassName
	// 指针类型：非 nil 且非空字符串才设置，否则清空
	if req.RuntimeClassName != nil && *req.RuntimeClassName != "" {
		spec.RuntimeClassName = req.RuntimeClassName
	} else {
		spec.RuntimeClassName = nil
	}

	// 处理 Affinity
	// 指针类型：非 nil 就转换并设置，nil 就清空
	if req.Affinity != nil {
		spec.Affinity = convertConfigToK8sAffinity(req.Affinity)
	} else {
		spec.Affinity = nil
	}

	// 处理 Tolerations
	// 切片类型：非 nil 就转换并设置，nil 就清空
	if req.Tolerations != nil {
		spec.Tolerations = convertConfigToK8sTolerations(req.Tolerations)
	} else {
		spec.Tolerations = nil
	}

	// 处理 TopologySpreadConstraints
	// 切片类型：非 nil 就转换并设置，nil 就清空
	if req.TopologySpreadConstraints != nil {
		spec.TopologySpreadConstraints = convertConfigToK8sTopologySpread(req.TopologySpreadConstraints)
	} else {
		spec.TopologySpreadConstraints = nil
	}
}

func convertConfigToK8sAffinity(config *AffinityConfig) *corev1.Affinity {
	if config == nil {
		return nil
	}

	result := &corev1.Affinity{}

	if config.NodeAffinity != nil {
		result.NodeAffinity = convertConfigToK8sNodeAffinity(config.NodeAffinity)
	}

	if config.PodAffinity != nil {
		result.PodAffinity = convertConfigToK8sPodAffinity(config.PodAffinity)
	}

	if config.PodAntiAffinity != nil {
		result.PodAntiAffinity = convertConfigToK8sPodAntiAffinity(config.PodAntiAffinity)
	}

	return result
}

func convertConfigToK8sNodeAffinity(config *NodeAffinityConfig) *corev1.NodeAffinity {
	if config == nil {
		return nil
	}

	result := &corev1.NodeAffinity{}

	if config.RequiredDuringSchedulingIgnoredDuringExecution != nil {
		result.RequiredDuringSchedulingIgnoredDuringExecution = &corev1.NodeSelector{
			NodeSelectorTerms: make([]corev1.NodeSelectorTerm, 0, len(config.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms)),
		}
		for _, term := range config.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms {
			result.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms = append(
				result.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms,
				convertConfigToK8sNodeSelectorTerm(term),
			)
		}
	}

	if len(config.PreferredDuringSchedulingIgnoredDuringExecution) > 0 {
		result.PreferredDuringSchedulingIgnoredDuringExecution = make([]corev1.PreferredSchedulingTerm, 0, len(config.PreferredDuringSchedulingIgnoredDuringExecution))
		for _, term := range config.PreferredDuringSchedulingIgnoredDuringExecution {
			result.PreferredDuringSchedulingIgnoredDuringExecution = append(
				result.PreferredDuringSchedulingIgnoredDuringExecution,
				corev1.PreferredSchedulingTerm{
					Weight:     term.Weight,
					Preference: convertConfigToK8sNodeSelectorTerm(term.Preference),
				},
			)
		}
	}

	return result
}

func convertConfigToK8sNodeSelectorTerm(config NodeSelectorTermConfig) corev1.NodeSelectorTerm {
	return corev1.NodeSelectorTerm{
		MatchExpressions: convertConfigToK8sNodeSelectorReqs(config.MatchExpressions),
		MatchFields:      convertConfigToK8sNodeSelectorReqs(config.MatchFields),
	}
}

func convertConfigToK8sNodeSelectorReqs(configs []NodeSelectorRequirementConfig) []corev1.NodeSelectorRequirement {
	if len(configs) == 0 {
		return nil
	}
	result := make([]corev1.NodeSelectorRequirement, 0, len(configs))
	for _, c := range configs {
		result = append(result, corev1.NodeSelectorRequirement{
			Key:      c.Key,
			Operator: corev1.NodeSelectorOperator(c.Operator),
			Values:   c.Values,
		})
	}
	return result
}

func convertConfigToK8sPodAffinity(config *PodAffinityConfig) *corev1.PodAffinity {
	if config == nil {
		return nil
	}

	result := &corev1.PodAffinity{}

	if len(config.RequiredDuringSchedulingIgnoredDuringExecution) > 0 {
		result.RequiredDuringSchedulingIgnoredDuringExecution = convertConfigToK8sPodAffinityTerms(config.RequiredDuringSchedulingIgnoredDuringExecution)
	}

	if len(config.PreferredDuringSchedulingIgnoredDuringExecution) > 0 {
		result.PreferredDuringSchedulingIgnoredDuringExecution = make([]corev1.WeightedPodAffinityTerm, 0, len(config.PreferredDuringSchedulingIgnoredDuringExecution))
		for _, term := range config.PreferredDuringSchedulingIgnoredDuringExecution {
			result.PreferredDuringSchedulingIgnoredDuringExecution = append(
				result.PreferredDuringSchedulingIgnoredDuringExecution,
				corev1.WeightedPodAffinityTerm{
					Weight:          term.Weight,
					PodAffinityTerm: convertConfigToK8sPodAffinityTerm(term.PodAffinityTerm),
				},
			)
		}
	}

	return result
}

func convertConfigToK8sPodAntiAffinity(config *PodAntiAffinityConfig) *corev1.PodAntiAffinity {
	if config == nil {
		return nil
	}

	result := &corev1.PodAntiAffinity{}

	if len(config.RequiredDuringSchedulingIgnoredDuringExecution) > 0 {
		result.RequiredDuringSchedulingIgnoredDuringExecution = convertConfigToK8sPodAffinityTerms(config.RequiredDuringSchedulingIgnoredDuringExecution)
	}

	if len(config.PreferredDuringSchedulingIgnoredDuringExecution) > 0 {
		result.PreferredDuringSchedulingIgnoredDuringExecution = make([]corev1.WeightedPodAffinityTerm, 0, len(config.PreferredDuringSchedulingIgnoredDuringExecution))
		for _, term := range config.PreferredDuringSchedulingIgnoredDuringExecution {
			result.PreferredDuringSchedulingIgnoredDuringExecution = append(
				result.PreferredDuringSchedulingIgnoredDuringExecution,
				corev1.WeightedPodAffinityTerm{
					Weight:          term.Weight,
					PodAffinityTerm: convertConfigToK8sPodAffinityTerm(term.PodAffinityTerm),
				},
			)
		}
	}

	return result
}

func convertConfigToK8sPodAffinityTerms(configs []PodAffinityTermConfig) []corev1.PodAffinityTerm {
	result := make([]corev1.PodAffinityTerm, 0, len(configs))
	for _, c := range configs {
		result = append(result, convertConfigToK8sPodAffinityTerm(c))
	}
	return result
}

func convertConfigToK8sPodAffinityTerm(config PodAffinityTermConfig) corev1.PodAffinityTerm {
	return corev1.PodAffinityTerm{
		LabelSelector:     convertConfigToK8sLabelSelector(config.LabelSelector),
		Namespaces:        config.Namespaces,
		TopologyKey:       config.TopologyKey,
		NamespaceSelector: convertConfigToK8sLabelSelector(config.NamespaceSelector),
	}
}

func convertConfigToK8sLabelSelector(config *LabelSelectorConfig) *metav1.LabelSelector {
	if config == nil {
		return nil
	}

	result := &metav1.LabelSelector{
		MatchLabels: config.MatchLabels,
	}

	if len(config.MatchExpressions) > 0 {
		result.MatchExpressions = make([]metav1.LabelSelectorRequirement, 0, len(config.MatchExpressions))
		for _, expr := range config.MatchExpressions {
			result.MatchExpressions = append(result.MatchExpressions, metav1.LabelSelectorRequirement{
				Key:      expr.Key,
				Operator: metav1.LabelSelectorOperator(expr.Operator),
				Values:   expr.Values,
			})
		}
	}

	return result
}

func convertConfigToK8sTolerations(configs []TolerationConfig) []corev1.Toleration {
	result := make([]corev1.Toleration, 0, len(configs))
	for _, c := range configs {
		t := corev1.Toleration{
			Key:      c.Key,
			Operator: corev1.TolerationOperator(c.Operator),
			Value:    c.Value,
			Effect:   corev1.TaintEffect(c.Effect),
		}
		// 修复：先检查 nil，再检查值
		if c.TolerationSeconds != nil && *c.TolerationSeconds > 0 {
			t.TolerationSeconds = c.TolerationSeconds
		}

		result = append(result, t)
	}
	return result
}

func convertConfigToK8sTopologySpread(configs []TopologySpreadConstraintConfig) []corev1.TopologySpreadConstraint {
	result := make([]corev1.TopologySpreadConstraint, 0, len(configs))
	for _, c := range configs {
		tsc := corev1.TopologySpreadConstraint{
			MaxSkew:           c.MaxSkew,
			TopologyKey:       c.TopologyKey,
			WhenUnsatisfiable: corev1.UnsatisfiableConstraintAction(c.WhenUnsatisfiable),
			LabelSelector:     convertConfigToK8sLabelSelector(c.LabelSelector),
			MinDomains:        c.MinDomains,
			MatchLabelKeys:    c.MatchLabelKeys,
		}
		if c.NodeAffinityPolicy != nil {
			policy := corev1.NodeInclusionPolicy(*c.NodeAffinityPolicy)
			tsc.NodeAffinityPolicy = &policy
		}
		if c.NodeTaintsPolicy != nil {
			policy := corev1.NodeInclusionPolicy(*c.NodeTaintsPolicy)
			tsc.NodeTaintsPolicy = &policy
		}
		result = append(result, tsc)
	}
	return result
}
