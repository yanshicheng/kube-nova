package operator

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/yanshicheng/kube-nova/common/prometheusmanager/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type FlaggerOperator struct {
	log logx.Logger
	ctx context.Context
	*BaseOperator
}

func NewFlaggerOperator(ctx context.Context, base *BaseOperator) types.FlaggerOperator {
	return &FlaggerOperator{
		log:          logx.WithContext(ctx),
		ctx:          ctx,
		BaseOperator: base,
	}
}

// ==================== 综合查询 ====================

// GetFlaggerMetrics 获取 Flagger 综合指标
func (f *FlaggerOperator) GetFlaggerMetrics(timeRange *types.TimeRange) (*types.FlaggerMetrics, error) {
	f.log.Infof(" 查询 Flagger 综合指标")

	metrics := &types.FlaggerMetrics{
		Timestamp: time.Now(),
		Canaries:  []types.CanaryMetrics{},
	}

	// 获取统计信息
	statistics, err := f.GetFlaggerStatistics(timeRange)
	if err == nil {
		metrics.Statistics = *statistics
	}

	// 获取 Controller 健康状态
	controller, err := f.GetFlaggerControllerHealth(timeRange)
	if err == nil {
		metrics.Controller = *controller
	}

	// 获取所有 Canary 列表
	canaryList, err := f.ListAllCanaries()
	if err == nil {
		for _, brief := range canaryList.Canaries {
			canary, err := f.GetCanaryMetrics(brief.Namespace, brief.Name, timeRange)
			if err == nil {
				metrics.Canaries = append(metrics.Canaries, *canary)
			}
		}
	}

	return metrics, nil
}

// ==================== Canary 查询 ====================

// GetCanaryMetrics 获取单个 Canary 指标
func (f *FlaggerOperator) GetCanaryMetrics(namespace, name string, timeRange *types.TimeRange) (*types.CanaryMetrics, error) {
	f.log.Infof(" 查询 Canary: namespace=%s, name=%s", namespace, name)

	metrics := &types.CanaryMetrics{
		Name:           name,
		Namespace:      namespace,
		LastUpdateTime: time.Now(),
		Trend:          []types.CanaryDataPoint{},
	}

	// 获取状态
	status, err := f.GetCanaryStatus(namespace, name)
	if err == nil {
		metrics.Status = *status
	}

	// 获取进度
	progress, err := f.GetCanaryProgress(namespace, name, timeRange)
	if err == nil {
		metrics.Progress = *progress
	}

	// 获取分析结果
	analysis, err := f.GetCanaryAnalysis(namespace, name, timeRange)
	if err == nil {
		metrics.Analysis = *analysis
	}

	// 获取对比数据
	comparison, err := f.GetCanaryComparison(namespace, name, timeRange)
	if err == nil {
		metrics.Comparison = *comparison
	}

	// 获取持续时间
	duration, err := f.GetCanaryDuration(namespace, name)
	if err == nil {
		metrics.Duration = *duration
	}

	// 获取 Target
	targetQuery := fmt.Sprintf(`flagger_canary_info{namespace="%s",name="%s"}`, namespace, name)
	targetResult, _ := f.query(targetQuery, nil)
	if len(targetResult) > 0 {
		if target, ok := targetResult[0].Metric["target"]; ok {
			metrics.Target = target
		}
	}

	// 趋势数据
	if timeRange != nil && !timeRange.Start.IsZero() && !timeRange.End.IsZero() {
		step := timeRange.Step
		if step == "" {
			step = f.calculateStep(timeRange.Start, timeRange.End)
		}

		phaseQuery := fmt.Sprintf(`flagger_canary_status{namespace="%s",name="%s"}`, namespace, name)
		trendResult, err := f.queryRange(phaseQuery, timeRange.Start, timeRange.End, step)
		if err == nil && len(trendResult) > 0 {
			metrics.Trend = f.parseCanaryTrend(trendResult[0].Values)
		}
	}

	return metrics, nil
}

// GetCanaryStatus 获取 Canary 状态
func (f *FlaggerOperator) GetCanaryStatus(namespace, name string) (*types.CanaryStatus, error) {
	status := &types.CanaryStatus{
		LastTransition: time.Now(),
	}

	phaseQuery := fmt.Sprintf(`flagger_canary_status{namespace="%s",name="%s"}`, namespace, name)
	phaseResult, _ := f.query(phaseQuery, nil)
	if len(phaseResult) > 0 {
		status.PhaseCode = int(phaseResult[0].Value)
		status.Phase = f.phaseCodeToString(status.PhaseCode)
	}

	return status, nil
}

