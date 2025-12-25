package types

import "time"

// ==================== Namespace 级别类型 ====================

// NamespaceMetrics Namespace 综合指标
type NamespaceMetrics struct {
	Namespace string                   `json:"namespace"`
	Timestamp time.Time                `json:"timestamp"`
	Resources NamespaceResourceMetrics `json:"resources"`
	Quota     NamespaceQuotaMetrics    `json:"quota"`
	Workloads NamespaceWorkloadMetrics `json:"workloads"`
	Network   NamespaceNetworkMetrics  `json:"network"`
	Storage   NamespaceStorageMetrics  `json:"storage"`
	Config    NamespaceConfigMetrics   `json:"config"`
}

// ==================== 资源使用指标 ====================

// NamespaceResourceMetrics Namespace 资源指标
type NamespaceResourceMetrics struct {
	CPU    NamespaceCPUMetrics    `json:"cpu"`
	Memory NamespaceMemoryMetrics `json:"memory"`
}

// NamespaceCPUMetrics CPU 指标
type NamespaceCPUMetrics struct {
	Current              NamespaceCPUSnapshot       `json:"current"`
	Requests             float64                    `json:"requests"`
	Limits               float64                    `json:"limits"`
	Trend                []NamespaceCPUDataPoint    `json:"trend"`
	TopPods              []ResourceRanking          `json:"topPods"`
	TopContainers        []ContainerResourceRanking `json:"topContainers"`
	ContainersNoRequests int64                      `json:"containersNoRequests"`
	ContainersNoLimits   int64                      `json:"containersNoLimits"`
}

// NamespaceCPUSnapshot CPU 快照
type NamespaceCPUSnapshot struct {
	Timestamp    time.Time `json:"timestamp"`
	UsageCores   float64   `json:"usageCores"`
	AvgCores     float64   `json:"avgCores"`
	MaxCores     float64   `json:"maxCores"`
	UsagePercent float64   `json:"usagePercent"` // 相对于 Requests 的使用率
}

// NamespaceCPUDataPoint CPU 数据点
type NamespaceCPUDataPoint struct {
	Timestamp    time.Time `json:"timestamp"`
	UsageCores   float64   `json:"usageCores"`
	UsagePercent float64   `json:"usagePercent"`
}

// NamespaceMemoryMetrics 内存指标
type NamespaceMemoryMetrics struct {
	Current       NamespaceMemorySnapshot    `json:"current"`
	Requests      int64                      `json:"requests"`
	Limits        int64                      `json:"limits"`
	Trend         []NamespaceMemoryDataPoint `json:"trend"`
	TopPods       []ResourceRanking          `json:"topPods"`
	TopContainers []ContainerResourceRanking `json:"topContainers"`
	OOMKills      int64                      `json:"oomKills"`
}

// NamespaceMemorySnapshot 内存快照
type NamespaceMemorySnapshot struct {
	Timestamp       time.Time `json:"timestamp"`
	WorkingSetBytes int64     `json:"workingSetBytes"`
	RSSBytes        int64     `json:"rssBytes"`
	CacheBytes      int64     `json:"cacheBytes"`
	AvgBytes        int64     `json:"avgBytes"`
	MaxBytes        int64     `json:"maxBytes"`
	UsagePercent    float64   `json:"usagePercent"` // 相对于 Requests 的使用率
}

// NamespaceMemoryDataPoint 内存数据点
type NamespaceMemoryDataPoint struct {
	Timestamp       time.Time `json:"timestamp"`
	WorkingSetBytes int64     `json:"workingSetBytes"`
	UsagePercent    float64   `json:"usagePercent"`
}

// ContainerResourceRanking 容器资源排名
type ContainerResourceRanking struct {
	PodName       string  `json:"podName"`
	ContainerName string  `json:"containerName"`
	Value         float64 `json:"value"`
	Unit          string  `json:"unit"`
}

// ResourceRanking 资源排名
type ResourceRanking struct {
	Name  string  `json:"name"`
	Value float64 `json:"value"`
	Unit  string  `json:"unit"`
}

