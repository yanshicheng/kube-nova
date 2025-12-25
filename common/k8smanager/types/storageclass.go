package types

import (
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
)

// StorageClassInfo StorageClass 信息（对应 kubectl get storageclasses -o wide）
type StorageClassInfo struct {
	Name                 string            `json:"name"`
	Provisioner          string            `json:"provisioner"`          // PROVISIONER
	ReclaimPolicy        string            `json:"reclaimPolicy"`        // RECLAIMPOLICY: Delete, Retain, Recycle
	VolumeBindingMode    string            `json:"volumeBindingMode"`    // VOLUMEBINDINGMODE: Immediate, WaitForFirstConsumer
	AllowVolumeExpansion bool              `json:"allowVolumeExpansion"` // ALLOWVOLUMEEXPANSION
	IsDefault            bool              `json:"isDefault"`            // 是否为默认 StorageClass
	Parameters           map[string]string `json:"parameters,omitempty"`
	MountOptions         []string          `json:"mountOptions,omitempty"`
	Labels               map[string]string `json:"labels,omitempty"`
	Annotations          map[string]string `json:"annotations,omitempty"`
	Age                  string            `json:"age"`
	CreationTimestamp    int64             `json:"creationTimestamp"`
}

// ListStorageClassResponse StorageClass 列表响应
type ListStorageClassResponse struct {
	Total int                `json:"total"`
	Items []StorageClassInfo `json:"items"`
}

// StorageClassPVInfo StorageClass 关联的 PV 信息
type StorageClassPVInfo struct {
	Name        string `json:"name"`
	Capacity    string `json:"capacity"`
	AccessModes string `json:"accessModes"`
	Status      string `json:"status"`
	Claim       string `json:"claim"` // namespace/pvcName
	Age         string `json:"age"`
}

// StorageClassPVResponse StorageClass 关联的 PV 列表响应
type StorageClassPVResponse struct {
	StorageClassName string               `json:"storageClassName"`
	PVCount          int                  `json:"pvCount"`
	BoundPVCount     int                  `json:"boundPVCount"`
	AvailablePVCount int                  `json:"availablePVCount"`
	TotalCapacity    string               `json:"totalCapacity"` // 总容量
	PVs              []StorageClassPVInfo `json:"pvs"`
}

// StorageClassDescribe StorageClass 详情（类似 kubectl describe）
type StorageClassDescribe struct {
	Name                 string            `json:"name"`
	IsDefaultClass       bool              `json:"isDefaultClass"`
	Provisioner          string            `json:"provisioner"`
	Parameters           map[string]string `json:"parameters,omitempty"`
	AllowVolumeExpansion *bool             `json:"allowVolumeExpansion,omitempty"`
	MountOptions         []string          `json:"mountOptions,omitempty"`
	ReclaimPolicy        string            `json:"reclaimPolicy"`
	VolumeBindingMode    string            `json:"volumeBindingMode"`
	AllowedTopologies    []string          `json:"allowedTopologies,omitempty"`
	Labels               map[string]string `json:"labels,omitempty"`
	Annotations          map[string]string `json:"annotations,omitempty"`
	CreationTimestamp    string            `json:"creationTimestamp"`
	Events               []EventInfo       `json:"events,omitempty"`
}

// StorageClassOperator StorageClass 操作器接口
type StorageClassOperator interface {
	// ========== 基础 CRUD 操作 ==========
	// Create 创建 StorageClass
	Create(sc *storagev1.StorageClass) (*storagev1.StorageClass, error)
	// Get 获取 StorageClass
	Get(name string) (*storagev1.StorageClass, error)
	// Update 更新 StorageClass
	Update(sc *storagev1.StorageClass) (*storagev1.StorageClass, error)
	// Delete 删除 StorageClass
	Delete(name string) error
	// List 获取 StorageClass 列表，支持搜索和标签过滤
	List(search string, labelSelector string) (*ListStorageClassResponse, error)

	// ========== YAML 操作 ==========
	// GetYaml 获取 StorageClass 的 YAML
	GetYaml(name string) (string, error)

	// ========== Describe 操作 ==========
	// Describe 获取 StorageClass 详情（类似 kubectl describe）
	Describe(name string) (string, error)

	// ========== Watch 操作 ==========
	// Watch 监听 StorageClass 变化
	Watch(opts metav1.ListOptions) (watch.Interface, error)

	// ========== 高级操作 ==========
	// UpdateLabels 更新标签
	UpdateLabels(name string, labels map[string]string) error
	// UpdateAnnotations 更新注解
	UpdateAnnotations(name string, annotations map[string]string) error
	// SetDefault 设置为默认 StorageClass
	SetDefault(name string) error
	// UnsetDefault 取消默认 StorageClass
	UnsetDefault(name string) error

	// ========== 特殊接口：关联查询 ==========
	// GetAssociatedPVs 获取关联的 PV 列表
	GetAssociatedPVs(name string) (*StorageClassPVResponse, error)
	// CanDelete 检查是否可以安全删除（是否有关联的 PV）
	CanDelete(name string) (bool, string, error)
}
