package flaggermonitor

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

// formatTimePtr 转换时间指针
func formatTimePtr(t *time.Time) int64 {
	if t == nil || t.IsZero() {
		return 0
	}
	return t.Unix()
}

// ==================== Flagger 综合指标转换 ====================

func convertFlaggerMetrics(m *pmtypes.FlaggerMetrics) types.FlaggerMetrics {
	return types.FlaggerMetrics{
		Timestamp:  formatTime(m.Timestamp),
		Canaries:   convertCanaryMetricsList(m.Canaries),
		Statistics: convertFlaggerStatistics(&m.Statistics),
		Controller: convertFlaggerControllerHealth(&m.Controller),
	}
}

// ==================== Canary 指标转换 ====================

func convertCanaryMetrics(m *pmtypes.CanaryMetrics) types.CanaryMetrics {
	return types.CanaryMetrics{
		Name:           m.Name,
		Namespace:      m.Namespace,
		Target:         m.Target,
		Status:         convertCanaryStatus(&m.Status),
		Progress:       convertCanaryProgress(&m.Progress),
		Analysis:       convertCanaryAnalysis(&m.Analysis),
		Comparison:     convertCanaryComparison(&m.Comparison),
		Trend:          convertCanaryDataPoints(m.Trend),
		Duration:       convertCanaryDuration(&m.Duration),
		LastUpdateTime: formatTime(m.LastUpdateTime),
	}
}

func convertCanaryMetricsList(list []pmtypes.CanaryMetrics) []types.CanaryMetrics {
	result := make([]types.CanaryMetrics, len(list))
	for i, m := range list {
		result[i] = convertCanaryMetrics(&m)
	}
	return result
}

func convertCanaryStatus(s *pmtypes.CanaryStatus) types.CanaryStatus {
	return types.CanaryStatus{
		Phase:          s.Phase,
		PhaseCode:      s.PhaseCode,
		LastTransition: formatTime(s.LastTransition),
	}
}

func convertCanaryProgress(p *pmtypes.CanaryProgress) types.CanaryProgress {
	return types.CanaryProgress{
		CurrentWeight:   p.CurrentWeight,
		TargetWeight:    p.TargetWeight,
		Iteration:       p.Iteration,
		ProgressPercent: p.ProgressPercent,
		Trend:           convertCanaryProgressDataPoints(p.Trend),
	}
}

func convertCanaryProgressDataPoints(points []pmtypes.CanaryProgressDataPoint) []types.CanaryProgressDataPoint {
	result := make([]types.CanaryProgressDataPoint, len(points))
	for i, p := range points {
		result[i] = types.CanaryProgressDataPoint{
			Timestamp: formatTime(p.Timestamp),
			Weight:    p.Weight,
			Iteration: p.Iteration,
		}
	}
	return result
}

func convertCanaryAnalysis(a *pmtypes.CanaryAnalysis) types.CanaryAnalysis {
	return types.CanaryAnalysis{
		MetricChecks:     convertMetricCheckResultList(a.MetricChecks),
		CheckSuccessRate: a.CheckSuccessRate,
		TotalChecks:      a.TotalChecks,
		FailedChecks:     a.FailedChecks,
		Trend:            convertCanaryAnalysisDataPoints(a.Trend),
	}
}

func convertMetricCheckResultList(list []pmtypes.MetricCheckResult) []types.MetricCheckResult {
	result := make([]types.MetricCheckResult, len(list))
	for i, m := range list {
		result[i] = types.MetricCheckResult{
			MetricName:   m.MetricName,
			ChecksPassed: m.ChecksPassed,
			ChecksFailed: m.ChecksFailed,
			SuccessRate:  m.SuccessRate,
			LastCheck:    formatTime(m.LastCheck),
		}
	}
	return result
}

func convertCanaryAnalysisDataPoints(points []pmtypes.CanaryAnalysisDataPoint) []types.CanaryAnalysisDataPoint {
	result := make([]types.CanaryAnalysisDataPoint, len(points))
	for i, p := range points {
		result[i] = types.CanaryAnalysisDataPoint{
			Timestamp:        formatTime(p.Timestamp),
			CheckSuccessRate: p.CheckSuccessRate,
			FailedChecks:     p.FailedChecks,
		}
	}
	return result
}

