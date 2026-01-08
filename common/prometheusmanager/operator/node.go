package operator

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/yanshicheng/kube-nova/common/prometheusmanager/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type NodeOperatorImpl struct {
	log logx.Logger
	ctx context.Context
	*BaseOperator
}

func NewNodeOperator(ctx context.Context, base *BaseOperator) types.NodeOperator {
	return &NodeOperatorImpl{
		log:          logx.WithContext(ctx),
		ctx:          ctx,
		BaseOperator: base,
	}
}

// ==================== 并发查询辅助结构 ====================

type queryTask struct {
	name  string
	query string
	f     func([]types.InstantQueryResult) error
}

type rangeQueryTask struct {
	name  string
	query string
	f     func([]types.RangeQueryResult) error
}

// executeParallelQueries 并发执行即时查询
func (n *NodeOperatorImpl) executeParallelQueries(tasks []queryTask) error {
	var wg sync.WaitGroup
	errChan := make(chan error, len(tasks))

	for _, task := range tasks {
		wg.Add(1)
		go func(t queryTask) {
			defer wg.Done()
			results, err := n.query(t.query, nil)
			if err != nil {
				n.log.Errorf("查询失败 [%s]: %v, query: %s", t.name, err, t.query)
				errChan <- fmt.Errorf("%s: %w", t.name, err)
				return
			}
			if err := t.f(results); err != nil {
				errChan <- fmt.Errorf("%s: %w", t.name, err)
			}
		}(task)
	}

	wg.Wait()
	close(errChan)

	// 收集错误（不阻断，只记录）
	for err := range errChan {
		n.log.Errorf("任务执行失败: %v", err)
	}

	return nil
}

// executeParallelRangeQueries 并发执行范围查询
func (n *NodeOperatorImpl) executeParallelRangeQueries(start, end time.Time, step string, tasks []rangeQueryTask) error {
	var wg sync.WaitGroup
	errChan := make(chan error, len(tasks))

	for _, task := range tasks {
		wg.Add(1)
		go func(t rangeQueryTask) {
			defer wg.Done()
			results, err := n.queryRange(t.query, start, end, step)
			if err != nil {
				n.log.Errorf("范围查询失败 [%s]: %v, query: %s", t.name, err, t.query)
				errChan <- fmt.Errorf("%s: %w", t.name, err)
				return
			}
			if err := t.f(results); err != nil {
				errChan <- fmt.Errorf("%s: %w", t.name, err)
			}
		}(task)
	}

	wg.Wait()
	close(errChan)

	for err := range errChan {
		n.log.Errorf("范围查询任务失败: %v", err)
	}

	return nil
}

func (n *NodeOperatorImpl) GetNodeMetrics(nodeName string, timeRange *types.TimeRange) (*types.NodeMetrics, error) {
	n.log.Infof(" 查询节点综合指标: node=%s", nodeName)

	metrics := &types.NodeMetrics{
		NodeName:  nodeName,
		Timestamp: time.Now(),
	}

	// 并发查询各类指标
	var wg sync.WaitGroup
	wg.Add(6)

	go func() {
		defer wg.Done()
		if cpu, err := n.GetNodeCPU(nodeName, timeRange); err == nil {
			metrics.CPU = *cpu
		} else {
			n.log.Errorf("获取 CPU 指标失败: %v", err)
		}
	}()

	go func() {
		defer wg.Done()
		if mem, err := n.GetNodeMemory(nodeName, timeRange); err == nil {
			metrics.Memory = *mem
		} else {
			n.log.Errorf("获取内存指标失败: %v", err)
		}
	}()

	go func() {
		defer wg.Done()
		if disk, err := n.GetNodeDisk(nodeName, timeRange); err == nil {
			metrics.Disk = *disk
		} else {
			n.log.Errorf("获取磁盘指标失败: %v", err)
		}
	}()

	go func() {
		defer wg.Done()
		if net, err := n.GetNodeNetwork(nodeName, timeRange); err == nil {
			metrics.Network = *net
		} else {
			n.log.Errorf("获取网络指标失败: %v", err)
		}
	}()

	go func() {
		defer wg.Done()
		if k8s, err := n.GetNodeK8sStatus(nodeName, timeRange); err == nil {
			metrics.K8sStatus = *k8s
		} else {
			n.log.Errorf("获取 K8s 状态失败: %v", err)
		}
	}()

	go func() {
		defer wg.Done()
		if sys, err := n.GetNodeSystem(nodeName, timeRange); err == nil {
			metrics.System = *sys
		} else {
			n.log.Errorf("获取系统指标失败: %v", err)
		}
	}()

	wg.Wait()

	return metrics, nil
}

func (n *NodeOperatorImpl) GetNodeCPU(nodeName string, timeRange *types.TimeRange) (*types.NodeCPUMetrics, error) {
	n.log.Infof(" 查询节点 CPU: node=%s", nodeName)

	metrics := &types.NodeCPUMetrics{}
	window := n.calculateRateWindow(timeRange)

	// 分别查询各个指标，避免使用 or 连接
	tasks := []queryTask{
		{
			name:  "cpu_cores",
			query: fmt.Sprintf(`count(node_cpu_seconds_total{instance=~".*%s.*",mode="idle"})`, nodeName),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					metrics.Current.TotalCores = int(results[0].Value)
					metrics.Current.Timestamp = results[0].Time
				}
				return nil
			},
		},
		{
			name:  "cpu_usage",
			query: fmt.Sprintf(`100 - (avg(irate(node_cpu_seconds_total{instance=~".*%s.*",mode="idle"}[%s])) * 100)`, nodeName, window),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					metrics.Current.UsagePercent = results[0].Value
					if metrics.Current.Timestamp.IsZero() {
						metrics.Current.Timestamp = results[0].Time
					}
				}
				return nil
			},
		},
		{
			name:  "cpu_user",
			query: fmt.Sprintf(`avg(irate(node_cpu_seconds_total{instance=~".*%s.*",mode="user"}[%s])) * 100`, nodeName, window),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					metrics.Current.UserPercent = results[0].Value
				}
				return nil
			},
		},
		{
			name:  "cpu_system",
			query: fmt.Sprintf(`avg(irate(node_cpu_seconds_total{instance=~".*%s.*",mode="system"}[%s])) * 100`, nodeName, window),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					metrics.Current.SystemPercent = results[0].Value
				}
				return nil
			},
		},
		{
			name:  "cpu_iowait",
			query: fmt.Sprintf(`avg(irate(node_cpu_seconds_total{instance=~".*%s.*",mode="iowait"}[%s])) * 100`, nodeName, window),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					metrics.Current.IowaitPercent = results[0].Value
				}
				return nil
			},
		},
		{
			name:  "cpu_steal",
			query: fmt.Sprintf(`avg(irate(node_cpu_seconds_total{instance=~".*%s.*",mode="steal"}[%s])) * 100`, nodeName, window),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					metrics.Current.StealPercent = results[0].Value
				}
				return nil
			},
		},
		{
			name:  "cpu_irq",
			query: fmt.Sprintf(`avg(irate(node_cpu_seconds_total{instance=~".*%s.*",mode="irq"}[%s])) * 100`, nodeName, window),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					metrics.Current.IrqPercent = results[0].Value
				}
				return nil
			},
		},
		{
			name:  "cpu_softirq",
			query: fmt.Sprintf(`avg(irate(node_cpu_seconds_total{instance=~".*%s.*",mode="softirq"}[%s])) * 100`, nodeName, window),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					metrics.Current.SoftirqPercent = results[0].Value
				}
				return nil
			},
		},
		{
			name:  "load1",
			query: fmt.Sprintf(`node_load1{instance=~".*%s.*"}`, nodeName),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					metrics.Current.Load1 = results[0].Value
				}
				return nil
			},
		},
		{
			name:  "load5",
			query: fmt.Sprintf(`node_load5{instance=~".*%s.*"}`, nodeName),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					metrics.Current.Load5 = results[0].Value
				}
				return nil
			},
		},
		{
			name:  "load15",
			query: fmt.Sprintf(`node_load15{instance=~".*%s.*"}`, nodeName),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					metrics.Current.Load15 = results[0].Value
				}
				return nil
			},
		},
		{
			name:  "context_switches",
			query: fmt.Sprintf(`irate(node_context_switches_total{instance=~".*%s.*"}[%s])`, nodeName, window),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					metrics.Current.ContextSwitchRate = results[0].Value
				}
				return nil
			},
		},
	}

	if err := n.executeParallelQueries(tasks); err != nil {
		return nil, err
	}

	// 查询趋势数据
	if timeRange != nil && !timeRange.Start.IsZero() && !timeRange.End.IsZero() {
		step := timeRange.Step
		if step == "" {
			step = n.calculateStep(timeRange.Start, timeRange.End)
		}

		var trendMu sync.Mutex
		rangeTasks := []rangeQueryTask{
			{
				name:  "cpu_usage_trend",
				query: fmt.Sprintf(`100 - (avg(irate(node_cpu_seconds_total{instance=~".*%s.*",mode="idle"}[%s])) * 100)`, nodeName, window),
				f: func(results []types.RangeQueryResult) error {
					if len(results) > 0 && len(results[0].Values) > 0 {
						trendMu.Lock()
						defer trendMu.Unlock()
						metrics.Trend = make([]types.NodeCPUDataPoint, len(results[0].Values))
						for i, v := range results[0].Values {
							metrics.Trend[i] = types.NodeCPUDataPoint{
								Timestamp:    v.Timestamp,
								UsagePercent: v.Value,
							}
						}
					}
					return nil
				},
			},
			{
				name:  "load5_trend",
				query: fmt.Sprintf(`node_load5{instance=~".*%s.*"}`, nodeName),
				f: func(results []types.RangeQueryResult) error {
					if len(results) > 0 && len(results[0].Values) > 0 {
						trendMu.Lock()
						defer trendMu.Unlock()
						for i, v := range results[0].Values {
							if i < len(metrics.Trend) {
								metrics.Trend[i].Load5 = v.Value
							}
						}
					}
					return nil
				},
			},
		}

		err := n.executeParallelRangeQueries(timeRange.Start, timeRange.End, step, rangeTasks)
		if err != nil {
			return nil, err
		}
		if len(metrics.Trend) > 0 {
			metrics.Summary = n.calculateCPUSummary(metrics.Trend)
		}
	}

	n.log.Infof(" 节点 CPU 指标查询完成: node=%s, usage=%.2f%%", nodeName, metrics.Current.UsagePercent)
	return metrics, nil
}

