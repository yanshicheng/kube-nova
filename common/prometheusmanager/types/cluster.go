package types

import "time"

// ==================== Cluster 级别类型 ====================

// ClusterOverview 集群综合概览
type ClusterOverview struct {
	ClusterName  string                     `json:"clusterName"`
	Timestamp    time.Time                  `json:"timestamp"`
	Resources    ClusterResourceMetrics     `json:"resources"`
	Nodes        ClusterNodeMetrics         `json:"nodes"`
	ControlPlane ClusterControlPlaneMetrics `json:"controlPlane"`
	Workloads    ClusterWorkloadMetrics     `json:"workloads"`
	Network      ClusterNetworkMetrics      `json:"network"`
	Storage      ClusterStorageMetrics      `json:"storage"`
	Namespaces   ClusterNamespaceMetrics    `json:"namespaces"`
}

// ==================== 集群资源指标 ====================

// ClusterResourceMetrics 集群资源指标
type ClusterResourceMetrics struct {
	CPU     ClusterResourceSummary `json:"cpu"`
	Memory  ClusterResourceSummary `json:"memory"`
	Pods    ClusterPodSummary      `json:"pods"`
	Storage ClusterStorageSummary  `json:"storage"`
}

// ClusterResourceSummary 资源汇总
type ClusterResourceSummary struct {
	Capacity          float64                    `json:"capacity"`          // 总容量
	Allocatable       float64                    `json:"allocatable"`       // 可分配
	RequestsAllocated float64                    `json:"requestsAllocated"` // 已分配 Requests
	LimitsAllocated   float64                    `json:"limitsAllocated"`   // 已分配 Limits
	Usage             float64                    `json:"usage"`             // 实际使用
	RequestsPercent   float64                    `json:"requestsPercent"`   // Requests 使用率
	UsagePercent      float64                    `json:"usagePercent"`      // 实际使用率
	Trend             []ClusterResourceDataPoint `json:"trend"`             // 趋势数据
	NoRequestsCount   int64                      `json:"noRequestsCount"`   // 无 Requests 的容器数
	NoLimitsCount     int64                      `json:"noLimitsCount"`     // 无 Limits 的容器数
}

// ClusterResourceDataPoint 资源数据点
type ClusterResourceDataPoint struct {
	Timestamp       time.Time `json:"timestamp"`
	Usage           float64   `json:"usage"`
	UsagePercent    float64   `json:"usagePercent"`
	RequestsPercent float64   `json:"requestsPercent"`
}

// ClusterPodSummary Pod 汇总
type ClusterPodSummary struct {
	Capacity     int64   `json:"capacity"`     // 最大 Pod 数
	Running      int64   `json:"running"`      // 运行中的 Pod 数
	UsagePercent float64 `json:"usagePercent"` // Pod 容量使用率
}

// ClusterStorageSummary 存储汇总
type ClusterStorageSummary struct {
	TotalCapacityBytes int64   `json:"totalCapacityBytes"` // PV 总容量
	AllocatedBytes     int64   `json:"allocatedBytes"`     // PVC 已分配容量
	AllocationPercent  float64 `json:"allocationPercent"`  // 分配率
}

// ==================== 节点状态统计 ====================

// ClusterNodeMetrics 集群节点统计
type ClusterNodeMetrics struct {
	Total          int64                  `json:"total"`
	Ready          int64                  `json:"ready"`
	NotReady       int64                  `json:"notReady"`
	Unknown        int64                  `json:"unknown"`
	MemoryPressure int64                  `json:"memoryPressure"`
	DiskPressure   int64                  `json:"diskPressure"`
	PIDPressure    int64                  `json:"pidPressure"`
	AvgCPUUsage    float64                `json:"avgCPUUsage"`    // 平均 CPU 使用率
	AvgMemoryUsage float64                `json:"avgMemoryUsage"` // 平均内存使用率
	HighLoadNodes  int64                  `json:"highLoadNodes"`  // 高负载节点数
	NodeList       []NodeBriefStatus      `json:"nodeList"`       // 节点列表
	Trend          []ClusterNodeDataPoint `json:"trend"`          // 趋势数据
}

// NodeBriefStatus 节点简要状态
type NodeBriefStatus struct {
	NodeName       string  `json:"nodeName"`
	Ready          bool    `json:"ready"`
	MemoryPressure bool    `json:"memoryPressure"`
	DiskPressure   bool    `json:"diskPressure"`
	CPUUsage       float64 `json:"cpuUsage"`
	MemoryUsage    float64 `json:"memoryUsage"`
	PodsCount      int64   `json:"podsCount"`
}

