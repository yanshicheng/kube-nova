package operator

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/yanshicheng/kube-nova/common/prometheusmanager/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type IngressOperator struct {
	log logx.Logger
	ctx context.Context
	*BaseOperator
}

func NewIngressOperator(ctx context.Context, base *BaseOperator) types.IngressOperator {
	return &IngressOperator{
		log:          logx.WithContext(ctx),
		ctx:          ctx,
		BaseOperator: base,
	}
}

// ==================== 综合查询 ====================

// GetIngressMetrics 获取 Ingress 综合指标
func (i *IngressOperator) GetIngressMetrics(namespace, ingressName string, timeRange *types.TimeRange) (*types.IngressMetrics, error) {
	i.log.Infof("📊 查询 Ingress 指标: namespace=%s, ingress=%s", namespace, ingressName)

	metrics := &types.IngressMetrics{
		Namespace:   namespace,
		IngressName: ingressName,
		Timestamp:   time.Now(),
	}

	// 并发查询各项指标
	errCh := make(chan error, 6)

	go func() {
		controller, err := i.GetControllerHealth(timeRange)
		if err == nil {
			metrics.Controller = *controller
		}
		errCh <- err
	}()

	go func() {
		traffic, err := i.GetIngressTraffic(namespace, ingressName, timeRange)
		if err == nil {
			metrics.Traffic = *traffic
		}
		errCh <- err
	}()

	go func() {
		perf, err := i.GetIngressPerformance(namespace, ingressName, timeRange)
		if err == nil {
			metrics.Performance = *perf
		}
		errCh <- err
	}()

	go func() {
		errors, err := i.GetIngressErrors(namespace, ingressName, timeRange)
		if err == nil {
			metrics.Errors = *errors
		}
		errCh <- err
	}()

	go func() {
		backends, err := i.GetIngressBackends(namespace, ingressName, timeRange)
		if err == nil {
			metrics.Backends = *backends
		}
		errCh <- err
	}()

	go func() {
		certs, err := i.GetIngressCertificates(namespace, ingressName)
		if err == nil {
			metrics.Certificates = *certs
		}
		errCh <- err
	}()

	// 收集错误
	for j := 0; j < 6; j++ {
		if err := <-errCh; err != nil {
			i.log.Errorf("获取指标失败: %v", err)
		}
	}

	return metrics, nil
}

// ==================== Controller 健康 ====================

// GetControllerHealth 获取 Controller 健康状态
func (i *IngressOperator) GetControllerHealth(timeRange *types.TimeRange) (*types.IngressControllerHealth, error) {
	i.log.Infof("📊 查询 Ingress Controller 健康状态")

	health := &types.IngressControllerHealth{
		ControllerName: "nginx-ingress-controller",
		Trend:          []types.IngressControllerDataPoint{},
	}

	// 查询 Controller Pod 状态
	totalPodsQuery := `count(kube_pod_info{namespace="ingress-nginx"})`
	totalPodsResult, _ := i.query(totalPodsQuery, nil)
	if len(totalPodsResult) > 0 {
		health.TotalPods = int64(totalPodsResult[0].Value)
	}

	runningPodsQuery := `count(kube_pod_status_phase{namespace="ingress-nginx",phase="Running"})`
	runningPodsResult, _ := i.query(runningPodsQuery, nil)
	if len(runningPodsResult) > 0 {
		health.RunningPods = int64(runningPodsResult[0].Value)
	}

	readyPodsQuery := `count(kube_pod_status_ready{namespace="ingress-nginx",condition="true"})`
	readyPodsResult, _ := i.query(readyPodsQuery, nil)
	if len(readyPodsResult) > 0 {
		health.ReadyPods = int64(readyPodsResult[0].Value)
	}

	// CPU 使用率 - 使用动态窗口
	window := i.calculateRateWindow(timeRange)
	cpuQuery := fmt.Sprintf(`sum(rate(container_cpu_usage_seconds_total{namespace="ingress-nginx",container="controller"}[%s]))`, window)
	cpuResult, _ := i.query(cpuQuery, nil)
	if len(cpuResult) > 0 {
		health.CPUUsage = cpuResult[0].Value
	}

	// 内存使用
	memQuery := `sum(container_memory_usage_bytes{namespace="ingress-nginx",container="controller"})`
	memResult, _ := i.query(memQuery, nil)
	if len(memResult) > 0 {
		health.MemoryUsage = int64(memResult[0].Value)
	}

	// 连接指标
	activeConnQuery := `sum(nginx_ingress_controller_nginx_process_connections{state="active"})`
	activeConnResult, _ := i.query(activeConnQuery, nil)
	if len(activeConnResult) > 0 {
		health.Connections.Active = int64(activeConnResult[0].Value)
	}

	readingQuery := `sum(nginx_ingress_controller_nginx_process_connections{state="reading"})`
	readingResult, _ := i.query(readingQuery, nil)
	if len(readingResult) > 0 {
		health.Connections.Reading = int64(readingResult[0].Value)
	}

	writingQuery := `sum(nginx_ingress_controller_nginx_process_connections{state="writing"})`
	writingResult, _ := i.query(writingQuery, nil)
	if len(writingResult) > 0 {
		health.Connections.Writing = int64(writingResult[0].Value)
	}

	waitingQuery := `sum(nginx_ingress_controller_nginx_process_connections{state="waiting"})`
	waitingResult, _ := i.query(waitingQuery, nil)
	if len(waitingResult) > 0 {
		health.Connections.Waiting = int64(waitingResult[0].Value)
	}

	health.Connections.Total = health.Connections.Active + health.Connections.Reading +
		health.Connections.Writing + health.Connections.Waiting

	// Reload 统计
	reloadRateQuery := fmt.Sprintf(`sum(rate(nginx_ingress_controller_success{controller_class=~".*"}[%s]))`, window)
	reloadRateResult, _ := i.query(reloadRateQuery, nil)
	if len(reloadRateResult) > 0 {
		health.ReloadRate = reloadRateResult[0].Value
	}

	reloadFailQuery := fmt.Sprintf(`sum(rate(nginx_ingress_controller_errors{controller_class=~".*"}[%s]))`, window)
	reloadFailResult, _ := i.query(reloadFailQuery, nil)
	if len(reloadFailResult) > 0 {
		health.ReloadFailRate = reloadFailResult[0].Value
	}

	// 最后一次 reload 状态
	lastReloadQuery := `nginx_ingress_controller_config_last_reload_successful`
	lastReloadResult, _ := i.query(lastReloadQuery, nil)
	if len(lastReloadResult) > 0 {
		health.LastReloadSuccess = lastReloadResult[0].Value > 0
	}

	lastReloadTimeQuery := `nginx_ingress_controller_config_last_reload_successful_timestamp_seconds`
	lastReloadTimeResult, _ := i.query(lastReloadTimeQuery, nil)
	if len(lastReloadTimeResult) > 0 {
		health.LastReloadTime = time.Unix(int64(lastReloadTimeResult[0].Value), 0)
	}

	// 趋势数据
	if timeRange != nil && !timeRange.Start.IsZero() && !timeRange.End.IsZero() {
		step := timeRange.Step
		if step == "" {
			step = i.calculateStep(timeRange.Start, timeRange.End)
		}

		trendQuery := `sum(nginx_ingress_controller_nginx_process_connections{state="active"})`
		trendResult, err := i.queryRange(trendQuery, timeRange.Start, timeRange.End, step)
		if err == nil && len(trendResult) > 0 {
			health.Trend = i.parseControllerTrend(trendResult[0].Values)
		}
	}

	return health, nil
}

