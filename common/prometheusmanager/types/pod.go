package types

import "time"

// ==================== 基础类型 ====================

// TimeRange 时间范围
type TimeRange struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
	Step  string    `json:"step"` // 查询步长，如 "15s", "1m", "5m"
}

// InstantQueryResult 即时查询结果
type InstantQueryResult struct {
	Metric map[string]string `json:"metric"`
	Value  float64           `json:"value"`
	Time   time.Time         `json:"time"`
}

// RangeQueryResult 范围查询结果
type RangeQueryResult struct {
	Metric map[string]string `json:"metric"`
	Values []MetricValue     `json:"values"`
}

// MetricValue 指标值
type MetricValue struct {
	Timestamp time.Time `json:"timestamp"`
	Value     float64   `json:"value"`
}

// ==================== CPU 指标 ====================

// PodCPUMetrics Pod CPU 指标
type PodCPUMetrics struct {
	Namespace string              `json:"namespace"`
	PodName   string              `json:"podName"`
	Current   CPUUsageSnapshot    `json:"current"` // 当前使用情况
	Trend     []CPUUsageDataPoint `json:"trend"`   // 使用趋势
	Limits    CPULimits           `json:"limits"`  // 资源限制
	Summary   CPUSummary          `json:"summary"` // 汇总统计
}

// CPUUsageSnapshot CPU 使用快照
type CPUUsageSnapshot struct {
	Timestamp     time.Time `json:"timestamp"`
	UsageCores    float64   `json:"usageCores"`    // 当前使用核心数
	UsagePercent  float64   `json:"usagePercent"`  // 使用率百分比
	RequestCores  float64   `json:"requestCores"`  // 请求核心数
	LimitCores    float64   `json:"limitCores"`    // 限制核心数
	ThrottledTime float64   `json:"throttledTime"` // 节流时间（秒）
}

// CPUUsageDataPoint CPU 使用数据点
type CPUUsageDataPoint struct {
	Timestamp    time.Time `json:"timestamp"`
	UsageCores   float64   `json:"usageCores"`
	UsagePercent float64   `json:"usagePercent"`
}

// CPULimits CPU 资源限制
type CPULimits struct {
	RequestCores float64 `json:"requestCores"` // CPU 请求
	LimitCores   float64 `json:"limitCores"`   // CPU 限制
	HasLimit     bool    `json:"hasLimit"`     // 是否设置了限制
}

// CPUSummary CPU 统计汇总
type CPUSummary struct {
	AvgUsageCores    float64 `json:"avgUsageCores"`
	MaxUsageCores    float64 `json:"maxUsageCores"`
	MinUsageCores    float64 `json:"minUsageCores"`
	AvgUsagePercent  float64 `json:"avgUsagePercent"`
	MaxUsagePercent  float64 `json:"maxUsagePercent"`
	TotalThrottled   float64 `json:"totalThrottled"`   // 总节流时间
	ThrottledPercent float64 `json:"throttledPercent"` // 节流百分比
}

// ContainerCPUMetrics 容器 CPU 指标
type ContainerCPUMetrics struct {
	Namespace     string              `json:"namespace"`
	PodName       string              `json:"podName"`
	ContainerName string              `json:"containerName"`
	Current       CPUUsageSnapshot    `json:"current"`
	Trend         []CPUUsageDataPoint `json:"trend"`
	Limits        CPULimits           `json:"limits"`
	Summary       CPUSummary          `json:"summary"`
}

// CPUThrottlingMetrics CPU 节流指标
type CPUThrottlingMetrics struct {
	Namespace        string                   `json:"namespace"`
	PodName          string                   `json:"podName"`
	TotalThrottled   float64                  `json:"totalThrottled"`   // 总节流时间
	ThrottledPercent float64                  `json:"throttledPercent"` // 节流百分比
	Trend            []CPUThrottlingDataPoint `json:"trend"`
	ByContainer      []ContainerThrottling    `json:"byContainer"` // 按容器分组
}

// CPUThrottlingDataPoint CPU 节流数据点
type CPUThrottlingDataPoint struct {
	Timestamp        time.Time `json:"timestamp"`
	ThrottledSeconds float64   `json:"throttledSeconds"`
	ThrottledPercent float64   `json:"throttledPercent"`
}

// ContainerThrottling 容器节流情况
type ContainerThrottling struct {
	ContainerName    string  `json:"containerName"`
	ThrottledSeconds float64 `json:"throttledSeconds"`
	ThrottledPercent float64 `json:"throttledPercent"`
}

// ==================== 内存指标 ====================

