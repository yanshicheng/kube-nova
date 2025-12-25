package types

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
)

// PersistentVolumeInfo PV 信息（对应 kubectl get pv -o wide）
type PersistentVolumeInfo struct {
	Name                  string            `json:"name"`
	Capacity              string            `json:"capacity"`              // CAPACITY: 5Gi
	AccessModes           string            `json:"accessModes"`           // ACCESS MODES: RWO, ROX, RWX, RWOP
	ReclaimPolicy         string            `json:"reclaimPolicy"`         // RECLAIM POLICY: Delete, Retain, Recycle
	Status                string            `json:"status"`                // STATUS: Available, Bound, Released, Failed
	Claim                 string            `json:"claim"`                 // CLAIM: namespace/pvc-name
	StorageClass          string            `json:"storageClass"`          // STORAGECLASS
	VolumeAttributesClass string            `json:"volumeAttributesClass"` // VOLUMEATTRIBUTESCLASS
	Reason                string            `json:"reason"`                // REASON
	VolumeMode            string            `json:"volumeMode"`            // VOLUMEMODE: Filesystem, Block
	Labels                map[string]string `json:"labels,omitempty"`
	Annotations           map[string]string `json:"annotations,omitempty"`
	Age                   string            `json:"age"`
	CreationTimestamp     int64             `json:"creationTimestamp"`
}

// ListPersistentVolumeResponse PV 列表响应
type ListPersistentVolumeResponse struct {
	Total int                    `json:"total"`
	Items []PersistentVolumeInfo `json:"items"`
}

// PVClaimInfo PV 绑定的 PVC 详情
type PVClaimInfo struct {
	Name         string            `json:"name"`
	Namespace    string            `json:"namespace"`
	Status       string            `json:"status"`
	Capacity     string            `json:"capacity"`
	AccessModes  string            `json:"accessModes"`
	StorageClass string            `json:"storageClass"`
	Labels       map[string]string `json:"labels,omitempty"`
	Age          string            `json:"age"`
}

// PVUsageInfo PV 使用情况
type PVUsageInfo struct {
	PVName       string       `json:"pvName"`
	Status       string       `json:"status"`
	ClaimInfo    *PVClaimInfo `json:"claimInfo,omitempty"` // 绑定的 PVC 信息
	UsedByPods   []PodRefInfo `json:"usedByPods"`          // 使用该 PV 的 Pod 列表
	CanDelete    bool         `json:"canDelete"`
	DeleteReason string       `json:"deleteReason,omitempty"`
}

// PodRefInfo Pod 引用信息
type PodRefInfo struct {
	Name       string `json:"name"`
	Namespace  string `json:"namespace"`
	NodeName   string `json:"nodeName"`
	Status     string `json:"status"`
	VolumeName string `json:"volumeName"` // Pod 中的 Volume 名称
}

// PersistentVolumeDescribe PV 详情
type PersistentVolumeDescribe struct {
	Name              string            `json:"name"`
	Labels            map[string]string `json:"labels,omitempty"`
	Annotations       map[string]string `json:"annotations,omitempty"`
	Finalizers        []string          `json:"finalizers,omitempty"`
	StorageClass      string            `json:"storageClass"`
	Status            string            `json:"status"`
	Claim             string            `json:"claim"`
	ReclaimPolicy     string            `json:"reclaimPolicy"`
	AccessModes       []string          `json:"accessModes"`
	VolumeMode        string            `json:"volumeMode"`
	Capacity          string            `json:"capacity"`
	NodeAffinity      string            `json:"nodeAffinity,omitempty"`
	Message           string            `json:"message,omitempty"`
	Source            string            `json:"source"` // NFS, HostPath, CSI 等
	SourceDetails     map[string]string `json:"sourceDetails,omitempty"`
	CreationTimestamp string            `json:"creationTimestamp"`
	Events            []EventInfo       `json:"events,omitempty"`
}

// PersistentVolumeOperator PV 操作器接口
type PersistentVolumeOperator interface {
	// ========== 基础 CRUD 操作 ==========
	// Create 创建 PV
	Create(pv *corev1.PersistentVolume) (*corev1.PersistentVolume, error)
	// Get 获取 PV
	Get(name string) (*corev1.PersistentVolume, error)
	// Update 更新 PV
	Update(pv *corev1.PersistentVolume) (*corev1.PersistentVolume, error)
	// Delete 删除 PV
	Delete(name string) error
	// List 获取 PV 列表，支持搜索、标签过滤、状态过滤、StorageClass 过滤
	List(search string, labelSelector string, status string, storageClass string) (*ListPersistentVolumeResponse, error)

	// ========== YAML 操作 ==========
	// GetYaml 获取 PV 的 YAML
	GetYaml(name string) (string, error)

	// ========== Describe 操作 ==========
	// Describe 获取 PV 详情（类似 kubectl describe）
	Describe(name string) (string, error)

	// ========== Watch 操作 ==========
	// Watch 监听 PV 变化
	Watch(opts metav1.ListOptions) (watch.Interface, error)

	// ========== 高级操作 ==========
	// UpdateLabels 更新标签
	UpdateLabels(name string, labels map[string]string) error
	// UpdateAnnotations 更新注解
	UpdateAnnotations(name string, annotations map[string]string) error

	// ========== 特殊接口 ==========
	// GetUsage 获取 PV 使用情况（绑定的 PVC 和使用的 Pod）
	GetUsage(name string) (*PVUsageInfo, error)
	// CanDelete 检查是否可以安全删除
	CanDelete(name string) (bool, string, error)
	// GetByStorageClass 根据 StorageClass 获取 PV 列表
	GetByStorageClass(storageClassName string) (*ListPersistentVolumeResponse, error)
	// GetByStatus 根据状态获取 PV 列表
	GetByStatus(status string) (*ListPersistentVolumeResponse, error)
}
