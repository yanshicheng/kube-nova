package types

import (
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	vpav1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
)

// ==================== HPA 类型定义 ====================

// HPAMetricTarget HPA 指标目标
type HPAMetricTarget struct {
	Type               string `json:"type"` // Utilization, AverageValue, Value
	AverageUtilization *int32 `json:"averageUtilization,omitempty"`
	AverageValue       string `json:"averageValue,omitempty"`
	Value              string `json:"value,omitempty"`
}

// HPAResourceMetric HPA 资源指标
type HPAResourceMetric struct {
	Name   string          `json:"name"`
	Target HPAMetricTarget `json:"target"`
}

// HPAMetricDef HPA 指标定义
type HPAMetricDef struct {
	Name     string            `json:"name"`
	Selector map[string]string `json:"selector,omitempty"`
}

// HPADescribedObject HPA 描述对象
type HPADescribedObject struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Name       string `json:"name"`
}

// HPAPodsMetric HPA Pods 指标
type HPAPodsMetric struct {
	Metric HPAMetricDef    `json:"metric"`
	Target HPAMetricTarget `json:"target"`
}

// HPAObjectMetric HPA Object 指标
type HPAObjectMetric struct {
	Metric          HPAMetricDef       `json:"metric"`
	DescribedObject HPADescribedObject `json:"describedObject"`
	Target          HPAMetricTarget    `json:"target"`
}

// HPAExternalMetric HPA External 指标
type HPAExternalMetric struct {
	Metric HPAMetricDef    `json:"metric"`
	Target HPAMetricTarget `json:"target"`
}

// HPAContainerResourceMetric HPA Container 资源指标
type HPAContainerResourceMetric struct {
	Name      string          `json:"name"`
	Container string          `json:"container"`
	Target    HPAMetricTarget `json:"target"`
}

// HPAMetric HPA 指标配置
type HPAMetric struct {
	Type              string                      `json:"type"`
	Resource          *HPAResourceMetric          `json:"resource,omitempty"`
	Pods              *HPAPodsMetric              `json:"pods,omitempty"`
	Object            *HPAObjectMetric            `json:"object,omitempty"`
	External          *HPAExternalMetric          `json:"external,omitempty"`
	ContainerResource *HPAContainerResourceMetric `json:"containerResource,omitempty"`
}

// HPAScalingPolicy HPA 扩缩容策略
type HPAScalingPolicy struct {
	Type          string `json:"type"`
	Value         int32  `json:"value"`
	PeriodSeconds int32  `json:"periodSeconds"`
}

// HPAScalingRules HPA 扩缩容规则
type HPAScalingRules struct {
	StabilizationWindowSeconds *int32             `json:"stabilizationWindowSeconds,omitempty"`
	SelectPolicy               string             `json:"selectPolicy,omitempty"`
	Policies                   []HPAScalingPolicy `json:"policies,omitempty"`
}

// HPABehavior HPA 行为配置
type HPABehavior struct {
	ScaleUp   *HPAScalingRules `json:"scaleUp,omitempty"`
	ScaleDown *HPAScalingRules `json:"scaleDown,omitempty"`
}

// HPACurrentMetricValue HPA 当前指标值详情
type HPACurrentMetricValue struct {
	AverageUtilization *int32 `json:"averageUtilization,omitempty"`
	AverageValue       string `json:"averageValue,omitempty"`
	Value              string `json:"value,omitempty"`
}

// HPACurrentMetric HPA 当前指标
type HPACurrentMetric struct {
	Type    string                `json:"type"`
	Current HPACurrentMetricValue `json:"current"`
}

// HPACondition HPA 状态条件
type HPACondition struct {
	Type               string `json:"type"`
	Status             string `json:"status"`
	Reason             string `json:"reason"`
	Message            string `json:"message"`
	LastTransitionTime int64  `json:"lastTransitionTime"`
}

// HPADetail HPA 详情
type HPADetail struct {
	Name              string             `json:"name"`
	Namespace         string             `json:"namespace"`
	TargetRef         TargetRefInfo      `json:"targetRef"`
	MinReplicas       *int32             `json:"minReplicas,omitempty"`
	MaxReplicas       int32              `json:"maxReplicas"`
	Metrics           []HPAMetric        `json:"metrics"`
	Behavior          *HPABehavior       `json:"behavior,omitempty"`
	CurrentReplicas   int32              `json:"currentReplicas"`
	DesiredReplicas   int32              `json:"desiredReplicas"`
	CurrentMetrics    []HPACurrentMetric `json:"currentMetrics,omitempty"`
	Conditions        []HPACondition     `json:"conditions,omitempty"`
	Labels            map[string]string  `json:"labels,omitempty"`
	Annotations       map[string]string  `json:"annotations,omitempty"`
	Age               string             `json:"age"`
	CreationTimestamp int64              `json:"creationTimestamp"`
}

// ==================== VPA 类型定义 ====================

// VPAUpdatePolicy VPA 更新策略
type VPAUpdatePolicy struct {
	UpdateMode  string `json:"updateMode,omitempty"`
	MinReplicas *int32 `json:"minReplicas,omitempty"`
}

// VPAResourceConstraints VPA 资源约束
type VPAResourceConstraints struct {
	CPU    string `json:"cpu,omitempty"`
	Memory string `json:"memory,omitempty"`
}

