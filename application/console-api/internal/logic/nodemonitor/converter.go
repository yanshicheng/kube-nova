package nodemonitor

import (
	"time"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	pmtypes "github.com/yanshicheng/kube-nova/common/prometheusmanager/types"
)

// formatTime 将 time.Time 转换为 RFC3339 格式字符串
func formatTime(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.Unix()
}

// formatTimePtr 将 *time.Time 转换为字符串
func formatTimePtr(t *time.Time) string {
	if t == nil || t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}

// ==================== Node 综合指标转换 ====================

func convertNodeMetrics(m *pmtypes.NodeMetrics) types.NodeMetrics {
	return types.NodeMetrics{
		NodeName:  m.NodeName,
		Timestamp: formatTime(m.Timestamp),
		CPU:       convertNodeCPUMetrics(&m.CPU),
		Memory:    convertNodeMemoryMetrics(&m.Memory),
		Disk:      convertNodeDiskMetrics(&m.Disk),
		Network:   convertNodeNetworkMetrics(&m.Network),
		K8sStatus: convertNodeK8sStatus(&m.K8sStatus),
		System:    convertNodeSystemMetrics(&m.System),
	}
}

// ==================== CPU 指标转换 ====================

func convertNodeCPUMetrics(m *pmtypes.NodeCPUMetrics) types.NodeCPUMetrics {
	return types.NodeCPUMetrics{
		Current: convertNodeCPUSnapshot(&m.Current),
		Trend:   convertNodeCPUDataPoints(m.Trend),
		Summary: convertNodeCPUSummary(&m.Summary),
	}
}

func convertNodeCPUSnapshot(s *pmtypes.NodeCPUSnapshot) types.NodeCPUSnapshot {
	return types.NodeCPUSnapshot{
		Timestamp:         formatTime(s.Timestamp),
		TotalCores:        s.TotalCores,
		UsagePercent:      s.UsagePercent,
		UserPercent:       s.UserPercent,
		SystemPercent:     s.SystemPercent,
		IowaitPercent:     s.IowaitPercent,
		StealPercent:      s.StealPercent,
		IrqPercent:        s.IrqPercent,
		SoftirqPercent:    s.SoftirqPercent,
		Load1:             s.Load1,
		Load5:             s.Load5,
		Load15:            s.Load15,
		ContextSwitchRate: s.ContextSwitchRate,
	}
}

func convertNodeCPUDataPoints(points []pmtypes.NodeCPUDataPoint) []types.NodeCPUDataPoint {
	result := make([]types.NodeCPUDataPoint, len(points))
	for i, p := range points {
		result[i] = types.NodeCPUDataPoint{
			Timestamp:    formatTime(p.Timestamp),
			UsagePercent: p.UsagePercent,
			Load5:        p.Load5,
		}
	}
	return result
}

func convertNodeCPUSummary(s *pmtypes.NodeCPUSummary) types.NodeCPUSummary {
	return types.NodeCPUSummary{
		AvgUsagePercent: s.AvgUsagePercent,
		MaxUsagePercent: s.MaxUsagePercent,
		MinUsagePercent: s.MinUsagePercent,
		AvgLoad5:        s.AvgLoad5,
		MaxLoad5:        s.MaxLoad5,
	}
}

// ==================== Memory 指标转换 ====================

func convertNodeMemoryMetrics(m *pmtypes.NodeMemoryMetrics) types.NodeMemoryMetrics {
	return types.NodeMemoryMetrics{
		Current: convertNodeMemorySnapshot(&m.Current),
		Trend:   convertNodeMemoryDataPoints(m.Trend),
		Summary: convertNodeMemorySummary(&m.Summary),
	}
}