// ==================== 流量查询 ====================

// GetIngressTraffic 获取流量指标
func (i *IngressOperator) GetIngressTraffic(namespace, ingressName string, timeRange *types.TimeRange) (*types.IngressTrafficMetrics, error) {
	i.log.Infof("📊 查询 Ingress 流量: namespace=%s, ingress=%s", namespace, ingressName)

	traffic := &types.IngressTrafficMetrics{
		Trend:     []types.IngressTrafficDataPoint{},
		ByHost:    []types.TrafficByDimension{},
		ByPath:    []types.TrafficByDimension{},
		ByService: []types.TrafficByDimension{},
		ByMethod:  []types.TrafficByMethod{},
	}

	window := i.calculateRateWindow(timeRange)

	// 当前 QPS
	qpsQuery := fmt.Sprintf(`sum(rate(nginx_ingress_controller_requests{namespace="%s",ingress="%s"}[%s]))`,
		namespace, ingressName, window)
	qpsResult, _ := i.query(qpsQuery, nil)
	if len(qpsResult) > 0 {
		traffic.Current.RequestsPerSecond = qpsResult[0].Value
		traffic.Current.Timestamp = qpsResult[0].Time
	}

	// 当前连接数
	connQuery := `sum(nginx_ingress_controller_nginx_process_connections{state="active"})`
	connResult, _ := i.query(connQuery, nil)
	if len(connResult) > 0 {
		traffic.Current.ActiveConnections = int64(connResult[0].Value)
	}

	// 流入流出字节
	ingressBytesQuery := fmt.Sprintf(`sum(rate(nginx_ingress_controller_request_size_sum{namespace="%s",ingress="%s"}[%s]))`,
		namespace, ingressName, window)
	ingressBytesResult, _ := i.query(ingressBytesQuery, nil)
	if len(ingressBytesResult) > 0 {
		traffic.Current.IngressBytesPerSec = ingressBytesResult[0].Value
	}

	egressBytesQuery := fmt.Sprintf(`sum(rate(nginx_ingress_controller_response_size_sum{namespace="%s",ingress="%s"}[%s]))`,
		namespace, ingressName, window)
	egressBytesResult, _ := i.query(egressBytesQuery, nil)
	if len(egressBytesResult) > 0 {
		traffic.Current.EgressBytesPerSec = egressBytesResult[0].Value
	}

	// 按 Host 统计
	byHostQuery := fmt.Sprintf(`sum(rate(nginx_ingress_controller_requests{namespace="%s",ingress="%s"}[%s])) by (host)`,
		namespace, ingressName, window)
	byHostResult, _ := i.query(byHostQuery, nil)
	for _, r := range byHostResult {
		if host, ok := r.Metric["host"]; ok {
			traffic.ByHost = append(traffic.ByHost, types.TrafficByDimension{
				Name:              host,
				Namespace:         namespace,
				RequestsPerSecond: r.Value,
			})
		}
	}

	// 按 Path 统计
	byPathQuery := fmt.Sprintf(`sum(rate(nginx_ingress_controller_requests{namespace="%s",ingress="%s"}[%s])) by (path)`,
		namespace, ingressName, window)
	byPathResult, _ := i.query(byPathQuery, nil)
	for _, r := range byPathResult {
		if path, ok := r.Metric["path"]; ok {
			traffic.ByPath = append(traffic.ByPath, types.TrafficByDimension{
				Name:              path,
				Namespace:         namespace,
				RequestsPerSecond: r.Value,
			})
		}
	}

	// 按 Service 统计
	byServiceQuery := fmt.Sprintf(`sum(rate(nginx_ingress_controller_requests{namespace="%s",ingress="%s"}[%s])) by (service)`,
		namespace, ingressName, window)
	byServiceResult, _ := i.query(byServiceQuery, nil)
	for _, r := range byServiceResult {
		if service, ok := r.Metric["service"]; ok {
			traffic.ByService = append(traffic.ByService, types.TrafficByDimension{
				Name:              service,
				Namespace:         namespace,
				RequestsPerSecond: r.Value,
			})
		}
	}

	// 按 Method 统计
	byMethodQuery := fmt.Sprintf(`sum(rate(nginx_ingress_controller_requests{namespace="%s",ingress="%s"}[%s])) by (method)`,
		namespace, ingressName, window)
	byMethodResult, _ := i.query(byMethodQuery, nil)
	for _, r := range byMethodResult {
		if method, ok := r.Metric["method"]; ok {
			traffic.ByMethod = append(traffic.ByMethod, types.TrafficByMethod{
				Method:            method,
				RequestsPerSecond: r.Value,
			})
		}
	}

	// 趋势数据
	if timeRange != nil && !timeRange.Start.IsZero() && !timeRange.End.IsZero() {
		step := timeRange.Step
		if step == "" {
			step = i.calculateStep(timeRange.Start, timeRange.End)
		}

		trendQuery := fmt.Sprintf(`sum(rate(nginx_ingress_controller_requests{namespace="%s",ingress="%s"}[%s]))`,
			namespace, ingressName, window)
		trendResult, err := i.queryRange(trendQuery, timeRange.Start, timeRange.End, step)
		if err == nil && len(trendResult) > 0 {
			traffic.Trend = i.parseTrafficTrend(trendResult[0].Values)
		}

		// 字节趋势
		ingressBytesTrend, _ := i.queryRange(ingressBytesQuery, timeRange.Start, timeRange.End, step)
		egressBytesTrend, _ := i.queryRange(egressBytesQuery, timeRange.Start, timeRange.End, step)

		if len(ingressBytesTrend) > 0 && len(egressBytesTrend) > 0 {
			for idx := range traffic.Trend {
				if idx < len(ingressBytesTrend[0].Values) {
					traffic.Trend[idx].IngressBytesPerSec = ingressBytesTrend[0].Values[idx].Value
				}
				if idx < len(egressBytesTrend[0].Values) {
					traffic.Trend[idx].EgressBytesPerSec = egressBytesTrend[0].Values[idx].Value
				}
			}
		}
	}

	// 汇总统计
	traffic.Summary = i.calculateTrafficSummary(traffic)

	return traffic, nil
}