// GetCanaryProgress 获取 Canary 进度
func (f *FlaggerOperator) GetCanaryProgress(namespace, name string, timeRange *types.TimeRange) (*types.CanaryProgress, error) {
	progress := &types.CanaryProgress{
		Trend: []types.CanaryProgressDataPoint{},
	}

	// 当前权重
	weightQuery := fmt.Sprintf(`flagger_canary_weight{namespace="%s",name="%s"}`, namespace, name)
	weightResult, _ := f.query(weightQuery, nil)
	if len(weightResult) > 0 {
		progress.CurrentWeight = weightResult[0].Value
	}

	// 目标权重（一般是 100）
	progress.TargetWeight = 100

	// 迭代次数
	iterationQuery := fmt.Sprintf(`flagger_canary_iteration{namespace="%s",name="%s"}`, namespace, name)
	iterationResult, _ := f.query(iterationQuery, nil)
	if len(iterationResult) > 0 {
		progress.Iteration = int64(iterationResult[0].Value)
	}

	// 计算进度百分比
	if progress.TargetWeight > 0 {
		progress.ProgressPercent = (progress.CurrentWeight / progress.TargetWeight) * 100
	}

	// 趋势数据
	if timeRange != nil && !timeRange.Start.IsZero() && !timeRange.End.IsZero() {
		step := timeRange.Step
		if step == "" {
			step = f.calculateStep(timeRange.Start, timeRange.End)
		}

		trendResult, err := f.queryRange(weightQuery, timeRange.Start, timeRange.End, step)
		if err == nil && len(trendResult) > 0 {
			progress.Trend = f.parseProgressTrend(trendResult[0].Values)
		}
	}

	return progress, nil
}

// GetCanaryAnalysis 获取 Canary 分析结果
func (f *FlaggerOperator) GetCanaryAnalysis(namespace, name string, timeRange *types.TimeRange) (*types.CanaryAnalysis, error) {
	analysis := &types.CanaryAnalysis{
		MetricChecks: []types.MetricCheckResult{},
		Trend:        []types.CanaryAnalysisDataPoint{},
	}

	// 总检查次数
	totalQuery := fmt.Sprintf(`flagger_canary_total{namespace="%s",name="%s"}`, namespace, name)
	totalResult, _ := f.query(totalQuery, nil)
	if len(totalResult) > 0 {
		analysis.TotalChecks = int64(totalResult[0].Value)
	}

	// 失败检查次数
	failedQuery := fmt.Sprintf(`flagger_canary_failed{namespace="%s",name="%s"}`, namespace, name)
	failedResult, _ := f.query(failedQuery, nil)
	if len(failedResult) > 0 {
		analysis.FailedChecks = int64(failedResult[0].Value)
	}

	// 计算成功率
	if analysis.TotalChecks > 0 {
		analysis.CheckSuccessRate = float64(analysis.TotalChecks-analysis.FailedChecks) / float64(analysis.TotalChecks) * 100
	}

	// 按指标的检查结果
	metricQuery := fmt.Sprintf(`flagger_canary_metric_check{namespace="%s",name="%s"}`, namespace, name)
	metricResult, _ := f.query(metricQuery, nil)
	for _, r := range metricResult {
		if metricName, ok := r.Metric["metric"]; ok {
			result := types.MetricCheckResult{
				MetricName: metricName,
				LastCheck:  r.Time,
			}

			// 这里需要根据实际的 Flagger 指标来解析
			// 假设值 > 0 表示检查通过
			if r.Value > 0 {
				result.ChecksPassed = int64(r.Value)
			} else {
				result.ChecksFailed = int64(-r.Value)
			}

			if result.ChecksPassed+result.ChecksFailed > 0 {
				result.SuccessRate = float64(result.ChecksPassed) / float64(result.ChecksPassed+result.ChecksFailed) * 100
			}

			analysis.MetricChecks = append(analysis.MetricChecks, result)
		}
	}

	// 趋势数据
	if timeRange != nil && !timeRange.Start.IsZero() && !timeRange.End.IsZero() {
		step := timeRange.Step
		if step == "" {
			step = f.calculateStep(timeRange.Start, timeRange.End)
		}

		trendQuery := fmt.Sprintf(`flagger_canary_failed{namespace="%s",name="%s"}`, namespace, name)
		trendResult, err := f.queryRange(trendQuery, timeRange.Start, timeRange.End, step)
		if err == nil && len(trendResult) > 0 {
			analysis.Trend = f.parseAnalysisTrend(trendResult[0].Values, analysis.TotalChecks)
		}
	}

	return analysis, nil
}

