package clustermonitor

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

// ==================== Cluster 综合概览转换 ====================

func convertClusterOverview(m *pmtypes.ClusterOverview) types.ClusterOverview {
	return types.ClusterOverview{
		ClusterName:  m.ClusterName,
		Timestamp:    formatTime(m.Timestamp),
		Resources:    convertClusterResourceMetrics(&m.Resources),
		Nodes:        convertClusterNodeMetrics(&m.Nodes),
		ControlPlane: convertClusterControlPlaneMetrics(&m.ControlPlane),
		Workloads:    convertClusterWorkloadMetrics(&m.Workloads),
		Network:      convertClusterNetworkMetrics(&m.Network),
		Storage:      convertClusterStorageMetrics(&m.Storage),
		Namespaces:   convertClusterNamespaceMetrics(&m.Namespaces),
	}
}

// ==================== 集群资源指标转换 ====================

func convertClusterResourceMetrics(m *pmtypes.ClusterResourceMetrics) types.ClusterResourceMetrics {
	return types.ClusterResourceMetrics{
		CPU:     convertClusterResourceSummary(&m.CPU),
		Memory:  convertClusterResourceSummary(&m.Memory),
		Pods:    convertClusterPodSummary(&m.Pods),
		Storage: convertClusterStorageSummary(&m.Storage),
	}
}

func convertClusterResourceSummary(s *pmtypes.ClusterResourceSummary) types.ClusterResourceSummary {
	return types.ClusterResourceSummary{
		Capacity:          sanitizeFloat64(s.Capacity),
		Allocatable:       sanitizeFloat64(s.Allocatable),
		RequestsAllocated: sanitizeFloat64(s.RequestsAllocated),
		LimitsAllocated:   sanitizeFloat64(s.LimitsAllocated),
		Usage:             sanitizeFloat64(s.Usage),
		RequestsPercent:   sanitizeFloat64(s.RequestsPercent),
		UsagePercent:      sanitizeFloat64(s.UsagePercent),
		Trend:             convertClusterResourceDataPoints(s.Trend),
		NoRequestsCount:   sanitizeInt64(s.NoRequestsCount),
		NoLimitsCount:     sanitizeInt64(s.NoLimitsCount),
	}
}

func convertClusterResourceDataPoints(points []pmtypes.ClusterResourceDataPoint) []types.ClusterResourceDataPoint {
	result := make([]types.ClusterResourceDataPoint, len(points))
	for i, p := range points {
		result[i] = types.ClusterResourceDataPoint{
			Timestamp:       formatTime(p.Timestamp),
			Usage:           sanitizeFloat64(p.Usage),
			UsagePercent:    sanitizeFloat64(p.UsagePercent),
			RequestsPercent: sanitizeFloat64(p.RequestsPercent),
		}
	}
	return result
}

func convertClusterPodSummary(s *pmtypes.ClusterPodSummary) types.ClusterPodSummary {
	return types.ClusterPodSummary{
		Capacity:     sanitizeInt64(s.Capacity),
		Running:      sanitizeInt64(s.Running),
		UsagePercent: sanitizeFloat64(s.UsagePercent),
	}
}

func convertClusterStorageSummary(s *pmtypes.ClusterStorageSummary) types.ClusterStorageSummary {
	return types.ClusterStorageSummary{
		TotalCapacityBytes: sanitizeInt64(s.TotalCapacityBytes),
		AllocatedBytes:     sanitizeInt64(s.AllocatedBytes),
		AllocationPercent:  sanitizeFloat64(s.AllocationPercent),
	}
}

// ==================== 节点状态统计转换 ====================

func convertClusterNodeMetrics(m *pmtypes.ClusterNodeMetrics) types.ClusterNodeMetrics {
	return types.ClusterNodeMetrics{
		Total:          sanitizeInt64(m.Total),
		Ready:          sanitizeInt64(m.Ready),
		NotReady:       sanitizeInt64(m.NotReady),
		Unknown:        sanitizeInt64(m.Unknown),
		MemoryPressure: sanitizeInt64(m.MemoryPressure),
		DiskPressure:   sanitizeInt64(m.DiskPressure),
		PIDPressure:    sanitizeInt64(m.PIDPressure),
		AvgCPUUsage:    sanitizeFloat64(m.AvgCPUUsage),
		AvgMemoryUsage: sanitizeFloat64(m.AvgMemoryUsage),
		HighLoadNodes:  sanitizeInt64(m.HighLoadNodes),
		NodeList:       convertNodeBriefStatusList(m.NodeList),
		Trend:          convertClusterNodeDataPoints(m.Trend),
	}
}

