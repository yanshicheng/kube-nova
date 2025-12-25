package types

import "time"

// ==================== Node 级别类型 ====================

// NodeMetrics 节点综合指标
type NodeMetrics struct {
	NodeName  string             `json:"nodeName"`
	Timestamp time.Time          `json:"timestamp"`
	CPU       NodeCPUMetrics     `json:"cpu"`
	Memory    NodeMemoryMetrics  `json:"memory"`
	Disk      NodeDiskMetrics    `json:"disk"`
	Network   NodeNetworkMetrics `json:"network"`
	K8sStatus NodeK8sStatus      `json:"k8sStatus"`
	System    NodeSystemMetrics  `json:"system"`
}

// ==================== CPU 监控 ====================

// NodeCPUMetrics 节点 CPU 指标
type NodeCPUMetrics struct {
	Current NodeCPUSnapshot    `json:"current"`
	Trend   []NodeCPUDataPoint `json:"trend"`
	Summary NodeCPUSummary     `json:"summary"`
}

// NodeCPUSnapshot CPU 快照
type NodeCPUSnapshot struct {
	Timestamp         time.Time `json:"timestamp"`
	TotalCores        int       `json:"totalCores"`
	UsagePercent      float64   `json:"usagePercent"`      // 总使用率
	UserPercent       float64   `json:"userPercent"`       // 用户态
	SystemPercent     float64   `json:"systemPercent"`     // 系统态
	IowaitPercent     float64   `json:"iowaitPercent"`     // IO 等待
	StealPercent      float64   `json:"stealPercent"`      // 偷取
	IrqPercent        float64   `json:"irqPercent"`        // 硬中断
	SoftirqPercent    float64   `json:"softirqPercent"`    // 软中断
	Load1             float64   `json:"load1"`             // 1分钟负载
	Load5             float64   `json:"load5"`             // 5分钟负载
	Load15            float64   `json:"load15"`            // 15分钟负载
	ContextSwitchRate float64   `json:"contextSwitchRate"` // 上下文切换率
}

// NodeCPUDataPoint CPU 数据点
type NodeCPUDataPoint struct {
	Timestamp    time.Time `json:"timestamp"`
	UsagePercent float64   `json:"usagePercent"`
	Load5        float64   `json:"load5"`
}

// NodeCPUSummary CPU 统计
type NodeCPUSummary struct {
	AvgUsagePercent float64 `json:"avgUsagePercent"`
	MaxUsagePercent float64 `json:"maxUsagePercent"`
	MinUsagePercent float64 `json:"minUsagePercent"`
	AvgLoad5        float64 `json:"avgLoad5"`
	MaxLoad5        float64 `json:"maxLoad5"`
}

// ==================== 内存监控 ====================

// NodeMemoryMetrics 节点内存指标
type NodeMemoryMetrics struct {
	Current NodeMemorySnapshot    `json:"current"`
	Trend   []NodeMemoryDataPoint `json:"trend"`
	Summary NodeMemorySummary     `json:"summary"`
}

// NodeMemorySnapshot 内存快照
type NodeMemorySnapshot struct {
	Timestamp        time.Time `json:"timestamp"`
	TotalBytes       int64     `json:"totalBytes"`
	UsedBytes        int64     `json:"usedBytes"`
	AvailableBytes   int64     `json:"availableBytes"`
	FreeBytes        int64     `json:"freeBytes"`
	BuffersBytes     int64     `json:"buffersBytes"`
	CachedBytes      int64     `json:"cachedBytes"`
	ActiveBytes      int64     `json:"activeBytes"`
	InactiveBytes    int64     `json:"inactiveBytes"`
	UsagePercent     float64   `json:"usagePercent"`
	SwapTotalBytes   int64     `json:"swapTotalBytes"`
	SwapUsedBytes    int64     `json:"swapUsedBytes"`
	SwapFreeBytes    int64     `json:"swapFreeBytes"`
	SwapUsagePercent float64   `json:"swapUsagePercent"`
	SwapInRate       float64   `json:"swapInRate"`  // 页面换入速率
	SwapOutRate      float64   `json:"swapOutRate"` // 页面换出速率
}

// NodeMemoryDataPoint 内存数据点
type NodeMemoryDataPoint struct {
	Timestamp      time.Time `json:"timestamp"`
	UsedBytes      int64     `json:"usedBytes"`
	UsagePercent   float64   `json:"usagePercent"`
	AvailableBytes int64     `json:"availableBytes"`
}

