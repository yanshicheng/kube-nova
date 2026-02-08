package types

// ==================== 调度配置相关类型定义 ====================

// SchedulingConfig 调度配置（查询响应）
type SchedulingConfig struct {
	// 基础调度配置
	NodeSelector      map[string]string `json:"nodeSelector,omitempty"`      // 节点标签选择器
	NodeName          string            `json:"nodeName,omitempty"`          // 指定节点名称
	SchedulerName     string            `json:"schedulerName,omitempty"`     // 调度器名称
	PriorityClassName string            `json:"priorityClassName,omitempty"` // 优先级类名
	Priority          *int32            `json:"priority,omitempty"`          // 优先级值
	RuntimeClassName  *string           `json:"runtimeClassName,omitempty"`  // 运行时类名

	// 亲和性配置
	Affinity *AffinityConfig `json:"affinity,omitempty"`

	// 容忍配置
	Tolerations []TolerationConfig `json:"tolerations,omitempty"`

	// 拓扑分布约束
	TopologySpreadConstraints []TopologySpreadConstraintConfig `json:"topologySpreadConstraints,omitempty"`
}

// ==================== 亲和性配置 ====================

// AffinityConfig 亲和性配置
type AffinityConfig struct {
	NodeAffinity    *NodeAffinityConfig    `json:"nodeAffinity,omitempty"`    // 节点亲和性
	PodAffinity     *PodAffinityConfig     `json:"podAffinity,omitempty"`     // Pod亲和性
	PodAntiAffinity *PodAntiAffinityConfig `json:"podAntiAffinity,omitempty"` // Pod反亲和性
}

// NodeAffinityConfig 节点亲和性配置
type NodeAffinityConfig struct {
	// 硬性要求：必须满足才能调度
	RequiredDuringSchedulingIgnoredDuringExecution *NodeSelectorConfig `json:"requiredDuringSchedulingIgnoredDuringExecution,omitempty"`
	// 软性偏好：优先满足，不满足也可调度
	PreferredDuringSchedulingIgnoredDuringExecution []PreferredSchedulingTermConfig `json:"preferredDuringSchedulingIgnoredDuringExecution,omitempty"`
}

// NodeSelectorConfig 节点选择器配置
type NodeSelectorConfig struct {
	NodeSelectorTerms []NodeSelectorTermConfig `json:"nodeSelectorTerms"`
}

// NodeSelectorTermConfig 节点选择器条件
type NodeSelectorTermConfig struct {
	MatchExpressions []NodeSelectorRequirementConfig `json:"matchExpressions,omitempty"` // 标签匹配表达式
	MatchFields      []NodeSelectorRequirementConfig `json:"matchFields,omitempty"`      // 字段匹配表达式
}

// NodeSelectorRequirementConfig 节点选择器要求
type NodeSelectorRequirementConfig struct {
	Key      string   `json:"key"`              // 键名
	Operator string   `json:"operator"`         // 操作符: In, NotIn, Exists, DoesNotExist, Gt, Lt
	Values   []string `json:"values,omitempty"` // 值列表
}

// PreferredSchedulingTermConfig 偏好调度条件
type PreferredSchedulingTermConfig struct {
	Weight     int32                  `json:"weight"`     // 权重 1-100
	Preference NodeSelectorTermConfig `json:"preference"` // 偏好条件
}

// ==================== Pod 亲和性配置 ====================

// PodAffinityConfig Pod亲和性配置
type PodAffinityConfig struct {
	// 硬性要求
	RequiredDuringSchedulingIgnoredDuringExecution []PodAffinityTermConfig `json:"requiredDuringSchedulingIgnoredDuringExecution,omitempty"`
	// 软性偏好
	PreferredDuringSchedulingIgnoredDuringExecution []WeightedPodAffinityTermConfig `json:"preferredDuringSchedulingIgnoredDuringExecution,omitempty"`
}

// PodAntiAffinityConfig Pod反亲和性配置
type PodAntiAffinityConfig struct {
	// 硬性要求
	RequiredDuringSchedulingIgnoredDuringExecution []PodAffinityTermConfig `json:"requiredDuringSchedulingIgnoredDuringExecution,omitempty"`
	// 软性偏好
	PreferredDuringSchedulingIgnoredDuringExecution []WeightedPodAffinityTermConfig `json:"preferredDuringSchedulingIgnoredDuringExecution,omitempty"`
}

// PodAffinityTermConfig Pod亲和性条件
type PodAffinityTermConfig struct {
	LabelSelector     *LabelSelectorConfig `json:"labelSelector,omitempty"`     // 标签选择器
	Namespaces        []string             `json:"namespaces,omitempty"`        // 命名空间列表
	TopologyKey       string               `json:"topologyKey"`                 // 拓扑键（如 kubernetes.io/hostname）
	NamespaceSelector *LabelSelectorConfig `json:"namespaceSelector,omitempty"` // 命名空间选择器
}

// WeightedPodAffinityTermConfig 带权重的Pod亲和性条件
type WeightedPodAffinityTermConfig struct {
	Weight          int32                 `json:"weight"`          // 权重 1-100
	PodAffinityTerm PodAffinityTermConfig `json:"podAffinityTerm"` // 亲和性条件
}

// LabelSelectorConfig 标签选择器配置
type LabelSelectorConfig struct {
	MatchLabels      map[string]string                `json:"matchLabels,omitempty"`      // 精确匹配标签
	MatchExpressions []LabelSelectorRequirementConfig `json:"matchExpressions,omitempty"` // 表达式匹配
}