func convertNodeMemorySnapshot(s *pmtypes.NodeMemorySnapshot) types.NodeMemorySnapshot {
	return types.NodeMemorySnapshot{
		Timestamp:        formatTime(s.Timestamp),
		TotalBytes:       s.TotalBytes,
		UsedBytes:        s.UsedBytes,
		AvailableBytes:   s.AvailableBytes,
		FreeBytes:        s.FreeBytes,
		BuffersBytes:     s.BuffersBytes,
		CachedBytes:      s.CachedBytes,
		ActiveBytes:      s.ActiveBytes,
		InactiveBytes:    s.InactiveBytes,
		UsagePercent:     s.UsagePercent,
		SwapTotalBytes:   s.SwapTotalBytes,
		SwapUsedBytes:    s.SwapUsedBytes,
		SwapFreeBytes:    s.SwapFreeBytes,
		SwapUsagePercent: s.SwapUsagePercent,
		SwapInRate:       s.SwapInRate,
		SwapOutRate:      s.SwapOutRate,
	}
}

func convertNodeMemoryDataPoints(points []pmtypes.NodeMemoryDataPoint) []types.NodeMemoryDataPoint {
	result := make([]types.NodeMemoryDataPoint, len(points))
	for i, p := range points {
		result[i] = types.NodeMemoryDataPoint{
			Timestamp:      formatTime(p.Timestamp),
			UsedBytes:      p.UsedBytes,
			UsagePercent:   p.UsagePercent,
			AvailableBytes: p.AvailableBytes,
		}
	}
	return result
}

func convertNodeMemorySummary(s *pmtypes.NodeMemorySummary) types.NodeMemorySummary {
	return types.NodeMemorySummary{
		AvgUsagePercent: s.AvgUsagePercent,
		MaxUsagePercent: s.MaxUsagePercent,
		MinUsagePercent: s.MinUsagePercent,
		AvgSwapUsage:    s.AvgSwapUsage,
		MaxSwapUsage:    s.MaxSwapUsage,
	}
}

// ==================== Disk 指标转换 ====================

func convertNodeDiskMetrics(m *pmtypes.NodeDiskMetrics) types.NodeDiskMetrics {
	return types.NodeDiskMetrics{
		Filesystems: convertNodeFilesystemMetricsList(m.Filesystems),
		Devices:     convertNodeDiskDeviceMetricsList(m.Devices),
	}
}

func convertNodeFilesystemMetrics(m *pmtypes.NodeFilesystemMetrics) types.NodeFilesystemMetrics {
	return types.NodeFilesystemMetrics{
		Mountpoint: m.Mountpoint,
		Device:     m.Device,
		FSType:     m.FSType,
		Current:    convertNodeFilesystemSnapshot(&m.Current),
		Trend:      convertNodeFilesystemDataPoints(m.Trend),
	}
}

func convertNodeFilesystemMetricsList(list []pmtypes.NodeFilesystemMetrics) []types.NodeFilesystemMetrics {
	result := make([]types.NodeFilesystemMetrics, len(list))
	for i, m := range list {
		result[i] = convertNodeFilesystemMetrics(&m)
	}
	return result
}

func convertNodeFilesystemSnapshot(s *pmtypes.NodeFilesystemSnapshot) types.NodeFilesystemSnapshot {
	return types.NodeFilesystemSnapshot{
		Timestamp:      formatTime(s.Timestamp),
		TotalBytes:     s.TotalBytes,
		UsedBytes:      s.UsedBytes,
		AvailableBytes: s.AvailableBytes,
		UsagePercent:   s.UsagePercent,
		TotalInodes:    s.TotalInodes,
		UsedInodes:     s.UsedInodes,
		FreeInodes:     s.FreeInodes,
		InodePercent:   s.InodePercent,
	}
}

func convertNodeFilesystemDataPoints(points []pmtypes.NodeFilesystemDataPoint) []types.NodeFilesystemDataPoint {
	result := make([]types.NodeFilesystemDataPoint, len(points))
	for i, p := range points {
		result[i] = types.NodeFilesystemDataPoint{
			Timestamp:    formatTime(p.Timestamp),
			UsedBytes:    p.UsedBytes,
			UsagePercent: p.UsagePercent,
		}
	}
	return result
}

