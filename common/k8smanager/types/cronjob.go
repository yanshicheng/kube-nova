package types

import (
	"time"

	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
)

// CronJobInfo CronJob 信息
type CronJobInfo struct {
	Name               string
	Namespace          string
	Schedule           string     // Cron 表达式
	Timezone           string     // 时区
	Suspend            bool       // 是否挂起
	Active             int        // 当前运行中的 Job 数量
	LastScheduleTime   *time.Time // 上次调度时间
	NextScheduleTime   *time.Time // 下次调度时间（计算值）
	LastSuccessfulTime *time.Time // 上次成功时间
	CreationTimestamp  time.Time
	Images             []string
}

// ListCronJobResponse CronJob 列表响应
type ListCronJobResponse struct {
	ListResponse
	Items []CronJobInfo
}

// CronJobScheduleConfig CronJob 调度配置
type CronJobScheduleConfig struct {
	Schedule                   string `json:"schedule"`           // Cron 表达式
	Timezone                   string `json:"timezone,omitempty"` // 时区
	ConcurrencyPolicy          string `json:"concurrencyPolicy"`  // Allow, Forbid, Replace
	Suspend                    bool   `json:"suspend"`            // 是否暂停
	StartingDeadlineSeconds    int64  `json:"startingDeadlineSeconds,omitempty"`
	SuccessfulJobsHistoryLimit int32  `json:"successfulJobsHistoryLimit"` // 成功历史保留数
	FailedJobsHistoryLimit     int32  `json:"failedJobsHistoryLimit"`     // 失败历史保留数
}

// UpdateCronJobScheduleRequest 修改 CronJob 调度配置请求
type UpdateCronJobScheduleRequest struct {
	Name                       string `json:"name"`
	Namespace                  string `json:"namespace"`
	Schedule                   string `json:"schedule,omitempty"`
	Timezone                   string `json:"timezone,omitempty"`
	ConcurrencyPolicy          string `json:"concurrencyPolicy,omitempty"`
	StartingDeadlineSeconds    *int64 `json:"startingDeadlineSeconds,omitempty"`
	SuccessfulJobsHistoryLimit *int32 `json:"successfulJobsHistoryLimit,omitempty"`
	FailedJobsHistoryLimit     *int32 `json:"failedJobsHistoryLimit,omitempty"`
}

// CronJobHistoryItem CronJob 历史 Job 项（与 API 定义一致）
type CronJobHistoryItem struct {
	Name              string `json:"name"`
	Status            string `json:"status"` // Complete, Failed, Running
	Completions       int32  `json:"completions"`
	Succeeded         int32  `json:"succeeded"`
	Active            int32  `json:"active"`
	Failed            int32  `json:"failed"`
	Duration          string `json:"duration,omitempty"`
	CreationTimestamp int64  `json:"creationTimestamp"`
	StartTime         int64  `json:"startTime"`
	CompletionTime    int64  `json:"completionTime,omitempty"`
}

// CronJobHistoryResponse CronJob 历史响应（与 API 定义一致）
type CronJobHistoryResponse struct {
	Jobs []CronJobHistoryItem `json:"jobs"`
}

// TriggerCronJobRequest 手动触发 Job 请求
type TriggerCronJobRequest struct {
	CronJobName string `json:"cronJobName"` // CronJob 名称
	Namespace   string `json:"namespace"`   // 命名空间
	JobName     string `json:"jobName"`     // Job 名称（可选，为空则自动生成）
}

// ==================== Job Spec 配置相关 ====================

// PodFailurePolicyOnExitCodesRequirement Pod 失败策略退出码要求
type PodFailurePolicyOnExitCodesRequirement struct {
	ContainerName *string `json:"containerName,optional"` // 容器名称（指针类型），为空表示所有容器
	Operator      string  `json:"operator"`               // In, NotIn
	Values        []int32 `json:"values"`                 // 退出码列表
}

// PodFailurePolicyOnPodConditionsPattern Pod 失败策略条件模式
type PodFailurePolicyOnPodConditionsPattern struct {
	Type   string `json:"type"`   // Pod 条件类型
	Status string `json:"status"` // 条件状态
}

// PodFailurePolicyRule Pod 失败策略规则
type PodFailurePolicyRule struct {
	Action          string                                   `json:"action"` // FailJob, Ignore, Count, FailIndex
	OnExitCodes     *PodFailurePolicyOnExitCodesRequirement  `json:"onExitCodes,optional"`
	OnPodConditions []PodFailurePolicyOnPodConditionsPattern `json:"onPodConditions,optional"`
}

