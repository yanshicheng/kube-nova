package types

// ListRequest 定义列表请求参数
type ListRequest struct {
	// 分页参数
	Page     int `json:"page" form:"page" binding:"min=1"`         // 页码，从1开始
	PageSize int `json:"pageSize" form:"pageSize" binding:"min=1"` // 每页大小

	// 过滤参数
	Search string `json:"search" form:"search"` // 搜索关键字（支持名称模糊匹配）
	Labels string `json:"labels" form:"labels"` // 标签选择器，如 "env=prod,tier=frontend"

	// 排序参数
	SortBy   string `json:"sortBy" form:"sortBy"`     // 排序字段: name, creationTime, status
	SortDesc bool   `json:"sortDesc" form:"sortDesc"` // 是否降序排序
}

// ListResponse 定义列表响应
type ListResponse struct {
	Total      int `json:"total"`      // 总数量
	Page       int `json:"page"`       // 当前页码
	PageSize   int `json:"pageSize"`   // 每页大小
	TotalPages int `json:"totalPages"` // 总页数
}

// ==================== 环境变量相关 ====================

// EnvVarSource 环境变量值来源（与 API 定义一致）
type EnvVarSource struct {
	Type             string                 `json:"type"` // value, configMapKeyRef, secretKeyRef, fieldRef, resourceFieldRef
	Value            string                 `json:"value,omitempty"`
	ConfigMapKeyRef  *ConfigMapKeySelector  `json:"configMapKeyRef,omitempty"`
	SecretKeyRef     *SecretKeySelector     `json:"secretKeyRef,omitempty"`
	FieldRef         *ObjectFieldSelector   `json:"fieldRef,omitempty"`
	ResourceFieldRef *ResourceFieldSelector `json:"resourceFieldRef,omitempty"`
}

type ConfigMapKeySelector struct {
	Name     string `json:"name"`
	Key      string `json:"key"`
	Optional bool   `json:"optional"`
}

type SecretKeySelector struct {
	Name     string `json:"name"`
	Key      string `json:"key"`
	Optional bool   `json:"optional"`
}

type ObjectFieldSelector struct {
	FieldPath string `json:"fieldPath"`
}

type ResourceFieldSelector struct {
	ContainerName string `json:"containerName,omitempty"`
	Resource      string `json:"resource"`
	Divisor       string `json:"divisor,omitempty"`
}

// EnvVar 环境变量定义（与 API 定义一致）
type EnvVar struct {
	Name   string       `json:"name"`
	Source EnvVarSource `json:"source"`
}

// ContainerEnvVars 容器环境变量列表（与 API 定义一致）
type ContainerEnvVars struct {
	ContainerName string   `json:"containerName"`
	Env           []EnvVar `json:"env"`
}

// EnvVarsResponse 查询环境变量响应（与 API 定义一致）
type EnvVarsResponse struct {
	Containers []ContainerEnvVars `json:"containers"`
}

// UpdateEnvVarsRequest 修改环境变量请求（与 API 定义一致）
type UpdateEnvVarsRequest struct {
	Name          string   `json:"name"`
	Namespace     string   `json:"namespace"`
	ContainerName string   `json:"containerName"`
	Env           []EnvVar `json:"env"`
}

// ==================== 资源配额相关 ====================

// ResourceList 资源列表（与 API 定义一致）
type ResourceList struct {
	Cpu    string `json:"cpu,omitempty"`
	Memory string `json:"memory,omitempty"`
}

// ResourceRequirements 资源需求（与 API 定义一致）
type ResourceRequirements struct {
	Limits   ResourceList `json:"limits,omitempty"`
	Requests ResourceList `json:"requests,omitempty"`
}

// ContainerResources 容器资源配额（与 API 定义一致）
type ContainerResources struct {
	ContainerName string               `json:"containerName"`
	Resources     ResourceRequirements `json:"resources"`
}

// ResourcesResponse 查询资源配额响应（与 API 定义一致）
type ResourcesResponse struct {
	Containers []ContainerResources `json:"containers"`
}

// UpdateResourcesRequest 修改资源配额请求（与 API 定义一致）
type UpdateResourcesRequest struct {
	Name          string               `json:"name"`
	Namespace     string               `json:"namespace"`
	ContainerName string               `json:"containerName"`
	Resources     ResourceRequirements `json:"resources"`
}

// ==================== 健康检查相关 ====================

// HTTPHeader HTTP 头
type HTTPHeader struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// HTTPGetAction HTTP 健康检查
type HTTPGetAction struct {
	Path        string       `json:"path"`
	Port        int32        `json:"port"`
	Host        string       `json:"host,omitempty"`
	Scheme      string       `json:"scheme,omitempty"` // HTTP, HTTPS
	HttpHeaders []HTTPHeader `json:"httpHeaders,omitempty"`
}

// TCPSocketAction TCP 健康检查
type TCPSocketAction struct {
	Port int32  `json:"port"`
	Host string `json:"host,omitempty"`
}

// ExecAction 执行命令健康检查
type ExecAction struct {
	Command []string `json:"command"`
}

// GRPCAction gRPC 健康检查
type GRPCAction struct {
	Port    int32  `json:"port"`
	Service string `json:"service,omitempty"`
}

