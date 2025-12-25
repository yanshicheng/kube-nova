package types

import (
	rbacv1 "k8s.io/api/rbac/v1"
)

// RoleBindingInfoo RoleBinding 信息（对应 kubectl get rolebindings -o wide）
type RoleBindingInfoo struct {
	Name              string            `json:"name"`
	Namespace         string            `json:"namespace"`
	Role              string            `json:"role"` // Role 名称（格式：Role/rolename 或 ClusterRole/clusterrolename）
	Age               string            `json:"age"`  // 如: "2d5h"
	Labels            map[string]string `json:"labels,omitempty"`
	Annotations       map[string]string `json:"annotations,omitempty"`
	CreationTimestamp int64             `json:"creationTimestamp"` // 时间戳（毫秒）
}

// ListRoleBindingResponse RoleBinding 列表响应
type ListRoleBindingResponse struct {
	Total int                `json:"total"`
	Items []RoleBindingInfoo `json:"items"`
}

// RoleBindingDescribe RoleBinding 详情（类似 kubectl describe）
type RoleBindingDescribe struct {
	Name              string            `json:"name"`
	Namespace         string            `json:"namespace"`
	Labels            map[string]string `json:"labels,omitempty"`
	Annotations       map[string]string `json:"annotations,omitempty"`
	RoleRef           RoleRefInfo       `json:"roleRef"`
	Subjects          []SubjectInfo     `json:"subjects"`
	CreationTimestamp string            `json:"creationTimestamp"`
	Events            []EventInfo       `json:"events,omitempty"`
}

// RoleBindingAssociation RoleBinding 关联信息
type RoleBindingAssociation struct {
	RoleBindingName string        `json:"roleBindingName"`
	Namespace       string        `json:"namespace"`
	RoleName        string        `json:"roleName"`     // 引用的 Role 名称
	RoleKind        string        `json:"roleKind"`     // Role 或 ClusterRole
	Subjects        []SubjectInfo `json:"subjects"`     // 绑定的主体列表
	SubjectCount    int           `json:"subjectCount"` // 主体数量
}

// RoleBindingOperator RoleBinding 操作器接口
type RoleBindingOperator interface {
	Create(*rbacv1.RoleBinding) error
	// ========== 基础查询操作 ==========
	// Get 获取 RoleBinding
	Get(namespace, name string) (*rbacv1.RoleBinding, error)
	// List 获取 RoleBinding 列表，支持搜索和标签过滤
	List(namespace string, search string, labelSelector string) (*ListRoleBindingResponse, error)

	// ========== YAML 操作 ==========
	// GetYaml 获取 RoleBinding 的 YAML
	GetYaml(namespace, name string) (string, error)

	// ========== 更新操作 ==========
	// Update 更新 RoleBinding（通过 YAML）
	Update(namespace, name string, rb *rbacv1.RoleBinding) error

	// ========== Describe 操作 ==========
	// Describe 获取 RoleBinding 详情（类似 kubectl describe）
	Describe(namespace, name string) (string, error)

	// ========== 删除操作 ==========
	// Delete 删除 RoleBinding
	Delete(namespace, name string) error

	// ========== 关联查询 ==========
	// GetAssociation 获取关联信息（Role、Subjects 等）
	GetAssociation(namespace, name string) (*RoleBindingAssociation, error)
}