// GetCanaryComparison 获取 Canary vs Primary 对比
func (f *FlaggerOperator) GetCanaryComparison(namespace, name string, timeRange *types.TimeRange) (*types.CanaryComparison, error) {
	comparison := &types.CanaryComparison{
		Trend: []types.CanaryComparisonDataPoint{},
	}

	window := f.calculateRateWindow(timeRange)

	// Canary 成功率
	canarySuccessQuery := fmt.Sprintf(`sum(rate(istio_requests_total{namespace="%s",destination_app=~"%s-canary",response_code!~"5.."}[%s])) / sum(rate(istio_requests_total{namespace="%s",destination_app=~"%s-canary"}[%s]))`,
		namespace, name, window, namespace, name, window)
	canarySuccessResult, _ := f.query(canarySuccessQuery, nil)
	if len(canarySuccessResult) > 0 {
		comparison.CanaryMetrics.SuccessRate = canarySuccessResult[0].Value * 100
		comparison.CanaryMetrics.ErrorRate = (1 - canarySuccessResult[0].Value) * 100
	}

	// Primary 成功率
	primarySuccessQuery := fmt.Sprintf(`sum(rate(istio_requests_total{namespace="%s",destination_app=~"%s-primary",response_code!~"5.."}[%s])) / sum(rate(istio_requests_total{namespace="%s",destination_app=~"%s-primary"}[%s]))`,
		namespace, name, window, namespace, name, window)
	primarySuccessResult, _ := f.query(primarySuccessQuery, nil)
	if len(primarySuccessResult) > 0 {
		comparison.PrimaryMetrics.SuccessRate = primarySuccessResult[0].Value * 100
		comparison.PrimaryMetrics.ErrorRate = (1 - primarySuccessResult[0].Value) * 100
	}

	// Canary P95 延迟
	canaryP95Query := fmt.Sprintf(`histogram_quantile(0.95, sum(rate(istio_request_duration_milliseconds_bucket{namespace="%s",destination_app=~"%s-canary"}[%s])) by (le))`,
		namespace, name, window)
	canaryP95Result, _ := f.query(canaryP95Query, nil)
	if len(canaryP95Result) > 0 {
		comparison.CanaryMetrics.P95Latency = canaryP95Result[0].Value
	}

	// Primary P95 延迟
	primaryP95Query := fmt.Sprintf(`histogram_quantile(0.95, sum(rate(istio_request_duration_milliseconds_bucket{namespace="%s",destination_app=~"%s-primary"}[%s])) by (le))`,
		namespace, name, window)
	primaryP95Result, _ := f.query(primaryP95Query, nil)
	if len(primaryP95Result) > 0 {
		comparison.PrimaryMetrics.P95Latency = primaryP95Result[0].Value
	}

	// Canary QPS
	canaryQPSQuery := fmt.Sprintf(`sum(rate(istio_requests_total{namespace="%s",destination_app=~"%s-canary"}[%s]))`,
		namespace, name, window)
	canaryQPSResult, _ := f.query(canaryQPSQuery, nil)
	if len(canaryQPSResult) > 0 {
		comparison.CanaryMetrics.RequestRate = canaryQPSResult[0].Value
	}

	// Primary QPS
	primaryQPSQuery := fmt.Sprintf(`sum(rate(istio_requests_total{namespace="%s",destination_app=~"%s-primary"}[%s]))`,
		namespace, name, window)
	primaryQPSResult, _ := f.query(primaryQPSQuery, nil)
	if len(primaryQPSResult) > 0 {
		comparison.PrimaryMetrics.RequestRate = primaryQPSResult[0].Value
	}

	// 趋势数据
	if timeRange != nil && !timeRange.Start.IsZero() && !timeRange.End.IsZero() {
		step := timeRange.Step
		if step == "" {
			step = f.calculateStep(timeRange.Start, timeRange.End)
		}

		// 对比趋势（成功率、延迟、错误率）
		canarySuccessTrend, _ := f.queryRange(canarySuccessQuery, timeRange.Start, timeRange.End, step)
		primarySuccessTrend, _ := f.queryRange(primarySuccessQuery, timeRange.Start, timeRange.End, step)
		canaryP95Trend, _ := f.queryRange(canaryP95Query, timeRange.Start, timeRange.End, step)
		primaryP95Trend, _ := f.queryRange(primaryP95Query, timeRange.Start, timeRange.End, step)

		if len(canarySuccessTrend) > 0 && len(primarySuccessTrend) > 0 {
			comparison.Trend = f.parseComparisonTrend(
				canarySuccessTrend[0].Values,
				primarySuccessTrend[0].Values,
				canaryP95Trend[0].Values,
				primaryP95Trend[0].Values,
			)
		}
	}

	return comparison, nil
}

