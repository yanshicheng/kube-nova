package types

import (
	"time"

	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
)

// JobInfo Job 信息
type JobInfo struct {
	Name              string
	Namespace         string
	Completions       int32 // 期望完成的 Pod 数
	Parallelism       int32 // 并行运行的 Pod 数
	Succeeded         int32 // 成功完成的 Pod 数
	Failed            int32 // 失败的 Pod 数
	Active            int32 // 运行中的 Pod 数
	StartTime         *time.Time
	CompletionTime    *time.Time
	Duration          string // 运行时长
	Status            string // Running, Completed, Failed, Suspended
	CreationTimestamp time.Time
}

// ListJobResponse Job 列表响应
type ListJobResponse struct {
	ListResponse
	Items []JobInfo
}

// JobStatusInfo Job 状态详细信息
type JobStatusInfo struct {
	Active         int32      `json:"active"`         // 运行中的 Pod 数
	Succeeded      int32      `json:"succeeded"`      // 成功的 Pod 数
	Failed         int32      `json:"failed"`         // 失败的 Pod 数
	StartTime      *time.Time `json:"startTime"`      // 开始时间
	CompletionTime *time.Time `json:"completionTime"` // 完成时间
	Duration       string     `json:"duration"`       // 运行时长
	Status         string     `json:"status"`         // 状态: Running/Completed/Failed/Suspended
}

// JobParallelismConfig Job 并行度配置（与 API 定义一致）
type JobParallelismConfig struct {
	Parallelism           int32 `json:"parallelism"`                     // 并行执行的 Pod 数
	Completions           int32 `json:"completions"`                     // 需要成功完成的 Pod 数
	BackoffLimit          int32 `json:"backoffLimit"`                    // 失败重试次数
	ActiveDeadlineSeconds int64 `json:"activeDeadlineSeconds,omitempty"` // 超时时间（秒）
}

// UpdateJobParallelismRequest 修改 Job 并行度配置请求（与 API 定义一致）
type UpdateJobParallelismRequest struct {
	Name                  string `json:"name"`
	Namespace             string `json:"namespace"`
	Parallelism           *int32 `json:"parallelism,omitempty"`
	Completions           *int32 `json:"completions,omitempty"`
	BackoffLimit          *int32 `json:"backoffLimit,omitempty"`
	ActiveDeadlineSeconds *int64 `json:"activeDeadlineSeconds,omitempty"`
}

