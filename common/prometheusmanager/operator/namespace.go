package operator

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/yanshicheng/kube-nova/common/prometheusmanager/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type NamespaceOperatorImpl struct {
	log logx.Logger
	ctx context.Context
	*BaseOperator
}

func NewNamespaceOperator(ctx context.Context, base *BaseOperator) types.NamespaceOperator {
	return &NamespaceOperatorImpl{
		log:          logx.WithContext(ctx),
		ctx:          ctx,
		BaseOperator: base,
	}
}

// GetNamespaceMetrics 获取 Namespace 综合指标
func (ns *NamespaceOperatorImpl) GetNamespaceMetrics(namespace string, timeRange *types.TimeRange) (*types.NamespaceMetrics, error) {
	ns.log.Infof(" 查询 Namespace 综合指标: namespace=%s", namespace)

	metrics := &types.NamespaceMetrics{
		Namespace: namespace,
		Timestamp: time.Now(),
	}

	// 使用并发查询提升性能
	var wg sync.WaitGroup
	var mu sync.Mutex
	errors := make([]error, 0)

	// CPU 指标
	wg.Add(1)
	go func() {
		defer wg.Done()
		if cpuMetrics, err := ns.GetNamespaceCPU(namespace, timeRange); err == nil {
			mu.Lock()
			metrics.Resources.CPU = *cpuMetrics
			mu.Unlock()
		} else {
			mu.Lock()
			errors = append(errors, fmt.Errorf("CPU: %w", err))
			mu.Unlock()
			ns.log.Errorf("获取 Namespace CPU 指标失败: %v", err)
		}
	}()

	// 内存指标
	wg.Add(1)
	go func() {
		defer wg.Done()
		if memoryMetrics, err := ns.GetNamespaceMemory(namespace, timeRange); err == nil {
			mu.Lock()
			metrics.Resources.Memory = *memoryMetrics
			mu.Unlock()
		} else {
			mu.Lock()
			errors = append(errors, fmt.Errorf("Memory: %w", err))
			mu.Unlock()
			ns.log.Errorf("获取 Namespace 内存指标失败: %v", err)
		}
	}()

	// 配额指标
	wg.Add(1)
	go func() {
		defer wg.Done()
		if quotaMetrics, err := ns.GetNamespaceQuota(namespace); err == nil {
			mu.Lock()
			metrics.Quota = *quotaMetrics
			mu.Unlock()
		} else {
			mu.Lock()
			errors = append(errors, fmt.Errorf("Quota: %w", err))
			mu.Unlock()
			ns.log.Errorf("获取 Namespace 配额失败: %v", err)
		}
	}()

	// 工作负载指标
	wg.Add(1)
	go func() {
		defer wg.Done()
		if workloadMetrics, err := ns.GetNamespaceWorkloads(namespace, timeRange); err == nil {
			mu.Lock()
			metrics.Workloads = *workloadMetrics
			mu.Unlock()
		} else {
			mu.Lock()
			errors = append(errors, fmt.Errorf("Workloads: %w", err))
			mu.Unlock()
			ns.log.Errorf("获取 Namespace 工作负载失败: %v", err)
		}
	}()

	// 网络指标
	wg.Add(1)
	go func() {
		defer wg.Done()
		if networkMetrics, err := ns.GetNamespaceNetwork(namespace, timeRange); err == nil {
			mu.Lock()
			metrics.Network = *networkMetrics
			mu.Unlock()
		} else {
			mu.Lock()
			errors = append(errors, fmt.Errorf("Network: %w", err))
			mu.Unlock()
			ns.log.Errorf("获取 Namespace 网络指标失败: %v", err)
		}
	}()

	// 存储指标
	wg.Add(1)
	go func() {
		defer wg.Done()
		if storageMetrics, err := ns.GetNamespaceStorage(namespace); err == nil {
			mu.Lock()
			metrics.Storage = *storageMetrics
			mu.Unlock()
		} else {
			mu.Lock()
			errors = append(errors, fmt.Errorf("Storage: %w", err))
			mu.Unlock()
			ns.log.Errorf("获取 Namespace 存储失败: %v", err)
		}
	}()

	// 配置指标
	wg.Add(1)
	go func() {
		defer wg.Done()
		if configMetrics, err := ns.GetNamespaceConfig(namespace); err == nil {
			mu.Lock()
			metrics.Config = *configMetrics
			mu.Unlock()
		} else {
			mu.Lock()
			errors = append(errors, fmt.Errorf("Config: %w", err))
			mu.Unlock()
			ns.log.Errorf("获取 Namespace 配置失败: %v", err)
		}
	}()

	wg.Wait()

	if len(errors) > 0 {
		ns.log.Errorf("部分指标查询失败: %v", errors)
	}

	return metrics, nil
}

