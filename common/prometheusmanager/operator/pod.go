package operator

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/yanshicheng/kube-nova/common/prometheusmanager/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type PodOperatorImpl struct {
	log logx.Logger
	ctx context.Context
	*BaseOperator
}

func NewPodOperator(ctx context.Context, base *BaseOperator) types.PodOperator {
	return &PodOperatorImpl{
		log:          logx.WithContext(ctx),
		ctx:          ctx,
		BaseOperator: base,
	}
}

// ==================== CPU 相关方法 ====================

// GetCPUUsage 获取 Pod CPU 使用情况
func (p *PodOperatorImpl) GetCPUUsage(namespace, pod string, timeRange *types.TimeRange) (*types.PodCPUMetrics, error) {
	p.log.Infof("📊 查询 Pod CPU: namespace=%s, pod=%s", namespace, pod)

	metrics := &types.PodCPUMetrics{
		Namespace: namespace,
		PodName:   pod,
	}

	window := p.calculateRateWindow(timeRange)

	// 并发查询所有指标
	var wg sync.WaitGroup
	var mu sync.Mutex
	wg.Add(4)

	// 1. 当前 CPU 使用
	go func() {
		defer wg.Done()
		currentQuery := fmt.Sprintf(`sum(rate(container_cpu_usage_seconds_total{namespace="%s",pod="%s",container!="",container!="POD"}[%s]))`, namespace, pod, window)
		if results, err := p.query(currentQuery, nil); err == nil && len(results) > 0 {
			mu.Lock()
			metrics.Current.UsageCores = results[0].Value
			metrics.Current.Timestamp = results[0].Time
			mu.Unlock()
		}
	}()

	// 2. CPU Request
	go func() {
		defer wg.Done()
		requestQuery := fmt.Sprintf(`sum(kube_pod_container_resource_requests{namespace="%s",pod="%s",resource="cpu"})`, namespace, pod)
		if results, err := p.query(requestQuery, nil); err == nil && len(results) > 0 {
			mu.Lock()
			metrics.Current.RequestCores = results[0].Value
			metrics.Limits.RequestCores = results[0].Value
			mu.Unlock()
		}
	}()

	// 3. CPU Limit
	go func() {
		defer wg.Done()
		limitQuery := fmt.Sprintf(`sum(kube_pod_container_resource_limits{namespace="%s",pod="%s",resource="cpu"})`, namespace, pod)
		if results, err := p.query(limitQuery, nil); err == nil && len(results) > 0 {
			mu.Lock()
			metrics.Current.LimitCores = results[0].Value
			metrics.Limits.LimitCores = results[0].Value
			metrics.Limits.HasLimit = true
			mu.Unlock()
		}
	}()

	// 4. CPU 节流
	go func() {
		defer wg.Done()
		throttleQuery := fmt.Sprintf(`sum(rate(container_cpu_cfs_throttled_seconds_total{namespace="%s",pod="%s",container!="",container!="POD"}[%s]))`, namespace, pod, window)
		if results, err := p.query(throttleQuery, nil); err == nil && len(results) > 0 {
			mu.Lock()
			metrics.Current.ThrottledTime = results[0].Value
			mu.Unlock()
		}
	}()

	wg.Wait()

	// 计算使用率
	if metrics.Limits.LimitCores > 0 {
		metrics.Current.UsagePercent = (metrics.Current.UsageCores / metrics.Limits.LimitCores) * 100
	} else if metrics.Limits.RequestCores > 0 {
		metrics.Current.UsagePercent = (metrics.Current.UsageCores / metrics.Limits.RequestCores) * 100
	}

	// 趋势数据
	if timeRange != nil && !timeRange.Start.IsZero() && !timeRange.End.IsZero() {
		step := timeRange.Step
		if step == "" {
			step = p.calculateStep(timeRange.Start, timeRange.End)
		}

		trendQuery := fmt.Sprintf(`sum(rate(container_cpu_usage_seconds_total{namespace="%s",pod="%s",container!="",container!="POD"}[%s]))`, namespace, pod, window)
		if trendResults, err := p.queryRange(trendQuery, timeRange.Start, timeRange.End, step); err == nil && len(trendResults) > 0 && len(trendResults[0].Values) > 0 {
			metrics.Trend = make([]types.CPUUsageDataPoint, 0, len(trendResults[0].Values))

			limitCores := metrics.Limits.LimitCores
			if limitCores == 0 {
				limitCores = metrics.Limits.RequestCores
			}

			for _, v := range trendResults[0].Values {
				usagePercent := 0.0
				if limitCores > 0 {
					usagePercent = (v.Value / limitCores) * 100
				}

				metrics.Trend = append(metrics.Trend, types.CPUUsageDataPoint{
					Timestamp:    v.Timestamp,
					UsageCores:   v.Value,
					UsagePercent: usagePercent,
				})
			}

			metrics.Summary = p.calculateCPUSummary(metrics.Trend, metrics.Current.ThrottledTime)
		}
	}

	return metrics, nil
}

// GetCPUUsageByContainer 获取容器 CPU 使用情况
func (p *PodOperatorImpl) GetCPUUsageByContainer(namespace, pod, container string, timeRange *types.TimeRange) (*types.ContainerCPUMetrics, error) {
	p.log.Infof("📊 查询容器 CPU: namespace=%s, pod=%s, container=%s", namespace, pod, container)

	metrics := &types.ContainerCPUMetrics{
		Namespace:     namespace,
		PodName:       pod,
		ContainerName: container,
	}

	window := p.calculateRateWindow(timeRange)

	// 并发查询
	var wg sync.WaitGroup
	var mu sync.Mutex
	wg.Add(4)

	go func() {
		defer wg.Done()
		currentQuery := fmt.Sprintf(`rate(container_cpu_usage_seconds_total{namespace="%s",pod="%s",container="%s"}[%s])`, namespace, pod, container, window)
		if results, err := p.query(currentQuery, nil); err == nil && len(results) > 0 {
			mu.Lock()
			metrics.Current.UsageCores = results[0].Value
			metrics.Current.Timestamp = results[0].Time
			mu.Unlock()
		}
	}()

	go func() {
		defer wg.Done()
		requestQuery := fmt.Sprintf(`kube_pod_container_resource_requests{namespace="%s",pod="%s",container="%s",resource="cpu"}`, namespace, pod, container)
		if results, err := p.query(requestQuery, nil); err == nil && len(results) > 0 {
			mu.Lock()
			metrics.Current.RequestCores = results[0].Value
			metrics.Limits.RequestCores = results[0].Value
			mu.Unlock()
		}
	}()

	go func() {
		defer wg.Done()
		limitQuery := fmt.Sprintf(`kube_pod_container_resource_limits{namespace="%s",pod="%s",container="%s",resource="cpu"}`, namespace, pod, container)
		if results, err := p.query(limitQuery, nil); err == nil && len(results) > 0 {
			mu.Lock()
			metrics.Current.LimitCores = results[0].Value
			metrics.Limits.LimitCores = results[0].Value
			metrics.Limits.HasLimit = true
			mu.Unlock()
		}
	}()

	go func() {
		defer wg.Done()
		throttleQuery := fmt.Sprintf(`rate(container_cpu_cfs_throttled_seconds_total{namespace="%s",pod="%s",container="%s"}[%s])`, namespace, pod, container, window)
		if results, err := p.query(throttleQuery, nil); err == nil && len(results) > 0 {
			mu.Lock()
			metrics.Current.ThrottledTime = results[0].Value
			mu.Unlock()
		}
	}()

	wg.Wait()

	// 计算使用率
	if metrics.Limits.LimitCores > 0 {
		metrics.Current.UsagePercent = (metrics.Current.UsageCores / metrics.Limits.LimitCores) * 100
	} else if metrics.Limits.RequestCores > 0 {
		metrics.Current.UsagePercent = (metrics.Current.UsageCores / metrics.Limits.RequestCores) * 100
	}

	// 趋势数据
	if timeRange != nil && !timeRange.Start.IsZero() && !timeRange.End.IsZero() {
		step := timeRange.Step
		if step == "" {
			step = p.calculateStep(timeRange.Start, timeRange.End)
		}

		trendQuery := fmt.Sprintf(`rate(container_cpu_usage_seconds_total{namespace="%s",pod="%s",container="%s"}[%s])`, namespace, pod, container, window)
		if trendResults, err := p.queryRange(trendQuery, timeRange.Start, timeRange.End, step); err == nil && len(trendResults) > 0 && len(trendResults[0].Values) > 0 {
			metrics.Trend = make([]types.CPUUsageDataPoint, 0, len(trendResults[0].Values))

			limitCores := metrics.Limits.LimitCores
			if limitCores == 0 {
				limitCores = metrics.Limits.RequestCores
			}

			for _, v := range trendResults[0].Values {
				usagePercent := 0.0
				if limitCores > 0 {
					usagePercent = (v.Value / limitCores) * 100
				}

				metrics.Trend = append(metrics.Trend, types.CPUUsageDataPoint{
					Timestamp:    v.Timestamp,
					UsageCores:   v.Value,
					UsagePercent: usagePercent,
				})
			}

			metrics.Summary = p.calculateCPUSummary(metrics.Trend, metrics.Current.ThrottledTime)
		}
	}

	return metrics, nil
}

// GetCPUThrottling 获取 CPU 节流情况
func (p *PodOperatorImpl) GetCPUThrottling(namespace, pod string, timeRange *types.TimeRange) (*types.CPUThrottlingMetrics, error) {
	metrics := &types.CPUThrottlingMetrics{
		Namespace: namespace,
		PodName:   pod,
	}

	window := p.calculateRateWindow(timeRange)

	// 并发查询
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		throttleQuery := fmt.Sprintf(`sum(rate(container_cpu_cfs_throttled_seconds_total{namespace="%s",pod="%s",container!="",container!="POD"}[%s]))`, namespace, pod, window)
		if results, err := p.query(throttleQuery, nil); err == nil && len(results) > 0 {
			metrics.TotalThrottled = results[0].Value
		}
	}()

	go func() {
		defer wg.Done()
		containerQuery := fmt.Sprintf(`sum by (container) (rate(container_cpu_cfs_throttled_seconds_total{namespace="%s",pod="%s",container!="",container!="POD"}[%s]))`, namespace, pod, window)
		if results, err := p.query(containerQuery, nil); err == nil {
			metrics.ByContainer = make([]types.ContainerThrottling, 0, len(results))
			for _, result := range results {
				if containerName, ok := result.Metric["container"]; ok {
					metrics.ByContainer = append(metrics.ByContainer, types.ContainerThrottling{
						ContainerName:    containerName,
						ThrottledSeconds: result.Value,
					})
				}
			}
		}
	}()

	wg.Wait()

	// 计算节流百分比
	usageQuery := fmt.Sprintf(`sum(rate(container_cpu_usage_seconds_total{namespace="%s",pod="%s",container!="",container!="POD"}[%s]))`, namespace, pod, window)
	if usageResults, err := p.query(usageQuery, nil); err == nil && len(usageResults) > 0 && usageResults[0].Value > 0 {
		metrics.ThrottledPercent = (metrics.TotalThrottled / usageResults[0].Value) * 100
	}

	// 趋势数据
	if timeRange != nil && !timeRange.Start.IsZero() && !timeRange.End.IsZero() {
		step := timeRange.Step
		if step == "" {
			step = p.calculateStep(timeRange.Start, timeRange.End)
		}

		throttleQuery := fmt.Sprintf(`sum(rate(container_cpu_cfs_throttled_seconds_total{namespace="%s",pod="%s",container!="",container!="POD"}[%s]))`, namespace, pod, window)
		if trendResults, err := p.queryRange(throttleQuery, timeRange.Start, timeRange.End, step); err == nil && len(trendResults) > 0 && len(trendResults[0].Values) > 0 {
			metrics.Trend = make([]types.CPUThrottlingDataPoint, 0, len(trendResults[0].Values))
			for _, v := range trendResults[0].Values {
				metrics.Trend = append(metrics.Trend, types.CPUThrottlingDataPoint{
					Timestamp:        v.Timestamp,
					ThrottledSeconds: v.Value,
				})
			}
		}
	}

	return metrics, nil
}