// JobOperator Job 操作器接口
type JobOperator interface {
	// ========== 基础 CRUD 操作 ==========
	Create(*batchv1.Job) (*batchv1.Job, error)
	Get(namespace, name string) (*batchv1.Job, error)
	Update(*batchv1.Job) (*batchv1.Job, error)
	Delete(namespace, name string) error         // 删除 Job（保留 Pods）
	DeleteWithPods(namespace, name string) error // 删除 Job 及其所有 Pods
	List(namespace string, req ListRequest) (*ListJobResponse, error)

	// ========== 高级操作 ==========
	Watch(namespace string, opts metav1.ListOptions) (watch.Interface, error)
	UpdateLabels(namespace, name string, labels map[string]string) error
	UpdateAnnotations(namespace, name string, annotations map[string]string) error

	// ========== YAML 操作 ==========
	GetYaml(namespace, name string) (string, error) // 获取资源 YAML
	GetDescribe(namespace, name string) (string, error)

	// ========== Pod 管理 ==========
	GetPods(namespace, name string) ([]PodDetailInfo, error) // 获取 Job 创建的所有 Pods（带容器信息）

	// ========== 镜像管理 ==========
	GetContainerImages(namespace, name string) (*ContainerInfoList, error) // 查询镜像
	UpdateImage(req *UpdateImageRequest) error                             // 更新单个镜像
	UpdateImages(req *UpdateImagesRequest) error                           // 批量更新镜像

	// ========== Job 状态和配置查询 ==========
	GetStatus(namespace, name string) (*JobStatusInfo, error)                   // 查询任务状态
	GetParallelismConfig(namespace, name string) (*JobParallelismConfig, error) // 查询并行度配置
	UpdateParallelismConfig(req *UpdateJobParallelismRequest) error             // 更新并行度配置

	// ========== 环境变量管理 ==========
	GetEnvVars(namespace, name string) (*EnvVarsResponse, error) // 查询环境变量
	UpdateEnvVars(req *UpdateEnvVarsRequest) error               // 修改环境变量

	// ========== 暂停/恢复更新 ==========
	GetPauseStatus(namespace, name string) (*PauseStatusResponse, error) // 查询暂停状态
	Suspend(namespace, name string) error                                // 挂起 Job（停止创建新 Pod）
	Resume(namespace, name string) error                                 // 恢复 Job

	// ========== 资源配额管理 ==========
	GetResources(namespace, name string) (*ResourcesResponse, error) // 查询资源配额
	UpdateResources(req *UpdateResourcesRequest) error               // 修改资源配额

	// ========== 健康检查管理 ==========
	GetProbes(namespace, name string) (*ProbesResponse, error) // 查询健康检查
	UpdateProbes(req *UpdateProbesRequest) error               // 修改健康检查

	// ========== Job 控制操作 ==========
	Stop(namespace, name string) error                     // 停止 Job（挂起并删除运行中的 Pods）
	Recreate(namespace, name string) (*batchv1.Job, error) // 重新运行（删除旧的，创建新的）- 对应 API 的 run-once

	// ========== 事件 ==========
	GetEvents(namespace, name string) ([]EventInfo, error) // 获取事件

	// ========== 状态检查 ==========
	IsCompleted(namespace, name string) (bool, error) // 检查是否完成
	IsFailed(namespace, name string) (bool, error)    // 检查是否失败
	IsSuspended(namespace, name string) (bool, error) // 检查是否已挂起
	// GetPodLabels 获取 Pod 标签（spec.template.metadata.labels）
	GetPodLabels(namespace, name string) (map[string]string, error)

	// GetPodSelectorLabels 获取 Pod 选择器标签（spec.selector.matchLabels）
	GetPodSelectorLabels(namespace, name string) (map[string]string, error)

	// GetStatus 获取 Job 状态
	GetVersionStatus(namespace, name string) (*ResourceStatus, error)
	// ========== 调度配置管理 ==========
	GetSchedulingConfig(namespace, name string) (*SchedulingConfig, error)
	UpdateSchedulingConfig(namespace, name string, config *UpdateSchedulingConfigRequest) error

	// ========== 存储配置管理 ==========
	GetStorageConfig(namespace, name string) (*StorageConfig, error)
	UpdateStorageConfig(namespace, name string, config *UpdateStorageConfigRequest) error
	GetJobsByCronJob(namespace, cronJobName string) ([]JobInfo, error)
	GetDetail(namespace, name string) (*JobDetailInfo, error)
}

// JobDetailInfo Job 详细信息
type JobDetailInfo struct {
	// 基本信息
	Name              string            `json:"name"`
	Namespace         string            `json:"namespace"`
	Labels            map[string]string `json:"labels,omitempty"`
	Annotations       map[string]string `json:"annotations,omitempty"`
	CreationTimestamp time.Time         `json:"creationTimestamp"`
	UID               string            `json:"uid"`

	// 所属信息
	OwnerReferences []OwnerReference `json:"ownerReferences,omitempty"`

	// Spec 配置
	Parallelism           int32  `json:"parallelism"`                     // 并行度
	Completions           int32  `json:"completions"`                     // 完成数
	BackoffLimit          int32  `json:"backoffLimit"`                    // 重试次数
	ActiveDeadlineSeconds *int64 `json:"activeDeadlineSeconds,omitempty"` // 超时时间
	Suspend               bool   `json:"suspend"`                         // 是否挂起
	TTLSecondsAfterFinish *int32 `json:"ttlSecondsAfterFinish,omitempty"` // 完成后保留时间

	// 状态信息
	Status              string                  `json:"status"` // Running, Completed, Failed, Suspended
	Active              int32                   `json:"active"`
	Succeeded           int32                   `json:"succeeded"`
	Failed              int32                   `json:"failed"`
	StartTime           *time.Time              `json:"startTime,omitempty"`
	CompletionTime      *time.Time              `json:"completionTime,omitempty"`
	Duration            string                  `json:"duration"`
	Conditions          []JobCondition          `json:"conditions,omitempty"`
	UncountedTerminated UncountedTerminatedPods `json:"uncountedTerminated,omitempty"`

	// Pod 模板信息
	PodTemplate PodTemplateInfo `json:"podTemplate"`

	// 关联的 Pods
	Pods []PodDetailInfo `json:"pods,omitempty"`

	// 事件
	Events []EventInfo `json:"events,omitempty"`
}