func (n *NodeOperatorImpl) GetNodeMemory(nodeName string, timeRange *types.TimeRange) (*types.NodeMemoryMetrics, error) {
	n.log.Infof(" 查询节点内存: node=%s", nodeName)

	metrics := &types.NodeMemoryMetrics{}
	window := n.calculateRateWindow(timeRange)

	// 分别查询各个指标
	tasks := []queryTask{
		{
			name:  "memory_total",
			query: fmt.Sprintf(`node_memory_MemTotal_bytes{instance=~".*%s.*"}`, nodeName),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					metrics.Current.TotalBytes = int64(results[0].Value)
					metrics.Current.Timestamp = results[0].Time
				}
				return nil
			},
		},
		{
			name:  "memory_available",
			query: fmt.Sprintf(`node_memory_MemAvailable_bytes{instance=~".*%s.*"}`, nodeName),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					metrics.Current.AvailableBytes = int64(results[0].Value)
				}
				return nil
			},
		},
		{
			name:  "memory_free",
			query: fmt.Sprintf(`node_memory_MemFree_bytes{instance=~".*%s.*"}`, nodeName),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					metrics.Current.FreeBytes = int64(results[0].Value)
				}
				return nil
			},
		},
		{
			name:  "memory_buffers",
			query: fmt.Sprintf(`node_memory_Buffers_bytes{instance=~".*%s.*"}`, nodeName),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					metrics.Current.BuffersBytes = int64(results[0].Value)
				}
				return nil
			},
		},
		{
			name:  "memory_cached",
			query: fmt.Sprintf(`node_memory_Cached_bytes{instance=~".*%s.*"}`, nodeName),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					metrics.Current.CachedBytes = int64(results[0].Value)
				}
				return nil
			},
		},
		{
			name:  "memory_active",
			query: fmt.Sprintf(`node_memory_Active_bytes{instance=~".*%s.*"}`, nodeName),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					metrics.Current.ActiveBytes = int64(results[0].Value)
				}
				return nil
			},
		},
		{
			name:  "memory_inactive",
			query: fmt.Sprintf(`node_memory_Inactive_bytes{instance=~".*%s.*"}`, nodeName),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					metrics.Current.InactiveBytes = int64(results[0].Value)
				}
				return nil
			},
		},
		{
			name:  "swap_total",
			query: fmt.Sprintf(`node_memory_SwapTotal_bytes{instance=~".*%s.*"}`, nodeName),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					metrics.Current.SwapTotalBytes = int64(results[0].Value)
				}
				return nil
			},
		},
		{
			name:  "swap_free",
			query: fmt.Sprintf(`node_memory_SwapFree_bytes{instance=~".*%s.*"}`, nodeName),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					metrics.Current.SwapFreeBytes = int64(results[0].Value)
				}
				return nil
			},
		},
		{
			name:  "swap_in",
			query: fmt.Sprintf(`irate(node_vmstat_pswpin{instance=~".*%s.*"}[%s])`, nodeName, window),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					metrics.Current.SwapInRate = results[0].Value
				}
				return nil
			},
		},
		{
			name:  "swap_out",
			query: fmt.Sprintf(`irate(node_vmstat_pswpout{instance=~".*%s.*"}[%s])`, nodeName, window),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					metrics.Current.SwapOutRate = results[0].Value
				}
				return nil
			},
		},
	}

	err := n.executeParallelQueries(tasks)
	if err != nil {
		return nil, err
	}

	// 计算使用率
	if metrics.Current.TotalBytes > 0 && metrics.Current.AvailableBytes > 0 {
		metrics.Current.UsedBytes = metrics.Current.TotalBytes - metrics.Current.AvailableBytes
		metrics.Current.UsagePercent = (float64(metrics.Current.UsedBytes) / float64(metrics.Current.TotalBytes)) * 100
	}

	if metrics.Current.SwapTotalBytes > 0 {
		metrics.Current.SwapUsedBytes = metrics.Current.SwapTotalBytes - metrics.Current.SwapFreeBytes
		metrics.Current.SwapUsagePercent = (float64(metrics.Current.SwapUsedBytes) / float64(metrics.Current.SwapTotalBytes)) * 100
	}

	// 趋势数据
	if timeRange != nil && !timeRange.Start.IsZero() && !timeRange.End.IsZero() {
		step := timeRange.Step
		if step == "" {
			step = n.calculateStep(timeRange.Start, timeRange.End)
		}

		usedQuery := fmt.Sprintf(`node_memory_MemTotal_bytes{instance=~".*%s.*"} - node_memory_MemAvailable_bytes{instance=~".*%s.*"}`, nodeName, nodeName)
		if usedTrend, err := n.queryRange(usedQuery, timeRange.Start, timeRange.End, step); err == nil && len(usedTrend) > 0 && len(usedTrend[0].Values) > 0 {
			metrics.Trend = make([]types.NodeMemoryDataPoint, len(usedTrend[0].Values))
			for i, v := range usedTrend[0].Values {
				usedBytes := int64(v.Value)
				usagePercent := 0.0
				if metrics.Current.TotalBytes > 0 {
					usagePercent = (float64(usedBytes) / float64(metrics.Current.TotalBytes)) * 100
				}
				metrics.Trend[i] = types.NodeMemoryDataPoint{
					Timestamp:      v.Timestamp,
					UsedBytes:      usedBytes,
					UsagePercent:   usagePercent,
					AvailableBytes: metrics.Current.TotalBytes - usedBytes,
				}
			}
			metrics.Summary = n.calculateMemorySummary(metrics.Trend)
		}
	}

	n.log.Infof(" 节点内存指标查询完成: node=%s, usage=%.2f%%", nodeName, metrics.Current.UsagePercent)
	return metrics, nil
}

func (n *NodeOperatorImpl) GetNodeDisk(nodeName string, timeRange *types.TimeRange) (*types.NodeDiskMetrics, error) {
	n.log.Infof(" 查询节点磁盘: node=%s", nodeName)

	metrics := &types.NodeDiskMetrics{}
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		if fs, err := n.GetNodeFilesystems(nodeName, timeRange); err == nil {
			metrics.Filesystems = fs
		} else {
			n.log.Errorf("获取文件系统失败: %v", err)
		}
	}()

	go func() {
		defer wg.Done()
		if dev, err := n.GetNodeDiskDevices(nodeName, timeRange); err == nil {
			metrics.Devices = dev
		} else {
			n.log.Errorf("获取磁盘设备失败: %v", err)
		}
	}()

	wg.Wait()

	n.log.Infof(" 节点磁盘指标查询完成: node=%s, filesystems=%d, devices=%d",
		nodeName, len(metrics.Filesystems), len(metrics.Devices))
	return metrics, nil
}

func (n *NodeOperatorImpl) GetNodeFilesystems(nodeName string, timeRange *types.TimeRange) ([]types.NodeFilesystemMetrics, error) {
	n.log.Infof(" 查询节点文件系统: node=%s", nodeName)

	// 分别查询各个指标
	var (
		sizeResults  []types.InstantQueryResult
		availResults []types.InstantQueryResult
		filesResults []types.InstantQueryResult
		freeResults  []types.InstantQueryResult
		mu           sync.Mutex
	)

	tasks := []queryTask{
		{
			name:  "fs_size",
			query: fmt.Sprintf(`node_filesystem_size_bytes{instance=~".*%s.*",fstype!~"tmpfs|devtmpfs|overlay"}`, nodeName),
			f: func(results []types.InstantQueryResult) error {
				mu.Lock()
				sizeResults = results
				mu.Unlock()
				return nil
			},
		},
		{
			name:  "fs_avail",
			query: fmt.Sprintf(`node_filesystem_avail_bytes{instance=~".*%s.*",fstype!~"tmpfs|devtmpfs|overlay"}`, nodeName),
			f: func(results []types.InstantQueryResult) error {
				mu.Lock()
				availResults = results
				mu.Unlock()
				return nil
			},
		},
		{
			name:  "fs_files",
			query: fmt.Sprintf(`node_filesystem_files{instance=~".*%s.*",fstype!~"tmpfs|devtmpfs|overlay"}`, nodeName),
			f: func(results []types.InstantQueryResult) error {
				mu.Lock()
				filesResults = results
				mu.Unlock()
				return nil
			},
		},
		{
			name:  "fs_files_free",
			query: fmt.Sprintf(`node_filesystem_files_free{instance=~".*%s.*",fstype!~"tmpfs|devtmpfs|overlay"}`, nodeName),
			f: func(results []types.InstantQueryResult) error {
				mu.Lock()
				freeResults = results
				mu.Unlock()
				return nil
			},
		},
	}

	err := n.executeParallelQueries(tasks)
	if err != nil {
		return nil, err
	}

	// 按挂载点分组
	fsMap := make(map[string]*types.NodeFilesystemMetrics)

	// 处理 size
	for _, r := range sizeResults {
		mountpoint := r.Metric["mountpoint"]
		if mountpoint == "" {
			continue
		}
		if _, exists := fsMap[mountpoint]; !exists {
			fsMap[mountpoint] = &types.NodeFilesystemMetrics{
				Mountpoint: mountpoint,
				Device:     r.Metric["device"],
				FSType:     r.Metric["fstype"],
			}
		}
		fsMap[mountpoint].Current.TotalBytes = int64(r.Value)
		fsMap[mountpoint].Current.Timestamp = r.Time
	}

	// 处理 avail
	for _, r := range availResults {
		mountpoint := r.Metric["mountpoint"]
		if fs, exists := fsMap[mountpoint]; exists {
			fs.Current.AvailableBytes = int64(r.Value)
		}
	}

	// 处理 files
	for _, r := range filesResults {
		mountpoint := r.Metric["mountpoint"]
		if fs, exists := fsMap[mountpoint]; exists {
			fs.Current.TotalInodes = int64(r.Value)
		}
	}

	// 处理 files_free
	for _, r := range freeResults {
		mountpoint := r.Metric["mountpoint"]
		if fs, exists := fsMap[mountpoint]; exists {
			fs.Current.FreeInodes = int64(r.Value)
		}
	}

	// 计算使用率并查询趋势
	filesystems := make([]types.NodeFilesystemMetrics, 0, len(fsMap))
	for _, fs := range fsMap {
		if fs.Current.TotalBytes > 0 {
			fs.Current.UsedBytes = fs.Current.TotalBytes - fs.Current.AvailableBytes
			fs.Current.UsagePercent = (float64(fs.Current.UsedBytes) / float64(fs.Current.TotalBytes)) * 100
		}
		if fs.Current.TotalInodes > 0 {
			fs.Current.UsedInodes = fs.Current.TotalInodes - fs.Current.FreeInodes
			fs.Current.InodePercent = (float64(fs.Current.UsedInodes) / float64(fs.Current.TotalInodes)) * 100
		}

		// 趋势数据（可选）
		if timeRange != nil && !timeRange.Start.IsZero() && !timeRange.End.IsZero() {
			step := timeRange.Step
			if step == "" {
				step = n.calculateStep(timeRange.Start, timeRange.End)
			}

			usedQuery := fmt.Sprintf(
				`node_filesystem_size_bytes{instance=~".*%s.*",mountpoint="%s"} - node_filesystem_avail_bytes{instance=~".*%s.*",mountpoint="%s"}`,
				nodeName, fs.Mountpoint, nodeName, fs.Mountpoint)

			if usedTrend, err := n.queryRange(usedQuery, timeRange.Start, timeRange.End, step); err == nil && len(usedTrend) > 0 && len(usedTrend[0].Values) > 0 {
				fs.Trend = make([]types.NodeFilesystemDataPoint, len(usedTrend[0].Values))
				for i, v := range usedTrend[0].Values {
					usedBytes := int64(v.Value)
					usagePercent := 0.0
					if fs.Current.TotalBytes > 0 {
						usagePercent = (float64(usedBytes) / float64(fs.Current.TotalBytes)) * 100
					}
					fs.Trend[i] = types.NodeFilesystemDataPoint{
						Timestamp:    v.Timestamp,
						UsedBytes:    usedBytes,
						UsagePercent: usagePercent,
					}
				}
			}
		}

		filesystems = append(filesystems, *fs)
	}

	n.log.Infof(" 节点文件系统查询完成: node=%s, count=%d", nodeName, len(filesystems))
	return filesystems, nil
}

