package types

import (
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
)

// IngressClassInfo IngressClass 信息（对应 kubectl get ingressclasses -o wide）
type IngressClassInfo struct {
	Name              string            `json:"name"`
	Controller        string            `json:"controller"` // CONTROLLER: k8s.io/ingress-nginx
	Parameters        string            `json:"parameters"` // PARAMETERS: <none> 或 Kind/Name
	IsDefault         bool              `json:"isDefault"`  // 是否为默认 IngressClass
	Labels            map[string]string `json:"labels,omitempty"`
	Annotations       map[string]string `json:"annotations,omitempty"`
	Age               string            `json:"age"`
	CreationTimestamp int64             `json:"creationTimestamp"`
}

// ListIngressClassResponse IngressClass 列表响应
type ListIngressClassResponse struct {
	Total int                `json:"total"`
	Items []IngressClassInfo `json:"items"`
}

// IngressClassParametersInfo IngressClass 参数信息
type IngressClassParametersInfo struct {
	APIGroup  string `json:"apiGroup,omitempty"`
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"` // 仅当 Scope 为 Namespace 时
	Scope     string `json:"scope,omitempty"`     // Cluster 或 Namespace
}

// IngressRefInfo 使用该 IngressClass 的 Ingress 信息
type IngressRefInfo struct {
	Name         string   `json:"name"`
	Namespace    string   `json:"namespace"`
	Hosts        []string `json:"hosts"`        // 主机名列表
	Address      string   `json:"address"`      // 地址
	TLSEnabled   bool     `json:"tlsEnabled"`   // 是否启用 TLS
	RulesCount   int      `json:"rulesCount"`   // 规则数量
	BackendCount int      `json:"backendCount"` // 后端数量
	Age          string   `json:"age"`
}

// IngressClassUsageResponse IngressClass 使用情况响应
type IngressClassUsageResponse struct {
	IngressClassName string           `json:"ingressClassName"`
	IsDefault        bool             `json:"isDefault"`
	IngressCount     int              `json:"ingressCount"`   // 使用该 IngressClass 的 Ingress 数量
	NamespaceCount   int              `json:"namespaceCount"` // 涉及的命名空间数量
	Ingresses        []IngressRefInfo `json:"ingresses"`      // Ingress 列表
	NamespaceStats   map[string]int   `json:"namespaceStats"` // 每个命名空间的 Ingress 数量
	CanDelete        bool             `json:"canDelete"`
	DeleteWarning    string           `json:"deleteWarning,omitempty"`
}

// IngressClassDescribe IngressClass 详情
type IngressClassDescribe struct {
	Name              string                      `json:"name"`
	Labels            map[string]string           `json:"labels,omitempty"`
	Annotations       map[string]string           `json:"annotations,omitempty"`
	Controller        string                      `json:"controller"`
	Parameters        *IngressClassParametersInfo `json:"parameters,omitempty"`
	IsDefault         bool                        `json:"isDefault"`
	CreationTimestamp string                      `json:"creationTimestamp"`
	Events            []EventInfo                 `json:"events,omitempty"`
}

// IngressClassControllerStatus IngressClass 控制器状态
type IngressClassControllerStatus struct {
	IngressClassName   string   `json:"ingressClassName"`
	ControllerName     string   `json:"controllerName"`
	ControllerPods     []string `json:"controllerPods,omitempty"`     // 控制器 Pod 名称
	ControllerReady    bool     `json:"controllerReady"`              // 控制器是否就绪
	ControllerReplicas int      `json:"controllerReplicas,omitempty"` // 控制器副本数
	Namespace          string   `json:"namespace,omitempty"`          // 控制器所在命名空间
}

// IngressClassOperator IngressClass 操作器接口
type IngressClassOperator interface {
	// ========== 基础 CRUD 操作 ==========
	// Create 创建 IngressClass
	Create(ic *networkingv1.IngressClass) (*networkingv1.IngressClass, error)
	// Get 获取 IngressClass
	Get(name string) (*networkingv1.IngressClass, error)
	// Update 更新 IngressClass
	Update(ic *networkingv1.IngressClass) (*networkingv1.IngressClass, error)
	// Delete 删除 IngressClass
	Delete(name string) error
	// List 获取 IngressClass 列表，支持搜索和标签过滤
	List(search string, labelSelector string) (*ListIngressClassResponse, error)

	// ========== YAML 操作 ==========
	// GetYaml 获取 IngressClass 的 YAML
	GetYaml(name string) (string, error)

	// ========== Describe 操作 ==========
	// Describe 获取 IngressClass 详情（类似 kubectl describe）
	Describe(name string) (string, error)

	// ========== Watch 操作 ==========
	// Watch 监听 IngressClass 变化
	Watch(opts metav1.ListOptions) (watch.Interface, error)

	// ========== 高级操作 ==========
	// UpdateLabels 更新标签
	UpdateLabels(name string, labels map[string]string) error
	// UpdateAnnotations 更新注解
	UpdateAnnotations(name string, annotations map[string]string) error
	// SetDefault 设置为默认 IngressClass
	SetDefault(name string) error
	// UnsetDefault 取消默认 IngressClass
	UnsetDefault(name string) error

	// ========== 特殊接口：关联查询 ==========
	// GetUsage 获取使用该 IngressClass 的 Ingress 列表
	GetUsage(name string) (*IngressClassUsageResponse, error)
	// GetIngressesByClass 根据 IngressClass 获取所有 Ingress
	GetIngressesByClass(name string, namespace string) ([]IngressRefInfo, error)
	// CanDelete 检查是否可以安全删除（是否有 Ingress 使用）
	CanDelete(name string) (bool, string, error)
	// GetControllerStatus 获取控制器状态（需要知道控制器的部署信息）
	GetControllerStatus(name string) (*IngressClassControllerStatus, error)
	// GetDefaultIngressClass 获取默认 IngressClass
	GetDefaultIngressClass() (*networkingv1.IngressClass, error)
}
