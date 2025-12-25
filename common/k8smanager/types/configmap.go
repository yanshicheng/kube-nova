package types

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
)

// ConfigMapInfo ConfigMap 信息
type ConfigMapInfo struct {
	Name              string            `json:"name"`
	Namespace         string            `json:"namespace"`
	Data              map[string]string `json:"data"`
	BinaryData        map[string][]byte `json:"binaryData,omitempty"`
	Labels            map[string]string `json:"labels,omitempty"`
	Annotations       map[string]string `json:"annotations,omitempty"`
	Age               string            `json:"age"`               // 如: "2d5h"
	CreationTimestamp int64             `json:"creationTimestamp"` // 时间戳（毫秒）
	DataCount         int               `json:"dataCount"`         // 数据项数量
}

// ConfigMapDataItem ConfigMap 数据项
type ConfigMapDataItem struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// GetConfigMapDataResponse 获取 ConfigMap 数据响应
type GetConfigMapDataResponse struct {
	Name      string              `json:"name"`
	Namespace string              `json:"namespace"`
	Data      []ConfigMapDataItem `json:"data"`
}

// ListConfigMapResponse ConfigMap 列表响应（修改为无分页）
type ListConfigMapResponse struct {
	Total int             `json:"total"`
	Items []ConfigMapInfo `json:"items"`
}

// ConfigMapUsageReference ConfigMap 被引用的信息
type ConfigMapUsageReference struct {
	ResourceType   string   `json:"resourceType"` // Deployment, StatefulSet, DaemonSet, Job, CronJob, Pod
	ResourceName   string   `json:"resourceName"`
	Namespace      string   `json:"namespace"`
	UsageType      []string `json:"usageType"`      // volume, env, envFrom
	UsedKeys       []string `json:"usedKeys"`       // 使用的具体 key
	ContainerNames []string `json:"containerNames"` // 使用的容器名称
}

// ConfigMapUsageResponse ConfigMap 引用情况响应
type ConfigMapUsageResponse struct {
	ConfigMapName      string                    `json:"configMapName"`
	ConfigMapNamespace string                    `json:"configMapNamespace"`
	UsedBy             []ConfigMapUsageReference `json:"usedBy"`
	TotalUsageCount    int                       `json:"totalUsageCount"`
	CanDelete          bool                      `json:"canDelete"`               // 是否可以安全删除
	DeleteWarning      string                    `json:"deleteWarning,omitempty"` // 删除警告信息
}

// ConfigMapOperator ConfigMap 操作器接口
type ConfigMapOperator interface {
	// ========== 基础 CRUD 操作 ==========
	Create(cm *corev1.ConfigMap) (*corev1.ConfigMap, error)
	Get(namespace, name string) (*corev1.ConfigMap, error)
	Update(cm *corev1.ConfigMap) (*corev1.ConfigMap, error)
	Delete(namespace, name string) error
	List(namespace string, search string, labelSelector string) (*ListConfigMapResponse, error)
	GetData(namespace, name string) (*GetConfigMapDataResponse, error)
	// ========== 高级操作 ==========
	Watch(namespace string, opts metav1.ListOptions) (watch.Interface, error)
	UpdateLabels(namespace, name string, labels map[string]string) error
	UpdateAnnotations(namespace, name string, annotations map[string]string) error

	// ========== YAML 操作 ==========
	GetYaml(namespace, name string) (string, error) // 获取 YAML

	// ========== 数据操作 ==========
	// 更新单个键值对
	UpdateKey(namespace, name, key, value string) error
	// 删除单个键
	DeleteKey(namespace, name, key string) error
	// 批量更新数据
	UpdateData(namespace, name string, data map[string]string) error

	// ========== 引用关系查询 ==========
	// 获取 ConfigMap 被哪些资源引用
	GetUsage(namespace, name string) (*ConfigMapUsageResponse, error)
	// 检查是否可以安全删除
	CanDelete(namespace, name string) (bool, string, error)
}
