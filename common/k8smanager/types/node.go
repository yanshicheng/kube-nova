package types

import "strings"

// NodeInfo 节点信息
type NodeInfo struct {
	NodeName        string  `json:"nodeName"`
	NodeUuid        string  `json:"nodeUuid"` // systemUUID
	NodeIp          string  `json:"nodeIp"`
	HostName        string  `json:"hostName"`
	Roles           string  `json:"roles"` // 逗号分割角色列表
	OsImage         string  `json:"osImage"`
	KernelVersion   string  `json:"kernelVersion"`
	OperatingSystem string  `json:"operatingSystem"`
	Architecture    string  `json:"architecture"`
	Cpu             float64 `json:"cpu"`
	Memory          int64   `json:"memory"`
	IsGpu           int64   `json:"isGpu"`
	Runtime         string  `json:"runtime"`
	JoinAt          int64   `json:"joinAt"`
	Unschedulable   int64   `json:"unschedulable"` // 1 可调度 2 不可调度
	KubeletVersion  string  `json:"kubeletVersion"`
	Taints          string  `json:"taints"`
	Status          string  `json:"status"`
	PodCidr         string  `json:"podCidr"`
	PodCidrs        string  `json:"podCidrs"`
	Pods            int64   `json:"pods"`
	PodsUsage       int64   `json:"podsUsage"`
}

// NodeListRequest List方法的请求参数
type NodeListRequest struct {
	Page       uint64 `form:"page" default:"1" validate:"required,min=1"`                  // 当前页码
	PageSize   uint64 `form:"pageSize" default:"10" validate:"required,min=1,max=200"`     // 每页条数
	OrderField string `form:"orderField,optional" default:"nodeName" validate:"omitempty"` // 排序字段
	IsAsc      bool   `form:"isAsc,optional" default:"false" validate:"omitempty"`         // 是否升序
	Search     string `form:"search,optional" default:"" validate:"omitempty"`             // 搜索关键词
}

// NodeListResponse List方法的响应
type NodeListResponse struct {
	Total uint64      `json:"total"` // 总条数
	Items []*NodeInfo `json:"items"` // 节点列表
}

// ClusterResourceTotal 集群资源总量
type ClusterResourceTotal struct {
	TotalCpu    float64 `json:"totalCpu"`    // CPU总核数
	TotalMemory float64 `json:"totalMemory"` // 内存总量(GiB)
	TotalNodes  int     `json:"totalNodes"`  // 节点总数
	TotalPods   int64   `json:"totalPods"`
}

type TaintInfo struct {
	Key      string `json:"key"`
	Value    string `json:"value"`
	Effect   string `json:"effect"`
	Time     string `json:"time"`
	IsDelete bool   `json:"isDelete"`
}

type LabelsInfo struct {
	Key      string `json:"key"`
	Value    string `json:"value"`
	IsDelete bool   `json:"is_delete"`
}

type NodeOperator interface {
	// List 获取节点列表（支持搜索、分页、排序）
	List(req *NodeListRequest) (*NodeListResponse, error)

	// GetClusterResourceTotal 获取集群资源总量（CPU和内存）
	GetClusterResourceTotal() (*ClusterResourceTotal, error)

	// GetNodeDetail 获取单个节点详情
	GetNodeDetail(nodeName string) (*NodeInfo, error)

	Describe(nodeName string) (string, error)

	// AddLabels 主机添加labels
	AddLabels(nodeName, key, value string) error
	// DeleteLabels 主机删除labels
	DeleteLabels(nodeName string, key string) error
	// GetLabels 查询labels
	GetLabels(nodeName string) ([]*LabelsInfo, error)

	// DeleteAnnotations 删除注解
	DeleteAnnotations(nodeName string, key string) error
	// AddAnnotations 主机添加注解
	AddAnnotations(nodeName, key, value string) error
	// GetAnnotations 查询注解
	GetAnnotations(nodeName string) ([]*LabelsInfo, error)

	// DisableScheduling 禁用调度
	DisableScheduling(nodeName string) error
	// EnableScheduling 启用调度
	EnableScheduling(nodeName string) error

	// DrainNode 驱逐节点
	DrainNode(nodeName string) error
	// AddTaint 增加污点
	AddTaint(nodeName, key, value, effect string) error
	// DeleteTaint 删除污点
	DeleteTaint(nodeName, key, effect string) error
	// GetTaints 查询污点
	GetTaints(nodeName string) ([]*TaintInfo, error)
}

// IsSystemLabel 判断标签键是否为 Kubernetes 系统标签
func IsSystemLabel(key string) bool {
	// 定义系统标签前缀列表
	systemLabelPrefixes := []string{
		"beta.kubernetes.io/arch",
		"beta.kubernetes.io/os",
		"kubernetes.io/arch",
		"kubernetes.io/hostname",
		"kubernetes.io/os",
		"kubernetes.io/role",
		"node-role.kubernetes.io/", // 节点角色相关
		"node.kubernetes.io/",      // 节点状态相关
	}

	// 检查是否匹配任一前缀
	for _, prefix := range systemLabelPrefixes {
		if strings.HasPrefix(key, prefix) {
			return true
		}
	}
	return false
}

// IsSystemAnnotation 判断注解键是否为 Kubernetes 系统注解
func IsSystemAnnotation(key string) bool {
	systemAnnotationPrefixes := []string{
		"node.alpha.kubernetes.io/ttl",
		"csi.volume.kubernetes.io/nodeid",
		"volumes.kubernetes.io/controller-managed-attach-detach",
		"projectcalico.org/IPv4Address",
		"projectcalico.org/IPv4VXLANTunnelAddr",
		"projectcalico.org/IPv6Address",
		"projectcalico.org/Interfaces",
	}
	for _, prefix := range systemAnnotationPrefixes {
		if strings.HasPrefix(key, prefix) {
			return true
		}
	}
	return false
}

// IsSystemTaint 判断污点键是否为 Kubernetes 系统污点
func IsSystemTaint(key string) bool {
	systemTaintPrefixes := []string{
		"node.kubernetes.io/unschedulable",
		"node-role.kubernetes.io/control-plane",
	}
	for _, prefix := range systemTaintPrefixes {
		if strings.HasPrefix(key, prefix) {
			return true
		}
	}
	return false
}