// NodeMemorySummary 内存统计
type NodeMemorySummary struct {
	AvgUsagePercent float64 `json:"avgUsagePercent"`
	MaxUsagePercent float64 `json:"maxUsagePercent"`
	MinUsagePercent float64 `json:"minUsagePercent"`
	AvgSwapUsage    float64 `json:"avgSwapUsage"`
	MaxSwapUsage    float64 `json:"maxSwapUsage"`
}

// ==================== 磁盘监控 ====================

// NodeDiskMetrics 节点磁盘指标
type NodeDiskMetrics struct {
	Filesystems []NodeFilesystemMetrics `json:"filesystems"`
	Devices     []NodeDiskDeviceMetrics `json:"devices"`
}

// NodeFilesystemMetrics 文件系统指标
type NodeFilesystemMetrics struct {
	Mountpoint string                    `json:"mountpoint"`
	Device     string                    `json:"device"`
	FSType     string                    `json:"fsType"`
	Current    NodeFilesystemSnapshot    `json:"current"`
	Trend      []NodeFilesystemDataPoint `json:"trend"`
}

// NodeFilesystemSnapshot 文件系统快照
type NodeFilesystemSnapshot struct {
	Timestamp      time.Time `json:"timestamp"`
	TotalBytes     int64     `json:"totalBytes"`
	UsedBytes      int64     `json:"usedBytes"`
	AvailableBytes int64     `json:"availableBytes"`
	UsagePercent   float64   `json:"usagePercent"`
	TotalInodes    int64     `json:"totalInodes"`
	UsedInodes     int64     `json:"usedInodes"`
	FreeInodes     int64     `json:"freeInodes"`
	InodePercent   float64   `json:"inodePercent"`
}

// NodeFilesystemDataPoint 文件系统数据点
type NodeFilesystemDataPoint struct {
	Timestamp    time.Time `json:"timestamp"`
	UsedBytes    int64     `json:"usedBytes"`
	UsagePercent float64   `json:"usagePercent"`
}

// NodeDiskDeviceMetrics 磁盘设备指标
type NodeDiskDeviceMetrics struct {
	Device  string                    `json:"device"`
	Current NodeDiskDeviceSnapshot    `json:"current"`
	Trend   []NodeDiskDeviceDataPoint `json:"trend"`
	Summary NodeDiskDeviceSummary     `json:"summary"`
}

// NodeDiskDeviceSnapshot 磁盘设备快照
type NodeDiskDeviceSnapshot struct {
	Timestamp            time.Time `json:"timestamp"`
	ReadIOPS             float64   `json:"readIOPS"`
	WriteIOPS            float64   `json:"writeIOPS"`
	ReadBytesPerSec      float64   `json:"readBytesPerSec"`
	WriteBytesPerSec     float64   `json:"writeBytesPerSec"`
	IOUtilizationPercent float64   `json:"ioUtilizationPercent"`
	AvgAwaitMs           float64   `json:"avgAwaitMs"` // 平均等待时间
}

// NodeDiskDeviceDataPoint 磁盘设备数据点
type NodeDiskDeviceDataPoint struct {
	Timestamp            time.Time `json:"timestamp"`
	ReadBytesPerSec      float64   `json:"readBytesPerSec"`
	WriteBytesPerSec     float64   `json:"writeBytesPerSec"`
	IOUtilizationPercent float64   `json:"ioUtilizationPercent"`
}

// NodeDiskDeviceSummary 磁盘设备统计
type NodeDiskDeviceSummary struct {
	AvgReadBytesPerSec  float64 `json:"avgReadBytesPerSec"`
	MaxReadBytesPerSec  float64 `json:"maxReadBytesPerSec"`
	AvgWriteBytesPerSec float64 `json:"avgWriteBytesPerSec"`
	MaxWriteBytesPerSec float64 `json:"maxWriteBytesPerSec"`
	AvgIOUtilization    float64 `json:"avgIOUtilization"`
	MaxIOUtilization    float64 `json:"maxIOUtilization"`
}

// ==================== 网络监控 ====================

// NodeNetworkMetrics 节点网络指标
type NodeNetworkMetrics struct {
	Interfaces []NodeNetworkInterfaceMetrics `json:"interfaces"`
	TCP        NodeTCPMetrics                `json:"tcp"`
}