func convertNodeBriefStatusList(list []pmtypes.NodeBriefStatus) []types.NodeBriefStatus {
	result := make([]types.NodeBriefStatus, len(list))
	for i, n := range list {
		result[i] = types.NodeBriefStatus{
			NodeName:       n.NodeName,
			Ready:          n.Ready,
			MemoryPressure: n.MemoryPressure,
			DiskPressure:   n.DiskPressure,
			CPUUsage:       sanitizeFloat64(n.CPUUsage),
			MemoryUsage:    sanitizeFloat64(n.MemoryUsage),
			PodsCount:      sanitizeInt64(n.PodsCount),
		}
	}
	return result
}

func convertClusterNodeDataPoints(points []pmtypes.ClusterNodeDataPoint) []types.ClusterNodeDataPoint {
	result := make([]types.ClusterNodeDataPoint, len(points))
	for i, p := range points {
		result[i] = types.ClusterNodeDataPoint{
			Timestamp:      formatTime(p.Timestamp),
			ReadyNodes:     sanitizeInt64(p.ReadyNodes),
			NotReadyNodes:  sanitizeInt64(p.NotReadyNodes),
			AvgCPUUsage:    sanitizeFloat64(p.AvgCPUUsage),
			AvgMemoryUsage: sanitizeFloat64(p.AvgMemoryUsage),
		}
	}
	return result
}

// ==================== 控制平面指标转换（完善版）====================

func convertClusterControlPlaneMetrics(m *pmtypes.ClusterControlPlaneMetrics) types.ClusterControlPlaneMetrics {
	return types.ClusterControlPlaneMetrics{
		APIServer:         convertAPIServerMetrics(&m.APIServer),
		Etcd:              convertEtcdMetrics(&m.Etcd),
		Scheduler:         convertSchedulerMetrics(&m.Scheduler),
		ControllerManager: convertControllerManagerMetrics(&m.ControllerManager),
	}
}

// ==================== API Server 转换（完善版）====================

func convertAPIServerMetrics(m *pmtypes.APIServerMetrics) types.APIServerMetrics {
	return types.APIServerMetrics{
		// 基础指标
		RequestsPerSecond: sanitizeFloat64(m.RequestsPerSecond),
		ErrorRate:         sanitizeFloat64(m.ErrorRate),

		// 延迟指标
		P50Latency: sanitizeFloat64(m.P50Latency),
		P95Latency: sanitizeFloat64(m.P95Latency),
		P99Latency: sanitizeFloat64(m.P99Latency),

		// 并发指标
		CurrentInflightRequests:  sanitizeInt64(m.CurrentInflightRequests),
		InflightReadRequests:     sanitizeInt64(m.InflightReadRequests),
		InflightMutatingRequests: sanitizeInt64(m.InflightMutatingRequests),
		LongRunningRequests:      sanitizeInt64(m.LongRunningRequests),
		WatchCount:               sanitizeInt64(m.WatchCount),

		// 性能指标
		RequestDropped:         sanitizeInt64(m.RequestDropped),
		RequestTimeout:         sanitizeInt64(m.RequestTimeout),
		ResponseSizeBytes:      sanitizeFloat64(m.ResponseSizeBytes),
		WebhookDurationSeconds: sanitizeFloat64(m.WebhookDurationSeconds),

		// 认证鉴权
		AuthenticationAttempts:   sanitizeFloat64(m.AuthenticationAttempts),
		AuthenticationFailures:   sanitizeFloat64(m.AuthenticationFailures),
		AuthorizationAttempts:    sanitizeFloat64(m.AuthorizationAttempts),
		AuthorizationDuration:    sanitizeFloat64(m.AuthorizationDuration),
		ClientCertExpirationDays: sanitizeFloat64(m.ClientCertExpirationDays),

		// 分类统计
		RequestsByVerb:     convertVerbMetricsList(m.RequestsByVerb),
		RequestsByCode:     convertStatusCodeDistribution(&m.RequestsByCode),
		RequestsByResource: convertResourceMetricsList(m.RequestsByResource),

		// 趋势数据
		Trend: convertAPIServerDataPoints(m.Trend),
	}
}

