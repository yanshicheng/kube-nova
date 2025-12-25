package types

import (
	rbacv1 "k8s.io/api/rbac/v1"
)

// RoleInfo Role 信息（对应 kubectl get roles -o wide）
type RoleInfo struct {
	Name              string            `json:"name"`
	Namespace         string            `json:"namespace"`
	Age               string            `json:"age"` // 如: "2d5h"
	Labels            map[string]string `json:"labels,omitempty"`
	Annotations       map[string]string `json:"annotations,omitempty"`
	CreationTimestamp int64             `json:"creationTimestamp"` // 时间戳（毫秒）
}

// ListRoleResponse Role 列表响应
type ListRoleResponse struct {
	Total int        `json:"total"`
	Items []RoleInfo `json:"items"`
}

// RoleDescribe Role 详情（类似 kubectl describe）
type RoleDescribe struct {
	Name              string            `json:"name"`
	Namespace         string            `json:"namespace"`
	Labels            map[string]string `json:"labels,omitempty"`
	Annotations       map[string]string `json:"annotations,omitempty"`
	PolicyRules       []PolicyRuleInfo  `json:"policyRules"`
	CreationTimestamp string            `json:"creationTimestamp"`
	Events            []EventInfo       `json:"events,omitempty"`
}

// PolicyRuleInfo 策略规则信息
type PolicyRuleInfo struct {
	APIGroups       []string `json:"apiGroups"`
	Resources       []string `json:"resources"`
	Verbs           []string `json:"verbs"`
	ResourceNames   []string `json:"resourceNames,omitempty"`
	NonResourceURLs []string `json:"nonResourceURLs,omitempty"`
}

// RoleAssociation Role 关联信息
type RoleAssociation struct {
	RoleName     string   `json:"roleName"`
	Namespace    string   `json:"namespace"`
	RoleBindings []string `json:"roleBindings"` // 引用该 Role 的 RoleBinding 列表
	BindingCount int      `json:"bindingCount"` // RoleBinding 数量
	Subjects     []string `json:"subjects"`     // 所有绑定的主体（User/Group/ServiceAccount）
}

// RoleOperator Role 操作器接口
type RoleOperator interface {
	Create(*rbacv1.Role) error
	// ========== 基础查询操作 ==========
	// Get 获取 Role
	Get(namespace, name string) (*rbacv1.Role, error)
	// List 获取 Role 列表，支持搜索和标签过滤
	List(namespace string, search string, labelSelector string) (*ListRoleResponse, error)

	// ========== YAML 操作 ==========
	// GetYaml 获取 Role 的 YAML
	GetYaml(namespace, name string) (string, error)

	// ========== 更新操作 ==========
	// Update 更新 Role（通过 YAML）
	Update(namespace, name string, role *rbacv1.Role) error

	// ========== Describe 操作 ==========
	// Describe 获取 Role 详情（类似 kubectl describe）
	Describe(namespace, name string) (string, error)

	// ========== 删除操作 ==========
	// Delete 删除 Role
	Delete(namespace, name string) error

	// ========== 关联查询 ==========
	// GetAssociation 获取关联信息（RoleBindings 等）
	GetAssociation(namespace, name string) (*RoleAssociation, error)
}