func (n *NodeOperatorImpl) GetNodeDiskDevices(nodeName string, timeRange *types.TimeRange) ([]types.NodeDiskDeviceMetrics, error) {
	n.log.Infof(" 查询节点磁盘设备: node=%s", nodeName)

	window := n.calculateRateWindow(timeRange)

	var (
		readBytesResults  []types.InstantQueryResult
		writeBytesResults []types.InstantQueryResult
		readIOPSResults   []types.InstantQueryResult
		writeIOPSResults  []types.InstantQueryResult
		ioTimeResults     []types.InstantQueryResult
		mu                sync.Mutex
	)

	// 分别查询各个指标
	tasks := []queryTask{
		{
			name:  "disk_read_bytes",
			query: fmt.Sprintf(`irate(node_disk_read_bytes_total{instance=~".*%s.*"}[%s])`, nodeName, window),
			f: func(results []types.InstantQueryResult) error {
				mu.Lock()
				readBytesResults = results
				mu.Unlock()
				return nil
			},
		},
		{
			name:  "disk_write_bytes",
			query: fmt.Sprintf(`irate(node_disk_written_bytes_total{instance=~".*%s.*"}[%s])`, nodeName, window),
			f: func(results []types.InstantQueryResult) error {
				mu.Lock()
				writeBytesResults = results
				mu.Unlock()
				return nil
			},
		},
		{
			name:  "disk_read_iops",
			query: fmt.Sprintf(`irate(node_disk_reads_completed_total{instance=~".*%s.*"}[%s])`, nodeName, window),
			f: func(results []types.InstantQueryResult) error {
				mu.Lock()
				readIOPSResults = results
				mu.Unlock()
				return nil
			},
		},
		{
			name:  "disk_write_iops",
			query: fmt.Sprintf(`irate(node_disk_writes_completed_total{instance=~".*%s.*"}[%s])`, nodeName, window),
			f: func(results []types.InstantQueryResult) error {
				mu.Lock()
				writeIOPSResults = results
				mu.Unlock()
				return nil
			},
		},
		{
			name:  "disk_io_time",
			query: fmt.Sprintf(`irate(node_disk_io_time_seconds_total{instance=~".*%s.*"}[%s]) * 100`, nodeName, window),
			f: func(results []types.InstantQueryResult) error {
				mu.Lock()
				ioTimeResults = results
				mu.Unlock()
				return nil
			},
		},
	}

	n.executeParallelQueries(tasks)

	deviceMap := make(map[string]*types.NodeDiskDeviceMetrics)

	// 处理读字节
	for _, r := range readBytesResults {
		device := r.Metric["device"]
		if device == "" {
			continue
		}
		if _, exists := deviceMap[device]; !exists {
			deviceMap[device] = &types.NodeDiskDeviceMetrics{Device: device}
		}
		deviceMap[device].Current.ReadBytesPerSec = r.Value
		deviceMap[device].Current.Timestamp = r.Time
	}

	// 处理写字节
	for _, r := range writeBytesResults {
		device := r.Metric["device"]
		if dev, exists := deviceMap[device]; exists {
			dev.Current.WriteBytesPerSec = r.Value
		}
	}

	// 处理读IOPS
	for _, r := range readIOPSResults {
		device := r.Metric["device"]
		if dev, exists := deviceMap[device]; exists {
			dev.Current.ReadIOPS = r.Value
		}
	}

	// 处理写IOPS
	for _, r := range writeIOPSResults {
		device := r.Metric["device"]
		if dev, exists := deviceMap[device]; exists {
			dev.Current.WriteIOPS = r.Value
		}
	}

	// 处理IO时间
	for _, r := range ioTimeResults {
		device := r.Metric["device"]
		if dev, exists := deviceMap[device]; exists {
			dev.Current.IOUtilizationPercent = r.Value
		}
	}

	// 查询平均等待时间
	for device, dev := range deviceMap {
		avgAwaitQuery := fmt.Sprintf(
			`(irate(node_disk_read_time_seconds_total{instance=~".*%s.*",device="%s"}[%s]) + irate(node_disk_write_time_seconds_total{instance=~".*%s.*",device="%s"}[%s])) / (irate(node_disk_reads_completed_total{instance=~".*%s.*",device="%s"}[%s]) + irate(node_disk_writes_completed_total{instance=~".*%s.*",device="%s"}[%s]) + 0.001) * 1000`,
			nodeName, device, window, nodeName, device, window, nodeName, device, window, nodeName, device, window)

		if avgAwaitResults, err := n.query(avgAwaitQuery, nil); err == nil && len(avgAwaitResults) > 0 {
			dev.Current.AvgAwaitMs = avgAwaitResults[0].Value
		}
	}

	// 趋势数据
	if timeRange != nil && !timeRange.Start.IsZero() && !timeRange.End.IsZero() {
		step := timeRange.Step
		if step == "" {
			step = n.calculateStep(timeRange.Start, timeRange.End)
		}

		var wg sync.WaitGroup
		for device, dev := range deviceMap {
			wg.Add(1)
			go func(device string, dev *types.NodeDiskDeviceMetrics) {
				defer wg.Done()

				readQuery := fmt.Sprintf(`irate(node_disk_read_bytes_total{instance=~".*%s.*",device="%s"}[%s])`, nodeName, device, window)
				writeQuery := fmt.Sprintf(`irate(node_disk_written_bytes_total{instance=~".*%s.*",device="%s"}[%s])`, nodeName, device, window)
				ioUtilQuery := fmt.Sprintf(`irate(node_disk_io_time_seconds_total{instance=~".*%s.*",device="%s"}[%s]) * 100`, nodeName, device, window)

				var innerWg sync.WaitGroup
				var innerMu sync.Mutex
				innerWg.Add(3)

				go func() {
					defer innerWg.Done()
					if readTrend, err := n.queryRange(readQuery, timeRange.Start, timeRange.End, step); err == nil && len(readTrend) > 0 && len(readTrend[0].Values) > 0 {
						innerMu.Lock()
						defer innerMu.Unlock()
						dev.Trend = make([]types.NodeDiskDeviceDataPoint, len(readTrend[0].Values))
						for i, v := range readTrend[0].Values {
							dev.Trend[i] = types.NodeDiskDeviceDataPoint{
								Timestamp:       v.Timestamp,
								ReadBytesPerSec: v.Value,
							}
						}
					}
				}()

				go func() {
					defer innerWg.Done()
					if writeTrend, err := n.queryRange(writeQuery, timeRange.Start, timeRange.End, step); err == nil && len(writeTrend) > 0 && len(writeTrend[0].Values) > 0 {
						innerMu.Lock()
						defer innerMu.Unlock()
						for i, v := range writeTrend[0].Values {
							if i < len(dev.Trend) {
								dev.Trend[i].WriteBytesPerSec = v.Value
							}
						}
					}
				}()

				go func() {
					defer innerWg.Done()
					if ioUtilTrend, err := n.queryRange(ioUtilQuery, timeRange.Start, timeRange.End, step); err == nil && len(ioUtilTrend) > 0 && len(ioUtilTrend[0].Values) > 0 {
						innerMu.Lock()
						defer innerMu.Unlock()
						for i, v := range ioUtilTrend[0].Values {
							if i < len(dev.Trend) {
								dev.Trend[i].IOUtilizationPercent = v.Value
							}
						}
					}
				}()

				innerWg.Wait()
				if len(dev.Trend) > 0 {
					dev.Summary = n.calculateDiskDeviceSummary(dev.Trend)
				}
			}(device, dev)
		}
		wg.Wait()
	}

	devices := make([]types.NodeDiskDeviceMetrics, 0, len(deviceMap))
	for _, dev := range deviceMap {
		devices = append(devices, *dev)
	}

	n.log.Infof(" 节点磁盘设备查询完成: node=%s, count=%d", nodeName, len(devices))
	return devices, nil
}

func (n *NodeOperatorImpl) GetNodeNetwork(nodeName string, timeRange *types.TimeRange) (*types.NodeNetworkMetrics, error) {
	n.log.Infof(" 查询节点网络: node=%s", nodeName)

	metrics := &types.NodeNetworkMetrics{}
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		if ifaces, err := n.GetNodeNetworkInterfaces(nodeName, timeRange); err == nil {
			metrics.Interfaces = ifaces
		} else {
			n.log.Errorf("获取网络接口失败: %v", err)
		}
	}()

	go func() {
		defer wg.Done()
		if tcp, err := n.GetNodeTCP(nodeName, timeRange); err == nil {
			metrics.TCP = *tcp
		} else {
			n.log.Errorf("获取 TCP 指标失败: %v", err)
		}
	}()

	wg.Wait()

	n.log.Infof(" 节点网络指标查询完成: node=%s, interfaces=%d", nodeName, len(metrics.Interfaces))
	return metrics, nil
}

