package namespacemonitor

import (
	"time"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	pmtypes "github.com/yanshicheng/kube-nova/common/prometheusmanager/types"
)

// formatTime 将 time.Time 转换为 Unix 时间戳
func formatTime(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.Unix()
}

// ==================== Namespace 综合指标转换 ====================

func convertNamespaceMetrics(m *pmtypes.NamespaceMetrics) types.NamespaceMetrics {
	return types.NamespaceMetrics{
		Namespace: m.Namespace,
		Timestamp: formatTime(m.Timestamp),
		Resources: convertNamespaceResourceMetrics(&m.Resources),
		Quota:     convertNamespaceQuotaMetrics(&m.Quota),
		Workloads: convertNamespaceWorkloadMetrics(&m.Workloads),
		Network:   convertNamespaceNetworkMetrics(&m.Network),
		Storage:   convertNamespaceStorageMetrics(&m.Storage),
		Config:    convertNamespaceConfigMetrics(&m.Config),
	}
}

// ==================== 资源使用指标转换 ====================

func convertNamespaceResourceMetrics(m *pmtypes.NamespaceResourceMetrics) types.NamespaceResourceMetrics {
	return types.NamespaceResourceMetrics{
		CPU:    convertNamespaceCPUMetrics(&m.CPU),
		Memory: convertNamespaceMemoryMetrics(&m.Memory),
	}
}

func convertNamespaceCPUMetrics(m *pmtypes.NamespaceCPUMetrics) types.NamespaceCPUMetrics {
	return types.NamespaceCPUMetrics{
		Current:              convertNamespaceCPUSnapshot(&m.Current),
		Requests:             m.Requests,
		Limits:               m.Limits,
		Trend:                convertNamespaceCPUDataPoints(m.Trend),
		TopPods:              convertResourceRankingList(m.TopPods),
		TopContainers:        convertContainerResourceRankingList(m.TopContainers),
		ContainersNoRequests: m.ContainersNoRequests,
		ContainersNoLimits:   m.ContainersNoLimits,
	}
}

func convertNamespaceCPUSnapshot(s *pmtypes.NamespaceCPUSnapshot) types.NamespaceCPUSnapshot {
	return types.NamespaceCPUSnapshot{
		Timestamp:    formatTime(s.Timestamp),
		UsageCores:   s.UsageCores,
		AvgCores:     s.AvgCores,
		MaxCores:     s.MaxCores,
		UsagePercent: s.UsagePercent,
	}
}

func convertNamespaceCPUDataPoints(points []pmtypes.NamespaceCPUDataPoint) []types.NamespaceCPUDataPoint {
	result := make([]types.NamespaceCPUDataPoint, len(points))
	for i, p := range points {
		result[i] = types.NamespaceCPUDataPoint{
			Timestamp:    formatTime(p.Timestamp),
			UsageCores:   p.UsageCores,
			UsagePercent: p.UsagePercent,
		}
	}
	return result
}

func convertNamespaceMemoryMetrics(m *pmtypes.NamespaceMemoryMetrics) types.NamespaceMemoryMetrics {
	return types.NamespaceMemoryMetrics{
		Current:       convertNamespaceMemorySnapshot(&m.Current),
		Requests:      m.Requests,
		Limits:        m.Limits,
		Trend:         convertNamespaceMemoryDataPoints(m.Trend),
		TopPods:       convertResourceRankingList(m.TopPods),
		TopContainers: convertContainerResourceRankingList(m.TopContainers),
		OOMKills:      m.OOMKills,
	}
}

func convertNamespaceMemorySnapshot(s *pmtypes.NamespaceMemorySnapshot) types.NamespaceMemorySnapshot {
	return types.NamespaceMemorySnapshot{
		Timestamp:       formatTime(s.Timestamp),
		WorkingSetBytes: s.WorkingSetBytes,
		RSSBytes:        s.RSSBytes,
		CacheBytes:      s.CacheBytes,
		AvgBytes:        s.AvgBytes,
		MaxBytes:        s.MaxBytes,
		UsagePercent:    s.UsagePercent,
	}
}

func convertNamespaceMemoryDataPoints(points []pmtypes.NamespaceMemoryDataPoint) []types.NamespaceMemoryDataPoint {
	result := make([]types.NamespaceMemoryDataPoint, len(points))
	for i, p := range points {
		result[i] = types.NamespaceMemoryDataPoint{
			Timestamp:       formatTime(p.Timestamp),
			WorkingSetBytes: p.WorkingSetBytes,
			UsagePercent:    p.UsagePercent,
		}
	}
	return result
}

func convertResourceRankingList(list []pmtypes.ResourceRanking) []types.ResourceRanking {
	result := make([]types.ResourceRanking, len(list))
	for i, r := range list {
		result[i] = types.ResourceRanking{
			Name:  r.Name,
			Value: r.Value,
			Unit:  r.Unit,
		}
	}
	return result
}

