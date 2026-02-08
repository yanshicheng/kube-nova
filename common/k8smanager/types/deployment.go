package types

import (
	"time"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
)

// DeploymentInfo Deployment 信息
type DeploymentInfo struct {
	Name              string
	Namespace         string
	Replicas          int32
	ReadyReplicas     int32
	AvailableReplicas int32
	CreationTimestamp time.Time
	Images            []string
}

// ListDeploymentResponse Deployment 列表响应
type ListDeploymentResponse struct {
	ListResponse
	Items []DeploymentInfo
}

// ReplicasInfo 副本数信息
type ReplicasInfo struct {
	Replicas          int32 `json:"replicas"`          // 副本数
	AvailableReplicas int32 `json:"availableReplicas"` // 可用副本数
	ReadyReplicas     int32 `json:"readyReplicas"`     // 就绪副本数
	UpdatedReplicas   int32 `json:"updatedReplicas"`   // 更新的副本数
	CurrentReplicas   int32 `json:"currentReplicas"`   // 当前副本数
}

// ScaleRequest 扩缩容请求
type ScaleRequest struct {
	Name      string
	Namespace string
	Replicas  int32
}

// DeploymentOperator Deployment 操作器接口
type DeploymentOperator interface {
	// ========== 基础 CRUD 操作 ==========
	Create(*appsv1.Deployment) (*appsv1.Deployment, error)
	Get(namespace, name string) (*appsv1.Deployment, error)
	Update(*appsv1.Deployment) (*appsv1.Deployment, error)
	Delete(namespace, name string) error
	List(namespace string, req ListRequest) (*ListDeploymentResponse, error)
	ListAll(namespace string) ([]appsv1.Deployment, error)
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
	GetReplicas(namespace, name string) (*ReplicasInfo, error) // 查询副本数
	Scale(req *ScaleRequest) error                             // 扩缩容

	// ========== 更新策略管理 ==========
	GetUpdateStrategy(namespace, name string) (*UpdateStrategyResponse, error) // 查询更新策略
	UpdateStrategy(req *UpdateStrategyRequest) error                           // 修改更新策略

	// ========== 版本历史与回滚 ==========
	GetRevisions(namespace, name string) ([]RevisionInfo, error) // 获取版本历史
	Rollback(req *RollbackToRevisionRequest) error               // 回滚到指定版本

	// ========== 环境变量管理 ==========
	GetEnvVars(namespace, name string) (*EnvVarsResponse, error) // 查询环境变量
	UpdateEnvVars(req *UpdateEnvVarsRequest) error               // 修改环境变量

	// ========== 暂停/恢复更新 ==========
	GetPauseStatus(namespace, name string) (*PauseStatusResponse, error) // 查询暂停状态
	PauseRollout(namespace, name string) error                           // 暂停更新
	ResumeRollout(namespace, name string) error                          // 恢复更新

	// ========== 资源配额管理 ==========
	GetResources(namespace, name string) (*ResourcesResponse, error) // 查询资源配额
	UpdateResources(req *UpdateResourcesRequest) error               // 修改资源配额

	// ========== 健康检查管理 ==========
	GetProbes(namespace, name string) (*ProbesResponse, error) // 查询健康检查
	UpdateProbes(req *UpdateProbesRequest) error               // 修改健康检查

	// ========== 停止和启动 ==========
	Stop(namespace, name string) error  // 停止：replicas=0，保存原副本数到注解
	Start(namespace, name string) error // 启动：从注解恢复副本数，无注解则设为1

	// ========== 重启 ==========
	Restart(namespace, name string) error // 重启（滚动重启所有 Pods）

	// ========== 事件 ==========
	GetEvents(namespace, name string) ([]EventInfo, error) // 获取事件

	// GetPodLabels 获取 Pod 标签（spec.template.metadata.labels）
	GetPodLabels(namespace, name string) (map[string]string, error)

	// GetPodSelectorLabels 获取 Pod 选择器标签（spec.selector.matchLabels）
	GetPodSelectorLabels(namespace, name string) (map[string]string, error)

	// GetStatus 获取 Deployment 状态
	GetVersionStatus(namespace, name string) (*ResourceStatus, error)
	// ========== 调度配置管理 ==========
	GetSchedulingConfig(namespace, name string) (*SchedulingConfig, error)                      // 获取调度配置
	UpdateSchedulingConfig(namespace, name string, config *UpdateSchedulingConfigRequest) error // 更新调度配置

	// ========== 存储配置管理 ==========
	GetStorageConfig(namespace, name string) (*StorageConfig, error)                      // 获取存储配置
	UpdateStorageConfig(namespace, name string, config *UpdateStorageConfigRequest) error // 更新存储配置
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