// PodMemoryMetrics Pod 内存指标
type PodMemoryMetrics struct {
	Namespace string                 `json:"namespace"`
	PodName   string                 `json:"podName"`
	Current   MemoryUsageSnapshot    `json:"current"`
	Trend     []MemoryUsageDataPoint `json:"trend"`
	Limits    MemoryLimits           `json:"limits"`
	Summary   MemorySummary          `json:"summary"`
}

// MemoryUsageSnapshot 内存使用快照
type MemoryUsageSnapshot struct {
	Timestamp       time.Time `json:"timestamp"`
	UsageBytes      int64     `json:"usageBytes"`
	UsagePercent    float64   `json:"usagePercent"`
	RequestBytes    int64     `json:"requestBytes"`
	LimitBytes      int64     `json:"limitBytes"`
	WorkingSetBytes int64     `json:"workingSetBytes"` // 工作集内存
	RSSBytes        int64     `json:"rssBytes"`        // 常驻内存
	CacheBytes      int64     `json:"cacheBytes"`      // 缓存内存
}

// MemoryUsageDataPoint 内存使用数据点
type MemoryUsageDataPoint struct {
	Timestamp       time.Time `json:"timestamp"`
	UsageBytes      int64     `json:"usageBytes"`      // 使用字节数
	UsagePercent    float64   `json:"usagePercent"`    // 使用百分比
	WorkingSetBytes int64     `json:"workingSetBytes"` // 工作集内存
}

// MemorySummary 内存统计汇总
type MemorySummary struct {
	AvgUsageBytes   int64   `json:"avgUsageBytes"`   // 平均使用字节数
	MaxUsageBytes   int64   `json:"maxUsageBytes"`   // 最大使用字节数
	MinUsageBytes   int64   `json:"minUsageBytes"`   // 最小使用字节数
	AvgUsagePercent float64 `json:"avgUsagePercent"` // 平均使用百分比
	MaxUsagePercent float64 `json:"maxUsagePercent"` // 最大使用百分比
	OOMKills        int64   `json:"oomKills"`        // OOM 杀死次数
}

// MemoryLimits 内存资源限制
type MemoryLimits struct {
	RequestBytes int64 `json:"requestBytes"`
	LimitBytes   int64 `json:"limitBytes"`
	HasLimit     bool  `json:"hasLimit"`
}

// ContainerMemoryMetrics 容器内存指标
type ContainerMemoryMetrics struct {
	Namespace     string                 `json:"namespace"`
	PodName       string                 `json:"podName"`
	ContainerName string                 `json:"containerName"`
	Current       MemoryUsageSnapshot    `json:"current"`
	Trend         []MemoryUsageDataPoint `json:"trend"`
	Limits        MemoryLimits           `json:"limits"`
	Summary       MemorySummary          `json:"summary"`
}

// OOMMetrics OOM 指标
type OOMMetrics struct {
	Namespace     string         `json:"namespace"`
	PodName       string         `json:"podName"`
	TotalOOMKills int64          `json:"totalOOMKills"`
	ByContainer   []ContainerOOM `json:"byContainer"`
	Events        []OOMEvent     `json:"events"`
}

// ContainerOOM 容器 OOM 情况
type ContainerOOM struct {
	ContainerName string `json:"containerName"`
	OOMKills      int64  `json:"oomKills"`
}

// OOMEvent OOM 事件
type OOMEvent struct {
	Timestamp     time.Time `json:"timestamp"`
	ContainerName string    `json:"containerName"`
	Reason        string    `json:"reason"`
}

// ==================== 网络指标 ====================

// NetworkMetrics 网络 I/O 指标（Pod 级别）
type NetworkMetrics struct {
	Namespace string             `json:"namespace"`
	PodName   string             `json:"podName"`
	Current   NetworkSnapshot    `json:"current"`
	Trend     []NetworkDataPoint `json:"trend"`
	Summary   NetworkSummary     `json:"summary"`
}

// ContainerNetworkMetrics 容器网络 I/O 指标
type ContainerNetworkMetrics struct {
	Namespace     string             `json:"namespace"`
	PodName       string             `json:"podName"`
	ContainerName string             `json:"containerName"`
	Current       NetworkSnapshot    `json:"current"`
	Trend         []NetworkDataPoint `json:"trend"`
	Summary       NetworkSummary     `json:"summary"`
}