func (ns *NamespaceOperatorImpl) GetNamespaceCPU(namespace string, timeRange *types.TimeRange) (*types.NamespaceCPUMetrics, error) {

	metrics := &types.NamespaceCPUMetrics{}
	window := ns.calculateRateWindow(timeRange)

	cpuMetricType := ns.detectCPUMetricType(namespace, window)
	ns.log.Infof("检测到 CPU 指标类型: %s", cpuMetricType)
	var usageQuery string
	switch cpuMetricType {
	case "user_system":
		usageQuery = fmt.Sprintf(`sum(rate(container_cpu_user_seconds_total{namespace="%s",container!=""}[%s]) + rate(container_cpu_system_seconds_total{namespace="%s",container!=""}[%s]))`, namespace, window, namespace, window)
	case "user":
		usageQuery = fmt.Sprintf(`sum(rate(container_cpu_user_seconds_total{namespace="%s",container!=""}[%s]))`, namespace, window)
	default:
		usageQuery = fmt.Sprintf(`sum(rate(container_cpu_usage_seconds_total{namespace="%s",container!=""}[%s]))`, namespace, window)
	}

	// 批量查询所有即时指标
	queries := map[string]string{
		"usage":      usageQuery,
		"requests":   fmt.Sprintf(`sum(kube_pod_container_resource_requests{namespace="%s",resource="cpu"})`, namespace),
		"limits":     fmt.Sprintf(`sum(kube_pod_container_resource_limits{namespace="%s",resource="cpu"})`, namespace),
		"noRequests": fmt.Sprintf(`count(kube_pod_container_resource_requests{namespace="%s",resource="cpu"} == 0)`, namespace),
		"totalCont":  fmt.Sprintf(`count(kube_pod_container_info{namespace="%s",container!=""})`, namespace),
		"withLimits": fmt.Sprintf(`count(kube_pod_container_resource_limits{namespace="%s",resource="cpu"})`, namespace),
	}

	results := ns.batchQuery(queries)

	// 解析当前使用
	if val, ok := results["usage"]; ok && len(val) > 0 {
		metrics.Current.UsageCores = val[0].Value
		metrics.Current.Timestamp = val[0].Time
	}

	// 解析 Requests
	if val, ok := results["requests"]; ok && len(val) > 0 {
		metrics.Requests = val[0].Value
		if metrics.Requests > 0 {
			metrics.Current.UsagePercent = (metrics.Current.UsageCores / metrics.Requests) * 100
		}
	}

	// 解析 Limits
	if val, ok := results["limits"]; ok && len(val) > 0 {
		metrics.Limits = val[0].Value
	}

	// 解析无 Requests 的容器数
	if val, ok := results["noRequests"]; ok && len(val) > 0 {
		metrics.ContainersNoRequests = int64(val[0].Value)
	}

	// 计算无 Limits 的容器数
	totalCont := int64(0)
	withLimits := int64(0)
	if val, ok := results["totalCont"]; ok && len(val) > 0 {
		totalCont = int64(val[0].Value)
	}
	if val, ok := results["withLimits"]; ok && len(val) > 0 {
		withLimits = int64(val[0].Value)
	}
	metrics.ContainersNoLimits = totalCont - withLimits

	// 并发查询趋势和 Top 数据
	var wg sync.WaitGroup
	var mu sync.Mutex

	// 趋势数据
	if timeRange != nil && !timeRange.Start.IsZero() && !timeRange.End.IsZero() {
		wg.Add(1)
		go func() {
			defer wg.Done()
			step := timeRange.Step
			if step == "" {
				step = ns.calculateStep(timeRange.Start, timeRange.End)
			}

			//  趋势查询也使用正确的指标
			var trendQuery string
			switch cpuMetricType {
			case "user_system":
				trendQuery = fmt.Sprintf(`sum(rate(container_cpu_user_seconds_total{namespace="%s",container!=""}[%s]) + rate(container_cpu_system_seconds_total{namespace="%s",container!=""}[%s]))`, namespace, window, namespace, window)
			case "user":
				trendQuery = fmt.Sprintf(`sum(rate(container_cpu_user_seconds_total{namespace="%s",container!=""}[%s]))`, namespace, window)
			default:
				trendQuery = fmt.Sprintf(`sum(rate(container_cpu_usage_seconds_total{namespace="%s",container!=""}[%s]))`, namespace, window)
			}

			trendResults, err := ns.queryRange(trendQuery, timeRange.Start, timeRange.End, step)
			if err == nil && len(trendResults) > 0 {
				var sumCores, maxCores float64
				trend := make([]types.NamespaceCPUDataPoint, 0, len(trendResults[0].Values))

				for _, v := range trendResults[0].Values {
					usagePercent := 0.0
					if metrics.Requests > 0 {
						usagePercent = (v.Value / metrics.Requests) * 100
					}

					trend = append(trend, types.NamespaceCPUDataPoint{
						Timestamp:    v.Timestamp,
						UsageCores:   v.Value,
						UsagePercent: usagePercent,
					})

					sumCores += v.Value
					if v.Value > maxCores {
						maxCores = v.Value
					}
				}

				mu.Lock()
				metrics.Trend = trend
				if len(trend) > 0 {
					metrics.Current.AvgCores = sumCores / float64(len(trend))
					metrics.Current.MaxCores = maxCores
				}
				mu.Unlock()
			}
		}()
	}

	wg.Add(1)
	go func() {
		defer wg.Done()

		var topPodsQuery string
		switch cpuMetricType {
		case "user_system":
			topPodsQuery = fmt.Sprintf(`topk(10, sum by (pod) (rate(container_cpu_user_seconds_total{namespace="%s",container!=""}[%s]) + rate(container_cpu_system_seconds_total{namespace="%s",container!=""}[%s])))`, namespace, window, namespace, window)
		case "user":
			topPodsQuery = fmt.Sprintf(`topk(10, sum by (pod) (rate(container_cpu_user_seconds_total{namespace="%s",container!=""}[%s])))`, namespace, window)
		default:
			topPodsQuery = fmt.Sprintf(`topk(10, sum by (pod) (rate(container_cpu_usage_seconds_total{namespace="%s",container!=""}[%s])))`, namespace, window)
		}

		if topPodsResults, err := ns.query(topPodsQuery, nil); err == nil {
			topPods := make([]types.ResourceRanking, 0, len(topPodsResults))
			for _, result := range topPodsResults {
				if pod, ok := result.Metric["pod"]; ok {
					topPods = append(topPods, types.ResourceRanking{
						Name:  pod,
						Value: result.Value,
						Unit:  "cores",
					})
				}
			}
			mu.Lock()
			metrics.TopPods = topPods
			mu.Unlock()
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()

		var topContainersQuery string
		switch cpuMetricType {
		case "user_system":
			topContainersQuery = fmt.Sprintf(`topk(10, rate(container_cpu_user_seconds_total{namespace="%s",container!=""}[%s]) + rate(container_cpu_system_seconds_total{namespace="%s",container!=""}[%s]))`, namespace, window, namespace, window)
		case "user":
			topContainersQuery = fmt.Sprintf(`topk(10, rate(container_cpu_user_seconds_total{namespace="%s",container!=""}[%s]))`, namespace, window)
		default:
			topContainersQuery = fmt.Sprintf(`topk(10, rate(container_cpu_usage_seconds_total{namespace="%s",container!=""}[%s]))`, namespace, window)
		}

		if topContainersResults, err := ns.query(topContainersQuery, nil); err == nil {
			topContainers := make([]types.ContainerResourceRanking, 0, len(topContainersResults))
			for _, result := range topContainersResults {
				if pod, ok := result.Metric["pod"]; ok {
					if container, ok := result.Metric["container"]; ok {
						topContainers = append(topContainers, types.ContainerResourceRanking{
							PodName:       pod,
							ContainerName: container,
							Value:         result.Value,
							Unit:          "cores",
						})
					}
				}
			}
			mu.Lock()
			metrics.TopContainers = topContainers
			mu.Unlock()
		}
	}()

	wg.Wait()

	return metrics, nil
}

// detectCPUMetricType 检测使用哪种 CPU 指标（增强版）
func (ns *NamespaceOperatorImpl) detectCPUMetricType(namespace, window string) string {
	// 方法1: 检查标准指标
	query1 := fmt.Sprintf(`sum(rate(container_cpu_usage_seconds_total{namespace="%s",container!=""}[%s]))`, namespace, window)
	if results, err := ns.query(query1, nil); err == nil && len(results) > 0 && results[0].Value > 0 {
		return "standard"
	}

	// 方法2: 检查 user + system
	query2 := fmt.Sprintf(`sum(rate(container_cpu_user_seconds_total{namespace="%s",container!=""}[%s]) + rate(container_cpu_system_seconds_total{namespace="%s",container!=""}[%s]))`, namespace, window, namespace, window)
	if results, err := ns.query(query2, nil); err == nil && len(results) > 0 && results[0].Value > 0 {
		return "user_system"
	}

	// 方法3: 只检查 user
	query3 := fmt.Sprintf(`sum(rate(container_cpu_user_seconds_total{namespace="%s",container!=""}[%s]))`, namespace, window)
	if results, err := ns.query(query3, nil); err == nil && len(results) > 0 && results[0].Value > 0 {
		return "user"
	}

	return "standard"
}

// GetNamespaceMemory 获取 Namespace 内存指标
func (ns *NamespaceOperatorImpl) GetNamespaceMemory(namespace string, timeRange *types.TimeRange) (*types.NamespaceMemoryMetrics, error) {
	ns.log.Infof(" 查询 Namespace 内存: namespace=%s", namespace)

	metrics := &types.NamespaceMemoryMetrics{}

	// 批量查询即时指标
	queries := map[string]string{
		"workingSet": fmt.Sprintf(`sum(container_memory_working_set_bytes{namespace="%s",container!=""})`, namespace),
		"rss":        fmt.Sprintf(`sum(container_memory_rss{namespace="%s",container!=""})`, namespace),
		"cache":      fmt.Sprintf(`sum(container_memory_cache{namespace="%s",container!=""})`, namespace),
		"requests":   fmt.Sprintf(`sum(kube_pod_container_resource_requests{namespace="%s",resource="memory"})`, namespace),
		"limits":     fmt.Sprintf(`sum(kube_pod_container_resource_limits{namespace="%s",resource="memory"})`, namespace),
		"oom":        fmt.Sprintf(`sum(container_oom_events_total{namespace="%s"})`, namespace),
	}

	results := ns.batchQuery(queries)

	// 解析结果
	if val, ok := results["workingSet"]; ok && len(val) > 0 {
		metrics.Current.WorkingSetBytes = int64(val[0].Value)
		metrics.Current.Timestamp = val[0].Time
	}
	if val, ok := results["rss"]; ok && len(val) > 0 {
		metrics.Current.RSSBytes = int64(val[0].Value)
	}
	if val, ok := results["cache"]; ok && len(val) > 0 {
		metrics.Current.CacheBytes = int64(val[0].Value)
	}
	if val, ok := results["requests"]; ok && len(val) > 0 {
		metrics.Requests = int64(val[0].Value)
		if metrics.Requests > 0 {
			metrics.Current.UsagePercent = (float64(metrics.Current.WorkingSetBytes) / float64(metrics.Requests)) * 100
		}
	}
	if val, ok := results["limits"]; ok && len(val) > 0 {
		metrics.Limits = int64(val[0].Value)
	}
	if val, ok := results["oom"]; ok && len(val) > 0 {
		metrics.OOMKills = int64(val[0].Value)
	}

	// 并发查询趋势和 Top 数据
	var wg sync.WaitGroup
	var mu sync.Mutex

	// 趋势数据
	if timeRange != nil && !timeRange.Start.IsZero() && !timeRange.End.IsZero() {
		wg.Add(1)
		go func() {
			defer wg.Done()
			step := timeRange.Step
			if step == "" {
				step = ns.calculateStep(timeRange.Start, timeRange.End)
			}

			workingSetQuery := fmt.Sprintf(`sum(container_memory_working_set_bytes{namespace="%s",container!=""})`, namespace)
			trendResults, err := ns.queryRange(workingSetQuery, timeRange.Start, timeRange.End, step)
			if err == nil && len(trendResults) > 0 {
				var sumBytes, maxBytes int64
				trend := make([]types.NamespaceMemoryDataPoint, 0, len(trendResults[0].Values))

				for _, v := range trendResults[0].Values {
					workingSetBytes := int64(v.Value)
					usagePercent := 0.0
					if metrics.Requests > 0 {
						usagePercent = (float64(workingSetBytes) / float64(metrics.Requests)) * 100
					}

					trend = append(trend, types.NamespaceMemoryDataPoint{
						Timestamp:       v.Timestamp,
						WorkingSetBytes: workingSetBytes,
						UsagePercent:    usagePercent,
					})

					sumBytes += workingSetBytes
					if workingSetBytes > maxBytes {
						maxBytes = workingSetBytes
					}
				}

				mu.Lock()
				metrics.Trend = trend
				if len(trend) > 0 {
					metrics.Current.AvgBytes = sumBytes / int64(len(trend))
					metrics.Current.MaxBytes = maxBytes
				}
				mu.Unlock()
			}
		}()
	}

	// Top Pods
	wg.Add(1)
	go func() {
		defer wg.Done()
		topPodsQuery := fmt.Sprintf(`topk(10, sum by (pod) (container_memory_working_set_bytes{namespace="%s",container!=""}))`, namespace)
		if topPodsResults, err := ns.query(topPodsQuery, nil); err == nil {
			topPods := make([]types.ResourceRanking, 0, len(topPodsResults))
			for _, result := range topPodsResults {
				if pod, ok := result.Metric["pod"]; ok {
					topPods = append(topPods, types.ResourceRanking{
						Name:  pod,
						Value: result.Value,
						Unit:  "bytes",
					})
				}
			}
			mu.Lock()
			metrics.TopPods = topPods
			mu.Unlock()
		}
	}()

	// Top Containers
	wg.Add(1)
	go func() {
		defer wg.Done()
		topContainersQuery := fmt.Sprintf(`topk(10, container_memory_working_set_bytes{namespace="%s",container!=""})`, namespace)
		if topContainersResults, err := ns.query(topContainersQuery, nil); err == nil {
			topContainers := make([]types.ContainerResourceRanking, 0, len(topContainersResults))
			for _, result := range topContainersResults {
				if pod, ok := result.Metric["pod"]; ok {
					if container, ok := result.Metric["container"]; ok {
						topContainers = append(topContainers, types.ContainerResourceRanking{
							PodName:       pod,
							ContainerName: container,
							Value:         result.Value,
							Unit:          "bytes",
						})
					}
				}
			}
			mu.Lock()
			metrics.TopContainers = topContainers
			mu.Unlock()
		}
	}()

	wg.Wait()

	ns.log.Infof(" Namespace 内存查询完成: namespace=%s, usage=%d bytes", namespace, metrics.Current.WorkingSetBytes)
	return metrics, nil
}

// GetNamespaceNetwork 获取 Namespace 网络指标
func (ns *NamespaceOperatorImpl) GetNamespaceNetwork(namespace string, timeRange *types.TimeRange) (*types.NamespaceNetworkMetrics, error) {

	metrics := &types.NamespaceNetworkMetrics{}
	window := ns.calculateRateWindow(timeRange)

	// 批量查询即时指标
	queries := map[string]string{
		"rx":        fmt.Sprintf(`sum(rate(container_network_receive_bytes_total{namespace="%s"}[%s]))`, namespace, window),
		"tx":        fmt.Sprintf(`sum(rate(container_network_transmit_bytes_total{namespace="%s"}[%s]))`, namespace, window),
		"rxPackets": fmt.Sprintf(`sum(rate(container_network_receive_packets_total{namespace="%s"}[%s]))`, namespace, window),
		"txPackets": fmt.Sprintf(`sum(rate(container_network_transmit_packets_total{namespace="%s"}[%s]))`, namespace, window),
		"rxErrors":  fmt.Sprintf(`sum(rate(container_network_receive_errors_total{namespace="%s"}[%s]))`, namespace, window),
		"txErrors":  fmt.Sprintf(`sum(rate(container_network_transmit_errors_total{namespace="%s"}[%s]))`, namespace, window),
		"rxDrops":   fmt.Sprintf(`sum(rate(container_network_receive_packets_dropped_total{namespace="%s"}[%s]))`, namespace, window),
		"txDrops":   fmt.Sprintf(`sum(rate(container_network_transmit_packets_dropped_total{namespace="%s"}[%s]))`, namespace, window),
		"totalRx":   fmt.Sprintf(`sum(container_network_receive_bytes_total{namespace="%s"})`, namespace),
		"totalTx":   fmt.Sprintf(`sum(container_network_transmit_bytes_total{namespace="%s"})`, namespace),
	}

	results := ns.batchQuery(queries)

	// 解析结果
	if val, ok := results["rx"]; ok && len(val) > 0 {
		metrics.Current.ReceiveBytesPerSec = val[0].Value
		metrics.Current.Timestamp = val[0].Time
	}
	if val, ok := results["tx"]; ok && len(val) > 0 {
		metrics.Current.TransmitBytesPerSec = val[0].Value
	}
	if val, ok := results["rxPackets"]; ok && len(val) > 0 {
		metrics.Current.ReceivePacketsPerSec = val[0].Value
	}
	if val, ok := results["txPackets"]; ok && len(val) > 0 {
		metrics.Current.TransmitPacketsPerSec = val[0].Value
	}
	if val, ok := results["rxErrors"]; ok && len(val) > 0 {
		metrics.Current.ReceiveErrors = val[0].Value
	}
	if val, ok := results["txErrors"]; ok && len(val) > 0 {
		metrics.Current.TransmitErrors = val[0].Value
	}
	if val, ok := results["rxDrops"]; ok && len(val) > 0 {
		metrics.Current.ReceiveDrops = val[0].Value
	}
	if val, ok := results["txDrops"]; ok && len(val) > 0 {
		metrics.Current.TransmitDrops = val[0].Value
	}
	if val, ok := results["totalRx"]; ok && len(val) > 0 {
		metrics.Summary.TotalReceiveBytes = int64(val[0].Value)
	}
	if val, ok := results["totalTx"]; ok && len(val) > 0 {
		metrics.Summary.TotalTransmitBytes = int64(val[0].Value)
	}

	metrics.Summary.TotalErrors = int64(metrics.Current.ReceiveErrors + metrics.Current.TransmitErrors)

	// 并发查询趋势和 Top 数据
	var wg sync.WaitGroup
	var mu sync.Mutex

	// 趋势数据
	if timeRange != nil && !timeRange.Start.IsZero() && !timeRange.End.IsZero() {
		wg.Add(1)
		go func() {
			defer wg.Done()
			step := timeRange.Step
			if step == "" {
				step = ns.calculateStep(timeRange.Start, timeRange.End)
			}

			// 并行查询 RX 和 TX 趋势
			var trendWg sync.WaitGroup
			var trendMu sync.Mutex
			trend := make([]types.NamespaceNetworkDataPoint, 0)

			receiveQuery := fmt.Sprintf(`sum(rate(container_network_receive_bytes_total{namespace="%s"}[%s]))`, namespace, window)
			transmitQuery := fmt.Sprintf(`sum(rate(container_network_transmit_bytes_total{namespace="%s"}[%s]))`, namespace, window)

			trendWg.Add(1)
			go func() {
				defer trendWg.Done()
				receiveTrend, err := ns.queryRange(receiveQuery, timeRange.Start, timeRange.End, step)
				if err == nil && len(receiveTrend) > 0 {
					trendMu.Lock()
					trend = make([]types.NamespaceNetworkDataPoint, len(receiveTrend[0].Values))
					for i, v := range receiveTrend[0].Values {
						trend[i].Timestamp = v.Timestamp
						trend[i].ReceiveBytesPerSec = v.Value
					}
					trendMu.Unlock()
				}
			}()

			trendWg.Add(1)
			go func() {
				defer trendWg.Done()
				transmitTrend, err := ns.queryRange(transmitQuery, timeRange.Start, timeRange.End, step)
				if err == nil && len(transmitTrend) > 0 {
					trendMu.Lock()
					for i, v := range transmitTrend[0].Values {
						if i < len(trend) {
							trend[i].TransmitBytesPerSec = v.Value
						}
					}
					trendMu.Unlock()
				}
			}()

			trendWg.Wait()

			// 计算统计
			if len(trend) > 0 {
				var sumRx, maxRx, sumTx, maxTx float64
				for _, point := range trend {
					sumRx += point.ReceiveBytesPerSec
					sumTx += point.TransmitBytesPerSec
					if point.ReceiveBytesPerSec > maxRx {
						maxRx = point.ReceiveBytesPerSec
					}
					if point.TransmitBytesPerSec > maxTx {
						maxTx = point.TransmitBytesPerSec
					}
				}

				mu.Lock()
				metrics.Trend = trend
				metrics.Summary.AvgReceiveBytesPerSec = sumRx / float64(len(trend))
				metrics.Summary.MaxReceiveBytesPerSec = maxRx
				metrics.Summary.AvgTransmitBytesPerSec = sumTx / float64(len(trend))
				metrics.Summary.MaxTransmitBytesPerSec = maxTx
				mu.Unlock()
			}
		}()
	}

	// Top Pods by Rx
	wg.Add(1)
	go func() {
		defer wg.Done()
		topRxQuery := fmt.Sprintf(`topk(10, sum by (pod) (rate(container_network_receive_bytes_total{namespace="%s"}[%s])))`, namespace, window)
		if topRxResults, err := ns.query(topRxQuery, nil); err == nil {
			topPods := make([]types.ResourceRanking, 0, len(topRxResults))
			for _, result := range topRxResults {
				if pod, ok := result.Metric["pod"]; ok {
					topPods = append(topPods, types.ResourceRanking{
						Name:  pod,
						Value: result.Value,
						Unit:  "bytes/s",
					})
				}
			}
			mu.Lock()
			metrics.TopPodsByRx = topPods
			mu.Unlock()
		}
	}()

	// Top Pods by Tx
	wg.Add(1)
	go func() {
		defer wg.Done()
		topTxQuery := fmt.Sprintf(`topk(10, sum by (pod) (rate(container_network_transmit_bytes_total{namespace="%s"}[%s])))`, namespace, window)
		if topTxResults, err := ns.query(topTxQuery, nil); err == nil {
			topPods := make([]types.ResourceRanking, 0, len(topTxResults))
			for _, result := range topTxResults {
				if pod, ok := result.Metric["pod"]; ok {
					topPods = append(topPods, types.ResourceRanking{
						Name:  pod,
						Value: result.Value,
						Unit:  "bytes/s",
					})
				}
			}
			mu.Lock()
			metrics.TopPodsByTx = topPods
			mu.Unlock()
		}
	}()

	wg.Wait()

	return metrics, nil
}

// GetNamespaceQuota 获取 Namespace 配额
func (ns *NamespaceOperatorImpl) GetNamespaceQuota(namespace string) (*types.NamespaceQuotaMetrics, error) {

	metrics := &types.NamespaceQuotaMetrics{}

	// 检查是否有配额
	quotaQuery := fmt.Sprintf(`kube_resourcequota{namespace="%s"}`, namespace)
	quotaResults, err := ns.query(quotaQuery, nil)
	if err != nil || len(quotaResults) == 0 {
		ns.log.Infof("Namespace 无配额: namespace=%s", namespace)
		metrics.HasQuota = false
		return metrics, nil
	}

	metrics.HasQuota = true

	// 批量查询所有配额指标
	queries := map[string]string{
		"cpuHard":        fmt.Sprintf(`sum(kube_resourcequota{namespace="%s",resource="requests.cpu",type="hard"})`, namespace),
		"cpuUsed":        fmt.Sprintf(`sum(kube_resourcequota{namespace="%s",resource="requests.cpu",type="used"})`, namespace),
		"memHard":        fmt.Sprintf(`sum(kube_resourcequota{namespace="%s",resource="requests.memory",type="hard"})`, namespace),
		"memUsed":        fmt.Sprintf(`sum(kube_resourcequota{namespace="%s",resource="requests.memory",type="used"})`, namespace),
		"podsHard":       fmt.Sprintf(`sum(kube_resourcequota{namespace="%s",resource="pods",type="hard"})`, namespace),
		"podsUsed":       fmt.Sprintf(`sum(kube_resourcequota{namespace="%s",resource="pods",type="used"})`, namespace),
		"servicesHard":   fmt.Sprintf(`sum(kube_resourcequota{namespace="%s",resource="services",type="hard"})`, namespace),
		"servicesUsed":   fmt.Sprintf(`sum(kube_resourcequota{namespace="%s",resource="services",type="used"})`, namespace),
		"configMapsHard": fmt.Sprintf(`sum(kube_resourcequota{namespace="%s",resource="configmaps",type="hard"})`, namespace),
		"configMapsUsed": fmt.Sprintf(`sum(kube_resourcequota{namespace="%s",resource="configmaps",type="used"})`, namespace),
		"secretsHard":    fmt.Sprintf(`sum(kube_resourcequota{namespace="%s",resource="secrets",type="hard"})`, namespace),
		"secretsUsed":    fmt.Sprintf(`sum(kube_resourcequota{namespace="%s",resource="secrets",type="used"})`, namespace),
		"pvcsHard":       fmt.Sprintf(`sum(kube_resourcequota{namespace="%s",resource="persistentvolumeclaims",type="hard"})`, namespace),
		"pvcsUsed":       fmt.Sprintf(`sum(kube_resourcequota{namespace="%s",resource="persistentvolumeclaims",type="used"})`, namespace),
		"storageHard":    fmt.Sprintf(`sum(kube_resourcequota{namespace="%s",resource="requests.storage",type="hard"})`, namespace),
		"storageUsed":    fmt.Sprintf(`sum(kube_resourcequota{namespace="%s",resource="requests.storage",type="used"})`, namespace),
	}

	results := ns.batchQuery(queries)

	// CPU 配额
	if val, ok := results["cpuHard"]; ok && len(val) > 0 {
		metrics.CPU.Hard = val[0].Value
		metrics.CPU.HasQuota = true
	}
	if val, ok := results["cpuUsed"]; ok && len(val) > 0 {
		metrics.CPU.Used = val[0].Value
		if metrics.CPU.Hard > 0 {
			metrics.CPU.UsagePercent = (metrics.CPU.Used / metrics.CPU.Hard) * 100
		}
	}

	// 内存配额
	if val, ok := results["memHard"]; ok && len(val) > 0 {
		metrics.Memory.Hard = val[0].Value
		metrics.Memory.HasQuota = true
	}
	if val, ok := results["memUsed"]; ok && len(val) > 0 {
		metrics.Memory.Used = val[0].Value
		if metrics.Memory.Hard > 0 {
			metrics.Memory.UsagePercent = (metrics.Memory.Used / metrics.Memory.Hard) * 100
		}
	}

	// Pods 配额
	if val, ok := results["podsHard"]; ok && len(val) > 0 {
		metrics.Pods.Hard = val[0].Value
		metrics.Pods.HasQuota = true
	}
	if val, ok := results["podsUsed"]; ok && len(val) > 0 {
		metrics.Pods.Used = val[0].Value
		if metrics.Pods.Hard > 0 {
			metrics.Pods.UsagePercent = (metrics.Pods.Used / metrics.Pods.Hard) * 100
		}
	}

	// Services 配额
	if val, ok := results["servicesHard"]; ok && len(val) > 0 {
		metrics.Services.Hard = val[0].Value
		metrics.Services.HasQuota = true
	}
	if val, ok := results["servicesUsed"]; ok && len(val) > 0 {
		metrics.Services.Used = val[0].Value
		if metrics.Services.Hard > 0 {
			metrics.Services.UsagePercent = (metrics.Services.Used / metrics.Services.Hard) * 100
		}
	}

	// ConfigMaps 配额
	if val, ok := results["configMapsHard"]; ok && len(val) > 0 {
		metrics.ConfigMaps.Hard = val[0].Value
		metrics.ConfigMaps.HasQuota = true
	}
	if val, ok := results["configMapsUsed"]; ok && len(val) > 0 {
		metrics.ConfigMaps.Used = val[0].Value
		if metrics.ConfigMaps.Hard > 0 {
			metrics.ConfigMaps.UsagePercent = (metrics.ConfigMaps.Used / metrics.ConfigMaps.Hard) * 100
		}
	}

	// Secrets 配额
	if val, ok := results["secretsHard"]; ok && len(val) > 0 {
		metrics.Secrets.Hard = val[0].Value
		metrics.Secrets.HasQuota = true
	}
	if val, ok := results["secretsUsed"]; ok && len(val) > 0 {
		metrics.Secrets.Used = val[0].Value
		if metrics.Secrets.Hard > 0 {
			metrics.Secrets.UsagePercent = (metrics.Secrets.Used / metrics.Secrets.Hard) * 100
		}
	}

	// PVCs 配额
	if val, ok := results["pvcsHard"]; ok && len(val) > 0 {
		metrics.PVCs.Hard = val[0].Value
		metrics.PVCs.HasQuota = true
	}
	if val, ok := results["pvcsUsed"]; ok && len(val) > 0 {
		metrics.PVCs.Used = val[0].Value
		if metrics.PVCs.Hard > 0 {
			metrics.PVCs.UsagePercent = (metrics.PVCs.Used / metrics.PVCs.Hard) * 100
		}
	}

	// Storage 配额
	if val, ok := results["storageHard"]; ok && len(val) > 0 {
		metrics.Storage.Hard = val[0].Value
		metrics.Storage.HasQuota = true
	}
	if val, ok := results["storageUsed"]; ok && len(val) > 0 {
		metrics.Storage.Used = val[0].Value
		if metrics.Storage.Hard > 0 {
			metrics.Storage.UsagePercent = (metrics.Storage.Used / metrics.Storage.Hard) * 100
		}
	}

	return metrics, nil
}

// GetNamespaceWorkloads 获取 Namespace 工作负载
func (ns *NamespaceOperatorImpl) GetNamespaceWorkloads(namespace string, timeRange *types.TimeRange) (*types.NamespaceWorkloadMetrics, error) {

	metrics := &types.NamespaceWorkloadMetrics{}

	// 并发查询各个工作负载类型
	var wg sync.WaitGroup
	var mu sync.Mutex

	// Pod 统计
	wg.Add(1)
	go func() {
		defer wg.Done()
		if podStats, err := ns.GetNamespacePods(namespace, timeRange); err == nil {
			mu.Lock()
			metrics.Pods = *podStats
			mu.Unlock()
		} else {
			ns.log.Errorf("获取 Pod 统计失败: %v", err)
		}
	}()

	// Deployment 统计
	wg.Add(1)
	go func() {
		defer wg.Done()
		if deploymentStats, err := ns.GetNamespaceDeployments(namespace); err == nil {
			mu.Lock()
			metrics.Deployments = *deploymentStats
			mu.Unlock()
		} else {
			ns.log.Errorf("获取 Deployment 统计失败: %v", err)
		}
	}()

	// Service 统计
	wg.Add(1)
	go func() {
		defer wg.Done()
		if serviceStats, err := ns.GetNamespaceServices(namespace); err == nil {
			mu.Lock()
			metrics.Services = *serviceStats
			mu.Unlock()
		} else {
			ns.log.Errorf("获取 Service 统计失败: %v", err)
		}
	}()

	// 其他工作负载统计（批量查询）
	wg.Add(1)
	go func() {
		defer wg.Done()
		queries := map[string]string{
			// StatefulSet
			"stsTotal":   fmt.Sprintf(`count(kube_statefulset_created{namespace="%s"})`, namespace),
			"stsReady":   fmt.Sprintf(`sum(kube_statefulset_status_replicas_ready{namespace="%s"})`, namespace),
			"stsCurrent": fmt.Sprintf(`sum(kube_statefulset_status_replicas_current{namespace="%s"})`, namespace),
			"stsDesired": fmt.Sprintf(`sum(kube_statefulset_replicas{namespace="%s"})`, namespace),
			// DaemonSet
			"dsTotal":       fmt.Sprintf(`count(kube_daemonset_created{namespace="%s"})`, namespace),
			"dsDesired":     fmt.Sprintf(`sum(kube_daemonset_status_desired_number_scheduled{namespace="%s"})`, namespace),
			"dsCurrent":     fmt.Sprintf(`sum(kube_daemonset_status_current_number_scheduled{namespace="%s"})`, namespace),
			"dsAvailable":   fmt.Sprintf(`sum(kube_daemonset_status_number_available{namespace="%s"})`, namespace),
			"dsUnavailable": fmt.Sprintf(`sum(kube_daemonset_status_number_unavailable{namespace="%s"})`, namespace),
			// Job
			"jobActive":    fmt.Sprintf(`count(kube_job_status_active{namespace="%s"} > 0)`, namespace),
			"jobSucceeded": fmt.Sprintf(`count(kube_job_status_succeeded{namespace="%s"} > 0)`, namespace),
			"jobFailed":    fmt.Sprintf(`count(kube_job_status_failed{namespace="%s"} > 0)`, namespace),
			"cronJob":      fmt.Sprintf(`count(kube_cronjob_created{namespace="%s"})`, namespace),
			// Container
			"containerTotal":      fmt.Sprintf(`count(kube_pod_container_info{namespace="%s",container!=""})`, namespace),
			"containerRunning":    fmt.Sprintf(`count(kube_pod_container_status_running{namespace="%s"} == 1)`, namespace),
			"containerWaiting":    fmt.Sprintf(`count(kube_pod_container_status_waiting{namespace="%s"} == 1)`, namespace),
			"containerTerminated": fmt.Sprintf(`count(kube_pod_container_status_terminated{namespace="%s"} == 1)`, namespace),
			// Endpoint
			"endpoint":         fmt.Sprintf(`sum(kube_endpoint_address_available{namespace="%s"})`, namespace),
			"endpointServices": fmt.Sprintf(`count(count by (service) (kube_endpoint_address_available{namespace="%s"}))`, namespace),
			// Ingress
			"ingress":     fmt.Sprintf(`count(kube_ingress_created{namespace="%s"})`, namespace),
			"ingressPath": fmt.Sprintf(`count(kube_ingress_path{namespace="%s"})`, namespace),
		}

		results := ns.batchQuery(queries)

		mu.Lock()
		defer mu.Unlock()

		// StatefulSet
		if val, ok := results["stsTotal"]; ok && len(val) > 0 {
			metrics.StatefulSets.Total = int64(val[0].Value)
		}
		if val, ok := results["stsReady"]; ok && len(val) > 0 {
			metrics.StatefulSets.ReadyReplicas = int64(val[0].Value)
		}
		if val, ok := results["stsCurrent"]; ok && len(val) > 0 {
			metrics.StatefulSets.CurrentReplicas = int64(val[0].Value)
		}
		if val, ok := results["stsDesired"]; ok && len(val) > 0 {
			metrics.StatefulSets.DesiredReplicas = int64(val[0].Value)
		}

		// DaemonSet
		if val, ok := results["dsTotal"]; ok && len(val) > 0 {
			metrics.DaemonSets.Total = int64(val[0].Value)
		}
		if val, ok := results["dsDesired"]; ok && len(val) > 0 {
			metrics.DaemonSets.DesiredScheduled = int64(val[0].Value)
		}
		if val, ok := results["dsCurrent"]; ok && len(val) > 0 {
			metrics.DaemonSets.CurrentScheduled = int64(val[0].Value)
		}
		if val, ok := results["dsAvailable"]; ok && len(val) > 0 {
			metrics.DaemonSets.AvailableScheduled = int64(val[0].Value)
		}
		if val, ok := results["dsUnavailable"]; ok && len(val) > 0 {
			metrics.DaemonSets.UnavailableScheduled = int64(val[0].Value)
		}

		// Job
		if val, ok := results["jobActive"]; ok && len(val) > 0 {
			metrics.Jobs.Active = int64(val[0].Value)
		}
		if val, ok := results["jobSucceeded"]; ok && len(val) > 0 {
			metrics.Jobs.Succeeded = int64(val[0].Value)
		}
		if val, ok := results["jobFailed"]; ok && len(val) > 0 {
			metrics.Jobs.Failed = int64(val[0].Value)
		}
		if val, ok := results["cronJob"]; ok && len(val) > 0 {
			metrics.Jobs.CronJobsTotal = int64(val[0].Value)
		}

		// Container
		if val, ok := results["containerTotal"]; ok && len(val) > 0 {
			metrics.Containers.Total = int64(val[0].Value)
		}
		if val, ok := results["containerRunning"]; ok && len(val) > 0 {
			metrics.Containers.Running = int64(val[0].Value)
		}
		if val, ok := results["containerWaiting"]; ok && len(val) > 0 {
			metrics.Containers.Waiting = int64(val[0].Value)
		}
		if val, ok := results["containerTerminated"]; ok && len(val) > 0 {
			metrics.Containers.Terminated = int64(val[0].Value)
		}

		// Endpoint
		if val, ok := results["endpoint"]; ok && len(val) > 0 {
			metrics.Endpoints.TotalAddresses = int64(val[0].Value)
		}
		if val, ok := results["endpointServices"]; ok && len(val) > 0 {
			metrics.Endpoints.ServicesWithEndpoints = int64(val[0].Value)
		}

		// Ingress
		if val, ok := results["ingress"]; ok && len(val) > 0 {
			metrics.Ingresses.Total = int64(val[0].Value)
		}
		if val, ok := results["ingressPath"]; ok && len(val) > 0 {
			metrics.Ingresses.PathCount = int64(val[0].Value)
		}
	}()

	wg.Wait()

	return metrics, nil
}

func (ns *NamespaceOperatorImpl) GetNamespacePods(namespace string, timeRange *types.TimeRange) (*types.NamespacePodStatistics, error) {
	stats := &types.NamespacePodStatistics{}

	// 批量查询所有 Pod 指标
	queries := map[string]string{
		"total":     fmt.Sprintf(`count(kube_pod_info{namespace="%s"})`, namespace),
		"running":   fmt.Sprintf(`count(kube_pod_status_phase{namespace="%s",phase="Running"} == 1)`, namespace),
		"pending":   fmt.Sprintf(`count(kube_pod_status_phase{namespace="%s",phase="Pending"} == 1)`, namespace),
		"failed":    fmt.Sprintf(`count(kube_pod_status_phase{namespace="%s",phase="Failed"} == 1)`, namespace),
		"succeeded": fmt.Sprintf(`count(kube_pod_status_phase{namespace="%s",phase="Succeeded"} == 1)`, namespace),
		"unknown":   fmt.Sprintf(`count(kube_pod_status_phase{namespace="%s",phase="Unknown"} == 1)`, namespace),
		"ready":     fmt.Sprintf(`sum(kube_pod_status_ready{namespace="%s"} == 1)`, namespace),
		"restarts":  fmt.Sprintf(`sum(kube_pod_container_status_restarts_total{namespace="%s"})`, namespace),
	}

	results := ns.batchQuery(queries)

	// 解析结果
	if val, ok := results["total"]; ok && len(val) > 0 {
		stats.Total = int64(val[0].Value)
	}
	if val, ok := results["running"]; ok && len(val) > 0 {
		stats.Running = int64(val[0].Value)
	}
	if val, ok := results["pending"]; ok && len(val) > 0 {
		stats.Pending = int64(val[0].Value)
	}
	if val, ok := results["failed"]; ok && len(val) > 0 {
		stats.Failed = int64(val[0].Value)
	}
	if val, ok := results["succeeded"]; ok && len(val) > 0 {
		stats.Succeeded = int64(val[0].Value)
	}
	if val, ok := results["unknown"]; ok && len(val) > 0 {
		stats.Unknown = int64(val[0].Value)
	}
	if val, ok := results["ready"]; ok && len(val) > 0 {
		stats.Ready = int64(val[0].Value)
		stats.NotReady = stats.Total - stats.Ready
	}
	if val, ok := results["restarts"]; ok && len(val) > 0 {
		stats.TotalRestarts = int64(val[0].Value)
	}

	// 高频重启 Pods
	highRestartQuery := fmt.Sprintf(`topk(10, sum by (pod) (kube_pod_container_status_restarts_total{namespace="%s"}))`, namespace)
	if highRestartResults, err := ns.query(highRestartQuery, nil); err == nil {
		stats.HighRestartPods = make([]types.PodRestartInfo, 0, len(highRestartResults))
		for _, result := range highRestartResults {
			if pod, ok := result.Metric["pod"]; ok {
				stats.HighRestartPods = append(stats.HighRestartPods, types.PodRestartInfo{
					Namespace:    namespace,
					PodName:      pod,
					RestartCount: int64(result.Value),
				})
			}
		}
	}

	// 趋势数据（并发查询）
	if timeRange != nil && !timeRange.Start.IsZero() && !timeRange.End.IsZero() {
		step := timeRange.Step
		if step == "" {
			step = ns.calculateStep(timeRange.Start, timeRange.End)
		}

		var wg sync.WaitGroup
		var mu sync.Mutex

		trendQueries := map[string]string{
			"total":   fmt.Sprintf(`count(kube_pod_info{namespace="%s"})`, namespace),
			"running": fmt.Sprintf(`count(kube_pod_status_phase{namespace="%s",phase="Running"} == 1)`, namespace),
			"pending": fmt.Sprintf(`count(kube_pod_status_phase{namespace="%s",phase="Pending"} == 1)`, namespace),
			"failed":  fmt.Sprintf(`count(kube_pod_status_phase{namespace="%s",phase="Failed"} == 1)`, namespace),
		}

		trendResults := make(map[string][]types.RangeQueryResult)
		for name, query := range trendQueries {
			name := name
			query := query
			wg.Add(1)
			go func() {
				defer wg.Done()
				if result, err := ns.queryRange(query, timeRange.Start, timeRange.End, step); err == nil {
					mu.Lock()
					trendResults[name] = result
					mu.Unlock()
				}
			}()
		}

		wg.Wait()

		// 合并趋势数据
		if totalTrend, ok := trendResults["total"]; ok && len(totalTrend) > 0 {
			stats.Trend = make([]types.NamespacePodDataPoint, len(totalTrend[0].Values))
			for i, v := range totalTrend[0].Values {
				stats.Trend[i].Timestamp = v.Timestamp
				stats.Trend[i].Total = int64(v.Value)
			}

			if runningTrend, ok := trendResults["running"]; ok && len(runningTrend) > 0 {
				for i, v := range runningTrend[0].Values {
					if i < len(stats.Trend) {
						stats.Trend[i].Running = int64(v.Value)
					}
				}
			}

			if pendingTrend, ok := trendResults["pending"]; ok && len(pendingTrend) > 0 {
				for i, v := range pendingTrend[0].Values {
					if i < len(stats.Trend) {
						stats.Trend[i].Pending = int64(v.Value)
					}
				}
			}

			if failedTrend, ok := trendResults["failed"]; ok && len(failedTrend) > 0 {
				for i, v := range failedTrend[0].Values {
					if i < len(stats.Trend) {
						stats.Trend[i].Failed = int64(v.Value)
					}
				}
			}
		}
	}

	return stats, nil
}

// GetNamespaceDeployments 获取 Namespace Deployment 统计
func (ns *NamespaceOperatorImpl) GetNamespaceDeployments(namespace string) (*types.NamespaceDeploymentStatistics, error) {
	stats := &types.NamespaceDeploymentStatistics{}

	queries := map[string]string{
		"total":       fmt.Sprintf(`count(kube_deployment_created{namespace="%s"})`, namespace),
		"desired":     fmt.Sprintf(`sum(kube_deployment_spec_replicas{namespace="%s"})`, namespace),
		"available":   fmt.Sprintf(`sum(kube_deployment_status_replicas_available{namespace="%s"})`, namespace),
		"unavailable": fmt.Sprintf(`sum(kube_deployment_status_replicas_unavailable{namespace="%s"})`, namespace),
		"updating":    fmt.Sprintf(`count(kube_deployment_status_replicas_updated{namespace="%s"} != bool kube_deployment_spec_replicas{namespace="%s"})`, namespace, namespace),
	}

	results := ns.batchQuery(queries)

	if val, ok := results["total"]; ok && len(val) > 0 {
		stats.Total = int64(val[0].Value)
	}
	if val, ok := results["desired"]; ok && len(val) > 0 {
		stats.DesiredReplicas = int64(val[0].Value)
	}
	if val, ok := results["available"]; ok && len(val) > 0 {
		stats.AvailableReplicas = int64(val[0].Value)
	}
	if val, ok := results["unavailable"]; ok && len(val) > 0 {
		stats.UnavailableReplicas = int64(val[0].Value)
	}
	if val, ok := results["updating"]; ok && len(val) > 0 {
		stats.Updating = int64(val[0].Value)
	}

	return stats, nil
}

// GetNamespaceServices 获取 Namespace Service 统计
func (ns *NamespaceOperatorImpl) GetNamespaceServices(namespace string) (*types.NamespaceServiceStatistics, error) {
	stats := &types.NamespaceServiceStatistics{}

	// 总数和无 Endpoint 的 Service
	queries := map[string]string{
		"total":      fmt.Sprintf(`count(kube_service_info{namespace="%s"})`, namespace),
		"noEndpoint": fmt.Sprintf(`count(kube_service_info{namespace="%s"} unless on(service) kube_endpoint_address_available{namespace="%s"})`, namespace, namespace),
	}

	results := ns.batchQuery(queries)

	if val, ok := results["total"]; ok && len(val) > 0 {
		stats.Total = int64(val[0].Value)
	}
	if val, ok := results["noEndpoint"]; ok && len(val) > 0 {
		stats.WithoutEndpoints = int64(val[0].Value)
	}

	// 按类型统计
	typeQuery := fmt.Sprintf(`count by (type) (kube_service_info{namespace="%s"})`, namespace)
	if typeResults, err := ns.query(typeQuery, nil); err == nil {
		stats.ByType = make([]types.ServiceTypeCount, 0, len(typeResults))
		for _, result := range typeResults {
			if serviceType, ok := result.Metric["type"]; ok {
				stats.ByType = append(stats.ByType, types.ServiceTypeCount{
					Type:  serviceType,
					Count: int64(result.Value),
				})
			}
		}
	}

	return stats, nil
}

func (ns *NamespaceOperatorImpl) GetNamespaceStorage(namespace string) (*types.NamespaceStorageMetrics, error) {
	ns.log.Infof(" 查询 Namespace 存储: namespace=%s", namespace)

	metrics := &types.NamespaceStorageMetrics{}

	// 批量查询 PVC 指标
	queries := map[string]string{
		"total":     fmt.Sprintf(`count(kube_persistentvolumeclaim_info{namespace="%s"})`, namespace),
		"bound":     fmt.Sprintf(`count(kube_persistentvolumeclaim_status_phase{namespace="%s",phase="Bound"} == 1)`, namespace),
		"pending":   fmt.Sprintf(`count(kube_persistentvolumeclaim_status_phase{namespace="%s",phase="Pending"} == 1)`, namespace),
		"lost":      fmt.Sprintf(`count(kube_persistentvolumeclaim_status_phase{namespace="%s",phase="Lost"} == 1)`, namespace),
		"requested": fmt.Sprintf(`sum(kube_persistentvolumeclaim_resource_requests_storage_bytes{namespace="%s"})`, namespace),
	}

	results := ns.batchQuery(queries)

	if val, ok := results["total"]; ok && len(val) > 0 {
		metrics.PVCTotal = int64(val[0].Value)
	}
	if val, ok := results["bound"]; ok && len(val) > 0 {
		metrics.PVCBound = int64(val[0].Value)
	}
	if val, ok := results["pending"]; ok && len(val) > 0 {
		metrics.PVCPending = int64(val[0].Value)
	}
	if val, ok := results["lost"]; ok && len(val) > 0 {
		metrics.PVCLost = int64(val[0].Value)
	}
	if val, ok := results["requested"]; ok && len(val) > 0 {
		metrics.TotalRequestedBytes = int64(val[0].Value)
	}

	// 按 StorageClass 统计
	scQuery := fmt.Sprintf(`sum by (storageclass) (kube_persistentvolumeclaim_resource_requests_storage_bytes{namespace="%s"})`, namespace)
	if scResults, err := ns.query(scQuery, nil); err == nil {
		metrics.ByStorageClass = make([]types.StorageClassUsage, 0, len(scResults))

		// 并发查询每个 StorageClass 的 PVC 数量
		var wg sync.WaitGroup
		var mu sync.Mutex

		for _, result := range scResults {
			if sc, ok := result.Metric["storageclass"]; ok {
				wg.Add(1)
				go func(sc string, totalBytes int64) {
					defer wg.Done()

					pvcCountQuery := fmt.Sprintf(`count(kube_persistentvolumeclaim_info{namespace="%s",storageclass="%s"})`, namespace, sc)
					var pvcCount int64
					if pvcCountResults, err := ns.query(pvcCountQuery, nil); err == nil && len(pvcCountResults) > 0 {
						pvcCount = int64(pvcCountResults[0].Value)
					}

					mu.Lock()
					metrics.ByStorageClass = append(metrics.ByStorageClass, types.StorageClassUsage{
						StorageClass: sc,
						PVCCount:     pvcCount,
						TotalBytes:   totalBytes,
					})
					mu.Unlock()
				}(sc, int64(result.Value))
			}
		}

		wg.Wait()
	}

	return metrics, nil
}

func (ns *NamespaceOperatorImpl) GetNamespaceConfig(namespace string) (*types.NamespaceConfigMetrics, error) {

	metrics := &types.NamespaceConfigMetrics{}

	// 批量查询配置指标
	queries := map[string]string{
		"configMapTotal": fmt.Sprintf(`count(kube_configmap_info{namespace="%s"})`, namespace),
		"secretTotal":    fmt.Sprintf(`count(kube_secret_info{namespace="%s"})`, namespace),
	}

	results := ns.batchQuery(queries)

	if val, ok := results["configMapTotal"]; ok && len(val) > 0 {
		metrics.ConfigMaps.Total = int64(val[0].Value)
	}
	if val, ok := results["secretTotal"]; ok && len(val) > 0 {
		metrics.Secrets.Total = int64(val[0].Value)
	}

	// 按 Secret 类型统计
	secretTypeQuery := fmt.Sprintf(`count by (type) (kube_secret_type{namespace="%s"})`, namespace)
	if secretTypeResults, err := ns.query(secretTypeQuery, nil); err == nil {
		metrics.Secrets.ByType = make([]types.SecretTypeCount, 0, len(secretTypeResults))
		for _, result := range secretTypeResults {
			if secretType, ok := result.Metric["type"]; ok {
				metrics.Secrets.ByType = append(metrics.Secrets.ByType, types.SecretTypeCount{
					Type:  secretType,
					Count: int64(result.Value),
				})
			}
		}
	}

	return metrics, nil
}
func (ns *NamespaceOperatorImpl) GetTopPodsByCPU(namespace string, limit int, timeRange *types.TimeRange) ([]types.ResourceRanking, error) {
	if limit <= 0 {
		limit = 10
	}

	window := ns.calculateRateWindow(timeRange)
	ns.log.Infof("🔍 开始查询 Top Pods by CPU: namespace=%s, limit=%d, window=%s", namespace, limit, window)

	// 策略1: 尝试最简单的查询
	queries := []string{
		fmt.Sprintf(`topk(%d, sum by (pod) (rate(container_cpu_usage_seconds_total{namespace="%s"}[%s])))`, limit, namespace, window),
		fmt.Sprintf(`topk(%d, sum by (pod) (rate(container_cpu_user_seconds_total{namespace="%s"}[%s]) + rate(container_cpu_system_seconds_total{namespace="%s"}[%s])))`, limit, namespace, window, namespace, window),
		fmt.Sprintf(`topk(%d, sum by (pod) (rate(container_cpu_usage_seconds_total{namespace="%s",container!="POD",container!=""}[%s])))`, limit, namespace, window),
	}

	for i, query := range queries {
		ns.log.Infof("尝试查询方案 %d: %s", i+1, query)

		results, err := ns.query(query, nil)
		if err != nil {
			ns.log.Errorf("查询方案 %d 失败: %v", i+1, err)
			continue
		}

		ns.log.Infof("查询方案 %d 返回 %d 个原始结果", i+1, len(results))

		if len(results) > 0 {
			for idx, result := range results {
				ns.log.Infof("  [%d] 原始数据 - Metric: %v, Value: %.6f", idx, result.Metric, result.Value)
			}

			rankings := make([]types.ResourceRanking, 0, len(results))
			for idx, result := range results {
				ns.log.Infof("  [%d] 处理数据 - 开始", idx)

				var podName string
				var found bool

				// 尝试 "pod"
				if pod, ok := result.Metric["pod"]; ok && pod != "" {
					podName = pod
					found = true
					ns.log.Infof("  [%d] 找到 pod label: %s", idx, pod)
				}

				// 尝试 "pod_name"
				if !found {
					if pod, ok := result.Metric["pod_name"]; ok && pod != "" {
						podName = pod
						found = true
						ns.log.Infof("  [%d] 找到 pod_name label: %s", idx, pod)
					}
				}

				// 尝试 "kubernetes_pod_name"
				if !found {
					if pod, ok := result.Metric["kubernetes_pod_name"]; ok && pod != "" {
						podName = pod
						found = true
						ns.log.Infof("  [%d] 找到 kubernetes_pod_name label: %s", idx, pod)
					}
				}

				if !found {
					ns.log.Errorf("  [%d]  未找到任何 pod label！完整 Metric: %v", idx, result.Metric)
					continue
				}

				ranking := types.ResourceRanking{
					Name:  podName,
					Value: result.Value,
					Unit:  "cores",
				}

				rankings = append(rankings, ranking)
				ns.log.Infof("  [%d]添加排名: Pod=%s, Value=%.6f cores", idx, ranking.Name, ranking.Value)
			}

			for idx, r := range rankings {
				ns.log.Infof("  [%d] Name: %s, Value: %.6f, Unit: %s", idx, r.Name, r.Value, r.Unit)
			}

			return rankings, nil
		} else {
			ns.log.Infof("查询方案 %d 返回 0 个结果", i+1)
		}
	}

	ns.log.Errorf("所有查询方案都失败")
	return []types.ResourceRanking{}, nil
}

// diagnoseCPUMetrics 诊断 CPU 指标问题
func (ns *NamespaceOperatorImpl) diagnoseCPUMetrics(namespace string) {
	ns.log.Infof("=== CPU 指标诊断开始 ===")

	// 检查1: 是否有任何 CPU 相关指标
	diagnosticQueries := map[string]string{
		"任何 container_cpu_usage":     fmt.Sprintf(`count(container_cpu_usage_seconds_total{namespace="%s"})`, namespace),
		"任何 container_cpu_user":      fmt.Sprintf(`count(container_cpu_user_seconds_total{namespace="%s"})`, namespace),
		"任何 container_cpu_system":    fmt.Sprintf(`count(container_cpu_system_seconds_total{namespace="%s"})`, namespace),
		"container_cpu_usage (非空容器)": fmt.Sprintf(`count(container_cpu_usage_seconds_total{namespace="%s",container!=""})`, namespace),
		"container_cpu_user (非空容器)":  fmt.Sprintf(`count(container_cpu_user_seconds_total{namespace="%s",container!=""})`, namespace),
		"该 namespace 的 pod 数":        fmt.Sprintf(`count(kube_pod_info{namespace="%s"})`, namespace),
		"该 namespace 的容器数":           fmt.Sprintf(`count(kube_pod_container_info{namespace="%s"})`, namespace),
	}

	for desc, query := range diagnosticQueries {
		if results, err := ns.query(query, nil); err == nil && len(results) > 0 {
			ns.log.Infof("%s: %.0f", desc, results[0].Value)
		} else {
			ns.log.Errorf(" %s: 无数据或查询失败", desc)
		}
	}

	// 检查2: 查看实际存在的 label
	labelQuery := fmt.Sprintf(`container_cpu_usage_seconds_total{namespace="%s"}`, namespace)
	if results, err := ns.query(labelQuery, nil); err == nil && len(results) > 0 {
		ns.log.Infof("找到 CPU 指标样本，labels: %v", results[0].Metric)
	} else {
		// 尝试 user 指标
		labelQuery = fmt.Sprintf(`container_cpu_user_seconds_total{namespace="%s"}`, namespace)
		if results, err := ns.query(labelQuery, nil); err == nil && len(results) > 0 {
			ns.log.Infof("找到 CPU user 指标样本，labels: %v", results[0].Metric)
		} else {
			ns.log.Errorf(" 找不到任何 CPU 指标样本")
		}
	}

	ns.log.Infof("=== CPU 指标诊断结束 ===")
}

func (ns *NamespaceOperatorImpl) GetTopContainersByCPU(namespace string, limit int, timeRange *types.TimeRange) ([]types.ContainerResourceRanking, error) {
	if limit <= 0 {
		limit = 10
	}

	window := ns.calculateRateWindow(timeRange)
	ns.log.Infof("🔍 开始查询 Top Containers by CPU: namespace=%s, limit=%d, window=%s", namespace, limit, window)

	// 策略: 尝试多种查询
	queries := []string{
		// 标准指标 - 最宽松
		fmt.Sprintf(`topk(%d, rate(container_cpu_usage_seconds_total{namespace="%s",container!=""}[%s]))`, limit, namespace, window),
		// user+system - 最宽松
		fmt.Sprintf(`topk(%d, rate(container_cpu_user_seconds_total{namespace="%s",container!=""}[%s]) + rate(container_cpu_system_seconds_total{namespace="%s",container!=""}[%s]))`, limit, namespace, window, namespace, window),
		// 只 user
		fmt.Sprintf(`topk(%d, rate(container_cpu_user_seconds_total{namespace="%s",container!=""}[%s]))`, limit, namespace, window),
		// 标准指标 - 过滤 POD
		fmt.Sprintf(`topk(%d, rate(container_cpu_usage_seconds_total{namespace="%s",container!="POD",container!=""}[%s]))`, limit, namespace, window),
	}

	for i, query := range queries {
		ns.log.Infof("尝试容器查询方案 %d", i+1)

		results, err := ns.query(query, nil)
		if err != nil {
			continue
		}

		if len(results) > 0 {
			rankings := make([]types.ContainerResourceRanking, 0, len(results))
			for _, result := range results {
				if pod, ok := result.Metric["pod"]; ok {
					if container, ok := result.Metric["container"]; ok && container != "" {
						rankings = append(rankings, types.ContainerResourceRanking{
							PodName:       pod,
							ContainerName: container,
							Value:         result.Value,
							Unit:          "cores",
						})
					}
				}
			}

			if len(rankings) > 0 {
				return rankings, nil
			}
		}
	}

	ns.log.Errorf("所有容器查询方案都失败")
	return []types.ContainerResourceRanking{}, nil
}

// GetTopPodsByMemory 获取内存使用 Top N Pods
func (ns *NamespaceOperatorImpl) GetTopPodsByMemory(namespace string, limit int, timeRange *types.TimeRange) ([]types.ResourceRanking, error) {
	query := fmt.Sprintf(`topk(%d, sum by (pod) (container_memory_working_set_bytes{namespace="%s",container!=""}))`, limit, namespace)

	results, err := ns.query(query, nil)
	if err != nil {
		return nil, err
	}

	rankings := make([]types.ResourceRanking, 0, len(results))
	for _, result := range results {
		if pod, ok := result.Metric["pod"]; ok {
			rankings = append(rankings, types.ResourceRanking{
				Name:  pod,
				Value: result.Value,
				Unit:  "bytes",
			})
		}
	}

	return rankings, nil
}

// GetTopPodsByNetwork 获取网络使用 Top N Pods
func (ns *NamespaceOperatorImpl) GetTopPodsByNetwork(namespace string, limit int, timeRange *types.TimeRange) ([]types.ResourceRanking, error) {
	window := ns.calculateRateWindow(timeRange)
	query := fmt.Sprintf(`topk(%d, sum by (pod) (rate(container_network_receive_bytes_total{namespace="%s"}[%s]) + rate(container_network_transmit_bytes_total{namespace="%s"}[%s])))`,
		limit, namespace, window, namespace, window)

	results, err := ns.query(query, nil)
	if err != nil {
		return nil, err
	}

	rankings := make([]types.ResourceRanking, 0, len(results))
	for _, result := range results {
		if pod, ok := result.Metric["pod"]; ok {
			rankings = append(rankings, types.ResourceRanking{
				Name:  pod,
				Value: result.Value,
				Unit:  "bytes/s",
			})
		}
	}

	return rankings, nil
}

// GetTopContainersByMemory 获取内存使用 Top N Containers
func (ns *NamespaceOperatorImpl) GetTopContainersByMemory(namespace string, limit int, timeRange *types.TimeRange) ([]types.ContainerResourceRanking, error) {
	query := fmt.Sprintf(`topk(%d, container_memory_working_set_bytes{namespace="%s",container!=""})`, limit, namespace)

	results, err := ns.query(query, nil)
	if err != nil {
		return nil, err
	}

	rankings := make([]types.ContainerResourceRanking, 0, len(results))
	for _, result := range results {
		if pod, ok := result.Metric["pod"]; ok {
			if container, ok := result.Metric["container"]; ok {
				rankings = append(rankings, types.ContainerResourceRanking{
					PodName:       pod,
					ContainerName: container,
					Value:         result.Value,
					Unit:          "bytes",
				})
			}
		}
	}

	return rankings, nil
}

// CompareNamespaces 对比多个 Namespace
func (ns *NamespaceOperatorImpl) CompareNamespaces(namespaces []string, timeRange *types.TimeRange) (*types.NamespaceComparison, error) {
	ns.log.Infof(" 对比 Namespace: namespaces=%v", namespaces)

	comparison := &types.NamespaceComparison{
		Timestamp:  time.Now(),
		Namespaces: make([]types.NamespaceComparisonItem, len(namespaces)),
	}

	window := ns.calculateRateWindow(timeRange)

	// 并发查询每个 namespace
	var wg sync.WaitGroup
	for i, namespace := range namespaces {
		i := i
		namespace := namespace
		wg.Add(1)
		go func() {
			defer wg.Done()

			item := types.NamespaceComparisonItem{
				Namespace: namespace,
			}

			// 批量查询该 namespace 的所有指标
			queries := map[string]string{
				"cpu": fmt.Sprintf(`sum(rate(container_cpu_usage_seconds_total{namespace="%s",container!=""}[%s]))`, namespace, window),
				"mem": fmt.Sprintf(`sum(container_memory_working_set_bytes{namespace="%s",container!=""})`, namespace),
				"pod": fmt.Sprintf(`count(kube_pod_info{namespace="%s"})`, namespace),
				"rx":  fmt.Sprintf(`sum(rate(container_network_receive_bytes_total{namespace="%s"}[%s]))`, namespace, window),
				"tx":  fmt.Sprintf(`sum(rate(container_network_transmit_bytes_total{namespace="%s"}[%s]))`, namespace, window),
			}

			results := ns.batchQuery(queries)

			if val, ok := results["cpu"]; ok && len(val) > 0 {
				item.CPUUsage = val[0].Value
			}
			if val, ok := results["mem"]; ok && len(val) > 0 {
				item.MemoryUsage = int64(val[0].Value)
			}
			if val, ok := results["pod"]; ok && len(val) > 0 {
				item.PodCount = int64(val[0].Value)
			}
			if val, ok := results["rx"]; ok && len(val) > 0 {
				item.NetworkRxRate = val[0].Value
			}
			if val, ok := results["tx"]; ok && len(val) > 0 {
				item.NetworkTxRate = val[0].Value
			}

			comparison.Namespaces[i] = item
		}()
	}

	wg.Wait()

	return comparison, nil
}

// ==================== 辅助方法 ====================

// batchQuery 批量查询多个指标
func (ns *NamespaceOperatorImpl) batchQuery(queries map[string]string) map[string][]types.InstantQueryResult {
	results := make(map[string][]types.InstantQueryResult)
	var mu sync.Mutex
	var wg sync.WaitGroup

	// 并发执行所有查询
	for name, query := range queries {
		name := name
		query := query
		wg.Add(1)
		go func() {
			defer wg.Done()
			if result, err := ns.query(query, nil); err == nil && len(result) > 0 {
				mu.Lock()
				results[name] = result
				mu.Unlock()
			}
		}()
	}

	wg.Wait()
	return results
}

// query 即时查询
func (ns *NamespaceOperatorImpl) query(query string, timestamp *time.Time) ([]types.InstantQueryResult, error) {
	params := map[string]string{
		"query": query,
	}

	if timestamp != nil {
		params["time"] = ns.formatTimestamp(*timestamp)
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

	if err := ns.doRequest("GET", "/api/v1/query", params, nil, &response); err != nil {
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
func (ns *NamespaceOperatorImpl) queryRange(query string, start, end time.Time, step string) ([]types.RangeQueryResult, error) {
	params := map[string]string{
		"query": query,
		"start": ns.formatTimestamp(start),
		"end":   ns.formatTimestamp(end),
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
		Error string `json:"error,omitempty"`
	}

	if err := ns.doRequest("GET", "/api/v1/query_range", params, nil, &response); err != nil {
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

// parseTimestamp 解析 Prometheus 时间戳
func parseTimestamp(ts float64) time.Time {
	sec := int64(ts)
	nsec := int64((ts - float64(sec)) * 1e9)
	return time.Unix(sec, nsec)
}

// formatTimestamp 格式化时间为 Unix 时间戳
func (ns *NamespaceOperatorImpl) formatTimestamp(t time.Time) string {
	return fmt.Sprintf("%.3f", float64(t.Unix())+float64(t.Nanosecond())/1e9)
}