// NodeNetworkInterfaceMetrics 网络接口指标
type NodeNetworkInterfaceMetrics struct {
	InterfaceName string                          `json:"interfaceName"`
	Current       NodeNetworkInterfaceSnapshot    `json:"current"`
	Trend         []NodeNetworkInterfaceDataPoint `json:"trend"`
	Summary       NodeNetworkInterfaceSummary     `json:"summary"`
}

// NodeNetworkInterfaceSnapshot 网络接口快照
type NodeNetworkInterfaceSnapshot struct {
	Timestamp             time.Time `json:"timestamp"`
	ReceiveBytesPerSec    float64   `json:"receiveBytesPerSec"`
	TransmitBytesPerSec   float64   `json:"transmitBytesPerSec"`
	ReceivePacketsPerSec  float64   `json:"receivePacketsPerSec"`
	TransmitPacketsPerSec float64   `json:"transmitPacketsPerSec"`
	ReceiveErrorsRate     float64   `json:"receiveErrorsRate"`
	TransmitErrorsRate    float64   `json:"transmitErrorsRate"`
	ReceiveDropsRate      float64   `json:"receiveDropsRate"`
	TransmitDropsRate     float64   `json:"transmitDropsRate"`
}

// NodeNetworkInterfaceDataPoint 网络接口数据点
type NodeNetworkInterfaceDataPoint struct {
	Timestamp           time.Time `json:"timestamp"`
	ReceiveBytesPerSec  float64   `json:"receiveBytesPerSec"`
	TransmitBytesPerSec float64   `json:"transmitBytesPerSec"`
}

// NodeNetworkInterfaceSummary 网络接口统计
type NodeNetworkInterfaceSummary struct {
	AvgReceiveBytesPerSec  float64 `json:"avgReceiveBytesPerSec"`
	MaxReceiveBytesPerSec  float64 `json:"maxReceiveBytesPerSec"`
	AvgTransmitBytesPerSec float64 `json:"avgTransmitBytesPerSec"`
	MaxTransmitBytesPerSec float64 `json:"maxTransmitBytesPerSec"`
	TotalReceiveErrors     int64   `json:"totalReceiveErrors"`
	TotalTransmitErrors    int64   `json:"totalTransmitErrors"`
	TotalReceiveDrops      int64   `json:"totalReceiveDrops"`
	TotalTransmitDrops     int64   `json:"totalTransmitDrops"`
}

// NodeTCPMetrics TCP 连接指标
type NodeTCPMetrics struct {
	Current NodeTCPSnapshot    `json:"current"`
	Trend   []NodeTCPDataPoint `json:"trend"`
}

// NodeTCPSnapshot TCP 快照
type NodeTCPSnapshot struct {
	Timestamp              time.Time `json:"timestamp"`
	EstablishedConnections int64     `json:"establishedConnections"`
	TimeWaitConnections    int64     `json:"timeWaitConnections"`
	ActiveOpensRate        float64   `json:"activeOpensRate"`
	PassiveOpensRate       float64   `json:"passiveOpensRate"`
	SocketsInUse           int64     `json:"socketsInUse"`
}

// NodeTCPDataPoint TCP 数据点
type NodeTCPDataPoint struct {
	Timestamp              time.Time `json:"timestamp"`
	EstablishedConnections int64     `json:"establishedConnections"`
	TimeWaitConnections    int64     `json:"timeWaitConnections"`
}

// ==================== Kubernetes 节点状态 ====================

// NodeK8sStatus Kubernetes 节点状态
type NodeK8sStatus struct {
	Conditions     []NodeCondition      `json:"conditions"`
	Capacity       NodeResourceQuantity `json:"capacity"`
	Allocatable    NodeResourceQuantity `json:"allocatable"`
	Allocated      NodeResourceQuantity `json:"allocated"`
	KubeletMetrics NodeKubeletMetrics   `json:"kubeletMetrics"`
	NodeInfo       NodeInfo             `json:"nodeInfo"`
	Taints         []NodeTaint          `json:"taints"`
	Labels         map[string]string    `json:"labels"`
	Annotations    map[string]string    `json:"annotations"`
}