func (n *NodeOperatorImpl) GetNodeNetworkInterfaces(nodeName string, timeRange *types.TimeRange) ([]types.NodeNetworkInterfaceMetrics, error) {
	n.log.Infof(" 查询节点网络接口: node=%s", nodeName)

	window := n.calculateRateWindow(timeRange)

	var (
		rxBytesResults   []types.InstantQueryResult
		txBytesResults   []types.InstantQueryResult
		rxPacketsResults []types.InstantQueryResult
		txPacketsResults []types.InstantQueryResult
		rxErrsResults    []types.InstantQueryResult
		txErrsResults    []types.InstantQueryResult
		rxDropResults    []types.InstantQueryResult
		txDropResults    []types.InstantQueryResult
		mu               sync.Mutex
	)

	// 分别查询各个指标
	tasks := []queryTask{
		{
			name:  "net_rx_bytes",
			query: fmt.Sprintf(`irate(node_network_receive_bytes_total{instance=~".*%s.*",device!="lo"}[%s])`, nodeName, window),
			f: func(results []types.InstantQueryResult) error {
				mu.Lock()
				rxBytesResults = results
				mu.Unlock()
				return nil
			},
		},
		{
			name:  "net_tx_bytes",
			query: fmt.Sprintf(`irate(node_network_transmit_bytes_total{instance=~".*%s.*",device!="lo"}[%s])`, nodeName, window),
			f: func(results []types.InstantQueryResult) error {
				mu.Lock()
				txBytesResults = results
				mu.Unlock()
				return nil
			},
		},
		{
			name:  "net_rx_packets",
			query: fmt.Sprintf(`irate(node_network_receive_packets_total{instance=~".*%s.*",device!="lo"}[%s])`, nodeName, window),
			f: func(results []types.InstantQueryResult) error {
				mu.Lock()
				rxPacketsResults = results
				mu.Unlock()
				return nil
			},
		},
		{
			name:  "net_tx_packets",
			query: fmt.Sprintf(`irate(node_network_transmit_packets_total{instance=~".*%s.*",device!="lo"}[%s])`, nodeName, window),
			f: func(results []types.InstantQueryResult) error {
				mu.Lock()
				txPacketsResults = results
				mu.Unlock()
				return nil
			},
		},
		{
			name:  "net_rx_errs",
			query: fmt.Sprintf(`irate(node_network_receive_errs_total{instance=~".*%s.*",device!="lo"}[%s])`, nodeName, window),
			f: func(results []types.InstantQueryResult) error {
				mu.Lock()
				rxErrsResults = results
				mu.Unlock()
				return nil
			},
		},
		{
			name:  "net_tx_errs",
			query: fmt.Sprintf(`irate(node_network_transmit_errs_total{instance=~".*%s.*",device!="lo"}[%s])`, nodeName, window),
			f: func(results []types.InstantQueryResult) error {
				mu.Lock()
				txErrsResults = results
				mu.Unlock()
				return nil
			},
		},
		{
			name:  "net_rx_drop",
			query: fmt.Sprintf(`irate(node_network_receive_drop_total{instance=~".*%s.*",device!="lo"}[%s])`, nodeName, window),
			f: func(results []types.InstantQueryResult) error {
				mu.Lock()
				rxDropResults = results
				mu.Unlock()
				return nil
			},
		},
		{
			name:  "net_tx_drop",
			query: fmt.Sprintf(`irate(node_network_transmit_drop_total{instance=~".*%s.*",device!="lo"}[%s])`, nodeName, window),
			f: func(results []types.InstantQueryResult) error {
				mu.Lock()
				txDropResults = results
				mu.Unlock()
				return nil
			},
		},
	}

	n.executeParallelQueries(tasks)

	ifaceMap := make(map[string]*types.NodeNetworkInterfaceMetrics)

	// 处理接收字节
	for _, r := range rxBytesResults {
		device := r.Metric["device"]
		if device == "" {
			continue
		}
		if _, exists := ifaceMap[device]; !exists {
			ifaceMap[device] = &types.NodeNetworkInterfaceMetrics{
				InterfaceName: device,
			}
		}
		ifaceMap[device].Current.ReceiveBytesPerSec = r.Value
		ifaceMap[device].Current.Timestamp = r.Time
	}

	// 处理发送字节
	for _, r := range txBytesResults {
		device := r.Metric["device"]
		if iface, exists := ifaceMap[device]; exists {
			iface.Current.TransmitBytesPerSec = r.Value
		}
	}

	// 处理接收包
	for _, r := range rxPacketsResults {
		device := r.Metric["device"]
		if iface, exists := ifaceMap[device]; exists {
			iface.Current.ReceivePacketsPerSec = r.Value
		}
	}

	// 处理发送包
	for _, r := range txPacketsResults {
		device := r.Metric["device"]
		if iface, exists := ifaceMap[device]; exists {
			iface.Current.TransmitPacketsPerSec = r.Value
		}
	}

	// 处理接收错误
	for _, r := range rxErrsResults {
		device := r.Metric["device"]
		if iface, exists := ifaceMap[device]; exists {
			iface.Current.ReceiveErrorsRate = r.Value
		}
	}

	// 处理发送错误
	for _, r := range txErrsResults {
		device := r.Metric["device"]
		if iface, exists := ifaceMap[device]; exists {
			iface.Current.TransmitErrorsRate = r.Value
		}
	}

	// 处理接收丢包
	for _, r := range rxDropResults {
		device := r.Metric["device"]
		if iface, exists := ifaceMap[device]; exists {
			iface.Current.ReceiveDropsRate = r.Value
		}
	}

	// 处理发送丢包
	for _, r := range txDropResults {
		device := r.Metric["device"]
		if iface, exists := ifaceMap[device]; exists {
			iface.Current.TransmitDropsRate = r.Value
		}
	}

	// 趋势数据
	if timeRange != nil && !timeRange.Start.IsZero() && !timeRange.End.IsZero() {
		step := timeRange.Step
		if step == "" {
			step = n.calculateStep(timeRange.Start, timeRange.End)
		}

		var wg sync.WaitGroup
		for device, iface := range ifaceMap {
			wg.Add(1)
			go func(device string, iface *types.NodeNetworkInterfaceMetrics) {
				defer wg.Done()

				rxQuery := fmt.Sprintf(`irate(node_network_receive_bytes_total{instance=~".*%s.*",device="%s"}[%s])`, nodeName, device, window)
				txQuery := fmt.Sprintf(`irate(node_network_transmit_bytes_total{instance=~".*%s.*",device="%s"}[%s])`, nodeName, device, window)

				var innerWg sync.WaitGroup
				var innerMu sync.Mutex
				innerWg.Add(2)

				go func() {
					defer innerWg.Done()
					if rxTrend, err := n.queryRange(rxQuery, timeRange.Start, timeRange.End, step); err == nil && len(rxTrend) > 0 && len(rxTrend[0].Values) > 0 {
						innerMu.Lock()
						defer innerMu.Unlock()
						iface.Trend = make([]types.NodeNetworkInterfaceDataPoint, len(rxTrend[0].Values))
						for i, v := range rxTrend[0].Values {
							iface.Trend[i] = types.NodeNetworkInterfaceDataPoint{
								Timestamp:          v.Timestamp,
								ReceiveBytesPerSec: v.Value,
							}
						}
					}
				}()

				go func() {
					defer innerWg.Done()
					if txTrend, err := n.queryRange(txQuery, timeRange.Start, timeRange.End, step); err == nil && len(txTrend) > 0 && len(txTrend[0].Values) > 0 {
						innerMu.Lock()
						defer innerMu.Unlock()
						for i, v := range txTrend[0].Values {
							if i < len(iface.Trend) {
								iface.Trend[i].TransmitBytesPerSec = v.Value
							}
						}
					}
				}()

				innerWg.Wait()
				if len(iface.Trend) > 0 {
					iface.Summary = n.calculateNetworkInterfaceSummary(iface.Trend, device)
				}
			}(device, iface)
		}
		wg.Wait()
	}

	interfaces := make([]types.NodeNetworkInterfaceMetrics, 0, len(ifaceMap))
	for _, iface := range ifaceMap {
		interfaces = append(interfaces, *iface)
	}

	n.log.Infof(" 节点网络接口查询完成: node=%s, count=%d", nodeName, len(interfaces))
	return interfaces, nil
}

func (n *NodeOperatorImpl) GetNodeTCP(nodeName string, timeRange *types.TimeRange) (*types.NodeTCPMetrics, error) {
	n.log.Infof(" 查询节点 TCP: node=%s", nodeName)

	metrics := &types.NodeTCPMetrics{}
	window := n.calculateRateWindow(timeRange)

	tasks := []queryTask{
		{
			name:  "tcp_established",
			query: fmt.Sprintf(`node_netstat_Tcp_CurrEstab{instance=~".*%s.*"}`, nodeName),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					metrics.Current.EstablishedConnections = int64(results[0].Value)
					metrics.Current.Timestamp = results[0].Time
				}
				return nil
			},
		},
		{
			name:  "tcp_time_wait",
			query: fmt.Sprintf(`node_sockstat_TCP_tw{instance=~".*%s.*"}`, nodeName),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					metrics.Current.TimeWaitConnections = int64(results[0].Value)
				}
				return nil
			},
		},
		{
			name:  "tcp_active_opens",
			query: fmt.Sprintf(`irate(node_netstat_Tcp_ActiveOpens{instance=~".*%s.*"}[%s])`, nodeName, window),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					metrics.Current.ActiveOpensRate = results[0].Value
				}
				return nil
			},
		},
		{
			name:  "tcp_passive_opens",
			query: fmt.Sprintf(`irate(node_netstat_Tcp_PassiveOpens{instance=~".*%s.*"}[%s])`, nodeName, window),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					metrics.Current.PassiveOpensRate = results[0].Value
				}
				return nil
			},
		},
		{
			name:  "tcp_sockets_used",
			query: fmt.Sprintf(`node_sockstat_sockets_used{instance=~".*%s.*"}`, nodeName),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					metrics.Current.SocketsInUse = int64(results[0].Value)
				}
				return nil
			},
		},
	}

	n.executeParallelQueries(tasks)

	// 趋势数据
	if timeRange != nil && !timeRange.Start.IsZero() && !timeRange.End.IsZero() {
		step := timeRange.Step
		if step == "" {
			step = n.calculateStep(timeRange.Start, timeRange.End)
		}

		establishedQuery := fmt.Sprintf(`node_netstat_Tcp_CurrEstab{instance=~".*%s.*"}`, nodeName)
		timeWaitQuery := fmt.Sprintf(`node_sockstat_TCP_tw{instance=~".*%s.*"}`, nodeName)

		var wg sync.WaitGroup
		var mu sync.Mutex
		wg.Add(2)

		go func() {
			defer wg.Done()
			if establishedTrend, err := n.queryRange(establishedQuery, timeRange.Start, timeRange.End, step); err == nil && len(establishedTrend) > 0 && len(establishedTrend[0].Values) > 0 {
				mu.Lock()
				defer mu.Unlock()
				metrics.Trend = make([]types.NodeTCPDataPoint, len(establishedTrend[0].Values))
				for i, v := range establishedTrend[0].Values {
					metrics.Trend[i] = types.NodeTCPDataPoint{
						Timestamp:              v.Timestamp,
						EstablishedConnections: int64(v.Value),
					}
				}
			}
		}()

		go func() {
			defer wg.Done()
			if timeWaitTrend, err := n.queryRange(timeWaitQuery, timeRange.Start, timeRange.End, step); err == nil && len(timeWaitTrend) > 0 && len(timeWaitTrend[0].Values) > 0 {
				mu.Lock()
				defer mu.Unlock()
				for i, v := range timeWaitTrend[0].Values {
					if i < len(metrics.Trend) {
						metrics.Trend[i].TimeWaitConnections = int64(v.Value)
					}
				}
			}
		}()

		wg.Wait()
	}

	n.log.Infof(" 节点 TCP 指标查询完成: node=%s, established=%d", nodeName, metrics.Current.EstablishedConnections)
	return metrics, nil
}