// LabelSelectorRequirementConfig 标签选择器要求
type LabelSelectorRequirementConfig struct {
	Key      string   `json:"key"`              // 键名
	Operator string   `json:"operator"`         // 操作符: In, NotIn, Exists, DoesNotExist
	Values   []string `json:"values,omitempty"` // 值列表
}

// ==================== 容忍配置 ====================

// TolerationConfig 容忍配置
type TolerationConfig struct {
	Key               string `json:"key,omitempty"`               // 污点键
	Operator          string `json:"operator,omitempty"`          // 操作符: Equal, Exists
	Value             string `json:"value,omitempty"`             // 污点值
	Effect            string `json:"effect,omitempty"`            // 效果: NoSchedule, PreferNoSchedule, NoExecute
	TolerationSeconds *int64 `json:"tolerationSeconds,omitempty"` // 容忍时间（秒）
}

// ==================== 拓扑分布约束 ====================

// TopologySpreadConstraintConfig 拓扑分布约束配置
type TopologySpreadConstraintConfig struct {
	MaxSkew            int32                `json:"maxSkew"`                      // 最大偏差
	TopologyKey        string               `json:"topologyKey"`                  // 拓扑键
	WhenUnsatisfiable  string               `json:"whenUnsatisfiable"`            // 不满足时策略: DoNotSchedule, ScheduleAnyway
	LabelSelector      *LabelSelectorConfig `json:"labelSelector,omitempty"`      // 标签选择器
	MinDomains         *int32               `json:"minDomains,omitempty"`         // 最小域数量
	NodeAffinityPolicy *string              `json:"nodeAffinityPolicy,omitempty"` // 节点亲和性策略: Honor, Ignore
	NodeTaintsPolicy   *string              `json:"nodeTaintsPolicy,omitempty"`   // 节点污点策略: Honor, Ignore
	MatchLabelKeys     []string             `json:"matchLabelKeys,omitempty"`     // 匹配标签键列表
}

// ==================== 更新请求 ====================

// UpdateSchedulingConfigRequest 更新调度配置请求
// 使用指针类型来区分"未设置"和"设置为空值"
type UpdateSchedulingConfigRequest struct {
	// 基础调度配置
	NodeSelector      map[string]string `json:"nodeSelector,omitempty"`      // nil=不修改, 空map=清空
	NodeName          *string           `json:"nodeName,omitempty"`          // nil=不修改, 空字符串=清空
	SchedulerName     *string           `json:"schedulerName,omitempty"`     // nil=不修改
	PriorityClassName *string           `json:"priorityClassName,omitempty"` // nil=不修改
	Priority          *int32            `json:"priority,omitempty"`          // nil=不修改
	RuntimeClassName  *string           `json:"runtimeClassName,omitempty"`  // nil=不修改

	// 亲和性配置 - 整体替换
	Affinity *AffinityConfig `json:"affinity,omitempty"` // nil=不修改

	// 容忍配置 - 整体替换
	Tolerations []TolerationConfig `json:"tolerations,omitempty"` // nil=不修改, 空切片=清空

	// 拓扑分布约束 - 整体替换
	TopologySpreadConstraints []TopologySpreadConstraintConfig `json:"topologySpreadConstraints,omitempty"` // nil=不修改

	// 清除标志（用于明确清除某些字段）
	ClearNodeName          bool `json:"clearNodeName,omitempty"`          // 是否清除NodeName
	ClearAffinity          bool `json:"clearAffinity,omitempty"`          // 是否清除所有亲和性
	ClearTolerations       bool `json:"clearTolerations,omitempty"`       // 是否清除所有容忍
	ClearTopologySpread    bool `json:"clearTopologySpread,omitempty"`    // 是否清除拓扑分布约束
	ClearSchedulerName     bool `json:"clearSchedulerName,omitempty"`     // 是否清除调度器名称
	ClearPriorityClassName bool `json:"clearPriorityClassName,omitempty"` // 是否清除优先级类名
	ClearRuntimeClassName  bool `json:"clearRuntimeClassName,omitempty"`  // 是否清除运行时类名
}

// ==================== 资源类型调度能力 ====================

// SchedulingCapability 调度能力描述
type SchedulingCapability struct {
	SupportsNodeSelector    bool   `json:"supportsNodeSelector"`
	SupportsNodeName        bool   `json:"supportsNodeName"`
	SupportsNodeAffinity    bool   `json:"supportsNodeAffinity"`
	SupportsPodAffinity     bool   `json:"supportsPodAffinity"`
	SupportsPodAntiAffinity bool   `json:"supportsPodAntiAffinity"`
	SupportsTopologySpread  bool   `json:"supportsTopologySpread"`
	SupportsTolerations     bool   `json:"supportsTolerations"`
	NodeNameWarning         string `json:"nodeNameWarning,omitempty"` // NodeName使用警告
}

// GetSchedulingCapability 获取资源类型的调度能力
func GetSchedulingCapability(resourceType string) *SchedulingCapability {
	cap := &SchedulingCapability{
		SupportsNodeSelector:    true,
		SupportsNodeName:        true,
		SupportsNodeAffinity:    true,
		SupportsPodAffinity:     true,
		SupportsPodAntiAffinity: true,
		SupportsTopologySpread:  true,
		SupportsTolerations:     true,
	}

	switch resourceType {
	case "DAEMONSET":
		// DaemonSet 设置 NodeName 会导致只在单节点运行，通常不是期望行为
		cap.NodeNameWarning = "DaemonSet 设置 NodeName 将导致只在指定节点运行单个Pod，建议使用 NodeSelector 或 NodeAffinity"
	}

	return cap
}