// GetIngressTrafficByHost 按 Host 获取流量
func (i *IngressOperator) GetIngressTrafficByHost(host string, timeRange *types.TimeRange) (*types.IngressTrafficMetrics, error) {
	traffic := &types.IngressTrafficMetrics{
		Trend:     []types.IngressTrafficDataPoint{},
		ByHost:    []types.TrafficByDimension{},
		ByPath:    []types.TrafficByDimension{},
		ByService: []types.TrafficByDimension{},
		ByMethod:  []types.TrafficByMethod{},
	}

	window := i.calculateRateWindow(timeRange)

	qpsQuery := fmt.Sprintf(`sum(rate(nginx_ingress_controller_requests{host="%s"}[%s]))`, host, window)
	qpsResult, _ := i.query(qpsQuery, nil)
	if len(qpsResult) > 0 {
		traffic.Current.RequestsPerSecond = qpsResult[0].Value
		traffic.Current.Timestamp = qpsResult[0].Time
	}

	// 趋势数据
	if timeRange != nil && !timeRange.Start.IsZero() && !timeRange.End.IsZero() {
		step := timeRange.Step
		if step == "" {
			step = i.calculateStep(timeRange.Start, timeRange.End)
		}

		trendResult, err := i.queryRange(qpsQuery, timeRange.Start, timeRange.End, step)
		if err == nil && len(trendResult) > 0 {
			traffic.Trend = i.parseTrafficTrend(trendResult[0].Values)
		}
	}

	return traffic, nil
}

// GetIngressTrafficByPath 按 Path 获取流量
func (i *IngressOperator) GetIngressTrafficByPath(path string, timeRange *types.TimeRange) (*types.IngressTrafficMetrics, error) {
	traffic := &types.IngressTrafficMetrics{
		Trend:     []types.IngressTrafficDataPoint{},
		ByHost:    []types.TrafficByDimension{},
		ByPath:    []types.TrafficByDimension{},
		ByService: []types.TrafficByDimension{},
		ByMethod:  []types.TrafficByMethod{},
	}

	window := i.calculateRateWindow(timeRange)

	qpsQuery := fmt.Sprintf(`sum(rate(nginx_ingress_controller_requests{path="%s"}[%s]))`, path, window)
	qpsResult, _ := i.query(qpsQuery, nil)
	if len(qpsResult) > 0 {
		traffic.Current.RequestsPerSecond = qpsResult[0].Value
		traffic.Current.Timestamp = qpsResult[0].Time
	}

	// 趋势数据
	if timeRange != nil && !timeRange.Start.IsZero() && !timeRange.End.IsZero() {
		step := timeRange.Step
		if step == "" {
			step = i.calculateStep(timeRange.Start, timeRange.End)
		}

		trendResult, err := i.queryRange(qpsQuery, timeRange.Start, timeRange.End, step)
		if err == nil && len(trendResult) > 0 {
			traffic.Trend = i.parseTrafficTrend(trendResult[0].Values)
		}
	}

	return traffic, nil
}

// ==================== 性能查询 ====================