func (n *NodeOperatorImpl) GetNodeK8sStatus(nodeName string, timeRange *types.TimeRange) (*types.NodeK8sStatus, error) {
	n.log.Infof(" 查询节点 K8s 状态: node=%s", nodeName)

	status := &types.NodeK8sStatus{}
	var wg sync.WaitGroup
	wg.Add(6)

	go func() {
		defer wg.Done()
		if cond, err := n.GetNodeConditions(nodeName); err == nil {
			status.Conditions = cond
		} else {
			n.log.Errorf("获取节点条件失败: %v", err)
		}
	}()

	go func() {
		defer wg.Done()
		if cap, err := n.GetNodeCapacity(nodeName); err == nil {
			status.Capacity = *cap
		} else {
			n.log.Errorf("获取节点容量失败: %v", err)
		}
	}()

	go func() {
		defer wg.Done()
		if alloc, err := n.GetNodeAllocatable(nodeName); err == nil {
			status.Allocatable = *alloc
		} else {
			n.log.Errorf("获取节点可分配资源失败: %v", err)
		}
	}()

	go func() {
		defer wg.Done()
		if allocated, err := n.GetNodeAllocated(nodeName); err == nil {
			status.Allocated = *allocated
		} else {
			n.log.Errorf("获取节点已分配资源失败: %v", err)
		}
	}()

	go func() {
		defer wg.Done()
		if kubelet, err := n.GetNodeKubelet(nodeName, timeRange); err == nil {
			status.KubeletMetrics = *kubelet
		} else {
			n.log.Errorf("获取 kubelet 指标失败: %v", err)
		}
	}()

	go func() {
		defer wg.Done()
		status.NodeInfo = n.getNodeInfo(nodeName)
		status.Labels = n.getNodeLabels(nodeName)
	}()

	wg.Wait()

	status.Taints = []types.NodeTaint{}
	status.Annotations = make(map[string]string)

	n.log.Infof(" 节点 K8s 状态查询完成: node=%s", nodeName)
	return status, nil
}

func (n *NodeOperatorImpl) GetNodeConditions(nodeName string) ([]types.NodeCondition, error) {
	query := fmt.Sprintf(`kube_node_status_condition{node="%s"}`, nodeName)
	results, err := n.query(query, nil)
	if err != nil {
		return nil, err
	}

	condMap := make(map[string]bool)
	for _, r := range results {
		condition := r.Metric["condition"]
		status := r.Metric["status"]
		if condition != "" && status == "true" && r.Value > 0 {
			condMap[condition] = true
		}
	}

	conditions := []types.NodeCondition{
		{Type: "Ready", Status: condMap["Ready"], LastTransitionTime: time.Now()},
		{Type: "MemoryPressure", Status: condMap["MemoryPressure"], LastTransitionTime: time.Now()},
		{Type: "DiskPressure", Status: condMap["DiskPressure"], LastTransitionTime: time.Now()},
		{Type: "PIDPressure", Status: condMap["PIDPressure"], LastTransitionTime: time.Now()},
	}

	return conditions, nil
}

func (n *NodeOperatorImpl) GetNodeCapacity(nodeName string) (*types.NodeResourceQuantity, error) {
	quantity := &types.NodeResourceQuantity{}
	query := fmt.Sprintf(`kube_node_status_capacity{node="%s"}`, nodeName)
	results, err := n.query(query, nil)
	if err != nil {
		return quantity, err
	}

	for _, r := range results {
		resource := r.Metric["resource"]
		switch resource {
		case "cpu":
			quantity.CPUCores = r.Value
		case "memory":
			quantity.MemoryBytes = int64(r.Value)
		case "pods":
			quantity.Pods = int64(r.Value)
		case "ephemeral_storage":
			quantity.EphemeralStorage = int64(r.Value)
		}
	}

	return quantity, nil
}

func (n *NodeOperatorImpl) GetNodeAllocatable(nodeName string) (*types.NodeResourceQuantity, error) {
	quantity := &types.NodeResourceQuantity{}
	query := fmt.Sprintf(`kube_node_status_allocatable{node="%s"}`, nodeName)
	results, err := n.query(query, nil)
	if err != nil {
		return quantity, err
	}

	for _, r := range results {
		resource := r.Metric["resource"]
		switch resource {
		case "cpu":
			quantity.CPUCores = r.Value
		case "memory":
			quantity.MemoryBytes = int64(r.Value)
		case "pods":
			quantity.Pods = int64(r.Value)
		case "ephemeral_storage":
			quantity.EphemeralStorage = int64(r.Value)
		}
	}

	return quantity, nil
}

func (n *NodeOperatorImpl) GetNodeAllocated(nodeName string) (*types.NodeResourceQuantity, error) {
	quantity := &types.NodeResourceQuantity{}

	tasks := []queryTask{
		{
			name:  "cpu_allocated",
			query: fmt.Sprintf(`sum(kube_pod_container_resource_requests{node="%s",resource="cpu"})`, nodeName),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					quantity.CPUCores = results[0].Value
				}
				return nil
			},
		},
		{
			name:  "memory_allocated",
			query: fmt.Sprintf(`sum(kube_pod_container_resource_requests{node="%s",resource="memory"})`, nodeName),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					quantity.MemoryBytes = int64(results[0].Value)
				}
				return nil
			},
		},
		{
			name:  "pods_allocated",
			query: fmt.Sprintf(`count(kube_pod_info{node="%s"})`, nodeName),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					quantity.Pods = int64(results[0].Value)
				}
				return nil
			},
		},
	}

	n.executeParallelQueries(tasks)
	return quantity, nil
}

func (n *NodeOperatorImpl) GetNodeKubelet(nodeName string, timeRange *types.TimeRange) (*types.NodeKubeletMetrics, error) {
	metrics := &types.NodeKubeletMetrics{}
	window := n.calculateRateWindow(timeRange)

	tasks := []queryTask{
		{
			name:  "running_pods",
			query: fmt.Sprintf(`count(kube_pod_info{node="%s"} and on(pod,namespace) kube_pod_status_phase{phase="Running"})`, nodeName),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					metrics.RunningPods = int64(results[0].Value)
				}
				return nil
			},
		},
		{
			name:  "running_containers",
			query: fmt.Sprintf(`count(kube_pod_container_status_running{node="%s"})`, nodeName),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					metrics.RunningContainers = int64(results[0].Value)
				}
				return nil
			},
		},
		{
			name:  "pleg_duration",
			query: fmt.Sprintf(`histogram_quantile(0.99, sum(rate(kubelet_pleg_relist_duration_seconds_bucket{instance=~".*%s.*"}[%s])) by (le))`, nodeName, window),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					metrics.PLEGRelistDuration = results[0].Value
				}
				return nil
			},
		},
		{
			name:  "runtime_ops",
			query: fmt.Sprintf(`sum(rate(kubelet_runtime_operations_total{instance=~".*%s.*"}[%s]))`, nodeName, window),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					metrics.RuntimeOpsRate = results[0].Value
				}
				return nil
			},
		},
		{
			name:  "runtime_ops_errors",
			query: fmt.Sprintf(`sum(rate(kubelet_runtime_operations_errors_total{instance=~".*%s.*"}[%s]))`, nodeName, window),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					metrics.RuntimeOpsErrorRate = results[0].Value
				}
				return nil
			},
		},
		{
			name:  "runtime_ops_duration",
			query: fmt.Sprintf(`histogram_quantile(0.99, sum(rate(kubelet_runtime_operations_duration_seconds_bucket{instance=~".*%s.*"}[%s])) by (le))`, nodeName, window),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					metrics.RuntimeOpsDuration = results[0].Value
				}
				return nil
			},
		},
	}

	n.executeParallelQueries(tasks)

	// 趋势数据
	if timeRange != nil && !timeRange.Start.IsZero() && !timeRange.End.IsZero() {
		step := timeRange.Step
		if step == "" {
			step = n.calculateStep(timeRange.Start, timeRange.End)
		}

		plegQuery := fmt.Sprintf(`histogram_quantile(0.99, sum(rate(kubelet_pleg_relist_duration_seconds_bucket{instance=~".*%s.*"}[%s])) by (le))`, nodeName, window)
		runtimeOpsQuery := fmt.Sprintf(`sum(rate(kubelet_runtime_operations_total{instance=~".*%s.*"}[%s]))`, nodeName, window)
		runtimeOpsErrorQuery := fmt.Sprintf(`sum(rate(kubelet_runtime_operations_errors_total{instance=~".*%s.*"}[%s]))`, nodeName, window)

		var wg sync.WaitGroup
		var mu sync.Mutex
		wg.Add(3)

		go func() {
			defer wg.Done()
			if plegTrend, err := n.queryRange(plegQuery, timeRange.Start, timeRange.End, step); err == nil && len(plegTrend) > 0 && len(plegTrend[0].Values) > 0 {
				mu.Lock()
				defer mu.Unlock()
				metrics.Trend = make([]types.NodeKubeletDataPoint, len(plegTrend[0].Values))
				for i, v := range plegTrend[0].Values {
					metrics.Trend[i] = types.NodeKubeletDataPoint{
						Timestamp:          v.Timestamp,
						PLEGRelistDuration: v.Value,
					}
				}
			}
		}()

		go func() {
			defer wg.Done()
			if runtimeOpsTrend, err := n.queryRange(runtimeOpsQuery, timeRange.Start, timeRange.End, step); err == nil && len(runtimeOpsTrend) > 0 && len(runtimeOpsTrend[0].Values) > 0 {
				mu.Lock()
				defer mu.Unlock()
				for i, v := range runtimeOpsTrend[0].Values {
					if i < len(metrics.Trend) {
						metrics.Trend[i].RuntimeOpsRate = v.Value
					}
				}
			}
		}()

		go func() {
			defer wg.Done()
			if runtimeOpsErrorTrend, err := n.queryRange(runtimeOpsErrorQuery, timeRange.Start, timeRange.End, step); err == nil && len(runtimeOpsErrorTrend) > 0 && len(runtimeOpsErrorTrend[0].Values) > 0 {
				mu.Lock()
				defer mu.Unlock()
				for i, v := range runtimeOpsErrorTrend[0].Values {
					if i < len(metrics.Trend) {
						metrics.Trend[i].RuntimeOpsErrorRate = v.Value
					}
				}
			}
		}()

		wg.Wait()
	}

	return metrics, nil
}