func convertResourceMetricsList(list []pmtypes.ResourceMetrics) []types.ResourceMetrics {
	result := make([]types.ResourceMetrics, len(list))
	for i, r := range list {
		result[i] = types.ResourceMetrics{
			Resource:          r.Resource,
			RequestsPerSecond: sanitizeFloat64(r.RequestsPerSecond),
		}
	}
	return result
}

func convertVerbMetricsList(list []pmtypes.VerbMetrics) []types.VerbMetrics {
	result := make([]types.VerbMetrics, len(list))
	for i, v := range list {
		result[i] = types.VerbMetrics{
			Verb:              v.Verb,
			RequestsPerSecond: sanitizeFloat64(v.RequestsPerSecond),
		}
	}
	return result
}

func convertStatusCodeDistribution(d *pmtypes.StatusCodeDistribution) types.StatusCodeDistribution {
	return types.StatusCodeDistribution{
		Status2xx: sanitizeMapValues(d.Status2xx),
		Status3xx: sanitizeMapValues(d.Status3xx),
		Status4xx: sanitizeMapValues(d.Status4xx),
		Status5xx: sanitizeMapValues(d.Status5xx),
	}
}

// sanitizeMapValues 清理 map 中的 NaN 值
func sanitizeMapValues(m map[string]float64) map[string]float64 {
	if m == nil {
		return make(map[string]float64)
	}
	result := make(map[string]float64, len(m))
	for k, v := range m {
		result[k] = sanitizeFloat64(v)
	}
	return result
}

func convertAPIServerDataPoints(points []pmtypes.APIServerDataPoint) []types.APIServerDataPoint {
	result := make([]types.APIServerDataPoint, len(points))
	for i, p := range points {
		result[i] = types.APIServerDataPoint{
			Timestamp:               formatTime(p.Timestamp),
			RequestsPerSecond:       sanitizeFloat64(p.RequestsPerSecond),
			ErrorRate:               sanitizeFloat64(p.ErrorRate),
			P95Latency:              sanitizeFloat64(p.P95Latency),
			CurrentInflightRequests: sanitizeInt64(p.CurrentInflightRequests),
			LongRunningRequests:     sanitizeInt64(p.LongRunningRequests),
		}
	}
	return result
}

// ==================== etcd 转换（完善版）====================

func convertEtcdMetrics(m *pmtypes.EtcdMetrics) types.EtcdMetrics {
	return types.EtcdMetrics{
		// 集群状态
		HasLeader:     m.HasLeader,
		LeaderChanges: sanitizeInt64(m.LeaderChanges),
		MemberCount:   sanitizeInt64(m.MemberCount),
		IsLearner:     m.IsLearner,

		// 存储指标
		DBSizeBytes: sanitizeInt64(m.DBSizeBytes),
		DBSizeInUse: sanitizeInt64(m.DBSizeInUse),
		DBSizeLimit: sanitizeInt64(m.DBSizeLimit),
		KeyTotal:    sanitizeInt64(m.KeyTotal),

		// 性能指标
		CommitLatency:   sanitizeFloat64(m.CommitLatency),
		WALFsyncLatency: sanitizeFloat64(m.WALFsyncLatency),
		ApplyLatency:    sanitizeFloat64(m.ApplyLatency),
		SnapshotLatency: sanitizeFloat64(m.SnapshotLatency),
		CompactLatency:  sanitizeFloat64(m.CompactLatency),

		// 网络指标
		PeerRTT:         sanitizeFloat64(m.PeerRTT),
		NetworkSendRate: sanitizeFloat64(m.NetworkSendRate),
		NetworkRecvRate: sanitizeFloat64(m.NetworkRecvRate),

		// 操作统计
		ProposalsFailed:    sanitizeInt64(m.ProposalsFailed),
		ProposalsPending:   sanitizeInt64(m.ProposalsPending),
		ProposalsApplied:   sanitizeInt64(m.ProposalsApplied),
		ProposalsCommitted: sanitizeInt64(m.ProposalsCommitted),

		// 操作速率
		GetRate:    sanitizeFloat64(m.GetRate),
		PutRate:    sanitizeFloat64(m.PutRate),
		DeleteRate: sanitizeFloat64(m.DeleteRate),

		// 慢操作
		SlowApplies: sanitizeInt64(m.SlowApplies),
		SlowCommits: sanitizeInt64(m.SlowCommits),

		// 趋势数据
		Trend: convertEtcdDataPoints(m.Trend),
	}
}