func convertCanaryComparison(c *pmtypes.CanaryComparison) types.CanaryComparison {
	return types.CanaryComparison{
		CanaryMetrics:  convertWorkloadMetrics(&c.CanaryMetrics),
		PrimaryMetrics: convertWorkloadMetrics(&c.PrimaryMetrics),
		Trend:          convertCanaryComparisonDataPoints(c.Trend),
	}
}

func convertWorkloadMetrics(w *pmtypes.WorkloadMetrics) types.WorkloadMetrics {
	return types.WorkloadMetrics{
		SuccessRate: w.SuccessRate,
		ErrorRate:   w.ErrorRate,
		P50Latency:  w.P50Latency,
		P95Latency:  w.P95Latency,
		P99Latency:  w.P99Latency,
		RequestRate: w.RequestRate,
	}
}

func convertCanaryComparisonDataPoints(points []pmtypes.CanaryComparisonDataPoint) []types.CanaryComparisonDataPoint {
	result := make([]types.CanaryComparisonDataPoint, len(points))
	for i, p := range points {
		result[i] = types.CanaryComparisonDataPoint{
			Timestamp:          formatTime(p.Timestamp),
			CanarySuccessRate:  p.CanarySuccessRate,
			PrimarySuccessRate: p.PrimarySuccessRate,
			CanaryP95Latency:   p.CanaryP95Latency,
			PrimaryP95Latency:  p.PrimaryP95Latency,
			CanaryErrorRate:    p.CanaryErrorRate,
			PrimaryErrorRate:   p.PrimaryErrorRate,
		}
	}
	return result
}

func convertCanaryDuration(d *pmtypes.CanaryDuration) types.CanaryDuration {
	return types.CanaryDuration{
		StartTime:       formatTime(d.StartTime),
		EndTime:         formatTimePtr(d.EndTime),
		DurationSeconds: d.DurationSeconds,
	}
}

func convertCanaryDataPoints(points []pmtypes.CanaryDataPoint) []types.CanaryDataPoint {
	result := make([]types.CanaryDataPoint, len(points))
	for i, p := range points {
		result[i] = types.CanaryDataPoint{
			Timestamp: formatTime(p.Timestamp),
			Phase:     p.Phase,
			PhaseCode: p.PhaseCode,
			Weight:    p.Weight,
			Iteration: p.Iteration,
		}
	}
	return result
}

// ==================== Flagger 统计转换 ====================

func convertFlaggerStatistics(s *pmtypes.FlaggerStatistics) types.FlaggerStatistics {
	return types.FlaggerStatistics{
		TotalCanaries:   s.TotalCanaries,
		ActiveCanaries:  s.ActiveCanaries,
		WaitingCanaries: s.WaitingCanaries,
		SuccessCanaries: s.SuccessCanaries,
		FailedCanaries:  s.FailedCanaries,
		SuccessRate:     s.SuccessRate,
		FailureRate:     s.FailureRate,
		Duration:        convertFlaggerDurationStatistics(&s.Duration),
		WebhookStats:    convertFlaggerWebhookStatistics(&s.WebhookStats),
		Trend:           convertFlaggerStatisticsDataPoints(s.Trend),
	}
}

func convertFlaggerDurationStatistics(d *pmtypes.FlaggerDurationStatistics) types.FlaggerDurationStatistics {
	return types.FlaggerDurationStatistics{
		AvgDurationSeconds: d.AvgDurationSeconds,
		MinDurationSeconds: d.MinDurationSeconds,
		MaxDurationSeconds: d.MaxDurationSeconds,
		P50DurationSeconds: d.P50DurationSeconds,
		P95DurationSeconds: d.P95DurationSeconds,
		P99DurationSeconds: d.P99DurationSeconds,
		Trend:              convertFlaggerDurationDataPoints(d.Trend),
	}
}