// ==================== 内存相关方法 ====================

// GetMemoryUsage 获取 Pod 内存使用情况
func (p *PodOperatorImpl) GetMemoryUsage(namespace, pod string, timeRange *types.TimeRange) (*types.PodMemoryMetrics, error) {
	p.log.Infof("📊 查询 Pod 内存: namespace=%s, pod=%s", namespace, pod)

	metrics := &types.PodMemoryMetrics{
		Namespace: namespace,
		PodName:   pod,
	}

	// 并发查询所有指标
	var wg sync.WaitGroup
	var mu sync.Mutex
	wg.Add(5)

	// 1. WorkingSet 内存
	go func() {
		defer wg.Done()
		currentQuery := fmt.Sprintf(`sum(container_memory_working_set_bytes{namespace="%s",pod="%s",container!="",container!="POD"})`, namespace, pod)
		if results, err := p.query(currentQuery, nil); err == nil && len(results) > 0 {
			mu.Lock()
			metrics.Current.WorkingSetBytes = int64(results[0].Value)
			metrics.Current.UsageBytes = int64(results[0].Value)
			metrics.Current.Timestamp = results[0].Time
			mu.Unlock()
		}
	}()

	// 2. RSS 内存
	go func() {
		defer wg.Done()
		rssQuery := fmt.Sprintf(`sum(container_memory_rss{namespace="%s",pod="%s",container!="",container!="POD"})`, namespace, pod)
		if results, err := p.query(rssQuery, nil); err == nil && len(results) > 0 {
			mu.Lock()
			metrics.Current.RSSBytes = int64(results[0].Value)
			mu.Unlock()
		}
	}()

	// 3. Cache 内存
	go func() {
		defer wg.Done()
		cacheQuery := fmt.Sprintf(`sum(container_memory_cache{namespace="%s",pod="%s",container!="",container!="POD"})`, namespace, pod)
		if results, err := p.query(cacheQuery, nil); err == nil && len(results) > 0 {
			mu.Lock()
			metrics.Current.CacheBytes = int64(results[0].Value)
			mu.Unlock()
		}
	}()

	// 4. Memory Request
	go func() {
		defer wg.Done()
		requestQuery := fmt.Sprintf(`sum(kube_pod_container_resource_requests{namespace="%s",pod="%s",resource="memory"})`, namespace, pod)
		if results, err := p.query(requestQuery, nil); err == nil && len(results) > 0 {
			mu.Lock()
			metrics.Current.RequestBytes = int64(results[0].Value)
			metrics.Limits.RequestBytes = int64(results[0].Value)
			mu.Unlock()
		}
	}()

	// 5. Memory Limit
	go func() {
		defer wg.Done()
		limitQuery := fmt.Sprintf(`sum(kube_pod_container_resource_limits{namespace="%s",pod="%s",resource="memory"})`, namespace, pod)
		if results, err := p.query(limitQuery, nil); err == nil && len(results) > 0 {
			mu.Lock()
			metrics.Current.LimitBytes = int64(results[0].Value)
			metrics.Limits.LimitBytes = int64(results[0].Value)
			metrics.Limits.HasLimit = true
			mu.Unlock()
		}
	}()

	wg.Wait()

	// 计算使用率
	if metrics.Limits.LimitBytes > 0 {
		metrics.Current.UsagePercent = (float64(metrics.Current.UsageBytes) / float64(metrics.Limits.LimitBytes)) * 100
	} else if metrics.Limits.RequestBytes > 0 {
		metrics.Current.UsagePercent = (float64(metrics.Current.UsageBytes) / float64(metrics.Limits.RequestBytes)) * 100
	}

	// 趋势数据
	if timeRange != nil && !timeRange.Start.IsZero() && !timeRange.End.IsZero() {
		step := timeRange.Step
		if step == "" {
			step = p.calculateStep(timeRange.Start, timeRange.End)
		}

		trendQuery := fmt.Sprintf(`sum(container_memory_working_set_bytes{namespace="%s",pod="%s",container!="",container!="POD"})`, namespace, pod)
		if trendResults, err := p.queryRange(trendQuery, timeRange.Start, timeRange.End, step); err == nil && len(trendResults) > 0 && len(trendResults[0].Values) > 0 {
			metrics.Trend = make([]types.MemoryUsageDataPoint, 0, len(trendResults[0].Values))

			limitBytes := metrics.Limits.LimitBytes
			if limitBytes == 0 {
				limitBytes = metrics.Limits.RequestBytes
			}

			for _, v := range trendResults[0].Values {
				usageBytes := int64(v.Value)
				usagePercent := 0.0
				if limitBytes > 0 {
					usagePercent = (float64(usageBytes) / float64(limitBytes)) * 100
				}

				metrics.Trend = append(metrics.Trend, types.MemoryUsageDataPoint{
					Timestamp:       v.Timestamp,
					UsageBytes:      usageBytes,
					UsagePercent:    usagePercent,
					WorkingSetBytes: usageBytes,
				})
			}

			metrics.Summary = p.calculateMemorySummary(metrics.Trend)
		}
	}

	return metrics, nil
}

// GetMemoryUsageByContainer 获取容器内存使用情况
func (p *PodOperatorImpl) GetMemoryUsageByContainer(namespace, pod, container string, timeRange *types.TimeRange) (*types.ContainerMemoryMetrics, error) {
	p.log.Infof("📊 查询容器内存: namespace=%s, pod=%s, container=%s", namespace, pod, container)

	metrics := &types.ContainerMemoryMetrics{
		Namespace:     namespace,
		PodName:       pod,
		ContainerName: container,
	}

	// 并发查询
	var wg sync.WaitGroup
	var mu sync.Mutex
	wg.Add(5)

	go func() {
		defer wg.Done()
		currentQuery := fmt.Sprintf(`container_memory_working_set_bytes{namespace="%s",pod="%s",container="%s"}`, namespace, pod, container)
		if results, err := p.query(currentQuery, nil); err == nil && len(results) > 0 {
			mu.Lock()
			metrics.Current.WorkingSetBytes = int64(results[0].Value)
			metrics.Current.UsageBytes = int64(results[0].Value)
			metrics.Current.Timestamp = results[0].Time
			mu.Unlock()
		}
	}()

	go func() {
		defer wg.Done()
		rssQuery := fmt.Sprintf(`container_memory_rss{namespace="%s",pod="%s",container="%s"}`, namespace, pod, container)
		if results, err := p.query(rssQuery, nil); err == nil && len(results) > 0 {
			mu.Lock()
			metrics.Current.RSSBytes = int64(results[0].Value)
			mu.Unlock()
		}
	}()

	go func() {
		defer wg.Done()
		cacheQuery := fmt.Sprintf(`container_memory_cache{namespace="%s",pod="%s",container="%s"}`, namespace, pod, container)
		if results, err := p.query(cacheQuery, nil); err == nil && len(results) > 0 {
			mu.Lock()
			metrics.Current.CacheBytes = int64(results[0].Value)
			mu.Unlock()
		}
	}()

	go func() {
		defer wg.Done()
		requestQuery := fmt.Sprintf(`kube_pod_container_resource_requests{namespace="%s",pod="%s",container="%s",resource="memory"}`, namespace, pod, container)
		if results, err := p.query(requestQuery, nil); err == nil && len(results) > 0 {
			mu.Lock()
			metrics.Current.RequestBytes = int64(results[0].Value)
			metrics.Limits.RequestBytes = int64(results[0].Value)
			mu.Unlock()
		}
	}()

	go func() {
		defer wg.Done()
		limitQuery := fmt.Sprintf(`kube_pod_container_resource_limits{namespace="%s",pod="%s",container="%s",resource="memory"}`, namespace, pod, container)
		if results, err := p.query(limitQuery, nil); err == nil && len(results) > 0 {
			mu.Lock()
			metrics.Current.LimitBytes = int64(results[0].Value)
			metrics.Limits.LimitBytes = int64(results[0].Value)
			metrics.Limits.HasLimit = true
			mu.Unlock()
		}
	}()

	wg.Wait()

	// 计算使用率
	if metrics.Limits.LimitBytes > 0 {
		metrics.Current.UsagePercent = (float64(metrics.Current.UsageBytes) / float64(metrics.Limits.LimitBytes)) * 100
	} else if metrics.Limits.RequestBytes > 0 {
		metrics.Current.UsagePercent = (float64(metrics.Current.UsageBytes) / float64(metrics.Limits.RequestBytes)) * 100
	}

	// 趋势数据
	if timeRange != nil && !timeRange.Start.IsZero() && !timeRange.End.IsZero() {
		step := timeRange.Step
		if step == "" {
			step = p.calculateStep(timeRange.Start, timeRange.End)
		}

		trendQuery := fmt.Sprintf(`container_memory_working_set_bytes{namespace="%s",pod="%s",container="%s"}`, namespace, pod, container)
		if trendResults, err := p.queryRange(trendQuery, timeRange.Start, timeRange.End, step); err == nil && len(trendResults) > 0 && len(trendResults[0].Values) > 0 {
			metrics.Trend = make([]types.MemoryUsageDataPoint, 0, len(trendResults[0].Values))

			limitBytes := metrics.Limits.LimitBytes
			if limitBytes == 0 {
				limitBytes = metrics.Limits.RequestBytes
			}

			for _, v := range trendResults[0].Values {
				usageBytes := int64(v.Value)
				usagePercent := 0.0
				if limitBytes > 0 {
					usagePercent = (float64(usageBytes) / float64(limitBytes)) * 100
				}

				metrics.Trend = append(metrics.Trend, types.MemoryUsageDataPoint{
					Timestamp:       v.Timestamp,
					UsageBytes:      usageBytes,
					UsagePercent:    usagePercent,
					WorkingSetBytes: usageBytes,
				})
			}

			metrics.Summary = p.calculateMemorySummary(metrics.Trend)
		}
	}

	return metrics, nil
}