// NetworkSnapshot 网络快照
type NetworkSnapshot struct {
	Timestamp       time.Time `json:"timestamp"`
	ReceiveBytes    int64     `json:"receiveBytes"`    // 接收字节数
	TransmitBytes   int64     `json:"transmitBytes"`   // 发送字节数
	ReceivePackets  int64     `json:"receivePackets"`  // 接收包数
	TransmitPackets int64     `json:"transmitPackets"` // 发送包数
	ReceiveErrors   int64     `json:"receiveErrors"`   // 接收错误数
	TransmitErrors  int64     `json:"transmitErrors"`  // 发送错误数
}

// NetworkDataPoint 网络数据点 (速率)
type NetworkDataPoint struct {
	Timestamp     time.Time `json:"timestamp"`
	ReceiveBytes  int64     `json:"receiveBytes"`  // Bytes/s (速率)
	TransmitBytes int64     `json:"transmitBytes"` // Bytes/s (速率)
}

// NetworkSummary 网络统计汇总
type NetworkSummary struct {
	TotalReceiveBytes      int64 `json:"totalReceiveBytes"`      // 总接收字节数
	TotalTransmitBytes     int64 `json:"totalTransmitBytes"`     // 总发送字节数
	TotalReceivePackets    int64 `json:"totalReceivePackets"`    // 总接收包数
	TotalTransmitPackets   int64 `json:"totalTransmitPackets"`   // 总发送包数
	TotalErrors            int64 `json:"totalErrors"`            // 总错误数
	MaxReceiveBytesPerSec  int64 `json:"maxReceiveBytesPerSec"`  // 最大接收速率 (Bytes/s)
	MaxTransmitBytesPerSec int64 `json:"maxTransmitBytesPerSec"` // 最大发送速率 (Bytes/s)
	AvgReceiveBytesPerSec  int64 `json:"avgReceiveBytesPerSec"`  // 平均接收速率 (Bytes/s)
	AvgTransmitBytesPerSec int64 `json:"avgTransmitBytesPerSec"` // 平均发送速率 (Bytes/s)
}

// NetworkRateMetrics 网络速率指标（Pod 级别）
type NetworkRateMetrics struct {
	Namespace string                 `json:"namespace"`
	PodName   string                 `json:"podName"`
	Current   NetworkRateSnapshot    `json:"current"`
	Trend     []NetworkRateDataPoint `json:"trend"`
	Summary   NetworkRateSummary     `json:"summary"`
}

// ContainerNetworkRateMetrics 容器网络速率指标
type ContainerNetworkRateMetrics struct {
	Namespace     string                 `json:"namespace"`
	PodName       string                 `json:"podName"`
	ContainerName string                 `json:"containerName"`
	Current       NetworkRateSnapshot    `json:"current"`
	Trend         []NetworkRateDataPoint `json:"trend"`
	Summary       NetworkRateSummary     `json:"summary"`
}

// NetworkRateSnapshot 网络速率快照
type NetworkRateSnapshot struct {
	Timestamp           time.Time `json:"timestamp"`
	ReceiveBytesPerSec  float64   `json:"receiveBytesPerSec"`  // 接收速率 (Bytes/s)
	TransmitBytesPerSec float64   `json:"transmitBytesPerSec"` // 发送速率 (Bytes/s)
}

// NetworkRateDataPoint 网络速率数据点
type NetworkRateDataPoint struct {
	Timestamp           time.Time `json:"timestamp"`
	ReceiveBytesPerSec  float64   `json:"receiveBytesPerSec"`  // 接收速率 (Bytes/s)
	TransmitBytesPerSec float64   `json:"transmitBytesPerSec"` // 发送速率 (Bytes/s)
}

// NetworkRateSummary 网络速率统计
type NetworkRateSummary struct {
	AvgReceiveBytesPerSec  int64 `json:"avgReceiveBytesPerSec"`  // 平均接收速率 (Bytes/s)
	MaxReceiveBytesPerSec  int64 `json:"maxReceiveBytesPerSec"`  // 最大接收速率 (Bytes/s)
	AvgTransmitBytesPerSec int64 `json:"avgTransmitBytesPerSec"` // 平均发送速率 (Bytes/s)
	MaxTransmitBytesPerSec int64 `json:"maxTransmitBytesPerSec"` // 最大发送速率 (Bytes/s)
}

// ==================== 磁盘指标 ====================

// DiskMetrics 磁盘 I/O 指标（Pod 级别）
type DiskMetrics struct {
	Namespace string          `json:"namespace"`
	PodName   string          `json:"podName"`
	Current   DiskSnapshot    `json:"current"`
	Trend     []DiskDataPoint `json:"trend"`
	Summary   DiskSummary     `json:"summary"`
}