func convertFlaggerDurationDataPoints(points []pmtypes.FlaggerDurationDataPoint) []types.FlaggerDurationDataPoint {
	result := make([]types.FlaggerDurationDataPoint, len(points))
	for i, p := range points {
		result[i] = types.FlaggerDurationDataPoint{
			Timestamp:          formatTime(p.Timestamp),
			AvgDurationSeconds: p.AvgDurationSeconds,
			P95DurationSeconds: p.P95DurationSeconds,
		}
	}
	return result
}

func convertFlaggerWebhookStatistics(w *pmtypes.FlaggerWebhookStatistics) types.FlaggerWebhookStatistics {
	return types.FlaggerWebhookStatistics{
		TotalRequests:   w.TotalRequests,
		SuccessRequests: w.SuccessRequests,
		FailedRequests:  w.FailedRequests,
		SuccessRate:     w.SuccessRate,
		FailureRate:     w.FailureRate,
		Trend:           convertFlaggerWebhookDataPoints(w.Trend),
	}
}

func convertFlaggerWebhookDataPoints(points []pmtypes.FlaggerWebhookDataPoint) []types.FlaggerWebhookDataPoint {
	result := make([]types.FlaggerWebhookDataPoint, len(points))
	for i, p := range points {
		result[i] = types.FlaggerWebhookDataPoint{
			Timestamp:     formatTime(p.Timestamp),
			TotalRequests: p.TotalRequests,
			SuccessRate:   p.SuccessRate,
		}
	}
	return result
}

func convertFlaggerStatisticsDataPoints(points []pmtypes.FlaggerStatisticsDataPoint) []types.FlaggerStatisticsDataPoint {
	result := make([]types.FlaggerStatisticsDataPoint, len(points))
	for i, p := range points {
		result[i] = types.FlaggerStatisticsDataPoint{
			Timestamp:       formatTime(p.Timestamp),
			TotalCanaries:   p.TotalCanaries,
			ActiveCanaries:  p.ActiveCanaries,
			SuccessCanaries: p.SuccessCanaries,
			FailedCanaries:  p.FailedCanaries,
			SuccessRate:     p.SuccessRate,
		}
	}
	return result
}

// ==================== Flagger Controller 健康转换 ====================

func convertFlaggerControllerHealth(h *pmtypes.FlaggerControllerHealth) types.FlaggerControllerHealth {
	return types.FlaggerControllerHealth{
		ReconcileTotal:     h.ReconcileTotal,
		ReconcileErrors:    h.ReconcileErrors,
		ReconcileErrorRate: h.ReconcileErrorRate,
		AvgReconcileTime:   h.AvgReconcileTime,
		P95ReconcileTime:   h.P95ReconcileTime,
		Trend:              convertFlaggerControllerDataPoints(h.Trend),
	}
}

func convertFlaggerControllerDataPoints(points []pmtypes.FlaggerControllerDataPoint) []types.FlaggerControllerDataPoint {
	result := make([]types.FlaggerControllerDataPoint, len(points))
	for i, p := range points {
		result[i] = types.FlaggerControllerDataPoint{
			Timestamp:          formatTime(p.Timestamp),
			ReconcileTotal:     p.ReconcileTotal,
			ReconcileErrorRate: p.ReconcileErrorRate,
			AvgReconcileTime:   p.AvgReconcileTime,
		}
	}
	return result
}

// ==================== Canary 列表和摘要转换 ====================

func convertCanaryListMetrics(m *pmtypes.CanaryListMetrics) types.CanaryListMetrics {
	return types.CanaryListMetrics{
		Timestamp: formatTime(m.Timestamp),
		Canaries:  convertCanaryBriefList(m.Canaries),
		Summary:   convertCanaryListSummary(&m.Summary),
	}
}

func convertCanaryBriefList(list []pmtypes.CanaryBrief) []types.CanaryBrief {
	result := make([]types.CanaryBrief, len(list))
	for i, c := range list {
		result[i] = types.CanaryBrief{
			Name:          c.Name,
			Namespace:     c.Namespace,
			Target:        c.Target,
			Phase:         c.Phase,
			CurrentWeight: c.CurrentWeight,
			Iteration:     c.Iteration,
			LastUpdate:    formatTime(c.LastUpdate),
		}
	}
	return result
}

