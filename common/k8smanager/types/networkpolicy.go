package types

import (
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
)

// NetworkPolicyInfo NetworkPolicy 信息
type NetworkPolicyInfo struct {
	Name                        string              `json:"name"`
	Namespace                   string              `json:"namespace"`
	PodSelector                 map[string]string   `json:"podSelector"`                           // Pod 选择器标签
	PodSelectorMatchExpressions []NPMatchExpression `json:"podSelectorMatchExpressions,omitempty"` // Pod 选择器表达式
	IngressRules                []NPIngressRuleInfo `json:"ingressRules"`                          // 入站规则
	EgressRules                 []NPEgressRuleInfo  `json:"egressRules"`                           // 出站规则
	PolicyTypes                 []string            `json:"policyTypes"`                           // 策略类型: Ingress, Egress
	Labels                      map[string]string   `json:"labels,omitempty"`
	Annotations                 map[string]string   `json:"annotations,omitempty"`
	Age                         string              `json:"age"`               // 如: "2d5h"
	CreationTimestamp           int64               `json:"creationTimestamp"` // 时间戳（毫秒）
}

// NPMatchExpression NetworkPolicy 标签选择器表达式
type NPMatchExpression struct {
	Key      string   `json:"key"`
	Operator string   `json:"operator"` // In, NotIn, Exists, DoesNotExist
	Values   []string `json:"values,omitempty"`
}

// NPIngressRuleInfo NetworkPolicy 入站规则信息
type NPIngressRuleInfo struct {
	Ports []NPNetworkPolicyPortInfo `json:"ports"` // 端口列表
	From  []NPNetworkPolicyPeerInfo `json:"from"`  // 来源规则
}

// NPEgressRuleInfo NetworkPolicy 出站规则信息
type NPEgressRuleInfo struct {
	Ports []NPNetworkPolicyPortInfo `json:"ports"` // 端口列表
	To    []NPNetworkPolicyPeerInfo `json:"to"`    // 目标规则
}

// NPNetworkPolicyPortInfo NetworkPolicy 端口信息
type NPNetworkPolicyPortInfo struct {
	Protocol  string `json:"protocol,omitempty"`  // TCP, UDP, SCTP
	Port      int32  `json:"port,omitempty"`      // 具体端口
	EndPort   int32  `json:"endPort,omitempty"`   // 端口范围结束(仅部分CNI支持)
	NamedPort string `json:"namedPort,omitempty"` // 命名端口
}

// NPNetworkPolicyPeerInfo NetworkPolicy 对端信息
type NPNetworkPolicyPeerInfo struct {
	PodSelector            map[string]string   `json:"podSelector,omitempty"`            // Pod 选择器
	PodSelectorExprs       []NPMatchExpression `json:"podSelectorExprs,omitempty"`       // Pod 选择器表达式
	NamespaceSelector      map[string]string   `json:"namespaceSelector,omitempty"`      // 命名空间选择器
	NamespaceSelectorExprs []NPMatchExpression `json:"namespaceSelectorExprs,omitempty"` // 命名空间选择器表达式
	IPBlock                *NPIPBlockInfo      `json:"ipBlock,omitempty"`                // IP 块
}

// NPIPBlockInfo NetworkPolicy IP 块信息
type NPIPBlockInfo struct {
	CIDR   string   `json:"cidr"`             // CIDR 格式 IP 段
	Except []string `json:"except,omitempty"` // 排除的 IP
}

// ListNetworkPolicyResponse NetworkPolicy 列表响应
type ListNetworkPolicyResponse struct {
	Total int                 `json:"total"`
	Items []NetworkPolicyInfo `json:"items"`
}

// NetworkPolicyRequest 创建/更新 NetworkPolicy 请求
type NetworkPolicyRequest struct {
	Name        string                `json:"name" validate:"required"`
	Namespace   string                `json:"namespace" validate:"required"`
	Labels      map[string]string     `json:"labels,omitempty"`
	Annotations map[string]string     `json:"annotations,omitempty"`
	PodSelector NPLabelSelectorConfig `json:"podSelector"` // Pod 选择器
	PolicyTypes []string              `json:"policyTypes"` // Ingress, Egress 或两者
	Ingress     []NPIngressRuleConfig `json:"ingress,omitempty"`
	Egress      []NPEgressRuleConfig  `json:"egress,omitempty"`
}