// GetCanaryDuration 获取 Canary 持续时间
func (f *FlaggerOperator) GetCanaryDuration(namespace, name string) (*types.CanaryDuration, error) {
	duration := &types.CanaryDuration{}

	// 查询 Canary 开始时间
	startQuery := fmt.Sprintf(`flagger_canary_start_time{namespace="%s",name="%s"}`, namespace, name)
	startResult, _ := f.query(startQuery, nil)
	if len(startResult) > 0 {
		duration.StartTime = time.Unix(int64(startResult[0].Value), 0)

		// 如果 Canary 还在进行中
		status, _ := f.GetCanaryStatus(namespace, name)
		if status != nil && (status.Phase == "Progressing" || status.Phase == "Waiting") {
			duration.DurationSeconds = time.Since(duration.StartTime).Seconds()
		} else {
			// 如果已结束，查询结束时间
			endQuery := fmt.Sprintf(`flagger_canary_end_time{namespace="%s",name="%s"}`, namespace, name)
			endResult, _ := f.query(endQuery, nil)
			if len(endResult) > 0 {
				endTime := time.Unix(int64(endResult[0].Value), 0)
				duration.EndTime = &endTime
				duration.DurationSeconds = endTime.Sub(duration.StartTime).Seconds()
			}
		}
	}

	return duration, nil
}

// ==================== Canary 列表 ====================

// ListCanaries 列出指定 namespace 的 Canary
func (f *FlaggerOperator) ListCanaries(namespace string) (*types.CanaryListMetrics, error) {
	list := &types.CanaryListMetrics{
		Timestamp: time.Now(),
		Canaries:  []types.CanaryBrief{},
		Summary:   types.CanaryListSummary{},
	}

	// 查询所有 Canary
	query := fmt.Sprintf(`flagger_canary_info{namespace="%s"}`, namespace)
	result, _ := f.query(query, nil)

	for _, r := range result {
		if name, ok := r.Metric["name"]; ok {
			brief := types.CanaryBrief{
				Name:       name,
				Namespace:  namespace,
				LastUpdate: r.Time,
			}

			// 获取其他信息
			if target, ok := r.Metric["target"]; ok {
				brief.Target = target
			}

			// 获取状态
			status, _ := f.GetCanaryStatus(namespace, name)
			if status != nil {
				brief.Phase = status.Phase
			}

			// 获取权重和迭代
			progress, _ := f.GetCanaryProgress(namespace, name, nil)
			if progress != nil {
				brief.CurrentWeight = progress.CurrentWeight
				brief.Iteration = progress.Iteration
			}

			list.Canaries = append(list.Canaries, brief)

			// 更新汇总
			list.Summary.Total++
			switch brief.Phase {
			case "Progressing":
				list.Summary.Active++
			case "Waiting":
				list.Summary.Waiting++
			case "Succeeded":
				list.Summary.Succeeded++
			case "Failed":
				list.Summary.Failed++
			case "Initialized":
				list.Summary.Initialized++
			}
		}
	}

	return list, nil
}

// ListAllCanaries 列出所有 Canary
func (f *FlaggerOperator) ListAllCanaries() (*types.CanaryListMetrics, error) {
	list := &types.CanaryListMetrics{
		Timestamp: time.Now(),
		Canaries:  []types.CanaryBrief{},
		Summary:   types.CanaryListSummary{},
	}

	// 查询所有 Canary
	query := `flagger_canary_info`
	result, _ := f.query(query, nil)

	for _, r := range result {
		namespace, _ := r.Metric["namespace"]
		name, _ := r.Metric["name"]

		brief := types.CanaryBrief{
			Name:       name,
			Namespace:  namespace,
			LastUpdate: r.Time,
		}

		if target, ok := r.Metric["target"]; ok {
			brief.Target = target
		}

		status, _ := f.GetCanaryStatus(namespace, name)
		if status != nil {
			brief.Phase = status.Phase
		}

		progress, _ := f.GetCanaryProgress(namespace, name, nil)
		if progress != nil {
			brief.CurrentWeight = progress.CurrentWeight
			brief.Iteration = progress.Iteration
		}

		list.Canaries = append(list.Canaries, brief)

		list.Summary.Total++
		switch brief.Phase {
		case "Progressing":
			list.Summary.Active++
		case "Waiting":
			list.Summary.Waiting++
		case "Succeeded":
			list.Summary.Succeeded++
		case "Failed":
			list.Summary.Failed++
		case "Initialized":
			list.Summary.Initialized++
		}
	}

	return list, nil
}

// ==================== 统计查询 ====================