func convertCanaryListSummary(s *pmtypes.CanaryListSummary) types.CanaryListSummary {
	return types.CanaryListSummary{
		Total:       s.Total,
		Active:      s.Active,
		Waiting:     s.Waiting,
		Succeeded:   s.Succeeded,
		Failed:      s.Failed,
		Initialized: s.Initialized,
	}
}

// ==================== Namespace 级别 Canary 统计转换 ====================

func convertNamespaceCanaryMetrics(m *pmtypes.NamespaceCanaryMetrics) types.NamespaceCanaryMetrics {
	return types.NamespaceCanaryMetrics{
		Namespace:  m.Namespace,
		Timestamp:  formatTime(m.Timestamp),
		Canaries:   convertCanaryBriefList(m.Canaries),
		Statistics: convertNamespaceCanaryStatistics(&m.Statistics),
	}
}

func convertNamespaceCanaryStatistics(s *pmtypes.NamespaceCanaryStatistics) types.NamespaceCanaryStatistics {
	return types.NamespaceCanaryStatistics{
		Total:       s.Total,
		Active:      s.Active,
		SuccessRate: s.SuccessRate,
		AvgDuration: s.AvgDuration,
	}
}

// ==================== Canary 排行转换 ====================

func convertCanaryRanking(r *pmtypes.CanaryRanking) types.CanaryRanking {
	return types.CanaryRanking{
		TopByDuration:     convertCanaryRankingItemList(r.TopByDuration),
		TopByIterations:   convertCanaryRankingItemList(r.TopByIterations),
		TopByFailedChecks: convertCanaryRankingItemList(r.TopByFailedChecks),
		RecentFailed:      convertCanaryBriefList(r.RecentFailed),
		RecentSucceeded:   convertCanaryBriefList(r.RecentSucceeded),
	}
}

func convertCanaryRankingItemList(list []pmtypes.CanaryRankingItem) []types.CanaryRankingItem {
	result := make([]types.CanaryRankingItem, len(list))
	for i, item := range list {
		result[i] = types.CanaryRankingItem{
			Namespace: item.Namespace,
			Name:      item.Name,
			Value:     item.Value,
			Unit:      item.Unit,
		}
	}
	return result
}

// ==================== 历史记录转换 ====================

func convertCanaryHistory(h *pmtypes.CanaryHistory) types.CanaryHistory {
	return types.CanaryHistory{
		Namespace: h.Namespace,
		Name:      h.Name,
		Records:   convertCanaryHistoryRecordList(h.Records),
	}
}

func convertCanaryHistoryRecordList(list []pmtypes.CanaryHistoryRecord) []types.CanaryHistoryRecord {
	result := make([]types.CanaryHistoryRecord, len(list))
	for i, r := range list {
		result[i] = types.CanaryHistoryRecord{
			StartTime:       formatTime(r.StartTime),
			EndTime:         formatTime(r.EndTime),
			DurationSeconds: r.DurationSeconds,
			FinalPhase:      r.FinalPhase,
			MaxWeight:       r.MaxWeight,
			Iterations:      r.Iterations,
			FailedChecks:    r.FailedChecks,
		}
	}
	return result
}

// ==================== 实时监控转换 ====================

func convertCanaryRealtimeMetrics(m *pmtypes.CanaryRealtimeMetrics) types.CanaryRealtimeMetrics {
	return types.CanaryRealtimeMetrics{
		Namespace:     m.Namespace,
		Name:          m.Name,
		Timestamp:     formatTime(m.Timestamp),
		CurrentStatus: convertCanaryStatus(&m.CurrentStatus),
		CurrentWeight: m.CurrentWeight,
		Comparison:    convertCanaryComparison(&m.Comparison),
		RecentChecks:  convertMetricCheckResultList(m.RecentChecks),
	}
}