func (n *NodeOperatorImpl) GetNodeSystem(nodeName string, timeRange *types.TimeRange) (*types.NodeSystemMetrics, error) {
	n.log.Infof(" 查询节点系统指标: node=%s", nodeName)

	metrics := &types.NodeSystemMetrics{}

	tasks := []queryTask{
		{
			name:  "procs_running",
			query: fmt.Sprintf(`node_procs_running{instance=~".*%s.*"}`, nodeName),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					metrics.Processes.Running = int64(results[0].Value)
				}
				return nil
			},
		},
		{
			name:  "procs_blocked",
			query: fmt.Sprintf(`node_procs_blocked{instance=~".*%s.*"}`, nodeName),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					metrics.Processes.Blocked = int64(results[0].Value)
				}
				return nil
			},
		},
		{
			name:  "filefd_allocated",
			query: fmt.Sprintf(`node_filefd_allocated{instance=~".*%s.*"}`, nodeName),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					metrics.FileDescriptors.Allocated = int64(results[0].Value)
				}
				return nil
			},
		},
		{
			name:  "filefd_maximum",
			query: fmt.Sprintf(`node_filefd_maximum{instance=~".*%s.*"}`, nodeName),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					metrics.FileDescriptors.Maximum = int64(results[0].Value)
				}
				return nil
			},
		},
	}

	n.executeParallelQueries(tasks)

	if metrics.FileDescriptors.Maximum > 0 {
		metrics.FileDescriptors.UsagePercent = (float64(metrics.FileDescriptors.Allocated) / float64(metrics.FileDescriptors.Maximum)) * 100
	}

	// 趋势数据
	if timeRange != nil && !timeRange.Start.IsZero() && !timeRange.End.IsZero() {
		step := timeRange.Step
		if step == "" {
			step = n.calculateStep(timeRange.Start, timeRange.End)
		}

		allocatedQuery := fmt.Sprintf(`node_filefd_allocated{instance=~".*%s.*"}`, nodeName)
		if allocatedTrend, err := n.queryRange(allocatedQuery, timeRange.Start, timeRange.End, step); err == nil && len(allocatedTrend) > 0 && len(allocatedTrend[0].Values) > 0 {
			metrics.FileDescriptors.Trend = make([]types.NodeFileDescriptorDataPoint, len(allocatedTrend[0].Values))
			for i, v := range allocatedTrend[0].Values {
				allocated := int64(v.Value)
				usagePercent := 0.0
				if metrics.FileDescriptors.Maximum > 0 {
					usagePercent = (float64(allocated) / float64(metrics.FileDescriptors.Maximum)) * 100
				}
				metrics.FileDescriptors.Trend[i] = types.NodeFileDescriptorDataPoint{
					Timestamp:    v.Timestamp,
					Allocated:    allocated,
					UsagePercent: usagePercent,
				}
			}
		}
	}

	n.log.Infof(" 节点系统指标查询完成: node=%s", nodeName)
	return metrics, nil
}

func (n *NodeOperatorImpl) GetNodePods(nodeName string, timeRange *types.TimeRange) (*types.NodePodMetrics, error) {
	n.log.Infof(" 查询节点 Pod: node=%s", nodeName)

	metrics := &types.NodePodMetrics{
		NodeName: nodeName,
	}

	window := n.calculateRateWindow(timeRange)

	tasks := []queryTask{
		{
			name:  "total_pods",
			query: fmt.Sprintf(`count(kube_pod_info{node="%s"})`, nodeName),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					metrics.TotalPods = int64(results[0].Value)
				}
				return nil
			},
		},
		{
			name:  "running_pods",
			query: fmt.Sprintf(`count(kube_pod_status_phase{phase="Running"} == 1 and on(namespace,pod) kube_pod_info{node="%s"})`, nodeName),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					metrics.RunningPods = int64(results[0].Value)
				}
				return nil
			},
		},
		{
			name:  "pending_pods",
			query: fmt.Sprintf(`count(kube_pod_status_phase{phase="Pending"} == 1 and on(namespace,pod) kube_pod_info{node="%s"})`, nodeName),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					metrics.PendingPods = int64(results[0].Value)
				}
				return nil
			},
		},
		{
			name:  "failed_pods",
			query: fmt.Sprintf(`count(kube_pod_status_phase{phase="Failed"} == 1 and on(namespace,pod) kube_pod_info{node="%s"})`, nodeName),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					metrics.FailedPods = int64(results[0].Value)
				}
				return nil
			},
		},
		{
			name:  "top_pods_cpu",
			query: fmt.Sprintf(`topk(10, sum by (pod, namespace) (rate(container_cpu_usage_seconds_total{node="%s",container!="",container!="POD"}[%s])))`, nodeName, window),
			f: func(results []types.InstantQueryResult) error {
				metrics.TopPodsByCPU = make([]types.PodResourceUsage, 0, len(results))
				for _, r := range results {
					if ns, ok := r.Metric["namespace"]; ok {
						if pod, ok := r.Metric["pod"]; ok {
							metrics.TopPodsByCPU = append(metrics.TopPodsByCPU, types.PodResourceUsage{
								Namespace: ns,
								PodName:   pod,
								Value:     r.Value,
								Unit:      "cores",
							})
						}
					}
				}
				return nil
			},
		},
		{
			name:  "top_pods_memory",
			query: fmt.Sprintf(`topk(10, sum by (pod, namespace) (container_memory_working_set_bytes{node="%s",container!="",container!="POD"}))`, nodeName),
			f: func(results []types.InstantQueryResult) error {
				metrics.TopPodsByMem = make([]types.PodResourceUsage, 0, len(results))
				for _, r := range results {
					if ns, ok := r.Metric["namespace"]; ok {
						if pod, ok := r.Metric["pod"]; ok {
							metrics.TopPodsByMem = append(metrics.TopPodsByMem, types.PodResourceUsage{
								Namespace: ns,
								PodName:   pod,
								Value:     r.Value,
								Unit:      "bytes",
							})
						}
					}
				}
				return nil
			},
		},
	}

	n.executeParallelQueries(tasks)

	// Pod 列表
	podListQuery := fmt.Sprintf(`kube_pod_info{node="%s"}`, nodeName)
	podListResults, err := n.query(podListQuery, nil)
	if err == nil && len(podListResults) > 0 {
		// 批量查询所有 pod 的状态 - 不能直接按 node 过滤，需要获取所有然后匹配
		phaseQuery := `kube_pod_status_phase`
		phaseResults, _ := n.query(phaseQuery, nil)

		// 批量查询所有 pod 的 CPU 使用
		cpuQuery := fmt.Sprintf(`sum by (namespace, pod) (rate(container_cpu_usage_seconds_total{node="%s",container!="",container!="POD"}[%s]))`, nodeName, window)
		cpuResults, _ := n.query(cpuQuery, nil)

		// 批量查询所有 pod 的内存使用
		memQuery := fmt.Sprintf(`sum by (namespace, pod) (container_memory_working_set_bytes{node="%s",container!="",container!="POD"})`, nodeName)
		memResults, _ := n.query(memQuery, nil)

		// 批量查询所有 pod 的重启次数
		restartQuery := fmt.Sprintf(`sum by (namespace, pod) (kube_pod_container_status_restarts_total{node="%s"})`, nodeName)
		restartResults, _ := n.query(restartQuery, nil)

		// 先构建该节点的 pod 集合
		nodePods := make(map[string]bool)
		for _, result := range podListResults {
			ns, nsOk := result.Metric["namespace"]
			pod, podOk := result.Metric["pod"]
			if nsOk && podOk {
				key := ns + "/" + pod
				nodePods[key] = true
			}
		}

		// 构建映射表以便快速查找
		phaseMap := make(map[string]string)
		for _, r := range phaseResults {
			ns := r.Metric["namespace"]
			pod := r.Metric["pod"]
			key := ns + "/" + pod

			// 只处理该节点上的 pod
			if !nodePods[key] {
				continue
			}

			if phase, ok := r.Metric["phase"]; ok && r.Value == 1 {
				// 只有当 value == 1 时，该 pod 才处于这个 phase
				phaseMap[key] = phase
			}
		}

		cpuMap := make(map[string]float64)
		for _, r := range cpuResults {
			key := r.Metric["namespace"] + "/" + r.Metric["pod"]
			cpuMap[key] = r.Value
		}

		memMap := make(map[string]int64)
		for _, r := range memResults {
			key := r.Metric["namespace"] + "/" + r.Metric["pod"]
			memMap[key] = int64(r.Value)
		}

		restartMap := make(map[string]int64)
		for _, r := range restartResults {
			key := r.Metric["namespace"] + "/" + r.Metric["pod"]
			restartMap[key] = int64(r.Value)
		}

		// 组装 Pod 列表
		metrics.PodList = make([]types.NodePodBrief, 0, len(podListResults))
		for _, result := range podListResults {
			ns, nsOk := result.Metric["namespace"]
			pod, podOk := result.Metric["pod"]
			if !nsOk || !podOk {
				continue
			}

			key := ns + "/" + pod
			brief := types.NodePodBrief{
				Namespace:    ns,
				PodName:      pod,
				Phase:        phaseMap[key],
				CPUUsage:     cpuMap[key],
				MemoryUsage:  memMap[key],
				RestartCount: restartMap[key],
			}

			if brief.Phase == "" {
				brief.Phase = "Unknown"
			}

			metrics.PodList = append(metrics.PodList, brief)
		}
	}

	n.log.Infof(" 节点 Pod 查询完成: node=%s, total=%d, running=%d, pending=%d, failed=%d",
		nodeName, metrics.TotalPods, metrics.RunningPods, metrics.PendingPods, metrics.FailedPods)
	return metrics, nil
}