// GetFlaggerStatistics 获取 Flagger 统计
func (f *FlaggerOperator) GetFlaggerStatistics(timeRange *types.TimeRange) (*types.FlaggerStatistics, error) {
	stats := &types.FlaggerStatistics{
		Trend: []types.FlaggerStatisticsDataPoint{},
	}

	// 总 Canary 数
	totalQuery := `count(flagger_canary_info)`
	totalResult, _ := f.query(totalQuery, nil)
	if len(totalResult) > 0 {
		stats.TotalCanaries = int64(totalResult[0].Value)
	}

	// Active Canaries (Progressing)
	activeQuery := `count(flagger_canary_status{status="Progressing"})`
	activeResult, _ := f.query(activeQuery, nil)
	if len(activeResult) > 0 {
		stats.ActiveCanaries = int64(activeResult[0].Value)
	}

	// Waiting Canaries
	waitingQuery := `count(flagger_canary_status{status="Waiting"})`
	waitingResult, _ := f.query(waitingQuery, nil)
	if len(waitingResult) > 0 {
		stats.WaitingCanaries = int64(waitingResult[0].Value)
	}

	// Success Canaries
	successQuery := `count(flagger_canary_status{status="Succeeded"})`
	successResult, _ := f.query(successQuery, nil)
	if len(successResult) > 0 {
		stats.SuccessCanaries = int64(successResult[0].Value)
	}

	// Failed Canaries
	failedQuery := `count(flagger_canary_status{status="Failed"})`
	failedResult, _ := f.query(failedQuery, nil)
	if len(failedResult) > 0 {
		stats.FailedCanaries = int64(failedResult[0].Value)
	}

	// 计算成功率和失败率
	total := stats.SuccessCanaries + stats.FailedCanaries
	if total > 0 {
		stats.SuccessRate = float64(stats.SuccessCanaries) / float64(total) * 100
		stats.FailureRate = float64(stats.FailedCanaries) / float64(total) * 100
	}

	// 持续时间统计
	durationStats, _ := f.GetFlaggerDurationStatistics(timeRange)
	if durationStats != nil {
		stats.Duration = *durationStats
	}

	// Webhook 统计
	webhookStats, _ := f.GetFlaggerWebhookStatistics(timeRange)
	if webhookStats != nil {
		stats.WebhookStats = *webhookStats
	}

	// 趋势数据
	if timeRange != nil && !timeRange.Start.IsZero() && !timeRange.End.IsZero() {
		step := timeRange.Step
		if step == "" {
			step = f.calculateStep(timeRange.Start, timeRange.End)
		}

		trendQuery := `count(flagger_canary_info)`
		trendResult, err := f.queryRange(trendQuery, timeRange.Start, timeRange.End, step)
		if err == nil && len(trendResult) > 0 {
			stats.Trend = f.parseStatisticsTrend(trendResult[0].Values)
		}
	}

	return stats, nil
}

// GetFlaggerDurationStatistics 获取持续时间统计
func (f *FlaggerOperator) GetFlaggerDurationStatistics(timeRange *types.TimeRange) (*types.FlaggerDurationStatistics, error) {
	stats := &types.FlaggerDurationStatistics{
		Trend: []types.FlaggerDurationDataPoint{},
	}

	window := f.calculateRateWindow(timeRange)

	// 平均持续时间
	avgQuery := fmt.Sprintf(`avg(flagger_canary_duration_seconds)`)
	avgResult, _ := f.query(avgQuery, nil)
	if len(avgResult) > 0 {
		stats.AvgDurationSeconds = avgResult[0].Value
	}

	// 最小持续时间
	minQuery := `min(flagger_canary_duration_seconds)`
	minResult, _ := f.query(minQuery, nil)
	if len(minResult) > 0 {
		stats.MinDurationSeconds = minResult[0].Value
	}

	// 最大持续时间
	maxQuery := `max(flagger_canary_duration_seconds)`
	maxResult, _ := f.query(maxQuery, nil)
	if len(maxResult) > 0 {
		stats.MaxDurationSeconds = maxResult[0].Value
	}

	// P50/P95/P99
	p50Query := fmt.Sprintf(`histogram_quantile(0.50, sum(rate(flagger_canary_duration_seconds_bucket[%s])) by (le))`, window)
	p50Result, _ := f.query(p50Query, nil)
	if len(p50Result) > 0 {
		stats.P50DurationSeconds = p50Result[0].Value
	}

	p95Query := fmt.Sprintf(`histogram_quantile(0.95, sum(rate(flagger_canary_duration_seconds_bucket[%s])) by (le))`, window)
	p95Result, _ := f.query(p95Query, nil)
	if len(p95Result) > 0 {
		stats.P95DurationSeconds = p95Result[0].Value
	}

	p99Query := fmt.Sprintf(`histogram_quantile(0.99, sum(rate(flagger_canary_duration_seconds_bucket[%s])) by (le))`, window)
	p99Result, _ := f.query(p99Query, nil)
	if len(p99Result) > 0 {
		stats.P99DurationSeconds = p99Result[0].Value
	}

	return stats, nil
}

