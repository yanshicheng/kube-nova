package types

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
)

// ResourceQuotaInfo ResourceQuota 信息
type ResourceQuotaInfo struct {
	Name              string            `json:"name"`
	Namespace         string            `json:"namespace"`
	Labels            map[string]string `json:"labels,omitempty"`
	Annotations       map[string]string `json:"annotations,omitempty"`
	Age               string            `json:"age"`               // 如: "2d5h"
	CreationTimestamp int64             `json:"creationTimestamp"` // 时间戳（毫秒）

	// 使用率统计
	UsedPercentage float64 `json:"usedPercentage"` // 整体使用率百分比
	ResourceCount  int     `json:"resourceCount"`  // 配额项数量
}

// ResourceQuotaDetail ResourceQuota 详细信息，包含配额详情
type ResourceQuotaDetail struct {
	ResourceQuotaInfo

	// 资源配额
	CPUHard              string `json:"cpuHard,omitempty"`              // CPU限额（核心数）
	CPUUsed              string `json:"cpuUsed,omitempty"`              // CPU已使用
	MemoryHard           string `json:"memoryHard,omitempty"`           // 内存限额（GiB）
	MemoryUsed           string `json:"memoryUsed,omitempty"`           // 内存已使用
	StorageHard          string `json:"storageHard,omitempty"`          // 存储限额（GiB）
	StorageUsed          string `json:"storageUsed,omitempty"`          // 存储已使用
	GPUHard              string `json:"gpuHard,omitempty"`              // GPU限额
	GPUUsed              string `json:"gpuUsed,omitempty"`              // GPU已使用
	EphemeralStorageHard string `json:"ephemeralStorageHard,omitempty"` // 临时存储限额（GiB）
	EphemeralStorageUsed string `json:"ephemeralStorageUsed,omitempty"` // 临时存储已使用

	// 对象数量配额
	PodsHard          int64 `json:"podsHard,omitempty"`
	PodsUsed          int64 `json:"podsUsed,omitempty"`
	ConfigMapsHard    int64 `json:"configMapsHard,omitempty"`
	ConfigMapsUsed    int64 `json:"configMapsUsed,omitempty"`
	SecretsHard       int64 `json:"secretsHard,omitempty"`
	SecretsUsed       int64 `json:"secretsUsed,omitempty"`
	PVCsHard          int64 `json:"pvcsHard,omitempty"`
	PVCsUsed          int64 `json:"pvcsUsed,omitempty"`
	ServicesHard      int64 `json:"servicesHard,omitempty"`
	ServicesUsed      int64 `json:"servicesUsed,omitempty"`
	LoadBalancersHard int64 `json:"loadBalancersHard,omitempty"`
	LoadBalancersUsed int64 `json:"loadBalancersUsed,omitempty"`
	NodePortsHard     int64 `json:"nodePortsHard,omitempty"`
	NodePortsUsed     int64 `json:"nodePortsUsed,omitempty"`

	// 工作负载配额
	DeploymentsHard  int64 `json:"deploymentsHard,omitempty"`
	DeploymentsUsed  int64 `json:"deploymentsUsed,omitempty"`
	JobsHard         int64 `json:"jobsHard,omitempty"`
	JobsUsed         int64 `json:"jobsUsed,omitempty"`
	CronJobsHard     int64 `json:"cronJobsHard,omitempty"`
	CronJobsUsed     int64 `json:"cronJobsUsed,omitempty"`
	DaemonSetsHard   int64 `json:"daemonSetsHard,omitempty"`
	DaemonSetsUsed   int64 `json:"daemonSetsUsed,omitempty"`
	StatefulSetsHard int64 `json:"statefulSetsHard,omitempty"`
	StatefulSetsUsed int64 `json:"statefulSetsUsed,omitempty"`
	IngressesHard    int64 `json:"ingressesHard,omitempty"`
	IngressesUsed    int64 `json:"ingressesUsed,omitempty"`
}

// ResourceQuotaAllocated 资源配额分配情况（用于返回格式化的配额数据）
type ResourceQuotaAllocated struct {
	// 资源配额
	CPUAllocated              string `json:"cpuAllocated,omitempty"`              // CPU核心数
	MemoryAllocated           string `json:"memoryAllocated,omitempty"`           // 内存（GiB）
	StorageAllocated          string `json:"storageAllocated,omitempty"`          // 存储（GiB）
	GPUAllocated              string `json:"gpuAllocated,omitempty"`              // GPU数量
	EphemeralStorageAllocated string `json:"ephemeralStorageAllocated,omitempty"` // 临时存储（GiB）

	// 对象数量配额
	PodsAllocated          int64 `json:"podsAllocated,omitempty"`
	ConfigMapsAllocated    int64 `json:"configMapsAllocated,omitempty"`
	SecretsAllocated       int64 `json:"secretsAllocated,omitempty"`
	PVCsAllocated          int64 `json:"pvcsAllocated,omitempty"`
	ServicesAllocated      int64 `json:"servicesAllocated,omitempty"`
	LoadBalancersAllocated int64 `json:"loadBalancersAllocated,omitempty"`
	NodePortsAllocated     int64 `json:"nodePortsAllocated,omitempty"`

	// 工作负载配额
	DeploymentsAllocated  int64 `json:"deploymentsAllocated,omitempty"`
	JobsAllocated         int64 `json:"jobsAllocated,omitempty"`
	CronJobsAllocated     int64 `json:"cronJobsAllocated,omitempty"`
	DaemonSetsAllocated   int64 `json:"daemonSetsAllocated,omitempty"`
	StatefulSetsAllocated int64 `json:"statefulSetsAllocated,omitempty"`
	IngressesAllocated    int64 `json:"ingressesAllocated,omitempty"`
}

// ListResourceQuotaResponse ResourceQuota 列表响应
type ListResourceQuotaResponse struct {
	Total int                 `json:"total"`
	Items []ResourceQuotaInfo `json:"items"`
}

// ResourceQuotaOperator ResourceQuota 操作器接口
type ResourceQuotaOperator interface {
	// ========== 基础 CRUD 操作 ==========
	Create(quota *corev1.ResourceQuota) (*corev1.ResourceQuota, error)
	Get(namespace, name string) (*corev1.ResourceQuota, error)
	Update(quota *corev1.ResourceQuota) (*corev1.ResourceQuota, error)
	Delete(namespace, name string) error
	List(namespace string, search string, labelSelector string) (*ListResourceQuotaResponse, error)

	// ========== 详细信息 ==========
	GetDetail(namespace, name string) (*ResourceQuotaDetail, error)
	GetAllocated(namespace, name string) (*ResourceQuotaAllocated, error)

	// ========== 高级操作 ==========
	Watch(namespace string, opts metav1.ListOptions) (watch.Interface, error)
	UpdateLabels(namespace, name string, labels map[string]string) error
	UpdateAnnotations(namespace, name string, annotations map[string]string) error

	// ========== YAML 操作 ==========
	GetYaml(namespace, name string) (string, error)

	// ========== 配额操作 ==========
	// 更新配额规格
	UpdateQuotaSpec(namespace, name string, hard corev1.ResourceList) error
	// 检查是否超出配额
	CheckQuotaExceeded(namespace, name string) (bool, []string, error)
}