// GetIngressPerformance 获取性能指标
func (i *IngressOperator) GetIngressPerformance(namespace, ingressName string, timeRange *types.TimeRange) (*types.IngressPerformanceMetrics, error) {
	i.log.Infof("📊 查询 Ingress 性能: namespace=%s, ingress=%s", namespace, ingressName)

	perf := &types.IngressPerformanceMetrics{
		ByHost: []types.LatencyByDimension{},
		ByPath: []types.LatencyByDimension{},
		Trend:  []types.IngressLatencyDataPoint{},
	}

	window := i.calculateRateWindow(timeRange)

	// 整体延迟统计
	p50Query := fmt.Sprintf(
		`histogram_quantile(0.50, sum(rate(nginx_ingress_controller_request_duration_seconds_bucket{namespace="%s",ingress="%s"}[%s])) by (le))`,
		namespace, ingressName, window)
	p50Result, _ := i.query(p50Query, nil)
	if len(p50Result) > 0 {
		perf.Overall.P50 = p50Result[0].Value
	}

	p95Query := fmt.Sprintf(
		`histogram_quantile(0.95, sum(rate(nginx_ingress_controller_request_duration_seconds_bucket{namespace="%s",ingress="%s"}[%s])) by (le))`,
		namespace, ingressName, window)
	p95Result, _ := i.query(p95Query, nil)
	if len(p95Result) > 0 {
		perf.Overall.P95 = p95Result[0].Value
	}

	p99Query := fmt.Sprintf(
		`histogram_quantile(0.99, sum(rate(nginx_ingress_controller_request_duration_seconds_bucket{namespace="%s",ingress="%s"}[%s])) by (le))`,
		namespace, ingressName, window)
	p99Result, _ := i.query(p99Query, nil)
	if len(p99Result) > 0 {
		perf.Overall.P99 = p99Result[0].Value
	}

	// 平均延迟
	avgQuery := fmt.Sprintf(
		`sum(rate(nginx_ingress_controller_request_duration_seconds_sum{namespace="%s",ingress="%s"}[%s])) / sum(rate(nginx_ingress_controller_request_duration_seconds_count{namespace="%s",ingress="%s"}[%s]))`,
		namespace, ingressName, window, namespace, ingressName, window)
	avgResult, _ := i.query(avgQuery, nil)
	if len(avgResult) > 0 {
		perf.Overall.Avg = avgResult[0].Value
	}

	// 最大延迟
	maxQuery := fmt.Sprintf(
		`max(nginx_ingress_controller_request_duration_seconds{namespace="%s",ingress="%s"})`,
		namespace, ingressName)
	maxResult, _ := i.query(maxQuery, nil)
	if len(maxResult) > 0 {
		perf.Overall.Max = maxResult[0].Value
	}

	// 按 Host 延迟
	byHostQuery := fmt.Sprintf(
		`histogram_quantile(0.95, sum(rate(nginx_ingress_controller_request_duration_seconds_bucket{namespace="%s",ingress="%s"}[%s])) by (host, le))`,
		namespace, ingressName, window)
	byHostResult, _ := i.query(byHostQuery, nil)
	for _, r := range byHostResult {
		if host, ok := r.Metric["host"]; ok {
			perf.ByHost = append(perf.ByHost, types.LatencyByDimension{
				Name:      host,
				Namespace: namespace,
				Latency: types.IngressLatencyStats{
					P95: r.Value,
				},
			})
		}
	}

	// 按 Path 延迟
	byPathQuery := fmt.Sprintf(
		`histogram_quantile(0.95, sum(rate(nginx_ingress_controller_request_duration_seconds_bucket{namespace="%s",ingress="%s"}[%s])) by (path, le))`,
		namespace, ingressName, window)
	byPathResult, _ := i.query(byPathQuery, nil)
	for _, r := range byPathResult {
		if path, ok := r.Metric["path"]; ok {
			perf.ByPath = append(perf.ByPath, types.LatencyByDimension{
				Name:      path,
				Namespace: namespace,
				Latency: types.IngressLatencyStats{
					P95: r.Value,
				},
			})
		}
	}

	// Upstream 延迟
	upstreamP95Query := fmt.Sprintf(
		`histogram_quantile(0.95, sum(rate(nginx_ingress_controller_ingress_upstream_latency_seconds_bucket{namespace="%s",ingress="%s"}[%s])) by (le))`,
		namespace, ingressName, window)
	upstreamP95Result, _ := i.query(upstreamP95Query, nil)
	if len(upstreamP95Result) > 0 {
		perf.UpstreamLatency.P95 = upstreamP95Result[0].Value
	}

	// 趋势数据
	if timeRange != nil && !timeRange.Start.IsZero() && !timeRange.End.IsZero() {
		step := timeRange.Step
		if step == "" {
			step = i.calculateStep(timeRange.Start, timeRange.End)
		}

		// P50/P95/P99 趋势
		p50Trend, _ := i.queryRange(p50Query, timeRange.Start, timeRange.End, step)
		p95Trend, _ := i.queryRange(p95Query, timeRange.Start, timeRange.End, step)
		p99Trend, _ := i.queryRange(p99Query, timeRange.Start, timeRange.End, step)

		if len(p95Trend) > 0 {
			perf.Trend = make([]types.IngressLatencyDataPoint, 0, len(p95Trend[0].Values))
			for idx, v := range p95Trend[0].Values {
				dataPoint := types.IngressLatencyDataPoint{
					Timestamp: v.Timestamp,
					P95:       v.Value,
				}

				if idx < len(p50Trend[0].Values) {
					dataPoint.P50 = p50Trend[0].Values[idx].Value
				}
				if idx < len(p99Trend[0].Values) {
					dataPoint.P99 = p99Trend[0].Values[idx].Value
				}

				perf.Trend = append(perf.Trend, dataPoint)
			}
		}
	}

	return perf, nil
}

// GetIngressLatencyByHost 按 Host 获取延迟
func (i *IngressOperator) GetIngressLatencyByHost(host string, timeRange *types.TimeRange) (*types.IngressLatencyStats, error) {
	latency := &types.IngressLatencyStats{}

	window := i.calculateRateWindow(timeRange)

	p95Query := fmt.Sprintf(
		`histogram_quantile(0.95, sum(rate(nginx_ingress_controller_request_duration_seconds_bucket{host="%s"}[%s])) by (le))`,
		host, window)
	p95Result, _ := i.query(p95Query, nil)
	if len(p95Result) > 0 {
		latency.P95 = p95Result[0].Value
	}

	return latency, nil
}

// GetIngressLatencyByPath 按 Path 获取延迟
func (i *IngressOperator) GetIngressLatencyByPath(path string, timeRange *types.TimeRange) (*types.IngressLatencyStats, error) {
	latency := &types.IngressLatencyStats{}

	window := i.calculateRateWindow(timeRange)

	p95Query := fmt.Sprintf(
		`histogram_quantile(0.95, sum(rate(nginx_ingress_controller_request_duration_seconds_bucket{path="%s"}[%s])) by (le))`,
		path, window)
	p95Result, _ := i.query(p95Query, nil)
	if len(p95Result) > 0 {
		latency.P95 = p95Result[0].Value
	}

	return latency, nil
}

// ==================== 错误查询 ====================

