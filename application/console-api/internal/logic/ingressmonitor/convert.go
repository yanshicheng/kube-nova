package ingressmonitor

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

// ==================== Ingress 综合指标转换 ====================

func convertIngressMetrics(m *pmtypes.IngressMetrics) types.IngressMetrics {
	return types.IngressMetrics{
		Namespace:    m.Namespace,
		IngressName:  m.IngressName,
		Timestamp:    formatTime(m.Timestamp),
		Controller:   convertIngressControllerHealth(&m.Controller),
		Traffic:      convertIngressTrafficMetrics(&m.Traffic),
		Performance:  convertIngressPerformanceMetrics(&m.Performance),
		Errors:       convertIngressErrorMetrics(&m.Errors),
		Backends:     convertIngressBackendMetrics(&m.Backends),
		Certificates: convertIngressCertificateMetrics(&m.Certificates),
	}
}

func convertIngressMetricsList(list []pmtypes.IngressMetrics) []types.IngressMetrics {
	result := make([]types.IngressMetrics, len(list))
	for i, m := range list {
		result[i] = convertIngressMetrics(&m)
	}
	return result
}

// ==================== Controller 健康转换 ====================

func convertIngressControllerHealth(h *pmtypes.IngressControllerHealth) types.IngressControllerHealth {
	return types.IngressControllerHealth{
		ControllerName:    h.ControllerName,
		TotalPods:         h.TotalPods,
		RunningPods:       h.RunningPods,
		ReadyPods:         h.ReadyPods,
		CPUUsage:          h.CPUUsage,
		MemoryUsage:       h.MemoryUsage,
		LastReloadSuccess: h.LastReloadSuccess,
		LastReloadTime:    formatTime(h.LastReloadTime),
		ReloadRate:        h.ReloadRate,
		ReloadFailRate:    h.ReloadFailRate,
		Connections:       convertIngressConnectionMetrics(&h.Connections),
		Trend:             convertIngressControllerDataPoints(h.Trend),
	}
}

func convertIngressConnectionMetrics(m *pmtypes.IngressConnectionMetrics) types.IngressConnectionMetrics {
	return types.IngressConnectionMetrics{
		Active:  m.Active,
		Reading: m.Reading,
		Writing: m.Writing,
		Waiting: m.Waiting,
		Total:   m.Total,
	}
}

func convertIngressControllerDataPoints(points []pmtypes.IngressControllerDataPoint) []types.IngressControllerDataPoint {
	result := make([]types.IngressControllerDataPoint, len(points))
	for i, p := range points {
		result[i] = types.IngressControllerDataPoint{
			Timestamp:         formatTime(p.Timestamp),
			ActiveConnections: p.ActiveConnections,
			ReloadRate:        p.ReloadRate,
			CPUUsage:          p.CPUUsage,
			MemoryUsage:       p.MemoryUsage,
		}
	}
	return result
}

// ==================== 流量指标转换 ====================

func convertIngressTrafficMetrics(m *pmtypes.IngressTrafficMetrics) types.IngressTrafficMetrics {
	return types.IngressTrafficMetrics{
		Current:   convertIngressTrafficSnapshot(&m.Current),
		Trend:     convertIngressTrafficDataPoints(m.Trend),
		ByHost:    convertTrafficByDimensionList(m.ByHost),
		ByPath:    convertTrafficByDimensionList(m.ByPath),
		ByService: convertTrafficByDimensionList(m.ByService),
		ByMethod:  convertTrafficByMethodList(m.ByMethod),
		Summary:   convertIngressTrafficSummary(&m.Summary),
	}
}

func convertIngressTrafficSnapshot(s *pmtypes.IngressTrafficSnapshot) types.IngressTrafficSnapshot {
	return types.IngressTrafficSnapshot{
		Timestamp:          formatTime(s.Timestamp),
		RequestsPerSecond:  s.RequestsPerSecond,
		ActiveConnections:  s.ActiveConnections,
		IngressBytesPerSec: s.IngressBytesPerSec,
		EgressBytesPerSec:  s.EgressBytesPerSec,
	}
}

func convertIngressTrafficDataPoints(points []pmtypes.IngressTrafficDataPoint) []types.IngressTrafficDataPoint {
	result := make([]types.IngressTrafficDataPoint, len(points))
	for i, p := range points {
		result[i] = types.IngressTrafficDataPoint{
			Timestamp:          formatTime(p.Timestamp),
			RequestsPerSecond:  p.RequestsPerSecond,
			IngressBytesPerSec: p.IngressBytesPerSec,
			EgressBytesPerSec:  p.EgressBytesPerSec,
		}
	}
	return result
}