// GetFlaggerWebhookStatistics 获取 Webhook 统计
func (f *FlaggerOperator) GetFlaggerWebhookStatistics(timeRange *types.TimeRange) (*types.FlaggerWebhookStatistics, error) {
	stats := &types.FlaggerWebhookStatistics{
		Trend: []types.FlaggerWebhookDataPoint{},
	}

	window := f.calculateRateWindow(timeRange)

	// 总请求数
	totalQuery := fmt.Sprintf(`sum(rate(flagger_webhook_requests_total[%s]))`, window)
	totalResult, _ := f.query(totalQuery, nil)
	if len(totalResult) > 0 {
		stats.TotalRequests = int64(totalResult[0].Value)
	}

	// 成功请求数
	successQuery := fmt.Sprintf(`sum(rate(flagger_webhook_requests_total{status="success"}[%s]))`, window)
	successResult, _ := f.query(successQuery, nil)
	if len(successResult) > 0 {
		stats.SuccessRequests = int64(successResult[0].Value)
	}

	// 失败请求数
	failedQuery := fmt.Sprintf(`sum(rate(flagger_webhook_requests_total{status="failed"}[%s]))`, window)
	failedResult, _ := f.query(failedQuery, nil)
	if len(failedResult) > 0 {
		stats.FailedRequests = int64(failedResult[0].Value)
	}

	// 计算成功率和失败率
	if stats.TotalRequests > 0 {
		stats.SuccessRate = float64(stats.SuccessRequests) / float64(stats.TotalRequests) * 100
		stats.FailureRate = float64(stats.FailedRequests) / float64(stats.TotalRequests) * 100
	}

	return stats, nil
}

// GetFlaggerControllerHealth 获取 Flagger Controller 健康
func (f *FlaggerOperator) GetFlaggerControllerHealth(timeRange *types.TimeRange) (*types.FlaggerControllerHealth, error) {
	health := &types.FlaggerControllerHealth{
		Trend: []types.FlaggerControllerDataPoint{},
	}

	window := f.calculateRateWindow(timeRange)

	// Reconcile 总数
	totalQuery := fmt.Sprintf(`sum(rate(flagger_reconcile_total[%s]))`, window)
	totalResult, _ := f.query(totalQuery, nil)
	if len(totalResult) > 0 {
		health.ReconcileTotal = int64(totalResult[0].Value)
	}

	// Reconcile 错误数
	errorsQuery := fmt.Sprintf(`sum(rate(flagger_reconcile_errors_total[%s]))`, window)
	errorsResult, _ := f.query(errorsQuery, nil)
	if len(errorsResult) > 0 {
		health.ReconcileErrors = int64(errorsResult[0].Value)
	}

	// 计算错误率
	if health.ReconcileTotal > 0 {
		health.ReconcileErrorRate = float64(health.ReconcileErrors) / float64(health.ReconcileTotal) * 100
	}

	// 平均 Reconcile 时间
	avgTimeQuery := fmt.Sprintf(`avg(rate(flagger_reconcile_duration_seconds_sum[%s]) / rate(flagger_reconcile_duration_seconds_count[%s]))`, window, window)
	avgTimeResult, _ := f.query(avgTimeQuery, nil)
	if len(avgTimeResult) > 0 {
		health.AvgReconcileTime = avgTimeResult[0].Value
	}

	// P95 Reconcile 时间
	p95TimeQuery := fmt.Sprintf(`histogram_quantile(0.95, sum(rate(flagger_reconcile_duration_seconds_bucket[%s])) by (le))`, window)
	p95TimeResult, _ := f.query(p95TimeQuery, nil)
	if len(p95TimeResult) > 0 {
		health.P95ReconcileTime = p95TimeResult[0].Value
	}

	return health, nil
}

// ==================== Namespace 级别查询 ====================

func (f *FlaggerOperator) GetNamespaceCanaryMetrics(namespace string, timeRange *types.TimeRange) (*types.NamespaceCanaryMetrics, error) {
	metrics := &types.NamespaceCanaryMetrics{
		Namespace: namespace,
		Timestamp: time.Now(),
		Canaries:  []types.CanaryBrief{},
	}

	// 列出该 namespace 的所有 Canary
	list, err := f.ListCanaries(namespace)
	if err == nil {
		metrics.Canaries = list.Canaries
		metrics.Statistics = types.NamespaceCanaryStatistics{
			Total:  list.Summary.Total,
			Active: list.Summary.Active,
		}

		// 计算成功率
		if list.Summary.Succeeded+list.Summary.Failed > 0 {
			metrics.Statistics.SuccessRate = float64(list.Summary.Succeeded) / float64(list.Summary.Succeeded+list.Summary.Failed) * 100
		}
	}

	return metrics, nil
}

// ==================== 排行查询 ====================

