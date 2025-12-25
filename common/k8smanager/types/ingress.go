package types

import (
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
)

// IngressInfo Ingress 信息
type IngressInfo struct {
	Name              string            `json:"name"`
	Namespace         string            `json:"namespace"`
	IngressClass      string            `json:"ingressClass"` // IngressClass 名称
	Hosts             []string          `json:"hosts"`        // 主机列表
	Address           string            `json:"address"`      // 负载均衡器地址
	Ports             string            `json:"ports"`        // 端口列表，如 "80, 443"
	Labels            map[string]string `json:"labels,omitempty"`
	Annotations       map[string]string `json:"annotations,omitempty"`
	Age               string            `json:"age"`               // 如: "2d5h"
	CreationTimestamp int64             `json:"creationTimestamp"` // 时间戳（毫秒）
}

// ListIngressResponse Ingress 列表响应
type ListIngressResponse struct {
	Total int           `json:"total"`
	Items []IngressInfo `json:"items"`
}

// IngressDetail Ingress 详细信息
type IngressDetail struct {
	Name              string                  `json:"name"`
	Namespace         string                  `json:"namespace"`
	IngressClass      string                  `json:"ingressClass"`
	Labels            map[string]string       `json:"labels,omitempty"`
	Annotations       map[string]string       `json:"annotations,omitempty"`
	Rules             []IngressRuleInfo       `json:"rules"`
	TLS               []IngressTLSInfo        `json:"tls,omitempty"`
	DefaultBackend    *IngressBackendInfo     `json:"defaultBackend,omitempty"`
	LoadBalancer      IngressLoadBalancerInfo `json:"loadBalancer"`
	Age               string                  `json:"age"`
	CreationTimestamp int64                   `json:"creationTimestamp"`
}

// IngressRuleInfo Ingress 规则信息
type IngressRuleInfo struct {
	Host  string            `json:"host"`
	Paths []IngressPathInfo `json:"paths"`
}

// IngressPathInfo Ingress 路径信息
type IngressPathInfo struct {
	Path     string             `json:"path"`
	PathType string             `json:"pathType"` // Exact, Prefix, ImplementationSpecific
	Backend  IngressBackendInfo `json:"backend"`
}

// IngressBackendInfo Ingress 后端信息
type IngressBackendInfo struct {
	ServiceName string              `json:"serviceName"`
	ServicePort string              `json:"servicePort"`           // 可以是端口号或端口名
	ResourceRef *IngressResourceRef `json:"resourceRef,omitempty"` // 资源引用（如果是资源后端）
}

// IngressResourceRef Ingress 资源引用
type IngressResourceRef struct {
	APIGroup string `json:"apiGroup,omitempty"`
	Kind     string `json:"kind"`
	Name     string `json:"name"`
}

// IngressTLSInfo Ingress TLS 信息
type IngressTLSInfo struct {
	Hosts      []string `json:"hosts"`
	SecretName string   `json:"secretName"`
}

// IngressLoadBalancerInfo Ingress 负载均衡器信息
type IngressLoadBalancerInfo struct {
	Ingress []IngressLoadBalancerIngress `json:"ingress,omitempty"`
}

// IngressLoadBalancerIngress 负载均衡器入口信息
type IngressLoadBalancerIngress struct {
	IP       string              `json:"ip,omitempty"`
	Hostname string              `json:"hostname,omitempty"`
	Ports    []IngressPortStatus `json:"ports,omitempty"`
}

// IngressPortStatus 端口状态信息
type IngressPortStatus struct {
	Port     int32  `json:"port"`
	Protocol string `json:"protocol"`
	Error    string `json:"error,omitempty"`
}

// IngressDescribeInfo Ingress 描述信息（包含更详细的信息）
type IngressDescribeInfo struct {
	IngressDetail
	Events   []string        `json:"events,omitempty"`   // 相关事件
	Backends []BackendStatus `json:"backends,omitempty"` // 后端服务状态
}

// BackendStatus 后端服务状态
type BackendStatus struct {
	ServiceName string `json:"serviceName"`
	Endpoints   int    `json:"endpoints"` // 端点数量
	Available   bool   `json:"available"` // 是否可用
	Message     string `json:"message,omitempty"`
}

// IngressOperator Ingress 操作器接口
type IngressOperator interface {
	// ========== 基础 CRUD 操作 ==========
	Create(ingress *networkingv1.Ingress) (*networkingv1.Ingress, error)
	Get(namespace, name string) (*networkingv1.Ingress, error)
	Update(ingress *networkingv1.Ingress) (*networkingv1.Ingress, error)
	Delete(namespace, name string) error
	List(namespace string, search string, labelSelector string) (*ListIngressResponse, error)
	ListByServiceNames(namespace string, serviceNames []string) (*ListIngressResponse, error)
	// ========== 高级操作 ==========
	Watch(namespace string, opts metav1.ListOptions) (watch.Interface, error)
	UpdateLabels(namespace, name string, labels map[string]string) error
	UpdateAnnotations(namespace, name string, annotations map[string]string) error

	// ========== YAML 操作 ==========
	GetYaml(namespace, name string) (string, error)

	// ========== 详细信息查询 ==========
	// 获取详细信息
	GetDetail(namespace, name string) (*IngressDetail, error)
	// 获取描述信息（类似 kubectl describe）
	Describe(namespace, name string) (*IngressDescribeInfo, error)

	// ========== 规则操作 ==========
	// 添加规则
	AddRule(namespace, name string, rule IngressRuleInfo) error
	// 删除规则
	DeleteRule(namespace, name, host string) error
	// 更新规则
	UpdateRule(namespace, name string, rule IngressRuleInfo) error

	// ========== TLS 操作 ==========
	// 添加 TLS 配置
	AddTLS(namespace, name string, tls IngressTLSInfo) error
	// 删除 TLS 配置
	DeleteTLS(namespace, name, secretName string) error
	// 更新 TLS 配置
	UpdateTLS(namespace, name string, tls IngressTLSInfo) error
}
