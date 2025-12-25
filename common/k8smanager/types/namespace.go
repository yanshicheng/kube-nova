package types

import (
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
)

type NamespaceInfo struct {
	Name              string
	Status            string
	CreationTimestamp time.Time
}

type ListNamespaceResponse struct {
	ListResponse
	Items []NamespaceInfo
}

// 定义 namespace 资源数量的统计信息
type NamespaceStatistics struct {
	Pods           int32
	PodsNotRunning int32
	PodsRunning    int32
	Service        int32
	ConfigMap      int32
	Secret         int32
	Pvc            int32
	Deployment     int32
	StatefulSet    int32
	DaemonSet      int32
	Job            int32
	CronJob        int32
	Ingress        int32
	ServiceAccount int32
}

// ResourceQuotaRequest 资源配额请求
type ResourceQuotaRequest struct {
	Name        string
	Namespace   string
	Annotations map[string]string
	Labels      map[string]string

	// 计算资源配额
	CPUAllocated              string // CPU核心数
	MemoryAllocated           string // 内存（GiB）
	StorageAllocated          string // 存储（GiB）
	GPUAllocated              string // GPU数量
	EphemeralStorageAllocated string // 临时存储（GiB）

	// 对象数量配额
	PodsAllocated          int64
	ConfigMapsAllocated    int64
	SecretsAllocated       int64
	PVCsAllocated          int64
	ServicesAllocated      int64
	LoadBalancersAllocated int64
	NodePortsAllocated     int64

	// 工作负载配额
	DeploymentsAllocated  int64
	JobsAllocated         int64
	CronJobsAllocated     int64
	DaemonSetsAllocated   int64
	StatefulSetsAllocated int64
	IngressesAllocated    int64
}

// LimitRangeRequest LimitRange请求
type LimitRangeRequest struct {
	Name        string
	Namespace   string
	Annotations map[string]string
	Labels      map[string]string

	// Pod级别限制
	PodMaxCPU              string // 核心数
	PodMaxMemory           string // GiB
	PodMaxEphemeralStorage string // GiB
	PodMinCPU              string // 核心数（支持毫核）
	PodMinMemory           string // MiB
	PodMinEphemeralStorage string // MiB

	// Container级别限制
	ContainerMaxCPU              string // 核心数
	ContainerMaxMemory           string // GiB
	ContainerMaxEphemeralStorage string // GiB
	ContainerMinCPU              string // 核心数（支持毫核）
	ContainerMinMemory           string // MiB
	ContainerMinEphemeralStorage string // MiB

	// Container默认限制（limits）
	ContainerDefaultCPU              string // 核心数
	ContainerDefaultMemory           string // MiB
	ContainerDefaultEphemeralStorage string // GiB

	// Container默认请求（requests）
	ContainerDefaultRequestCPU              string // 核心数
	ContainerDefaultRequestMemory           string // MiB
	ContainerDefaultRequestEphemeralStorage string // MiB
}

// NamespaceOperator Namespace 操作器接口
type NamespaceOperator interface {
	// 基础 CRUD 操作
	Create(*corev1.Namespace) (*corev1.Namespace, error)
	Get(string) (*corev1.Namespace, error)
	Update(*corev1.Namespace) (*corev1.Namespace, error)
	Delete(string) error
	List(ListRequest) (*ListNamespaceResponse, error)
	ListAll() ([]corev1.Namespace, error)
	// 统计
	GetStatistics(string) (*NamespaceStatistics, error)

	// 高级操作
	Watch(metav1.ListOptions) (watch.Interface, error)
	UpdateLabels(string, map[string]string) error
	UpdateAnnotations(string, map[string]string) error
	UpdateFinalizers(string, []string) error

	// 状态检查
	IsActive(string) (bool, error)
	IsTerminating(string) (bool, error)
	// 资源配额和限制管理
	CreateOrUpdateResourceQuota(*ResourceQuotaRequest) error
	CreateOrUpdateLimitRange(*LimitRangeRequest) error
}