// OwnerReference 所有者引用
type OwnerReference struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Name       string `json:"name"`
	UID        string `json:"uid"`
	Controller bool   `json:"controller"`
}

// JobCondition Job 状态条件
type JobCondition struct {
	Type               string     `json:"type"`
	Status             string     `json:"status"`
	LastProbeTime      *time.Time `json:"lastProbeTime,omitempty"`
	LastTransitionTime *time.Time `json:"lastTransitionTime,omitempty"`
	Reason             string     `json:"reason,omitempty"`
	Message            string     `json:"message,omitempty"`
}

// UncountedTerminatedPods 未计数的终止 Pods
type UncountedTerminatedPods struct {
	Succeeded []string `json:"succeeded,omitempty"`
	Failed    []string `json:"failed,omitempty"`
}

// PodTemplateInfo Pod 模板信息
type PodTemplateInfo struct {
	Labels           map[string]string     `json:"labels,omitempty"`
	Annotations      map[string]string     `json:"annotations,omitempty"`
	ServiceAccount   string                `json:"serviceAccount,omitempty"`
	NodeSelector     map[string]string     `json:"nodeSelector,omitempty"`
	NodeName         string                `json:"nodeName,omitempty"`
	RestartPolicy    string                `json:"restartPolicy"`
	Containers       []ContainerDetailInfo `json:"containers"`
	InitContainers   []ContainerDetailInfo `json:"initContainers,omitempty"`
	Volumes          []VolumeInfo          `json:"volumes,omitempty"`
	ImagePullSecrets []string              `json:"imagePullSecrets,omitempty"`
}

// ContainerDetailInfo 容器详细信息
type ContainerDetailInfo struct {
	Name            string               `json:"name"`
	Image           string               `json:"image"`
	ImagePullPolicy string               `json:"imagePullPolicy,omitempty"`
	Command         []string             `json:"command,omitempty"`
	Args            []string             `json:"args,omitempty"`
	WorkingDir      string               `json:"workingDir,omitempty"`
	Ports           []ContainerPort      `json:"ports,omitempty"`
	Env             []EnvVar             `json:"env,omitempty"`
	Resources       ResourceRequirements `json:"resources,omitempty"`
	VolumeMounts    []JobVolumeMount     `json:"volumeMounts,omitempty"`
	LivenessProbe   *Probe               `json:"livenessProbe,omitempty"`
	ReadinessProbe  *Probe               `json:"readinessProbe,omitempty"`
	StartupProbe    *Probe               `json:"startupProbe,omitempty"`
}

// ContainerPort 容器端口
type ContainerPort struct {
	Name          string `json:"name,omitempty"`
	ContainerPort int32  `json:"containerPort"`
	Protocol      string `json:"protocol,omitempty"`
	HostPort      int32  `json:"hostPort,omitempty"`
}

// VolumeInfo 卷信息
type VolumeInfo struct {
	Name         string                 `json:"name"`
	VolumeSource map[string]interface{} `json:"volumeSource"`
}

// VolumeMount 卷挂载
type JobVolumeMount struct {
	Name      string `json:"name"`
	MountPath string `json:"mountPath"`
	SubPath   string `json:"subPath,omitempty"`
	ReadOnly  bool   `json:"readOnly,omitempty"`
}