func convertNodeDiskDeviceMetrics(m *pmtypes.NodeDiskDeviceMetrics) types.NodeDiskDeviceMetrics {
	return types.NodeDiskDeviceMetrics{
		Device:  m.Device,
		Current: convertNodeDiskDeviceSnapshot(&m.Current),
		Trend:   convertNodeDiskDeviceDataPoints(m.Trend),
		Summary: convertNodeDiskDeviceSummary(&m.Summary),
	}
}

func convertNodeDiskDeviceMetricsList(list []pmtypes.NodeDiskDeviceMetrics) []types.NodeDiskDeviceMetrics {
	result := make([]types.NodeDiskDeviceMetrics, len(list))
	for i, m := range list {
		result[i] = convertNodeDiskDeviceMetrics(&m)
	}
	return result
}

func convertNodeDiskDeviceSnapshot(s *pmtypes.NodeDiskDeviceSnapshot) types.NodeDiskDeviceSnapshot {
	return types.NodeDiskDeviceSnapshot{
		Timestamp:            formatTime(s.Timestamp),
		ReadIOPS:             s.ReadIOPS,
		WriteIOPS:            s.WriteIOPS,
		ReadBytesPerSec:      s.ReadBytesPerSec,
		WriteBytesPerSec:     s.WriteBytesPerSec,
		IOUtilizationPercent: s.IOUtilizationPercent,
		AvgAwaitMs:           s.AvgAwaitMs,
	}
}

func convertNodeDiskDeviceDataPoints(points []pmtypes.NodeDiskDeviceDataPoint) []types.NodeDiskDeviceDataPoint {
	result := make([]types.NodeDiskDeviceDataPoint, len(points))
	for i, p := range points {
		result[i] = types.NodeDiskDeviceDataPoint{
			Timestamp:            formatTime(p.Timestamp),
			ReadBytesPerSec:      p.ReadBytesPerSec,
			WriteBytesPerSec:     p.WriteBytesPerSec,
			IOUtilizationPercent: p.IOUtilizationPercent,
		}
	}
	return result
}

func convertNodeDiskDeviceSummary(s *pmtypes.NodeDiskDeviceSummary) types.NodeDiskDeviceSummary {
	return types.NodeDiskDeviceSummary{
		AvgReadBytesPerSec:  s.AvgReadBytesPerSec,
		MaxReadBytesPerSec:  s.MaxReadBytesPerSec,
		AvgWriteBytesPerSec: s.AvgWriteBytesPerSec,
		MaxWriteBytesPerSec: s.MaxWriteBytesPerSec,
		AvgIOUtilization:    s.AvgIOUtilization,
		MaxIOUtilization:    s.MaxIOUtilization,
	}
}

// ==================== Network 指标转换 ====================

func convertNodeNetworkMetrics(m *pmtypes.NodeNetworkMetrics) types.NodeNetworkMetrics {
	return types.NodeNetworkMetrics{
		Interfaces: convertNodeNetworkInterfaceMetricsList(m.Interfaces),
		TCP:        convertNodeTCPMetrics(&m.TCP),
	}
}

func convertNodeNetworkInterfaceMetrics(m *pmtypes.NodeNetworkInterfaceMetrics) types.NodeNetworkInterfaceMetrics {
	return types.NodeNetworkInterfaceMetrics{
		InterfaceName: m.InterfaceName,
		Current:       convertNodeNetworkInterfaceSnapshot(&m.Current),
		Trend:         convertNodeNetworkInterfaceDataPoints(m.Trend),
		Summary:       convertNodeNetworkInterfaceSummary(&m.Summary),
	}
}

func convertNodeNetworkInterfaceMetricsList(list []pmtypes.NodeNetworkInterfaceMetrics) []types.NodeNetworkInterfaceMetrics {
	result := make([]types.NodeNetworkInterfaceMetrics, len(list))
	for i, m := range list {
		result[i] = convertNodeNetworkInterfaceMetrics(&m)
	}
	return result
}

