package types

import (
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
)

// ClusterRoleBindingInfo ClusterRoleBinding 信息（对应 kubectl get clusterrolebindings -o wide）
type ClusterRoleBindingInfo struct {
	Name              string            `json:"name"`
	Role              string            `json:"role"`            // ROLE: ClusterRole/xxx
	Users             string            `json:"users"`           // USERS
	Groups            string            `json:"groups"`          // GROUPS
	ServiceAccounts   string            `json:"serviceAccounts"` // SERVICEACCOUNTS: namespace/name
	SubjectCount      int               `json:"subjectCount"`    // 主体总数
	Labels            map[string]string `json:"labels,omitempty"`
	Annotations       map[string]string `json:"annotations,omitempty"`
	Age               string            `json:"age"`
	CreationTimestamp int64             `json:"creationTimestamp"`
}

// ListClusterRoleBindingResponse ClusterRoleBinding 列表响应
type ListClusterRoleBindingResponse struct {
	Total int                      `json:"total"`
	Items []ClusterRoleBindingInfo `json:"items"`
}

// ClusterRoleBindingSubject ClusterRoleBinding 主体详情
type ClusterRoleBindingSubject struct {
	Kind      string `json:"kind"` // User, Group, ServiceAccount
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"` // 仅 ServiceAccount 有
	APIGroup  string `json:"apiGroup,omitempty"`
}

// ClusterRoleBindingDescribe ClusterRoleBinding 详情
type ClusterRoleBindingDescribe struct {
	Name              string                      `json:"name"`
	Labels            map[string]string           `json:"labels,omitempty"`
	Annotations       map[string]string           `json:"annotations,omitempty"`
	RoleRef           RoleRefInfo                 `json:"roleRef"`
	Subjects          []ClusterRoleBindingSubject `json:"subjects"`
	CreationTimestamp string                      `json:"creationTimestamp"`
}

// RoleRefInfo 角色引用信息
type RoleRefInfo struct {
	Kind     string `json:"kind"` // ClusterRole
	Name     string `json:"name"`
	APIGroup string `json:"apiGroup"` // rbac.authorization.k8s.io
}

// ClusterRoleBindingEffectivePermissions ClusterRoleBinding 实际权限
type ClusterRoleBindingEffectivePermissions struct {
	BindingName     string                `json:"bindingName"`
	RoleName        string                `json:"roleName"`
	RoleExists      bool                  `json:"roleExists"` // 角色是否存在
	Subjects        []SubjectInfo         `json:"subjects"`
	Rules           []ClusterRoleRuleInfo `json:"rules,omitempty"` // 角色规则
	EffectiveScopes []string              `json:"effectiveScopes"` // 有效范围
	IsSuperAdmin    bool                  `json:"isSuperAdmin"`    // 是否授予超级管理员权限
	RiskLevel       string                `json:"riskLevel"`       // 风险等级
}

// ServiceAccountBindingInfo ServiceAccount 绑定信息
type ServiceAccountBindingInfo struct {
	Name         string   `json:"name"`
	Namespace    string   `json:"namespace"`
	BoundRoles   []string `json:"boundRoles"`   // 绑定的 ClusterRole 名称
	BindingNames []string `json:"bindingNames"` // ClusterRoleBinding 名称
}

// SubjectBindingsResponse 主体绑定查询响应
type SubjectBindingsResponse struct {
	SubjectKind           string                   `json:"subjectKind"`
	SubjectName           string                   `json:"subjectName"`
	SubjectNamespace      string                   `json:"subjectNamespace,omitempty"`
	ClusterRoleBindings   []ClusterRoleBindingInfo `json:"clusterRoleBindings"`
	RoleBindings          []RoleBindingInfo        `json:"roleBindings"` // 命名空间内的 RoleBinding
	EffectiveClusterRoles []string                 `json:"effectiveClusterRoles"`
	EffectiveRoles        []NamespacedRoleInfo     `json:"effectiveRoles"`
	TotalBindings         int                      `json:"totalBindings"`
}

// RoleBindingInfo RoleBinding 信息
type RoleBindingInfo struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	RoleName  string `json:"roleName"`
	RoleKind  string `json:"roleKind"` // Role 或 ClusterRole
	Age       string `json:"age"`
}

// NamespacedRoleInfo 命名空间角色信息
type NamespacedRoleInfo struct {
	RoleName  string `json:"roleName"`
	RoleKind  string `json:"roleKind"` // Role 或 ClusterRole
	Namespace string `json:"namespace"`
}

// ClusterRoleBindingOperator ClusterRoleBinding 操作器接口
type ClusterRoleBindingOperator interface {
	// ========== 基础 CRUD 操作 ==========
	// Create 创建 ClusterRoleBinding
	Create(crb *rbacv1.ClusterRoleBinding) (*rbacv1.ClusterRoleBinding, error)
	// Get 获取 ClusterRoleBinding
	Get(name string) (*rbacv1.ClusterRoleBinding, error)
	// Update 更新 ClusterRoleBinding
	Update(crb *rbacv1.ClusterRoleBinding) (*rbacv1.ClusterRoleBinding, error)
	// Delete 删除 ClusterRoleBinding
	Delete(name string) error
	// List 获取 ClusterRoleBinding 列表，支持搜索和标签过滤
	List(search string, labelSelector string) (*ListClusterRoleBindingResponse, error)

	// ========== YAML 操作 ==========
	// GetYaml 获取 ClusterRoleBinding 的 YAML
	GetYaml(name string) (string, error)

	// ========== Describe 操作 ==========
	// Describe 获取 ClusterRoleBinding 详情（类似 kubectl describe）
	Describe(name string) (string, error)

	// ========== Watch 操作 ==========
	// Watch 监听 ClusterRoleBinding 变化
	Watch(opts metav1.ListOptions) (watch.Interface, error)

	// ========== 高级操作 ==========
	// UpdateLabels 更新标签
	UpdateLabels(name string, labels map[string]string) error
	// UpdateAnnotations 更新注解
	UpdateAnnotations(name string, annotations map[string]string) error
	// AddSubject 添加主体
	AddSubject(name string, subject rbacv1.Subject) error
	// RemoveSubject 移除主体
	RemoveSubject(name string, subjectKind, subjectName, subjectNamespace string) error
	// UpdateSubjects 更新所有主体
	UpdateSubjects(name string, subjects []rbacv1.Subject) error

	// ========== 特殊接口：关联查询 ==========
	// GetEffectivePermissions 获取实际授予的权限（包括角色规则）
	GetEffectivePermissions(name string) (*ClusterRoleBindingEffectivePermissions, error)
	// GetByClusterRole 根据 ClusterRole 名称获取绑定列表
	GetByClusterRole(clusterRoleName string) (*ListClusterRoleBindingResponse, error)
	// GetBySubject 根据主体获取绑定列表
	GetBySubject(subjectKind, subjectName, subjectNamespace string) (*SubjectBindingsResponse, error)
	// GetServiceAccountBindings 获取 ServiceAccount 的所有绑定
	GetServiceAccountBindings(namespace, name string) (*SubjectBindingsResponse, error)
	// CanDelete 检查是否可以安全删除
	CanDelete(name string) (bool, string, error)
	// ValidateRoleRef 验证角色引用是否有效（ClusterRole 是否存在）
	ValidateRoleRef(name string) (bool, string, error)
}
