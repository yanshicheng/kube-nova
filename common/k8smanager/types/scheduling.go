package types

// ==================== 调度配置相关 ====================

// SchedulingConfig 调度配置
type SchedulingConfig struct {
	NodeSelector              map[string]string                `json:"nodeSelector,omitempty"`              // 节点选择器
	NodeName                  string                           `json:"nodeName,omitempty"`                  // 指定节点名称
	Affinity                  *AffinityConfig                  `json:"affinity,omitempty"`                  // 亲和性配置
	Tolerations               []TolerationConfig               `json:"tolerations,omitempty"`               // 容忍配置
	TopologySpreadConstraints []TopologySpreadConstraintConfig `json:"topologySpreadConstraints,omitempty"` // 拓扑分布约束
	SchedulerName             string                           `json:"schedulerName,omitempty"`             // 调度器名称
	PriorityClassName         string                           `json:"priorityClassName,omitempty"`         // 优先级类名称
	Priority                  *int32                           `json:"priority,omitempty"`                  // 优先级
	RuntimeClassName          *string                          `json:"runtimeClassName,omitempty"`          // 运行时类名称
}

// AffinityConfig 亲和性配置
type AffinityConfig struct {
	NodeAffinity    *NodeAffinityConfig    `json:"nodeAffinity,omitempty"`    // 节点亲和性
	PodAffinity     *PodAffinityConfig     `json:"podAffinity,omitempty"`     // Pod 亲和性
	PodAntiAffinity *PodAntiAffinityConfig `json:"podAntiAffinity,omitempty"` // Pod 反亲和性
}

// NodeAffinityConfig 节点亲和性配置
type NodeAffinityConfig struct {
	RequiredDuringSchedulingIgnoredDuringExecution  *NodeSelectorConfig             `json:"requiredDuringSchedulingIgnoredDuringExecution,omitempty"`
	PreferredDuringSchedulingIgnoredDuringExecution []PreferredSchedulingTermConfig `json:"preferredDuringSchedulingIgnoredDuringExecution,omitempty"`
}

// NodeSelectorConfig 节点选择器配置
type NodeSelectorConfig struct {
	NodeSelectorTerms []NodeSelectorTermConfig `json:"nodeSelectorTerms"`
}

// NodeSelectorTermConfig 节点选择器条件
type NodeSelectorTermConfig struct {
	MatchExpressions []NodeSelectorRequirementConfig `json:"matchExpressions,omitempty"`
	MatchFields      []NodeSelectorRequirementConfig `json:"matchFields,omitempty"`
}

// NodeSelectorRequirementConfig 节点选择器需求
type NodeSelectorRequirementConfig struct {
	Key      string   `json:"key"`
	Operator string   `json:"operator"` // In, NotIn, Exists, DoesNotExist, Gt, Lt
	Values   []string `json:"values,omitempty"`
}

// PreferredSchedulingTermConfig 优先调度条件
type PreferredSchedulingTermConfig struct {
	Weight     int32                  `json:"weight"`
	Preference NodeSelectorTermConfig `json:"preference"`
}

// PodAffinityConfig Pod 亲和性配置
type PodAffinityConfig struct {
	RequiredDuringSchedulingIgnoredDuringExecution  []PodAffinityTermConfig         `json:"requiredDuringSchedulingIgnoredDuringExecution,omitempty"`
	PreferredDuringSchedulingIgnoredDuringExecution []WeightedPodAffinityTermConfig `json:"preferredDuringSchedulingIgnoredDuringExecution,omitempty"`
}

// PodAntiAffinityConfig Pod 反亲和性配置
type PodAntiAffinityConfig struct {
	RequiredDuringSchedulingIgnoredDuringExecution  []PodAffinityTermConfig         `json:"requiredDuringSchedulingIgnoredDuringExecution,omitempty"`
	PreferredDuringSchedulingIgnoredDuringExecution []WeightedPodAffinityTermConfig `json:"preferredDuringSchedulingIgnoredDuringExecution,omitempty"`
}

// PodAffinityTermConfig Pod 亲和性条件
type PodAffinityTermConfig struct {
	LabelSelector     *LabelSelectorConfig `json:"labelSelector,omitempty"`
	Namespaces        []string             `json:"namespaces,omitempty"`
	TopologyKey       string               `json:"topologyKey"`
	NamespaceSelector *LabelSelectorConfig `json:"namespaceSelector,omitempty"`
}

// WeightedPodAffinityTermConfig 带权重的 Pod 亲和性条件
type WeightedPodAffinityTermConfig struct {
	Weight          int32                 `json:"weight"`
	PodAffinityTerm PodAffinityTermConfig `json:"podAffinityTerm"`
}

// LabelSelectorConfig 标签选择器配置
type LabelSelectorConfig struct {
	MatchLabels      map[string]string                `json:"matchLabels,omitempty"`
	MatchExpressions []LabelSelectorRequirementConfig `json:"matchExpressions,omitempty"`
}

// LabelSelectorRequirementConfig 标签选择器需求
type LabelSelectorRequirementConfig struct {
	Key      string   `json:"key"`
	Operator string   `json:"operator"` // In, NotIn, Exists, DoesNotExist
	Values   []string `json:"values,omitempty"`
}

// TolerationConfig 容忍配置
type TolerationConfig struct {
	Key               string `json:"key,omitempty"`
	Operator          string `json:"operator,omitempty"` // Exists, Equal
	Value             string `json:"value,omitempty"`
	Effect            string `json:"effect,omitempty"`            // NoSchedule, PreferNoSchedule, NoExecute
	TolerationSeconds *int64 `json:"tolerationSeconds,omitempty"` // 容忍时间（秒）
}

// TopologySpreadConstraintConfig 拓扑分布约束配置
type TopologySpreadConstraintConfig struct {
	MaxSkew            int32                `json:"maxSkew"`                      // 最大偏差
	TopologyKey        string               `json:"topologyKey"`                  // 拓扑键
	WhenUnsatisfiable  string               `json:"whenUnsatisfiable"`            // DoNotSchedule, ScheduleAnyway
	LabelSelector      *LabelSelectorConfig `json:"labelSelector,omitempty"`      // 标签选择器
	MinDomains         *int32               `json:"minDomains,omitempty"`         // 最小域数
	NodeAffinityPolicy *string              `json:"nodeAffinityPolicy,omitempty"` // Honor, Ignore
	NodeTaintsPolicy   *string              `json:"nodeTaintsPolicy,omitempty"`   // Honor, Ignore
}

// UpdateSchedulingConfigRequest 更新调度配置请求
type UpdateSchedulingConfigRequest struct {
	NodeSelector              map[string]string                `json:"nodeSelector,omitempty"`
	NodeName                  string                           `json:"nodeName,omitempty"`
	Affinity                  *AffinityConfig                  `json:"affinity,omitempty"`
	Tolerations               []TolerationConfig               `json:"tolerations,omitempty"`
	TopologySpreadConstraints []TopologySpreadConstraintConfig `json:"topologySpreadConstraints,omitempty"`
	SchedulerName             string                           `json:"schedulerName,omitempty"`
	PriorityClassName         string                           `json:"priorityClassName,omitempty"`
	Priority                  *int32                           `json:"priority,omitempty"`
	RuntimeClassName          *string                          `json:"runtimeClassName,omitempty"`
}