// GetIngressErrors 获取错误指标
func (i *IngressOperator) GetIngressErrors(namespace, ingressName string, timeRange *types.TimeRange) (*types.IngressErrorMetrics, error) {
	i.log.Infof("📊 查询 Ingress 错误: namespace=%s, ingress=%s", namespace, ingressName)

	errors := &types.IngressErrorMetrics{
		ByHost: []types.ErrorRateByDimension{},
		ByPath: []types.ErrorRateByDimension{},
		Trend:  []types.IngressErrorDataPoint{},
	}

	window := i.calculateRateWindow(timeRange)

	// 整体错误率
	totalErrorQuery := fmt.Sprintf(
		`sum(rate(nginx_ingress_controller_requests{namespace="%s",ingress="%s",status=~"[4-5].."}[%s])) / sum(rate(nginx_ingress_controller_requests{namespace="%s",ingress="%s"}[%s]))`,
		namespace, ingressName, window, namespace, ingressName, window)
	totalErrorResult, _ := i.query(totalErrorQuery, nil)
	if len(totalErrorResult) > 0 {
		errors.Overall.TotalErrorRate = totalErrorResult[0].Value * 100
	}

	// 4xx 错误率
	error4xxQuery := fmt.Sprintf(
		`sum(rate(nginx_ingress_controller_requests{namespace="%s",ingress="%s",status=~"4.."}[%s])) / sum(rate(nginx_ingress_controller_requests{namespace="%s",ingress="%s"}[%s]))`,
		namespace, ingressName, window, namespace, ingressName, window)
	error4xxResult, _ := i.query(error4xxQuery, nil)
	if len(error4xxResult) > 0 {
		errors.Overall.Error4xxRate = error4xxResult[0].Value * 100
	}

	// 5xx 错误率
	error5xxQuery := fmt.Sprintf(
		`sum(rate(nginx_ingress_controller_requests{namespace="%s",ingress="%s",status=~"5.."}[%s])) / sum(rate(nginx_ingress_controller_requests{namespace="%s",ingress="%s"}[%s]))`,
		namespace, ingressName, window, namespace, ingressName, window)
	error5xxResult, _ := i.query(error5xxQuery, nil)
	if len(error5xxResult) > 0 {
		errors.Overall.Error5xxRate = error5xxResult[0].Value * 100
	}

	// 状态码分布
	statusCodes, err := i.GetIngressStatusCodes(namespace, ingressName, timeRange)
	if err == nil {
		errors.StatusCodes = *statusCodes
	}

	// 按 Host 错误率
	byHostErrorQuery := fmt.Sprintf(
		`sum(rate(nginx_ingress_controller_requests{namespace="%s",ingress="%s",status=~"[4-5].."}[%s])) by (host) / sum(rate(nginx_ingress_controller_requests{namespace="%s",ingress="%s"}[%s])) by (host)`,
		namespace, ingressName, window, namespace, ingressName, window)
	byHostErrorResult, _ := i.query(byHostErrorQuery, nil)
	for _, r := range byHostErrorResult {
		if host, ok := r.Metric["host"]; ok {
			errors.ByHost = append(errors.ByHost, types.ErrorRateByDimension{
				Name:      host,
				Namespace: namespace,
				ErrorRate: types.IngressErrorRateStats{
					TotalErrorRate: r.Value * 100,
				},
			})
		}
	}

	// 按 Path 错误率
	byPathErrorQuery := fmt.Sprintf(
		`sum(rate(nginx_ingress_controller_requests{namespace="%s",ingress="%s",status=~"[4-5].."}[%s])) by (path) / sum(rate(nginx_ingress_controller_requests{namespace="%s",ingress="%s"}[%s])) by (path)`,
		namespace, ingressName, window, namespace, ingressName, window)
	byPathErrorResult, _ := i.query(byPathErrorQuery, nil)
	for _, r := range byPathErrorResult {
		if path, ok := r.Metric["path"]; ok {
			errors.ByPath = append(errors.ByPath, types.ErrorRateByDimension{
				Name:      path,
				Namespace: namespace,
				ErrorRate: types.IngressErrorRateStats{
					TotalErrorRate: r.Value * 100,
				},
			})
		}
	}

	// 趋势数据
	if timeRange != nil && !timeRange.Start.IsZero() && !timeRange.End.IsZero() {
		step := timeRange.Step
		if step == "" {
			step = i.calculateStep(timeRange.Start, timeRange.End)
		}

		totalErrorTrend, _ := i.queryRange(totalErrorQuery, timeRange.Start, timeRange.End, step)
		error4xxTrend, _ := i.queryRange(error4xxQuery, timeRange.Start, timeRange.End, step)
		error5xxTrend, _ := i.queryRange(error5xxQuery, timeRange.Start, timeRange.End, step)

		if len(totalErrorTrend) > 0 {
			errors.Trend = make([]types.IngressErrorDataPoint, 0, len(totalErrorTrend[0].Values))
			for idx, v := range totalErrorTrend[0].Values {
				dataPoint := types.IngressErrorDataPoint{
					Timestamp:      v.Timestamp,
					TotalErrorRate: v.Value * 100,
				}

				if idx < len(error4xxTrend[0].Values) {
					dataPoint.Error4xxRate = error4xxTrend[0].Values[idx].Value * 100
				}
				if idx < len(error5xxTrend[0].Values) {
					dataPoint.Error5xxRate = error5xxTrend[0].Values[idx].Value * 100
				}

				errors.Trend = append(errors.Trend, dataPoint)
			}
		}
	}

	return errors, nil
}

// GetIngressStatusCodes 获取状态码分布
func (i *IngressOperator) GetIngressStatusCodes(namespace, ingressName string, timeRange *types.TimeRange) (*types.IngressStatusCodeDistribution, error) {
	dist := &types.IngressStatusCodeDistribution{
		Status2xx: make(map[string]float64),
		Status3xx: make(map[string]float64),
		Status4xx: make(map[string]float64),
		Status5xx: make(map[string]float64),
	}

	window := i.calculateRateWindow(timeRange)

	// 查询各状态码
	query := fmt.Sprintf(
		`sum(rate(nginx_ingress_controller_requests{namespace="%s",ingress="%s"}[%s])) by (status)`,
		namespace, ingressName, window)
	result, _ := i.query(query, nil)

	for _, r := range result {
		if status, ok := r.Metric["status"]; ok {
			if len(status) < 1 {
				continue
			}

			switch status[0] {
			case '2':
				dist.Status2xx[status] = r.Value
			case '3':
				dist.Status3xx[status] = r.Value
			case '4':
				dist.Status4xx[status] = r.Value
			case '5':
				dist.Status5xx[status] = r.Value
			}
		}
	}

	return dist, nil
}

// ==================== 后端查询 ====================

// GetIngressBackends 获取后端指标
func (i *IngressOperator) GetIngressBackends(namespace, ingressName string, timeRange *types.TimeRange) (*types.IngressBackendMetrics, error) {
	i.log.Infof("📊 查询 Ingress 后端: namespace=%s, ingress=%s", namespace, ingressName)

	backends := &types.IngressBackendMetrics{
		EndpointsByService: []types.ServiceEndpoints{},
		BackendHealth:      []types.BackendHealthStatus{},
	}

	window := i.calculateRateWindow(timeRange)

	// Upstream 延迟
	upstreamQuery := fmt.Sprintf(
		`avg(nginx_ingress_controller_ingress_upstream_latency_seconds{namespace="%s",ingress="%s"})`,
		namespace, ingressName)
	upstreamResult, _ := i.query(upstreamQuery, nil)
	if len(upstreamResult) > 0 {
		backends.UpstreamLatency = upstreamResult[0].Value
	}

	// 查询 Endpoints
	endpointsQuery := fmt.Sprintf(
		`sum(kube_endpoint_address_available{namespace="%s"}) by (endpoint)`,
		namespace)
	endpointsResult, _ := i.query(endpointsQuery, nil)
	for _, r := range endpointsResult {
		if serviceName, ok := r.Metric["endpoint"]; ok {
			backends.EndpointsByService = append(backends.EndpointsByService, types.ServiceEndpoints{
				ServiceName:        serviceName,
				Namespace:          namespace,
				AvailableEndpoints: int64(r.Value),
				HasEndpoints:       r.Value > 0,
			})
		}
	}

	// 后端健康状态
	healthQuery := fmt.Sprintf(
		`sum(rate(nginx_ingress_controller_requests{namespace="%s",ingress="%s",status=~"2.."}[%s])) by (service) / sum(rate(nginx_ingress_controller_requests{namespace="%s",ingress="%s"}[%s])) by (service)`,
		namespace, ingressName, window, namespace, ingressName, window)
	healthResult, _ := i.query(healthQuery, nil)
	for _, r := range healthResult {
		if upstream, ok := r.Metric["service"]; ok {
			backends.BackendHealth = append(backends.BackendHealth, types.BackendHealthStatus{
				Upstream:    upstream,
				SuccessRate: r.Value * 100,
			})
		}
	}

	return backends, nil
}