// DiskSnapshot 磁盘快照
type DiskSnapshot struct {
	Timestamp  time.Time `json:"timestamp"`
	ReadBytes  int64     `json:"readBytes"`  // 读取字节数
	WriteBytes int64     `json:"writeBytes"` // 写入字节数
	ReadOps    int64     `json:"readOps"`    // 读操作数
	WriteOps   int64     `json:"writeOps"`   // 写操作数
}

// DiskDataPoint 磁盘数据点 (速率)
type DiskDataPoint struct {
	Timestamp  time.Time `json:"timestamp"`
	ReadBytes  int64     `json:"readBytes"`  // Bytes/s (速率)
	WriteBytes int64     `json:"writeBytes"` // Bytes/s (速率)
}

// DiskSummary 磁盘统计汇总
type DiskSummary struct {
	TotalReadBytes      int64 `json:"totalReadBytes"`      // 总读取字节数
	TotalWriteBytes     int64 `json:"totalWriteBytes"`     // 总写入字节数
	TotalReadOps        int64 `json:"totalReadOps"`        // 总读操作数
	TotalWriteOps       int64 `json:"totalWriteOps"`       // 总写操作数
	MaxReadBytesPerSec  int64 `json:"maxReadBytesPerSec"`  // 最大读速率 (Bytes/s)
	MaxWriteBytesPerSec int64 `json:"maxWriteBytesPerSec"` // 最大写速率 (Bytes/s)
	AvgReadBytesPerSec  int64 `json:"avgReadBytesPerSec"`  // 平均读速率 (Bytes/s)
	AvgWriteBytesPerSec int64 `json:"avgWriteBytesPerSec"` // 平均写速率 (Bytes/s)
}

// DiskRateMetrics 磁盘速率指标
type DiskRateMetrics struct {
	Namespace string              `json:"namespace"`
	PodName   string              `json:"podName"`
	Current   DiskRateSnapshot    `json:"current"`
	Trend     []DiskRateDataPoint `json:"trend"`
	Summary   DiskRateSummary     `json:"summary"`
}

// DiskRateSnapshot 磁盘速率快照
type DiskRateSnapshot struct {
	Timestamp        time.Time `json:"timestamp"`
	ReadBytesPerSec  float64   `json:"readBytesPerSec"`  // 读速率 (Bytes/s)
	WriteBytesPerSec float64   `json:"writeBytesPerSec"` // 写速率 (Bytes/s)
	ReadOpsPerSec    float64   `json:"readOpsPerSec"`    // 读 IOPS
	WriteOpsPerSec   float64   `json:"writeOpsPerSec"`   // 写 IOPS
}

// DiskRateDataPoint 磁盘速率数据点
type DiskRateDataPoint struct {
	Timestamp        time.Time `json:"timestamp"`
	ReadBytesPerSec  float64   `json:"readBytesPerSec"`  // 读速率 (Bytes/s)
	WriteBytesPerSec float64   `json:"writeBytesPerSec"` // 写速率 (Bytes/s)
}

// DiskRateSummary 磁盘速率统计
type DiskRateSummary struct {
	AvgReadBytesPerSec  float64 `json:"avgReadBytesPerSec"`
	MaxReadBytesPerSec  float64 `json:"maxReadBytesPerSec"`
	AvgWriteBytesPerSec float64 `json:"avgWriteBytesPerSec"`
	MaxWriteBytesPerSec float64 `json:"maxWriteBytesPerSec"`
}

// ContainerDiskMetrics 容器磁盘指标
type ContainerDiskMetrics struct {
	Namespace     string          `json:"namespace"`
	PodName       string          `json:"podName"`
	ContainerName string          `json:"containerName"`
	Current       DiskSnapshot    `json:"current"`
	Trend         []DiskDataPoint `json:"trend"`
	Summary       DiskSummary     `json:"summary"`
}

// ==================== Pod 状态指标 ====================

// PodStatusMetrics Pod 状态指标
type PodStatusMetrics struct {
	Namespace   string               `json:"namespace"`
	PodName     string               `json:"podName"`
	Current     PodStatusSnapshot    `json:"current"`
	History     []PodStatusDataPoint `json:"history"`
	Transitions []StatusTransition   `json:"transitions"` // 状态转换记录
}

// PodStatusSnapshot Pod 状态快照
type PodStatusSnapshot struct {
	Timestamp       time.Time        `json:"timestamp"`
	Phase           string           `json:"phase"` // Running, Pending, Succeeded, Failed, Unknown
	Ready           bool             `json:"ready"`
	Reason          string           `json:"reason"`
	Message         string           `json:"message"`
	ContainerStates []ContainerState `json:"containerStates"`
}

