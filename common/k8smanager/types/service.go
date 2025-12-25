package types

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
)

// ServiceInfo Service 信息
// ServiceInfo Service 信息（更新版）
type ServiceInfo struct {
	Name                  string            `json:"name"`
	Namespace             string            `json:"namespace"`
	Type                  string            `json:"type"`                            // ClusterIP, NodePort, LoadBalancer, ExternalName
	ClusterIP             string            `json:"clusterIP"`                       // CLUSTER-IP
	ExternalIP            string            `json:"externalIP"`                      // EXTERNAL-IP
	Ports                 string            `json:"ports"`                           // PORT(S): "80:30080/TCP,443:30443/TCP"
	Age                   string            `json:"age"`                             // 如: "2d5h"
	CreationTimestamp     int64             `json:"creationTimestamp"`               // 时间戳（毫秒）
	Labels                map[string]string `json:"labels,omitempty"`                // Service 标签
	ClusterIPs            []string          `json:"clusterIPs,omitempty"`            // 多个 ClusterIP
	IpFamilies            []string          `json:"ipFamilies,omitempty"`            // IP 协议族 (IPv4, IPv6)
	IpFamilyPolicy        string            `json:"ipFamilyPolicy,omitempty"`        // IP 协议族策略
	ExternalTrafficPolicy string            `json:"externalTrafficPolicy,omitempty"` // 外部流量策略 (Cluster, Local)
	SessionAffinity       string            `json:"sessionAffinity,omitempty"`       // 会话亲和性 (None, ClientIP)
	LoadBalancerClass     string            `json:"loadBalancerClass,omitempty"`     // 负载均衡器类
	Selector              map[string]string `json:"selector,omitempty"`              // Pod 选择器
	Annotations           map[string]string `json:"annotations,omitempty"`           // 注解
}

// ListServiceResponse Service 列表响应
type ListServiceResponse struct {
	Total int           `json:"total"`
	Items []ServiceInfo `json:"items"`
}

// ServicePortInfo 端口详细信息
type ServicePortInfo struct {
	Name       string `json:"name"`
	Protocol   string `json:"protocol"`           // TCP, UDP, SCTP
	Port       int32  `json:"port"`               // Service 端口
	TargetPort string `json:"targetPort"`         // 目标端口（可以是数字或名称）
	NodePort   int32  `json:"nodePort,omitempty"` // NodePort（仅 NodePort/LoadBalancer 类型）
}

// ServiceDetailInfo Service 详细信息
type ServiceDetailInfo struct {
	ServiceInfo
	Ports              []ServicePortInfo `json:"ports"`
	ExternalIPs        []string          `json:"externalIPs,omitempty"`
	LoadBalancerIP     string            `json:"loadBalancerIP,omitempty"`
	LoadBalancerStatus string            `json:"loadBalancerStatus,omitempty"` // Pending, Available
	EndpointCount      int               `json:"endpointCount"`                // 关联的 Endpoint 数量
}

// ServiceEndpointInfo Service Endpoint 信息
type ServiceEndpointInfo struct {
	PodName   string   `json:"podName"`
	PodIP     string   `json:"podIP"`
	NodeName  string   `json:"nodeName"`
	Ready     bool     `json:"ready"`
	Ports     []int32  `json:"ports"`
	Addresses []string `json:"addresses"` // 所有地址
}

// GetServiceEndpointsResponse 获取 Service Endpoints 响应
type GetServiceEndpointsResponse struct {
	ServiceName string                `json:"serviceName"`
	Namespace   string                `json:"namespace"`
	Endpoints   []ServiceEndpointInfo `json:"endpoints"`
	TotalCount  int                   `json:"totalCount"`
}

// GetResourceServicesRequest 获取资源关联的 Service 请求
type GetResourceServicesRequest struct {
	Namespace    string // 命名空间
	ResourceType string // 资源类型: deployment, statefulset, daemonset, pod
	ResourceName string // 资源名称
}

// ServiceOperator Service 操作器接口
type ServiceOperator interface {
	// ========== 基础 CRUD 操作 ==========
	Create(service *corev1.Service) (*corev1.Service, error)
	Get(namespace, name string) (*corev1.Service, error)
	Update(service *corev1.Service) (*corev1.Service, error)
	Delete(namespace, name string) error
	List(namespace string, search string, labelSelector string) (*ListServiceResponse, error)

	// ========== 高级操作 ==========
	Watch(namespace string, opts metav1.ListOptions) (watch.Interface, error)
	UpdateLabels(namespace, name string, labels map[string]string) error
	UpdateAnnotations(namespace, name string, annotations map[string]string) error

	// ========== YAML 操作 ==========
	GetYaml(namespace, name string) (string, error) // 获取 YAML

	// ========== Service 详情 ==========
	GetDetail(namespace, name string) (*ServiceDetailInfo, error) // 获取详细信息

	// ========== Endpoints 管理 ==========
	// 获取 Service 的所有 Endpoints
	GetEndpoints(namespace, name string) (*GetServiceEndpointsResponse, error)

	// ========== 选择器管理 ==========
	// 更新 Service 的选择器
	UpdateSelector(namespace, name string, selector map[string]string) error
	// 获取选择器匹配的 Pods
	GetMatchingPods(namespace, name string) ([]PodInfo, error)

	// ========== 端口管理 ==========
	// 添加端口
	AddPort(namespace, name string, port ServicePortInfo) error
	// 删除端口
	DeletePort(namespace, name, portName string) error
	// 更新端口
	UpdatePort(namespace, name string, port ServicePortInfo) error

	// ========== 资源关联查询 ==========
	// 获取指定资源关联的所有 Services
	GetResourceServices(req *GetResourceServicesRequest) ([]ServiceInfo, error)
	// 获取通过标签选择器匹配的 Services
	GetServicesByLabels(namespace string, labels map[string]string) ([]ServiceInfo, error)
}