func convertContainerResourceRankingList(list []pmtypes.ContainerResourceRanking) []types.ContainerResourceRanking {
	result := make([]types.ContainerResourceRanking, len(list))
	for i, r := range list {
		result[i] = types.ContainerResourceRanking{
			PodName:       r.PodName,
			ContainerName: r.ContainerName,
			Value:         r.Value,
			Unit:          r.Unit,
		}
	}
	return result
}

// ==================== 配额管理转换 ====================

func convertNamespaceQuotaMetrics(m *pmtypes.NamespaceQuotaMetrics) types.NamespaceQuotaMetrics {
	return types.NamespaceQuotaMetrics{
		HasQuota:     m.HasQuota,
		QuotaName:    m.QuotaName,
		CPU:          convertQuotaUsage(&m.CPU),
		Memory:       convertQuotaUsage(&m.Memory),
		Pods:         convertQuotaUsage(&m.Pods),
		Services:     convertQuotaUsage(&m.Services),
		ConfigMaps:   convertQuotaUsage(&m.ConfigMaps),
		Secrets:      convertQuotaUsage(&m.Secrets),
		PVCs:         convertQuotaUsage(&m.PVCs),
		Storage:      convertQuotaUsage(&m.Storage),
		AllResources: convertResourceQuotaDetailList(m.AllResources),
	}
}

func convertQuotaUsage(q *pmtypes.QuotaUsage) types.QuotaUsage {
	return types.QuotaUsage{
		Hard:         q.Hard,
		Used:         q.Used,
		UsagePercent: q.UsagePercent,
		HasQuota:     q.HasQuota,
	}
}

func convertResourceQuotaDetailList(list []pmtypes.ResourceQuotaDetail) []types.ResourceQuotaDetail {
	result := make([]types.ResourceQuotaDetail, len(list))
	for i, r := range list {
		result[i] = types.ResourceQuotaDetail{
			Resource: r.Resource,
			Used:     r.Used,
			Hard:     r.Hard,
			Percent:  r.Percent,
		}
	}
	return result
}

// ==================== 工作负载统计转换 ====================

func convertNamespaceWorkloadMetrics(m *pmtypes.NamespaceWorkloadMetrics) types.NamespaceWorkloadMetrics {
	return types.NamespaceWorkloadMetrics{
		Pods:         convertNamespacePodStatistics(&m.Pods),
		Deployments:  convertNamespaceDeploymentStatistics(&m.Deployments),
		StatefulSets: convertNamespaceStatefulSetStatistics(&m.StatefulSets),
		DaemonSets:   convertNamespaceDaemonSetStatistics(&m.DaemonSets),
		Jobs:         convertNamespaceJobStatistics(&m.Jobs),
		Containers:   convertNamespaceContainerStatistics(&m.Containers),
		Services:     convertNamespaceServiceStatistics(&m.Services),
		Endpoints:    convertNamespaceEndpointStatistics(&m.Endpoints),
		Ingresses:    convertNamespaceIngressStatistics(&m.Ingresses),
	}
}

func convertNamespacePodStatistics(s *pmtypes.NamespacePodStatistics) types.NamespacePodStatistics {
	return types.NamespacePodStatistics{
		Total:           s.Total,
		Running:         s.Running,
		Pending:         s.Pending,
		Failed:          s.Failed,
		Succeeded:       s.Succeeded,
		Unknown:         s.Unknown,
		Ready:           s.Ready,
		NotReady:        s.NotReady,
		TotalRestarts:   s.TotalRestarts,
		HighRestartPods: convertPodRestartInfoList(s.HighRestartPods),
		Trend:           convertNamespacePodDataPoints(s.Trend),
	}
}

func convertPodRestartInfoList(list []pmtypes.PodRestartInfo) []types.PodRestartInfo {
	result := make([]types.PodRestartInfo, len(list))
	for i, p := range list {
		result[i] = types.PodRestartInfo{
			Namespace:    p.Namespace,
			PodName:      p.PodName,
			RestartCount: p.RestartCount,
			RestartRate:  p.RestartRate,
		}
	}
	return result
}

func convertNamespacePodDataPoints(points []pmtypes.NamespacePodDataPoint) []types.NamespacePodDataPoint {
	result := make([]types.NamespacePodDataPoint, len(points))
	for i, p := range points {
		result[i] = types.NamespacePodDataPoint{
			Timestamp: formatTime(p.Timestamp),
			Total:     p.Total,
			Running:   p.Running,
			Pending:   p.Pending,
			Failed:    p.Failed,
		}
	}
	return result
}