// ==================== 证书查询 ====================

// GetIngressCertificates 获取证书指标
func (i *IngressOperator) GetIngressCertificates(namespace, ingressName string) (*types.IngressCertificateMetrics, error) {
	i.log.Infof("📊 查询 Ingress 证书: namespace=%s, ingress=%s", namespace, ingressName)

	certs := &types.IngressCertificateMetrics{
		Certificates: []types.CertificateInfo{},
	}

	// HTTPS 请求占比
	httpsQuery := fmt.Sprintf(
		`sum(rate(nginx_ingress_controller_requests{namespace="%s",ingress="%s",scheme="https"}[5m])) / sum(rate(nginx_ingress_controller_requests{namespace="%s",ingress="%s"}[5m]))`,
		namespace, ingressName, namespace, ingressName)
	httpsResult, _ := i.query(httpsQuery, nil)
	if len(httpsResult) > 0 {
		certs.HTTPSRequestPercent = httpsResult[0].Value * 100
	}

	// 查询证书信息
	certQuery := `nginx_ingress_controller_ssl_expire_time_seconds`
	certResult, _ := i.query(certQuery, nil)

	now := time.Now()
	for _, r := range certResult {
		if certName, ok := r.Metric["name"]; ok {
			certNamespace, _ := r.Metric["namespace"]
			expirationTime := time.Unix(int64(r.Value), 0)
			daysRemaining := int(expirationTime.Sub(now).Hours() / 24)

			cert := types.CertificateInfo{
				Name:           certName,
				Namespace:      certNamespace,
				ExpirationTime: expirationTime,
				DaysRemaining:  daysRemaining,
				IsExpiring:     daysRemaining < 30 && daysRemaining >= 0,
				IsExpired:      daysRemaining < 0,
			}

			certs.Certificates = append(certs.Certificates, cert)

			if cert.IsExpired {
				certs.ExpiredCount++
			} else if cert.IsExpiring {
				certs.ExpiringCount++
			}
		}
	}

	return certs, nil
}

// GetExpiringCertificates 获取即将过期的证书
func (i *IngressOperator) GetExpiringCertificates(daysThreshold int) ([]types.CertificateInfo, error) {
	certs := []types.CertificateInfo{}

	certQuery := `nginx_ingress_controller_ssl_expire_time_seconds`
	certResult, _ := i.query(certQuery, nil)

	now := time.Now()
	for _, r := range certResult {
		if certName, ok := r.Metric["name"]; ok {
			certNamespace, _ := r.Metric["namespace"]
			expirationTime := time.Unix(int64(r.Value), 0)
			daysRemaining := int(expirationTime.Sub(now).Hours() / 24)

			if daysRemaining < daysThreshold && daysRemaining >= 0 {
				certs = append(certs, types.CertificateInfo{
					Name:           certName,
					Namespace:      certNamespace,
					ExpirationTime: expirationTime,
					DaysRemaining:  daysRemaining,
					IsExpiring:     true,
					IsExpired:      false,
				})
			}
		}
	}

	// 按剩余天数排序
	sort.Slice(certs, func(i, j int) bool {
		return certs[i].DaysRemaining < certs[j].DaysRemaining
	})

	return certs, nil
}

// ==================== Ingress 对象查询 ====================

// GetIngressObject 获取 Ingress 对象指标
func (i *IngressOperator) GetIngressObject(namespace, ingressName string) (*types.IngressObjectMetrics, error) {
	metrics := &types.IngressObjectMetrics{
		Namespace:   namespace,
		IngressName: ingressName,
		Hosts:       []string{},
		Paths:       []types.IngressPathInfo{},
		Labels:      make(map[string]string),
		Annotations: make(map[string]string),
	}

	// 查询 Ingress 信息
	infoQuery := fmt.Sprintf(`kube_ingress_info{namespace="%s",ingress="%s"}`, namespace, ingressName)
	infoResult, _ := i.query(infoQuery, nil)
	if len(infoResult) > 0 {
		metrics.Labels = infoResult[0].Metric
	}

	// 查询 Path 数量
	pathQuery := fmt.Sprintf(`kube_ingress_path{namespace="%s",ingress="%s"}`, namespace, ingressName)
	pathResult, _ := i.query(pathQuery, nil)
	metrics.PathCount = int64(len(pathResult))

	// 查询 TLS 状态
	tlsQuery := fmt.Sprintf(`kube_ingress_tls{namespace="%s",ingress="%s"}`, namespace, ingressName)
	tlsResult, _ := i.query(tlsQuery, nil)
	metrics.TLSEnabled = len(tlsResult) > 0

	// 查询创建时间
	createdQuery := fmt.Sprintf(`kube_ingress_created{namespace="%s",ingress="%s"}`, namespace, ingressName)
	createdResult, _ := i.query(createdQuery, nil)
	if len(createdResult) > 0 {
		metrics.CreatedAt = time.Unix(int64(createdResult[0].Value), 0)
	}

	return metrics, nil
}

// ==================== 限流查询 ====================

