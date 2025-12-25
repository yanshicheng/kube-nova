package types

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
)

// LimitRangeInfo LimitRange 信息
type LimitRangeInfo struct {
	Name              string            `json:"name"`
	Namespace         string            `json:"namespace"`
	Labels            map[string]string `json:"labels,omitempty"`
	Annotations       map[string]string `json:"annotations,omitempty"`
	Age               string            `json:"age"`               // 如: "2d5h"
	CreationTimestamp int64             `json:"creationTimestamp"` // 时间戳（毫秒）
	LimitCount        int               `json:"limitCount"`        // 限制项数量
}

// LimitRangeDetail LimitRange 详细信息
type LimitRangeDetail struct {
	LimitRangeInfo

	// Pod级别限制
	PodMaxCPU              string `json:"podMaxCpu,omitempty"`              // 核心数
	PodMaxMemory           string `json:"podMaxMemory,omitempty"`           // GiB
	PodMaxEphemeralStorage string `json:"podMaxEphemeralStorage,omitempty"` // GiB
	PodMinCPU              string `json:"podMinCpu,omitempty"`              // 核心数（支持毫核）
	PodMinMemory           string `json:"podMinMemory,omitempty"`           // MiB
	PodMinEphemeralStorage string `json:"podMinEphemeralStorage,omitempty"` // MiB

	// Container级别限制
	ContainerMaxCPU              string `json:"containerMaxCpu,omitempty"`              // 核心数
	ContainerMaxMemory           string `json:"containerMaxMemory,omitempty"`           // GiB
	ContainerMaxEphemeralStorage string `json:"containerMaxEphemeralStorage,omitempty"` // GiB
	ContainerMinCPU              string `json:"containerMinCpu,omitempty"`              // 核心数（支持毫核）
	ContainerMinMemory           string `json:"containerMinMemory,omitempty"`           // MiB
	ContainerMinEphemeralStorage string `json:"containerMinEphemeralStorage,omitempty"` // MiB

	// Container默认限制（limits）
	ContainerDefaultCPU              string `json:"containerDefaultCpu,omitempty"`              // 核心数
	ContainerDefaultMemory           string `json:"containerDefaultMemory,omitempty"`           // MiB
	ContainerDefaultEphemeralStorage string `json:"containerDefaultEphemeralStorage,omitempty"` // GiB

	// Container默认请求（requests）
	ContainerDefaultRequestCPU              string `json:"containerDefaultRequestCpu,omitempty"`              // 核心数
	ContainerDefaultRequestMemory           string `json:"containerDefaultRequestMemory,omitempty"`           // MiB
	ContainerDefaultRequestEphemeralStorage string `json:"containerDefaultRequestEphemeralStorage,omitempty"` // MiB

	// PVC级别限制
	PVCMaxStorage string `json:"pvcMaxStorage,omitempty"` // GiB
	PVCMinStorage string `json:"pvcMinStorage,omitempty"` // GiB
}

// LimitRangeLimits 格式化的限制信息（用于返回）
type LimitRangeLimits struct {
	// Pod级别限制
	PodMaxCPU              string `json:"podMaxCpu,omitempty"`              // 核心数
	PodMaxMemory           string `json:"podMaxMemory,omitempty"`           // GiB
	PodMaxEphemeralStorage string `json:"podMaxEphemeralStorage,omitempty"` // GiB
	PodMinCPU              string `json:"podMinCpu,omitempty"`              // 核心数（支持毫核）
	PodMinMemory           string `json:"podMinMemory,omitempty"`           // MiB
	PodMinEphemeralStorage string `json:"podMinEphemeralStorage,omitempty"` // MiB

	// Container级别限制
	ContainerMaxCPU              string `json:"containerMaxCpu,omitempty"`              // 核心数
	ContainerMaxMemory           string `json:"containerMaxMemory,omitempty"`           // GiB
	ContainerMaxEphemeralStorage string `json:"containerMaxEphemeralStorage,omitempty"` // GiB
	ContainerMinCPU              string `json:"containerMinCpu,omitempty"`              // 核心数（支持毫核）
	ContainerMinMemory           string `json:"containerMinMemory,omitempty"`           // MiB
	ContainerMinEphemeralStorage string `json:"containerMinEphemeralStorage,omitempty"` // MiB

	// Container默认限制（limits）
	ContainerDefaultCPU              string `json:"containerDefaultCpu,omitempty"`              // 核心数
	ContainerDefaultMemory           string `json:"containerDefaultMemory,omitempty"`           // MiB
	ContainerDefaultEphemeralStorage string `json:"containerDefaultEphemeralStorage,omitempty"` // GiB

	// Container默认请求（requests）
	ContainerDefaultRequestCPU              string `json:"containerDefaultRequestCpu,omitempty"`              // 核心数
	ContainerDefaultRequestMemory           string `json:"containerDefaultRequestMemory,omitempty"`           // MiB
	ContainerDefaultRequestEphemeralStorage string `json:"containerDefaultRequestEphemeralStorage,omitempty"` // MiB

	// PVC级别限制
	PVCMaxStorage string `json:"pvcMaxStorage,omitempty"` // GiB
	PVCMinStorage string `json:"pvcMinStorage,omitempty"` // GiB
}

// ListLimitRangeResponse LimitRange 列表响应
type ListLimitRangeResponse struct {
	Total int              `json:"total"`
	Items []LimitRangeInfo `json:"items"`
}

// LimitRangeOperator LimitRange 操作器接口
type LimitRangeOperator interface {
	// ========== 基础 CRUD 操作 ==========
	Create(limitRange *corev1.LimitRange) (*corev1.LimitRange, error)
	Get(namespace, name string) (*corev1.LimitRange, error)
	Update(limitRange *corev1.LimitRange) (*corev1.LimitRange, error)
	Delete(namespace, name string) error
	List(namespace string, search string, labelSelector string) (*ListLimitRangeResponse, error)

	// ========== 详细信息 ==========
	GetDetail(namespace, name string) (*LimitRangeDetail, error)
	GetLimits(namespace, name string) (*LimitRangeLimits, error)

	// ========== 高级操作 ==========
	Watch(namespace string, opts metav1.ListOptions) (watch.Interface, error)
	UpdateLabels(namespace, name string, labels map[string]string) error
	UpdateAnnotations(namespace, name string, annotations map[string]string) error

	// ========== YAML 操作 ==========
	GetYaml(namespace, name string) (string, error)

	// ========== 限制范围操作 ==========
	// 更新限制范围规格
	UpdateLimitRangeSpec(namespace, name string, limits []corev1.LimitRangeItem) error
	// 验证资源请求是否满足限制
	ValidateResourceRequest(namespace, name string, resourceType corev1.LimitType,
		requests, limits corev1.ResourceList) (bool, []string, error)
}