func convertNodeNetworkInterfaceSnapshot(s *pmtypes.NodeNetworkInterfaceSnapshot) types.NodeNetworkInterfaceSnapshot {
	return types.NodeNetworkInterfaceSnapshot{
		Timestamp:             formatTime(s.Timestamp),
		ReceiveBytesPerSec:    s.ReceiveBytesPerSec,
		TransmitBytesPerSec:   s.TransmitBytesPerSec,
		ReceivePacketsPerSec:  s.ReceivePacketsPerSec,
		TransmitPacketsPerSec: s.TransmitPacketsPerSec,
		ReceiveErrorsRate:     s.ReceiveErrorsRate,
		TransmitErrorsRate:    s.TransmitErrorsRate,
		ReceiveDropsRate:      s.ReceiveDropsRate,
		TransmitDropsRate:     s.TransmitDropsRate,
	}
}

func convertNodeNetworkInterfaceDataPoints(points []pmtypes.NodeNetworkInterfaceDataPoint) []types.NodeNetworkInterfaceDataPoint {
	result := make([]types.NodeNetworkInterfaceDataPoint, len(points))
	for i, p := range points {
		result[i] = types.NodeNetworkInterfaceDataPoint{
			Timestamp:           formatTime(p.Timestamp),
			ReceiveBytesPerSec:  p.ReceiveBytesPerSec,
			TransmitBytesPerSec: p.TransmitBytesPerSec,
		}
	}
	return result
}

func convertNodeNetworkInterfaceSummary(s *pmtypes.NodeNetworkInterfaceSummary) types.NodeNetworkInterfaceSummary {
	return types.NodeNetworkInterfaceSummary{
		AvgReceiveBytesPerSec:  s.AvgReceiveBytesPerSec,
		MaxReceiveBytesPerSec:  s.MaxReceiveBytesPerSec,
		AvgTransmitBytesPerSec: s.AvgTransmitBytesPerSec,
		MaxTransmitBytesPerSec: s.MaxTransmitBytesPerSec,
		TotalReceiveErrors:     s.TotalReceiveErrors,
		TotalTransmitErrors:    s.TotalTransmitErrors,
		TotalReceiveDrops:      s.TotalReceiveDrops,
		TotalTransmitDrops:     s.TotalTransmitDrops,
	}
}

func convertNodeTCPMetrics(m *pmtypes.NodeTCPMetrics) types.NodeTCPMetrics {
	return types.NodeTCPMetrics{
		Current: convertNodeTCPSnapshot(&m.Current),
		Trend:   convertNodeTCPDataPoints(m.Trend),
	}
}

func convertNodeTCPSnapshot(s *pmtypes.NodeTCPSnapshot) types.NodeTCPSnapshot {
	return types.NodeTCPSnapshot{
		Timestamp:              formatTime(s.Timestamp),
		EstablishedConnections: s.EstablishedConnections,
		TimeWaitConnections:    s.TimeWaitConnections,
		ActiveOpensRate:        s.ActiveOpensRate,
		PassiveOpensRate:       s.PassiveOpensRate,
		SocketsInUse:           s.SocketsInUse,
	}
}

func convertNodeTCPDataPoints(points []pmtypes.NodeTCPDataPoint) []types.NodeTCPDataPoint {
	result := make([]types.NodeTCPDataPoint, len(points))
	for i, p := range points {
		result[i] = types.NodeTCPDataPoint{
			Timestamp:              formatTime(p.Timestamp),
			EstablishedConnections: p.EstablishedConnections,
			TimeWaitConnections:    p.TimeWaitConnections,
		}
	}
	return result
}

// ==================== K8s Status 转换 ====================

func convertNodeK8sStatus(s *pmtypes.NodeK8sStatus) types.NodeK8sStatus {
	return types.NodeK8sStatus{
		Conditions:     convertNodeConditions(s.Conditions),
		Capacity:       convertNodeResourceQuantity(&s.Capacity),
		Allocatable:    convertNodeResourceQuantity(&s.Allocatable),
		Allocated:      convertNodeResourceQuantity(&s.Allocated),
		KubeletMetrics: convertNodeKubeletMetrics(&s.KubeletMetrics),
		NodeInfo:       convertNodeInfo(&s.NodeInfo),
		Taints:         convertNodeTaints(s.Taints),
		Labels:         s.Labels,
		Annotations:    s.Annotations,
	}
}

