package types

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ========== 通用请求响应结构 ==========

// Pagination 分页选项
type Pagination struct {
	Page     int    `json:"page"`     // 页码
	PageSize int    `json:"pageSize"` // 每页大小
	Continue string `json:"continue"` // 继续标记
}

// SortOptions 排序选项
type SortOptions struct {
	Field string `json:"field"` // 排序字段
	Order string `json:"order"` // 排序顺序 (asc, desc)
}

// ========== Pod 列表相关 ==========

// PodListRequest Pod列表请求
type PodListRequest struct {
	ListRequest
	Namespace string `json:"namespace"` // 如果为空则搜索全部
	Name      string `json:"name"`      // 模糊匹配
}

// PodListResponse Pod列表响应（单项）
type PodListResponse struct {
	ListResponse
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Ready     string `json:"ready"`
	Status    string `json:"status"`
	Restarts  string `json:"restarts"`
	Age       string `json:"age"`
	Ip        string `json:"ip"`
	Node      string `json:"node"`
	CreateAt  string `json:"createAt"`
}

// ListPodResponse Pod列表响应（完整）
type ListPodResponse struct {
	ListResponse
	Items []PodDetailInfo `json:"items"`
}

// ========== 容器状态结构（放在最前面） ==========

// ContainerStateInfo 容器状态信息（简化版本）
type ContainerStateInfo struct {
	Type       string                    `json:"type"` // Running, Waiting, Terminated
	Running    *ContainerStateRunning    `json:"running,omitempty"`
	Waiting    *ContainerStateWaiting    `json:"waiting,omitempty"`
	Terminated *ContainerStateTerminated `json:"terminated,omitempty"`
}

// ContainerStateRunning 运行状态
type ContainerStateRunning struct {
	StartedAt int64 `json:"startedAt"` // 毫秒时间戳
}

// ContainerStateWaiting 等待状态
type ContainerStateWaiting struct {
	Reason  string `json:"reason,omitempty"`
	Message string `json:"message,omitempty"`
}

// ContainerStateTerminated 终止状态
type ContainerStateTerminated struct {
	ExitCode   int32  `json:"exitCode"`
	Reason     string `json:"reason,omitempty"`
	Message    string `json:"message,omitempty"`
	StartedAt  int64  `json:"startedAt,omitempty"`  // 毫秒时间戳
	FinishedAt int64  `json:"finishedAt,omitempty"` // 毫秒时间戳
}

// ========== Pod 详情相关 ==========

// PodDetailInfo Pod详细信息
type PodDetailInfo struct {
	Name         string            `json:"name"`
	Namespace    string            `json:"namespace"`
	Status       string            `json:"status"`
	Ready        string            `json:"ready"`
	Restarts     int32             `json:"restarts"`
	Age          string            `json:"age"`
	Node         string            `json:"node"`
	PodIP        string            `json:"podIP"`
	Labels       map[string]string `json:"labels"`
	CreationTime int64             `json:"creationTime"` // 毫秒时间戳
}

// ContainerInfoList 容器信息列表
type ContainerInfoList struct {
	InitContainers      []ContainerInfo `json:"initContainers"`
	Containers          []ContainerInfo `json:"containers"`
	EphemeralContainers []ContainerInfo `json:"ephemeralContainers,omitempty"`
}

// ContainerInfo 容器信息
type ContainerInfo struct {
	Name         string                  `json:"name"`
	Image        string                  `json:"image"`
	Ready        bool                    `json:"ready"`
	RestartCount int32                   `json:"restartCount"`
	State        string                  `json:"state"` // Running, Waiting, Terminated
	Status       *ContainerStatusDetails `json:"status,omitempty"`
}

// ContainerStatusDetails 容器状态详情（简化版本）- 现在可以安全使用 ContainerStateInfo
type ContainerStatusDetails struct {
	State        ContainerStateInfo `json:"state"`
	LastState    ContainerStateInfo `json:"lastState,omitempty"`
	Ready        bool               `json:"ready"`
	RestartCount int32              `json:"restartCount"`
	Image        string             `json:"image"`
	ImageID      string             `json:"imageID"`
	ContainerID  string             `json:"containerID,omitempty"`
	Started      *bool              `json:"started,omitempty"`
}

// ========== Pod 资源查询 ==========

// GetPodsRequest 获取 Pods 请求
type GetPodsRequest struct {
	Namespace     string // 命名空间
	ResourceType  string // Pod, Deployment, StatefulSet, DaemonSet, ReplicaSet, Job
	ResourceName  string // 资源名称（ResourceType=Pod时可为空，表示获取所有裸Pod）
	LabelSelector string
}

// GetResourcePodsDetailRequest 获取资源 Pods 详情请求
type GetResourcePodsDetailRequest struct {
	Namespace    string // 命名空间
	ResourceType string // 资源类型: pod, deployment, statefulset/sts, daemonset, job, cronjob
	ResourceName string // 资源名称（可选，为空则返回该类型的所有 pods）
}