// ContainerState 容器状态
type ContainerState struct {
	ContainerName string `json:"containerName"`
	Ready         bool   `json:"ready"`
	RestartCount  int64  `json:"restartCount"`
	State         string `json:"state"` // Running, Waiting, Terminated
	Reason        string `json:"reason"`
}

// PodStatusDataPoint Pod 状态数据点
type PodStatusDataPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Phase     string    `json:"phase"`
	Ready     bool      `json:"ready"`
}

// StatusTransition 状态转换
type StatusTransition struct {
	Timestamp time.Time `json:"timestamp"`
	FromPhase string    `json:"fromPhase"`
	ToPhase   string    `json:"toPhase"`
	Reason    string    `json:"reason"`
}

// RestartMetrics 重启指标
type RestartMetrics struct {
	Namespace      string             `json:"namespace"`
	PodName        string             `json:"podName"`
	TotalRestarts  int64              `json:"totalRestarts"`
	ByContainer    []ContainerRestart `json:"byContainer"`
	RecentRestarts []RestartEvent     `json:"recentRestarts"`
	Trend          []RestartDataPoint `json:"trend"`
}

// ContainerRestart 容器重启情况
type ContainerRestart struct {
	ContainerName string     `json:"containerName"`
	RestartCount  int64      `json:"restartCount"`
	LastRestart   *time.Time `json:"lastRestart,omitempty"`
}

// RestartEvent 重启事件
type RestartEvent struct {
	Timestamp     time.Time `json:"timestamp"`
	ContainerName string    `json:"containerName"`
	Reason        string    `json:"reason"`
	ExitCode      int       `json:"exitCode"`
}

// RestartDataPoint 重启数据点
type RestartDataPoint struct {
	Timestamp    time.Time `json:"timestamp"`
	RestartCount int64     `json:"restartCount"`
}

// PodAgeMetrics Pod 存活时间指标
type PodAgeMetrics struct {
	Namespace     string    `json:"namespace"`
	PodName       string    `json:"podName"`
	CreationTime  time.Time `json:"creationTime"`
	Age           string    `json:"age"` // 人类可读的年龄，如 "2d5h"
	AgeSeconds    int64     `json:"ageSeconds"`
	Uptime        string    `json:"uptime"` // 连续运行时间
	UptimeSeconds int64     `json:"uptimeSeconds"`
}

// ==================== Pod 综合概览 ====================

// PodOverview Pod 综合概览
type PodOverview struct {
	Namespace    string              `json:"namespace"`
	PodName      string              `json:"podName"`
	Status       PodStatusSnapshot   `json:"status"`
	CPU          CPUUsageSnapshot    `json:"cpu"`
	Memory       MemoryUsageSnapshot `json:"memory"`
	Network      NetworkSnapshot     `json:"network"`
	Disk         DiskSnapshot        `json:"disk"`
	RestartCount int64               `json:"restartCount"`
	Age          PodAgeMetrics       `json:"age"`
	Labels       map[string]string   `json:"labels"`
	Annotations  map[string]string   `json:"annotations"`
}

// PodRanking Pod 排行
type PodRanking struct {
	Namespace string  `json:"namespace"`
	PodName   string  `json:"podName"`
	Value     float64 `json:"value"`
	Unit      string  `json:"unit"`
}

// ==================== 存储/Volume 指标 ====================

// PodVolumeMetrics Pod 存储指标
type PodVolumeMetrics struct {
	Namespace     string        `json:"namespace"`
	PodName       string        `json:"podName"`
	Volumes       []VolumeUsage `json:"volumes"`
	TotalCapacity int64         `json:"totalCapacity"`
	TotalUsed     int64         `json:"totalUsed"`
	UsagePercent  float64       `json:"usagePercent"`
}

// VolumeUsage 卷使用情况
type VolumeUsage struct {
	VolumeName    string  `json:"volumeName"`
	MountPath     string  `json:"mountPath"`
	Device        string  `json:"device"`
	CapacityBytes int64   `json:"capacityBytes"`
	UsedBytes     int64   `json:"usedBytes"`
	AvailBytes    int64   `json:"availBytes"`
	UsagePercent  float64 `json:"usagePercent"`
	PVCName       string  `json:"pvcName"`      // PVC 名称
	StorageClass  string  `json:"storageClass"` // 存储类
	AccessMode    string  `json:"accessMode"`   // 访问模式
}