func convertTrafficByDimensionList(list []pmtypes.TrafficByDimension) []types.TrafficByDimension {
	result := make([]types.TrafficByDimension, len(list))
	for i, t := range list {
		result[i] = types.TrafficByDimension{
			Name:              t.Name,
			Namespace:         t.Namespace,
			RequestsPerSecond: t.RequestsPerSecond,
			BytesPerSecond:    t.BytesPerSecond,
		}
	}
	return result
}

func convertTrafficByMethodList(list []pmtypes.TrafficByMethod) []types.TrafficByMethod {
	result := make([]types.TrafficByMethod, len(list))
	for i, t := range list {
		result[i] = types.TrafficByMethod{
			Method:            t.Method,
			RequestsPerSecond: t.RequestsPerSecond,
		}
	}
	return result
}

func convertIngressTrafficSummary(s *pmtypes.IngressTrafficSummary) types.IngressTrafficSummary {
	return types.IngressTrafficSummary{
		TotalRequests:     s.TotalRequests,
		TotalIngressBytes: s.TotalIngressBytes,
		TotalEgressBytes:  s.TotalEgressBytes,
		AvgRequestsPerSec: s.AvgRequestsPerSec,
		MaxRequestsPerSec: s.MaxRequestsPerSec,
		AvgRequestSize:    s.AvgRequestSize,
		AvgResponseSize:   s.AvgResponseSize,
	}
}

// ==================== 性能指标转换 ====================

func convertIngressPerformanceMetrics(m *pmtypes.IngressPerformanceMetrics) types.IngressPerformanceMetrics {
	return types.IngressPerformanceMetrics{
		Overall:         convertIngressLatencyStats(&m.Overall),
		ByHost:          convertLatencyByDimensionList(m.ByHost),
		ByPath:          convertLatencyByDimensionList(m.ByPath),
		UpstreamLatency: convertIngressLatencyStats(&m.UpstreamLatency),
		Trend:           convertIngressLatencyDataPoints(m.Trend),
	}
}

func convertIngressLatencyStats(s *pmtypes.IngressLatencyStats) types.IngressLatencyStats {
	return types.IngressLatencyStats{
		P50: s.P50,
		P95: s.P95,
		P99: s.P99,
		Avg: s.Avg,
		Max: s.Max,
	}
}

func convertLatencyByDimensionList(list []pmtypes.LatencyByDimension) []types.LatencyByDimension {
	result := make([]types.LatencyByDimension, len(list))
	for i, l := range list {
		result[i] = types.LatencyByDimension{
			Name:      l.Name,
			Namespace: l.Namespace,
			Latency:   convertIngressLatencyStats(&l.Latency),
		}
	}
	return result
}

func convertIngressLatencyDataPoints(points []pmtypes.IngressLatencyDataPoint) []types.IngressLatencyDataPoint {
	result := make([]types.IngressLatencyDataPoint, len(points))
	for i, p := range points {
		result[i] = types.IngressLatencyDataPoint{
			Timestamp: formatTime(p.Timestamp),
			P50:       p.P50,
			P95:       p.P95,
			P99:       p.P99,
		}
	}
	return result
}

// ==================== 错误指标转换 ====================

func convertIngressErrorMetrics(m *pmtypes.IngressErrorMetrics) types.IngressErrorMetrics {
	return types.IngressErrorMetrics{
		Overall:     convertIngressErrorRateStats(&m.Overall),
		StatusCodes: convertIngressStatusCodeDistribution(&m.StatusCodes),
		ByHost:      convertErrorRateByDimensionList(m.ByHost),
		ByPath:      convertErrorRateByDimensionList(m.ByPath),
		Trend:       convertIngressErrorDataPoints(m.Trend),
	}
}

func convertIngressErrorRateStats(s *pmtypes.IngressErrorRateStats) types.IngressErrorRateStats {
	return types.IngressErrorRateStats{
		TotalErrorRate: s.TotalErrorRate,
		Error4xxRate:   s.Error4xxRate,
		Error5xxRate:   s.Error5xxRate,
	}
}