func convertEtcdDataPoints(points []pmtypes.EtcdDataPoint) []types.EtcdDataPoint {
	result := make([]types.EtcdDataPoint, len(points))
	for i, p := range points {
		result[i] = types.EtcdDataPoint{
			Timestamp:       formatTime(p.Timestamp),
			DBSizeBytes:     sanitizeInt64(p.DBSizeBytes),
			CommitLatency:   sanitizeFloat64(p.CommitLatency),
			ProposalsFailed: sanitizeInt64(p.ProposalsFailed),
			PeerRTT:         sanitizeFloat64(p.PeerRTT),
			GetRate:         sanitizeFloat64(p.GetRate),
			PutRate:         sanitizeFloat64(p.PutRate),
		}
	}
	return result
}

// ==================== Scheduler 转换（完善版）====================

func convertSchedulerMetrics(m *pmtypes.SchedulerMetrics) types.SchedulerMetrics {
	return types.SchedulerMetrics{
		// 基础指标
		ScheduleSuccessRate: sanitizeFloat64(m.ScheduleSuccessRate),
		ScheduleAttempts:    sanitizeFloat64(m.ScheduleAttempts),
		PendingPods:         sanitizeInt64(m.PendingPods),
		UnschedulablePods:   sanitizeInt64(m.UnschedulablePods),

		// 延迟指标
		P50ScheduleLatency: sanitizeFloat64(m.P50ScheduleLatency),
		P95ScheduleLatency: sanitizeFloat64(m.P95ScheduleLatency),
		P99ScheduleLatency: sanitizeFloat64(m.P99ScheduleLatency),
		BindingLatency:     sanitizeFloat64(m.BindingLatency),

		// 调度结果
		ScheduledPods:      sanitizeInt64(m.ScheduledPods),
		FailedScheduling:   sanitizeInt64(m.FailedScheduling),
		PreemptionAttempts: sanitizeInt64(m.PreemptionAttempts),
		PreemptionVictims:  sanitizeInt64(m.PreemptionVictims),

		// 失败原因
		FailureReasons: convertScheduleFailureReasons(&m.FailureReasons),

		// 调度队列
		SchedulingQueueLength: sanitizeInt64(m.SchedulingQueueLength),
		ActiveQueueLength:     sanitizeInt64(m.ActiveQueueLength),
		BackoffQueueLength:    sanitizeInt64(m.BackoffQueueLength),

		// 插件延迟
		PluginLatency: convertSchedulerPluginLatency(&m.PluginLatency),

		// 趋势数据
		Trend: convertSchedulerDataPoints(m.Trend),
	}
}

func convertScheduleFailureReasons(r *pmtypes.ScheduleFailureReasons) types.ScheduleFailureReasons {
	return types.ScheduleFailureReasons{
		InsufficientCPU:    sanitizeInt64(r.InsufficientCPU),
		InsufficientMemory: sanitizeInt64(r.InsufficientMemory),
		NodeAffinity:       sanitizeInt64(r.NodeAffinity),
		PodAffinity:        sanitizeInt64(r.PodAffinity),
		Taint:              sanitizeInt64(r.Taint),
		VolumeBinding:      sanitizeInt64(r.VolumeBinding),
		NoNodesAvailable:   sanitizeInt64(r.NoNodesAvailable),
	}
}

func convertSchedulerPluginLatency(l *pmtypes.SchedulerPluginLatency) types.SchedulerPluginLatency {
	return types.SchedulerPluginLatency{
		FilterLatency:  sanitizeFloat64(l.FilterLatency),
		ScoreLatency:   sanitizeFloat64(l.ScoreLatency),
		PreBindLatency: sanitizeFloat64(l.PreBindLatency),
		BindLatency:    sanitizeFloat64(l.BindLatency),
	}
}

func convertSchedulerDataPoints(points []pmtypes.SchedulerDataPoint) []types.SchedulerDataPoint {
	result := make([]types.SchedulerDataPoint, len(points))
	for i, p := range points {
		result[i] = types.SchedulerDataPoint{
			Timestamp:           formatTime(p.Timestamp),
			ScheduleSuccessRate: sanitizeFloat64(p.ScheduleSuccessRate),
			PendingPods:         sanitizeInt64(p.PendingPods),
			P95ScheduleLatency:  sanitizeFloat64(p.P95ScheduleLatency),
			ScheduleAttempts:    sanitizeFloat64(p.ScheduleAttempts),
		}
	}
	return result
}