// NPLabelSelectorConfig NetworkPolicy 标签选择器配置
type NPLabelSelectorConfig struct {
	MatchLabels      map[string]string   `json:"matchLabels,omitempty"`
	MatchExpressions []NPMatchExpression `json:"matchExpressions,omitempty"`
}

// NPIngressRuleConfig NetworkPolicy 入站规则配置
type NPIngressRuleConfig struct {
	Ports []NPNetworkPolicyPortConfig `json:"ports,omitempty"`
	From  []NPNetworkPolicyPeerConfig `json:"from,omitempty"`
}

// NPEgressRuleConfig NetworkPolicy 出站规则配置
type NPEgressRuleConfig struct {
	Ports []NPNetworkPolicyPortConfig `json:"ports,omitempty"`
	To    []NPNetworkPolicyPeerConfig `json:"to,omitempty"`
}

// NPNetworkPolicyPortConfig NetworkPolicy 端口配置
type NPNetworkPolicyPortConfig struct {
	Protocol  string `json:"protocol,omitempty"`  // TCP, UDP, SCTP
	Port      int32  `json:"port,omitempty"`      // 具体端口
	EndPort   int32  `json:"endPort,omitempty"`   // 端口范围结束
	NamedPort string `json:"namedPort,omitempty"` // 命名端口
}

// NPNetworkPolicyPeerConfig NetworkPolicy 对端配置
type NPNetworkPolicyPeerConfig struct {
	PodSelector       *NPLabelSelectorConfig `json:"podSelector,omitempty"`
	NamespaceSelector *NPLabelSelectorConfig `json:"namespaceSelector,omitempty"`
	IPBlock           *NPIPBlockConfig       `json:"ipBlock,omitempty"`
}

// NPIPBlockConfig NetworkPolicy IP 块配置
type NPIPBlockConfig struct {
	CIDR   string   `json:"cidr" validate:"required"`
	Except []string `json:"except,omitempty"`
}

// GetNetworkPolicyYamlResponse 获取 YAML 响应
type GetNetworkPolicyYamlResponse struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Yaml      string `json:"yaml"`
}

// NetworkPolicyAffectedPod 受 NetworkPolicy 影响的 Pod
type NetworkPolicyAffectedPod struct {
	Name      string            `json:"name"`
	Namespace string            `json:"namespace"`
	Labels    map[string]string `json:"labels"`
	Matched   bool              `json:"matched"` // 是否匹配 NetworkPolicy
	Reason    string            `json:"reason"`  // 匹配原因
}

// NetworkPolicyAffectedPodsResponse 受影响的 Pod 响应
type NetworkPolicyAffectedPodsResponse struct {
	NetworkPolicyName string                     `json:"networkPolicyName"`
	NetworkPolicyNS   string                     `json:"networkPolicyNamespace"`
	AffectedPods      []NetworkPolicyAffectedPod `json:"affectedPods"`
	TotalAffected     int                        `json:"totalAffected"`
}

// NetworkPolicyOperator NetworkPolicy 操作器接口
type NetworkPolicyOperator interface {
	// ========== 基础 CRUD 操作 ==========
	Create(np *networkingv1.NetworkPolicy) (*networkingv1.NetworkPolicy, error)
	Get(namespace, name string) (*networkingv1.NetworkPolicy, error)
	Update(np *networkingv1.NetworkPolicy) (*networkingv1.NetworkPolicy, error)
	Delete(namespace, name string) error
	List(namespace string, search string, labelSelector string) (*ListNetworkPolicyResponse, error)

	// ========== 高级操作 ==========
	Watch(namespace string, opts metav1.ListOptions) (watch.Interface, error)
	UpdateLabels(namespace, name string, labels map[string]string) error
	UpdateAnnotations(namespace, name string, annotations map[string]string) error

	// ========== YAML 操作 ==========
	GetYaml(namespace, name string) (string, error)

	// ========== 关联查询 ==========
	// 获取受此 NetworkPolicy 影响的 Pod
	GetAffectedPods(namespace, name string) (*NetworkPolicyAffectedPodsResponse, error)
}