func convertIngressStatusCodeDistribution(d *pmtypes.IngressStatusCodeDistribution) types.IngressStatusCodeDistribution {
	return types.IngressStatusCodeDistribution{
		Status2xx: d.Status2xx,
		Status3xx: d.Status3xx,
		Status4xx: d.Status4xx,
		Status5xx: d.Status5xx,
	}
}

func convertErrorRateByDimensionList(list []pmtypes.ErrorRateByDimension) []types.ErrorRateByDimension {
	result := make([]types.ErrorRateByDimension, len(list))
	for i, e := range list {
		result[i] = types.ErrorRateByDimension{
			Name:      e.Name,
			Namespace: e.Namespace,
			ErrorRate: convertIngressErrorRateStats(&e.ErrorRate),
			TopErrors: convertStatusCodeRateList(e.TopErrors),
		}
	}
	return result
}

func convertStatusCodeRateList(list []pmtypes.StatusCodeRate) []types.StatusCodeRate {
	result := make([]types.StatusCodeRate, len(list))
	for i, s := range list {
		result[i] = types.StatusCodeRate{
			StatusCode: s.StatusCode,
			Rate:       s.Rate,
			Percent:    s.Percent,
		}
	}
	return result
}

func convertIngressErrorDataPoints(points []pmtypes.IngressErrorDataPoint) []types.IngressErrorDataPoint {
	result := make([]types.IngressErrorDataPoint, len(points))
	for i, p := range points {
		result[i] = types.IngressErrorDataPoint{
			Timestamp:      formatTime(p.Timestamp),
			TotalErrorRate: p.TotalErrorRate,
			Error4xxRate:   p.Error4xxRate,
			Error5xxRate:   p.Error5xxRate,
		}
	}
	return result
}

// ==================== 后端健康转换 ====================

func convertIngressBackendMetrics(m *pmtypes.IngressBackendMetrics) types.IngressBackendMetrics {
	return types.IngressBackendMetrics{
		UpstreamLatency:    m.UpstreamLatency,
		EndpointsByService: convertServiceEndpointsList(m.EndpointsByService),
		BackendHealth:      convertBackendHealthStatusList(m.BackendHealth),
	}
}

func convertServiceEndpointsList(list []pmtypes.ServiceEndpoints) []types.ServiceEndpoints {
	result := make([]types.ServiceEndpoints, len(list))
	for i, s := range list {
		result[i] = types.ServiceEndpoints{
			ServiceName:        s.ServiceName,
			Namespace:          s.Namespace,
			AvailableEndpoints: s.AvailableEndpoints,
			HasEndpoints:       s.HasEndpoints,
		}
	}
	return result
}

func convertBackendHealthStatusList(list []pmtypes.BackendHealthStatus) []types.BackendHealthStatus {
	result := make([]types.BackendHealthStatus, len(list))
	for i, b := range list {
		result[i] = types.BackendHealthStatus{
			Upstream:          b.Upstream,
			ResponseLatency:   b.ResponseLatency,
			SuccessRate:       b.SuccessRate,
			ActiveConnections: b.ActiveConnections,
		}
	}
	return result
}

// ==================== 证书指标转换 ====================

func convertIngressCertificateMetrics(m *pmtypes.IngressCertificateMetrics) types.IngressCertificateMetrics {
	return types.IngressCertificateMetrics{
		HTTPSRequestPercent: m.HTTPSRequestPercent,
		Certificates:        convertCertificateInfoList(m.Certificates),
		ExpiringCount:       m.ExpiringCount,
		ExpiredCount:        m.ExpiredCount,
	}
}

func convertCertificateInfoList(list []pmtypes.CertificateInfo) []types.CertificateInfo {
	result := make([]types.CertificateInfo, len(list))
	for i, c := range list {
		result[i] = types.CertificateInfo{
			Name:           c.Name,
			Namespace:      c.Namespace,
			ExpirationTime: formatTime(c.ExpirationTime),
			DaysRemaining:  c.DaysRemaining,
			IsExpiring:     c.IsExpiring,
			IsExpired:      c.IsExpired,
		}
	}
	return result
}

// ==================== Ingress 对象状态转换 ====================

func convertIngressObjectMetrics(m *pmtypes.IngressObjectMetrics) types.IngressObjectMetrics {
	return types.IngressObjectMetrics{
		Namespace:   m.Namespace,
		IngressName: m.IngressName,
		PathCount:   m.PathCount,
		RuleCount:   m.RuleCount,
		TLSEnabled:  m.TLSEnabled,
		Hosts:       m.Hosts,
		Paths:       convertIngressPathInfoList(m.Paths),
		CreatedAt:   formatTime(m.CreatedAt),
		Labels:      m.Labels,
		Annotations: m.Annotations,
	}
}