// ==================== Controller Manager 转换（完善版）====================

func convertControllerManagerMetrics(m *pmtypes.ControllerManagerMetrics) types.ControllerManagerMetrics {
	return types.ControllerManagerMetrics{
		// Leader 选举
		IsLeader:      m.IsLeader,
		LeaderChanges: sanitizeInt64(m.LeaderChanges),

		// 工作队列深度
		DeploymentQueueDepth:  sanitizeInt64(m.DeploymentQueueDepth),
		ReplicaSetQueueDepth:  sanitizeInt64(m.ReplicaSetQueueDepth),
		StatefulSetQueueDepth: sanitizeInt64(m.StatefulSetQueueDepth),
		DaemonSetQueueDepth:   sanitizeInt64(m.DaemonSetQueueDepth),
		JobQueueDepth:         sanitizeInt64(m.JobQueueDepth),
		NodeQueueDepth:        sanitizeInt64(m.NodeQueueDepth),
		ServiceQueueDepth:     sanitizeInt64(m.ServiceQueueDepth),
		EndpointQueueDepth:    sanitizeInt64(m.EndpointQueueDepth),

		// 队列延迟
		QueueLatency: convertQueueLatencyMetrics(&m.QueueLatency),

		// 工作队列统计
		WorkQueueMetrics: convertControllerWorkQueueMetrics(&m.WorkQueueMetrics),

		// 控制器协调延迟
		ReconcileLatency: convertControllerReconcileLatency(&m.ReconcileLatency),

		// 错误和重试
		TotalSyncErrors: sanitizeInt64(m.TotalSyncErrors),
		RetryRate:       sanitizeFloat64(m.RetryRate),

		// 趋势数据
		Trend: convertControllerManagerDataPoints(m.Trend),
	}
}

func convertQueueLatencyMetrics(m *pmtypes.QueueLatencyMetrics) types.QueueLatencyMetrics {
	return types.QueueLatencyMetrics{
		QueueDuration: sanitizeFloat64(m.QueueDuration),
		WorkDuration:  sanitizeFloat64(m.WorkDuration),
	}
}

func convertControllerWorkQueueMetrics(m *pmtypes.ControllerWorkQueueMetrics) types.ControllerWorkQueueMetrics {
	return types.ControllerWorkQueueMetrics{
		AddsRate:       sanitizeFloat64(m.AddsRate),
		DepthTotal:     sanitizeInt64(m.DepthTotal),
		UnfinishedWork: sanitizeInt64(m.UnfinishedWork),
		LongestRunning: sanitizeFloat64(m.LongestRunning),
		RetriesRate:    sanitizeFloat64(m.RetriesRate),
	}
}

func convertControllerReconcileLatency(l *pmtypes.ControllerReconcileLatency) types.ControllerReconcileLatency {
	return types.ControllerReconcileLatency{
		DeploymentP99:  sanitizeFloat64(l.DeploymentP99),
		ReplicaSetP99:  sanitizeFloat64(l.ReplicaSetP99),
		StatefulSetP99: sanitizeFloat64(l.StatefulSetP99),
		DaemonSetP99:   sanitizeFloat64(l.DaemonSetP99),
		JobP99:         sanitizeFloat64(l.JobP99),
	}
}

func convertControllerManagerDataPoints(points []pmtypes.ControllerManagerDataPoint) []types.ControllerManagerDataPoint {
	result := make([]types.ControllerManagerDataPoint, len(points))
	for i, p := range points {
		result[i] = types.ControllerManagerDataPoint{
			Timestamp:             formatTime(p.Timestamp),
			DeploymentQueueDepth:  sanitizeInt64(p.DeploymentQueueDepth),
			ReplicaSetQueueDepth:  sanitizeInt64(p.ReplicaSetQueueDepth),
			StatefulSetQueueDepth: sanitizeInt64(p.StatefulSetQueueDepth),
			TotalQueueDepth:       sanitizeInt64(p.TotalQueueDepth),
			RetryRate:             sanitizeFloat64(p.RetryRate),
		}
	}
	return result
}