// ==================== 配额管理 ====================

// NamespaceQuotaMetrics 配额指标
type NamespaceQuotaMetrics struct {
	HasQuota     bool                  `json:"hasQuota"`
	QuotaName    string                `json:"quotaName,omitempty"`
	CPU          QuotaUsage            `json:"cpu"`
	Memory       QuotaUsage            `json:"memory"`
	Pods         QuotaUsage            `json:"pods"`
	Services     QuotaUsage            `json:"services"`
	ConfigMaps   QuotaUsage            `json:"configMaps"`
	Secrets      QuotaUsage            `json:"secrets"`
	PVCs         QuotaUsage            `json:"pvcs"`
	Storage      QuotaUsage            `json:"storage"`
	AllResources []ResourceQuotaDetail `json:"allResources,omitempty"`
}

// QuotaUsage 配额使用情况
type QuotaUsage struct {
	Hard         float64 `json:"hard"`
	Used         float64 `json:"used"`
	UsagePercent float64 `json:"usagePercent"`
	HasQuota     bool    `json:"hasQuota"`
}

// ==================== 工作负载统计 ====================

// NamespaceWorkloadMetrics 工作负载统计
type NamespaceWorkloadMetrics struct {
	Pods         NamespacePodStatistics         `json:"pods"`
	Deployments  NamespaceDeploymentStatistics  `json:"deployments"`
	StatefulSets NamespaceStatefulSetStatistics `json:"statefulSets"`
	DaemonSets   NamespaceDaemonSetStatistics   `json:"daemonSets"`
	Jobs         NamespaceJobStatistics         `json:"jobs"`
	Containers   NamespaceContainerStatistics   `json:"containers"`
	Services     NamespaceServiceStatistics     `json:"services"`
	Endpoints    NamespaceEndpointStatistics    `json:"endpoints"`
	Ingresses    NamespaceIngressStatistics     `json:"ingresses"`
}

// NamespacePodStatistics Pod 统计
type NamespacePodStatistics struct {
	Total           int64                   `json:"total"`
	Running         int64                   `json:"running"`
	Pending         int64                   `json:"pending"`
	Failed          int64                   `json:"failed"`
	Succeeded       int64                   `json:"succeeded"`
	Unknown         int64                   `json:"unknown"`
	Ready           int64                   `json:"ready"`
	NotReady        int64                   `json:"notReady"`
	TotalRestarts   int64                   `json:"totalRestarts"`
	HighRestartPods []PodRestartInfo        `json:"highRestartPods"`
	Trend           []NamespacePodDataPoint `json:"trend"`
}

// NamespacePodDataPoint Pod 数据点
type NamespacePodDataPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Total     int64     `json:"total"`
	Running   int64     `json:"running"`
	Pending   int64     `json:"pending"`
	Failed    int64     `json:"failed"`
}

// NamespaceDeploymentStatistics Deployment 统计
type NamespaceDeploymentStatistics struct {
	Total               int64 `json:"total"`
	DesiredReplicas     int64 `json:"desiredReplicas"`
	AvailableReplicas   int64 `json:"availableReplicas"`
	UnavailableReplicas int64 `json:"unavailableReplicas"`
	Updating            int64 `json:"updating"`
}

// NamespaceStatefulSetStatistics StatefulSet 统计
type NamespaceStatefulSetStatistics struct {
	Total           int64 `json:"total"`
	DesiredReplicas int64 `json:"desiredReplicas"`
	ReadyReplicas   int64 `json:"readyReplicas"`
	CurrentReplicas int64 `json:"currentReplicas"`
}

// NamespaceDaemonSetStatistics DaemonSet 统计
type NamespaceDaemonSetStatistics struct {
	Total                int64 `json:"total"`
	DesiredScheduled     int64 `json:"desiredScheduled"`
	CurrentScheduled     int64 `json:"currentScheduled"`
	AvailableScheduled   int64 `json:"availableScheduled"`
	UnavailableScheduled int64 `json:"unavailableScheduled"`
}