func convertIngressPathInfoList(list []pmtypes.IngressPathInfo) []types.IngressPathInfo {
	result := make([]types.IngressPathInfo, len(list))
	for i, p := range list {
		result[i] = types.IngressPathInfo{
			Host:        p.Host,
			Path:        p.Path,
			PathType:    p.PathType,
			ServiceName: p.ServiceName,
			ServicePort: p.ServicePort,
		}
	}
	return result
}

// ==================== 限流指标转换 ====================

func convertIngressRateLimitMetrics(m *pmtypes.IngressRateLimitMetrics) types.IngressRateLimitMetrics {
	return types.IngressRateLimitMetrics{
		Namespace:        m.Namespace,
		IngressName:      m.IngressName,
		LimitedRequests:  m.LimitedRequests,
		LimitTriggerRate: m.LimitTriggerRate,
		ByPath:           convertRateLimitByPathList(m.ByPath),
		Trend:            convertIngressRateLimitDataPoints(m.Trend),
	}
}

func convertRateLimitByPathList(list []pmtypes.RateLimitByPath) []types.RateLimitByPath {
	result := make([]types.RateLimitByPath, len(list))
	for i, r := range list {
		result[i] = types.RateLimitByPath{
			Path:                r.Path,
			LimitedRequests:     r.LimitedRequests,
			TotalRequests:       r.TotalRequests,
			LimitTriggerPercent: r.LimitTriggerPercent,
		}
	}
	return result
}

func convertIngressRateLimitDataPoints(points []pmtypes.IngressRateLimitDataPoint) []types.IngressRateLimitDataPoint {
	result := make([]types.IngressRateLimitDataPoint, len(points))
	for i, p := range points {
		result[i] = types.IngressRateLimitDataPoint{
			Timestamp:        formatTime(p.Timestamp),
			LimitedRequests:  p.LimitedRequests,
			LimitTriggerRate: p.LimitTriggerRate,
		}
	}
	return result
}

// ==================== Ingress 排行转换 ====================

func convertIngressRanking(r *pmtypes.IngressRanking) types.IngressRanking {
	return types.IngressRanking{
		TopByQPS:       convertIngressRankingItemList(r.TopByQPS),
		TopByErrorRate: convertIngressRankingItemList(r.TopByErrorRate),
		TopByLatency:   convertIngressRankingItemList(r.TopByLatency),
		TopByTraffic:   convertIngressRankingItemList(r.TopByTraffic),
	}
}

func convertIngressRankingItemList(list []pmtypes.IngressRankingItem) []types.IngressRankingItem {
	result := make([]types.IngressRankingItem, len(list))
	for i, item := range list {
		result[i] = types.IngressRankingItem{
			Namespace:   item.Namespace,
			IngressName: item.IngressName,
			Value:       item.Value,
			Unit:        item.Unit,
		}
	}
	return result
}

func convertPathRanking(r *pmtypes.PathRanking) types.PathRanking {
	return types.PathRanking{
		TopByQPS:       convertPathRankingItemList(r.TopByQPS),
		TopByErrorRate: convertPathRankingItemList(r.TopByErrorRate),
		TopByLatency:   convertPathRankingItemList(r.TopByLatency),
	}
}

func convertPathRankingItemList(list []pmtypes.PathRankingItem) []types.PathRankingItem {
	result := make([]types.PathRankingItem, len(list))
	for i, item := range list {
		result[i] = types.PathRankingItem{
			Host:  item.Host,
			Path:  item.Path,
			Value: item.Value,
			Unit:  item.Unit,
		}
	}
	return result
}

func convertHostRanking(r *pmtypes.HostRanking) types.HostRanking {
	return types.HostRanking{
		TopByQPS:       convertHostRankingItemList(r.TopByQPS),
		TopByErrorRate: convertHostRankingItemList(r.TopByErrorRate),
		TopByLatency:   convertHostRankingItemList(r.TopByLatency),
	}
}

func convertHostRankingItemList(list []pmtypes.HostRankingItem) []types.HostRankingItem {
	result := make([]types.HostRankingItem, len(list))
	for i, item := range list {
		result[i] = types.HostRankingItem{
			Host:  item.Host,
			Value: item.Value,
			Unit:  item.Unit,
		}
	}
	return result
}
