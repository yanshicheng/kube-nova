package types

import (
	"time"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
)

// DaemonSetInfo DaemonSet 信息
type DaemonSetInfo struct {
	Name                   string
	Namespace              string
	DesiredNumberScheduled int32
	CurrentNumberScheduled int32
	NumberReady            int32
	NumberAvailable        int32
	CreationTimestamp      time.Time
	Images                 []string
}

// ListDaemonSetResponse DaemonSet 列表响应
type ListDaemonSetResponse struct {
	ListResponse
	Items []DaemonSetInfo
}

// DaemonSetStatusInfo DaemonSet 状态信息
type DaemonSetStatusInfo struct {
	DesiredNumberScheduled int32 `json:"desiredNumberScheduled"`
	CurrentNumberScheduled int32 `json:"currentNumberScheduled"`
	NumberReady            int32 `json:"numberReady"`
	NumberAvailable        int32 `json:"numberAvailable"`
	NumberMisscheduled     int32 `json:"numberMisscheduled"`
	UpdatedNumberScheduled int32 `json:"updatedNumberScheduled"`
}

// DaemonSetOperator DaemonSet 操作器接口
type DaemonSetOperator interface {
	// ========== 基础 CRUD 操作 ==========
	Create(*appsv1.DaemonSet) (*appsv1.DaemonSet, error)
	Get(namespace, name string) (*appsv1.DaemonSet, error)
	Update(*appsv1.DaemonSet) (*appsv1.DaemonSet, error)
	Delete(namespace, name string) error
	List(namespace string, req ListRequest) (*ListDaemonSetResponse, error)
	ListAll(namespace string) ([]appsv1.DaemonSet, error)
	// ========== 高级操作 ==========
	Watch(namespace string, opts metav1.ListOptions) (watch.Interface, error)
	UpdateLabels(namespace, name string, labels map[string]string) error
	UpdateAnnotations(namespace, name string, annotations map[string]string) error

	// ========== YAML 操作 ==========
	GetYaml(namespace, name string) (string, error) // 获取资源 YAML
	GetDescribe(namespace, name string) (string, error)

	// ========== Pod 管理 ==========
	GetPods(namespace, name string) ([]PodDetailInfo, error) // 获取关联的 Pods（带容器信息）

	// ========== 镜像管理 ==========
	GetContainerImages(namespace, name string) (*ContainerInfoList, error) // 查询镜像
	UpdateImage(req *UpdateImageRequest) error                             // 更新单个镜像
	UpdateImages(req *UpdateImagesRequest) error                           // 批量更新镜像

	// ========== 副本数管理 ==========
	// 注意：DaemonSet 没有副本数概念（每个节点一个 Pod），返回状态信息
	GetStatus(namespace, name string) (*DaemonSetStatusInfo, error)

	// ========== 更新策略管理 ==========
	GetUpdateStrategy(namespace, name string) (*UpdateStrategyResponse, error) // 查询更新策略
	UpdateStrategy(req *UpdateStrategyRequest) error                           // 修改更新策略

	// ========== 版本历史与回滚 ==========
	GetRevisions(namespace, name string) ([]RevisionInfo, error)          // 获取 K8s 原生版本历史
	GetConfigHistory(namespace, name string) ([]ConfigHistoryInfo, error) // 获取业务层配置历史
	Rollback(req *RollbackToRevisionRequest) error                        // 回滚到 K8s 版本
	RollbackToConfig(req *RollbackToConfigRequest) error                  // 回滚到历史配置

	// ========== 环境变量管理 ==========
	GetEnvVars(namespace, name string) (*EnvVarsResponse, error) // 查询环境变量
	UpdateEnvVars(req *UpdateEnvVarsRequest) error               // 修改环境变量

	// ========== 暂停/恢复更新 ==========
	// 注意：DaemonSet 不支持暂停，返回 supportType=none
	GetPauseStatus(namespace, name string) (*PauseStatusResponse, error)

	// ========== 资源配额管理 ==========
	GetResources(namespace, name string) (*ResourcesResponse, error) // 查询资源配额
	UpdateResources(req *UpdateResourcesRequest) error               // 修改资源配额

	// ========== 健康检查管理 ==========
	GetProbes(namespace, name string) (*ProbesResponse, error) // 查询健康检查
	UpdateProbes(req *UpdateProbesRequest) error               // 修改健康检查

	// ========== 停止和启动 ==========
	Stop(namespace, name string) error  // 停止：通过节点选择器实现
	Start(namespace, name string) error // 启动：恢复节点选择器

	// ========== 重启 ==========
	Restart(namespace, name string) error // 重启（滚动重启所有 Pods）

	// ========== 事件 ==========
	GetEvents(namespace, name string) ([]EventInfo, error) // 获取事件
	// GetPodLabels 获取 Pod 标签（spec.template.metadata.labels）
	GetPodLabels(namespace, name string) (map[string]string, error)

	// GetPodSelectorLabels 获取 Pod 选择器标签（spec.selector.matchLabels）
	GetPodSelectorLabels(namespace, name string) (map[string]string, error)

	// GetStatus 获取 DaemonSet 状态
	GetVersionStatus(namespace, name string) (*ResourceStatus, error)
	// ========== 调度配置管理 ==========
	GetSchedulingConfig(namespace, name string) (*SchedulingConfig, error)
	UpdateSchedulingConfig(namespace, name string, config *UpdateSchedulingConfigRequest) error

	// ========== 存储配置管理 ==========
	GetStorageConfig(namespace, name string) (*StorageConfig, error)
	UpdateStorageConfig(namespace, name string, config *UpdateStorageConfigRequest) error
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

// WorkloadResourceSummary 工作负载资源摘要（通用于 Deployment、StatefulSet、DaemonSet、CronJob）
type WorkloadResourceSummary struct {
	PodCount         int                  `json:"podCount"`         // Pod 总数
	AbnormalPodCount int                  `json:"abnormalPodCount"` // 异常 Pod 数量
	ServiceCount     int                  `json:"serviceCount"`     // 关联的 Service 数量
	IngressCount     int                  `json:"ingressCount"`     // 关联的 Ingress 数量
	IsAppSelector    bool                 `json:"isAppSelector"`
	Service          ServiceAccessSummary `json:"service"`        // Service 访问汇总
	IngressDomains   []string             `json:"ingressDomains"` // Ingress 域名列表
}

// ServiceAccessSummary Service 访问汇总
type ServiceAccessSummary struct {
	InternalAccessList []string `json:"internalAccessList"` // 内部访问域名列表（ClusterIP）
	ExternalAccessList []string `json:"externalAccessList"` // 外部访问域名列表（LoadBalancer）
	NodePortList       []string `json:"nodePortList"`       // NodePort 访问列表
}

// 为了保持向后兼容，保留别名
type DeploymentResourceSummary = WorkloadResourceSummary
type StatefulSetResourceSummary = WorkloadResourceSummary
type DaemonSetResourceSummary = WorkloadResourceSummary
type CronJobResourceSummary = WorkloadResourceSummary