func (f *FlaggerOperator) GetCanaryRanking(limit int, timeRange *types.TimeRange) (*types.CanaryRanking, error) {
	ranking := &types.CanaryRanking{
		TopByDuration:     []types.CanaryRankingItem{},
		TopByIterations:   []types.CanaryRankingItem{},
		TopByFailedChecks: []types.CanaryRankingItem{},
		RecentFailed:      []types.CanaryBrief{},
		RecentSucceeded:   []types.CanaryBrief{},
	}

	// Top by Duration
	durationQuery := fmt.Sprintf(`topk(%d, flagger_canary_duration_seconds)`, limit)
	durationResult, _ := f.query(durationQuery, nil)
	for _, r := range durationResult {
		namespace, _ := r.Metric["namespace"]
		name, _ := r.Metric["name"]
		ranking.TopByDuration = append(ranking.TopByDuration, types.CanaryRankingItem{
			Namespace: namespace,
			Name:      name,
			Value:     r.Value,
			Unit:      "seconds",
		})
	}

	// Top by Iterations
	iterationQuery := fmt.Sprintf(`topk(%d, flagger_canary_iteration)`, limit)
	iterationResult, _ := f.query(iterationQuery, nil)
	for _, r := range iterationResult {
		namespace, _ := r.Metric["namespace"]
		name, _ := r.Metric["name"]
		ranking.TopByIterations = append(ranking.TopByIterations, types.CanaryRankingItem{
			Namespace: namespace,
			Name:      name,
			Value:     r.Value,
			Unit:      "iterations",
		})
	}

	// Top by Failed Checks
	failedQuery := fmt.Sprintf(`topk(%d, flagger_canary_failed)`, limit)
	failedResult, _ := f.query(failedQuery, nil)
	for _, r := range failedResult {
		namespace, _ := r.Metric["namespace"]
		name, _ := r.Metric["name"]
		ranking.TopByFailedChecks = append(ranking.TopByFailedChecks, types.CanaryRankingItem{
			Namespace: namespace,
			Name:      name,
			Value:     r.Value,
			Unit:      "checks",
		})
	}

	return ranking, nil
}

// ==================== 历史记录 ====================

func (f *FlaggerOperator) GetCanaryHistory(namespace, name string, timeRange *types.TimeRange) (*types.CanaryHistory, error) {
	history := &types.CanaryHistory{
		Namespace: namespace,
		Name:      name,
		Records:   []types.CanaryHistoryRecord{},
	}

	// 这里需要根据实际的历史记录存储来实现
	// 如果使用 Prometheus，可能需要查询历史事件

	return history, nil
}

// ==================== 实时监控 ====================

func (f *FlaggerOperator) GetCanaryRealtimeMetrics(namespace, name string) (*types.CanaryRealtimeMetrics, error) {
	metrics := &types.CanaryRealtimeMetrics{
		Namespace:    namespace,
		Name:         name,
		Timestamp:    time.Now(),
		RecentChecks: []types.MetricCheckResult{},
	}

	// 当前状态
	status, _ := f.GetCanaryStatus(namespace, name)
	if status != nil {
		metrics.CurrentStatus = *status
	}

	// 当前权重
	progress, _ := f.GetCanaryProgress(namespace, name, nil)
	if progress != nil {
		metrics.CurrentWeight = progress.CurrentWeight
	}

	// 对比数据
	comparison, _ := f.GetCanaryComparison(namespace, name, nil)
	if comparison != nil {
		metrics.Comparison = *comparison
	}

	// 最近的检查结果
	analysis, _ := f.GetCanaryAnalysis(namespace, name, nil)
	if analysis != nil {
		metrics.RecentChecks = analysis.MetricChecks
	}

	return metrics, nil
}

// query 即时查询
func (f *FlaggerOperator) query(query string, timestamp *time.Time) ([]types.InstantQueryResult, error) {
	params := map[string]string{"query": query}
	if timestamp != nil {
		params["time"] = f.formatTimestamp(*timestamp)
	}

	var response struct {
		Status string `json:"status"`
		Data   struct {
			Result []struct {
				Metric map[string]string `json:"metric"`
				Value  []interface{}     `json:"value"`
			} `json:"result"`
		} `json:"data"`
		Error string `json:"error,omitempty"`
	}

	if err := f.doRequest("GET", "/api/v1/query", params, nil, &response); err != nil {
		return nil, err
	}

	if response.Status != "success" {
		return nil, fmt.Errorf("查询失败: %s", response.Error)
	}

	results := make([]types.InstantQueryResult, 0, len(response.Data.Result))
	for _, item := range response.Data.Result {
		if len(item.Value) != 2 {
			continue
		}
		timestampFloat, _ := item.Value[0].(float64)
		valueStr, _ := item.Value[1].(string)
		value, _ := strconv.ParseFloat(valueStr, 64)

		results = append(results, types.InstantQueryResult{
			Metric: item.Metric,
			Value:  value,
			Time:   time.Unix(int64(timestampFloat), 0),
		})
	}

	return results, nil
}

