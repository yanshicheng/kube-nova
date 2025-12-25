package types

import (
	corev1 "k8s.io/api/core/v1"
)

// ServiceAccountInfo ServiceAccount 信息（对应 kubectl get sa -o wide）
type ServiceAccountInfo struct {
	Name              string            `json:"name"`
	Namespace         string            `json:"namespace"`
	Secrets           int               `json:"secrets"` // Secrets 数量
	Age               string            `json:"age"`     // 如: "2d5h"
	Labels            map[string]string `json:"labels,omitempty"`
	Annotations       map[string]string `json:"annotations,omitempty"`
	CreationTimestamp int64             `json:"creationTimestamp"` // 时间戳（毫秒）
}

// ListServiceAccountResponse ServiceAccount 列表响应
type ListServiceAccountResponse struct {
	Total int                  `json:"total"`
	Items []ServiceAccountInfo `json:"items"`
}

// ServiceAccountDescribe ServiceAccount 详情（类似 kubectl describe）
type ServiceAccountDescribe struct {
	Name                    string            `json:"name"`
	Namespace               string            `json:"namespace"`
	Labels                  map[string]string `json:"labels,omitempty"`
	Annotations             map[string]string `json:"annotations,omitempty"`
	ImagePullSecrets        []string          `json:"imagePullSecrets,omitempty"`
	Secrets                 []string          `json:"secrets,omitempty"`
	AutomountServiceAccount *bool             `json:"automountServiceAccountToken,omitempty"`
	CreationTimestamp       string            `json:"creationTimestamp"`
	Events                  []EventInfo       `json:"events,omitempty"`
}

// ServiceAccountAssociation ServiceAccount 关联信息
type ServiceAccountAssociation struct {
	ServiceAccountName string   `json:"serviceAccountName"`
	Namespace          string   `json:"namespace"`
	Pods               []string `json:"pods"`             // 使用该 SA 的 Pod 列表
	PodCount           int      `json:"podCount"`         // Pod 数量
	Secrets            []string `json:"secrets"`          // 关联的 Secret 列表
	ImagePullSecrets   []string `json:"imagePullSecrets"` // ImagePullSecrets 列表
}

// ServiceAccountOperator ServiceAccount 操作器接口
type ServiceAccountOperator interface {
	Create(*corev1.ServiceAccount) error
	// ========== 基础查询操作 ==========
	// Get 获取 ServiceAccount
	Get(namespace, name string) (*corev1.ServiceAccount, error)
	// List 获取 ServiceAccount 列表，支持搜索和标签过滤
	List(namespace string, search string, labelSelector string) (*ListServiceAccountResponse, error)

	// ========== YAML 操作 ==========
	// GetYaml 获取 ServiceAccount 的 YAML
	GetYaml(namespace, name string) (string, error)

	// ========== 更新操作 ==========
	// Update 更新 ServiceAccount（通过 YAML）
	Update(namespace, name string, sa *corev1.ServiceAccount) error

	// ========== Describe 操作 ==========
	// Describe 获取 ServiceAccount 详情（类似 kubectl describe）
	Describe(namespace, name string) (string, error)

	// ========== 删除操作 ==========
	// Delete 删除 ServiceAccount
	Delete(namespace, name string) error

	// ========== 关联查询 ==========
	// GetAssociation 获取关联信息（Pods、Secrets 等）
	GetAssociation(namespace, name string) (*ServiceAccountAssociation, error)
}