func convertNodeConditions(conditions []pmtypes.NodeCondition) []types.NodeCondition {
	result := make([]types.NodeCondition, len(conditions))
	for i, c := range conditions {
		result[i] = types.NodeCondition{
			Type:               c.Type,
			Status:             c.Status,
			Reason:             c.Reason,
			Message:            c.Message,
			LastTransitionTime: formatTime(c.LastTransitionTime),
		}
	}
	return result
}

func convertNodeResourceQuantity(q *pmtypes.NodeResourceQuantity) types.NodeResourceQuantity {
	return types.NodeResourceQuantity{
		CPUCores:         q.CPUCores,
		MemoryBytes:      q.MemoryBytes,
		Pods:             q.Pods,
		EphemeralStorage: q.EphemeralStorage,
	}
}

func convertNodeKubeletMetrics(m *pmtypes.NodeKubeletMetrics) types.NodeKubeletMetrics {
	return types.NodeKubeletMetrics{
		RunningPods:         m.RunningPods,
		RunningContainers:   m.RunningContainers,
		PLEGRelistDuration:  m.PLEGRelistDuration,
		RuntimeOpsRate:      m.RuntimeOpsRate,
		RuntimeOpsErrorRate: m.RuntimeOpsErrorRate,
		RuntimeOpsDuration:  m.RuntimeOpsDuration,
		Trend:               convertNodeKubeletDataPoints(m.Trend),
	}
}

func convertNodeKubeletDataPoints(points []pmtypes.NodeKubeletDataPoint) []types.NodeKubeletDataPoint {
	result := make([]types.NodeKubeletDataPoint, len(points))
	for i, p := range points {
		result[i] = types.NodeKubeletDataPoint{
			Timestamp:           formatTime(p.Timestamp),
			PLEGRelistDuration:  p.PLEGRelistDuration,
			RuntimeOpsRate:      p.RuntimeOpsRate,
			RuntimeOpsErrorRate: p.RuntimeOpsErrorRate,
		}
	}
	return result
}

func convertNodeInfo(info *pmtypes.NodeInfo) types.NodeInfo {
	return types.NodeInfo{
		KubeletVersion:          info.KubeletVersion,
		ContainerRuntimeVersion: info.ContainerRuntimeVersion,
		KernelVersion:           info.KernelVersion,
		OSImage:                 info.OSImage,
		Architecture:            info.Architecture,
		BootTime:                formatTime(info.BootTime),
		UptimeSeconds:           info.UptimeSeconds,
	}
}

func convertNodeTaints(taints []pmtypes.NodeTaint) []types.NodeTaint {
	result := make([]types.NodeTaint, len(taints))
	for i, t := range taints {
		result[i] = types.NodeTaint{
			Key:    t.Key,
			Value:  t.Value,
			Effect: t.Effect,
		}
	}
	return result
}

// ==================== System 指标转换 ====================

func convertNodeSystemMetrics(m *pmtypes.NodeSystemMetrics) types.NodeSystemMetrics {
	return types.NodeSystemMetrics{
		Processes:       convertNodeProcessMetrics(&m.Processes),
		FileDescriptors: convertNodeFileDescriptorMetrics(&m.FileDescriptors),
	}
}

func convertNodeProcessMetrics(m *pmtypes.NodeProcessMetrics) types.NodeProcessMetrics {
	return types.NodeProcessMetrics{
		Running: m.Running,
		Blocked: m.Blocked,
	}
}