// ==================== 对比和排行 ====================

func (n *NodeOperatorImpl) CompareNodes(nodeNames []string, timeRange *types.TimeRange) (*types.NodeComparison, error) {
	n.log.Infof(" 对比节点: nodes=%v", nodeNames)

	comparison := &types.NodeComparison{
		Timestamp: time.Now(),
		Nodes:     make([]types.NodeComparisonItem, 0, len(nodeNames)),
	}

	window := n.calculateRateWindow(timeRange)

	// 并发查询所有节点
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, nodeName := range nodeNames {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()

			item := types.NodeComparisonItem{
				NodeName: name,
			}

			// 分别查询各个指标
			cpuQuery := fmt.Sprintf(`100 - (avg(irate(node_cpu_seconds_total{instance=~".*%s.*",mode="idle"}[%s])) * 100)`, name, window)
			if cpuResults, err := n.query(cpuQuery, nil); err == nil && len(cpuResults) > 0 {
				item.CPUUsage = cpuResults[0].Value
			}

			memQuery := fmt.Sprintf(`(1 - (node_memory_MemAvailable_bytes{instance=~".*%s.*"} / node_memory_MemTotal_bytes{instance=~".*%s.*"})) * 100`, name, name)
			if memResults, err := n.query(memQuery, nil); err == nil && len(memResults) > 0 {
				item.MemoryUsage = memResults[0].Value
			}

			diskQuery := fmt.Sprintf(`100 - ((node_filesystem_avail_bytes{instance=~".*%s.*",mountpoint="/"} / node_filesystem_size_bytes{instance=~".*%s.*",mountpoint="/"}) * 100)`, name, name)
			if diskResults, err := n.query(diskQuery, nil); err == nil && len(diskResults) > 0 {
				item.DiskUsage = diskResults[0].Value
			}

			loadQuery := fmt.Sprintf(`node_load5{instance=~".*%s.*"}`, name)
			if loadResults, err := n.query(loadQuery, nil); err == nil && len(loadResults) > 0 {
				item.Load5 = loadResults[0].Value
			}

			// Pod 数量
			podQuery := fmt.Sprintf(`count(kube_pod_info{node="%s"})`, name)
			if podResults, err := n.query(podQuery, nil); err == nil && len(podResults) > 0 {
				item.PodCount = int64(podResults[0].Value)
			}

			// Ready 状态
			readyQuery := fmt.Sprintf(`kube_node_status_condition{node="%s",condition="Ready",status="true"}`, name)
			if readyResults, err := n.query(readyQuery, nil); err == nil && len(readyResults) > 0 {
				item.Ready = readyResults[0].Value > 0
			}

			mu.Lock()
			comparison.Nodes = append(comparison.Nodes, item)
			mu.Unlock()
		}(nodeName)
	}

	wg.Wait()

	n.log.Infof(" 节点对比完成: count=%d", len(comparison.Nodes))
	return comparison, nil
}

func (n *NodeOperatorImpl) GetNodeRanking(limit int, timeRange *types.TimeRange) (*types.NodeRanking, error) {
	n.log.Infof(" 查询节点排行: limit=%d", limit)

	ranking := &types.NodeRanking{}
	window := n.calculateRateWindow(timeRange)

	var wg sync.WaitGroup
	wg.Add(5)

	go func() {
		defer wg.Done()
		topCPUQuery := fmt.Sprintf(`topk(%d, 100 - (avg by (instance) (irate(node_cpu_seconds_total{mode="idle"}[%s])) * 100))`, limit, window)
		if topCPUResults, err := n.query(topCPUQuery, nil); err == nil {
			ranking.TopByCPU = make([]types.NodeRankingItem, 0, len(topCPUResults))
			for _, r := range topCPUResults {
				if instance, ok := r.Metric["instance"]; ok {
					ranking.TopByCPU = append(ranking.TopByCPU, types.NodeRankingItem{
						NodeName: n.extractNodeName(instance),
						Value:    r.Value,
						Unit:     "percent",
					})
				}
			}
		}
	}()

	go func() {
		defer wg.Done()
		topMemQuery := fmt.Sprintf(`topk(%d, (1 - (node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes)) * 100)`, limit)
		if topMemResults, err := n.query(topMemQuery, nil); err == nil {
			ranking.TopByMemory = make([]types.NodeRankingItem, 0, len(topMemResults))
			for _, r := range topMemResults {
				if instance, ok := r.Metric["instance"]; ok {
					ranking.TopByMemory = append(ranking.TopByMemory, types.NodeRankingItem{
						NodeName: n.extractNodeName(instance),
						Value:    r.Value,
						Unit:     "percent",
					})
				}
			}
		}
	}()

	go func() {
		defer wg.Done()
		topDiskQuery := fmt.Sprintf(`topk(%d, 100 - ((node_filesystem_avail_bytes{mountpoint="/"} / node_filesystem_size_bytes{mountpoint="/"}) * 100))`, limit)
		if topDiskResults, err := n.query(topDiskQuery, nil); err == nil {
			ranking.TopByDisk = make([]types.NodeRankingItem, 0, len(topDiskResults))
			for _, r := range topDiskResults {
				if instance, ok := r.Metric["instance"]; ok {
					ranking.TopByDisk = append(ranking.TopByDisk, types.NodeRankingItem{
						NodeName: n.extractNodeName(instance),
						Value:    r.Value,
						Unit:     "percent",
					})
				}
			}
		}
	}()

	go func() {
		defer wg.Done()
		topLoadQuery := fmt.Sprintf(`topk(%d, node_load5)`, limit)
		if topLoadResults, err := n.query(topLoadQuery, nil); err == nil {
			ranking.TopByLoad = make([]types.NodeRankingItem, 0, len(topLoadResults))
			for _, r := range topLoadResults {
				if instance, ok := r.Metric["instance"]; ok {
					ranking.TopByLoad = append(ranking.TopByLoad, types.NodeRankingItem{
						NodeName: n.extractNodeName(instance),
						Value:    r.Value,
						Unit:     "load",
					})
				}
			}
		}
	}()

	go func() {
		defer wg.Done()
		topPodsQuery := fmt.Sprintf(`topk(%d, count by (node) (kube_pod_info))`, limit)
		if topPodsResults, err := n.query(topPodsQuery, nil); err == nil {
			ranking.TopByPods = make([]types.NodeRankingItem, 0, len(topPodsResults))
			for _, r := range topPodsResults {
				if node, ok := r.Metric["node"]; ok {
					ranking.TopByPods = append(ranking.TopByPods, types.NodeRankingItem{
						NodeName: node,
						Value:    r.Value,
						Unit:     "pods",
					})
				}
			}
		}
	}()

	wg.Wait()

	n.log.Infof(" 节点排行查询完成")
	return ranking, nil
}

func (n *NodeOperatorImpl) ListNodesMetrics(timeRange *types.TimeRange) ([]types.NodeMetrics, error) {
	n.log.Info(" 列出所有节点指标")

	// 获取所有节点
	nodesQuery := `kube_node_info`
	nodesResults, err := n.query(nodesQuery, nil)
	if err != nil {
		return nil, fmt.Errorf("查询节点列表失败: %w", err)
	}

	// 并发查询所有节点的指标
	var wg sync.WaitGroup
	var mu sync.Mutex
	nodeMetrics := make([]types.NodeMetrics, 0, len(nodesResults))

	for _, result := range nodesResults {
		if nodeName, ok := result.Metric["node"]; ok {
			wg.Add(1)
			go func(name string) {
				defer wg.Done()
				if metrics, err := n.GetNodeMetrics(name, timeRange); err == nil {
					mu.Lock()
					nodeMetrics = append(nodeMetrics, *metrics)
					mu.Unlock()
				} else {
					n.log.Errorf("获取节点指标失败: node=%s, error=%v", name, err)
				}
			}(nodeName)
		}
	}

	wg.Wait()

	n.log.Infof(" 列出所有节点指标完成: count=%d", len(nodeMetrics))
	return nodeMetrics, nil
}