func convertNamespaceDeploymentStatistics(s *pmtypes.NamespaceDeploymentStatistics) types.NamespaceDeploymentStatistics {
	return types.NamespaceDeploymentStatistics{
		Total:               s.Total,
		DesiredReplicas:     s.DesiredReplicas,
		AvailableReplicas:   s.AvailableReplicas,
		UnavailableReplicas: s.UnavailableReplicas,
		Updating:            s.Updating,
	}
}

func convertNamespaceStatefulSetStatistics(s *pmtypes.NamespaceStatefulSetStatistics) types.NamespaceStatefulSetStatistics {
	return types.NamespaceStatefulSetStatistics{
		Total:           s.Total,
		DesiredReplicas: s.DesiredReplicas,
		ReadyReplicas:   s.ReadyReplicas,
		CurrentReplicas: s.CurrentReplicas,
	}
}

func convertNamespaceDaemonSetStatistics(s *pmtypes.NamespaceDaemonSetStatistics) types.NamespaceDaemonSetStatistics {
	return types.NamespaceDaemonSetStatistics{
		Total:                s.Total,
		DesiredScheduled:     s.DesiredScheduled,
		CurrentScheduled:     s.CurrentScheduled,
		AvailableScheduled:   s.AvailableScheduled,
		UnavailableScheduled: s.UnavailableScheduled,
	}
}

func convertNamespaceJobStatistics(s *pmtypes.NamespaceJobStatistics) types.NamespaceJobStatistics {
	return types.NamespaceJobStatistics{
		Active:        s.Active,
		Succeeded:     s.Succeeded,
		Failed:        s.Failed,
		CronJobsTotal: s.CronJobsTotal,
		AvgDuration:   s.AvgDuration,
	}
}

func convertNamespaceContainerStatistics(s *pmtypes.NamespaceContainerStatistics) types.NamespaceContainerStatistics {
	return types.NamespaceContainerStatistics{
		Total:      s.Total,
		Running:    s.Running,
		Waiting:    s.Waiting,
		Terminated: s.Terminated,
	}
}

func convertNamespaceServiceStatistics(s *pmtypes.NamespaceServiceStatistics) types.NamespaceServiceStatistics {
	return types.NamespaceServiceStatistics{
		Total:            s.Total,
		ByType:           convertServiceTypeCountList(s.ByType),
		WithoutEndpoints: s.WithoutEndpoints,
	}
}

func convertServiceTypeCountList(list []pmtypes.ServiceTypeCount) []types.ServiceTypeCount {
	result := make([]types.ServiceTypeCount, len(list))
	for i, s := range list {
		result[i] = types.ServiceTypeCount{
			Type:  s.Type,
			Count: s.Count,
		}
	}
	return result
}

func convertNamespaceEndpointStatistics(s *pmtypes.NamespaceEndpointStatistics) types.NamespaceEndpointStatistics {
	return types.NamespaceEndpointStatistics{
		TotalAddresses:        s.TotalAddresses,
		ServicesWithEndpoints: s.ServicesWithEndpoints,
	}
}

func convertNamespaceIngressStatistics(s *pmtypes.NamespaceIngressStatistics) types.NamespaceIngressStatistics {
	return types.NamespaceIngressStatistics{
		Total:     s.Total,
		PathCount: s.PathCount,
	}
}

// ==================== 网络指标转换 ====================

func convertNamespaceNetworkMetrics(m *pmtypes.NamespaceNetworkMetrics) types.NamespaceNetworkMetrics {
	return types.NamespaceNetworkMetrics{
		Current:     convertNamespaceNetworkSnapshot(&m.Current),
		Trend:       convertNamespaceNetworkDataPoints(m.Trend),
		Summary:     convertNamespaceNetworkSummary(&m.Summary),
		TopPodsByRx: convertResourceRankingList(m.TopPodsByRx),
		TopPodsByTx: convertResourceRankingList(m.TopPodsByTx),
	}
}

func convertNamespaceNetworkSnapshot(s *pmtypes.NamespaceNetworkSnapshot) types.NamespaceNetworkSnapshot {
	return types.NamespaceNetworkSnapshot{
		Timestamp:             formatTime(s.Timestamp),
		ReceiveBytesPerSec:    s.ReceiveBytesPerSec,
		TransmitBytesPerSec:   s.TransmitBytesPerSec,
		ReceivePacketsPerSec:  s.ReceivePacketsPerSec,
		TransmitPacketsPerSec: s.TransmitPacketsPerSec,
		ReceiveErrors:         s.ReceiveErrors,
		TransmitErrors:        s.TransmitErrors,
		ReceiveDrops:          s.ReceiveDrops,
		TransmitDrops:         s.TransmitDrops,
	}
}

func convertNamespaceNetworkDataPoints(points []pmtypes.NamespaceNetworkDataPoint) []types.NamespaceNetworkDataPoint {
	result := make([]types.NamespaceNetworkDataPoint, len(points))
	for i, p := range points {
		result[i] = types.NamespaceNetworkDataPoint{
			Timestamp:           formatTime(p.Timestamp),
			ReceiveBytesPerSec:  p.ReceiveBytesPerSec,
			TransmitBytesPerSec: p.TransmitBytesPerSec,
		}
	}
	return result
}

