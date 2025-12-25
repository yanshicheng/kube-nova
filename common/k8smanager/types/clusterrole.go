package types

import (
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
)

// ClusterRoleInfo ClusterRole 信息（对应 kubectl get clusterrole -o wide）
type ClusterRoleInfo struct {
	Name              string            `json:"name"`
	CreatedAt         string            `json:"createdAt"`         // CREATED AT
	RuleCount         int               `json:"ruleCount"`         // 规则数量
	AggregationLabels []string          `json:"aggregationLabels"` // 聚合标签（如果有）
	Labels            map[string]string `json:"labels,omitempty"`
	Annotations       map[string]string `json:"annotations,omitempty"`
	Age               string            `json:"age"`
	CreationTimestamp int64             `json:"creationTimestamp"`
}

// ListClusterRoleResponse ClusterRole 列表响应
type ListClusterRoleResponse struct {
	Total int               `json:"total"`
	Items []ClusterRoleInfo `json:"items"`
}

// ClusterRoleRuleInfo ClusterRole 规则信息
type ClusterRoleRuleInfo struct {
	Verbs           []string `json:"verbs"`           // 动作: get, list, watch, create, update, delete, patch
	APIGroups       []string `json:"apiGroups"`       // API 组: "", apps, batch, etc.
	Resources       []string `json:"resources"`       // 资源: pods, deployments, etc.
	ResourceNames   []string `json:"resourceNames"`   // 资源名称（可选）
	NonResourceURLs []string `json:"nonResourceURLs"` // 非资源 URL（可选）
}

// ClusterRoleBindingRef 绑定到 ClusterRole 的 ClusterRoleBinding 信息
type ClusterRoleBindingRef struct {
	Name     string        `json:"name"`
	Subjects []SubjectInfo `json:"subjects"`
	Age      string        `json:"age"`
}

// SubjectInfo 主体信息
type SubjectInfo struct {
	Kind      string `json:"kind"` // User, Group, ServiceAccount
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"` // 仅 ServiceAccount 有
	APIGroup  string `json:"apiGroup,omitempty"`
}

// ClusterRoleUsageResponse ClusterRole 使用情况响应
type ClusterRoleUsageResponse struct {
	ClusterRoleName     string                  `json:"clusterRoleName"`
	BindingCount        int                     `json:"bindingCount"`        // 被多少个 ClusterRoleBinding 引用
	RoleBindingCount    int                     `json:"roleBindingCount"`    // 被多少个 RoleBinding 引用
	ClusterRoleBindings []ClusterRoleBindingRef `json:"clusterRoleBindings"` // ClusterRoleBinding 列表
	RoleBindings        []RoleBindingRef        `json:"roleBindings"`        // RoleBinding 列表
	TotalSubjects       int                     `json:"totalSubjects"`       // 总主体数
	ServiceAccountCount int                     `json:"serviceAccountCount"` // ServiceAccount 数量
	UserCount           int                     `json:"userCount"`           // User 数量
	GroupCount          int                     `json:"groupCount"`          // Group 数量
	CanDelete           bool                    `json:"canDelete"`
	DeleteWarning       string                  `json:"deleteWarning,omitempty"`
}

// RoleBindingRef RoleBinding 引用信息
type RoleBindingRef struct {
	Name      string        `json:"name"`
	Namespace string        `json:"namespace"`
	Subjects  []SubjectInfo `json:"subjects"`
	Age       string        `json:"age"`
}

// ClusterRoleDescribe ClusterRole 详情
type ClusterRoleDescribe struct {
	Name              string                `json:"name"`
	Labels            map[string]string     `json:"labels,omitempty"`
	Annotations       map[string]string     `json:"annotations,omitempty"`
	Rules             []ClusterRoleRuleInfo `json:"rules"`
	AggregationRule   *AggregationRuleInfo  `json:"aggregationRule,omitempty"`
	CreationTimestamp string                `json:"creationTimestamp"`
}

// AggregationRuleInfo 聚合规则信息
type AggregationRuleInfo struct {
	ClusterRoleSelectors []metav1.LabelSelector `json:"clusterRoleSelectors"`
}

// ClusterRolePermissionSummary ClusterRole 权限摘要
type ClusterRolePermissionSummary struct {
	ClusterRoleName string              `json:"clusterRoleName"`
	HasAllVerbs     bool                `json:"hasAllVerbs"`     // 是否有 * 权限
	HasAllResources bool                `json:"hasAllResources"` // 是否有所有资源权限
	VerbSummary     map[string][]string `json:"verbSummary"`     // 按动作分组的资源
	ResourceSummary map[string][]string `json:"resourceSummary"` // 按资源分组的动作
	IsSuperAdmin    bool                `json:"isSuperAdmin"`    // 是否是超级管理员角色
	RiskLevel       string              `json:"riskLevel"`       // 风险等级: low, medium, high, critical
}

// ClusterRoleOperator ClusterRole 操作器接口
type ClusterRoleOperator interface {
	// ========== 基础 CRUD 操作 ==========
	// Create 创建 ClusterRole
	Create(cr *rbacv1.ClusterRole) (*rbacv1.ClusterRole, error)
	// Get 获取 ClusterRole
	Get(name string) (*rbacv1.ClusterRole, error)
	// Update 更新 ClusterRole
	Update(cr *rbacv1.ClusterRole) (*rbacv1.ClusterRole, error)
	// Delete 删除 ClusterRole
	Delete(name string) error
	// List 获取 ClusterRole 列表，支持搜索和标签过滤
	List(search string, labelSelector string) (*ListClusterRoleResponse, error)

	// ========== YAML 操作 ==========
	// GetYaml 获取 ClusterRole 的 YAML
	GetYaml(name string) (string, error)

	// ========== Describe 操作 ==========
	// Describe 获取 ClusterRole 详情（类似 kubectl describe）
	Describe(name string) (string, error)

	// ========== Watch 操作 ==========
	// Watch 监听 ClusterRole 变化
	Watch(opts metav1.ListOptions) (watch.Interface, error)

	// ========== 高级操作 ==========
	// UpdateLabels 更新标签
	UpdateLabels(name string, labels map[string]string) error
	// UpdateAnnotations 更新注解
	UpdateAnnotations(name string, annotations map[string]string) error
	// AddRule 添加规则
	AddRule(name string, rule rbacv1.PolicyRule) error
	// RemoveRule 移除规则（按索引）
	RemoveRule(name string, ruleIndex int) error
	// UpdateRules 更新所有规则
	UpdateRules(name string, rules []rbacv1.PolicyRule) error

	// ========== 特殊接口：关联查询 ==========
	// GetUsage 获取 ClusterRole 被哪些 ClusterRoleBinding/RoleBinding 引用
	GetUsage(name string) (*ClusterRoleUsageResponse, error)
	// GetPermissionSummary 获取权限摘要分析
	GetPermissionSummary(name string) (*ClusterRolePermissionSummary, error)
	// CanDelete 检查是否可以安全删除
	CanDelete(name string) (bool, string, error)
	// GetByAggregationLabel 根据聚合标签获取 ClusterRole 列表
	GetByAggregationLabel(labelSelector string) (*ListClusterRoleResponse, error)
}