func convertNodeFileDescriptorMetrics(m *pmtypes.NodeFileDescriptorMetrics) types.NodeFileDescriptorMetrics {
	return types.NodeFileDescriptorMetrics{
		Allocated:    m.Allocated,
		Maximum:      m.Maximum,
		UsagePercent: m.UsagePercent,
		Trend:        convertNodeFileDescriptorDataPoints(m.Trend),
	}
}

func convertNodeFileDescriptorDataPoints(points []pmtypes.NodeFileDescriptorDataPoint) []types.NodeFileDescriptorDataPoint {
	result := make([]types.NodeFileDescriptorDataPoint, len(points))
	for i, p := range points {
		result[i] = types.NodeFileDescriptorDataPoint{
			Timestamp:    formatTime(p.Timestamp),
			Allocated:    p.Allocated,
			UsagePercent: p.UsagePercent,
		}
	}
	return result
}

// ==================== Pod 指标转换 ====================

func convertNodePodMetrics(m *pmtypes.NodePodMetrics) types.NodePodMetrics {
	return types.NodePodMetrics{
		NodeName:     m.NodeName,
		TotalPods:    m.TotalPods,
		RunningPods:  m.RunningPods,
		PendingPods:  m.PendingPods,
		FailedPods:   m.FailedPods,
		TopPodsByCPU: convertPodResourceUsageList(m.TopPodsByCPU),
		TopPodsByMem: convertPodResourceUsageList(m.TopPodsByMem),
		PodList:      convertNodePodBriefList(m.PodList),
	}
}

func convertPodResourceUsageList(list []pmtypes.PodResourceUsage) []types.PodResourceUsage {
	result := make([]types.PodResourceUsage, len(list))
	for i, p := range list {
		result[i] = types.PodResourceUsage{
			Namespace: p.Namespace,
			PodName:   p.PodName,
			Value:     p.Value,
			Unit:      p.Unit,
		}
	}
	return result
}

func convertNodePodBriefList(list []pmtypes.NodePodBrief) []types.NodePodBrief {
	result := make([]types.NodePodBrief, len(list))
	for i, p := range list {
		result[i] = types.NodePodBrief{
			Namespace:    p.Namespace,
			PodName:      p.PodName,
			Phase:        p.Phase,
			CPUUsage:     p.CPUUsage,
			MemoryUsage:  p.MemoryUsage,
			RestartCount: p.RestartCount,
		}
	}
	return result
}

// ==================== Comparison 和 Ranking 转换 ====================

func convertNodeComparison(c *pmtypes.NodeComparison) types.NodeComparison {
	return types.NodeComparison{
		Nodes:     convertNodeComparisonItems(c.Nodes),
		Timestamp: formatTime(c.Timestamp),
	}
}

func convertNodeComparisonItems(items []pmtypes.NodeComparisonItem) []types.NodeComparisonItem {
	result := make([]types.NodeComparisonItem, len(items))
	for i, item := range items {
		result[i] = types.NodeComparisonItem{
			NodeName:    item.NodeName,
			CPUUsage:    item.CPUUsage,
			MemoryUsage: item.MemoryUsage,
			DiskUsage:   item.DiskUsage,
			Load5:       item.Load5,
			PodCount:    item.PodCount,
			Ready:       item.Ready,
		}
	}
	return result
}

func convertNodeRanking(r *pmtypes.NodeRanking) types.NodeRanking {
	return types.NodeRanking{
		TopByCPU:    convertNodeRankingItems(r.TopByCPU),
		TopByMemory: convertNodeRankingItems(r.TopByMemory),
		TopByDisk:   convertNodeRankingItems(r.TopByDisk),
		TopByLoad:   convertNodeRankingItems(r.TopByLoad),
		TopByPods:   convertNodeRankingItems(r.TopByPods),
	}
}

func convertNodeRankingItems(items []pmtypes.NodeRankingItem) []types.NodeRankingItem {
	result := make([]types.NodeRankingItem, len(items))
	for i, item := range items {
		result[i] = types.NodeRankingItem{
			NodeName: item.NodeName,
			Value:    item.Value,
			Unit:     item.Unit,
		}
	}
	return result
}