// VolumeIOPSMetrics Volume IOPS 指标
type VolumeIOPSMetrics struct {
	Namespace string          `json:"namespace"`
	PodName   string          `json:"podName"`
	Current   IOPSSnapshot    `json:"current"`
	Trend     []IOPSDataPoint `json:"trend"`
	Summary   IOPSSummary     `json:"summary"`
}

// IOPSSnapshot IOPS 快照
type IOPSSnapshot struct {
	Timestamp time.Time `json:"timestamp"`
	ReadIOPS  float64   `json:"readIOPS"`
	WriteIOPS float64   `json:"writeIOPS"`
	TotalIOPS float64   `json:"totalIOPS"`
}

// IOPSDataPoint IOPS 数据点
type IOPSDataPoint struct {
	Timestamp time.Time `json:"timestamp"`
	ReadIOPS  float64   `json:"readIOPS"`
	WriteIOPS float64   `json:"writeIOPS"`
	TotalIOPS float64   `json:"totalIOPS"`
}

// IOPSSummary IOPS 统计
type IOPSSummary struct {
	AvgReadIOPS  float64 `json:"avgReadIOPS"`
	MaxReadIOPS  float64 `json:"maxReadIOPS"`
	AvgWriteIOPS float64 `json:"avgWriteIOPS"`
	MaxWriteIOPS float64 `json:"maxWriteIOPS"`
}

// ==================== 健康检查/探针指标 ====================

// PodProbeMetrics Pod 探针指标
type PodProbeMetrics struct {
	Namespace       string        `json:"namespace"`
	PodName         string        `json:"podName"`
	LivenessProbes  []ProbeStatus `json:"livenessProbes"`
	ReadinessProbes []ProbeStatus `json:"readinessProbes"`
	StartupProbes   []ProbeStatus `json:"startupProbes"`
}

// ProbeStatus 探针状态
type ProbeStatus struct {
	ContainerName   string     `json:"containerName"`
	ProbeType       string     `json:"probeType"` // HTTP, TCP, Exec
	Status          string     `json:"status"`    // Success, Failure
	SuccessCount    int64      `json:"successCount"`
	FailureCount    int64      `json:"failureCount"`
	TotalProbes     int64      `json:"totalProbes"`
	LastProbeTime   time.Time  `json:"lastProbeTime"`
	LastSuccessTime *time.Time `json:"lastSuccessTime,omitempty"`
	LastFailureTime *time.Time `json:"lastFailureTime,omitempty"`
	FailureReason   string     `json:"failureReason,omitempty"`
}

// ==================== 文件描述符指标 ====================

// FileDescriptorMetrics 文件描述符指标
type FileDescriptorMetrics struct {
	Namespace string        `json:"namespace"`
	PodName   string        `json:"podName"`
	Current   FDSnapshot    `json:"current"`
	Trend     []FDDataPoint `json:"trend"`
	Summary   FDSummary     `json:"summary"`
}

// FDSnapshot 文件描述符快照
type FDSnapshot struct {
	Timestamp    time.Time `json:"timestamp"`
	OpenFDs      int64     `json:"openFDs"`
	MaxFDs       int64     `json:"maxFDs"`
	UsagePercent float64   `json:"usagePercent"`
}

// FDDataPoint 文件描述符数据点
type FDDataPoint struct {
	Timestamp    time.Time `json:"timestamp"`
	OpenFDs      int64     `json:"openFDs"`
	UsagePercent float64   `json:"usagePercent"`
}

// FDSummary 文件描述符统计
type FDSummary struct {
	AvgOpenFDs      int64   `json:"avgOpenFDs"`
	MaxOpenFDs      int64   `json:"maxOpenFDs"`
	AvgUsagePercent float64 `json:"avgUsagePercent"`
	MaxUsagePercent float64 `json:"maxUsagePercent"`
}

// ==================== 网络连接指标 ====================

// NetworkConnectionMetrics 网络连接指标
type NetworkConnectionMetrics struct {
	Namespace string                `json:"namespace"`
	PodName   string                `json:"podName"`
	Current   ConnectionSnapshot    `json:"current"`
	Trend     []ConnectionDataPoint `json:"trend"`
	Summary   ConnectionSummary     `json:"summary"`
}

// ConnectionSnapshot 连接快照
type ConnectionSnapshot struct {
	Timestamp      time.Time `json:"timestamp"`
	TCPConnections int64     `json:"tcpConnections"`
	UDPConnections int64     `json:"udpConnections"`
	TCPListening   int64     `json:"tcpListening"`
	TCPEstablished int64     `json:"tcpEstablished"`
	TCPTimeWait    int64     `json:"tcpTimeWait"`
	TCPCloseWait   int64     `json:"tcpCloseWait"`
}