// PodFailurePolicyConfig Pod 失败策略配置
type PodFailurePolicyConfig struct {
	Rules []PodFailurePolicyRule `json:"rules"`
}

// JobSpecConfig Job 规格完整配置
type JobSpecConfig struct {
	// 基础配置（K8s 1.0+）
	Parallelism             *int32 `json:"parallelism,optional"`             // 并行度
	Completions             *int32 `json:"completions,optional"`             // 完成数
	BackoffLimit            *int32 `json:"backoffLimit,optional"`            // 失败重试次数
	ActiveDeadlineSeconds   *int64 `json:"activeDeadlineSeconds,optional"`   // 活跃截止时间
	TTLSecondsAfterFinished *int32 `json:"ttlSecondsAfterFinished,optional"` // 完成后保留时间（K8s 1.12+）

	// 完成模式（K8s 1.21+）
	CompletionMode string `json:"completionMode"` // NonIndexed, Indexed

	// Job 暂停（K8s 1.21+）
	Suspend bool `json:"suspend"` // Job 是否暂停

	// Pod 替换策略（K8s 1.28+）
	PodReplacementPolicy string `json:"podReplacementPolicy"` // TerminatingOrFailed, Failed

	// Indexed 模式专用（K8s 1.28+）
	BackoffLimitPerIndex *int32 `json:"backoffLimitPerIndex,optional"` // 每索引失败限制
	MaxFailedIndexes     *int32 `json:"maxFailedIndexes,optional"`     // 最大失败索引数

	// Pod 失败策略（K8s 1.25+）
	PodFailurePolicy *PodFailurePolicyConfig `json:"podFailurePolicy,optional"`
}

// UpdateJobSpecRequest 更新 Job 规格请求
type UpdateJobSpecRequest struct {
	Name                    string                  `json:"name"`
	Namespace               string                  `json:"namespace"`
	Parallelism             *int32                  `json:"parallelism,optional"`
	Completions             *int32                  `json:"completions,optional"`
	BackoffLimit            *int32                  `json:"backoffLimit,optional"`
	ActiveDeadlineSeconds   *int64                  `json:"activeDeadlineSeconds,optional"`
	TTLSecondsAfterFinished *int32                  `json:"ttlSecondsAfterFinished,optional"`
	CompletionMode          *string                 `json:"completionMode,optional"`
	Suspend                 *bool                   `json:"suspend,optional"`
	PodReplacementPolicy    *string                 `json:"podReplacementPolicy,optional"`
	BackoffLimitPerIndex    *int32                  `json:"backoffLimitPerIndex,optional"`
	MaxFailedIndexes        *int32                  `json:"maxFailedIndexes,optional"`
	PodFailurePolicy        *PodFailurePolicyConfig `json:"podFailurePolicy,optional"`
}

// ==================== 下次执行时间相关 ====================

// NextScheduleTimeResponse 下次调度时间响应
type NextScheduleTimeResponse struct {
	NextScheduleTime *time.Time `json:"nextScheduleTime"` // 下次调度时间
	Schedule         string     `json:"schedule"`         // Cron 表达式
	Timezone         string     `json:"timezone"`         // 时区
	IsSuspended      bool       `json:"isSuspended"`      // 是否已挂起
	CurrentTime      time.Time  `json:"currentTime"`      // 当前时间
}