// NamespaceJobStatistics Job 统计
type NamespaceJobStatistics struct {
	Active        int64   `json:"active"`
	Succeeded     int64   `json:"succeeded"`
	Failed        int64   `json:"failed"`
	CronJobsTotal int64   `json:"cronJobsTotal"`
	AvgDuration   float64 `json:"avgDuration"` // 平均执行时间（秒）
}

// NamespaceContainerStatistics 容器统计
type NamespaceContainerStatistics struct {
	Total      int64 `json:"total"`
	Running    int64 `json:"running"`
	Waiting    int64 `json:"waiting"`
	Terminated int64 `json:"terminated"`
}

// NamespaceServiceStatistics Service 统计
type NamespaceServiceStatistics struct {
	Total            int64              `json:"total"`
	ByType           []ServiceTypeCount `json:"byType"`
	WithoutEndpoints int64              `json:"withoutEndpoints"` // 无 Endpoint 的 Service 数
}

// NamespaceEndpointStatistics Endpoint 统计
type NamespaceEndpointStatistics struct {
	TotalAddresses        int64 `json:"totalAddresses"`
	ServicesWithEndpoints int64 `json:"servicesWithEndpoints"`
}

// NamespaceIngressStatistics Ingress 统计
type NamespaceIngressStatistics struct {
	Total     int64 `json:"total"`
	PathCount int64 `json:"pathCount"`
}

// ==================== 网络指标 ====================

// NamespaceNetworkMetrics 网络指标
type NamespaceNetworkMetrics struct {
	Current     NamespaceNetworkSnapshot    `json:"current"`
	Trend       []NamespaceNetworkDataPoint `json:"trend"`
	Summary     NamespaceNetworkSummary     `json:"summary"`
	TopPodsByRx []ResourceRanking           `json:"topPodsByRx"`
	TopPodsByTx []ResourceRanking           `json:"topPodsByTx"`
}

// NamespaceNetworkSnapshot 网络快照
type NamespaceNetworkSnapshot struct {
	Timestamp             time.Time `json:"timestamp"`
	ReceiveBytesPerSec    float64   `json:"receiveBytesPerSec"`
	TransmitBytesPerSec   float64   `json:"transmitBytesPerSec"`
	ReceivePacketsPerSec  float64   `json:"receivePacketsPerSec"`
	TransmitPacketsPerSec float64   `json:"transmitPacketsPerSec"`
	ReceiveErrors         float64   `json:"receiveErrors"`
	TransmitErrors        float64   `json:"transmitErrors"`
	ReceiveDrops          float64   `json:"receiveDrops"`
	TransmitDrops         float64   `json:"transmitDrops"`
}

// NamespaceNetworkDataPoint 网络数据点
type NamespaceNetworkDataPoint struct {
	Timestamp           time.Time `json:"timestamp"`
	ReceiveBytesPerSec  float64   `json:"receiveBytesPerSec"`
	TransmitBytesPerSec float64   `json:"transmitBytesPerSec"`
}

// NamespaceNetworkSummary 网络统计
type NamespaceNetworkSummary struct {
	TotalReceiveBytes      int64   `json:"totalReceiveBytes"`
	TotalTransmitBytes     int64   `json:"totalTransmitBytes"`
	MaxReceiveBytesPerSec  float64 `json:"maxReceiveBytesPerSec"`
	MaxTransmitBytesPerSec float64 `json:"maxTransmitBytesPerSec"`
	AvgReceiveBytesPerSec  float64 `json:"avgReceiveBytesPerSec"`
	AvgTransmitBytesPerSec float64 `json:"avgTransmitBytesPerSec"`
	TotalErrors            int64   `json:"totalErrors"`
}

// ==================== 存储指标 ====================