// ConnectionDataPoint 连接数据点
type ConnectionDataPoint struct {
	Timestamp      time.Time `json:"timestamp"`
	TCPEstablished int64     `json:"tcpEstablished"`
	TCPTimeWait    int64     `json:"tcpTimeWait"`
	TotalActive    int64     `json:"totalActive"`
}

// ConnectionSummary 连接统计
type ConnectionSummary struct {
	AvgTCPEstablished int64 `json:"avgTCPEstablished"`
	MaxTCPEstablished int64 `json:"maxTCPEstablished"`
	AvgTCPTimeWait    int64 `json:"avgTCPTimeWait"`
	MaxTCPTimeWait    int64 `json:"maxTCPTimeWait"`
}

// ==================== 容器详细状态指标 ====================

// ContainerStatusMetrics 容器状态指标
type ContainerStatusMetrics struct {
	Namespace     string     `json:"namespace"`
	PodName       string     `json:"podName"`
	ContainerName string     `json:"containerName"`
	State         string     `json:"state"` // Running, Waiting, Terminated
	Ready         bool       `json:"ready"`
	RestartCount  int64      `json:"restartCount"`
	StartTime     *time.Time `json:"startTime,omitempty"`
	LastRestart   *time.Time `json:"lastRestart,omitempty"`
	ExitCode      int        `json:"exitCode"`
	Reason        string     `json:"reason"`
	Message       string     `json:"message"`
	Image         string     `json:"image"`
	ImageID       string     `json:"imageID"`
	ContainerID   string     `json:"containerID"`
}

// ContainerEnvironment 容器环境信息
type ContainerEnvironment struct {
	Namespace       string            `json:"namespace"`
	PodName         string            `json:"podName"`
	ContainerName   string            `json:"containerName"`
	Image           string            `json:"image"`
	ImagePullPolicy string            `json:"imagePullPolicy"`
	Command         []string          `json:"command,omitempty"`
	Args            []string          `json:"args,omitempty"`
	EnvVars         map[string]string `json:"envVars,omitempty"`
	EnvFrom         []EnvFromSource   `json:"envFrom,omitempty"`
	VolumeMounts    []VolumeMountInfo `json:"volumeMounts,omitempty"`
	WorkingDir      string            `json:"workingDir,omitempty"`
}

// EnvFromSource 环境变量来源
type EnvFromSource struct {
	ConfigMapRef string `json:"configMapRef,omitempty"`
	SecretRef    string `json:"secretRef,omitempty"`
}

// VolumeMountInfo 卷挂载信息
type VolumeMountInfo struct {
	Name      string `json:"name"`
	MountPath string `json:"mountPath"`
	SubPath   string `json:"subPath,omitempty"`
	ReadOnly  bool   `json:"readOnly"`
}

// ==================== 资源配额指标 ====================

// ResourceQuotaMetrics 资源配额指标
type ResourceQuotaMetrics struct {
	Namespace     string                `json:"namespace"`
	QuotaName     string                `json:"quotaName,omitempty"`
	CPUUsed       float64               `json:"cpuUsed"`
	CPUHard       float64               `json:"cpuHard"`
	CPUPercent    float64               `json:"cpuPercent"`
	MemoryUsed    int64                 `json:"memoryUsed"`
	MemoryHard    int64                 `json:"memoryHard"`
	MemoryPercent float64               `json:"memoryPercent"`
	PodsUsed      int64                 `json:"podsUsed"`
	PodsHard      int64                 `json:"podsHard"`
	PodsPercent   float64               `json:"podsPercent"`
	Resources     []ResourceQuotaDetail `json:"resources,omitempty"`
}

// ResourceQuotaDetail 资源配额详情
type ResourceQuotaDetail struct {
	Resource string  `json:"resource"`
	Used     string  `json:"used"`
	Hard     string  `json:"hard"`
	Percent  float64 `json:"percent"`
}

// ==================== 日志指标 ====================

// ContainerLogMetrics 容器日志指标
type ContainerLogMetrics struct {
	Namespace     string             `json:"namespace"`
	PodName       string             `json:"podName"`
	ContainerName string             `json:"containerName"`
	TotalLines    int64              `json:"totalLines"`
	ErrorCount    int64              `json:"errorCount"`
	WarningCount  int64              `json:"warningCount"`
	LogRate       float64            `json:"logRate"` // 每秒日志行数
	Trend         []LogRateDataPoint `json:"trend,omitempty"`
}