// GetMemoryOOM 获取 OOM 情况
func (p *PodOperatorImpl) GetMemoryOOM(namespace, pod string, timeRange *types.TimeRange) (*types.OOMMetrics, error) {
	metrics := &types.OOMMetrics{
		Namespace: namespace,
		PodName:   pod,
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		oomQuery := fmt.Sprintf(`sum(container_oom_events_total{namespace="%s",pod="%s"})`, namespace, pod)
		if results, err := p.query(oomQuery, nil); err == nil && len(results) > 0 {
			metrics.TotalOOMKills = int64(results[0].Value)
		}
	}()

	go func() {
		defer wg.Done()
		containerOOMQuery := fmt.Sprintf(`sum by (container) (container_oom_events_total{namespace="%s",pod="%s"})`, namespace, pod)
		if results, err := p.query(containerOOMQuery, nil); err == nil {
			metrics.ByContainer = make([]types.ContainerOOM, 0, len(results))
			for _, result := range results {
				if containerName, ok := result.Metric["container"]; ok && containerName != "" && containerName != "POD" {
					metrics.ByContainer = append(metrics.ByContainer, types.ContainerOOM{
						ContainerName: containerName,
						OOMKills:      int64(result.Value),
					})
				}
			}
		}
	}()

	wg.Wait()
	return metrics, nil
}

// ==================== 网络相关方法 ====================

// GetNetworkIO 获取 Pod 网络 I/O
func (p *PodOperatorImpl) GetNetworkIO(namespace, pod string, timeRange *types.TimeRange) (*types.NetworkMetrics, error) {
	metrics := &types.NetworkMetrics{
		Namespace: namespace,
		PodName:   pod,
	}

	// 并发查询当前累计值
	var wg sync.WaitGroup
	var mu sync.Mutex
	wg.Add(6)

	go func() {
		defer wg.Done()
		query := fmt.Sprintf(`sum(container_network_receive_bytes_total{pod="%s"})`, pod)
		if results, err := p.query(query, nil); err == nil && len(results) > 0 {
			mu.Lock()
			metrics.Current.ReceiveBytes = int64(results[0].Value)
			metrics.Current.Timestamp = results[0].Time
			mu.Unlock()
		}
	}()

	go func() {
		defer wg.Done()
		query := fmt.Sprintf(`sum(container_network_transmit_bytes_total{pod="%s"})`, pod)
		if results, err := p.query(query, nil); err == nil && len(results) > 0 {
			mu.Lock()
			metrics.Current.TransmitBytes = int64(results[0].Value)
			mu.Unlock()
		}
	}()

	go func() {
		defer wg.Done()
		query := fmt.Sprintf(`sum(container_network_receive_packets_total{pod="%s"})`, pod)
		if results, err := p.query(query, nil); err == nil && len(results) > 0 {
			mu.Lock()
			metrics.Current.ReceivePackets = int64(results[0].Value)
			mu.Unlock()
		}
	}()

	go func() {
		defer wg.Done()
		query := fmt.Sprintf(`sum(container_network_transmit_packets_total{pod="%s"})`, pod)
		if results, err := p.query(query, nil); err == nil && len(results) > 0 {
			mu.Lock()
			metrics.Current.TransmitPackets = int64(results[0].Value)
			mu.Unlock()
		}
	}()

	go func() {
		defer wg.Done()
		query := fmt.Sprintf(`sum(container_network_receive_errors_total{pod="%s"})`, pod)
		if results, err := p.query(query, nil); err == nil && len(results) > 0 {
			mu.Lock()
			metrics.Current.ReceiveErrors = int64(results[0].Value)
			mu.Unlock()
		}
	}()

	go func() {
		defer wg.Done()
		query := fmt.Sprintf(`sum(container_network_transmit_errors_total{pod="%s"})`, pod)
		if results, err := p.query(query, nil); err == nil && len(results) > 0 {
			mu.Lock()
			metrics.Current.TransmitErrors = int64(results[0].Value)
			mu.Unlock()
		}
	}()

	wg.Wait()

	// 趋势数据（速率）
	if timeRange != nil && !timeRange.Start.IsZero() && !timeRange.End.IsZero() {
		step := timeRange.Step
		if step == "" {
			step = p.calculateStep(timeRange.Start, timeRange.End)
		}

		window := p.calculateRateWindow(timeRange)

		var trendWg sync.WaitGroup
		trendWg.Add(2)

		go func() {
			defer trendWg.Done()
			receiveRateQuery := fmt.Sprintf(`sum(rate(container_network_receive_bytes_total{pod="%s"}[%s]))`, pod, window)
			if receiveTrend, err := p.queryRange(receiveRateQuery, timeRange.Start, timeRange.End, step); err == nil && len(receiveTrend) > 0 && len(receiveTrend[0].Values) > 0 {
				mu.Lock()
				metrics.Trend = make([]types.NetworkDataPoint, len(receiveTrend[0].Values))
				for i, v := range receiveTrend[0].Values {
					metrics.Trend[i] = types.NetworkDataPoint{
						Timestamp:    v.Timestamp,
						ReceiveBytes: int64(v.Value),
					}
				}
				mu.Unlock()
			}
		}()

		go func() {
			defer trendWg.Done()
			transmitRateQuery := fmt.Sprintf(`sum(rate(container_network_transmit_bytes_total{pod="%s"}[%s]))`, pod, window)
			if transmitTrend, err := p.queryRange(transmitRateQuery, timeRange.Start, timeRange.End, step); err == nil && len(transmitTrend) > 0 && len(transmitTrend[0].Values) > 0 {
				mu.Lock()
				for i, v := range transmitTrend[0].Values {
					if i < len(metrics.Trend) {
						metrics.Trend[i].TransmitBytes = int64(v.Value)
					}
				}
				mu.Unlock()
			}
		}()

		trendWg.Wait()

		// 计算 Summary
		if len(metrics.Trend) > 0 {
			var maxReceive, maxTransmit, sumReceive, sumTransmit int64
			for _, point := range metrics.Trend {
				if point.ReceiveBytes > maxReceive {
					maxReceive = point.ReceiveBytes
				}
				if point.TransmitBytes > maxTransmit {
					maxTransmit = point.TransmitBytes
				}
				sumReceive += point.ReceiveBytes
				sumTransmit += point.TransmitBytes
			}

			metrics.Summary.MaxReceiveBytesPerSec = maxReceive
			metrics.Summary.MaxTransmitBytesPerSec = maxTransmit
			metrics.Summary.AvgReceiveBytesPerSec = sumReceive / int64(len(metrics.Trend))
			metrics.Summary.AvgTransmitBytesPerSec = sumTransmit / int64(len(metrics.Trend))
		}
	}

	// 总量
	metrics.Summary.TotalReceiveBytes = metrics.Current.ReceiveBytes
	metrics.Summary.TotalTransmitBytes = metrics.Current.TransmitBytes
	metrics.Summary.TotalReceivePackets = metrics.Current.ReceivePackets
	metrics.Summary.TotalTransmitPackets = metrics.Current.TransmitPackets
	metrics.Summary.TotalErrors = metrics.Current.ReceiveErrors + metrics.Current.TransmitErrors

	return metrics, nil
}

// GetNetworkIOByContainer 获取容器网络 I/O
func (p *PodOperatorImpl) GetNetworkIOByContainer(namespace, pod, container string, timeRange *types.TimeRange) (*types.ContainerNetworkMetrics, error) {
	metrics := &types.ContainerNetworkMetrics{
		Namespace:     namespace,
		PodName:       pod,
		ContainerName: container,
	}

	// 注意：网络指标通常只在 Pod 级别，容器级别可能不可用
	// 这里尝试容器级别，失败则回退到 Pod 级别

	var wg sync.WaitGroup
	var mu sync.Mutex
	wg.Add(6)

	go func() {
		defer wg.Done()
		query := fmt.Sprintf(`container_network_receive_bytes_total{pod="%s",container="%s"}`, pod, container)
		results, err := p.query(query, nil)
		if err != nil || len(results) == 0 {
			// 回退到 Pod 级别
			query = fmt.Sprintf(`sum(container_network_receive_bytes_total{pod="%s"})`, pod)
			results, err = p.query(query, nil)
		}
		if err == nil && len(results) > 0 {
			mu.Lock()
			metrics.Current.ReceiveBytes = int64(results[0].Value)
			metrics.Current.Timestamp = results[0].Time
			mu.Unlock()
		}
	}()

	go func() {
		defer wg.Done()
		query := fmt.Sprintf(`container_network_transmit_bytes_total{pod="%s",container="%s"}`, pod, container)
		results, err := p.query(query, nil)
		if err != nil || len(results) == 0 {
			query = fmt.Sprintf(`sum(container_network_transmit_bytes_total{pod="%s"})`, pod)
			results, err = p.query(query, nil)
		}
		if err == nil && len(results) > 0 {
			mu.Lock()
			metrics.Current.TransmitBytes = int64(results[0].Value)
			mu.Unlock()
		}
	}()

	go func() {
		defer wg.Done()
		query := fmt.Sprintf(`sum(container_network_receive_packets_total{pod="%s"})`, pod)
		if results, err := p.query(query, nil); err == nil && len(results) > 0 {
			mu.Lock()
			metrics.Current.ReceivePackets = int64(results[0].Value)
			mu.Unlock()
		}
	}()

	go func() {
		defer wg.Done()
		query := fmt.Sprintf(`sum(container_network_transmit_packets_total{pod="%s"})`, pod)
		if results, err := p.query(query, nil); err == nil && len(results) > 0 {
			mu.Lock()
			metrics.Current.TransmitPackets = int64(results[0].Value)
			mu.Unlock()
		}
	}()

	go func() {
		defer wg.Done()
		query := fmt.Sprintf(`sum(container_network_receive_errors_total{pod="%s"})`, pod)
		if results, err := p.query(query, nil); err == nil && len(results) > 0 {
			mu.Lock()
			metrics.Current.ReceiveErrors = int64(results[0].Value)
			mu.Unlock()
		}
	}()

	go func() {
		defer wg.Done()
		query := fmt.Sprintf(`sum(container_network_transmit_errors_total{pod="%s"})`, pod)
		if results, err := p.query(query, nil); err == nil && len(results) > 0 {
			mu.Lock()
			metrics.Current.TransmitErrors = int64(results[0].Value)
			mu.Unlock()
		}
	}()

	wg.Wait()

	metrics.Summary.TotalReceivePackets = metrics.Current.ReceivePackets
	metrics.Summary.TotalTransmitPackets = metrics.Current.TransmitPackets
	metrics.Summary.TotalErrors = metrics.Current.ReceiveErrors + metrics.Current.TransmitErrors

	return metrics, nil
}