// ClusterNodeDataPoint 节点数据点
type ClusterNodeDataPoint struct {
	Timestamp      time.Time `json:"timestamp"`
	ReadyNodes     int64     `json:"readyNodes"`
	NotReadyNodes  int64     `json:"notReadyNodes"`
	AvgCPUUsage    float64   `json:"avgCPUUsage"`
	AvgMemoryUsage float64   `json:"avgMemoryUsage"`
}

// ==================== 控制平面指标 ====================

// ClusterControlPlaneMetrics 控制平面指标
type ClusterControlPlaneMetrics struct {
	APIServer         APIServerMetrics         `json:"apiServer"`
	Etcd              EtcdMetrics              `json:"etcd"`
	Scheduler         SchedulerMetrics         `json:"scheduler"`
	ControllerManager ControllerManagerMetrics `json:"controllerManager"`
}

// APIServerMetrics API Server 指标
// APIServerMetrics API Server 指标
type APIServerMetrics struct {
	// 基础指标
	RequestsPerSecond float64 `json:"requestsPerSecond"`
	ErrorRate         float64 `json:"errorRate"`

	// 延迟指标
	P50Latency float64 `json:"p50Latency"`
	P95Latency float64 `json:"p95Latency"`
	P99Latency float64 `json:"p99Latency"`

	// 并发指标
	CurrentInflightRequests  int64 `json:"currentInflightRequests"`  // 当前处理中的请求数
	InflightReadRequests     int64 `json:"inflightReadRequests"`     // 读请求数
	InflightMutatingRequests int64 `json:"inflightMutatingRequests"` // 写请求数
	LongRunningRequests      int64 `json:"longRunningRequests"`      // 长连接请求（如 watch）
	WatchCount               int64 `json:"watchCount"`               // Watch 连接数

	// 性能指标
	RequestDropped         int64   `json:"requestDropped"`         // 丢弃的请求数
	RequestTimeout         int64   `json:"requestTimeout"`         // 超时的请求数
	ResponseSizeBytes      float64 `json:"responseSizeBytes"`      // 平均响应大小
	WebhookDurationSeconds float64 `json:"webhookDurationSeconds"` // Webhook 调用延迟

	// 审计和认证
	AuthenticationAttempts float64 `json:"authenticationAttempts"` // 认证尝试次数/秒
	AuthenticationFailures float64 `json:"authenticationFailures"` // 认证失败次数/秒
	AuthorizationAttempts  float64 `json:"authorizationAttempts"`  // 鉴权尝试次数/秒
	AuthorizationDuration  float64 `json:"authorizationDuration"`  // 鉴权延迟

	// 客户端连接
	ClientCertExpirationDays float64 `json:"clientCertExpirationDays"` // 客户端证书过期天数

	// 分类统计
	RequestsByVerb     []VerbMetrics          `json:"requestsByVerb"`
	RequestsByCode     StatusCodeDistribution `json:"requestsByCode"`
	RequestsByResource []ResourceMetrics      `json:"requestsByResource"` // 按资源类型分类

	// 趋势数据
	Trend []APIServerDataPoint `json:"trend"`
}

// ResourceMetrics 按资源类型的指标
type ResourceMetrics struct {
	Resource          string  `json:"resource"` // pods, deployments, services
	RequestsPerSecond float64 `json:"requestsPerSecond"`
}

// APIServerDataPoint API Server 数据点
type APIServerDataPoint struct {
	Timestamp               time.Time `json:"timestamp"`
	RequestsPerSecond       float64   `json:"requestsPerSecond"`
	ErrorRate               float64   `json:"errorRate"`
	P95Latency              float64   `json:"p95Latency"`
	CurrentInflightRequests int64     `json:"currentInflightRequests"`
	LongRunningRequests     int64     `json:"longRunningRequests"`
}

// VerbMetrics 按操作类型的指标
type VerbMetrics struct {
	Verb              string  `json:"verb"` // GET, POST, PUT, DELETE, PATCH, LIST
	RequestsPerSecond float64 `json:"requestsPerSecond"`
}