func convertNamespaceNetworkSummary(s *pmtypes.NamespaceNetworkSummary) types.NamespaceNetworkSummary {
	return types.NamespaceNetworkSummary{
		TotalReceiveBytes:      s.TotalReceiveBytes,
		TotalTransmitBytes:     s.TotalTransmitBytes,
		MaxReceiveBytesPerSec:  s.MaxReceiveBytesPerSec,
		MaxTransmitBytesPerSec: s.MaxTransmitBytesPerSec,
		AvgReceiveBytesPerSec:  s.AvgReceiveBytesPerSec,
		AvgTransmitBytesPerSec: s.AvgTransmitBytesPerSec,
		TotalErrors:            s.TotalErrors,
	}
}

// ==================== 存储指标转换 ====================

func convertNamespaceStorageMetrics(m *pmtypes.NamespaceStorageMetrics) types.NamespaceStorageMetrics {
	return types.NamespaceStorageMetrics{
		PVCTotal:            m.PVCTotal,
		PVCBound:            m.PVCBound,
		PVCPending:          m.PVCPending,
		PVCLost:             m.PVCLost,
		TotalRequestedBytes: m.TotalRequestedBytes,
		ByStorageClass:      convertStorageClassUsageList(m.ByStorageClass),
	}
}

func convertStorageClassUsageList(list []pmtypes.StorageClassUsage) []types.StorageClassUsage {
	result := make([]types.StorageClassUsage, len(list))
	for i, s := range list {
		result[i] = types.StorageClassUsage{
			StorageClass: s.StorageClass,
			PVCCount:     s.PVCCount,
			TotalBytes:   s.TotalBytes,
		}
	}
	return result
}

// ==================== 配置管理转换 ====================

func convertNamespaceConfigMetrics(m *pmtypes.NamespaceConfigMetrics) types.NamespaceConfigMetrics {
	return types.NamespaceConfigMetrics{
		ConfigMaps: convertConfigMapStatistics(&m.ConfigMaps),
		Secrets:    convertSecretStatistics(&m.Secrets),
	}
}

func convertConfigMapStatistics(s *pmtypes.ConfigMapStatistics) types.ConfigMapStatistics {
	return types.ConfigMapStatistics{
		Total:     s.Total,
		DataItems: s.DataItems,
	}
}

func convertSecretStatistics(s *pmtypes.SecretStatistics) types.SecretStatistics {
	return types.SecretStatistics{
		Total:  s.Total,
		ByType: convertSecretTypeCountList(s.ByType),
	}
}

func convertSecretTypeCountList(list []pmtypes.SecretTypeCount) []types.SecretTypeCount {
	result := make([]types.SecretTypeCount, len(list))
	for i, s := range list {
		result[i] = types.SecretTypeCount{
			Type:  s.Type,
			Count: s.Count,
		}
	}
	return result
}

// ==================== Namespace 对比转换 ====================

func convertNamespaceComparison(m *pmtypes.NamespaceComparison) types.NamespaceComparison {
	return types.NamespaceComparison{
		Namespaces: convertNamespaceComparisonItemList(m.Namespaces),
		Timestamp:  formatTime(m.Timestamp),
	}
}

func convertNamespaceComparisonItemList(list []pmtypes.NamespaceComparisonItem) []types.NamespaceComparisonItem {
	result := make([]types.NamespaceComparisonItem, len(list))
	for i, item := range list {
		result[i] = types.NamespaceComparisonItem{
			Namespace:     item.Namespace,
			CPUUsage:      item.CPUUsage,
			MemoryUsage:   item.MemoryUsage,
			PodCount:      item.PodCount,
			NetworkRxRate: item.NetworkRxRate,
			NetworkTxRate: item.NetworkTxRate,
		}
	}
	return result
}

// ==================== Namespace 概览转换 ====================

func convertResourceQuotaMetrics(m *pmtypes.ResourceQuotaMetrics) types.ResourceQuotaMetrics {
	return types.ResourceQuotaMetrics{
		Namespace:     m.Namespace,
		QuotaName:     m.QuotaName,
		CPUUsed:       m.CPUUsed,
		CPUHard:       m.CPUHard,
		CPUPercent:    m.CPUPercent,
		MemoryUsed:    m.MemoryUsed,
		MemoryHard:    m.MemoryHard,
		MemoryPercent: m.MemoryPercent,
		PodsUsed:      m.PodsUsed,
		PodsHard:      m.PodsHard,
		PodsPercent:   m.PodsPercent,
		Resources:     convertResourceQuotaDetailList(m.Resources),
	}
}