// ==================== 工作负载统计转换 ====================

func convertClusterWorkloadMetrics(m *pmtypes.ClusterWorkloadMetrics) types.ClusterWorkloadMetrics {
	return types.ClusterWorkloadMetrics{
		Pods:         convertClusterPodMetrics(&m.Pods),
		Deployments:  convertClusterDeploymentMetrics(&m.Deployments),
		StatefulSets: convertClusterStatefulSetMetrics(&m.StatefulSets),
		DaemonSets:   convertClusterDaemonSetMetrics(&m.DaemonSets),
		Jobs:         convertClusterJobMetrics(&m.Jobs),
		Services:     convertClusterServiceMetrics(&m.Services),
		Containers:   convertClusterContainerMetrics(&m.Containers),
	}
}

func convertClusterPodMetrics(m *pmtypes.ClusterPodMetrics) types.ClusterPodMetrics {
	return types.ClusterPodMetrics{
		Total:           sanitizeInt64(m.Total),
		Running:         sanitizeInt64(m.Running),
		Pending:         sanitizeInt64(m.Pending),
		Failed:          sanitizeInt64(m.Failed),
		Succeeded:       sanitizeInt64(m.Succeeded),
		Unknown:         sanitizeInt64(m.Unknown),
		Ready:           sanitizeInt64(m.Ready),
		NotReady:        sanitizeInt64(m.NotReady),
		TotalRestarts:   sanitizeInt64(m.TotalRestarts),
		HighRestartPods: convertPodRestartInfoList(m.HighRestartPods),
		Trend:           convertClusterPodDataPoints(m.Trend),
	}
}

func convertPodRestartInfoList(list []pmtypes.PodRestartInfo) []types.PodRestartInfo {
	result := make([]types.PodRestartInfo, len(list))
	for i, p := range list {
		result[i] = types.PodRestartInfo{
			Namespace:    p.Namespace,
			PodName:      p.PodName,
			RestartCount: sanitizeInt64(p.RestartCount),
			RestartRate:  sanitizeFloat64(p.RestartRate),
		}
	}
	return result
}

func convertClusterPodDataPoints(points []pmtypes.ClusterPodDataPoint) []types.ClusterPodDataPoint {
	result := make([]types.ClusterPodDataPoint, len(points))
	for i, p := range points {
		result[i] = types.ClusterPodDataPoint{
			Timestamp: formatTime(p.Timestamp),
			Total:     sanitizeInt64(p.Total),
			Running:   sanitizeInt64(p.Running),
			Pending:   sanitizeInt64(p.Pending),
			Failed:    sanitizeInt64(p.Failed),
		}
	}
	return result
}

func convertClusterDeploymentMetrics(m *pmtypes.ClusterDeploymentMetrics) types.ClusterDeploymentMetrics {
	return types.ClusterDeploymentMetrics{
		Total:               sanitizeInt64(m.Total),
		AvailableReplicas:   sanitizeInt64(m.AvailableReplicas),
		UnavailableReplicas: sanitizeInt64(m.UnavailableReplicas),
		Updating:            sanitizeInt64(m.Updating),
	}
}

func convertClusterStatefulSetMetrics(m *pmtypes.ClusterStatefulSetMetrics) types.ClusterStatefulSetMetrics {
	return types.ClusterStatefulSetMetrics{
		Total:           sanitizeInt64(m.Total),
		ReadyReplicas:   sanitizeInt64(m.ReadyReplicas),
		CurrentReplicas: sanitizeInt64(m.CurrentReplicas),
	}
}

func convertClusterDaemonSetMetrics(m *pmtypes.ClusterDaemonSetMetrics) types.ClusterDaemonSetMetrics {
	return types.ClusterDaemonSetMetrics{
		Total:           sanitizeInt64(m.Total),
		DesiredPods:     sanitizeInt64(m.DesiredPods),
		AvailablePods:   sanitizeInt64(m.AvailablePods),
		UnavailablePods: sanitizeInt64(m.UnavailablePods),
	}
}

func convertClusterJobMetrics(m *pmtypes.ClusterJobMetrics) types.ClusterJobMetrics {
	return types.ClusterJobMetrics{
		Active:        sanitizeInt64(m.Active),
		Succeeded:     sanitizeInt64(m.Succeeded),
		Failed:        sanitizeInt64(m.Failed),
		CronJobsTotal: sanitizeInt64(m.CronJobsTotal),
	}
}