// StatusCodeDistribution 状态码分布
type StatusCodeDistribution struct {
	Status2xx map[string]float64 `json:"status2xx"` // "200": rate, "201": rate
	Status3xx map[string]float64 `json:"status3xx"` // "301": rate, "302": rate
	Status4xx map[string]float64 `json:"status4xx"` // "400": rate, "404": rate
	Status5xx map[string]float64 `json:"status5xx"` // "500": rate, "502": rate
}

// EtcdMetrics etcd 指标
// EtcdMetrics etcd 指标
type EtcdMetrics struct {
	// 集群状态
	HasLeader     bool  `json:"hasLeader"`
	LeaderChanges int64 `json:"leaderChanges"`
	MemberCount   int64 `json:"memberCount"` // 成员数量
	IsLearner     bool  `json:"isLearner"`   // 是否是 Learner

	// 存储指标
	DBSizeBytes int64 `json:"dbSizeBytes"`
	DBSizeInUse int64 `json:"dbSizeInUse"` // 实际使用大小
	DBSizeLimit int64 `json:"dbSizeLimit"` // 大小限制
	KeyTotal    int64 `json:"keyTotal"`    // 键总数

	// 性能指标
	CommitLatency   float64 `json:"commitLatency"`   // 磁盘提交延迟（P99）
	WALFsyncLatency float64 `json:"walFsyncLatency"` // WAL fsync 延迟（P99）
	ApplyLatency    float64 `json:"applyLatency"`    // Apply 操作延迟（P99）
	SnapshotLatency float64 `json:"snapshotLatency"` // 快照延迟
	CompactLatency  float64 `json:"compactLatency"`  // 压缩延迟

	// 网络指标
	PeerRTT         float64 `json:"peerRTT"`         // Peer 网络往返延迟（P99）
	NetworkSendRate float64 `json:"networkSendRate"` // 网络发送速率
	NetworkRecvRate float64 `json:"networkRecvRate"` // 网络接收速率

	// 操作统计
	ProposalsFailed    int64 `json:"proposalsFailed"`    // 失败的提案数
	ProposalsPending   int64 `json:"proposalsPending"`   // 待处理的提案数
	ProposalsApplied   int64 `json:"proposalsApplied"`   // 已应用的提案数
	ProposalsCommitted int64 `json:"proposalsCommitted"` // 已提交的提案数

	// 操作速率
	GetRate    float64 `json:"getRate"`    // 读操作 QPS
	PutRate    float64 `json:"putRate"`    // 写操作 QPS
	DeleteRate float64 `json:"deleteRate"` // 删除操作 QPS

	// 慢操作
	SlowApplies int64 `json:"slowApplies"` // 慢 Apply 操作数
	SlowCommits int64 `json:"slowCommits"` // 慢 Commit 操作数

	// 趋势数据
	Trend []EtcdDataPoint `json:"trend"`
}

// EtcdDataPoint etcd 数据点
type EtcdDataPoint struct {
	Timestamp       time.Time `json:"timestamp"`
	DBSizeBytes     int64     `json:"dbSizeBytes"`
	CommitLatency   float64   `json:"commitLatency"`
	ProposalsFailed int64     `json:"proposalsFailed"`
	PeerRTT         float64   `json:"peerRTT"`
	GetRate         float64   `json:"getRate"`
	PutRate         float64   `json:"putRate"`
}

// SchedulerMetrics 调度器指标
// SchedulerMetrics 调度器指标
type SchedulerMetrics struct {
	// 基础指标
	ScheduleSuccessRate float64 `json:"scheduleSuccessRate"` // 调度成功率
	ScheduleAttempts    float64 `json:"scheduleAttempts"`    // 调度尝试次数/秒
	PendingPods         int64   `json:"pendingPods"`         // 待调度 Pod 数
	UnschedulablePods   int64   `json:"unschedulablePods"`   // 不可调度 Pod 数

	// 延迟指标
	P50ScheduleLatency float64 `json:"p50ScheduleLatency"`
	P95ScheduleLatency float64 `json:"p95ScheduleLatency"`
	P99ScheduleLatency float64 `json:"p99ScheduleLatency"`
	BindingLatency     float64 `json:"bindingLatency"` // 绑定延迟（P99）

	// 调度结果分类
	ScheduledPods      int64 `json:"scheduledPods"`      // 成功调度数
	FailedScheduling   int64 `json:"failedScheduling"`   // 调度失败数
	PreemptionAttempts int64 `json:"preemptionAttempts"` // 抢占尝试次数
	PreemptionVictims  int64 `json:"preemptionVictims"`  // 被抢占的 Pod 数

	// 调度失败原因分类
	FailureReasons ScheduleFailureReasons `json:"failureReasons"`

	// 调度队列
	SchedulingQueueLength int64 `json:"schedulingQueueLength"` // 调度队列长度
	ActiveQueueLength     int64 `json:"activeQueueLength"`     // 活跃队列长度
	BackoffQueueLength    int64 `json:"backoffQueueLength"`    // 退避队列长度

	// Framework 插件延迟
	PluginLatency SchedulerPluginLatency `json:"pluginLatency"`

	// 趋势数据
	Trend []SchedulerDataPoint `json:"trend"`
}