// NodeCondition 节点状况
type NodeCondition struct {
	Type               string    `json:"type"` // Ready, MemoryPressure, DiskPressure, PIDPressure, NetworkUnavailable
	Status             bool      `json:"status"`
	Reason             string    `json:"reason,omitempty"`
	Message            string    `json:"message,omitempty"`
	LastTransitionTime time.Time `json:"lastTransitionTime"`
}

// NodeResourceQuantity 资源数量
type NodeResourceQuantity struct {
	CPUCores         float64 `json:"cpuCores"`
	MemoryBytes      int64   `json:"memoryBytes"`
	Pods             int64   `json:"pods"`
	EphemeralStorage int64   `json:"ephemeralStorage"`
}

// NodeKubeletMetrics kubelet 指标
type NodeKubeletMetrics struct {
	RunningPods         int64                  `json:"runningPods"`
	RunningContainers   int64                  `json:"runningContainers"`
	PLEGRelistDuration  float64                `json:"plegRelistDuration"` // PLEG 延迟（秒）
	RuntimeOpsRate      float64                `json:"runtimeOpsRate"`
	RuntimeOpsErrorRate float64                `json:"runtimeOpsErrorRate"`
	RuntimeOpsDuration  float64                `json:"runtimeOpsDuration"`
	Trend               []NodeKubeletDataPoint `json:"trend"`
}

// NodeKubeletDataPoint kubelet 数据点
type NodeKubeletDataPoint struct {
	Timestamp           time.Time `json:"timestamp"`
	PLEGRelistDuration  float64   `json:"plegRelistDuration"`
	RuntimeOpsRate      float64   `json:"runtimeOpsRate"`
	RuntimeOpsErrorRate float64   `json:"runtimeOpsErrorRate"`
}

// NodeInfo 节点信息
type NodeInfo struct {
	KubeletVersion          string    `json:"kubeletVersion"`
	ContainerRuntimeVersion string    `json:"containerRuntimeVersion"`
	KernelVersion           string    `json:"kernelVersion"`
	OSImage                 string    `json:"osImage"`
	Architecture            string    `json:"architecture"`
	BootTime                time.Time `json:"bootTime"`
	UptimeSeconds           int64     `json:"uptimeSeconds"`
}

// NodeTaint 节点污点
type NodeTaint struct {
	Key    string `json:"key"`
	Value  string `json:"value"`
	Effect string `json:"effect"` // NoSchedule, PreferNoSchedule, NoExecute
}

// ==================== 系统级指标 ====================

// NodeSystemMetrics 系统级指标
type NodeSystemMetrics struct {
	Processes       NodeProcessMetrics        `json:"processes"`
	FileDescriptors NodeFileDescriptorMetrics `json:"fileDescriptors"`
}

// NodeProcessMetrics 进程指标
type NodeProcessMetrics struct {
	Running int64 `json:"running"`
	Blocked int64 `json:"blocked"`
}

// NodeFileDescriptorMetrics 文件描述符指标
type NodeFileDescriptorMetrics struct {
	Allocated    int64                         `json:"allocated"`
	Maximum      int64                         `json:"maximum"`
	UsagePercent float64                       `json:"usagePercent"`
	Trend        []NodeFileDescriptorDataPoint `json:"trend"`
}

// NodeFileDescriptorDataPoint 文件描述符数据点
type NodeFileDescriptorDataPoint struct {
	Timestamp    time.Time `json:"timestamp"`
	Allocated    int64     `json:"allocated"`
	UsagePercent float64   `json:"usagePercent"`
}

// ==================== Node 上的 Pod 信息 ====================

// NodePodMetrics 节点上的 Pod 指标
type NodePodMetrics struct {
	NodeName     string             `json:"nodeName"`
	TotalPods    int64              `json:"totalPods"`
	RunningPods  int64              `json:"runningPods"`
	PendingPods  int64              `json:"pendingPods"`
	FailedPods   int64              `json:"failedPods"`
	TopPodsByCPU []PodResourceUsage `json:"topPodsByCPU"`
	TopPodsByMem []PodResourceUsage `json:"topPodsByMem"`
	PodList      []NodePodBrief     `json:"podList"`
}

// PodResourceUsage Pod 资源使用
type PodResourceUsage struct {
	Namespace string  `json:"namespace"`
	PodName   string  `json:"podName"`
	Value     float64 `json:"value"`
	Unit      string  `json:"unit"`
}