// CronJobOperator CronJob 操作器接口
type CronJobOperator interface {
	// ========== 基础 CRUD 操作 ==========
	Create(*batchv1.CronJob) (*batchv1.CronJob, error)
	Get(namespace, name string) (*batchv1.CronJob, error)
	Update(*batchv1.CronJob) (*batchv1.CronJob, error)
	Delete(namespace, name string) error
	List(namespace string, req ListRequest) (*ListCronJobResponse, error)
	ListAll(namespace string) ([]batchv1.CronJob, error)
	// ========== 高级操作 ==========
	Watch(namespace string, opts metav1.ListOptions) (watch.Interface, error)
	UpdateLabels(namespace, name string, labels map[string]string) error
	UpdateAnnotations(namespace, name string, annotations map[string]string) error

	// ========== YAML 操作 ==========
	GetYaml(namespace, name string) (string, error) // 获取资源 YAML
	GetDescribe(namespace, name string) (string, error)

	// ========== Pod 管理 ==========
	GetPods(namespace, name string) ([]PodDetailInfo, error) // 获取所有 Job 的 Pods（带容器信息）

	// ========== 镜像管理 ==========
	GetContainerImages(namespace, name string) (*ContainerInfoList, error) // 查询镜像
	UpdateImage(req *UpdateImageRequest) error                             // 更新单个镜像
	UpdateImages(req *UpdateImagesRequest) error                           // 批量更新镜像

	// ========== 调度配置管理 ==========
	GetScheduleConfig(namespace, name string) (*CronJobScheduleConfig, error) // 查询调度配置
	UpdateScheduleConfig(req *UpdateCronJobScheduleRequest) error             // 修改调度配置

	// ========== 环境变量管理 ==========
	GetEnvVars(namespace, name string) (*EnvVarsResponse, error) // 查询环境变量
	UpdateEnvVars(req *UpdateEnvVarsRequest) error               // 修改环境变量

	// ========== 暂停/恢复更新 ==========
	GetPauseStatus(namespace, name string) (*PauseStatusResponse, error) // 查询暂停状态
	Suspend(namespace, name string) error                                // 挂起 CronJob（停止调度）
	Resume(namespace, name string) error                                 // 恢复 CronJob 调度

	// ========== 资源配额管理 ==========
	GetResources(namespace, name string) (*ResourcesResponse, error) // 查询资源配额
	UpdateResources(req *UpdateResourcesRequest) error               // 修改资源配额

	// ========== 健康检查管理 ==========
	GetProbes(namespace, name string) (*ProbesResponse, error) // 查询健康检查
	UpdateProbes(req *UpdateProbesRequest) error               // 修改健康检查

	// ========== CronJob 控制操作 ==========
	Stop(namespace, name string) error                           // 停止（挂起）
	Start(namespace, name string) error                          // 启动（恢复）
	TriggerJob(req *TriggerCronJobRequest) (*batchv1.Job, error) // 手动触发创建 Job

	// ========== Job 和事件管理 ==========
	GetJobHistory(namespace, name string) (*CronJobHistoryResponse, error) // 获取历史 Job 列表
	GetEvents(namespace, name string) ([]EventInfo, error)                 // 获取事件

	// ========== 调度信息 ==========
	GetNextScheduleTime(namespace, name string) (*time.Time, error) // 计算下次调度时间
	GetScheduleExpression(namespace, name string) (string, error)   // 获取调度表达式

	// GetPodLabels 获取 Pod 标签（spec.jobTemplate.spec.template.metadata.labels）
	GetPodLabels(namespace, name string) (map[string]string, error)

	// GetPodSelectorLabels 获取 Pod 选择器标签（spec.jobTemplate.spec.selector.matchLabels）
	GetPodSelectorLabels(namespace, name string) (map[string]string, error)

	// GetStatus 获取 CronJob 状态
	GetVersionStatus(namespace, name string) (*ResourceStatus, error)
	// ========== 调度配置管理 ==========
	GetSchedulingConfig(namespace, name string) (*SchedulingConfig, error)
	UpdateSchedulingConfig(namespace, name string, config *UpdateSchedulingConfigRequest) error

	// ========== 存储配置管理 ==========
	GetStorageConfig(namespace, name string) (*StorageConfig, error)
	UpdateStorageConfig(namespace, name string, config *UpdateStorageConfigRequest) error

	GetJobSpec(namespace, name string) (*JobSpecConfig, error)                         // 获取 Job Spec 配置
	UpdateJobSpec(req *UpdateJobSpecRequest) error                                     // 更新 Job Spec 配置
	GetNextScheduleTimeInfo(namespace, name string) (*NextScheduleTimeResponse, error) // 获取下次调度时间详细信息
	GetResourceSummary(
		namespace string,
		name string,
		domainSuffix string,
		nodeLb []string,
		podOp PodOperator,
		svcOp ServiceOperator,
		ingressOp IngressOperator,
	) (*WorkloadResourceSummary, error)
	// GetAdvancedConfig 获取高级容器配置
	// 返回 Pod 级别配置 + 所有容器（init/main/ephemeral）的高级配置
	GetAdvancedConfig(namespace, name string) (*AdvancedConfigResponse, error)

	// UpdateAdvancedConfig 更新高级容器配置（全量更新）
	// 支持同时更新 Pod 级别配置和所有容器的高级配置
	UpdateAdvancedConfig(req *UpdateAdvancedConfigRequest) error
}