// ScheduleFailureReasons 调度失败原因统计
type ScheduleFailureReasons struct {
	InsufficientCPU    int64 `json:"insufficientCPU"`    // CPU 不足
	InsufficientMemory int64 `json:"insufficientMemory"` // 内存不足
	NodeAffinity       int64 `json:"nodeAffinity"`       // 节点亲和性
	PodAffinity        int64 `json:"podAffinity"`        // Pod 亲和性
	Taint              int64 `json:"taint"`              // 污点
	VolumeBinding      int64 `json:"volumeBinding"`      // 卷绑定失败
	NoNodesAvailable   int64 `json:"noNodesAvailable"`   // 无可用节点
}

// SchedulerPluginLatency 调度器插件延迟
type SchedulerPluginLatency struct {
	FilterLatency  float64 `json:"filterLatency"`  // Filter 插件延迟（P99）
	ScoreLatency   float64 `json:"scoreLatency"`   // Score 插件延迟（P99）
	PreBindLatency float64 `json:"preBindLatency"` // PreBind 插件延迟（P99）
	BindLatency    float64 `json:"bindLatency"`    // Bind 插件延迟（P99）
}

// SchedulerDataPoint 调度器数据点
type SchedulerDataPoint struct {
	Timestamp           time.Time `json:"timestamp"`
	ScheduleSuccessRate float64   `json:"scheduleSuccessRate"`
	PendingPods         int64     `json:"pendingPods"`
	P95ScheduleLatency  float64   `json:"p95ScheduleLatency"`
	ScheduleAttempts    float64   `json:"scheduleAttempts"`
}

// ControllerManagerMetrics 控制器管理器指标
type ControllerManagerMetrics struct {
	// Leader 选举
	IsLeader      bool  `json:"isLeader"`      // 是否是 Leader
	LeaderChanges int64 `json:"leaderChanges"` // Leader 变更次数

	// 工作队列深度
	DeploymentQueueDepth  int64 `json:"deploymentQueueDepth"`
	ReplicaSetQueueDepth  int64 `json:"replicaSetQueueDepth"`
	StatefulSetQueueDepth int64 `json:"statefulSetQueueDepth"`
	DaemonSetQueueDepth   int64 `json:"daemonSetQueueDepth"`
	JobQueueDepth         int64 `json:"jobQueueDepth"`
	NodeQueueDepth        int64 `json:"nodeQueueDepth"`
	ServiceQueueDepth     int64 `json:"serviceQueueDepth"`
	EndpointQueueDepth    int64 `json:"endpointQueueDepth"`

	// 队列延迟
	QueueLatency QueueLatencyMetrics `json:"queueLatency"`

	// 工作队列统计
	WorkQueueMetrics ControllerWorkQueueMetrics `json:"workQueueMetrics"`

	// 控制器协调延迟
	ReconcileLatency ControllerReconcileLatency `json:"reconcileLatency"`

	// 错误和重试
	TotalSyncErrors int64   `json:"totalSyncErrors"` // 总同步错误数
	RetryRate       float64 `json:"retryRate"`       // 重试率

	// 趋势数据
	Trend []ControllerManagerDataPoint `json:"trend"`
}

// ControllerWorkQueueMetrics 工作队列详细指标
type ControllerWorkQueueMetrics struct {
	AddsRate       float64 `json:"addsRate"`       // 添加速率（items/s）
	DepthTotal     int64   `json:"depthTotal"`     // 所有队列总深度
	UnfinishedWork int64   `json:"unfinishedWork"` // 未完成工作数
	LongestRunning float64 `json:"longestRunning"` // 最长运行时间（秒）
	RetriesRate    float64 `json:"retriesRate"`    // 重试速率（items/s）
}