func convertClusterServiceMetrics(m *pmtypes.ClusterServiceMetrics) types.ClusterServiceMetrics {
	return types.ClusterServiceMetrics{
		Total:  sanitizeInt64(m.Total),
		ByType: convertServiceTypeCountList(m.ByType),
	}
}

func convertServiceTypeCountList(list []pmtypes.ServiceTypeCount) []types.ServiceTypeCount {
	result := make([]types.ServiceTypeCount, len(list))
	for i, s := range list {
		result[i] = types.ServiceTypeCount{
			Type:  s.Type,
			Count: sanitizeInt64(s.Count),
		}
	}
	return result
}

func convertClusterContainerMetrics(m *pmtypes.ClusterContainerMetrics) types.ClusterContainerMetrics {
	return types.ClusterContainerMetrics{
		Total:   sanitizeInt64(m.Total),
		Running: sanitizeInt64(m.Running),
	}
}

// ==================== 网络和存储转换 ====================

func convertClusterNetworkMetrics(m *pmtypes.ClusterNetworkMetrics) types.ClusterNetworkMetrics {
	return types.ClusterNetworkMetrics{
		TotalIngressBytesPerSec: sanitizeFloat64(m.TotalIngressBytesPerSec),
		TotalEgressBytesPerSec:  sanitizeFloat64(m.TotalEgressBytesPerSec),
		Trend:                   convertClusterNetworkDataPoints(m.Trend),
	}
}

func convertClusterNetworkDataPoints(points []pmtypes.ClusterNetworkDataPoint) []types.ClusterNetworkDataPoint {
	result := make([]types.ClusterNetworkDataPoint, len(points))
	for i, p := range points {
		result[i] = types.ClusterNetworkDataPoint{
			Timestamp:          formatTime(p.Timestamp),
			IngressBytesPerSec: sanitizeFloat64(p.IngressBytesPerSec),
			EgressBytesPerSec:  sanitizeFloat64(p.EgressBytesPerSec),
		}
	}
	return result
}

func convertClusterStorageMetrics(m *pmtypes.ClusterStorageMetrics) types.ClusterStorageMetrics {
	return types.ClusterStorageMetrics{
		PVTotal:        sanitizeInt64(m.PVTotal),
		PVBound:        sanitizeInt64(m.PVBound),
		PVReleased:     sanitizeInt64(m.PVReleased),
		PVAvailable:    sanitizeInt64(m.PVAvailable),
		PVFailed:       sanitizeInt64(m.PVFailed),
		PVCTotal:       sanitizeInt64(m.PVCTotal),
		PVCBound:       sanitizeInt64(m.PVCBound),
		PVCPending:     sanitizeInt64(m.PVCPending),
		PVCLost:        sanitizeInt64(m.PVCLost),
		StorageClasses: convertStorageClassUsageList(m.StorageClasses),
	}
}

func convertStorageClassUsageList(list []pmtypes.StorageClassUsage) []types.StorageClassUsage {
	result := make([]types.StorageClassUsage, len(list))
	for i, s := range list {
		result[i] = types.StorageClassUsage{
			StorageClass: s.StorageClass,
			PVCCount:     sanitizeInt64(s.PVCCount),
			TotalBytes:   sanitizeInt64(s.TotalBytes),
		}
	}
	return result
}

// ==================== Namespace 统计转换 ====================

func convertClusterNamespaceMetrics(m *pmtypes.ClusterNamespaceMetrics) types.ClusterNamespaceMetrics {
	return types.ClusterNamespaceMetrics{
		Total:       sanitizeInt64(m.Total),
		Active:      sanitizeInt64(m.Active),
		TopByCPU:    convertNamespaceResourceRankingList(m.TopByCPU),
		TopByMemory: convertNamespaceResourceRankingList(m.TopByMemory),
		TopByPods:   convertNamespaceResourceRankingList(m.TopByPods),
	}
}

func convertNamespaceResourceRankingList(list []pmtypes.NamespaceResourceRanking) []types.NamespaceResourceRanking {
	result := make([]types.NamespaceResourceRanking, len(list))
	for i, n := range list {
		result[i] = types.NamespaceResourceRanking{
			Namespace: n.Namespace,
			Value:     sanitizeFloat64(n.Value),
			Unit:      n.Unit,
		}
	}
	return result
}