// GetIngressRateLimit 获取限流指标
func (i *IngressOperator) GetIngressRateLimit(namespace, ingressName string, timeRange *types.TimeRange) (*types.IngressRateLimitMetrics, error) {
	metrics := &types.IngressRateLimitMetrics{
		Namespace:   namespace,
		IngressName: ingressName,
		ByPath:      []types.RateLimitByPath{},
		Trend:       []types.IngressRateLimitDataPoint{},
	}

	window := i.calculateRateWindow(timeRange)

	// 被限流的请求速率
	limitedQuery := fmt.Sprintf(
		`sum(rate(nginx_ingress_controller_requests{namespace="%s",ingress="%s",status="429"}[%s]))`,
		namespace, ingressName, window)
	limitedResult, _ := i.query(limitedQuery, nil)
	if len(limitedResult) > 0 {
		metrics.LimitedRequests = limitedResult[0].Value
	}

	// 总请求速率
	totalQuery := fmt.Sprintf(
		`sum(rate(nginx_ingress_controller_requests{namespace="%s",ingress="%s"}[%s]))`,
		namespace, ingressName, window)
	totalResult, _ := i.query(totalQuery, nil)
	if len(totalResult) > 0 && totalResult[0].Value > 0 {
		metrics.LimitTriggerRate = (metrics.LimitedRequests / totalResult[0].Value) * 100
	}

	// 按 Path 限流
	byPathQuery := fmt.Sprintf(
		`sum(rate(nginx_ingress_controller_requests{namespace="%s",ingress="%s",status="429"}[%s])) by (path)`,
		namespace, ingressName, window)
	byPathResult, _ := i.query(byPathQuery, nil)
	for _, r := range byPathResult {
		if path, ok := r.Metric["path"]; ok {
			metrics.ByPath = append(metrics.ByPath, types.RateLimitByPath{
				Path:            path,
				LimitedRequests: r.Value,
			})
		}
	}

	// 趋势数据
	if timeRange != nil && !timeRange.Start.IsZero() && !timeRange.End.IsZero() {
		step := timeRange.Step
		if step == "" {
			step = i.calculateStep(timeRange.Start, timeRange.End)
		}

		trendResult, err := i.queryRange(limitedQuery, timeRange.Start, timeRange.End, step)
		if err == nil && len(trendResult) > 0 {
			metrics.Trend = i.parseRateLimitTrend(trendResult[0].Values)
		}
	}

	return metrics, nil
}

// ==================== 排行查询 ====================

// GetIngressRanking 获取 Ingress 排行
func (i *IngressOperator) GetIngressRanking(limit int, timeRange *types.TimeRange) (*types.IngressRanking, error) {
	ranking := &types.IngressRanking{
		TopByQPS:       []types.IngressRankingItem{},
		TopByErrorRate: []types.IngressRankingItem{},
		TopByLatency:   []types.IngressRankingItem{},
		TopByTraffic:   []types.IngressRankingItem{},
	}

	window := i.calculateRateWindow(timeRange)

	// Top by QPS
	qpsQuery := fmt.Sprintf(`topk(%d, sum(rate(nginx_ingress_controller_requests[%s])) by (namespace, ingress))`, limit, window)
	qpsResult, _ := i.query(qpsQuery, nil)
	for _, r := range qpsResult {
		namespace, _ := r.Metric["namespace"]
		ingressName, _ := r.Metric["ingress"]
		ranking.TopByQPS = append(ranking.TopByQPS, types.IngressRankingItem{
			Namespace:   namespace,
			IngressName: ingressName,
			Value:       r.Value,
			Unit:        "req/s",
		})
	}

	// Top by Error Rate
	errorRateQuery := fmt.Sprintf(
		`topk(%d, sum(rate(nginx_ingress_controller_requests{status=~"[4-5].."}[%s])) by (namespace, ingress) / sum(rate(nginx_ingress_controller_requests[%s])) by (namespace, ingress))`,
		limit, window, window)
	errorRateResult, _ := i.query(errorRateQuery, nil)
	for _, r := range errorRateResult {
		namespace, _ := r.Metric["namespace"]
		ingressName, _ := r.Metric["ingress"]
		ranking.TopByErrorRate = append(ranking.TopByErrorRate, types.IngressRankingItem{
			Namespace:   namespace,
			IngressName: ingressName,
			Value:       r.Value * 100,
			Unit:        "%",
		})
	}

	// Top by Latency (P95)
	latencyQuery := fmt.Sprintf(
		`topk(%d, histogram_quantile(0.95, sum(rate(nginx_ingress_controller_request_duration_seconds_bucket[%s])) by (namespace, ingress, le)))`,
		limit, window)
	latencyResult, _ := i.query(latencyQuery, nil)
	for _, r := range latencyResult {
		namespace, _ := r.Metric["namespace"]
		ingressName, _ := r.Metric["ingress"]
		ranking.TopByLatency = append(ranking.TopByLatency, types.IngressRankingItem{
			Namespace:   namespace,
			IngressName: ingressName,
			Value:       r.Value,
			Unit:        "seconds",
		})
	}

	// Top by Traffic
	trafficQuery := fmt.Sprintf(
		`topk(%d, sum(rate(nginx_ingress_controller_request_size_sum[%s]) + rate(nginx_ingress_controller_response_size_sum[%s])) by (namespace, ingress))`,
		limit, window, window)
	trafficResult, _ := i.query(trafficQuery, nil)
	for _, r := range trafficResult {
		namespace, _ := r.Metric["namespace"]
		ingressName, _ := r.Metric["ingress"]
		ranking.TopByTraffic = append(ranking.TopByTraffic, types.IngressRankingItem{
			Namespace:   namespace,
			IngressName: ingressName,
			Value:       r.Value,
			Unit:        "bytes/s",
		})
	}

	return ranking, nil
}

// GetPathRanking 获取 Path 排行
func (i *IngressOperator) GetPathRanking(limit int, timeRange *types.TimeRange) (*types.PathRanking, error) {
	ranking := &types.PathRanking{
		TopByQPS:       []types.PathRankingItem{},
		TopByErrorRate: []types.PathRankingItem{},
		TopByLatency:   []types.PathRankingItem{},
	}

	window := i.calculateRateWindow(timeRange)

	// Top by QPS
	qpsQuery := fmt.Sprintf(`topk(%d, sum(rate(nginx_ingress_controller_requests[%s])) by (host, path))`, limit, window)
	qpsResult, _ := i.query(qpsQuery, nil)
	for _, r := range qpsResult {
		host, _ := r.Metric["host"]
		path, _ := r.Metric["path"]
		ranking.TopByQPS = append(ranking.TopByQPS, types.PathRankingItem{
			Host:  host,
			Path:  path,
			Value: r.Value,
			Unit:  "req/s",
		})
	}

	// Top by Error Rate
	errorRateQuery := fmt.Sprintf(
		`topk(%d, sum(rate(nginx_ingress_controller_requests{status=~"[4-5].."}[%s])) by (host, path) / sum(rate(nginx_ingress_controller_requests[%s])) by (host, path))`,
		limit, window, window)
	errorRateResult, _ := i.query(errorRateQuery, nil)
	for _, r := range errorRateResult {
		host, _ := r.Metric["host"]
		path, _ := r.Metric["path"]
		ranking.TopByErrorRate = append(ranking.TopByErrorRate, types.PathRankingItem{
			Host:  host,
			Path:  path,
			Value: r.Value * 100,
			Unit:  "%",
		})
	}

	// Top by Latency
	latencyQuery := fmt.Sprintf(
		`topk(%d, histogram_quantile(0.95, sum(rate(nginx_ingress_controller_request_duration_seconds_bucket[%s])) by (host, path, le)))`,
		limit, window)
	latencyResult, _ := i.query(latencyQuery, nil)
	for _, r := range latencyResult {
		host, _ := r.Metric["host"]
		path, _ := r.Metric["path"]
		ranking.TopByLatency = append(ranking.TopByLatency, types.PathRankingItem{
			Host:  host,
			Path:  path,
			Value: r.Value,
			Unit:  "seconds",
		})
	}

	return ranking, nil
}