// ControllerReconcileLatency 控制器协调延迟
type ControllerReconcileLatency struct {
	DeploymentP99  float64 `json:"deploymentP99"`
	ReplicaSetP99  float64 `json:"replicaSetP99"`
	StatefulSetP99 float64 `json:"statefulSetP99"`
	DaemonSetP99   float64 `json:"daemonSetP99"`
	JobP99         float64 `json:"jobP99"`
}

// QueueLatencyMetrics 队列延迟指标
type QueueLatencyMetrics struct {
	QueueDuration float64 `json:"queueDuration"` // 队列等待时长（P99）
	WorkDuration  float64 `json:"workDuration"`  // 工作处理时长（P99）
}

// ControllerManagerDataPoint 控制器管理器数据点
type ControllerManagerDataPoint struct {
	Timestamp             time.Time `json:"timestamp"`
	DeploymentQueueDepth  int64     `json:"deploymentQueueDepth"`
	ReplicaSetQueueDepth  int64     `json:"replicaSetQueueDepth"`
	StatefulSetQueueDepth int64     `json:"statefulSetQueueDepth"`
	TotalQueueDepth       int64     `json:"totalQueueDepth"`
	RetryRate             float64   `json:"retryRate"`
}

// ==================== 工作负载统计 ====================

// ClusterWorkloadMetrics 工作负载统计
type ClusterWorkloadMetrics struct {
	Pods         ClusterPodMetrics         `json:"pods"`
	Deployments  ClusterDeploymentMetrics  `json:"deployments"`
	StatefulSets ClusterStatefulSetMetrics `json:"statefulSets"`
	DaemonSets   ClusterDaemonSetMetrics   `json:"daemonSets"`
	Jobs         ClusterJobMetrics         `json:"jobs"`
	Services     ClusterServiceMetrics     `json:"services"`
	Containers   ClusterContainerMetrics   `json:"containers"`
}

// ClusterPodMetrics Pod 统计
type ClusterPodMetrics struct {
	Total           int64                 `json:"total"`
	Running         int64                 `json:"running"`
	Pending         int64                 `json:"pending"`
	Failed          int64                 `json:"failed"`
	Succeeded       int64                 `json:"succeeded"`
	Unknown         int64                 `json:"unknown"`
	Ready           int64                 `json:"ready"`
	NotReady        int64                 `json:"notReady"`
	TotalRestarts   int64                 `json:"totalRestarts"`
	HighRestartPods []PodRestartInfo      `json:"highRestartPods"` // 高频重启 Pod
	Trend           []ClusterPodDataPoint `json:"trend"`
}

// ClusterPodDataPoint Pod 数据点
type ClusterPodDataPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Total     int64     `json:"total"`
	Running   int64     `json:"running"`
	Pending   int64     `json:"pending"`
	Failed    int64     `json:"failed"`
}

// PodRestartInfo Pod 重启信息
type PodRestartInfo struct {
	Namespace    string  `json:"namespace"`
	PodName      string  `json:"podName"`
	RestartCount int64   `json:"restartCount"`
	RestartRate  float64 `json:"restartRate"`
}

// ClusterDeploymentMetrics Deployment 统计
type ClusterDeploymentMetrics struct {
	Total               int64 `json:"total"`
	AvailableReplicas   int64 `json:"availableReplicas"`
	UnavailableReplicas int64 `json:"unavailableReplicas"`
	Updating            int64 `json:"updating"` // 更新中的 Deployment
}

// ClusterStatefulSetMetrics StatefulSet 统计
type ClusterStatefulSetMetrics struct {
	Total           int64 `json:"total"`
	ReadyReplicas   int64 `json:"readyReplicas"`
	CurrentReplicas int64 `json:"currentReplicas"`
}

// ClusterDaemonSetMetrics DaemonSet 统计
type ClusterDaemonSetMetrics struct {
	Total           int64 `json:"total"`
	DesiredPods     int64 `json:"desiredPods"`
	AvailablePods   int64 `json:"availablePods"`
	UnavailablePods int64 `json:"unavailablePods"`
}

// ClusterJobMetrics Job 统计
type ClusterJobMetrics struct {
	Active        int64 `json:"active"`
	Succeeded     int64 `json:"succeeded"`
	Failed        int64 `json:"failed"`
	CronJobsTotal int64 `json:"cronJobsTotal"`
}

// ClusterServiceMetrics Service 统计
type ClusterServiceMetrics struct {
	Total  int64              `json:"total"`
	ByType []ServiceTypeCount `json:"byType"`
}