// Probe 健康检查探针
type Probe struct {
	Type                string           `json:"type"` // httpGet, tcpSocket, exec, grpc
	HttpGet             *HTTPGetAction   `json:"httpGet,omitempty"`
	TcpSocket           *TCPSocketAction `json:"tcpSocket,omitempty"`
	Exec                *ExecAction      `json:"exec,omitempty"`
	Grpc                *GRPCAction      `json:"grpc,omitempty"`
	InitialDelaySeconds int32            `json:"initialDelaySeconds,omitempty"`
	TimeoutSeconds      int32            `json:"timeoutSeconds,omitempty"`
	PeriodSeconds       int32            `json:"periodSeconds,omitempty"`
	SuccessThreshold    int32            `json:"successThreshold,omitempty"`
	FailureThreshold    int32            `json:"failureThreshold,omitempty"`
}

// ContainerProbes 容器健康检查
type ContainerProbes struct {
	ContainerName  string `json:"containerName"`
	LivenessProbe  *Probe `json:"livenessProbe,omitempty"`
	ReadinessProbe *Probe `json:"readinessProbe,omitempty"`
	StartupProbe   *Probe `json:"startupProbe,omitempty"`
}

// ProbesResponse 查询健康检查响应
type ProbesResponse struct {
	Containers []ContainerProbes `json:"containers"`
}

// UpdateProbesRequest 修改健康检查请求
type UpdateProbesRequest struct {
	Name           string `json:"name"`
	Namespace      string `json:"namespace"`
	ContainerName  string `json:"containerName"`
	LivenessProbe  *Probe `json:"livenessProbe,omitempty"`
	ReadinessProbe *Probe `json:"readinessProbe,omitempty"`
	StartupProbe   *Probe `json:"startupProbe,omitempty"`
}

// ==================== 更新策略相关 ====================

// RollingUpdateConfig 滚动更新配置（与 API 定义一致）
type RollingUpdateConfig struct {
	MaxUnavailable string `json:"maxUnavailable,omitempty"`
	MaxSurge       string `json:"maxSurge,omitempty"`
	Partition      int32  `json:"partition,omitempty"`
}

// UpdateStrategyResponse 更新策略响应（与 API 定义一致）
type UpdateStrategyResponse struct {
	Type          string               `json:"type"`
	RollingUpdate *RollingUpdateConfig `json:"rollingUpdate,omitempty"`
}

// UpdateStrategyRequest 修改更新策略请求（与 API 定义一致）
type UpdateStrategyRequest struct {
	Name          string               `json:"name"`
	Namespace     string               `json:"namespace"`
	Type          string               `json:"type"`
	RollingUpdate *RollingUpdateConfig `json:"rollingUpdate,omitempty"`
}

// ==================== 暂停/恢复相关 ====================

// PauseStatusResponse 暂停状态响应（与 API 定义一致）
type PauseStatusResponse struct {
	Paused      bool   `json:"paused"`
	SupportType string `json:"supportType"` // pause(Deployment), suspend(Job/CronJob), none
}

// ==================== 版本历史相关 ====================

// RevisionInfo K8s 原生版本历史（与 API 定义一致）
type RevisionInfo struct {
	Revision          int64    `json:"revision"`
	CreationTimestamp int64    `json:"creationTimestamp"`
	Images            []string `json:"images"`
	Replicas          int32    `json:"replicas"`
	Reason            string   `json:"reason"`
}

// ConfigHistoryInfo 配置历史记录（业务层，与 API 定义一致）
type ConfigHistoryInfo struct {
	Id          uint64   `json:"id"`
	Revision    int32    `json:"revision"`
	Images      []string `json:"images"`
	CreatedAt   int64    `json:"createdAt"`
	UpdatedBy   string   `json:"updatedBy"`
	Reason      string   `json:"reason"`
	SpecPreview string   `json:"specPreview"`
	IsCurrent   bool     `json:"isCurrent"`
}

// RollbackToRevisionRequest 回滚到指定版本请求
type RollbackToRevisionRequest struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Revision  int64  `json:"revision"`
}

// RollbackToConfigRequest 回滚到历史配置请求
type RollbackToConfigRequest struct {
	Name            string `json:"name"`
	Namespace       string `json:"namespace"`
	ConfigHistoryId uint64 `json:"configHistoryId"`
}

// ListEventResponse 事件列表响应
type ListEventResponse struct {
	Items []EventInfo
}

// ==================== Pod 相关 ====================
type PodInfo struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Status    string `json:"status"`
	Ready     string `json:"ready"`
	Restarts  int32  `json:"restarts"`
	Age       string `json:"age"`
	Node      string `json:"node"`
	PodIP     string `json:"podIP"`
}

// ==================== 镜像管理相关 ====================

// UpdateImageRequest 更新单个镜像请求
type UpdateImageRequest struct {
	Name          string `json:"name"`
	Namespace     string `json:"namespace"`
	ContainerName string `json:"containerName"`
	Image         string `json:"image"`
}

// UpdateImagesRequest 批量更新镜像请求
type UpdateImagesRequest struct {
	Name       string            `json:"name"`
	Namespace  string            `json:"namespace"`
	Containers ContainerInfoList `json:"containers"`
}