// LogRateDataPoint 日志速率数据点
type LogRateDataPoint struct {
	Timestamp  time.Time `json:"timestamp"`
	LinesCount int64     `json:"linesCount"`
	ErrorRate  float64   `json:"errorRate"`
}

// ==================== 进程指标 ====================

// ProcessMetrics 进程指标
type ProcessMetrics struct {
	Namespace     string        `json:"namespace"`
	PodName       string        `json:"podName"`
	ContainerName string        `json:"containerName"`
	ProcessCount  int64         `json:"processCount"`
	ThreadCount   int64         `json:"threadCount"`
	ZombieCount   int64         `json:"zombieCount"`
	Processes     []ProcessInfo `json:"processes,omitempty"`
}

// ProcessInfo 进程信息
type ProcessInfo struct {
	PID         int64   `json:"pid"`
	Name        string  `json:"name"`
	State       string  `json:"state"`
	CPUPercent  float64 `json:"cpuPercent"`
	MemoryBytes int64   `json:"memoryBytes"`
	Threads     int64   `json:"threads"`
}

// ==================== PodOperator 接口 ====================

// PodOperator Pod 资源操作器接口
type PodOperator interface {
	// CPU 相关
	GetCPUUsage(namespace, pod string, timeRange *TimeRange) (*PodCPUMetrics, error)
	GetCPUUsageByContainer(namespace, pod, container string, timeRange *TimeRange) (*ContainerCPUMetrics, error)
	GetCPUThrottling(namespace, pod string, timeRange *TimeRange) (*CPUThrottlingMetrics, error)

	// 内存相关
	GetMemoryUsage(namespace, pod string, timeRange *TimeRange) (*PodMemoryMetrics, error)
	GetMemoryUsageByContainer(namespace, pod, container string, timeRange *TimeRange) (*ContainerMemoryMetrics, error)
	GetMemoryOOM(namespace, pod string, timeRange *TimeRange) (*OOMMetrics, error)

	// 网络相关（Pod 级别）
	GetNetworkIO(namespace, pod string, timeRange *TimeRange) (*NetworkMetrics, error)
	GetNetworkRate(namespace, pod string, timeRange *TimeRange) (*NetworkRateMetrics, error)

	// 网络相关（容器级别）
	GetNetworkIOByContainer(namespace, pod, container string, timeRange *TimeRange) (*ContainerNetworkMetrics, error)
	GetNetworkRateByContainer(namespace, pod, container string, timeRange *TimeRange) (*ContainerNetworkRateMetrics, error)

	// 磁盘相关
	GetDiskIO(namespace, pod string, timeRange *TimeRange) (*DiskMetrics, error)
	GetDiskRate(namespace, pod string, timeRange *TimeRange) (*DiskRateMetrics, error)

	// Pod 状态
	GetPodStatus(namespace, pod string, timeRange *TimeRange) (*PodStatusMetrics, error)
	GetRestartCount(namespace, pod string, timeRange *TimeRange) (*RestartMetrics, error)
	GetPodAge(namespace, pod string) (*PodAgeMetrics, error)

	// 聚合查询
	GetPodOverview(namespace, pod string, timeRange *TimeRange) (*PodOverview, error)
	ListPodsMetrics(namespace string, timeRange *TimeRange) ([]PodOverview, error)

	// Top 查询
	GetTopPodsByCPU(namespace string, limit int, timeRange *TimeRange) ([]PodRanking, error)
	GetTopPodsByMemory(namespace string, limit int, timeRange *TimeRange) ([]PodRanking, error)
	GetTopPodsByNetwork(namespace string, limit int, timeRange *TimeRange) ([]PodRanking, error)

	// 存储/Volume 相关
	GetVolumeUsage(namespace, pod string) (*PodVolumeMetrics, error)
	GetVolumeIOPS(namespace, pod string, timeRange *TimeRange) (*VolumeIOPSMetrics, error)

	// 健康检查/探针相关
	GetProbeStatus(namespace, pod string) (*PodProbeMetrics, error)

	// 文件描述符相关
	GetFileDescriptorUsage(namespace, pod string, timeRange *TimeRange) (*FileDescriptorMetrics, error)

	// 网络连接相关
	GetNetworkConnections(namespace, pod string, timeRange *TimeRange) (*NetworkConnectionMetrics, error)

	// 容器详细信息
	GetContainerStatus(namespace, pod, container string) (*ContainerStatusMetrics, error)
	GetDiskIOByContainer(namespace, pod, container string, timeRange *TimeRange) (*ContainerDiskMetrics, error)

	// 资源配额相关
	GetResourceQuota(namespace string) (*ResourceQuotaMetrics, error)
}