// ServiceTypeCount 按类型统计 Service
type ServiceTypeCount struct {
	Type  string `json:"type"` // ClusterIP, NodePort, LoadBalancer
	Count int64  `json:"count"`
}

// ClusterContainerMetrics 容器统计
type ClusterContainerMetrics struct {
	Total   int64 `json:"total"`
	Running int64 `json:"running"`
}

// ==================== 网络和存储 ====================

// ClusterNetworkMetrics 集群网络统计
type ClusterNetworkMetrics struct {
	TotalIngressBytesPerSec float64                   `json:"totalIngressBytesPerSec"`
	TotalEgressBytesPerSec  float64                   `json:"totalEgressBytesPerSec"`
	Trend                   []ClusterNetworkDataPoint `json:"trend"`
}

// ClusterNetworkDataPoint 网络数据点
type ClusterNetworkDataPoint struct {
	Timestamp          time.Time `json:"timestamp"`
	IngressBytesPerSec float64   `json:"ingressBytesPerSec"`
	EgressBytesPerSec  float64   `json:"egressBytesPerSec"`
}

// ClusterStorageMetrics 集群存储统计
type ClusterStorageMetrics struct {
	PVTotal        int64               `json:"pvTotal"`
	PVBound        int64               `json:"pvBound"`
	PVAvailable    int64               `json:"pvAvailable"`
	PVReleased     int64               `json:"pvReleased"`
	PVFailed       int64               `json:"pvFailed"`
	PVCTotal       int64               `json:"pvcTotal"`
	PVCBound       int64               `json:"pvcBound"`
	PVCPending     int64               `json:"pvcPending"`
	PVCLost        int64               `json:"pvcLost"`
	StorageClasses []StorageClassUsage `json:"storageClasses"`
}

// StorageClassUsage StorageClass 使用情况
type StorageClassUsage struct {
	StorageClass string `json:"storageClass"`
	PVCCount     int64  `json:"pvcCount"`
	TotalBytes   int64  `json:"totalBytes"`
}

// ==================== Namespace 统计 ====================

// ClusterNamespaceMetrics Namespace 统计
type ClusterNamespaceMetrics struct {
	Total       int64                      `json:"total"`
	Active      int64                      `json:"active"`
	TopByCPU    []NamespaceResourceRanking `json:"topByCPU"`
	TopByMemory []NamespaceResourceRanking `json:"topByMemory"`
	TopByPods   []NamespaceResourceRanking `json:"topByPods"`
}

// NamespaceResourceRanking Namespace 资源排名
type NamespaceResourceRanking struct {
	Namespace string  `json:"namespace"`
	Value     float64 `json:"value"`
	Unit      string  `json:"unit"`
}

// ==================== ClusterOperator 接口 ====================

// ClusterOperator 集群操作器接口
type ClusterOperator interface {
	// 综合查询
	GetClusterOverview(timeRange *TimeRange) (*ClusterOverview, error)

	// 资源查询
	GetClusterResources(timeRange *TimeRange) (*ClusterResourceMetrics, error)
	GetClusterCPUMetrics(timeRange *TimeRange) (*ClusterResourceSummary, error)
	GetClusterMemoryMetrics(timeRange *TimeRange) (*ClusterResourceSummary, error)

	// 节点查询
	GetClusterNodes(timeRange *TimeRange) (*ClusterNodeMetrics, error)

	// 控制平面查询
	GetAPIServerMetrics(timeRange *TimeRange) (*APIServerMetrics, error)
	GetEtcdMetrics(timeRange *TimeRange) (*EtcdMetrics, error)
	GetSchedulerMetrics(timeRange *TimeRange) (*SchedulerMetrics, error)
	GetControllerManagerMetrics(timeRange *TimeRange) (*ControllerManagerMetrics, error)

	// 工作负载查询
	GetClusterWorkloads(timeRange *TimeRange) (*ClusterWorkloadMetrics, error)
	GetClusterPods(timeRange *TimeRange) (*ClusterPodMetrics, error)
	GetClusterDeployments() (*ClusterDeploymentMetrics, error)

	// 网络和存储查询
	GetClusterNetwork(timeRange *TimeRange) (*ClusterNetworkMetrics, error)
	GetClusterStorage() (*ClusterStorageMetrics, error)

	// Namespace 查询
	GetClusterNamespaces(timeRange *TimeRange) (*ClusterNamespaceMetrics, error)
}