// NamespaceStorageMetrics 存储指标
type NamespaceStorageMetrics struct {
	PVCTotal            int64               `json:"pvcTotal"`
	PVCBound            int64               `json:"pvcBound"`
	PVCPending          int64               `json:"pvcPending"`
	PVCLost             int64               `json:"pvcLost"`
	TotalRequestedBytes int64               `json:"totalRequestedBytes"`
	ByStorageClass      []StorageClassUsage `json:"byStorageClass"`
}

// ==================== 配置管理 ====================

// NamespaceConfigMetrics 配置指标
type NamespaceConfigMetrics struct {
	ConfigMaps ConfigMapStatistics `json:"configMaps"`
	Secrets    SecretStatistics    `json:"secrets"`
}

// ConfigMapStatistics ConfigMap 统计
type ConfigMapStatistics struct {
	Total     int64 `json:"total"`
	DataItems int64 `json:"dataItems"`
}

// SecretStatistics Secret 统计
type SecretStatistics struct {
	Total  int64             `json:"total"`
	ByType []SecretTypeCount `json:"byType"`
}

// SecretTypeCount 按类型统计 Secret
type SecretTypeCount struct {
	Type  string `json:"type"`
	Count int64  `json:"count"`
}

// ==================== Namespace 对比 ====================

// NamespaceComparison 多个 Namespace 对比
type NamespaceComparison struct {
	Namespaces []NamespaceComparisonItem `json:"namespaces"`
	Timestamp  time.Time                 `json:"timestamp"`
}

// NamespaceComparisonItem Namespace 对比项
type NamespaceComparisonItem struct {
	Namespace     string  `json:"namespace"`
	CPUUsage      float64 `json:"cpuUsage"`
	MemoryUsage   int64   `json:"memoryUsage"`
	PodCount      int64   `json:"podCount"`
	NetworkRxRate float64 `json:"networkRxRate"`
	NetworkTxRate float64 `json:"networkTxRate"`
}

// ==================== NamespaceOperator 接口 ====================

// NamespaceOperator Namespace 操作器接口
type NamespaceOperator interface {
	// 综合查询
	GetNamespaceMetrics(namespace string, timeRange *TimeRange) (*NamespaceMetrics, error)

	// 资源使用查询
	GetNamespaceCPU(namespace string, timeRange *TimeRange) (*NamespaceCPUMetrics, error)
	GetNamespaceMemory(namespace string, timeRange *TimeRange) (*NamespaceMemoryMetrics, error)
	GetNamespaceNetwork(namespace string, timeRange *TimeRange) (*NamespaceNetworkMetrics, error)

	// 配额管理查询
	GetNamespaceQuota(namespace string) (*NamespaceQuotaMetrics, error)

	// 工作负载查询
	GetNamespaceWorkloads(namespace string, timeRange *TimeRange) (*NamespaceWorkloadMetrics, error)
	GetNamespacePods(namespace string, timeRange *TimeRange) (*NamespacePodStatistics, error)
	GetNamespaceDeployments(namespace string) (*NamespaceDeploymentStatistics, error)
	GetNamespaceServices(namespace string) (*NamespaceServiceStatistics, error)

	// 存储和配置查询
	GetNamespaceStorage(namespace string) (*NamespaceStorageMetrics, error)
	GetNamespaceConfig(namespace string) (*NamespaceConfigMetrics, error)

	// Top N 查询
	GetTopPodsByCPU(namespace string, limit int, timeRange *TimeRange) ([]ResourceRanking, error)
	GetTopPodsByMemory(namespace string, limit int, timeRange *TimeRange) ([]ResourceRanking, error)
	GetTopPodsByNetwork(namespace string, limit int, timeRange *TimeRange) ([]ResourceRanking, error)
	GetTopContainersByCPU(namespace string, limit int, timeRange *TimeRange) ([]ContainerResourceRanking, error)
	GetTopContainersByMemory(namespace string, limit int, timeRange *TimeRange) ([]ContainerResourceRanking, error)

	// 对比查询
	CompareNamespaces(namespaces []string, timeRange *TimeRange) (*NamespaceComparison, error)
}
