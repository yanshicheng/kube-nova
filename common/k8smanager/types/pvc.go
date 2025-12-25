package types

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PVCInfo PVC 信息
type PVCInfo struct {
	Name              string                              `json:"name"`
	Namespace         string                              `json:"namespace"`
	Status            corev1.PersistentVolumeClaimPhase   `json:"status"`       // Pending, Bound, Lost
	Volume            string                              `json:"volume"`       // 绑定的 PV 名称
	Capacity          string                              `json:"capacity"`     // 如: "10Gi"
	AccessModes       []corev1.PersistentVolumeAccessMode `json:"accessModes"`  // ReadWriteOnce, ReadOnlyMany, ReadWriteMany
	StorageClass      string                              `json:"storageClass"` // 存储类名称
	VolumeMode        *corev1.PersistentVolumeMode        `json:"volumeMode"`   // Filesystem, Block
	Labels            map[string]string                   `json:"labels,omitempty"`
	Annotations       map[string]string                   `json:"annotations,omitempty"`
	Age               string                              `json:"age"`               // 如: "2d5h"
	CreationTimestamp int64                               `json:"creationTimestamp"` // 时间戳（毫秒）
}

// ListPVCResponse PVC 列表响应
type ListPVCResponse struct {
	Total int       `json:"total"`
	Items []PVCInfo `json:"items"`
}

// PVCDetail PVC 详细信息
type PVCDetail struct {
	Name              string                              `json:"name"`
	Namespace         string                              `json:"namespace"`
	Status            corev1.PersistentVolumeClaimPhase   `json:"status"`
	Volume            string                              `json:"volume"`
	Capacity          string                              `json:"capacity"`
	AccessModes       []corev1.PersistentVolumeAccessMode `json:"accessModes"`
	StorageClass      string                              `json:"storageClass"`
	VolumeMode        *corev1.PersistentVolumeMode        `json:"volumeMode"`
	Selector          *metav1.LabelSelector               `json:"selector,omitempty"`
	Labels            map[string]string                   `json:"labels,omitempty"`
	Annotations       map[string]string                   `json:"annotations,omitempty"`
	Finalizers        []string                            `json:"finalizers,omitempty"`
	Age               string                              `json:"age"`
	CreationTimestamp int64                               `json:"creationTimestamp"`
	RequestStorage    string                              `json:"requestStorage"` // 请求的存储大小
}

// PVCAssociation PVC 关联信息
type PVCAssociation struct {
	PVCName   string   `json:"pvcName"`
	Namespace string   `json:"namespace"`
	PVName    string   `json:"pvName"`   // 绑定的 PV
	Pods      []string `json:"pods"`     // 使用该 PVC 的 Pod 列表
	PodCount  int      `json:"podCount"` // Pod 数量
}

// PVCOperator PVC 操作器接口
type PVCOperator interface {
	// ========== 基础查询操作 ==========
	Create(*corev1.PersistentVolumeClaim) error
	// Get 获取 PVC
	Get(namespace, name string) (*corev1.PersistentVolumeClaim, error)
	// List 获取 PVC 列表，支持搜索和标签过滤
	List(namespace string, search string, labelSelector string) (*ListPVCResponse, error)

	// ========== YAML 操作 ==========
	// GetYaml 获取 PVC 的 YAML
	GetYaml(namespace, name string) (string, error)

	// ========== 更新操作 ==========
	// Update 更新 PVC（通过 YAML）
	Update(namespace, name string, pvc *corev1.PersistentVolumeClaim) error

	// ========== Describe 操作 ==========
	// Describe 获取 PVC 详情（类似 kubectl describe）
	Describe(namespace, name string) (string, error)

	// ========== 删除操作 ==========
	// Delete 删除 PVC
	Delete(namespace, name string) error

	// ========== 关联查询 ==========
	// GetAssociation 获取关联信息（PV、Pods 等）
	GetAssociation(namespace, name string) (*PVCAssociation, error)
}
