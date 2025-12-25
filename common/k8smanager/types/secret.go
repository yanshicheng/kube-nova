package types

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
)

// SecretInfo Secret 信息
type SecretInfo struct {
	Name              string            `json:"name"`
	Namespace         string            `json:"namespace"`
	Type              string            `json:"type"`                 // Opaque, kubernetes.io/tls, kubernetes.io/dockerconfigjson, etc.
	Data              map[string][]byte `json:"data,omitempty"`       // 实际数据（base64编码）
	StringData        map[string]string `json:"stringData,omitempty"` // 字符串数据（用于创建/更新）
	Labels            map[string]string `json:"labels,omitempty"`
	Annotations       map[string]string `json:"annotations,omitempty"`
	DataCount         int               `json:"dataCount"`         // DATA 列：数据项数量
	Age               string            `json:"age"`               // 如: "2d5h"
	CreationTimestamp int64             `json:"creationTimestamp"` // 时间戳（毫秒）
}

// SecretDataItem Secret 数据项
type SecretDataItem struct {
	Key   string `json:"key"`
	Value string `json:"value"` // base64 解码后的值
}

// GetSecretDataResponse 获取 Secret 数据响应
type GetSecretDataResponse struct {
	Name      string           `json:"name"`
	Namespace string           `json:"namespace"`
	Type      string           `json:"type"`
	Data      []SecretDataItem `json:"data"`
}

// ListSecretResponse Secret 列表响应（修改为无分页）
type ListSecretResponse struct {
	Total int          `json:"total"`
	Items []SecretInfo `json:"items"`
}

// SecretUsageReference Secret 被引用的信息
type SecretUsageReference struct {
	ResourceType   string   `json:"resourceType"` // Deployment, StatefulSet, DaemonSet, Job, CronJob, Pod, ServiceAccount
	ResourceName   string   `json:"resourceName"`
	Namespace      string   `json:"namespace"`
	UsageType      []string `json:"usageType"`      // volume, env, envFrom, imagePullSecret, serviceAccountToken
	UsedKeys       []string `json:"usedKeys"`       // 使用的具体 key
	ContainerNames []string `json:"containerNames"` // 使用的容器名称
}

// SecretUsageResponse Secret 引用情况响应
type SecretUsageResponse struct {
	SecretName      string                 `json:"secretName"`
	SecretNamespace string                 `json:"secretNamespace"`
	SecretType      string                 `json:"secretType"`
	UsedBy          []SecretUsageReference `json:"usedBy"`
	TotalUsageCount int                    `json:"totalUsageCount"`
	CanDelete       bool                   `json:"canDelete"`               // 是否可以安全删除
	DeleteWarning   string                 `json:"deleteWarning,omitempty"` // 删除警告信息
}

// SecretOperator Secret 操作器接口
type SecretOperator interface {
	// ========== 基础 CRUD 操作 ==========
	Create(secret *corev1.Secret) (*corev1.Secret, error)
	Get(namespace, name string) (*corev1.Secret, error)
	Update(secret *corev1.Secret) (*corev1.Secret, error)
	Delete(namespace, name string) error
	// List 方法增加了 secretType 参数用于类型过滤
	List(namespace string, search string, labelSelector string, secretType string) (*ListSecretResponse, error)
	GetData(namespace, name string) (*GetSecretDataResponse, error)

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

	// ========== 特定类型 Secret 操作 ==========
	// 创建 TLS Secret
	CreateTLSSecret(namespace, name string, cert, key []byte, labels, annotations map[string]string) (*corev1.Secret, error)
	// 创建 Docker Registry Secret
	CreateDockerRegistrySecret(namespace, name, server, username, password, email string, labels, annotations map[string]string) (*corev1.Secret, error)
	// 创建 Basic Auth Secret
	CreateBasicAuthSecret(namespace, name, username, password string, labels, annotations map[string]string) (*corev1.Secret, error)

	// ========== 引用关系查询 ==========
	// 获取 Secret 被哪些资源引用
	GetUsage(namespace, name string) (*SecretUsageResponse, error)
	// 检查是否可以安全删除
	CanDelete(namespace, name string) (bool, string, error)
}