// ========== 事件和状态 ==========

// EventInfo 事件信息

// ResourceStatus 资源状态
type ResourceStatus struct {
	Status  StatusType `json:"status"`  // 状态类型
	Ready   bool       `json:"ready"`   // 是否就绪
	Message string     `json:"message"` // 状态消息
}

// StatusType 状态类型
type StatusType string

const (
	StatusCreating StatusType = "Creating" // 创建中
	StatusRunning  StatusType = "Running"  // 运行中
	StatusStopping StatusType = "Stopping" // 停止中
	StatusStopped  StatusType = "Stopped"  // 已停止
	StatusError    StatusType = "Error"    // 异常
)

// ========== 临时容器注入 ==========

// InjectEphemeralContainerRequest 注入临时容器请求
type InjectEphemeralContainerRequest struct {
	Namespace      string   `json:"namespace"`      // 命名空间
	PodName        string   `json:"podName"`        // Pod名称
	ContainerName  string   `json:"containerName"`  // 临时容器名称
	Image          string   `json:"image"`          // 镜像
	Command        []string `json:"command"`        // 命令
	Args           []string `json:"args"`           // 参数
	TargetPID      bool     `json:"targetPID"`      // 是否共享目标进程命名空间
	TargetNetwork  bool     `json:"targetNetwork"`  // 是否共享网络命名空间
	TargetIPC      bool     `json:"targetIPC"`      // 是否共享IPC命名空间
	Stdin          bool     `json:"stdin"`          // 是否分配标准输入
	TTY            bool     `json:"tty"`            // 是否分配TTY
	Env            []string `json:"env"`            // 环境变量
	WorkingDir     string   `json:"workingDir"`     // 工作目录
	VolumeMounts   []string `json:"volumeMounts"`   // 卷挂载
	Capabilities   []string `json:"capabilities"`   // 能力
	Privileged     bool     `json:"privileged"`     // 是否特权模式
	RunAsUser      *int64   `json:"runAsUser"`      // 运行用户ID
	RunAsGroup     *int64   `json:"runAsGroup"`     // 运行组ID
	ReadOnlyRootFS bool     `json:"readOnlyRootFS"` // 只读根文件系统
}

// ========== Pod 操作器接口 ==========

// PodOperator Pod 操作器接口
type PodOperator interface {
	// ========== 基础 CRUD 操作 ==========
	Get(namespace, name string) (*corev1.Pod, error)
	List(namespace string, req ListRequest) (*ListPodResponse, error)
	GetYaml(namespace, name string) (string, error)
	GetDescribe(namespace, name string) (string, error)

	Delete(namespace, name string) error
	BatchDelete(namespace string, names []string, opts metav1.DeleteOptions) (succeeded int, failed int, errors []error)
	DeleteBySelector(namespace string, labelSelector string, opts metav1.DeleteOptions) error
	Create(*corev1.Pod) (*corev1.Pod, error)
	Update(*corev1.Pod) (*corev1.Pod, error)

	// ========== 资源查询 ==========
	GetResourcePodsDetailList(req *GetResourcePodsDetailRequest) ([]PodDetailInfo, error)
	GetResourcePods(req *GetPodsRequest) ([]PodDetailInfo, error)
	GetStandalonePods(namespace string) ([]PodDetailInfo, error)

	// ========== 容器操作 ==========
	GetContainers(namespace, name string) ([]corev1.Container, error)
	GetDefaultContainer(namespace, name string) (string, error)
	GetAllContainers(namespace, name string) (*ContainerInfoList, error)

	// ========== 状态检查 ==========
	IsReady(namespace, name string) (bool, error)
	GetPhase(namespace, name string) (corev1.PodPhase, error)
	IsRunning(namespace, name string) (bool, error)
	IsPending(namespace, name string) (bool, error)
	IsSucceeded(namespace, name string) (bool, error)
	IsFailed(namespace, name string) (bool, error)
	IsTerminating(namespace, name string) (bool, error)

	// ========== 高级操作 ==========
	Evict(namespace, name string) error
	UpdateLabels(namespace, name string, labels map[string]string) error
	UpdateAnnotations(namespace, name string, annotations map[string]string) error
	InjectEphemeralContainer(req *InjectEphemeralContainerRequest) error

	// ========== 标签和状态 ==========
	GetPodLabels(namespace, name string) (map[string]string, error)
	GetPodSelectorLabels(namespace, name string) (map[string]string, error)
	GetVersionStatus(namespace, name string) (*ResourceStatus, error)
	GetPods(namespace, name string) ([]PodDetailInfo, error)
	GetEvents(namespace, name string) ([]EventInfo, error)

	// ========== Pod 日志操作 ==========
	PodLogOperator

	// ========== Pod 执行命令操作 ==========
	PodExecOperator

	// ========== Pod 文件操作 ==========
	PodFileOperator
}