// GetNetworkRate 获取 Pod 网络速率
func (p *PodOperatorImpl) GetNetworkRate(namespace, pod string, timeRange *types.TimeRange) (*types.NetworkRateMetrics, error) {
	metrics := &types.NetworkRateMetrics{
		Namespace: namespace,
		PodName:   pod,
	}

	window := p.calculateRateWindow(timeRange)

	var wg sync.WaitGroup
	var mu sync.Mutex
	wg.Add(2)

	go func() {
		defer wg.Done()
		receiveRateQuery := fmt.Sprintf(`sum(rate(container_network_receive_bytes_total{pod="%s"}[%s]))`, pod, window)
		if results, err := p.query(receiveRateQuery, nil); err == nil && len(results) > 0 {
			mu.Lock()
			metrics.Current.ReceiveBytesPerSec = results[0].Value
			metrics.Current.Timestamp = results[0].Time
			mu.Unlock()
		}
	}()

	go func() {
		defer wg.Done()
		transmitRateQuery := fmt.Sprintf(`sum(rate(container_network_transmit_bytes_total{pod="%s"}[%s]))`, pod, window)
		if results, err := p.query(transmitRateQuery, nil); err == nil && len(results) > 0 {
			mu.Lock()
			metrics.Current.TransmitBytesPerSec = results[0].Value
			mu.Unlock()
		}
	}()

	wg.Wait()

	if timeRange != nil && !timeRange.Start.IsZero() && !timeRange.End.IsZero() {
		step := timeRange.Step
		if step == "" {
			step = p.calculateStep(timeRange.Start, timeRange.End)
		}

		receiveRateQuery := fmt.Sprintf(`sum(rate(container_network_receive_bytes_total{pod="%s"}[%s]))`, pod, window)
		transmitRateQuery := fmt.Sprintf(`sum(rate(container_network_transmit_bytes_total{pod="%s"}[%s]))`, pod, window)

		var trendWg sync.WaitGroup
		trendWg.Add(2)

		go func() {
			defer trendWg.Done()
			if receiveTrend, err := p.queryRange(receiveRateQuery, timeRange.Start, timeRange.End, step); err == nil && len(receiveTrend) > 0 && len(receiveTrend[0].Values) > 0 {
				mu.Lock()
				metrics.Trend = make([]types.NetworkRateDataPoint, len(receiveTrend[0].Values))
				for i, v := range receiveTrend[0].Values {
					metrics.Trend[i] = types.NetworkRateDataPoint{
						Timestamp:          v.Timestamp,
						ReceiveBytesPerSec: v.Value,
					}
				}
				mu.Unlock()
			}
		}()

		go func() {
			defer trendWg.Done()
			if transmitTrend, err := p.queryRange(transmitRateQuery, timeRange.Start, timeRange.End, step); err == nil && len(transmitTrend) > 0 && len(transmitTrend[0].Values) > 0 {
				mu.Lock()
				for i, v := range transmitTrend[0].Values {
					if i < len(metrics.Trend) {
						metrics.Trend[i].TransmitBytesPerSec = v.Value
					}
				}
				mu.Unlock()
			}
		}()

		trendWg.Wait()
		metrics.Summary = p.calculateNetworkRateSummary(metrics.Trend)
	}

	return metrics, nil
}

// GetNetworkRateByContainer 获取容器网络速率
func (p *PodOperatorImpl) GetNetworkRateByContainer(namespace, pod, container string, timeRange *types.TimeRange) (*types.ContainerNetworkRateMetrics, error) {
	metrics := &types.ContainerNetworkRateMetrics{
		Namespace:     namespace,
		PodName:       pod,
		ContainerName: container,
	}

	window := p.calculateRateWindow(timeRange)

	var wg sync.WaitGroup
	var mu sync.Mutex
	wg.Add(2)

	go func() {
		defer wg.Done()
		receiveRateQuery := fmt.Sprintf(`rate(container_network_receive_bytes_total{pod="%s",container="%s"}[%s])`, pod, container, window)
		results, err := p.query(receiveRateQuery, nil)
		if err != nil || len(results) == 0 {
			receiveRateQuery = fmt.Sprintf(`sum(rate(container_network_receive_bytes_total{pod="%s"}[%s]))`, pod, window)
			results, err = p.query(receiveRateQuery, nil)
		}
		if err == nil && len(results) > 0 {
			mu.Lock()
			metrics.Current.ReceiveBytesPerSec = results[0].Value
			metrics.Current.Timestamp = results[0].Time
			mu.Unlock()
		}
	}()

	go func() {
		defer wg.Done()
		transmitRateQuery := fmt.Sprintf(`rate(container_network_transmit_bytes_total{pod="%s",container="%s"}[%s])`, pod, container, window)
		results, err := p.query(transmitRateQuery, nil)
		if err != nil || len(results) == 0 {
			transmitRateQuery = fmt.Sprintf(`sum(rate(container_network_transmit_bytes_total{pod="%s"}[%s]))`, pod, window)
			results, err = p.query(transmitRateQuery, nil)
		}
		if err == nil && len(results) > 0 {
			mu.Lock()
			metrics.Current.TransmitBytesPerSec = results[0].Value
			mu.Unlock()
		}
	}()

	wg.Wait()

	if timeRange != nil && !timeRange.Start.IsZero() && !timeRange.End.IsZero() {
		step := timeRange.Step
		if step == "" {
			step = p.calculateStep(timeRange.Start, timeRange.End)
		}

		receiveRateQuery := fmt.Sprintf(`sum(rate(container_network_receive_bytes_total{pod="%s"}[%s]))`, pod, window)
		transmitRateQuery := fmt.Sprintf(`sum(rate(container_network_transmit_bytes_total{pod="%s"}[%s]))`, pod, window)

		var trendWg sync.WaitGroup
		trendWg.Add(2)

		go func() {
			defer trendWg.Done()
			if receiveTrend, err := p.queryRange(receiveRateQuery, timeRange.Start, timeRange.End, step); err == nil && len(receiveTrend) > 0 && len(receiveTrend[0].Values) > 0 {
				mu.Lock()
				metrics.Trend = make([]types.NetworkRateDataPoint, len(receiveTrend[0].Values))
				for i, v := range receiveTrend[0].Values {
					metrics.Trend[i] = types.NetworkRateDataPoint{
						Timestamp:          v.Timestamp,
						ReceiveBytesPerSec: v.Value,
					}
				}
				mu.Unlock()
			}
		}()

		go func() {
			defer trendWg.Done()
			if transmitTrend, err := p.queryRange(transmitRateQuery, timeRange.Start, timeRange.End, step); err == nil && len(transmitTrend) > 0 && len(transmitTrend[0].Values) > 0 {
				mu.Lock()
				for i, v := range transmitTrend[0].Values {
					if i < len(metrics.Trend) {
						metrics.Trend[i].TransmitBytesPerSec = v.Value
					}
				}
				mu.Unlock()
			}
		}()

		trendWg.Wait()
		metrics.Summary = p.calculateNetworkRateSummary(metrics.Trend)
	}

	return metrics, nil
}

// ==================== 磁盘相关方法 ====================

// GetDiskIO 获取 Pod 磁盘 I/O
func (p *PodOperatorImpl) GetDiskIO(namespace, pod string, timeRange *types.TimeRange) (*types.DiskMetrics, error) {
	metrics := &types.DiskMetrics{
		Namespace: namespace,
		PodName:   pod,
	}

	// 并发查询当前累计值
	var wg sync.WaitGroup
	var mu sync.Mutex
	wg.Add(4)

	go func() {
		defer wg.Done()
		readQuery := fmt.Sprintf(`sum(container_fs_reads_bytes_total{namespace="%s",pod="%s",container!="",container!="POD"})`, namespace, pod)
		if results, err := p.query(readQuery, nil); err == nil && len(results) > 0 {
			mu.Lock()
			metrics.Current.ReadBytes = int64(results[0].Value)
			metrics.Current.Timestamp = results[0].Time
			mu.Unlock()
		}
	}()

	go func() {
		defer wg.Done()
		writeQuery := fmt.Sprintf(`sum(container_fs_writes_bytes_total{namespace="%s",pod="%s",container!="",container!="POD"})`, namespace, pod)
		if results, err := p.query(writeQuery, nil); err == nil && len(results) > 0 {
			mu.Lock()
			metrics.Current.WriteBytes = int64(results[0].Value)
			mu.Unlock()
		}
	}()

	go func() {
		defer wg.Done()
		readOpsQuery := fmt.Sprintf(`sum(container_fs_reads_total{namespace="%s",pod="%s",container!="",container!="POD"})`, namespace, pod)
		if results, err := p.query(readOpsQuery, nil); err == nil && len(results) > 0 {
			mu.Lock()
			metrics.Current.ReadOps = int64(results[0].Value)
			mu.Unlock()
		}
	}()

	go func() {
		defer wg.Done()
		writeOpsQuery := fmt.Sprintf(`sum(container_fs_writes_total{namespace="%s",pod="%s",container!="",container!="POD"})`, namespace, pod)
		if results, err := p.query(writeOpsQuery, nil); err == nil && len(results) > 0 {
			mu.Lock()
			metrics.Current.WriteOps = int64(results[0].Value)
			mu.Unlock()
		}
	}()

	wg.Wait()

	// 趋势数据（速率）
	if timeRange != nil && !timeRange.Start.IsZero() && !timeRange.End.IsZero() {
		step := timeRange.Step
		if step == "" {
			step = p.calculateStep(timeRange.Start, timeRange.End)
		}

		window := p.calculateRateWindow(timeRange)

		readRateQuery := fmt.Sprintf(`sum(rate(container_fs_reads_bytes_total{namespace="%s",pod="%s",container!="",container!="POD"}[%s]))`, namespace, pod, window)
		writeRateQuery := fmt.Sprintf(`sum(rate(container_fs_writes_bytes_total{namespace="%s",pod="%s",container!="",container!="POD"}[%s]))`, namespace, pod, window)

		var trendWg sync.WaitGroup
		trendWg.Add(2)

		go func() {
			defer trendWg.Done()
			if readTrend, err := p.queryRange(readRateQuery, timeRange.Start, timeRange.End, step); err == nil && len(readTrend) > 0 && len(readTrend[0].Values) > 0 {
				mu.Lock()
				metrics.Trend = make([]types.DiskDataPoint, len(readTrend[0].Values))
				for i, v := range readTrend[0].Values {
					metrics.Trend[i] = types.DiskDataPoint{
						Timestamp: v.Timestamp,
						ReadBytes: int64(v.Value),
					}
				}
				mu.Unlock()
			}
		}()

		go func() {
			defer trendWg.Done()
			if writeTrend, err := p.queryRange(writeRateQuery, timeRange.Start, timeRange.End, step); err == nil && len(writeTrend) > 0 && len(writeTrend[0].Values) > 0 {
				mu.Lock()
				for i, v := range writeTrend[0].Values {
					if i < len(metrics.Trend) {
						metrics.Trend[i].WriteBytes = int64(v.Value)
					}
				}
				mu.Unlock()
			}
		}()

		trendWg.Wait()

		if len(metrics.Trend) > 0 {
			var maxRead, maxWrite, sumRead, sumWrite int64
			for _, point := range metrics.Trend {
				if point.ReadBytes > maxRead {
					maxRead = point.ReadBytes
				}
				if point.WriteBytes > maxWrite {
					maxWrite = point.WriteBytes
				}
				sumRead += point.ReadBytes
				sumWrite += point.WriteBytes
			}

			metrics.Summary.MaxReadBytesPerSec = maxRead
			metrics.Summary.MaxWriteBytesPerSec = maxWrite
			metrics.Summary.AvgReadBytesPerSec = sumRead / int64(len(metrics.Trend))
			metrics.Summary.AvgWriteBytesPerSec = sumWrite / int64(len(metrics.Trend))
		}
	}

	metrics.Summary.TotalReadBytes = metrics.Current.ReadBytes
	metrics.Summary.TotalWriteBytes = metrics.Current.WriteBytes
	metrics.Summary.TotalReadOps = metrics.Current.ReadOps
	metrics.Summary.TotalWriteOps = metrics.Current.WriteOps

	return metrics, nil
}