func (n *NodeOperatorImpl) GetNodeNetworkInterface(nodeName string, interfaceName string, timeRange *types.TimeRange) (*types.NodeNetworkInterfaceMetrics, error) {
	n.log.Infof(" 查询节点指定网络接口: node=%s, interface=%s", nodeName, interfaceName)

	ifMetrics := &types.NodeNetworkInterfaceMetrics{
		InterfaceName: interfaceName,
	}

	window := n.calculateRateWindow(timeRange)

	// 分别查询各个指标
	tasks := []queryTask{
		{
			name:  "net_rx_bytes",
			query: fmt.Sprintf(`irate(node_network_receive_bytes_total{instance=~".*%s.*",device="%s"}[%s])`, nodeName, interfaceName, window),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					ifMetrics.Current.ReceiveBytesPerSec = results[0].Value
					ifMetrics.Current.Timestamp = results[0].Time
				}
				return nil
			},
		},
		{
			name:  "net_tx_bytes",
			query: fmt.Sprintf(`irate(node_network_transmit_bytes_total{instance=~".*%s.*",device="%s"}[%s])`, nodeName, interfaceName, window),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					ifMetrics.Current.TransmitBytesPerSec = results[0].Value
				}
				return nil
			},
		},
		{
			name:  "net_rx_packets",
			query: fmt.Sprintf(`irate(node_network_receive_packets_total{instance=~".*%s.*",device="%s"}[%s])`, nodeName, interfaceName, window),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					ifMetrics.Current.ReceivePacketsPerSec = results[0].Value
				}
				return nil
			},
		},
		{
			name:  "net_tx_packets",
			query: fmt.Sprintf(`irate(node_network_transmit_packets_total{instance=~".*%s.*",device="%s"}[%s])`, nodeName, interfaceName, window),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					ifMetrics.Current.TransmitPacketsPerSec = results[0].Value
				}
				return nil
			},
		},
		{
			name:  "net_rx_errs",
			query: fmt.Sprintf(`irate(node_network_receive_errs_total{instance=~".*%s.*",device="%s"}[%s])`, nodeName, interfaceName, window),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					ifMetrics.Current.ReceiveErrorsRate = results[0].Value
				}
				return nil
			},
		},
		{
			name:  "net_tx_errs",
			query: fmt.Sprintf(`irate(node_network_transmit_errs_total{instance=~".*%s.*",device="%s"}[%s])`, nodeName, interfaceName, window),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					ifMetrics.Current.TransmitErrorsRate = results[0].Value
				}
				return nil
			},
		},
		{
			name:  "net_rx_drop",
			query: fmt.Sprintf(`irate(node_network_receive_drop_total{instance=~".*%s.*",device="%s"}[%s])`, nodeName, interfaceName, window),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					ifMetrics.Current.ReceiveDropsRate = results[0].Value
				}
				return nil
			},
		},
		{
			name:  "net_tx_drop",
			query: fmt.Sprintf(`irate(node_network_transmit_drop_total{instance=~".*%s.*",device="%s"}[%s])`, nodeName, interfaceName, window),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					ifMetrics.Current.TransmitDropsRate = results[0].Value
				}
				return nil
			},
		},
	}

	n.executeParallelQueries(tasks)

	if ifMetrics.Current.Timestamp.IsZero() {
		return nil, fmt.Errorf("未找到指定网络接口: node=%s, interface=%s", nodeName, interfaceName)
	}

	// 趋势数据
	if timeRange != nil && !timeRange.Start.IsZero() && !timeRange.End.IsZero() {
		step := timeRange.Step
		if step == "" {
			step = n.calculateStep(timeRange.Start, timeRange.End)
		}

		rxQuery := fmt.Sprintf(`irate(node_network_receive_bytes_total{instance=~".*%s.*",device="%s"}[%s])`, nodeName, interfaceName, window)
		txQuery := fmt.Sprintf(`irate(node_network_transmit_bytes_total{instance=~".*%s.*",device="%s"}[%s])`, nodeName, interfaceName, window)

		var wg sync.WaitGroup
		var mu sync.Mutex
		wg.Add(2)

		go func() {
			defer wg.Done()
			if rxTrend, err := n.queryRange(rxQuery, timeRange.Start, timeRange.End, step); err == nil && len(rxTrend) > 0 && len(rxTrend[0].Values) > 0 {
				mu.Lock()
				defer mu.Unlock()
				ifMetrics.Trend = make([]types.NodeNetworkInterfaceDataPoint, len(rxTrend[0].Values))
				for i, v := range rxTrend[0].Values {
					ifMetrics.Trend[i] = types.NodeNetworkInterfaceDataPoint{
						Timestamp:          v.Timestamp,
						ReceiveBytesPerSec: v.Value,
					}
				}
			}
		}()

		go func() {
			defer wg.Done()
			if txTrend, err := n.queryRange(txQuery, timeRange.Start, timeRange.End, step); err == nil && len(txTrend) > 0 && len(txTrend[0].Values) > 0 {
				mu.Lock()
				defer mu.Unlock()
				for i, v := range txTrend[0].Values {
					if i < len(ifMetrics.Trend) {
						ifMetrics.Trend[i].TransmitBytesPerSec = v.Value
					}
				}
			}
		}()

		wg.Wait()
		if len(ifMetrics.Trend) > 0 {
			ifMetrics.Summary = n.calculateNetworkInterfaceSummary(ifMetrics.Trend, interfaceName)
		}
	}

	n.log.Infof(" 节点网络接口查询完成: node=%s, interface=%s", nodeName, interfaceName)
	return ifMetrics, nil
}

// ==================== 辅助方法 ====================

func (n *NodeOperatorImpl) query(query string, timestamp *time.Time) ([]types.InstantQueryResult, error) {
	params := map[string]string{"query": query}
	if timestamp != nil {
		params["time"] = n.formatTimestamp(*timestamp)
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

	if err := n.doRequest("GET", "/api/v1/query", params, nil, &response); err != nil {
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

func (n *NodeOperatorImpl) queryRange(query string, start, end time.Time, step string) ([]types.RangeQueryResult, error) {
	params := map[string]string{
		"query": query,
		"start": n.formatTimestamp(start),
		"end":   n.formatTimestamp(end),
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

	if err := n.doRequest("GET", "/api/v1/query_range", params, nil, &response); err != nil {
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

func (n *NodeOperatorImpl) formatTimestamp(t time.Time) string {
	return fmt.Sprintf("%.3f", float64(t.Unix())+float64(t.Nanosecond())/1e9)
}

func (n *NodeOperatorImpl) calculateRateWindow(timeRange *types.TimeRange) string {
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

func (n *NodeOperatorImpl) calculateStep(start, end time.Time) string {
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

func (n *NodeOperatorImpl) calculateCPUSummary(trend []types.NodeCPUDataPoint) types.NodeCPUSummary {
	if len(trend) == 0 {
		return types.NodeCPUSummary{}
	}

	var sum, max, min, sumLoad, maxLoad float64
	min = math.MaxFloat64

	for _, point := range trend {
		sum += point.UsagePercent
		sumLoad += point.Load5
		if point.UsagePercent > max {
			max = point.UsagePercent
		}
		if point.UsagePercent < min {
			min = point.UsagePercent
		}
		if point.Load5 > maxLoad {
			maxLoad = point.Load5
		}
	}

	return types.NodeCPUSummary{
		AvgUsagePercent: sum / float64(len(trend)),
		MaxUsagePercent: max,
		MinUsagePercent: min,
		AvgLoad5:        sumLoad / float64(len(trend)),
		MaxLoad5:        maxLoad,
	}
}

func (n *NodeOperatorImpl) calculateMemorySummary(trend []types.NodeMemoryDataPoint) types.NodeMemorySummary {
	if len(trend) == 0 {
		return types.NodeMemorySummary{}
	}

	var sumPercent, maxPercent, minPercent float64
	minPercent = math.MaxFloat64

	for _, point := range trend {
		sumPercent += point.UsagePercent
		if point.UsagePercent > maxPercent {
			maxPercent = point.UsagePercent
		}
		if point.UsagePercent < minPercent {
			minPercent = point.UsagePercent
		}
	}

	return types.NodeMemorySummary{
		AvgUsagePercent: sumPercent / float64(len(trend)),
		MaxUsagePercent: maxPercent,
		MinUsagePercent: minPercent,
	}
}

func (n *NodeOperatorImpl) calculateDiskDeviceSummary(trend []types.NodeDiskDeviceDataPoint) types.NodeDiskDeviceSummary {
	if len(trend) == 0 {
		return types.NodeDiskDeviceSummary{}
	}

	var sumRead, maxRead, sumWrite, maxWrite, sumIO, maxIO float64

	for _, point := range trend {
		sumRead += point.ReadBytesPerSec
		sumWrite += point.WriteBytesPerSec
		sumIO += point.IOUtilizationPercent

		if point.ReadBytesPerSec > maxRead {
			maxRead = point.ReadBytesPerSec
		}
		if point.WriteBytesPerSec > maxWrite {
			maxWrite = point.WriteBytesPerSec
		}
		if point.IOUtilizationPercent > maxIO {
			maxIO = point.IOUtilizationPercent
		}
	}

	return types.NodeDiskDeviceSummary{
		AvgReadBytesPerSec:  sumRead / float64(len(trend)),
		MaxReadBytesPerSec:  maxRead,
		AvgWriteBytesPerSec: sumWrite / float64(len(trend)),
		MaxWriteBytesPerSec: maxWrite,
		AvgIOUtilization:    sumIO / float64(len(trend)),
		MaxIOUtilization:    maxIO,
	}
}

func (n *NodeOperatorImpl) calculateNetworkInterfaceSummary(trend []types.NodeNetworkInterfaceDataPoint, ifName string) types.NodeNetworkInterfaceSummary {
	if len(trend) == 0 {
		return types.NodeNetworkInterfaceSummary{}
	}

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

	return types.NodeNetworkInterfaceSummary{
		AvgReceiveBytesPerSec:  sumRx / float64(len(trend)),
		MaxReceiveBytesPerSec:  maxRx,
		AvgTransmitBytesPerSec: sumTx / float64(len(trend)),
		MaxTransmitBytesPerSec: maxTx,
	}
}

func (n *NodeOperatorImpl) getNodeInfo(nodeName string) types.NodeInfo {
	info := types.NodeInfo{}
	query := fmt.Sprintf(`kube_node_info{node="%s"}`, nodeName)
	if results, err := n.query(query, nil); err == nil && len(results) > 0 {
		m := results[0].Metric
		info.KubeletVersion = m["kubelet_version"]
		info.ContainerRuntimeVersion = m["container_runtime_version"]
		info.KernelVersion = m["kernel_version"]
		info.OSImage = m["os_image"]
		info.Architecture = m["architecture"]
	}

	// Boot 时间
	bootQuery := fmt.Sprintf(`node_boot_time_seconds{instance=~".*%s.*"}`, nodeName)
	if bootResults, err := n.query(bootQuery, nil); err == nil && len(bootResults) > 0 {
		bootTime := int64(bootResults[0].Value)
		info.BootTime = time.Unix(bootTime, 0)
		info.UptimeSeconds = time.Now().Unix() - bootTime
	}

	return info
}

func (n *NodeOperatorImpl) getNodeLabels(nodeName string) map[string]string {
	labels := make(map[string]string)
	query := fmt.Sprintf(`kube_node_labels{node="%s"}`, nodeName)
	if results, err := n.query(query, nil); err == nil && len(results) > 0 {
		for k, v := range results[0].Metric {
			if k != "__name__" && k != "node" && k != "instance" && k != "job" {
				labels[k] = v
			}
		}
	}
	return labels
}

func (n *NodeOperatorImpl) extractNodeName(instance string) string {
	// 从 instance 中提取节点名（格式通常为 "node-name:port"）
	if idx := strings.Index(instance, ":"); idx > 0 {
		return instance[:idx]
	}
	return instance
}