// NodePodBrief 节点上 Pod 简要信息
type NodePodBrief struct {
	Namespace    string  `json:"namespace"`
	PodName      string  `json:"podName"`
	Phase        string  `json:"phase"`
	CPUUsage     float64 `json:"cpuUsage"`
	MemoryUsage  int64   `json:"memoryUsage"`
	RestartCount int64   `json:"restartCount"`
}

// ==================== Node 对比 ====================

// NodeComparison 多个节点对比
type NodeComparison struct {
	Nodes     []NodeComparisonItem `json:"nodes"`
	Timestamp time.Time            `json:"timestamp"`
}

// NodeComparisonItem 节点对比项
type NodeComparisonItem struct {
	NodeName    string  `json:"nodeName"`
	CPUUsage    float64 `json:"cpuUsage"`
	MemoryUsage float64 `json:"memoryUsage"`
	DiskUsage   float64 `json:"diskUsage"`
	Load5       float64 `json:"load5"`
	PodCount    int64   `json:"podCount"`
	Ready       bool    `json:"ready"`
}

// ==================== Node 排行 ====================

// NodeRanking 节点排行
type NodeRanking struct {
	TopByCPU    []NodeRankingItem `json:"topByCPU"`
	TopByMemory []NodeRankingItem `json:"topByMemory"`
	TopByDisk   []NodeRankingItem `json:"topByDisk"`
	TopByLoad   []NodeRankingItem `json:"topByLoad"`
	TopByPods   []NodeRankingItem `json:"topByPods"`
}

// NodeRankingItem 节点排行项
type NodeRankingItem struct {
	NodeName string  `json:"nodeName"`
	Value    float64 `json:"value"`
	Unit     string  `json:"unit"`
}

// ==================== NodeOperator 接口 ====================

// NodeOperator 节点操作器接口
type NodeOperator interface {
	// 综合查询
	GetNodeMetrics(nodeName string, timeRange *TimeRange) (*NodeMetrics, error)

	// CPU 查询
	GetNodeCPU(nodeName string, timeRange *TimeRange) (*NodeCPUMetrics, error)

	// 内存查询
	GetNodeMemory(nodeName string, timeRange *TimeRange) (*NodeMemoryMetrics, error)

	// 磁盘查询
	GetNodeDisk(nodeName string, timeRange *TimeRange) (*NodeDiskMetrics, error)
	GetNodeFilesystems(nodeName string, timeRange *TimeRange) ([]NodeFilesystemMetrics, error)
	GetNodeDiskDevices(nodeName string, timeRange *TimeRange) ([]NodeDiskDeviceMetrics, error)

	// 网络查询
	GetNodeNetwork(nodeName string, timeRange *TimeRange) (*NodeNetworkMetrics, error)
	GetNodeNetworkInterfaces(nodeName string, timeRange *TimeRange) ([]NodeNetworkInterfaceMetrics, error)
	GetNodeTCP(nodeName string, timeRange *TimeRange) (*NodeTCPMetrics, error)
	// NodeOperator 接口中添加
	GetNodeNetworkInterface(nodeName string, interfaceName string, timeRange *TimeRange) (*NodeNetworkInterfaceMetrics, error)

	// Kubernetes 状态查询
	GetNodeK8sStatus(nodeName string, timeRange *TimeRange) (*NodeK8sStatus, error)
	GetNodeConditions(nodeName string) ([]NodeCondition, error)
	GetNodeCapacity(nodeName string) (*NodeResourceQuantity, error)
	GetNodeAllocatable(nodeName string) (*NodeResourceQuantity, error)
	GetNodeAllocated(nodeName string) (*NodeResourceQuantity, error)
	GetNodeKubelet(nodeName string, timeRange *TimeRange) (*NodeKubeletMetrics, error)

	// 系统查询
	GetNodeSystem(nodeName string, timeRange *TimeRange) (*NodeSystemMetrics, error)

	// Pod 查询
	GetNodePods(nodeName string, timeRange *TimeRange) (*NodePodMetrics, error)

	// 对比和排行
	CompareNodes(nodeNames []string, timeRange *TimeRange) (*NodeComparison, error)
	GetNodeRanking(limit int, timeRange *TimeRange) (*NodeRanking, error)

	// 列表查询
	ListNodesMetrics(timeRange *TimeRange) ([]NodeMetrics, error)
}