// GetDiskIOByContainer 获取容器磁盘 I/O
func (p *PodOperatorImpl) GetDiskIOByContainer(namespace, pod, container string, timeRange *types.TimeRange) (*types.ContainerDiskMetrics, error) {
	metrics := &types.ContainerDiskMetrics{
		Namespace:     namespace,
		PodName:       pod,
		ContainerName: container,
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	wg.Add(2)

	go func() {
		defer wg.Done()
		readQuery := fmt.Sprintf(`container_fs_reads_bytes_total{namespace="%s",pod="%s",container="%s"}`, namespace, pod, container)
		if results, err := p.query(readQuery, nil); err == nil && len(results) > 0 {
			mu.Lock()
			metrics.Current.ReadBytes = int64(results[0].Value)
			metrics.Current.Timestamp = results[0].Time
			mu.Unlock()
		}
	}()

	go func() {
		defer wg.Done()
		writeQuery := fmt.Sprintf(`container_fs_writes_bytes_total{namespace="%s",pod="%s",container="%s"}`, namespace, pod, container)
		if results, err := p.query(writeQuery, nil); err == nil && len(results) > 0 {
			mu.Lock()
			metrics.Current.WriteBytes = int64(results[0].Value)
			mu.Unlock()
		}
	}()

	wg.Wait()

	if timeRange != nil && !timeRange.Start.IsZero() && !timeRange.End.IsZero() {
		step := timeRange.Step
		if step == "" {
			step = p.calculateStep(timeRange.Start, timeRange.End)
		}

		window := p.calculateRateWindow(timeRange)

		readRateQuery := fmt.Sprintf(`rate(container_fs_reads_bytes_total{namespace="%s",pod="%s",container="%s"}[%s])`, namespace, pod, container, window)
		writeRateQuery := fmt.Sprintf(`rate(container_fs_writes_bytes_total{namespace="%s",pod="%s",container="%s"}[%s])`, namespace, pod, container, window)

		var trendWg sync.WaitGroup
		trendWg.Add(2)

		go func() {
			defer trendWg.Done()
			if readTrend, err := p.queryRange(readRateQuery, timeRange.Start, timeRange.End, step); err == nil && len(readTrend) > 0 && len(readTrend[0].Values) > 0 {
				mu.Lock()
				metrics.Trend = make([]types.DiskDataPoint, len(readTrend[0].Values))
				for i, v := range readTrend[0].Values {
					metrics.Trend[i] = types.DiskDataPoint{
						Timestamp: v.Timestamp,
						ReadBytes: int64(v.Value),
					}
				}
				mu.Unlock()
			}
		}()

		go func() {
			defer trendWg.Done()
			if writeTrend, err := p.queryRange(writeRateQuery, timeRange.Start, timeRange.End, step); err == nil && len(writeTrend) > 0 && len(writeTrend[0].Values) > 0 {
				mu.Lock()
				for i, v := range writeTrend[0].Values {
					if i < len(metrics.Trend) {
						metrics.Trend[i].WriteBytes = int64(v.Value)
					}
				}
				mu.Unlock()
			}
		}()

		trendWg.Wait()

		if len(metrics.Trend) > 0 {
			metrics.Summary.TotalReadBytes = metrics.Current.ReadBytes
			metrics.Summary.TotalWriteBytes = metrics.Current.WriteBytes
		}
	}

	return metrics, nil
}

// GetDiskRate 获取磁盘速率
func (p *PodOperatorImpl) GetDiskRate(namespace, pod string, timeRange *types.TimeRange) (*types.DiskRateMetrics, error) {
	metrics := &types.DiskRateMetrics{
		Namespace: namespace,
		PodName:   pod,
	}

	window := p.calculateRateWindow(timeRange)

	var wg sync.WaitGroup
	var mu sync.Mutex
	wg.Add(4)

	go func() {
		defer wg.Done()
		readRateQuery := fmt.Sprintf(`sum(rate(container_fs_reads_bytes_total{namespace="%s",pod="%s",container!="",container!="POD"}[%s]))`, namespace, pod, window)
		if results, err := p.query(readRateQuery, nil); err == nil && len(results) > 0 {
			mu.Lock()
			metrics.Current.ReadBytesPerSec = results[0].Value
			metrics.Current.Timestamp = results[0].Time
			mu.Unlock()
		}
	}()

	go func() {
		defer wg.Done()
		writeRateQuery := fmt.Sprintf(`sum(rate(container_fs_writes_bytes_total{namespace="%s",pod="%s",container!="",container!="POD"}[%s]))`, namespace, pod, window)
		if results, err := p.query(writeRateQuery, nil); err == nil && len(results) > 0 {
			mu.Lock()
			metrics.Current.WriteBytesPerSec = results[0].Value
			mu.Unlock()
		}
	}()

	go func() {
		defer wg.Done()
		readOpsRateQuery := fmt.Sprintf(`sum(rate(container_fs_reads_total{namespace="%s",pod="%s",container!="",container!="POD"}[%s]))`, namespace, pod, window)
		if results, err := p.query(readOpsRateQuery, nil); err == nil && len(results) > 0 {
			mu.Lock()
			metrics.Current.ReadOpsPerSec = results[0].Value
			mu.Unlock()
		}
	}()

	go func() {
		defer wg.Done()
		writeOpsRateQuery := fmt.Sprintf(`sum(rate(container_fs_writes_total{namespace="%s",pod="%s",container!="",container!="POD"}[%s]))`, namespace, pod, window)
		if results, err := p.query(writeOpsRateQuery, nil); err == nil && len(results) > 0 {
			mu.Lock()
			metrics.Current.WriteOpsPerSec = results[0].Value
			mu.Unlock()
		}
	}()

	wg.Wait()

	if timeRange != nil && !timeRange.Start.IsZero() && !timeRange.End.IsZero() {
		step := timeRange.Step
		if step == "" {
			step = p.calculateStep(timeRange.Start, timeRange.End)
		}

		readRateQuery := fmt.Sprintf(`sum(rate(container_fs_reads_bytes_total{namespace="%s",pod="%s",container!="",container!="POD"}[%s]))`, namespace, pod, window)
		writeRateQuery := fmt.Sprintf(`sum(rate(container_fs_writes_bytes_total{namespace="%s",pod="%s",container!="",container!="POD"}[%s]))`, namespace, pod, window)

		var trendWg sync.WaitGroup
		trendWg.Add(2)

		go func() {
			defer trendWg.Done()
			if readTrend, err := p.queryRange(readRateQuery, timeRange.Start, timeRange.End, step); err == nil && len(readTrend) > 0 && len(readTrend[0].Values) > 0 {
				mu.Lock()
				metrics.Trend = make([]types.DiskRateDataPoint, len(readTrend[0].Values))
				for i, v := range readTrend[0].Values {
					metrics.Trend[i] = types.DiskRateDataPoint{
						Timestamp:       v.Timestamp,
						ReadBytesPerSec: v.Value,
					}
				}
				mu.Unlock()
			}
		}()

		go func() {
			defer trendWg.Done()
			if writeTrend, err := p.queryRange(writeRateQuery, timeRange.Start, timeRange.End, step); err == nil && len(writeTrend) > 0 && len(writeTrend[0].Values) > 0 {
				mu.Lock()
				for i, v := range writeTrend[0].Values {
					if i < len(metrics.Trend) {
						metrics.Trend[i].WriteBytesPerSec = v.Value
					}
				}
				mu.Unlock()
			}
		}()

		trendWg.Wait()
		metrics.Summary = p.calculateDiskRateSummary(metrics.Trend)
	}

	return metrics, nil
}

// ==================== Pod 状态相关方法 ====================

// GetPodStatus 获取 Pod 状态
func (p *PodOperatorImpl) GetPodStatus(namespace, pod string, timeRange *types.TimeRange) (*types.PodStatusMetrics, error) {
	metrics := &types.PodStatusMetrics{
		Namespace: namespace,
		PodName:   pod,
	}

	var wg sync.WaitGroup
	wg.Add(3)

	// 1. Pod Phase
	go func() {
		defer wg.Done()
		phaseQuery := fmt.Sprintf(`kube_pod_status_phase{namespace="%s",pod="%s"}`, namespace, pod)
		if results, err := p.query(phaseQuery, nil); err == nil {
			for _, result := range results {
				if result.Value > 0 {
					if phase, ok := result.Metric["phase"]; ok {
						metrics.Current.Phase = phase
						metrics.Current.Timestamp = result.Time
						break
					}
				}
			}
		}
	}()

	// 2. Pod Ready
	go func() {
		defer wg.Done()
		readyQuery := fmt.Sprintf(`kube_pod_status_ready{namespace="%s",pod="%s",condition="true"}`, namespace, pod)
		if results, err := p.query(readyQuery, nil); err == nil && len(results) > 0 {
			metrics.Current.Ready = results[0].Value > 0
		}
	}()

	// 3. Container States
	go func() {
		defer wg.Done()
		containerStateQuery := fmt.Sprintf(`kube_pod_container_status_ready{namespace="%s",pod="%s"}`, namespace, pod)
		if results, err := p.query(containerStateQuery, nil); err == nil {
			metrics.Current.ContainerStates = make([]types.ContainerState, 0, len(results))
			for _, result := range results {
				if containerName, ok := result.Metric["container"]; ok {
					metrics.Current.ContainerStates = append(metrics.Current.ContainerStates, types.ContainerState{
						ContainerName: containerName,
						Ready:         result.Value > 0,
						State:         "Running",
					})
				}
			}
		}
	}()

	wg.Wait()
	return metrics, nil
}

// GetRestartCount 获取重启次数
func (p *PodOperatorImpl) GetRestartCount(namespace, pod string, timeRange *types.TimeRange) (*types.RestartMetrics, error) {
	metrics := &types.RestartMetrics{
		Namespace: namespace,
		PodName:   pod,
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		restartQuery := fmt.Sprintf(`sum(kube_pod_container_status_restarts_total{namespace="%s",pod="%s"})`, namespace, pod)
		if results, err := p.query(restartQuery, nil); err == nil && len(results) > 0 {
			metrics.TotalRestarts = int64(results[0].Value)
		}
	}()

	go func() {
		defer wg.Done()
		containerRestartQuery := fmt.Sprintf(`kube_pod_container_status_restarts_total{namespace="%s",pod="%s"}`, namespace, pod)
		if results, err := p.query(containerRestartQuery, nil); err == nil {
			metrics.ByContainer = make([]types.ContainerRestart, 0, len(results))
			for _, result := range results {
				if containerName, ok := result.Metric["container"]; ok {
					metrics.ByContainer = append(metrics.ByContainer, types.ContainerRestart{
						ContainerName: containerName,
						RestartCount:  int64(result.Value),
					})
				}
			}
		}
	}()

	wg.Wait()

	// 趋势数据
	if timeRange != nil && !timeRange.Start.IsZero() && !timeRange.End.IsZero() {
		step := timeRange.Step
		if step == "" {
			step = p.calculateStep(timeRange.Start, timeRange.End)
		}

		restartQuery := fmt.Sprintf(`sum(kube_pod_container_status_restarts_total{namespace="%s",pod="%s"})`, namespace, pod)
		if trendResults, err := p.queryRange(restartQuery, timeRange.Start, timeRange.End, step); err == nil && len(trendResults) > 0 && len(trendResults[0].Values) > 0 {
			metrics.Trend = make([]types.RestartDataPoint, 0, len(trendResults[0].Values))
			for _, v := range trendResults[0].Values {
				metrics.Trend = append(metrics.Trend, types.RestartDataPoint{
					Timestamp:    v.Timestamp,
					RestartCount: int64(v.Value),
				})
			}
		}
	}

	return metrics, nil
}

// GetPodAge 获取 Pod 存活时间
func (p *PodOperatorImpl) GetPodAge(namespace, pod string) (*types.PodAgeMetrics, error) {
	metrics := &types.PodAgeMetrics{
		Namespace: namespace,
		PodName:   pod,
	}

	creationQuery := fmt.Sprintf(`kube_pod_created{namespace="%s",pod="%s"}`, namespace, pod)
	creationResults, err := p.query(creationQuery, nil)
	if err != nil {
		return nil, fmt.Errorf("查询 Pod 创建时间失败: %w", err)
	}

	if len(creationResults) > 0 {
		creationTimestamp := int64(creationResults[0].Value)
		metrics.CreationTime = time.Unix(creationTimestamp, 0)

		now := time.Now()
		age := now.Sub(metrics.CreationTime)
		metrics.AgeSeconds = int64(age.Seconds())
		metrics.Age = formatDuration(age)
		metrics.Uptime = metrics.Age
		metrics.UptimeSeconds = metrics.AgeSeconds
	}

	return metrics, nil
}

// ==================== 综合查询方法（优化版）====================

// GetPodOverview 获取 Pod 综合概览 - 并发优化
func (p *PodOperatorImpl) GetPodOverview(namespace, pod string, timeRange *types.TimeRange) (*types.PodOverview, error) {
	overview := &types.PodOverview{
		Namespace: namespace,
		PodName:   pod,
	}

	var wg sync.WaitGroup
	wg.Add(7)

	go func() {
		defer wg.Done()
		if status, err := p.GetPodStatus(namespace, pod, nil); err == nil {
			overview.Status = status.Current
		}
	}()

	go func() {
		defer wg.Done()
		if cpu, err := p.GetCPUUsage(namespace, pod, timeRange); err == nil {
			overview.CPU = cpu.Current
		}
	}()

	go func() {
		defer wg.Done()
		if memory, err := p.GetMemoryUsage(namespace, pod, timeRange); err == nil {
			overview.Memory = memory.Current
		}
	}()

	go func() {
		defer wg.Done()
		if network, err := p.GetNetworkIO(namespace, pod, nil); err == nil {
			overview.Network = network.Current
		}
	}()

	go func() {
		defer wg.Done()
		if disk, err := p.GetDiskIO(namespace, pod, nil); err == nil {
			overview.Disk = disk.Current
		}
	}()

	go func() {
		defer wg.Done()
		if restart, err := p.GetRestartCount(namespace, pod, nil); err == nil {
			overview.RestartCount = restart.TotalRestarts
		}
	}()

	go func() {
		defer wg.Done()
		if age, err := p.GetPodAge(namespace, pod); err == nil {
			overview.Age = *age
		}
	}()

	wg.Wait()

	// 获取 Labels
	labelsQuery := fmt.Sprintf(`kube_pod_labels{namespace="%s",pod="%s"}`, namespace, pod)
	if labelsResults, err := p.query(labelsQuery, nil); err == nil && len(labelsResults) > 0 {
		overview.Labels = labelsResults[0].Metric
	}

	return overview, nil
}

// ListPodsMetrics 列出命名空间下所有 Pod 的指标 - 批量优化
func (p *PodOperatorImpl) ListPodsMetrics(namespace string, timeRange *types.TimeRange) ([]types.PodOverview, error) {
	p.log.Infof("📊 批量查询命名空间 Pod: namespace=%s", namespace)

	// 获取所有 Pod
	podsQuery := fmt.Sprintf(`kube_pod_info{namespace="%s"}`, namespace)
	podsResults, err := p.query(podsQuery, nil)
	if err != nil {
		return nil, fmt.Errorf("查询 Pod 列表失败: %w", err)
	}

	if len(podsResults) == 0 {
		return []types.PodOverview{}, nil
	}

	// 并发查询所有 Pod
	var wg sync.WaitGroup
	var mu sync.Mutex
	overviews := make([]types.PodOverview, 0, len(podsResults))

	for _, result := range podsResults {
		if podName, ok := result.Metric["pod"]; ok {
			wg.Add(1)
			go func(name string) {
				defer wg.Done()
				if overview, err := p.GetPodOverview(namespace, name, timeRange); err == nil {
					mu.Lock()
					overviews = append(overviews, *overview)
					mu.Unlock()
				} else {
					p.log.Errorf("获取 Pod 概览失败: pod=%s, error=%v", name, err)
				}
			}(podName)
		}
	}

	wg.Wait()

	p.log.Infof("✅ 批量查询完成: namespace=%s, count=%d", namespace, len(overviews))
	return overviews, nil
}

// ==================== Top 排行方法 ====================

// GetTopPodsByCPU 获取 CPU 使用 Top N
func (p *PodOperatorImpl) GetTopPodsByCPU(namespace string, limit int, timeRange *types.TimeRange) ([]types.PodRanking, error) {
	window := p.calculateRateWindow(timeRange)

	query := fmt.Sprintf(`topk(%d, sum by (pod) (rate(container_cpu_usage_seconds_total{namespace="%s",container!="",container!="POD"}[%s])))`, limit, namespace, window)
	results, err := p.query(query, nil)
	if err != nil {
		return nil, fmt.Errorf("查询 CPU Top Pods 失败: %w", err)
	}

	rankings := make([]types.PodRanking, 0, len(results))
	for _, result := range results {
		if podName, ok := result.Metric["pod"]; ok {
			rankings = append(rankings, types.PodRanking{
				Namespace: namespace,
				PodName:   podName,
				Value:     result.Value,
				Unit:      "cores",
			})
		}
	}

	sort.Slice(rankings, func(i, j int) bool {
		return rankings[i].Value > rankings[j].Value
	})

	return rankings, nil
}

// GetTopPodsByMemory 获取内存使用 Top N
func (p *PodOperatorImpl) GetTopPodsByMemory(namespace string, limit int, timeRange *types.TimeRange) ([]types.PodRanking, error) {
	query := fmt.Sprintf(`topk(%d, sum by (pod) (container_memory_working_set_bytes{namespace="%s",container!="",container!="POD"}))`, limit, namespace)
	results, err := p.query(query, nil)
	if err != nil {
		return nil, fmt.Errorf("查询内存 Top Pods 失败: %w", err)
	}

	rankings := make([]types.PodRanking, 0, len(results))
	for _, result := range results {
		if podName, ok := result.Metric["pod"]; ok {
			rankings = append(rankings, types.PodRanking{
				Namespace: namespace,
				PodName:   podName,
				Value:     result.Value,
				Unit:      "bytes",
			})
		}
	}

	sort.Slice(rankings, func(i, j int) bool {
		return rankings[i].Value > rankings[j].Value
	})

	return rankings, nil
}

// GetTopPodsByNetwork 获取网络使用 Top N
func (p *PodOperatorImpl) GetTopPodsByNetwork(namespace string, limit int, timeRange *types.TimeRange) ([]types.PodRanking, error) {
	window := p.calculateRateWindow(timeRange)

	query := fmt.Sprintf(`topk(%d, sum by (pod) (rate(container_network_receive_bytes_total{namespace="%s"}[%s]) + rate(container_network_transmit_bytes_total{namespace="%s"}[%s])))`, limit, namespace, window, namespace, window)
	results, err := p.query(query, nil)
	if err != nil {
		return nil, fmt.Errorf("查询网络 Top Pods 失败: %w", err)
	}

	rankings := make([]types.PodRanking, 0, len(results))
	for _, result := range results {
		if podName, ok := result.Metric["pod"]; ok {
			rankings = append(rankings, types.PodRanking{
				Namespace: namespace,
				PodName:   podName,
				Value:     result.Value,
				Unit:      "bytes/s",
			})
		}
	}

	sort.Slice(rankings, func(i, j int) bool {
		return rankings[i].Value > rankings[j].Value
	})

	return rankings, nil
}

// ==================== 存储/Volume 相关方法 ====================

// GetVolumeUsage 获取 Pod 存储使用情况
func (p *PodOperatorImpl) GetVolumeUsage(namespace, pod string) (*types.PodVolumeMetrics, error) {
	metrics := &types.PodVolumeMetrics{
		Namespace: namespace,
		PodName:   pod,
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		capacityQuery := fmt.Sprintf(`sum(container_fs_limit_bytes{namespace="%s",pod="%s"})`, namespace, pod)
		if results, err := p.query(capacityQuery, nil); err == nil && len(results) > 0 {
			metrics.TotalCapacity = int64(results[0].Value)
		}
	}()

	go func() {
		defer wg.Done()
		usageQuery := fmt.Sprintf(`sum(container_fs_usage_bytes{namespace="%s",pod="%s"})`, namespace, pod)
		if results, err := p.query(usageQuery, nil); err == nil && len(results) > 0 {
			metrics.TotalUsed = int64(results[0].Value)
		}
	}()

	wg.Wait()

	if metrics.TotalCapacity > 0 {
		metrics.UsagePercent = (float64(metrics.TotalUsed) / float64(metrics.TotalCapacity)) * 100
	}

	// 查询各个设备
	deviceQuery := fmt.Sprintf(`container_fs_usage_bytes{namespace="%s",pod="%s"}`, namespace, pod)
	if deviceResults, err := p.query(deviceQuery, nil); err == nil {
		metrics.Volumes = make([]types.VolumeUsage, 0, len(deviceResults))
		for _, result := range deviceResults {
			device, hasDevice := result.Metric["device"]
			if !hasDevice {
				continue
			}

			volume := types.VolumeUsage{
				Device:    device,
				UsedBytes: int64(result.Value),
			}

			deviceCapacityQuery := fmt.Sprintf(`container_fs_limit_bytes{namespace="%s",pod="%s",device="%s"}`,
				namespace, pod, device)
			if deviceCapacityResults, _ := p.query(deviceCapacityQuery, nil); len(deviceCapacityResults) > 0 {
				volume.CapacityBytes = int64(deviceCapacityResults[0].Value)
				volume.AvailBytes = volume.CapacityBytes - volume.UsedBytes
				if volume.CapacityBytes > 0 {
					volume.UsagePercent = (float64(volume.UsedBytes) / float64(volume.CapacityBytes)) * 100
				}
			}

			metrics.Volumes = append(metrics.Volumes, volume)
		}
	}

	return metrics, nil
}

// GetVolumeIOPS 获取 Volume IOPS
func (p *PodOperatorImpl) GetVolumeIOPS(namespace, pod string, timeRange *types.TimeRange) (*types.VolumeIOPSMetrics, error) {
	metrics := &types.VolumeIOPSMetrics{
		Namespace: namespace,
		PodName:   pod,
	}

	window := p.calculateRateWindow(timeRange)

	var wg sync.WaitGroup
	var mu sync.Mutex
	wg.Add(2)

	go func() {
		defer wg.Done()
		readIOPSQuery := fmt.Sprintf(`sum(rate(container_fs_reads_total{namespace="%s",pod="%s"}[%s]))`, namespace, pod, window)
		if results, err := p.query(readIOPSQuery, nil); err == nil && len(results) > 0 {
			mu.Lock()
			metrics.Current.ReadIOPS = results[0].Value
			metrics.Current.Timestamp = results[0].Time
			mu.Unlock()
		}
	}()

	go func() {
		defer wg.Done()
		writeIOPSQuery := fmt.Sprintf(`sum(rate(container_fs_writes_total{namespace="%s",pod="%s"}[%s]))`, namespace, pod, window)
		if results, err := p.query(writeIOPSQuery, nil); err == nil && len(results) > 0 {
			mu.Lock()
			metrics.Current.WriteIOPS = results[0].Value
			mu.Unlock()
		}
	}()

	wg.Wait()

	metrics.Current.TotalIOPS = metrics.Current.ReadIOPS + metrics.Current.WriteIOPS

	// 趋势数据
	if timeRange != nil && !timeRange.Start.IsZero() && !timeRange.End.IsZero() {
		step := timeRange.Step
		if step == "" {
			step = p.calculateStep(timeRange.Start, timeRange.End)
		}

		readIOPSQuery := fmt.Sprintf(`sum(rate(container_fs_reads_total{namespace="%s",pod="%s"}[%s]))`, namespace, pod, window)
		writeIOPSQuery := fmt.Sprintf(`sum(rate(container_fs_writes_total{namespace="%s",pod="%s"}[%s]))`, namespace, pod, window)

		var trendWg sync.WaitGroup
		trendWg.Add(2)

		go func() {
			defer trendWg.Done()
			if readTrend, err := p.queryRange(readIOPSQuery, timeRange.Start, timeRange.End, step); err == nil && len(readTrend) > 0 && len(readTrend[0].Values) > 0 {
				mu.Lock()
				metrics.Trend = make([]types.IOPSDataPoint, len(readTrend[0].Values))
				for i, v := range readTrend[0].Values {
					metrics.Trend[i] = types.IOPSDataPoint{
						Timestamp: v.Timestamp,
						ReadIOPS:  v.Value,
					}
				}
				mu.Unlock()
			}
		}()

		go func() {
			defer trendWg.Done()
			if writeTrend, err := p.queryRange(writeIOPSQuery, timeRange.Start, timeRange.End, step); err == nil && len(writeTrend) > 0 && len(writeTrend[0].Values) > 0 {
				mu.Lock()
				for i, v := range writeTrend[0].Values {
					if i < len(metrics.Trend) {
						metrics.Trend[i].WriteIOPS = v.Value
						metrics.Trend[i].TotalIOPS = metrics.Trend[i].ReadIOPS + v.Value
					}
				}
				mu.Unlock()
			}
		}()

		trendWg.Wait()

		if len(metrics.Trend) > 0 {
			var sumRead, maxRead, sumWrite, maxWrite float64
			for _, point := range metrics.Trend {
				sumRead += point.ReadIOPS
				if point.ReadIOPS > maxRead {
					maxRead = point.ReadIOPS
				}
				sumWrite += point.WriteIOPS
				if point.WriteIOPS > maxWrite {
					maxWrite = point.WriteIOPS
				}
			}
			metrics.Summary.AvgReadIOPS = sumRead / float64(len(metrics.Trend))
			metrics.Summary.MaxReadIOPS = maxRead
			metrics.Summary.AvgWriteIOPS = sumWrite / float64(len(metrics.Trend))
			metrics.Summary.MaxWriteIOPS = maxWrite
		}
	}

	return metrics, nil
}

// GetProbeStatus 获取探针状态
func (p *PodOperatorImpl) GetProbeStatus(namespace, pod string) (*types.PodProbeMetrics, error) {
	metrics := &types.PodProbeMetrics{
		Namespace: namespace,
		PodName:   pod,
	}
	return metrics, nil
}

// GetFileDescriptorUsage 获取文件描述符使用情况
func (p *PodOperatorImpl) GetFileDescriptorUsage(namespace, pod string, timeRange *types.TimeRange) (*types.FileDescriptorMetrics, error) {
	metrics := &types.FileDescriptorMetrics{
		Namespace: namespace,
		PodName:   pod,
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	wg.Add(2)

	go func() {
		defer wg.Done()
		openFDQuery := fmt.Sprintf(`sum(process_open_fds{namespace="%s",pod="%s"})`, namespace, pod)
		if results, err := p.query(openFDQuery, nil); err == nil && len(results) > 0 {
			mu.Lock()
			metrics.Current.OpenFDs = int64(results[0].Value)
			metrics.Current.Timestamp = results[0].Time
			mu.Unlock()
		}
	}()

	go func() {
		defer wg.Done()
		maxFDQuery := fmt.Sprintf(`sum(process_max_fds{namespace="%s",pod="%s"})`, namespace, pod)
		if results, err := p.query(maxFDQuery, nil); err == nil && len(results) > 0 {
			mu.Lock()
			metrics.Current.MaxFDs = int64(results[0].Value)
			mu.Unlock()
		}
	}()

	wg.Wait()

	if metrics.Current.MaxFDs > 0 {
		metrics.Current.UsagePercent = (float64(metrics.Current.OpenFDs) / float64(metrics.Current.MaxFDs)) * 100
	}

	// 趋势数据
	if timeRange != nil && !timeRange.Start.IsZero() && !timeRange.End.IsZero() {
		step := timeRange.Step
		if step == "" {
			step = p.calculateStep(timeRange.Start, timeRange.End)
		}

		openFDQuery := fmt.Sprintf(`sum(process_open_fds{namespace="%s",pod="%s"})`, namespace, pod)
		if trendResults, err := p.queryRange(openFDQuery, timeRange.Start, timeRange.End, step); err == nil && len(trendResults) > 0 && len(trendResults[0].Values) > 0 {
			metrics.Trend = make([]types.FDDataPoint, 0, len(trendResults[0].Values))
			for _, v := range trendResults[0].Values {
				usagePercent := 0.0
				if metrics.Current.MaxFDs > 0 {
					usagePercent = (v.Value / float64(metrics.Current.MaxFDs)) * 100
				}
				metrics.Trend = append(metrics.Trend, types.FDDataPoint{
					Timestamp:    v.Timestamp,
					OpenFDs:      int64(v.Value),
					UsagePercent: usagePercent,
				})
			}

			if len(metrics.Trend) > 0 {
				var sumOpen, maxOpen, sumPercent, maxPercent float64
				for _, point := range metrics.Trend {
					sumOpen += float64(point.OpenFDs)
					if float64(point.OpenFDs) > maxOpen {
						maxOpen = float64(point.OpenFDs)
					}
					sumPercent += point.UsagePercent
					if point.UsagePercent > maxPercent {
						maxPercent = point.UsagePercent
					}
				}
				metrics.Summary.AvgOpenFDs = int64(sumOpen / float64(len(metrics.Trend)))
				metrics.Summary.MaxOpenFDs = int64(maxOpen)
				metrics.Summary.AvgUsagePercent = sumPercent / float64(len(metrics.Trend))
				metrics.Summary.MaxUsagePercent = maxPercent
			}
		}
	}

	return metrics, nil
}

// GetNetworkConnections 获取网络连接数
func (p *PodOperatorImpl) GetNetworkConnections(namespace, pod string, timeRange *types.TimeRange) (*types.NetworkConnectionMetrics, error) {
	metrics := &types.NetworkConnectionMetrics{
		Namespace: namespace,
		PodName:   pod,
	}
	return metrics, nil
}

// GetContainerStatus 获取容器详细状态
func (p *PodOperatorImpl) GetContainerStatus(namespace, pod, container string) (*types.ContainerStatusMetrics, error) {
	metrics := &types.ContainerStatusMetrics{
		Namespace:     namespace,
		PodName:       pod,
		ContainerName: container,
	}

	var wg sync.WaitGroup
	wg.Add(3)

	go func() {
		defer wg.Done()
		stateQuery := fmt.Sprintf(`kube_pod_container_status_running{namespace="%s",pod="%s",container="%s"}`,
			namespace, pod, container)
		if results, err := p.query(stateQuery, nil); err == nil && len(results) > 0 {
			if results[0].Value > 0 {
				metrics.State = "Running"
				metrics.Ready = true
			}
		}
	}()

	go func() {
		defer wg.Done()
		restartQuery := fmt.Sprintf(`kube_pod_container_status_restarts_total{namespace="%s",pod="%s",container="%s"}`,
			namespace, pod, container)
		if results, err := p.query(restartQuery, nil); err == nil && len(results) > 0 {
			metrics.RestartCount = int64(results[0].Value)
		}
	}()

	go func() {
		defer wg.Done()
		infoQuery := fmt.Sprintf(`kube_pod_container_info{namespace="%s",pod="%s",container="%s"}`,
			namespace, pod, container)
		if results, err := p.query(infoQuery, nil); err == nil && len(results) > 0 {
			if image, ok := results[0].Metric["image"]; ok {
				metrics.Image = image
			}
			if imageID, ok := results[0].Metric["image_id"]; ok {
				metrics.ImageID = imageID
			}
			if containerID, ok := results[0].Metric["container_id"]; ok {
				metrics.ContainerID = containerID
			}
		}
	}()

	wg.Wait()
	return metrics, nil
}

// GetResourceQuota 获取命名空间资源配额使用情况
func (p *PodOperatorImpl) GetResourceQuota(namespace string) (*types.ResourceQuotaMetrics, error) {
	metrics := &types.ResourceQuotaMetrics{
		Namespace: namespace,
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	wg.Add(6)

	go func() {
		defer wg.Done()
		cpuUsedQuery := fmt.Sprintf(`sum(kube_resourcequota{namespace="%s",resource="requests.cpu",type="used"})`, namespace)
		if results, err := p.query(cpuUsedQuery, nil); err == nil && len(results) > 0 {
			mu.Lock()
			metrics.CPUUsed = results[0].Value
			mu.Unlock()
		}
	}()

	go func() {
		defer wg.Done()
		cpuHardQuery := fmt.Sprintf(`sum(kube_resourcequota{namespace="%s",resource="requests.cpu",type="hard"})`, namespace)
		if results, err := p.query(cpuHardQuery, nil); err == nil && len(results) > 0 {
			mu.Lock()
			metrics.CPUHard = results[0].Value
			mu.Unlock()
		}
	}()

	go func() {
		defer wg.Done()
		memUsedQuery := fmt.Sprintf(`sum(kube_resourcequota{namespace="%s",resource="requests.memory",type="used"})`, namespace)
		if results, err := p.query(memUsedQuery, nil); err == nil && len(results) > 0 {
			mu.Lock()
			metrics.MemoryUsed = int64(results[0].Value)
			mu.Unlock()
		}
	}()

	go func() {
		defer wg.Done()
		memHardQuery := fmt.Sprintf(`sum(kube_resourcequota{namespace="%s",resource="requests.memory",type="hard"})`, namespace)
		if results, err := p.query(memHardQuery, nil); err == nil && len(results) > 0 {
			mu.Lock()
			metrics.MemoryHard = int64(results[0].Value)
			mu.Unlock()
		}
	}()

	go func() {
		defer wg.Done()
		podsUsedQuery := fmt.Sprintf(`sum(kube_resourcequota{namespace="%s",resource="pods",type="used"})`, namespace)
		if results, err := p.query(podsUsedQuery, nil); err == nil && len(results) > 0 {
			mu.Lock()
			metrics.PodsUsed = int64(results[0].Value)
			mu.Unlock()
		}
	}()

	go func() {
		defer wg.Done()
		podsHardQuery := fmt.Sprintf(`sum(kube_resourcequota{namespace="%s",resource="pods",type="hard"})`, namespace)
		if results, err := p.query(podsHardQuery, nil); err == nil && len(results) > 0 {
			mu.Lock()
			metrics.PodsHard = int64(results[0].Value)
			mu.Unlock()
		}
	}()

	wg.Wait()

	// 计算百分比
	if metrics.CPUHard > 0 {
		metrics.CPUPercent = (metrics.CPUUsed / metrics.CPUHard) * 100
	}
	if metrics.MemoryHard > 0 {
		metrics.MemoryPercent = (float64(metrics.MemoryUsed) / float64(metrics.MemoryHard)) * 100
	}
	if metrics.PodsHard > 0 {
		metrics.PodsPercent = (float64(metrics.PodsUsed) / float64(metrics.PodsHard)) * 100
	}

	return metrics, nil
}

// ==================== 辅助方法 ====================

// query 即时查询
func (p *PodOperatorImpl) query(query string, timestamp *time.Time) ([]types.InstantQueryResult, error) {
	params := map[string]string{
		"query": query,
	}

	if timestamp != nil {
		params["time"] = p.formatTimestamp(*timestamp)
	}

	var response struct {
		Status string `json:"status"`
		Data   struct {
			ResultType string `json:"resultType"`
			Result     []struct {
				Metric map[string]string `json:"metric"`
				Value  []interface{}     `json:"value"`
			} `json:"result"`
		} `json:"data"`
		Error string `json:"error,omitempty"`
	}

	if err := p.doRequest("GET", "/api/v1/query", params, nil, &response); err != nil {
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

		timestampFloat, ok := item.Value[0].(float64)
		if !ok {
			continue
		}

		valueStr, ok := item.Value[1].(string)
		if !ok {
			continue
		}

		value, err := strconv.ParseFloat(valueStr, 64)
		if err != nil {
			continue
		}

		results = append(results, types.InstantQueryResult{
			Metric: item.Metric,
			Value:  value,
			Time:   parseTimestamp(timestampFloat),
		})
	}

	return results, nil
}

// queryRange 范围查询
func (p *PodOperatorImpl) queryRange(query string, start, end time.Time, step string) ([]types.RangeQueryResult, error) {
	params := map[string]string{
		"query": query,
		"start": p.formatTimestamp(start),
		"end":   p.formatTimestamp(end),
		"step":  step,
	}

	var response struct {
		Status string `json:"status"`
		Data   struct {
			ResultType string `json:"resultType"`
			Result     []struct {
				Metric map[string]string `json:"metric"`
				Values [][]interface{}   `json:"values"`
			} `json:"result"`
		} `json:"data"`
		Error     string   `json:"error,omitempty"`
		ErrorType string   `json:"errorType,omitempty"`
		Warnings  []string `json:"warnings,omitempty"`
	}

	if err := p.doRequest("GET", "/api/v1/query_range", params, nil, &response); err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}

	if response.Status != "success" {
		return nil, fmt.Errorf("查询失败: %s", response.Error)
	}

	results := make([]types.RangeQueryResult, 0, len(response.Data.Result))

	for _, item := range response.Data.Result {
		if len(item.Values) == 0 {
			continue
		}

		values := make([]types.MetricValue, 0, len(item.Values))

		for _, v := range item.Values {
			if len(v) != 2 {
				continue
			}

			timestampFloat, ok := v[0].(float64)
			if !ok {
				continue
			}

			valueStr, ok := v[1].(string)
			if !ok {
				continue
			}

			value, err := strconv.ParseFloat(valueStr, 64)
			if err != nil {
				continue
			}

			values = append(values, types.MetricValue{
				Timestamp: parseTimestamp(timestampFloat),
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

// formatTimestamp 格式化时间为 Unix 时间戳（秒，带小数）
func (p *PodOperatorImpl) formatTimestamp(t time.Time) string {
	return fmt.Sprintf("%.3f", float64(t.Unix())+float64(t.Nanosecond())/1e9)
}

// calculateRateWindow 根据时间范围智能计算查询窗口
func (p *PodOperatorImpl) calculateRateWindow(timeRange *types.TimeRange) string {
	if timeRange == nil || timeRange.Start.IsZero() || timeRange.End.IsZero() {
		return "5m"
	}

	duration := timeRange.End.Sub(timeRange.Start)
	if duration <= 5*time.Minute {
		return "1m"
	} else if duration <= 30*time.Minute {
		return "5m"
	} else if duration <= 2*time.Hour {
		return "10m"
	} else if duration <= 6*time.Hour {
		return "15m"
	} else if duration <= 24*time.Hour {
		return "30m"
	}
	return "1h"
}

// calculateStep 根据时间范围计算采样步长
func (p *PodOperatorImpl) calculateStep(start, end time.Time) string {
	duration := end.Sub(start)
	if duration <= 1*time.Hour {
		return "15s"
	} else if duration <= 6*time.Hour {
		return "30s"
	} else if duration <= 24*time.Hour {
		return "1m"
	} else if duration <= 7*24*time.Hour {
		return "5m"
	}
	return "15m"
}

// calculateCPUSummary 计算 CPU 统计汇总
func (p *PodOperatorImpl) calculateCPUSummary(trend []types.CPUUsageDataPoint, throttledTime float64) types.CPUSummary {
	if len(trend) == 0 {
		return types.CPUSummary{}
	}

	var sum, max, min float64
	min = math.MaxFloat64

	for _, point := range trend {
		sum += point.UsageCores
		if point.UsageCores > max {
			max = point.UsageCores
		}
		if point.UsageCores < min {
			min = point.UsageCores
		}
	}

	avg := sum / float64(len(trend))

	var sumPercent, maxPercent float64
	for _, point := range trend {
		sumPercent += point.UsagePercent
		if point.UsagePercent > maxPercent {
			maxPercent = point.UsagePercent
		}
	}
	avgPercent := sumPercent / float64(len(trend))

	return types.CPUSummary{
		AvgUsageCores:    avg,
		MaxUsageCores:    max,
		MinUsageCores:    min,
		AvgUsagePercent:  avgPercent,
		MaxUsagePercent:  maxPercent,
		TotalThrottled:   throttledTime,
		ThrottledPercent: 0,
	}
}

// calculateMemorySummary 计算内存统计汇总
func (p *PodOperatorImpl) calculateMemorySummary(trend []types.MemoryUsageDataPoint) types.MemorySummary {
	if len(trend) == 0 {
		return types.MemorySummary{}
	}

	var sumBytes int64
	var maxBytes int64 = 0
	var minBytes int64 = math.MaxInt64

	for _, point := range trend {
		sumBytes += point.UsageBytes
		if point.UsageBytes > maxBytes {
			maxBytes = point.UsageBytes
		}
		if point.UsageBytes < minBytes {
			minBytes = point.UsageBytes
		}
	}

	avgBytes := sumBytes / int64(len(trend))

	var sumPercent, maxPercent float64
	for _, point := range trend {
		sumPercent += point.UsagePercent
		if point.UsagePercent > maxPercent {
			maxPercent = point.UsagePercent
		}
	}
	avgPercent := sumPercent / float64(len(trend))

	return types.MemorySummary{
		AvgUsageBytes:   avgBytes,
		MaxUsageBytes:   maxBytes,
		MinUsageBytes:   minBytes,
		AvgUsagePercent: avgPercent,
		MaxUsagePercent: maxPercent,
	}
}

// calculateNetworkRateSummary 计算网络速率统计
func (p *PodOperatorImpl) calculateNetworkRateSummary(trend []types.NetworkRateDataPoint) types.NetworkRateSummary {
	if len(trend) == 0 {
		return types.NetworkRateSummary{}
	}

	var sumReceive, maxReceive, sumTransmit, maxTransmit float64

	for _, point := range trend {
		sumReceive += point.ReceiveBytesPerSec
		if point.ReceiveBytesPerSec > maxReceive {
			maxReceive = point.ReceiveBytesPerSec
		}
		sumTransmit += point.TransmitBytesPerSec
		if point.TransmitBytesPerSec > maxTransmit {
			maxTransmit = point.TransmitBytesPerSec
		}
	}

	return types.NetworkRateSummary{
		AvgReceiveBytesPerSec:  int64(sumReceive / float64(len(trend))),
		MaxReceiveBytesPerSec:  int64(maxReceive),
		AvgTransmitBytesPerSec: int64(sumTransmit / float64(len(trend))),
		MaxTransmitBytesPerSec: int64(maxTransmit),
	}
}

// calculateDiskRateSummary 计算磁盘速率统计
func (p *PodOperatorImpl) calculateDiskRateSummary(trend []types.DiskRateDataPoint) types.DiskRateSummary {
	if len(trend) == 0 {
		return types.DiskRateSummary{}
	}

	var sumRead, maxRead, sumWrite, maxWrite float64

	for _, point := range trend {
		sumRead += point.ReadBytesPerSec
		if point.ReadBytesPerSec > maxRead {
			maxRead = point.ReadBytesPerSec
		}
		sumWrite += point.WriteBytesPerSec
		if point.WriteBytesPerSec > maxWrite {
			maxWrite = point.WriteBytesPerSec
		}
	}

	return types.DiskRateSummary{
		AvgReadBytesPerSec:  sumRead / float64(len(trend)),
		MaxReadBytesPerSec:  maxRead,
		AvgWriteBytesPerSec: sumWrite / float64(len(trend)),
		MaxWriteBytesPerSec: maxWrite,
	}
}

// formatDuration 格式化时长
func formatDuration(d time.Duration) string {
	days := int(d.Hours() / 24)
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60

	if days > 0 {
		return fmt.Sprintf("%dd%dh%dm", days, hours, minutes)
	} else if hours > 0 {
		return fmt.Sprintf("%dh%dm", hours, minutes)
	} else {
		return fmt.Sprintf("%dm", minutes)
	}
}