// GetHostRanking 获取 Host 排行
func (i *IngressOperator) GetHostRanking(limit int, timeRange *types.TimeRange) (*types.HostRanking, error) {
	ranking := &types.HostRanking{
		TopByQPS:       []types.HostRankingItem{},
		TopByErrorRate: []types.HostRankingItem{},
		TopByLatency:   []types.HostRankingItem{},
	}

	window := i.calculateRateWindow(timeRange)

	// Top by QPS
	qpsQuery := fmt.Sprintf(`topk(%d, sum(rate(nginx_ingress_controller_requests[%s])) by (host))`, limit, window)
	qpsResult, _ := i.query(qpsQuery, nil)
	for _, r := range qpsResult {
		host, _ := r.Metric["host"]
		ranking.TopByQPS = append(ranking.TopByQPS, types.HostRankingItem{
			Host:  host,
			Value: r.Value,
			Unit:  "req/s",
		})
	}

	// Top by Error Rate
	errorRateQuery := fmt.Sprintf(
		`topk(%d, sum(rate(nginx_ingress_controller_requests{status=~"[4-5].."}[%s])) by (host) / sum(rate(nginx_ingress_controller_requests[%s])) by (host))`,
		limit, window, window)
	errorRateResult, _ := i.query(errorRateQuery, nil)
	for _, r := range errorRateResult {
		host, _ := r.Metric["host"]
		ranking.TopByErrorRate = append(ranking.TopByErrorRate, types.HostRankingItem{
			Host:  host,
			Value: r.Value * 100,
			Unit:  "%",
		})
	}

	// Top by Latency
	latencyQuery := fmt.Sprintf(
		`topk(%d, histogram_quantile(0.95, sum(rate(nginx_ingress_controller_request_duration_seconds_bucket[%s])) by (host, le)))`,
		limit, window)
	latencyResult, _ := i.query(latencyQuery, nil)
	for _, r := range latencyResult {
		host, _ := r.Metric["host"]
		ranking.TopByLatency = append(ranking.TopByLatency, types.HostRankingItem{
			Host:  host,
			Value: r.Value,
			Unit:  "seconds",
		})
	}

	return ranking, nil
}

// ==================== 列表查询 ====================

// ListIngressMetrics 列出命名空间下的所有 Ingress
func (i *IngressOperator) ListIngressMetrics(namespace string, timeRange *types.TimeRange) ([]types.IngressMetrics, error) {
	metrics := []types.IngressMetrics{}

	// 查询命名空间下的所有 Ingress
	ingressQuery := fmt.Sprintf(`nginx_ingress_controller_ingress_upstream_latency_seconds{namespace="%s"}`, namespace)
	ingressResult, _ := i.query(ingressQuery, nil)

	// 去重获取 ingress 名称
	ingressNames := make(map[string]bool)
	for _, r := range ingressResult {
		if ingressName, ok := r.Metric["ingress"]; ok {
			ingressNames[ingressName] = true
		}
	}

	// 获取每个 Ingress 的指标
	for ingressName := range ingressNames {
		metric, err := i.GetIngressMetrics(namespace, ingressName, timeRange)
		if err != nil {
			i.log.Errorf("获取 Ingress 指标失败: ingress=%s, error=%v", ingressName, err)
			continue
		}
		metrics = append(metrics, *metric)
	}

	return metrics, nil
}

// query 即时查询
func (i *IngressOperator) query(query string, timestamp *time.Time) ([]types.InstantQueryResult, error) {
	params := map[string]string{"query": query}
	if timestamp != nil {
		params["time"] = i.formatTimestamp(*timestamp)
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

	if err := i.doRequest("GET", "/api/v1/query", params, nil, &response); err != nil {
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
func (i *IngressOperator) queryRange(query string, start, end time.Time, step string) ([]types.RangeQueryResult, error) {
	params := map[string]string{
		"query": query,
		"start": i.formatTimestamp(start),
		"end":   i.formatTimestamp(end),
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

	if err := i.doRequest("GET", "/api/v1/query_range", params, nil, &response); err != nil {
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

// 解析趋势数据的辅助方法
func (i *IngressOperator) parseControllerTrend(values []types.MetricValue) []types.IngressControllerDataPoint {
	dataPoints := make([]types.IngressControllerDataPoint, 0, len(values))
	for _, v := range values {
		dataPoints = append(dataPoints, types.IngressControllerDataPoint{
			Timestamp:         v.Timestamp,
			ActiveConnections: int64(v.Value),
		})
	}
	return dataPoints
}

func (i *IngressOperator) parseTrafficTrend(values []types.MetricValue) []types.IngressTrafficDataPoint {
	dataPoints := make([]types.IngressTrafficDataPoint, 0, len(values))
	for _, v := range values {
		dataPoints = append(dataPoints, types.IngressTrafficDataPoint{
			Timestamp:         v.Timestamp,
			RequestsPerSecond: v.Value,
		})
	}
	return dataPoints
}

func (i *IngressOperator) parseRateLimitTrend(values []types.MetricValue) []types.IngressRateLimitDataPoint {
	dataPoints := make([]types.IngressRateLimitDataPoint, 0, len(values))
	for _, v := range values {
		dataPoints = append(dataPoints, types.IngressRateLimitDataPoint{
			Timestamp:       v.Timestamp,
			LimitedRequests: v.Value,
		})
	}
	return dataPoints
}

func (i *IngressOperator) calculateTrafficSummary(traffic *types.IngressTrafficMetrics) types.IngressTrafficSummary {
	summary := types.IngressTrafficSummary{}

	if len(traffic.Trend) > 0 {
		var sum, max float64
		for _, point := range traffic.Trend {
			sum += point.RequestsPerSecond
			if point.RequestsPerSecond > max {
				max = point.RequestsPerSecond
			}
		}
		summary.AvgRequestsPerSec = sum / float64(len(traffic.Trend))
		summary.MaxRequestsPerSec = max
	}

	return summary
}