// queryRange 范围查询
func (f *FlaggerOperator) queryRange(query string, start, end time.Time, step string) ([]types.RangeQueryResult, error) {
	params := map[string]string{
		"query": query,
		"start": f.formatTimestamp(start),
		"end":   f.formatTimestamp(end),
		"step":  step,
	}

	var response struct {
		Status string `json:"status"`
		Data   struct {
			Result []struct {
				Metric map[string]string `json:"metric"`
				Values [][]interface{}   `json:"values"`
			} `json:"result"`
		} `json:"data"`
		Error string `json:"error,omitempty"`
	}

	if err := f.doRequest("GET", "/api/v1/query_range", params, nil, &response); err != nil {
		return nil, err
	}

	if response.Status != "success" {
		return nil, fmt.Errorf("查询失败: %s", response.Error)
	}

	results := make([]types.RangeQueryResult, 0, len(response.Data.Result))
	for _, item := range response.Data.Result {
		values := make([]types.MetricValue, 0, len(item.Values))
		for _, v := range item.Values {
			if len(v) != 2 {
				continue
			}
			timestampFloat, _ := v[0].(float64)
			valueStr, _ := v[1].(string)
			value, _ := strconv.ParseFloat(valueStr, 64)

			values = append(values, types.MetricValue{
				Timestamp: time.Unix(int64(timestampFloat), 0),
				Value:     value,
			})
		}

		if len(values) > 0 {
			results = append(results, types.RangeQueryResult{
				Metric: item.Metric,
				Values: values,
			})
		}
	}

	return results, nil
}

// phaseCodeToString 将状态码转换为字符串
func (f *FlaggerOperator) phaseCodeToString(code int) string {
	switch code {
	case 0:
		return "Initialized"
	case 1:
		return "Waiting"
	case 2:
		return "Progressing"
	case 3:
		return "Promoting"
	case 4:
		return "Finalising"
	case 5:
		return "Succeeded"
	case 6:
		return "Failed"
	default:
		return "Unknown"
	}
}

// 解析趋势数据的辅助方法
func (f *FlaggerOperator) parseCanaryTrend(values []types.MetricValue) []types.CanaryDataPoint {
	dataPoints := make([]types.CanaryDataPoint, 0, len(values))
	for _, v := range values {
		phaseCode := int(v.Value)
		dataPoints = append(dataPoints, types.CanaryDataPoint{
			Timestamp: v.Timestamp,
			Phase:     f.phaseCodeToString(phaseCode),
			PhaseCode: phaseCode,
		})
	}
	return dataPoints
}

func (f *FlaggerOperator) parseProgressTrend(values []types.MetricValue) []types.CanaryProgressDataPoint {
	dataPoints := make([]types.CanaryProgressDataPoint, 0, len(values))
	for _, v := range values {
		dataPoints = append(dataPoints, types.CanaryProgressDataPoint{
			Timestamp: v.Timestamp,
			Weight:    v.Value,
		})
	}
	return dataPoints
}

func (f *FlaggerOperator) parseAnalysisTrend(values []types.MetricValue, totalChecks int64) []types.CanaryAnalysisDataPoint {
	dataPoints := make([]types.CanaryAnalysisDataPoint, 0, len(values))
	for _, v := range values {
		failedChecks := int64(v.Value)
		successRate := 0.0
		if totalChecks > 0 {
			successRate = float64(totalChecks-failedChecks) / float64(totalChecks) * 100
		}
		dataPoints = append(dataPoints, types.CanaryAnalysisDataPoint{
			Timestamp:        v.Timestamp,
			CheckSuccessRate: successRate,
			FailedChecks:     failedChecks,
		})
	}
	return dataPoints
}

func (f *FlaggerOperator) parseComparisonTrend(canarySuccess, primarySuccess, canaryP95, primaryP95 []types.MetricValue) []types.CanaryComparisonDataPoint {
	dataPoints := make([]types.CanaryComparisonDataPoint, 0)
	minLen := len(canarySuccess)
	if len(primarySuccess) < minLen {
		minLen = len(primarySuccess)
	}

	for i := 0; i < minLen; i++ {
		dataPoint := types.CanaryComparisonDataPoint{
			Timestamp:          canarySuccess[i].Timestamp,
			CanarySuccessRate:  canarySuccess[i].Value * 100,
			PrimarySuccessRate: primarySuccess[i].Value * 100,
			CanaryErrorRate:    (1 - canarySuccess[i].Value) * 100,
			PrimaryErrorRate:   (1 - primarySuccess[i].Value) * 100,
		}

		if i < len(canaryP95) {
			dataPoint.CanaryP95Latency = canaryP95[i].Value
		}
		if i < len(primaryP95) {
			dataPoint.PrimaryP95Latency = primaryP95[i].Value
		}

		dataPoints = append(dataPoints, dataPoint)
	}

	return dataPoints
}

func (f *FlaggerOperator) parseStatisticsTrend(values []types.MetricValue) []types.FlaggerStatisticsDataPoint {
	dataPoints := make([]types.FlaggerStatisticsDataPoint, 0, len(values))
	for _, v := range values {
		dataPoints = append(dataPoints, types.FlaggerStatisticsDataPoint{
			Timestamp:     v.Timestamp,
			TotalCanaries: int64(v.Value),
		})
	}
	return dataPoints
}