// VPAResourcePolicy VPA 容器资源策略
type VPAResourcePolicy struct {
	ContainerName       string                 `json:"containerName"`
	Mode                string                 `json:"mode,omitempty"`
	MinAllowed          VPAResourceConstraints `json:"minAllowed,omitempty"`
	MaxAllowed          VPAResourceConstraints `json:"maxAllowed,omitempty"`
	ControlledResources []string               `json:"controlledResources,omitempty"`
	ControlledValues    string                 `json:"controlledValues,omitempty"`
}

// VPAResourcePolicyConfig VPA 资源策略配置
type VPAResourcePolicyConfig struct {
	ContainerPolicies []VPAResourcePolicy `json:"containerPolicies,omitempty"`
}

// VPAResourceRecommendation VPA 资源推荐
type VPAResourceRecommendation struct {
	CPU    string `json:"cpu,omitempty"`
	Memory string `json:"memory,omitempty"`
}

// VPARecommendation VPA 容器推荐
type VPARecommendation struct {
	ContainerName  string                    `json:"containerName"`
	Target         VPAResourceRecommendation `json:"target,omitempty"`
	LowerBound     VPAResourceRecommendation `json:"lowerBound,omitempty"`
	UpperBound     VPAResourceRecommendation `json:"upperBound,omitempty"`
	UncappedTarget VPAResourceRecommendation `json:"uncappedTarget,omitempty"`
}

// VPARecommendationConfig VPA 推荐配置
type VPARecommendationConfig struct {
	ContainerRecommendations []VPARecommendation `json:"containerRecommendations,omitempty"`
}

// VPACondition VPA 状态条件
type VPACondition struct {
	Type               string `json:"type"`
	Status             string `json:"status"`
	Reason             string `json:"reason,omitempty"`
	Message            string `json:"message,omitempty"`
	LastTransitionTime int64  `json:"lastTransitionTime"`
}

// VPADetail VPA 详情
type VPADetail struct {
	Name              string                  `json:"name"`
	Namespace         string                  `json:"namespace"`
	TargetRef         TargetRefInfo           `json:"targetRef"`
	UpdatePolicy      *VPAUpdatePolicy        `json:"updatePolicy,omitempty"`
	ResourcePolicy    VPAResourcePolicyConfig `json:"resourcePolicy,omitempty"`
	Recommendation    VPARecommendationConfig `json:"recommendation,omitempty"`
	Conditions        []VPACondition          `json:"conditions,omitempty"`
	Labels            map[string]string       `json:"labels,omitempty"`
	Annotations       map[string]string       `json:"annotations,omitempty"`
	Age               string                  `json:"age"`
	CreationTimestamp int64                   `json:"creationTimestamp"`
}

// ==================== Operator 接口定义 ====================

// HPAOperator HPA 操作器接口
type HPAOperator interface {
	// ========== 基础 CRUD 操作 ==========
	Create(hpa *autoscalingv2.HorizontalPodAutoscaler) (*autoscalingv2.HorizontalPodAutoscaler, error)
	Get(namespace, name string) (*autoscalingv2.HorizontalPodAutoscaler, error)
	Update(hpa *autoscalingv2.HorizontalPodAutoscaler) (*autoscalingv2.HorizontalPodAutoscaler, error)
	Delete(namespace, name string) error

	// ========== 详情和 YAML 操作 ==========
	GetDetail(namespace, name string) (*HPADetail, error)
	GetByTargetRef(namespace string, targetRef TargetRefInfo) (*autoscalingv2.HorizontalPodAutoscaler, error)
	GetDetailByTargetRef(namespace string, targetRef TargetRefInfo) (*HPADetail, error)

	GetYaml(namespace, name string) (string, error)

	// ========== 高级操作 ==========
	Watch(namespace string, opts metav1.ListOptions) (watch.Interface, error)
	UpdateLabels(namespace, name string, labels map[string]string) error
	UpdateAnnotations(namespace, name string, annotations map[string]string) error

	// ========== YAML 创建/更新 ==========
	CreateFromYaml(namespace string, yamlStr string) (*autoscalingv2.HorizontalPodAutoscaler, error)
	UpdateFromYaml(namespace, name string, yamlStr string) (*autoscalingv2.HorizontalPodAutoscaler, error)
}

// VPAOperator VPA 操作器接口
type VPAOperator interface {
	// ========== 基础 CRUD 操作 ==========
	Create(vpa *vpav1.VerticalPodAutoscaler) (*vpav1.VerticalPodAutoscaler, error)
	Get(namespace, name string) (*vpav1.VerticalPodAutoscaler, error)
	Update(vpa *vpav1.VerticalPodAutoscaler) (*vpav1.VerticalPodAutoscaler, error)
	Delete(namespace, name string) error
	// ========== 通过 TargetRef 查询 ==========
	GetByTargetRef(namespace string, targetRef TargetRefInfo) (*vpav1.VerticalPodAutoscaler, error)
	GetDetailByTargetRef(namespace string, targetRef TargetRefInfo) (*VPADetail, error)

	// ========== 详情和 YAML 操作 ==========
	GetDetail(namespace, name string) (*VPADetail, error)
	GetYaml(namespace, name string) (string, error)

	// ========== 高级操作 ==========
	Watch(namespace string, opts metav1.ListOptions) (watch.Interface, error)
	UpdateLabels(namespace, name string, labels map[string]string) error
	UpdateAnnotations(namespace, name string, annotations map[string]string) error

	// ========== YAML 创建/更新 ==========
	CreateFromYaml(namespace string, yamlStr string) (*vpav1.VerticalPodAutoscaler, error)
	UpdateFromYaml(namespace, name string, yamlStr string) (*vpav1.VerticalPodAutoscaler, error)
}
