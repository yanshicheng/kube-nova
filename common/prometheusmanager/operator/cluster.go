package operator

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/yanshicheng/kube-nova/common/prometheusmanager/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type ClusterOperator struct {
	log logx.Logger
	ctx context.Context
	*BaseOperator
}

func NewClusterOperator(ctx context.Context, base *BaseOperator) types.ClusterOperator {
	return &ClusterOperator{
		log:          logx.WithContext(ctx),
		ctx:          ctx,
		BaseOperator: base,
	}
}

// ==================== 并发查询辅助方法 ====================

type clusterQueryTask struct {
	name  string
	query string
	f     func([]types.InstantQueryResult) error
}

type clusterRangeTask struct {
	name  string
	query string
	f     func([]types.RangeQueryResult) error
}

func (c *ClusterOperator) executeParallelQueries(tasks []clusterQueryTask) {
	var wg sync.WaitGroup
	for _, task := range tasks {
		wg.Add(1)
		go func(t clusterQueryTask) {
			defer wg.Done()
			results, err := c.query(t.query, nil)
			if err != nil {
				c.log.Errorf("查询失败 [%s]: %v", t.name, err)
				return
			}
			if err := t.f(results); err != nil {
				c.log.Errorf("处理结果失败 [%s]: %v", t.name, err)
			}
		}(task)
	}
	wg.Wait()
}

func (c *ClusterOperator) executeParallelRangeQueries(start, end time.Time, step string, tasks []clusterRangeTask) {
	var wg sync.WaitGroup
	for _, task := range tasks {
		wg.Add(1)
		go func(t clusterRangeTask) {
			defer wg.Done()
			results, err := c.queryRange(t.query, start, end, step)
			if err != nil {
				c.log.Errorf("范围查询失败 [%s]: %v", t.name, err)
				return
			}
			if err := t.f(results); err != nil {
				c.log.Errorf("处理范围结果失败 [%s]: %v", t.name, err)
			}
		}(task)
	}
	wg.Wait()
}

func (c *ClusterOperator) GetClusterOverview(timeRange *types.TimeRange) (*types.ClusterOverview, error) {
	c.log.Infof(" 查询集群概览")

	overview := &types.ClusterOverview{
		ClusterName: "kubernetes-cluster",
		Timestamp:   time.Now(),
	}

	var wg sync.WaitGroup
	wg.Add(6)

	go func() {
		defer wg.Done()
		if resources, err := c.GetClusterResources(timeRange); err == nil {
			overview.Resources = *resources
		} else {
			c.log.Errorf("获取资源指标失败: %v", err)
		}
	}()

	go func() {
		defer wg.Done()
		if nodes, err := c.GetClusterNodes(timeRange); err == nil {
			overview.Nodes = *nodes
		} else {
			c.log.Errorf("获取节点指标失败: %v", err)
		}
	}()

	go func() {
		defer wg.Done()
		if controlPlane, err := c.getControlPlaneMetrics(timeRange); err == nil {
			overview.ControlPlane = *controlPlane
		} else {
			c.log.Errorf("获取控制平面指标失败: %v", err)
		}
	}()

	go func() {
		defer wg.Done()
		if workloads, err := c.GetClusterWorkloads(timeRange); err == nil {
			overview.Workloads = *workloads
		} else {
			c.log.Errorf("获取工作负载指标失败: %v", err)
		}
	}()

	go func() {
		defer wg.Done()
		if network, err := c.GetClusterNetwork(timeRange); err == nil {
			overview.Network = *network
		} else {
			c.log.Errorf("获取网络指标失败: %v", err)
		}
	}()

	go func() {
		defer wg.Done()
		if storage, err := c.GetClusterStorage(); err == nil {
			overview.Storage = *storage
		} else {
			c.log.Errorf("获取存储指标失败: %v", err)
		}
	}()

	wg.Wait()

	c.log.Infof("集群概览查询完成")
	return overview, nil
}

func (c *ClusterOperator) GetClusterResources(timeRange *types.TimeRange) (*types.ClusterResourceMetrics, error) {
	resources := &types.ClusterResourceMetrics{}

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		if cpu, err := c.GetClusterCPUMetrics(timeRange); err == nil {
			resources.CPU = *cpu
		}
	}()

	go func() {
		defer wg.Done()
		if memory, err := c.GetClusterMemoryMetrics(timeRange); err == nil {
			resources.Memory = *memory
		}
	}()

	wg.Wait()

	// 合并查询 Pods 和 Storage
	tasks := []clusterQueryTask{
		{
			name:  "pods_running",
			query: `sum(kube_pod_status_phase{phase="Running"})`,
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					resources.Pods.Running = int64(results[0].Value)
				}
				return nil
			},
		},
		{
			name:  "pods_capacity",
			query: `sum(kube_node_status_capacity{resource="pods"})`,
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					resources.Pods.Capacity = int64(results[0].Value)
					if resources.Pods.Capacity > 0 {
						resources.Pods.UsagePercent = (float64(resources.Pods.Running) / float64(resources.Pods.Capacity)) * 100
					}
				}
				return nil
			},
		},
		{
			name:  "pv_capacity",
			query: `sum(kube_persistentvolume_capacity_bytes)`,
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					resources.Storage.TotalCapacityBytes = int64(results[0].Value)
				}
				return nil
			},
		},
		{
			name:  "pvc_requests",
			query: `sum(kube_persistentvolumeclaim_resource_requests_storage_bytes)`,
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					resources.Storage.AllocatedBytes = int64(results[0].Value)
					if resources.Storage.TotalCapacityBytes > 0 {
						resources.Storage.AllocationPercent = (float64(resources.Storage.AllocatedBytes) / float64(resources.Storage.TotalCapacityBytes)) * 100
					}
				}
				return nil
			},
		},
	}

	c.executeParallelQueries(tasks)
	return resources, nil
}

func (c *ClusterOperator) GetClusterCPUMetrics(timeRange *types.TimeRange) (*types.ClusterResourceSummary, error) {
	cpu := &types.ClusterResourceSummary{Trend: []types.ClusterResourceDataPoint{}}
	window := c.calculateRateWindow(timeRange)

	// 合并查询所有 CPU 指标
	tasks := []clusterQueryTask{
		{
			name:  "cpu_capacity",
			query: `sum(kube_node_status_capacity{resource="cpu"})`,
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					cpu.Capacity = results[0].Value
				}
				return nil
			},
		},
		{
			name:  "cpu_allocatable",
			query: `sum(kube_node_status_allocatable{resource="cpu"})`,
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					cpu.Allocatable = results[0].Value
				}
				return nil
			},
		},
		{
			name:  "cpu_requests",
			query: `sum(kube_pod_container_resource_requests{resource="cpu"})`,
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					cpu.RequestsAllocated = results[0].Value
				}
				return nil
			},
		},
		{
			name:  "cpu_limits",
			query: `sum(kube_pod_container_resource_limits{resource="cpu"})`,
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					cpu.LimitsAllocated = results[0].Value
				}
				return nil
			},
		},
		{
			name:  "cpu_usage",
			query: fmt.Sprintf(`sum(rate(container_cpu_usage_seconds_total{container!=""}[%s]))`, window),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					cpu.Usage = results[0].Value
				}
				return nil
			},
		},
		{
			name:  "cpu_no_requests",
			query: `count(kube_pod_container_resource_requests{resource="cpu"} == 0)`,
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					cpu.NoRequestsCount = int64(results[0].Value)
				}
				return nil
			},
		},
		{
			name:  "cpu_no_limits",
			query: `count(kube_pod_container_resource_limits{resource="cpu"} == 0)`,
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					cpu.NoLimitsCount = int64(results[0].Value)
				}
				return nil
			},
		},
	}

	c.executeParallelQueries(tasks)

	// 计算百分比
	if cpu.Allocatable > 0 {
		cpu.RequestsPercent = (cpu.RequestsAllocated / cpu.Allocatable) * 100
		cpu.UsagePercent = (cpu.Usage / cpu.Allocatable) * 100
	}

	// 趋势数据
	if timeRange != nil && !timeRange.Start.IsZero() && !timeRange.End.IsZero() {
		step := timeRange.Step
		if step == "" {
			step = c.calculateStep(timeRange.Start, timeRange.End)
		}

		trendQuery := fmt.Sprintf(`sum(rate(container_cpu_usage_seconds_total{container!=""}[%s]))`, window)
		if trendResult, err := c.queryRange(trendQuery, timeRange.Start, timeRange.End, step); err == nil && len(trendResult) > 0 {
			cpu.Trend = make([]types.ClusterResourceDataPoint, len(trendResult[0].Values))
			for i, v := range trendResult[0].Values {
				usagePercent := 0.0
				requestsPercent := 0.0
				if cpu.Allocatable > 0 {
					usagePercent = (v.Value / cpu.Allocatable) * 100
					requestsPercent = (cpu.RequestsAllocated / cpu.Allocatable) * 100
				}
				cpu.Trend[i] = types.ClusterResourceDataPoint{
					Timestamp:       v.Timestamp,
					Usage:           v.Value,
					UsagePercent:    usagePercent,
					RequestsPercent: requestsPercent,
				}
			}
		}
	}

	return cpu, nil
}

func (c *ClusterOperator) GetClusterMemoryMetrics(timeRange *types.TimeRange) (*types.ClusterResourceSummary, error) {
	memory := &types.ClusterResourceSummary{Trend: []types.ClusterResourceDataPoint{}}

	// 合并查询所有内存指标
	tasks := []clusterQueryTask{
		{
			name:  "memory_capacity",
			query: `sum(kube_node_status_capacity{resource="memory"})`,
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					memory.Capacity = results[0].Value
				}
				return nil
			},
		},
		{
			name:  "memory_allocatable",
			query: `sum(kube_node_status_allocatable{resource="memory"})`,
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					memory.Allocatable = results[0].Value
				}
				return nil
			},
		},
		{
			name:  "memory_requests",
			query: `sum(kube_pod_container_resource_requests{resource="memory"})`,
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					memory.RequestsAllocated = results[0].Value
				}
				return nil
			},
		},
		{
			name:  "memory_limits",
			query: `sum(kube_pod_container_resource_limits{resource="memory"})`,
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					memory.LimitsAllocated = results[0].Value
				}
				return nil
			},
		},
		{
			name:  "memory_usage",
			query: `sum(container_memory_usage_bytes{container!=""})`,
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					memory.Usage = results[0].Value
				}
				return nil
			},
		},
	}

	c.executeParallelQueries(tasks)

	// 计算百分比
	if memory.Allocatable > 0 {
		memory.RequestsPercent = (memory.RequestsAllocated / memory.Allocatable) * 100
		memory.UsagePercent = (memory.Usage / memory.Allocatable) * 100
	}

	// 趋势数据
	if timeRange != nil && !timeRange.Start.IsZero() && !timeRange.End.IsZero() {
		step := timeRange.Step
		if step == "" {
			step = c.calculateStep(timeRange.Start, timeRange.End)
		}

		trendQuery := `sum(container_memory_usage_bytes{container!=""})`
		if trendResult, err := c.queryRange(trendQuery, timeRange.Start, timeRange.End, step); err == nil && len(trendResult) > 0 {
			memory.Trend = make([]types.ClusterResourceDataPoint, len(trendResult[0].Values))
			for i, v := range trendResult[0].Values {
				usagePercent := 0.0
				requestsPercent := 0.0
				if memory.Allocatable > 0 {
					usagePercent = (v.Value / memory.Allocatable) * 100
					requestsPercent = (memory.RequestsAllocated / memory.Allocatable) * 100
				}
				memory.Trend[i] = types.ClusterResourceDataPoint{
					Timestamp:       v.Timestamp,
					Usage:           v.Value,
					UsagePercent:    usagePercent,
					RequestsPercent: requestsPercent,
				}
			}
		}
	}

	return memory, nil
}

func (c *ClusterOperator) GetClusterNodes(timeRange *types.TimeRange) (*types.ClusterNodeMetrics, error) {
	nodes := &types.ClusterNodeMetrics{
		NodeList: []types.NodeBriefStatus{},
		Trend:    []types.ClusterNodeDataPoint{},
	}

	window := c.calculateRateWindow(timeRange)

	// 合并查询所有节点统计
	tasks := []clusterQueryTask{
		{
			name:  "nodes_total",
			query: `count(kube_node_info)`,
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					nodes.Total = int64(results[0].Value)
				}
				return nil
			},
		},
		{
			name:  "nodes_ready",
			query: `sum(kube_node_status_condition{condition="Ready",status="true"})`,
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					nodes.Ready = int64(results[0].Value)
				}
				return nil
			},
		},
		{
			name:  "nodes_not_ready",
			query: `sum(kube_node_status_condition{condition="Ready",status="false"})`,
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					nodes.NotReady = int64(results[0].Value)
				}
				return nil
			},
		},
		{
			name:  "nodes_unknown",
			query: `sum(kube_node_status_condition{condition="Ready",status="unknown"})`,
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					nodes.Unknown = int64(results[0].Value)
				}
				return nil
			},
		},
		{
			name:  "memory_pressure",
			query: `sum(kube_node_status_condition{condition="MemoryPressure",status="true"})`,
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					nodes.MemoryPressure = int64(results[0].Value)
				}
				return nil
			},
		},
		{
			name:  "disk_pressure",
			query: `sum(kube_node_status_condition{condition="DiskPressure",status="true"})`,
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					nodes.DiskPressure = int64(results[0].Value)
				}
				return nil
			},
		},
		{
			name:  "pid_pressure",
			query: `sum(kube_node_status_condition{condition="PIDPressure",status="true"})`,
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					nodes.PIDPressure = int64(results[0].Value)
				}
				return nil
			},
		},
		{
			name:  "avg_cpu",
			query: fmt.Sprintf(`avg(rate(node_cpu_seconds_total{mode!="idle"}[%s]))`, window),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					nodes.AvgCPUUsage = results[0].Value * 100
				}
				return nil
			},
		},
		{
			name:  "avg_memory",
			query: `avg((node_memory_MemTotal_bytes - node_memory_MemAvailable_bytes) / node_memory_MemTotal_bytes)`,
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					nodes.AvgMemoryUsage = results[0].Value * 100
				}
				return nil
			},
		},
		{
			name:  "high_load",
			query: fmt.Sprintf(`count((rate(node_cpu_seconds_total{mode!="idle"}[%s]) > 0.8) OR ((node_memory_MemTotal_bytes - node_memory_MemAvailable_bytes) / node_memory_MemTotal_bytes > 0.8))`, window),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					nodes.HighLoadNodes = int64(results[0].Value)
				}
				return nil
			},
		},
	}

	c.executeParallelQueries(tasks)

	// 并发获取节点列表
	go func() {
		if nodeList, err := c.getDetailedNodeListOptimized(timeRange); err == nil && len(nodeList) > 0 {
			nodes.NodeList = nodeList
		} else {
			c.log.Errorf("获取节点列表失败: %v", err)
		}
	}()

	// 趋势数据
	if timeRange != nil && !timeRange.Start.IsZero() && !timeRange.End.IsZero() {
		step := timeRange.Step
		if step == "" {
			step = c.calculateStep(timeRange.Start, timeRange.End)
		}

		var mu sync.Mutex
		rangeTasks := []clusterRangeTask{
			{
				name:  "ready_trend",
				query: `sum(kube_node_status_condition{condition="Ready",status="true"})`,
				f: func(results []types.RangeQueryResult) error {
					if len(results) > 0 {
						mu.Lock()
						defer mu.Unlock()
						nodes.Trend = make([]types.ClusterNodeDataPoint, len(results[0].Values))
						for i, v := range results[0].Values {
							nodes.Trend[i] = types.ClusterNodeDataPoint{
								Timestamp:  v.Timestamp,
								ReadyNodes: int64(v.Value),
							}
						}
					}
					return nil
				},
			},
			{
				name:  "not_ready_trend",
				query: `sum(kube_node_status_condition{condition="Ready",status="false"})`,
				f: func(results []types.RangeQueryResult) error {
					if len(results) > 0 {
						mu.Lock()
						defer mu.Unlock()
						for i, v := range results[0].Values {
							if i < len(nodes.Trend) {
								nodes.Trend[i].NotReadyNodes = int64(v.Value)
							}
						}
					}
					return nil
				},
			},
			{
				name:  "avg_cpu_trend",
				query: fmt.Sprintf(`avg(rate(node_cpu_seconds_total{mode!="idle"}[%s])) * 100`, window),
				f: func(results []types.RangeQueryResult) error {
					if len(results) > 0 {
						mu.Lock()
						defer mu.Unlock()
						for i, v := range results[0].Values {
							if i < len(nodes.Trend) {
								nodes.Trend[i].AvgCPUUsage = v.Value
							}
						}
					}
					return nil
				},
			},
			{
				name:  "avg_memory_trend",
				query: `avg((node_memory_MemTotal_bytes - node_memory_MemAvailable_bytes) / node_memory_MemTotal_bytes) * 100`,
				f: func(results []types.RangeQueryResult) error {
					if len(results) > 0 {
						mu.Lock()
						defer mu.Unlock()
						for i, v := range results[0].Values {
							if i < len(nodes.Trend) {
								nodes.Trend[i].AvgMemoryUsage = v.Value
							}
						}
					}
					return nil
				},
			},
		}

		c.executeParallelRangeQueries(timeRange.Start, timeRange.End, step, rangeTasks)
	}

	return nodes, nil
}

func (c *ClusterOperator) getDetailedNodeListOptimized(timeRange *types.TimeRange) ([]types.NodeBriefStatus, error) {
	window := c.calculateRateWindow(timeRange)

	// 1. 获取所有节点基本信息
	nodeQuery := `kube_node_info`
	nodeResults, err := c.query(nodeQuery, nil)
	if err != nil {
		return nil, fmt.Errorf("查询节点信息失败: %w", err)
	}

	if len(nodeResults) == 0 {
		return []types.NodeBriefStatus{}, nil
	}

	// 创建节点映射
	nodeMap := make(map[string]*types.NodeBriefStatus)
	for _, result := range nodeResults {
		if nodeName, ok := result.Metric["node"]; ok {
			nodeMap[nodeName] = &types.NodeBriefStatus{
				NodeName: nodeName,
			}
		}
	}

	// 2. 并发批量查询所有节点指标
	var wg sync.WaitGroup
	wg.Add(5)

	go func() {
		defer wg.Done()
		readyQuery := `kube_node_status_condition{condition="Ready",status="true"}`
		if readyResults, err := c.query(readyQuery, nil); err == nil {
			for _, r := range readyResults {
				if nodeName, ok := r.Metric["node"]; ok {
					if node, exists := nodeMap[nodeName]; exists {
						node.Ready = r.Value > 0
					}
				}
			}
		}
	}()

	go func() {
		defer wg.Done()
		memPressureQuery := `kube_node_status_condition{condition="MemoryPressure",status="true"}`
		if memPressureResults, err := c.query(memPressureQuery, nil); err == nil {
			for _, r := range memPressureResults {
				if nodeName, ok := r.Metric["node"]; ok {
					if node, exists := nodeMap[nodeName]; exists {
						node.MemoryPressure = r.Value > 0
					}
				}
			}
		}
	}()

	go func() {
		defer wg.Done()
		diskPressureQuery := `kube_node_status_condition{condition="DiskPressure",status="true"}`
		if diskPressureResults, err := c.query(diskPressureQuery, nil); err == nil {
			for _, r := range diskPressureResults {
				if nodeName, ok := r.Metric["node"]; ok {
					if node, exists := nodeMap[nodeName]; exists {
						node.DiskPressure = r.Value > 0
					}
				}
			}
		}
	}()

	go func() {
		defer wg.Done()
		cpuQuery := fmt.Sprintf(`(1 - avg by (instance) (rate(node_cpu_seconds_total{mode="idle"}[%s]))) * 100`, window)
		if cpuResults, err := c.query(cpuQuery, nil); err == nil {
			for _, r := range cpuResults {
				if instance, ok := r.Metric["instance"]; ok {
					instanceParts := strings.Split(instance, ":")
					instanceHost := instanceParts[0]
					for nodeName, node := range nodeMap {
						if strings.Contains(instance, nodeName) || strings.Contains(instanceHost, nodeName) || strings.HasPrefix(nodeName, instanceHost) {
							node.CPUUsage = r.Value
							break
						}
					}
				}
			}
		}
	}()

	go func() {
		defer wg.Done()
		memQuery := `((node_memory_MemTotal_bytes - node_memory_MemAvailable_bytes) / node_memory_MemTotal_bytes) * 100`
		if memResults, err := c.query(memQuery, nil); err == nil {
			for _, r := range memResults {
				if instance, ok := r.Metric["instance"]; ok {
					instanceParts := strings.Split(instance, ":")
					instanceHost := instanceParts[0]
					for nodeName, node := range nodeMap {
						if strings.Contains(instance, nodeName) || strings.Contains(instanceHost, nodeName) || strings.HasPrefix(nodeName, instanceHost) {
							node.MemoryUsage = r.Value
							break
						}
					}
				}
			}
		}
	}()

	wg.Wait()

	// 批量查询 Pod 数量
	podQuery := `count by (node) (kube_pod_info)`
	if podResults, err := c.query(podQuery, nil); err == nil {
		for _, r := range podResults {
			if nodeName, ok := r.Metric["node"]; ok {
				if node, exists := nodeMap[nodeName]; exists {
					node.PodsCount = int64(r.Value)
				}
			}
		}
	}

	// 转换为列表
	nodeList := make([]types.NodeBriefStatus, 0, len(nodeMap))
	for _, node := range nodeMap {
		nodeList = append(nodeList, *node)
	}

	return nodeList, nil
}

func (c *ClusterOperator) getControlPlaneMetrics(timeRange *types.TimeRange) (*types.ClusterControlPlaneMetrics, error) {
	controlPlane := &types.ClusterControlPlaneMetrics{}

	var wg sync.WaitGroup
	wg.Add(4)

	go func() {
		defer wg.Done()
		if apiServer, err := c.GetAPIServerMetrics(timeRange); err == nil {
			controlPlane.APIServer = *apiServer
		}
	}()

	go func() {
		defer wg.Done()
		if etcd, err := c.GetEtcdMetrics(timeRange); err == nil {
			controlPlane.Etcd = *etcd
		}
	}()

	go func() {
		defer wg.Done()
		if scheduler, err := c.GetSchedulerMetrics(timeRange); err == nil {
			controlPlane.Scheduler = *scheduler
		}
	}()

	go func() {
		defer wg.Done()
		if cm, err := c.GetControllerManagerMetrics(timeRange); err == nil {
			controlPlane.ControllerManager = *cm
		}
	}()

	wg.Wait()
	return controlPlane, nil
}

// GetAPIServerMetrics 获取 API Server 指标
func (c *ClusterOperator) GetAPIServerMetrics(timeRange *types.TimeRange) (*types.APIServerMetrics, error) {
	apiServer := &types.APIServerMetrics{
		RequestsByVerb: []types.VerbMetrics{},
		RequestsByCode: types.StatusCodeDistribution{
			Status2xx: make(map[string]float64),
			Status3xx: make(map[string]float64),
			Status4xx: make(map[string]float64),
			Status5xx: make(map[string]float64),
		},
		RequestsByResource: []types.ResourceMetrics{},
		Trend:              []types.APIServerDataPoint{},
	}

	window := c.calculateRateWindow(timeRange)
	c.log.Infof("开始查询 API Server 指标 (window: %s)", window)

	// 基础指标和延迟查询任务
	tasks := []clusterQueryTask{
		{
			name:  "qps",
			query: fmt.Sprintf(`sum(rate(apiserver_request_total[%s]))`, window),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					apiServer.RequestsPerSecond = results[0].Value
					c.log.Infof("API Server QPS: %.2f req/s", apiServer.RequestsPerSecond)
				}
				return nil
			},
		},
		{
			name:  "error_rate",
			query: fmt.Sprintf(`(sum(rate(apiserver_request_total{code=~"5.."}[%s])) or vector(0)) / (sum(rate(apiserver_request_total[%s])) or vector(1))`, window, window),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					apiServer.ErrorRate = results[0].Value
					c.log.Infof("API Server Error Rate: %.2f%%", apiServer.ErrorRate*100)
				}
				return nil
			},
		},
		{
			name:  "p50",
			query: fmt.Sprintf(`histogram_quantile(0.50, sum(rate(apiserver_request_duration_seconds_bucket[%s])) by (le))`, window),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					apiServer.P50Latency = results[0].Value
					c.log.Infof("API Server P50 Latency: %.4f s", apiServer.P50Latency)
				}
				return nil
			},
		},
		{
			name:  "p95",
			query: fmt.Sprintf(`histogram_quantile(0.95, sum(rate(apiserver_request_duration_seconds_bucket[%s])) by (le))`, window),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					apiServer.P95Latency = results[0].Value
					c.log.Infof("API Server P95 Latency: %.4f s", apiServer.P95Latency)
				}
				return nil
			},
		},
		{
			name:  "p99",
			query: fmt.Sprintf(`histogram_quantile(0.99, sum(rate(apiserver_request_duration_seconds_bucket[%s])) by (le))`, window),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					apiServer.P99Latency = results[0].Value
					c.log.Infof("API Server P99 Latency: %.4f s", apiServer.P99Latency)
				}
				return nil
			},
		},

		// 并发指标
		{
			name:  "inflight_requests",
			query: `sum(apiserver_current_inflight_requests)`,
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					apiServer.CurrentInflightRequests = int64(results[0].Value)
					c.log.Infof("Current Inflight Requests: %d", apiServer.CurrentInflightRequests)
				}
				return nil
			},
		},
		{
			name:  "inflight_read",
			query: `sum(apiserver_current_inflight_requests{request_kind="readOnly"})`,
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					apiServer.InflightReadRequests = int64(results[0].Value)
				}
				return nil
			},
		},
		{
			name:  "inflight_mutating",
			query: `sum(apiserver_current_inflight_requests{request_kind="mutating"})`,
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					apiServer.InflightMutatingRequests = int64(results[0].Value)
				}
				return nil
			},
		},
		{
			name:  "longrunning",
			query: `sum(apiserver_longrunning_requests)`,
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					apiServer.LongRunningRequests = int64(results[0].Value)
					c.log.Infof("Long Running Requests: %d", apiServer.LongRunningRequests)
				}
				return nil
			},
		},
		// Watch 连接数通过长连接请求中 verb 为 WATCH 的统计
		{
			name:  "watch_count",
			query: `sum(apiserver_longrunning_requests{verb="WATCH"})`,
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					apiServer.WatchCount = int64(results[0].Value)
					c.log.Infof("Watch Count: %d", apiServer.WatchCount)
				}
				return nil
			},
		},

		// 性能指标：请求终止统计
		{
			name:  "request_dropped",
			query: fmt.Sprintf(`sum(increase(apiserver_request_terminations_total[%s]))`, window),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					apiServer.RequestDropped = int64(results[0].Value)
				}
				return nil
			},
		},
		// 请求超时统计：使用状态码 504 或终止原因为 timeout 的请求
		{
			name:  "request_timeout",
			query: fmt.Sprintf(`sum(increase(apiserver_request_total{code="504"}[%s])) or sum(increase(apiserver_request_terminations_total{reason="timeout"}[%s])) or vector(0)`, window, window),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					apiServer.RequestTimeout = int64(results[0].Value)
				}
				return nil
			},
		},
		{
			name:  "response_size",
			query: fmt.Sprintf(`sum(rate(apiserver_response_sizes_sum[%s])) / sum(rate(apiserver_response_sizes_count[%s]))`, window, window),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					apiServer.ResponseSizeBytes = results[0].Value
				}
				return nil
			},
		},
		// Webhook 延迟：优先使用 admission webhook 指标，备选使用 admission controller 指标
		{
			name:  "webhook_duration",
			query: fmt.Sprintf(`histogram_quantile(0.99, sum(rate(apiserver_admission_webhook_admission_duration_seconds_bucket[%s])) by (le)) or histogram_quantile(0.99, sum(rate(apiserver_admission_controller_admission_duration_seconds_bucket[%s])) by (le))`, window, window),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					apiServer.WebhookDurationSeconds = results[0].Value
					c.log.Infof("Webhook Duration P99: %.4f s", apiServer.WebhookDurationSeconds)
				}
				return nil
			},
		},

		// 认证和鉴权
		{
			name:  "auth_attempts",
			query: fmt.Sprintf(`sum(rate(authentication_attempts[%s]))`, window),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					apiServer.AuthenticationAttempts = results[0].Value
				}
				return nil
			},
		},
		{
			name:  "auth_failures",
			query: fmt.Sprintf(`sum(rate(authentication_attempts{result="failure"}[%s]))`, window),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					apiServer.AuthenticationFailures = results[0].Value
				}
				return nil
			},
		},
		{
			name:  "authz_attempts",
			query: fmt.Sprintf(`sum(rate(apiserver_authorization_decisions_total[%s]))`, window),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					apiServer.AuthorizationAttempts = results[0].Value
				}
				return nil
			},
		},
		// 鉴权延迟：该指标在新版本可能不存在，返回空值表示无数据
		{
			name:  "authz_duration",
			query: fmt.Sprintf(`histogram_quantile(0.99, sum(rate(apiserver_authorization_duration_seconds_bucket[%s])) by (le))`, window),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					apiServer.AuthorizationDuration = results[0].Value
				}
				return nil
			},
		},
		// 客户端证书过期检测：使用 histogram 的低百分位获取最小剩余有效期
		// apiserver_client_certificate_expiration_seconds 是 histogram 类型
		// 使用 1% 分位值近似最小过期时间，转换为天数
		{
			name:  "client_cert_expiration",
			query: fmt.Sprintf(`histogram_quantile(0.01, sum(rate(apiserver_client_certificate_expiration_seconds_bucket[%s])) by (le)) / 86400`, window),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 && results[0].Value > 0 {
					apiServer.ClientCertExpirationDays = results[0].Value
					c.log.Infof("Client Cert Expiration Days (min): %.2f", apiServer.ClientCertExpirationDays)
				}
				return nil
			},
		},

		// 分类统计
		{
			name:  "verb",
			query: fmt.Sprintf(`sum(rate(apiserver_request_total[%s])) by (verb)`, window),
			f: func(results []types.InstantQueryResult) error {
				for _, r := range results {
					if verb, ok := r.Metric["verb"]; ok {
						apiServer.RequestsByVerb = append(apiServer.RequestsByVerb, types.VerbMetrics{
							Verb:              verb,
							RequestsPerSecond: r.Value,
						})
					}
				}
				return nil
			},
		},
		{
			name:  "code",
			query: fmt.Sprintf(`sum(rate(apiserver_request_total[%s])) by (code)`, window),
			f: func(results []types.InstantQueryResult) error {
				for _, r := range results {
					if code, ok := r.Metric["code"]; ok {
						c.addStatusCode(&apiServer.RequestsByCode, code, r.Value)
					}
				}
				return nil
			},
		},
		{
			name:  "resource",
			query: fmt.Sprintf(`topk(10, sum(rate(apiserver_request_total[%s])) by (resource))`, window),
			f: func(results []types.InstantQueryResult) error {
				for _, r := range results {
					if resource, ok := r.Metric["resource"]; ok {
						apiServer.RequestsByResource = append(apiServer.RequestsByResource, types.ResourceMetrics{
							Resource:          resource,
							RequestsPerSecond: r.Value,
						})
					}
				}
				return nil
			},
		},
	}

	c.executeParallelQueries(tasks)

	// 趋势数据
	if timeRange != nil && !timeRange.Start.IsZero() && !timeRange.End.IsZero() {
		step := timeRange.Step
		if step == "" {
			step = c.calculateStep(timeRange.Start, timeRange.End)
		}

		c.log.Infof("查询 API Server 趋势数据: start=%s, end=%s, step=%s",
			timeRange.Start.Format("2006-01-02 15:04:05"),
			timeRange.End.Format("2006-01-02 15:04:05"),
			step)

		var mu sync.Mutex
		rangeTasks := []clusterRangeTask{
			{
				name:  "qps_trend",
				query: fmt.Sprintf(`sum(rate(apiserver_request_total[%s]))`, window),
				f: func(results []types.RangeQueryResult) error {
					if len(results) > 0 && len(results[0].Values) > 0 {
						mu.Lock()
						defer mu.Unlock()
						apiServer.Trend = make([]types.APIServerDataPoint, len(results[0].Values))
						for i, v := range results[0].Values {
							apiServer.Trend[i] = types.APIServerDataPoint{
								Timestamp:         v.Timestamp,
								RequestsPerSecond: v.Value,
							}
						}
					}
					return nil
				},
			},
			{
				name:  "error_trend",
				query: fmt.Sprintf(`(sum(rate(apiserver_request_total{code=~"5.."}[%s])) or vector(0)) / (sum(rate(apiserver_request_total[%s])) or vector(1))`, window, window),
				f: func(results []types.RangeQueryResult) error {
					if len(results) > 0 && len(results[0].Values) > 0 {
						mu.Lock()
						defer mu.Unlock()
						for i, v := range results[0].Values {
							if i < len(apiServer.Trend) {
								apiServer.Trend[i].ErrorRate = v.Value
							}
						}
					}
					return nil
				},
			},
			{
				name:  "p95_trend",
				query: fmt.Sprintf(`histogram_quantile(0.95, sum(rate(apiserver_request_duration_seconds_bucket[%s])) by (le))`, window),
				f: func(results []types.RangeQueryResult) error {
					if len(results) > 0 && len(results[0].Values) > 0 {
						mu.Lock()
						defer mu.Unlock()
						for i, v := range results[0].Values {
							if i < len(apiServer.Trend) {
								apiServer.Trend[i].P95Latency = v.Value
							}
						}
					}
					return nil
				},
			},
			{
				name:  "inflight_trend",
				query: `sum(apiserver_current_inflight_requests)`,
				f: func(results []types.RangeQueryResult) error {
					if len(results) > 0 && len(results[0].Values) > 0 {
						mu.Lock()
						defer mu.Unlock()
						for i, v := range results[0].Values {
							if i < len(apiServer.Trend) {
								apiServer.Trend[i].CurrentInflightRequests = int64(v.Value)
							}
						}
					}
					return nil
				},
			},
			{
				name:  "longrunning_trend",
				query: `sum(apiserver_longrunning_requests)`,
				f: func(results []types.RangeQueryResult) error {
					if len(results) > 0 && len(results[0].Values) > 0 {
						mu.Lock()
						defer mu.Unlock()
						for i, v := range results[0].Values {
							if i < len(apiServer.Trend) {
								apiServer.Trend[i].LongRunningRequests = int64(v.Value)
							}
						}
					}
					return nil
				},
			},
		}

		c.executeParallelRangeQueries(timeRange.Start, timeRange.End, step, rangeTasks)
		c.log.Infof("API Server 趋势数据查询完成，共 %d 个数据点", len(apiServer.Trend))
	}

	return apiServer, nil
}

// GetSchedulerMetrics 获取 Scheduler 指标
func (c *ClusterOperator) GetSchedulerMetrics(timeRange *types.TimeRange) (*types.SchedulerMetrics, error) {
	scheduler := &types.SchedulerMetrics{
		FailureReasons: types.ScheduleFailureReasons{},
		PluginLatency:  types.SchedulerPluginLatency{},
		Trend:          []types.SchedulerDataPoint{},
	}
	window := c.calculateRateWindow(timeRange)

	c.log.Infof("开始查询 Scheduler 指标 (window: %s)", window)

	// 基础指标查询任务
	tasks := []clusterQueryTask{
		// 调度尝试次数：使用 scheduler_pod_scheduling_attempts_count
		{
			name:  "schedule_attempts",
			query: fmt.Sprintf(`sum(rate(scheduler_pod_scheduling_attempts_count[%s]))`, window),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					scheduler.ScheduleAttempts = results[0].Value
					c.log.Infof("Schedule Attempts: %.2f/s", scheduler.ScheduleAttempts)
				}
				return nil
			},
		},
		// 调度成功率：通过 Pending Pod 与总 Pod 比例推算
		{
			name:  "success_rate",
			query: `1 - (sum(kube_pod_status_phase{phase="Pending"}) / (sum(kube_pod_status_phase{phase=~"Running|Pending"}) or vector(1)))`,
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					scheduler.ScheduleSuccessRate = results[0].Value
					c.log.Infof("Schedule Success Rate: %.2f%%", scheduler.ScheduleSuccessRate*100)
				} else {
					scheduler.ScheduleSuccessRate = 1.0
				}
				return nil
			},
		},
		{
			name:  "pending_pods",
			query: `sum(kube_pod_status_phase{phase="Pending"})`,
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					scheduler.PendingPods = int64(results[0].Value)
					c.log.Infof("Pending Pods: %d", scheduler.PendingPods)
				}
				return nil
			},
		},
		// 不可调度 Pod 数：使用 PodScheduled 条件为 false 的统计
		{
			name:  "unschedulable_pods",
			query: `sum(kube_pod_status_condition{condition="PodScheduled",status="false"})`,
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					scheduler.UnschedulablePods = int64(results[0].Value)
					c.log.Infof("Unschedulable Pods: %d", scheduler.UnschedulablePods)
				}
				return nil
			},
		},

		// 延迟指标：使用 scheduler_scheduling_algorithm_duration_seconds
		{
			name:  "p50",
			query: fmt.Sprintf(`histogram_quantile(0.50, sum(rate(scheduler_scheduling_algorithm_duration_seconds_bucket[%s])) by (le))`, window),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					scheduler.P50ScheduleLatency = results[0].Value
					c.log.Infof("Schedule P50 Latency: %.4f s", scheduler.P50ScheduleLatency)
				}
				return nil
			},
		},
		{
			name:  "p95",
			query: fmt.Sprintf(`histogram_quantile(0.95, sum(rate(scheduler_scheduling_algorithm_duration_seconds_bucket[%s])) by (le))`, window),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					scheduler.P95ScheduleLatency = results[0].Value
					c.log.Infof("Schedule P95 Latency: %.4f s", scheduler.P95ScheduleLatency)
				}
				return nil
			},
		},
		{
			name:  "p99",
			query: fmt.Sprintf(`histogram_quantile(0.99, sum(rate(scheduler_scheduling_algorithm_duration_seconds_bucket[%s])) by (le))`, window),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					scheduler.P99ScheduleLatency = results[0].Value
					c.log.Infof("Schedule P99 Latency: %.4f s", scheduler.P99ScheduleLatency)
				}
				return nil
			},
		},
		// 绑定延迟：使用 framework 扩展点的 Bind 阶段
		{
			name:  "binding_latency",
			query: fmt.Sprintf(`histogram_quantile(0.99, sum(rate(scheduler_framework_extension_point_duration_seconds_bucket{extension_point="Bind"}[%s])) by (le))`, window),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					scheduler.BindingLatency = results[0].Value
					c.log.Infof("Binding Latency P99: %.4f s", scheduler.BindingLatency)
				}
				return nil
			},
		},

		// 调度结果统计
		{
			name:  "scheduled",
			query: fmt.Sprintf(`sum(increase(scheduler_pod_scheduling_attempts_count[%s]))`, window),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					scheduler.ScheduledPods = int64(results[0].Value)
				}
				return nil
			},
		},
		// 调度失败数：通过 Pod 调度尝试的 histogram 统计
		{
			name:  "failed",
			query: fmt.Sprintf(`sum(increase(scheduler_pod_scheduling_attempts_sum[%s])) - sum(increase(scheduler_pod_scheduling_attempts_count[%s]))`, window, window),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 && results[0].Value > 0 {
					scheduler.FailedScheduling = int64(results[0].Value)
					c.log.Infof("Failed Scheduling: %d", scheduler.FailedScheduling)
				}
				return nil
			},
		},
		{
			name:  "preemption_attempts",
			query: fmt.Sprintf(`sum(increase(scheduler_preemption_attempts_total[%s]))`, window),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					scheduler.PreemptionAttempts = int64(results[0].Value)
					c.log.Infof("Preemption Attempts: %d", scheduler.PreemptionAttempts)
				}
				return nil
			},
		},
		// 抢占牺牲者数：scheduler_preemption_victims 是 histogram，使用 sum 获取总数
		{
			name:  "preemption_victims",
			query: fmt.Sprintf(`sum(increase(scheduler_preemption_victims_sum[%s]))`, window),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					scheduler.PreemptionVictims = int64(results[0].Value)
				}
				return nil
			},
		},

		// 调度队列
		{
			name:  "queue_length",
			query: `sum(scheduler_pending_pods)`,
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					scheduler.SchedulingQueueLength = int64(results[0].Value)
					c.log.Infof("Scheduling Queue Length: %d", scheduler.SchedulingQueueLength)
				}
				return nil
			},
		},
		{
			name:  "active_queue",
			query: `sum(scheduler_pending_pods{queue="active"})`,
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					scheduler.ActiveQueueLength = int64(results[0].Value)
				}
				return nil
			},
		},
		{
			name:  "backoff_queue",
			query: `sum(scheduler_pending_pods{queue="backoff"})`,
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					scheduler.BackoffQueueLength = int64(results[0].Value)
				}
				return nil
			},
		},

		// Framework 插件延迟
		{
			name:  "filter_latency",
			query: fmt.Sprintf(`histogram_quantile(0.99, sum(rate(scheduler_framework_extension_point_duration_seconds_bucket{extension_point="Filter"}[%s])) by (le))`, window),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					scheduler.PluginLatency.FilterLatency = results[0].Value
					c.log.Infof("Filter Plugin Latency P99: %.4f s", scheduler.PluginLatency.FilterLatency)
				}
				return nil
			},
		},
		{
			name:  "score_latency",
			query: fmt.Sprintf(`histogram_quantile(0.99, sum(rate(scheduler_framework_extension_point_duration_seconds_bucket{extension_point="Score"}[%s])) by (le))`, window),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					scheduler.PluginLatency.ScoreLatency = results[0].Value
					c.log.Infof("Score Plugin Latency P99: %.4f s", scheduler.PluginLatency.ScoreLatency)
				}
				return nil
			},
		},
		{
			name:  "prebind_latency",
			query: fmt.Sprintf(`histogram_quantile(0.99, sum(rate(scheduler_framework_extension_point_duration_seconds_bucket{extension_point="PreBind"}[%s])) by (le))`, window),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					scheduler.PluginLatency.PreBindLatency = results[0].Value
				}
				return nil
			},
		},
		{
			name:  "bind_latency_plugin",
			query: fmt.Sprintf(`histogram_quantile(0.99, sum(rate(scheduler_framework_extension_point_duration_seconds_bucket{extension_point="Bind"}[%s])) by (le))`, window),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					scheduler.PluginLatency.BindLatency = results[0].Value
				}
				return nil
			},
		},
	}

	c.executeParallelQueries(tasks)

	// 调度失败原因分类
	c.getScheduleFailureReasons(scheduler, window)

	// 趋势数据
	if timeRange != nil && !timeRange.Start.IsZero() && !timeRange.End.IsZero() {
		step := timeRange.Step
		if step == "" {
			step = c.calculateStep(timeRange.Start, timeRange.End)
		}

		c.log.Infof("查询 Scheduler 趋势数据: start=%s, end=%s, step=%s",
			timeRange.Start.Format("2006-01-02 15:04:05"),
			timeRange.End.Format("2006-01-02 15:04:05"),
			step)

		var mu sync.Mutex

		rangeTasks := []clusterRangeTask{
			{
				name:  "pending_pods_trend",
				query: `sum(kube_pod_status_phase{phase="Pending"})`,
				f: func(results []types.RangeQueryResult) error {
					if len(results) > 0 && len(results[0].Values) > 0 {
						mu.Lock()
						defer mu.Unlock()
						scheduler.Trend = make([]types.SchedulerDataPoint, len(results[0].Values))
						for i, v := range results[0].Values {
							scheduler.Trend[i] = types.SchedulerDataPoint{
								Timestamp:   v.Timestamp,
								PendingPods: int64(v.Value),
							}
						}
					}
					return nil
				},
			},
			{
				name:  "success_rate_trend",
				query: `1 - (sum(kube_pod_status_phase{phase="Pending"}) / (sum(kube_pod_status_phase{phase=~"Running|Pending"}) or vector(1)))`,
				f: func(results []types.RangeQueryResult) error {
					if len(results) > 0 && len(results[0].Values) > 0 {
						mu.Lock()
						defer mu.Unlock()
						for i, v := range results[0].Values {
							if i < len(scheduler.Trend) {
								scheduler.Trend[i].ScheduleSuccessRate = v.Value
							}
						}
					}
					return nil
				},
			},
			{
				name:  "p95_latency_trend",
				query: fmt.Sprintf(`histogram_quantile(0.95, sum(rate(scheduler_scheduling_algorithm_duration_seconds_bucket[%s])) by (le))`, window),
				f: func(results []types.RangeQueryResult) error {
					if len(results) > 0 && len(results[0].Values) > 0 {
						mu.Lock()
						defer mu.Unlock()
						for i, v := range results[0].Values {
							if i < len(scheduler.Trend) {
								scheduler.Trend[i].P95ScheduleLatency = v.Value
							}
						}
					}
					return nil
				},
			},
			{
				name:  "attempts_trend",
				query: fmt.Sprintf(`sum(rate(scheduler_pod_scheduling_attempts_count[%s]))`, window),
				f: func(results []types.RangeQueryResult) error {
					if len(results) > 0 && len(results[0].Values) > 0 {
						mu.Lock()
						defer mu.Unlock()
						for i, v := range results[0].Values {
							if i < len(scheduler.Trend) {
								scheduler.Trend[i].ScheduleAttempts = v.Value
							}
						}
					}
					return nil
				},
			},
		}

		c.executeParallelRangeQueries(timeRange.Start, timeRange.End, step, rangeTasks)
		c.log.Infof("Scheduler 趋势数据查询完成，共 %d 个数据点", len(scheduler.Trend))
	}

	return scheduler, nil
}

// getScheduleFailureReasons 获取调度失败原因分类
func (c *ClusterOperator) getScheduleFailureReasons(scheduler *types.SchedulerMetrics, window string) {
	c.log.Infof("查询调度失败原因分类")

	// 通过 scheduler_plugin_evaluation_total 统计各插件的失败情况
	// 不同 Kubernetes 版本的指标格式可能有差异
	tasks := []clusterQueryTask{
		// 资源不足相关：通过 plugin 名称匹配
		{
			name:  "insufficient_cpu",
			query: fmt.Sprintf(`sum(increase(scheduler_plugin_evaluation_total{plugin="NodeResourcesFit",status!="success"}[%s])) or vector(0)`, window),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 && results[0].Value > 0 {
					scheduler.FailureReasons.InsufficientCPU = int64(results[0].Value)
					c.log.Infof("Failure Reason [InsufficientResources]: %d", scheduler.FailureReasons.InsufficientCPU)
				}
				return nil
			},
		},
		// 节点亲和性
		{
			name:  "node_affinity",
			query: fmt.Sprintf(`sum(increase(scheduler_plugin_evaluation_total{plugin="NodeAffinity",status!="success"}[%s])) or vector(0)`, window),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 && results[0].Value > 0 {
					scheduler.FailureReasons.NodeAffinity = int64(results[0].Value)
					c.log.Infof("Failure Reason [NodeAffinity]: %d", scheduler.FailureReasons.NodeAffinity)
				}
				return nil
			},
		},
		// Pod 亲和性
		{
			name:  "pod_affinity",
			query: fmt.Sprintf(`sum(increase(scheduler_plugin_evaluation_total{plugin=~"InterPodAffinity|PodTopologySpread",status!="success"}[%s])) or vector(0)`, window),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 && results[0].Value > 0 {
					scheduler.FailureReasons.PodAffinity = int64(results[0].Value)
					c.log.Infof("Failure Reason [PodAffinity]: %d", scheduler.FailureReasons.PodAffinity)
				}
				return nil
			},
		},
		// 污点
		{
			name:  "taint",
			query: fmt.Sprintf(`sum(increase(scheduler_plugin_evaluation_total{plugin="TaintToleration",status!="success"}[%s])) or vector(0)`, window),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 && results[0].Value > 0 {
					scheduler.FailureReasons.Taint = int64(results[0].Value)
					c.log.Infof("Failure Reason [Taint]: %d", scheduler.FailureReasons.Taint)
				}
				return nil
			},
		},
		// 卷绑定
		{
			name:  "volume_binding",
			query: fmt.Sprintf(`sum(increase(scheduler_plugin_evaluation_total{plugin="VolumeBinding",status!="success"}[%s])) or vector(0)`, window),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 && results[0].Value > 0 {
					scheduler.FailureReasons.VolumeBinding = int64(results[0].Value)
					c.log.Infof("Failure Reason [VolumeBinding]: %d", scheduler.FailureReasons.VolumeBinding)
				}
				return nil
			},
		},
		// 无可用节点：使用不可调度 Pod 数量作为近似值
		{
			name:  "no_nodes_available",
			query: `sum(kube_pod_status_condition{condition="PodScheduled",status="false"}) or vector(0)`,
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 && results[0].Value > 0 {
					scheduler.FailureReasons.NoNodesAvailable = int64(results[0].Value)
					c.log.Infof("Failure Reason [NoNodesAvailable]: %d", scheduler.FailureReasons.NoNodesAvailable)
				}
				return nil
			},
		},
	}

	c.executeParallelQueries(tasks)
}

func (c *ClusterOperator) GetControllerManagerMetrics(timeRange *types.TimeRange) (*types.ControllerManagerMetrics, error) {
	cm := &types.ControllerManagerMetrics{
		QueueLatency:     types.QueueLatencyMetrics{},
		WorkQueueMetrics: types.ControllerWorkQueueMetrics{},
		ReconcileLatency: types.ControllerReconcileLatency{},
		Trend:            []types.ControllerManagerDataPoint{},
	}
	window := c.calculateRateWindow(timeRange)

	c.log.Infof("开始查询 Controller Manager 指标 (window: %s)", window)

	// 先查询所有可用的workqueue名称
	allWorkqueueQuery := `workqueue_depth`
	allWorkqueues, err := c.query(allWorkqueueQuery, nil)
	if err != nil {
		c.log.Errorf("无法查询 workqueue_depth: %v", err)
	}

	// 提取所有可用的 name 标签
	availableNames := make(map[string]float64)
	if len(allWorkqueues) > 0 {
		for _, result := range allWorkqueues {
			if name, ok := result.Metric["name"]; ok {
				availableNames[name] += result.Value
			}
		}
		c.log.Infof("发现的 workqueue 名称:")
		for name, depth := range availableNames {
			c.log.Infof("  - %s: %.0f", name, depth)
		}
	}

	// ==================== Leader 选举 ====================
	tasks := []clusterQueryTask{
		{
			name:  "is_leader",
			query: `max(leader_election_master_status{name="kube-controller-manager"})`,
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					cm.IsLeader = results[0].Value > 0
					c.log.Infof("Controller Manager Is Leader: %v", cm.IsLeader)
				}
				return nil
			},
		},
		{
			name:  "leader_changes",
			query: `sum(increase(leader_election_master_status{name="kube-controller-manager"}[1h]))`,
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					cm.LeaderChanges = int64(results[0].Value)
					if cm.LeaderChanges > 0 {
						c.log.Infof("  Leader Changes (1h): %d", cm.LeaderChanges)
					}
				}
				return nil
			},
		},

		// ==================== 工作队列深度（直接使用实际名称）====================
		{
			name:  "deployment_queue",
			query: c.findWorkqueueQuery(availableNames, []string{"deployment"}),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					cm.DeploymentQueueDepth = int64(results[0].Value)
					c.log.Infof("Deployment Queue Depth: %d", cm.DeploymentQueueDepth)
				}
				return nil
			},
		},
		{
			name:  "replicaset_queue",
			query: c.findWorkqueueQuery(availableNames, []string{"replicaset"}),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					cm.ReplicaSetQueueDepth = int64(results[0].Value)
					c.log.Infof("ReplicaSet Queue Depth: %d", cm.ReplicaSetQueueDepth)
				}
				return nil
			},
		},
		{
			name:  "statefulset_queue",
			query: c.findWorkqueueQuery(availableNames, []string{"statefulset"}),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					cm.StatefulSetQueueDepth = int64(results[0].Value)
					c.log.Infof("StatefulSet Queue Depth: %d", cm.StatefulSetQueueDepth)
				}
				return nil
			},
		},
		{
			name:  "daemonset_queue",
			query: c.findWorkqueueQuery(availableNames, []string{"daemonset"}),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					cm.DaemonSetQueueDepth = int64(results[0].Value)
					c.log.Infof("DaemonSet Queue Depth: %d", cm.DaemonSetQueueDepth)
				}
				return nil
			},
		},
		{
			name:  "job_queue",
			query: c.findWorkqueueQuery(availableNames, []string{"job"}),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					cm.JobQueueDepth = int64(results[0].Value)
					c.log.Infof("Job Queue Depth: %d", cm.JobQueueDepth)
				}
				return nil
			},
		},
		{
			name:  "node_queue",
			query: c.findWorkqueueQuery(availableNames, []string{"node", "nodelifecycle"}),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					cm.NodeQueueDepth = int64(results[0].Value)
					c.log.Infof("Node Queue Depth: %d", cm.NodeQueueDepth)
				}
				return nil
			},
		},
		{
			name:  "service_queue",
			query: c.findWorkqueueQuery(availableNames, []string{"service"}),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					cm.ServiceQueueDepth = int64(results[0].Value)
					c.log.Infof("Service Queue Depth: %d", cm.ServiceQueueDepth)
				}
				return nil
			},
		},
		{
			name:  "endpoint_queue",
			query: c.findWorkqueueQuery(availableNames, []string{"endpoint", "endpointslice"}),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					cm.EndpointQueueDepth = int64(results[0].Value)
					c.log.Infof("Endpoint Queue Depth: %d", cm.EndpointQueueDepth)
				}
				return nil
			},
		},

		// ==================== 队列延迟 ====================
		{
			name:  "queue_duration",
			query: fmt.Sprintf(`histogram_quantile(0.99, sum(rate(workqueue_queue_duration_seconds_bucket[%s])) by (le))`, window),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					cm.QueueLatency.QueueDuration = results[0].Value
					c.log.Infof("Queue Duration P99: %.4f s", cm.QueueLatency.QueueDuration)
				}
				return nil
			},
		},
		{
			name:  "work_duration",
			query: fmt.Sprintf(`histogram_quantile(0.99, sum(rate(workqueue_work_duration_seconds_bucket[%s])) by (le))`, window),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					cm.QueueLatency.WorkDuration = results[0].Value
					c.log.Infof("Work Duration P99: %.4f s", cm.QueueLatency.WorkDuration)
				}
				return nil
			},
		},

		// ==================== 工作队列统计 ====================
		{
			name:  "adds_rate",
			query: fmt.Sprintf(`sum(rate(workqueue_adds_total[%s]))`, window),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					cm.WorkQueueMetrics.AddsRate = results[0].Value
					c.log.Infof("Workqueue Adds Rate: %.2f items/s", cm.WorkQueueMetrics.AddsRate)
				}
				return nil
			},
		},
		{
			name:  "depth_total",
			query: `sum(workqueue_depth)`,
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					cm.WorkQueueMetrics.DepthTotal = int64(results[0].Value)
					c.log.Infof("Total Queue Depth: %d", cm.WorkQueueMetrics.DepthTotal)
				}
				return nil
			},
		},
		{
			name:  "unfinished_work",
			query: `sum(workqueue_unfinished_work_seconds)`,
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					cm.WorkQueueMetrics.UnfinishedWork = int64(results[0].Value)
				}
				return nil
			},
		},
		{
			name:  "longest_running",
			query: `max(workqueue_longest_running_processor_seconds)`,
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					cm.WorkQueueMetrics.LongestRunning = results[0].Value
					if cm.WorkQueueMetrics.LongestRunning > 60 {
						c.log.Errorf("  Longest Running Work: %.2f s", cm.WorkQueueMetrics.LongestRunning)
					}
				}
				return nil
			},
		},
		{
			name:  "retries_rate",
			query: fmt.Sprintf(`sum(rate(workqueue_retries_total[%s]))`, window),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					cm.WorkQueueMetrics.RetriesRate = results[0].Value
					cm.RetryRate = results[0].Value
					c.log.Infof("Retry Rate: %.2f items/s", cm.RetryRate)
				}
				return nil
			},
		},

		// ==================== 错误和重试 ====================
		{
			name:  "sync_errors",
			query: fmt.Sprintf(`sum(increase(controller_runtime_reconcile_errors_total[%s]))`, window),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					cm.TotalSyncErrors = int64(results[0].Value)
					if cm.TotalSyncErrors > 0 {
						c.log.Infof("Total Sync Errors: %d", cm.TotalSyncErrors)
					}
				}
				return nil
			},
		},
	}

	c.executeParallelQueries(tasks)

	// ==================== 控制器协调延迟 ====================
	c.getReconcileLatency(cm, window, availableNames)

	// ==================== 趋势数据 ====================
	if timeRange != nil && !timeRange.Start.IsZero() && !timeRange.End.IsZero() {
		step := timeRange.Step
		if step == "" {
			step = c.calculateStep(timeRange.Start, timeRange.End)
		}

		c.log.Infof(" 查询 Controller Manager 趋势数据: start=%s, end=%s, step=%s",
			timeRange.Start.Format("2006-01-02 15:04:05"),
			timeRange.End.Format("2006-01-02 15:04:05"),
			step)

		var mu sync.Mutex
		rangeTasks := []clusterRangeTask{}

		// 总队列深度趋势（保底策略）
		rangeTasks = append(rangeTasks, clusterRangeTask{
			name:  "total_queue_trend",
			query: `sum(workqueue_depth)`,
			f: func(results []types.RangeQueryResult) error {
				if len(results) > 0 && len(results[0].Values) > 0 {
					mu.Lock()
					defer mu.Unlock()
					c.log.Infof("总队列趋势: %d 个数据点", len(results[0].Values))
					cm.Trend = make([]types.ControllerManagerDataPoint, len(results[0].Values))
					for i, v := range results[0].Values {
						cm.Trend[i] = types.ControllerManagerDataPoint{
							Timestamp:       v.Timestamp,
							TotalQueueDepth: int64(v.Value),
						}
					}
				}
				return nil
			},
		})

		// Deployment队列趋势
		deploymentQuery := c.findWorkqueueQuery(availableNames, []string{"deployment"})
		if deploymentQuery != "" {
			rangeTasks = append(rangeTasks, clusterRangeTask{
				name:  "deployment_queue_trend",
				query: deploymentQuery,
				f: func(results []types.RangeQueryResult) error {
					if len(results) > 0 && len(results[0].Values) > 0 {
						mu.Lock()
						defer mu.Unlock()
						c.log.Infof("Deployment 趋势: %d 个数据点", len(results[0].Values))
						for i, v := range results[0].Values {
							if i < len(cm.Trend) {
								cm.Trend[i].DeploymentQueueDepth = int64(v.Value)
							}
						}
					}
					return nil
				},
			})
		}

		// ReplicaSet队列趋势
		replicasetQuery := c.findWorkqueueQuery(availableNames, []string{"replicaset"})
		if replicasetQuery != "" {
			rangeTasks = append(rangeTasks, clusterRangeTask{
				name:  "replicaset_queue_trend",
				query: replicasetQuery,
				f: func(results []types.RangeQueryResult) error {
					if len(results) > 0 && len(results[0].Values) > 0 {
						mu.Lock()
						defer mu.Unlock()
						c.log.Infof("ReplicaSet 趋势: %d 个数据点", len(results[0].Values))
						for i, v := range results[0].Values {
							if i < len(cm.Trend) {
								cm.Trend[i].ReplicaSetQueueDepth = int64(v.Value)
							}
						}
					}
					return nil
				},
			})
		}

		// StatefulSet队列趋势
		statefulsetQuery := c.findWorkqueueQuery(availableNames, []string{"statefulset"})
		if statefulsetQuery != "" {
			rangeTasks = append(rangeTasks, clusterRangeTask{
				name:  "statefulset_queue_trend",
				query: statefulsetQuery,
				f: func(results []types.RangeQueryResult) error {
					if len(results) > 0 && len(results[0].Values) > 0 {
						mu.Lock()
						defer mu.Unlock()
						c.log.Infof("StatefulSet 趋势: %d 个数据点", len(results[0].Values))
						for i, v := range results[0].Values {
							if i < len(cm.Trend) {
								cm.Trend[i].StatefulSetQueueDepth = int64(v.Value)
							}
						}
					}
					return nil
				},
			})
		}

		// 重试率趋势
		rangeTasks = append(rangeTasks, clusterRangeTask{
			name:  "retry_rate_trend",
			query: fmt.Sprintf(`sum(rate(workqueue_retries_total[%s]))`, window),
			f: func(results []types.RangeQueryResult) error {
				if len(results) > 0 && len(results[0].Values) > 0 {
					mu.Lock()
					defer mu.Unlock()
					c.log.Infof("重试率趋势: %d 个数据点", len(results[0].Values))
					for i, v := range results[0].Values {
						if i < len(cm.Trend) {
							cm.Trend[i].RetryRate = v.Value
						}
					}
				}
				return nil
			},
		})

		c.executeParallelRangeQueries(timeRange.Start, timeRange.End, step, rangeTasks)
		c.log.Infof(" Controller Manager 趋势数据查询完成，共 %d 个数据点", len(cm.Trend))
	}

	return cm, nil
}

// findWorkqueueQuery 智能查找workqueue查询
func (c *ClusterOperator) findWorkqueueQuery(availableNames map[string]float64, keywords []string) string {
	if len(availableNames) == 0 {
		c.log.Errorf("  没有可用的 workqueue")
		return ""
	}

	// 策略1：精确匹配
	for _, keyword := range keywords {
		for name := range availableNames {
			if strings.EqualFold(name, keyword) {
				query := fmt.Sprintf(`sum(workqueue_depth{name="%s"})`, name)
				c.log.Infof("精确匹配 [%s] -> %s", keyword, name)
				return query
			}
		}
	}

	// 策略2：包含匹配（优先最短）
	var bestMatch string
	minLen := 999
	for name := range availableNames {
		nameLower := strings.ToLower(name)
		for _, keyword := range keywords {
			if strings.Contains(nameLower, strings.ToLower(keyword)) {
				if len(name) < minLen {
					bestMatch = name
					minLen = len(name)
				}
			}
		}
	}
	if bestMatch != "" {
		query := fmt.Sprintf(`sum(workqueue_depth{name="%s"})`, bestMatch)
		c.log.Infof("包含匹配 %v -> %s", keywords, bestMatch)
		return query
	}

	// 策略3：正则匹配（最后手段）
	pattern := strings.Join(keywords, "|")
	query := fmt.Sprintf(`sum(workqueue_depth{name=~"(?i).*(%s).*"})`, pattern)
	c.log.Errorf("  使用正则匹配 %v: %s", keywords, query)
	return query
}

// getReconcileLatency 获取控制器协调延迟
func (c *ClusterOperator) getReconcileLatency(cm *types.ControllerManagerMetrics, window string, availableNames map[string]float64) {
	c.log.Infof(" 查询控制器协调延迟")

	tasks := []clusterQueryTask{
		{
			name:  "deployment_reconcile",
			query: c.buildReconcileQuery(availableNames, []string{"deployment"}, window),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					cm.ReconcileLatency.DeploymentP99 = results[0].Value
					c.log.Infof("Deployment Reconcile P99: %.4f s", cm.ReconcileLatency.DeploymentP99)
				}
				return nil
			},
		},
		{
			name:  "replicaset_reconcile",
			query: c.buildReconcileQuery(availableNames, []string{"replicaset"}, window),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					cm.ReconcileLatency.ReplicaSetP99 = results[0].Value
					c.log.Infof("ReplicaSet Reconcile P99: %.4f s", cm.ReconcileLatency.ReplicaSetP99)
				}
				return nil
			},
		},
		{
			name:  "statefulset_reconcile",
			query: c.buildReconcileQuery(availableNames, []string{"statefulset"}, window),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					cm.ReconcileLatency.StatefulSetP99 = results[0].Value
					c.log.Infof("StatefulSet Reconcile P99: %.4f s", cm.ReconcileLatency.StatefulSetP99)
				}
				return nil
			},
		},
		{
			name:  "daemonset_reconcile",
			query: c.buildReconcileQuery(availableNames, []string{"daemonset"}, window),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					cm.ReconcileLatency.DaemonSetP99 = results[0].Value
					c.log.Infof("DaemonSet Reconcile P99: %.4f s", cm.ReconcileLatency.DaemonSetP99)
				}
				return nil
			},
		},
		{
			name:  "job_reconcile",
			query: c.buildReconcileQuery(availableNames, []string{"job"}, window),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					cm.ReconcileLatency.JobP99 = results[0].Value
					c.log.Infof("Job Reconcile P99: %.4f s", cm.ReconcileLatency.JobP99)
				}
				return nil
			},
		},
	}

	c.executeParallelQueries(tasks)
}

// buildReconcileQuery 构建协调延迟查询
func (c *ClusterOperator) buildReconcileQuery(availableNames map[string]float64, keywords []string, window string) string {
	// 先尝试 controller_runtime_reconcile_time_seconds
	for _, keyword := range keywords {
		query := fmt.Sprintf(`histogram_quantile(0.99, sum(rate(controller_runtime_reconcile_time_seconds_bucket{controller=~"(?i).*%s.*"}[%s])) by (le))`, keyword, window)
		results, err := c.query(query, nil)
		if err == nil && len(results) > 0 {
			c.log.Infof("使用 controller_runtime 指标: %s", keyword)
			return query
		}
	}

	// 降级：使用workqueue_work_duration
	matchedName := ""
	for _, keyword := range keywords {
		for name := range availableNames {
			if strings.Contains(strings.ToLower(name), strings.ToLower(keyword)) {
				matchedName = name
				break
			}
		}
		if matchedName != "" {
			break
		}
	}

	if matchedName != "" {
		query := fmt.Sprintf(`histogram_quantile(0.99, sum(rate(workqueue_work_duration_seconds_bucket{name="%s"}[%s])) by (le))`, matchedName, window)
		c.log.Infof("使用 workqueue_work_duration: %s", matchedName)
		return query
	}

	// 最后降级
	c.log.Errorf("  无法构建 reconcile 查询: %v", keywords)
	return fmt.Sprintf(`histogram_quantile(0.99, sum(rate(workqueue_work_duration_seconds_bucket[%s])) by (le))`, window)
}

// ==================== 辅助结构体和函数 ====================

// WorkqueueInfo 存储workqueue诊断信息
type WorkqueueInfo struct {
	available      bool                       // workqueue_depth 指标是否可用
	metricName     string                     // 实际的指标名称
	nameMap        map[string]float64         // name -> current depth
	allMetrics     []types.InstantQueryResult // 所有原始指标
	matchedQueries map[string]string          // keyword -> matched queue name
}

// diagnoseWorkqueueMetrics 诊断workqueue指标可用性
func (c *ClusterOperator) diagnoseWorkqueueMetrics() *WorkqueueInfo {
	info := &WorkqueueInfo{
		nameMap:        make(map[string]float64),
		matchedQueries: make(map[string]string),
	}

	c.log.Infof("🔍 开始诊断 workqueue 指标...")

	// 1. 尝试查询不同的指标名称
	possibleMetrics := []string{
		"workqueue_depth",
		"workqueue_queue_length",
		"workqueue_current_depth",
	}

	var results []types.InstantQueryResult
	var err error

	for _, metric := range possibleMetrics {
		results, err = c.query(metric, nil)
		if err == nil && len(results) > 0 {
			info.available = true
			info.metricName = metric
			info.allMetrics = results
			c.log.Infof("找到可用指标: %s (共 %d 个时序)", metric, len(results))
			break
		}
	}

	if !info.available {
		c.log.Errorf("❌ 未找到任何 workqueue depth 指标")
		return info
	}

	// 2. 解析所有name标签
	for _, result := range results {
		name, ok := result.Metric["name"]
		if !ok {
			c.log.Errorf("  发现没有name标签的指标: %+v", result.Metric)
			continue
		}
		info.nameMap[name] += result.Value
	}

	// 3. 尝试匹配常见controller
	keywords := []string{
		"deployment",
		"replicaset",
		"statefulset",
		"daemonset",
		"job",
		"node",
		"service",
		"endpoint",
	}

	c.log.Infof("🔎 开始匹配分析:")
	for _, keyword := range keywords {
		if matched := c.findBestMatch(info.nameMap, keyword); matched != "" {
			info.matchedQueries[keyword] = matched
			c.log.Infof("  %s -> %s", keyword, matched)
		} else {
			c.log.Errorf("  ❌ %s 未匹配到任何队列", keyword)
		}
	}

	return info
}

// findBestMatch 查找最佳匹配的workqueue名称
func (c *ClusterOperator) findBestMatch(nameMap map[string]float64, keyword string) string {
	keywordLower := strings.ToLower(keyword)

	// 策略1：精确匹配（忽略大小写）
	for name := range nameMap {
		if strings.EqualFold(name, keyword) {
			return name
		}
	}

	// 策略2：完全包含匹配（优先最短）
	var candidates []string
	for name := range nameMap {
		nameLower := strings.ToLower(name)
		if nameLower == keywordLower {
			return name
		}
		if strings.Contains(nameLower, keywordLower) {
			candidates = append(candidates, name)
		}
	}

	if len(candidates) > 0 {
		// 返回最短的匹配（最精确）
		shortest := candidates[0]
		for _, c := range candidates[1:] {
			if len(c) < len(shortest) {
				shortest = c
			}
		}
		return shortest
	}

	// 策略3：后缀匹配 (xxx_controller)
	suffix := keyword + "_controller"
	for name := range nameMap {
		if strings.HasSuffix(strings.ToLower(name), suffix) {
			return name
		}
	}

	// 策略4：前缀匹配 (controller-xxx)
	prefix := "controller-" + keyword
	for name := range nameMap {
		if strings.HasPrefix(strings.ToLower(name), prefix) {
			return name
		}
	}

	// 策略5：包含匹配（允许下划线、横线分隔）
	patterns := []string{
		keyword + "_",
		keyword + "-",
		"_" + keyword,
		"-" + keyword,
	}
	for name := range nameMap {
		nameLower := strings.ToLower(name)
		for _, pattern := range patterns {
			if strings.Contains(nameLower, pattern) {
				return name
			}
		}
	}

	return ""
}

// buildWorkqueueQueryV2 构建workqueue查询（改进版）
func (c *ClusterOperator) buildWorkqueueQueryV2(info *WorkqueueInfo, keywords []string) (string, bool) {
	if !info.available {
		c.log.Errorf("  workqueue 指标不可用")
		return "", false
	}

	// 尝试从已匹配的查询中查找
	for _, keyword := range keywords {
		if matched, ok := info.matchedQueries[keyword]; ok {
			query := fmt.Sprintf(`sum(%s{name="%s"})`, info.metricName, matched)
			return query, true
		}
	}

	// 降级：尝试动态匹配
	for _, keyword := range keywords {
		if matched := c.findBestMatch(info.nameMap, keyword); matched != "" {
			query := fmt.Sprintf(`sum(%s{name="%s"})`, info.metricName, matched)
			c.log.Infof("动态匹配 %s -> %s", keyword, matched)
			return query, true
		}
	}

	// 最后降级：正则匹配
	pattern := strings.Join(keywords, "|")
	query := fmt.Sprintf(`sum(%s{name=~"(?i).*(%s).*"})`, info.metricName, pattern)
	c.log.Errorf("  使用正则匹配: %s", query)

	// 验证正则查询是否有结果
	if results, err := c.query(query, nil); err == nil && len(results) > 0 {
		return query, true
	}

	return query, false
}

// selectAvailableQuery 选择第一个有数据的查询
func (c *ClusterOperator) selectAvailableQuery(queries []string) string {
	for i, query := range queries {
		results, err := c.query(query, nil)
		if err == nil && len(results) > 0 {
			if i > 0 {
				c.log.Infof("使用备选查询[%d]: %s", i, query)
			}
			return query
		}
	}
	c.log.Errorf("  所有查询都不可用，使用第一个: %s", queries[0])
	return queries[0]
}

// getReconcileLatencyV2 获取控制器协调延迟（改进版）
func (c *ClusterOperator) getReconcileLatencyV2(cm *types.ControllerManagerMetrics, window string, info *WorkqueueInfo) {
	c.log.Infof(" 查询控制器协调延迟")

	// 控制器配置
	configs := []struct {
		keywords []string
		target   *float64
		name     string
	}{
		{[]string{"deployment"}, &cm.ReconcileLatency.DeploymentP99, "Deployment"},
		{[]string{"replicaset"}, &cm.ReconcileLatency.ReplicaSetP99, "ReplicaSet"},
		{[]string{"statefulset"}, &cm.ReconcileLatency.StatefulSetP99, "StatefulSet"},
		{[]string{"daemonset"}, &cm.ReconcileLatency.DaemonSetP99, "DaemonSet"},
		{[]string{"job"}, &cm.ReconcileLatency.JobP99, "Job"},
	}

	tasks := []clusterQueryTask{}

	for _, config := range configs {
		query := c.buildReconcileLatencyQuery(config.keywords, window, info)
		target := config.target
		name := config.name

		tasks = append(tasks, clusterQueryTask{
			name:  strings.ToLower(name) + "_reconcile",
			query: query,
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					*target = results[0].Value
					c.log.Infof("%s Reconcile P99: %.4f s", name, *target)
				}
				return nil
			},
		})
	}

	c.executeParallelQueries(tasks)
}

// buildReconcileLatencyQuery 构建协调延迟查询
func (c *ClusterOperator) buildReconcileLatencyQuery(keywords []string, window string, info *WorkqueueInfo) string {
	// 尝试多种指标格式
	queries := []string{}

	// 格式1：controller_runtime_reconcile_time_seconds
	for _, keyword := range keywords {
		queries = append(queries,
			fmt.Sprintf(`histogram_quantile(0.99, sum(rate(controller_runtime_reconcile_time_seconds_bucket{controller=~"(?i).*%s.*"}[%s])) by (le))`, keyword, window),
		)
	}

	// 格式2：使用workqueue_work_duration作为近似值
	if info.available {
		for _, keyword := range keywords {
			if matched := c.findBestMatch(info.nameMap, keyword); matched != "" {
				queries = append(queries,
					fmt.Sprintf(`histogram_quantile(0.99, sum(rate(workqueue_work_duration_seconds_bucket{name="%s"}[%s])) by (le))`, matched, window),
				)
			}
		}
	}

	return c.selectAvailableQuery(queries)
}

// queryControllerManagerTrendV2 查询趋势数据（改进版）
func (c *ClusterOperator) queryControllerManagerTrendV2(cm *types.ControllerManagerMetrics, timeRange *types.TimeRange, window string, info *WorkqueueInfo) {
	step := timeRange.Step
	if step == "" {
		step = c.calculateStep(timeRange.Start, timeRange.End)
	}

	c.log.Infof(" 查询 Controller Manager 趋势数据: start=%s, end=%s, step=%s",
		timeRange.Start.Format("2006-01-02 15:04:05"),
		timeRange.End.Format("2006-01-02 15:04:05"),
		step)

	var mu sync.Mutex
	rangeTasks := []clusterRangeTask{}

	// 如果有deployment队列，查询其趋势
	if query, found := c.buildWorkqueueQueryV2(info, []string{"deployment"}); found {
		rangeTasks = append(rangeTasks, clusterRangeTask{
			name:  "deployment_queue_trend",
			query: query,
			f: func(results []types.RangeQueryResult) error {
				if len(results) > 0 && len(results[0].Values) > 0 {
					mu.Lock()
					defer mu.Unlock()
					cm.Trend = make([]types.ControllerManagerDataPoint, len(results[0].Values))
					for i, v := range results[0].Values {
						cm.Trend[i] = types.ControllerManagerDataPoint{
							Timestamp:            v.Timestamp,
							DeploymentQueueDepth: int64(v.Value),
						}
					}
				}
				return nil
			},
		})

		// ReplicaSet趋势
		if query, found := c.buildWorkqueueQueryV2(info, []string{"replicaset"}); found {
			rangeTasks = append(rangeTasks, clusterRangeTask{
				name:  "replicaset_queue_trend",
				query: query,
				f: func(results []types.RangeQueryResult) error {
					if len(results) > 0 && len(results[0].Values) > 0 {
						mu.Lock()
						defer mu.Unlock()
						for i, v := range results[0].Values {
							if i < len(cm.Trend) {
								cm.Trend[i].ReplicaSetQueueDepth = int64(v.Value)
							}
						}
					}
					return nil
				},
			})
		}

		// StatefulSet趋势
		if query, found := c.buildWorkqueueQueryV2(info, []string{"statefulset"}); found {
			rangeTasks = append(rangeTasks, clusterRangeTask{
				name:  "statefulset_queue_trend",
				query: query,
				f: func(results []types.RangeQueryResult) error {
					if len(results) > 0 && len(results[0].Values) > 0 {
						mu.Lock()
						defer mu.Unlock()
						for i, v := range results[0].Values {
							if i < len(cm.Trend) {
								cm.Trend[i].StatefulSetQueueDepth = int64(v.Value)
							}
						}
					}
					return nil
				},
			})
		}
	}

	// 总队列深度趋势
	if info.available {
		rangeTasks = append(rangeTasks, clusterRangeTask{
			name:  "total_queue_trend",
			query: fmt.Sprintf(`sum(%s)`, info.metricName),
			f: func(results []types.RangeQueryResult) error {
				if len(results) > 0 && len(results[0].Values) > 0 {
					mu.Lock()
					defer mu.Unlock()
					for i, v := range results[0].Values {
						if i < len(cm.Trend) {
							cm.Trend[i].TotalQueueDepth = int64(v.Value)
						}
					}
				}
				return nil
			},
		})
	}

	// 重试率趋势
	rangeTasks = append(rangeTasks, clusterRangeTask{
		name:  "retry_rate_trend",
		query: fmt.Sprintf(`sum(rate(workqueue_retries_total[%s]))`, window),
		f: func(results []types.RangeQueryResult) error {
			if len(results) > 0 && len(results[0].Values) > 0 {
				mu.Lock()
				defer mu.Unlock()
				for i, v := range results[0].Values {
					if i < len(cm.Trend) {
						cm.Trend[i].RetryRate = v.Value
					}
				}
			}
			return nil
		},
	})

	if len(rangeTasks) > 0 {
		c.executeParallelRangeQueries(timeRange.Start, timeRange.End, step, rangeTasks)
		c.log.Infof(" Controller Manager 趋势数据查询完成，共 %d 个数据点", len(cm.Trend))
	} else {
		c.log.Errorf("  无法构建趋势查询，跳过")
	}
}

// DiagnoseControllerManagerMetrics 诊断Controller Manager指标（增强版）
func (c *ClusterOperator) DiagnoseControllerManagerMetrics() map[string]interface{} {
	diagnosis := make(map[string]interface{})

	c.log.Infof("🔍 开始完整诊断 Controller Manager 指标...")

	// 1. 诊断workqueue指标
	info := c.diagnoseWorkqueueMetrics()
	diagnosis["workqueue_available"] = info.available
	diagnosis["workqueue_metric_name"] = info.metricName
	diagnosis["workqueue_count"] = len(info.nameMap)
	diagnosis["workqueue_names"] = info.nameMap
	diagnosis["matched_queries"] = info.matchedQueries

	// 2. 检查Controller Manager进程状态
	cmUpQuery := `up{job=~".*controller.*manager.*"}`
	cmResults, err := c.query(cmUpQuery, nil)
	diagnosis["controller_manager_up_query_success"] = err == nil
	diagnosis["controller_manager_instances"] = len(cmResults)

	if len(cmResults) > 0 {
		upCount := 0
		for _, r := range cmResults {
			if r.Value > 0 {
				upCount++
			}
		}
		diagnosis["controller_manager_up_count"] = upCount
	}

	// 3. 检查leader状态
	leaderQueries := []string{
		`leader_election_master_status{name=~".*controller.*manager.*"}`,
		`leader_election_master_status{name="kube-controller-manager"}`,
		`leader_election_master_status`,
	}

	for i, query := range leaderQueries {
		results, err := c.query(query, nil)
		if err == nil && len(results) > 0 {
			diagnosis["leader_query_success"] = true
			diagnosis["leader_query_index"] = i
			diagnosis["leader_instances"] = len(results)

			for _, r := range results {
				if r.Value > 0 {
					diagnosis["has_leader"] = true
					diagnosis["leader_metric"] = r.Metric
					break
				}
			}
			break
		}
	}

	// 4. 检查各种workqueue指标的可用性
	metricChecks := map[string]string{
		"workqueue_depth":                     `workqueue_depth`,
		"workqueue_adds_total":                `workqueue_adds_total`,
		"workqueue_retries_total":             `workqueue_retries_total`,
		"workqueue_queue_duration_seconds":    `workqueue_queue_duration_seconds_bucket`,
		"workqueue_work_duration_seconds":     `workqueue_work_duration_seconds_bucket`,
		"workqueue_longest_running_processor": `workqueue_longest_running_processor_seconds`,
		"controller_runtime_reconcile_errors": `controller_runtime_reconcile_errors_total`,
		"controller_runtime_reconcile_time":   `controller_runtime_reconcile_time_seconds_bucket`,
	}

	availableMetrics := make(map[string]bool)
	for name, query := range metricChecks {
		results, err := c.query(query, nil)
		availableMetrics[name] = err == nil && len(results) > 0
	}
	diagnosis["available_metrics"] = availableMetrics

	// 5. 统计建议
	suggestions := []string{}
	if !info.available {
		suggestions = append(suggestions, "Controller Manager 未暴露 workqueue_depth 指标，请检查 --bind-address 配置")
	}
	if !diagnosis["has_leader"].(bool) {
		suggestions = append(suggestions, "未检测到 Leader，请检查 leader election 配置")
	}
	if len(info.matchedQueries) < 4 {
		suggestions = append(suggestions, fmt.Sprintf("只匹配到 %d 个 controller 队列，可能有遗漏", len(info.matchedQueries)))
	}
	diagnosis["suggestions"] = suggestions

	c.log.Infof("诊断完成，建议: %v", suggestions)
	return diagnosis
}

// ==================== 辅助函数 ====================

// discoverWorkqueues 发现所有可用的workqueue
func (c *ClusterOperator) discoverWorkqueues() (map[string]bool, error) {
	allWorkqueueQuery := `workqueue_depth`
	results, err := c.query(allWorkqueueQuery, nil)
	if err != nil {
		return nil, err
	}

	workqueueMap := make(map[string]bool)
	for _, result := range results {
		if name, ok := result.Metric["name"]; ok && name != "" {
			workqueueMap[name] = true
		}
	}

	return workqueueMap, nil
}

// buildWorkqueueQuery 构建workqueue查询（改进版）
func (c *ClusterOperator) buildWorkqueueQuery(workqueueMap map[string]bool, keywords []string) string {
	if len(workqueueMap) == 0 {
		// 降级：使用正则匹配
		pattern := strings.Join(keywords, "|")
		return fmt.Sprintf(`sum(workqueue_depth{name=~"(?i).*(%s).*"})`, pattern)
	}

	// 策略1：精确匹配（忽略大小写）
	for _, keyword := range keywords {
		for name := range workqueueMap {
			if strings.EqualFold(name, keyword) {
				c.log.Infof("精确匹配 workqueue: %s", name)
				return fmt.Sprintf(`sum(workqueue_depth{name="%s"})`, name)
			}
		}
	}

	// 策略2：包含匹配（优先匹配最短的名称）
	var bestMatch string
	minLen := 999
	for name := range workqueueMap {
		nameLower := strings.ToLower(name)
		for _, keyword := range keywords {
			keywordLower := strings.ToLower(keyword)
			if strings.Contains(nameLower, keywordLower) {
				if len(name) < minLen {
					bestMatch = name
					minLen = len(name)
				}
			}
		}
	}
	if bestMatch != "" {
		c.log.Infof("包含匹配 workqueue: %s", bestMatch)
		return fmt.Sprintf(`sum(workqueue_depth{name="%s"})`, bestMatch)
	}

	// 策略3：后缀匹配（controller后缀）
	for _, keyword := range keywords {
		suffix := keyword + "_controller"
		for name := range workqueueMap {
			if strings.HasSuffix(strings.ToLower(name), strings.ToLower(suffix)) {
				c.log.Infof("后缀匹配 workqueue: %s", name)
				return fmt.Sprintf(`sum(workqueue_depth{name="%s"})`, name)
			}
		}
	}

	// 策略4：正则匹配（最后手段）
	pattern := strings.Join(keywords, "|")
	query := fmt.Sprintf(`sum(workqueue_depth{name=~"(?i).*(%s).*"})`, pattern)
	c.log.Errorf("  使用正则匹配: %s", query)
	return query
}

// queryControllerManagerTrend 查询趋势数据
func (c *ClusterOperator) queryControllerManagerTrend(cm *types.ControllerManagerMetrics, timeRange *types.TimeRange, window string, workqueueMap map[string]bool) {
	step := timeRange.Step
	if step == "" {
		step = c.calculateStep(timeRange.Start, timeRange.End)
	}

	c.log.Infof(" 查询 Controller Manager 趋势数据: start=%s, end=%s, step=%s",
		timeRange.Start.Format("2006-01-02 15:04:05"),
		timeRange.End.Format("2006-01-02 15:04:05"),
		step)

	var mu sync.Mutex
	rangeTasks := []clusterRangeTask{
		{
			name:  "deployment_queue_trend",
			query: c.buildWorkqueueQuery(workqueueMap, []string{"deployment"}),
			f: func(results []types.RangeQueryResult) error {
				if len(results) > 0 && len(results[0].Values) > 0 {
					mu.Lock()
					defer mu.Unlock()
					cm.Trend = make([]types.ControllerManagerDataPoint, len(results[0].Values))
					for i, v := range results[0].Values {
						cm.Trend[i] = types.ControllerManagerDataPoint{
							Timestamp:            v.Timestamp,
							DeploymentQueueDepth: int64(v.Value),
						}
					}
				}
				return nil
			},
		},
		{
			name:  "total_queue_trend",
			query: `sum(workqueue_depth)`,
			f: func(results []types.RangeQueryResult) error {
				if len(results) > 0 && len(results[0].Values) > 0 {
					mu.Lock()
					defer mu.Unlock()
					for i, v := range results[0].Values {
						if i < len(cm.Trend) {
							cm.Trend[i].TotalQueueDepth = int64(v.Value)
						}
					}
				}
				return nil
			},
		},
		{
			name:  "retry_rate_trend",
			query: fmt.Sprintf(`sum(rate(workqueue_retries_total[%s]))`, window),
			f: func(results []types.RangeQueryResult) error {
				if len(results) > 0 && len(results[0].Values) > 0 {
					mu.Lock()
					defer mu.Unlock()
					for i, v := range results[0].Values {
						if i < len(cm.Trend) {
							cm.Trend[i].RetryRate = v.Value
						}
					}
				}
				return nil
			},
		},
	}

	c.executeParallelRangeQueries(timeRange.Start, timeRange.End, step, rangeTasks)
	c.log.Infof(" Controller Manager 趋势数据查询完成，共 %d 个数据点", len(cm.Trend))
}

// GetEtcdMetrics 获取 Etcd 指标
func (c *ClusterOperator) GetEtcdMetrics(timeRange *types.TimeRange) (*types.EtcdMetrics, error) {
	etcd := &types.EtcdMetrics{Trend: []types.EtcdDataPoint{}}
	window := c.calculateRateWindow(timeRange)

	c.log.Infof(" 开始查询 Etcd 指标 (window: %s)", window)

	// ==================== 集群状态 ====================
	tasks := []clusterQueryTask{
		{
			name:  "has_leader",
			query: `etcd_server_has_leader`,
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					etcd.HasLeader = results[0].Value > 0
					c.log.Infof("Etcd HasLeader: %v", etcd.HasLeader)
				} else {
					c.log.Errorf("  etcd_server_has_leader 指标不存在")
				}
				return nil
			},
		},
		{
			name:  "leader_changes",
			query: `sum(increase(etcd_server_leader_changes_seen_total[1h]))`,
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					etcd.LeaderChanges = int64(results[0].Value)
					c.log.Infof("Etcd Leader Changes (1h): %d", etcd.LeaderChanges)
				}
				return nil
			},
		},
		{
			name:  "member_count",
			query: `count(etcd_server_has_leader)`,
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					etcd.MemberCount = int64(results[0].Value)
					c.log.Infof("Etcd Member Count: %d", etcd.MemberCount)
				}
				return nil
			},
		},
		{
			name:  "is_learner",
			query: `sum(etcd_server_is_learner)`,
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					etcd.IsLearner = results[0].Value > 0
				}
				return nil
			},
		},

		// ==================== 存储指标 ====================
		{
			name:  "db_size",
			query: `sum(etcd_mvcc_db_total_size_in_bytes)`,
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					etcd.DBSizeBytes = int64(results[0].Value)
					c.log.Infof("Etcd DB Size: %d bytes (%.2f MB)",
						etcd.DBSizeBytes, float64(etcd.DBSizeBytes)/1024/1024)
				}
				return nil
			},
		},
		{
			name:  "db_size_in_use",
			query: `sum(etcd_mvcc_db_total_size_in_use_in_bytes)`,
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					etcd.DBSizeInUse = int64(results[0].Value)
				}
				return nil
			},
		},
		{
			name:  "db_size_limit",
			query: `sum(etcd_server_quota_backend_bytes)`,
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					etcd.DBSizeLimit = int64(results[0].Value)
					c.log.Infof("Etcd DB Size Limit: %d bytes (%.2f GB)",
						etcd.DBSizeLimit, float64(etcd.DBSizeLimit)/1024/1024/1024)
				}
				return nil
			},
		},
		{
			name:  "key_total",
			query: `sum(etcd_debugging_mvcc_keys_total)`,
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					etcd.KeyTotal = int64(results[0].Value)
					c.log.Infof("Etcd Key Total: %d", etcd.KeyTotal)
				}
				return nil
			},
		},

		// ==================== 性能指标 ====================
		{
			name:  "commit_latency",
			query: fmt.Sprintf(`histogram_quantile(0.99, sum(rate(etcd_disk_backend_commit_duration_seconds_bucket[%s])) by (le))`, window),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					etcd.CommitLatency = results[0].Value
					c.log.Infof("Etcd Commit Latency P99: %.4f s (%.2f ms)",
						etcd.CommitLatency, etcd.CommitLatency*1000)
				}
				return nil
			},
		},
		{
			name:  "wal_fsync",
			query: fmt.Sprintf(`histogram_quantile(0.99, sum(rate(etcd_disk_wal_fsync_duration_seconds_bucket[%s])) by (le))`, window),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					etcd.WALFsyncLatency = results[0].Value
					c.log.Infof("Etcd WAL Fsync Latency P99: %.4f s (%.2f ms)",
						etcd.WALFsyncLatency, etcd.WALFsyncLatency*1000)
				}
				return nil
			},
		},
		{
			name:  "apply_latency",
			query: fmt.Sprintf(`histogram_quantile(0.99, sum(rate(etcd_server_apply_duration_seconds_bucket[%s])) by (le))`, window),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					etcd.ApplyLatency = results[0].Value
					c.log.Infof("Etcd Apply Latency P99: %.4f s", etcd.ApplyLatency)
				}
				return nil
			},
		},
		{
			name:  "snapshot_latency",
			query: fmt.Sprintf(`histogram_quantile(0.99, sum(rate(etcd_debugging_snap_save_total_duration_seconds_bucket[%s])) by (le))`, window),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					etcd.SnapshotLatency = results[0].Value
				}
				return nil
			},
		},
		{
			name:  "compact_latency",
			query: fmt.Sprintf(`histogram_quantile(0.99, sum(rate(etcd_debugging_mvcc_db_compaction_pause_duration_milliseconds_bucket[%s])) by (le)) / 1000`, window),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					etcd.CompactLatency = results[0].Value
				}
				return nil
			},
		},

		// ==================== 网络指标 ====================
		{
			name:  "peer_rtt",
			query: fmt.Sprintf(`histogram_quantile(0.99, sum(rate(etcd_network_peer_round_trip_time_seconds_bucket[%s])) by (le))`, window),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					etcd.PeerRTT = results[0].Value
					c.log.Infof("Etcd Peer RTT P99: %.4f s (%.2f ms)",
						etcd.PeerRTT, etcd.PeerRTT*1000)
				}
				return nil
			},
		},
		{
			name:  "network_send",
			query: fmt.Sprintf(`sum(rate(etcd_network_peer_sent_bytes_total[%s]))`, window),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					etcd.NetworkSendRate = results[0].Value
					c.log.Infof("Etcd Network Send Rate: %.2f bytes/s", etcd.NetworkSendRate)
				}
				return nil
			},
		},
		{
			name:  "network_recv",
			query: fmt.Sprintf(`sum(rate(etcd_network_peer_received_bytes_total[%s]))`, window),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					etcd.NetworkRecvRate = results[0].Value
					c.log.Infof("Etcd Network Recv Rate: %.2f bytes/s", etcd.NetworkRecvRate)
				}
				return nil
			},
		},

		// ==================== 操作统计 ====================
		{
			name:  "proposals_failed",
			query: fmt.Sprintf(`sum(increase(etcd_server_proposals_failed_total[%s]))`, window),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					etcd.ProposalsFailed = int64(results[0].Value)
					c.log.Infof("Etcd Proposals Failed: %d", etcd.ProposalsFailed)
				}
				return nil
			},
		},
		{
			name:  "proposals_pending",
			query: `sum(etcd_server_proposals_pending)`,
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					etcd.ProposalsPending = int64(results[0].Value)
					c.log.Infof("Etcd Proposals Pending: %d", etcd.ProposalsPending)
				}
				return nil
			},
		},
		{
			name:  "proposals_applied",
			query: fmt.Sprintf(`sum(increase(etcd_server_proposals_applied_total[%s]))`, window),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					etcd.ProposalsApplied = int64(results[0].Value)
				}
				return nil
			},
		},
		{
			name:  "proposals_committed",
			query: fmt.Sprintf(`sum(increase(etcd_server_proposals_committed_total[%s]))`, window),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					etcd.ProposalsCommitted = int64(results[0].Value)
				}
				return nil
			},
		},

		// ==================== 操作速率 ====================
		{
			name:  "get_rate",
			query: fmt.Sprintf(`sum(rate(etcd_mvcc_range_total[%s]))`, window),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					etcd.GetRate = results[0].Value
					c.log.Infof("Etcd Get Rate: %.2f ops/s", etcd.GetRate)
				}
				return nil
			},
		},
		{
			name:  "put_rate",
			query: fmt.Sprintf(`sum(rate(etcd_mvcc_put_total[%s]))`, window),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					etcd.PutRate = results[0].Value
					c.log.Infof("Etcd Put Rate: %.2f ops/s", etcd.PutRate)
				}
				return nil
			},
		},
		{
			name:  "delete_rate",
			query: fmt.Sprintf(`sum(rate(etcd_mvcc_delete_total[%s]))`, window),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					etcd.DeleteRate = results[0].Value
					c.log.Infof("Etcd Delete Rate: %.2f ops/s", etcd.DeleteRate)
				}
				return nil
			},
		},

		// ==================== 慢操作 ====================
		{
			name:  "slow_applies",
			query: fmt.Sprintf(`sum(increase(etcd_server_slow_apply_total[%s]))`, window),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					etcd.SlowApplies = int64(results[0].Value)
					if etcd.SlowApplies > 0 {
						c.log.Errorf("  Etcd Slow Applies: %d", etcd.SlowApplies)
					}
				}
				return nil
			},
		},
		{
			name:  "slow_commits",
			query: fmt.Sprintf(`sum(increase(etcd_disk_backend_commit_duration_seconds_count{le="0.1"}[%s])) - sum(increase(etcd_disk_backend_commit_duration_seconds_count{le="0.025"}[%s]))`, window, window),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 && results[0].Value > 0 {
					etcd.SlowCommits = int64(results[0].Value)
					if etcd.SlowCommits > 0 {
						c.log.Errorf("  Etcd Slow Commits: %d", etcd.SlowCommits)
					}
				}
				return nil
			},
		},
	}

	c.executeParallelQueries(tasks)

	// ==================== 趋势数据 ====================
	if timeRange != nil && !timeRange.Start.IsZero() && !timeRange.End.IsZero() {
		step := timeRange.Step
		if step == "" {
			step = c.calculateStep(timeRange.Start, timeRange.End)
		}

		c.log.Infof(" 查询 Etcd 趋势数据: start=%s, end=%s, step=%s",
			timeRange.Start.Format("2006-01-02 15:04:05"),
			timeRange.End.Format("2006-01-02 15:04:05"),
			step)

		var mu sync.Mutex
		rangeTasks := []clusterRangeTask{
			{
				name:  "db_size_trend",
				query: `sum(etcd_mvcc_db_total_size_in_bytes)`,
				f: func(results []types.RangeQueryResult) error {
					if len(results) > 0 && len(results[0].Values) > 0 {
						mu.Lock()
						defer mu.Unlock()
						etcd.Trend = make([]types.EtcdDataPoint, len(results[0].Values))
						for i, v := range results[0].Values {
							etcd.Trend[i] = types.EtcdDataPoint{
								Timestamp:   v.Timestamp,
								DBSizeBytes: int64(v.Value),
							}
						}
					}
					return nil
				},
			},
			{
				name:  "commit_latency_trend",
				query: fmt.Sprintf(`histogram_quantile(0.99, sum(rate(etcd_disk_backend_commit_duration_seconds_bucket[%s])) by (le))`, window),
				f: func(results []types.RangeQueryResult) error {
					if len(results) > 0 && len(results[0].Values) > 0 {
						mu.Lock()
						defer mu.Unlock()
						for i, v := range results[0].Values {
							if i < len(etcd.Trend) {
								etcd.Trend[i].CommitLatency = v.Value
							}
						}
					}
					return nil
				},
			},
			{
				name:  "proposals_failed_trend",
				query: fmt.Sprintf(`sum(increase(etcd_server_proposals_failed_total[%s]))`, window),
				f: func(results []types.RangeQueryResult) error {
					if len(results) > 0 && len(results[0].Values) > 0 {
						mu.Lock()
						defer mu.Unlock()
						for i, v := range results[0].Values {
							if i < len(etcd.Trend) {
								etcd.Trend[i].ProposalsFailed = int64(v.Value)
							}
						}
					}
					return nil
				},
			},
			{
				name:  "peer_rtt_trend",
				query: fmt.Sprintf(`histogram_quantile(0.99, sum(rate(etcd_network_peer_round_trip_time_seconds_bucket[%s])) by (le))`, window),
				f: func(results []types.RangeQueryResult) error {
					if len(results) > 0 && len(results[0].Values) > 0 {
						mu.Lock()
						defer mu.Unlock()
						for i, v := range results[0].Values {
							if i < len(etcd.Trend) {
								etcd.Trend[i].PeerRTT = v.Value
							}
						}
					}
					return nil
				},
			},
			{
				name:  "get_rate_trend",
				query: fmt.Sprintf(`sum(rate(etcd_mvcc_range_total[%s]))`, window),
				f: func(results []types.RangeQueryResult) error {
					if len(results) > 0 && len(results[0].Values) > 0 {
						mu.Lock()
						defer mu.Unlock()
						for i, v := range results[0].Values {
							if i < len(etcd.Trend) {
								etcd.Trend[i].GetRate = v.Value
							}
						}
					}
					return nil
				},
			},
			{
				name:  "put_rate_trend",
				query: fmt.Sprintf(`sum(rate(etcd_mvcc_put_total[%s]))`, window),
				f: func(results []types.RangeQueryResult) error {
					if len(results) > 0 && len(results[0].Values) > 0 {
						mu.Lock()
						defer mu.Unlock()
						for i, v := range results[0].Values {
							if i < len(etcd.Trend) {
								etcd.Trend[i].PutRate = v.Value
							}
						}
					}
					return nil
				},
			},
		}

		c.executeParallelRangeQueries(timeRange.Start, timeRange.End, step, rangeTasks)
		c.log.Infof(" Etcd 趋势数据查询完成，共 %d 个数据点", len(etcd.Trend))
	}

	return etcd, nil
}

func (c *ClusterOperator) GetClusterWorkloads(timeRange *types.TimeRange) (*types.ClusterWorkloadMetrics, error) {
	workloads := &types.ClusterWorkloadMetrics{}

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		if pods, err := c.GetClusterPods(timeRange); err == nil {
			workloads.Pods = *pods
		}
	}()

	go func() {
		defer wg.Done()
		if deployments, err := c.GetClusterDeployments(); err == nil {
			workloads.Deployments = *deployments
		}
	}()

	wg.Wait()

	// 其他工作负载统计
	workloads.StatefulSets = c.getClusterStatefulSets()
	workloads.DaemonSets = c.getClusterDaemonSets()
	workloads.Jobs = c.getClusterJobs()
	workloads.Services = c.getClusterServices()
	workloads.Containers = c.getClusterContainers()

	return workloads, nil
}

func (c *ClusterOperator) GetClusterPods(timeRange *types.TimeRange) (*types.ClusterPodMetrics, error) {
	pods := &types.ClusterPodMetrics{
		HighRestartPods: []types.PodRestartInfo{},
		Trend:           []types.ClusterPodDataPoint{},
	}

	//window := c.calculateRateWindow(timeRange)

	// 合并查询所有 Pod 统计
	tasks := []clusterQueryTask{
		{
			name:  "total",
			query: `sum(kube_pod_status_phase)`,
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					pods.Total = int64(results[0].Value)
				}
				return nil
			},
		},
		{
			name:  "running",
			query: `sum(kube_pod_status_phase{phase="Running"})`,
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					pods.Running = int64(results[0].Value)
				}
				return nil
			},
		},
		{
			name:  "pending",
			query: `sum(kube_pod_status_phase{phase="Pending"})`,
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					pods.Pending = int64(results[0].Value)
				}
				return nil
			},
		},
		{
			name:  "failed",
			query: `sum(kube_pod_status_phase{phase="Failed"})`,
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					pods.Failed = int64(results[0].Value)
				}
				return nil
			},
		},
		{
			name:  "succeeded",
			query: `sum(kube_pod_status_phase{phase="Succeeded"})`,
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					pods.Succeeded = int64(results[0].Value)
				}
				return nil
			},
		},
		{
			name:  "unknown",
			query: `sum(kube_pod_status_phase{phase="Unknown"})`,
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					pods.Unknown = int64(results[0].Value)
				}
				return nil
			},
		},
		{
			name:  "ready",
			query: `sum(kube_pod_status_ready{condition="true"})`,
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					pods.Ready = int64(results[0].Value)
					pods.NotReady = pods.Total - pods.Ready
				}
				return nil
			},
		},
		{
			name:  "restarts",
			query: `sum(kube_pod_container_status_restarts_total)`,
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					pods.TotalRestarts = int64(results[0].Value)
				}
				return nil
			},
		},
		{
			name:  "high_restart",
			query: `topk(10, sum by (namespace, pod) (kube_pod_container_status_restarts_total))`,
			f: func(results []types.InstantQueryResult) error {
				for _, r := range results {
					namespace, okNs := r.Metric["namespace"]
					pod, okPod := r.Metric["pod"]
					if !okNs || !okPod {
						continue
					}
					restartCount := int64(r.Value)
					restartRate := 0.0
					if timeRange != nil && !timeRange.Start.IsZero() {
						duration := time.Since(timeRange.Start)
						hours := duration.Hours()
						if hours > 0 {
							restartRate = float64(restartCount) / hours
						}
					}
					pods.HighRestartPods = append(pods.HighRestartPods, types.PodRestartInfo{
						Namespace:    namespace,
						PodName:      pod,
						RestartCount: restartCount,
						RestartRate:  restartRate,
					})
				}
				return nil
			},
		},
	}

	c.executeParallelQueries(tasks)

	// 趋势数据
	if timeRange != nil && !timeRange.Start.IsZero() && !timeRange.End.IsZero() {
		step := timeRange.Step
		if step == "" {
			step = c.calculateStep(timeRange.Start, timeRange.End)
		}

		trendQuery := `sum(kube_pod_status_phase{phase="Running"})`
		if trendResult, err := c.queryRange(trendQuery, timeRange.Start, timeRange.End, step); err == nil && len(trendResult) > 0 {
			pods.Trend = c.parsePodTrend(trendResult[0].Values)
		}
	}

	return pods, nil
}

func (c *ClusterOperator) GetClusterDeployments() (*types.ClusterDeploymentMetrics, error) {
	deployments := &types.ClusterDeploymentMetrics{}

	tasks := []clusterQueryTask{
		{
			name:  "total",
			query: `count(kube_deployment_created)`,
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					deployments.Total = int64(results[0].Value)
				}
				return nil
			},
		},
		{
			name:  "available",
			query: `sum(kube_deployment_status_replicas_available)`,
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					deployments.AvailableReplicas = int64(results[0].Value)
				}
				return nil
			},
		},
		{
			name:  "unavailable",
			query: `sum(kube_deployment_status_replicas_unavailable)`,
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					deployments.UnavailableReplicas = int64(results[0].Value)
				}
				return nil
			},
		},
		{
			name:  "updating",
			query: `count(kube_deployment_status_replicas_updated != kube_deployment_spec_replicas)`,
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					deployments.Updating = int64(results[0].Value)
				}
				return nil
			},
		},
	}

	c.executeParallelQueries(tasks)
	return deployments, nil
}

func (c *ClusterOperator) GetClusterNetwork(timeRange *types.TimeRange) (*types.ClusterNetworkMetrics, error) {
	network := &types.ClusterNetworkMetrics{Trend: []types.ClusterNetworkDataPoint{}}
	window := c.calculateRateWindow(timeRange)

	// ==================== 1. 查询当前流量速率 ====================
	tasks := []clusterQueryTask{
		{
			name:  "ingress",
			query: fmt.Sprintf(`sum(rate(container_network_receive_bytes_total[%s]))`, window),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					network.TotalIngressBytesPerSec = results[0].Value
				}
				return nil
			},
		},
		{
			name:  "egress",
			query: fmt.Sprintf(`sum(rate(container_network_transmit_bytes_total[%s]))`, window),
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					network.TotalEgressBytesPerSec = results[0].Value
				}
				return nil
			},
		},
	}

	c.executeParallelQueries(tasks)

	// ==================== 2. 查询趋势数据 ====================
	if timeRange != nil && !timeRange.Start.IsZero() && !timeRange.End.IsZero() {
		step := timeRange.Step
		if step == "" {
			step = c.calculateStep(timeRange.Start, timeRange.End)
		}

		c.log.Infof(" 查询网络流量趋势: start=%s, end=%s, step=%s, window=%s",
			timeRange.Start.Format(time.RFC3339),
			timeRange.End.Format(time.RFC3339),
			step,
			window)

		var mu sync.Mutex
		rangeTasks := []clusterRangeTask{
			{
				name:  "ingress_trend",
				query: fmt.Sprintf(`sum(rate(container_network_receive_bytes_total[%s]))`, window),
				f: func(results []types.RangeQueryResult) error {
					if len(results) > 0 {
						mu.Lock()
						defer mu.Unlock()

						c.log.Infof("入流量趋势数据点数: %d", len(results[0].Values))

						// 初始化 Trend 数组
						network.Trend = make([]types.ClusterNetworkDataPoint, len(results[0].Values))
						for i, v := range results[0].Values {
							network.Trend[i] = types.ClusterNetworkDataPoint{
								Timestamp:          v.Timestamp,
								IngressBytesPerSec: v.Value,
							}
						}
					} else {
						c.log.Errorf("  入流量趋势数据为空")
					}
					return nil
				},
			},
			{
				name:  "egress_trend",
				query: fmt.Sprintf(`sum(rate(container_network_transmit_bytes_total[%s]))`, window),
				f: func(results []types.RangeQueryResult) error {
					if len(results) > 0 {
						mu.Lock()
						defer mu.Unlock()

						c.log.Infof("出流量趋势数据点数: %d", len(results[0].Values))

						// 将出流量数据填充到已有的 Trend 数组中
						for i, v := range results[0].Values {
							if i < len(network.Trend) {
								network.Trend[i].EgressBytesPerSec = v.Value
							}
						}
					} else {
						c.log.Errorf("  出流量趋势数据为空")
					}
					return nil
				},
			},
		}

		c.executeParallelRangeQueries(timeRange.Start, timeRange.End, step, rangeTasks)

		c.log.Infof(" 网络流量趋势数据查询完成，共 %d 个数据点", len(network.Trend))
	} else {
		c.log.Errorf("  未提供时间范围，跳过趋势数据查询")
	}

	return network, nil
}

func (c *ClusterOperator) GetClusterStorage() (*types.ClusterStorageMetrics, error) {
	storage := &types.ClusterStorageMetrics{StorageClasses: []types.StorageClassUsage{}}

	tasks := []clusterQueryTask{
		{
			name: "pv_total",
			// 方案1：使用 sum 统计所有状态
			query: `sum(kube_persistentvolume_status_phase)`,
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					storage.PVTotal = int64(results[0].Value)
				}
				return nil
			},
		},
		{
			name: "pv_bound",
			// 使用 sum 而不是 count
			query: `sum(kube_persistentvolume_status_phase{phase="Bound"})`,
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					storage.PVBound = int64(results[0].Value)
				}
				return nil
			},
		},
		{
			name:  "pv_available",
			query: `sum(kube_persistentvolume_status_phase{phase="Available"})`,
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					storage.PVAvailable = int64(results[0].Value)
				}
				return nil
			},
		},
		{
			name:  "pv_released",
			query: `sum(kube_persistentvolume_status_phase{phase="Released"})`,
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					storage.PVReleased = int64(results[0].Value)
				}
				return nil
			},
		},
		{
			name:  "pv_failed",
			query: `sum(kube_persistentvolume_status_phase{phase="Failed"})`,
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					storage.PVFailed = int64(results[0].Value)
				}
				return nil
			},
		},
		{
			name: "pvc_total",
			// 使用 sum 统计所有状态
			query: `sum(kube_persistentvolumeclaim_status_phase)`,
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					storage.PVCTotal = int64(results[0].Value)
				}
				return nil
			},
		},
		{
			name:  "pvc_bound",
			query: `sum(kube_persistentvolumeclaim_status_phase{phase="Bound"})`,
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					storage.PVCBound = int64(results[0].Value)
				}
				return nil
			},
		},
		{
			name:  "pvc_pending",
			query: `sum(kube_persistentvolumeclaim_status_phase{phase="Pending"})`,
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					storage.PVCPending = int64(results[0].Value)
				}
				return nil
			},
		},
		{
			name:  "pvc_lost",
			query: `sum(kube_persistentvolumeclaim_status_phase{phase="Lost"})`,
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					storage.PVCLost = int64(results[0].Value)
				}
				return nil
			},
		},
	}

	c.executeParallelQueries(tasks)

	// StorageClass 统计（保持不变）
	scBytesQuery := `sum by (storageclass) (kube_persistentvolumeclaim_resource_requests_storage_bytes)`
	scCountQuery := `count by (storageclass) (kube_persistentvolumeclaim_info)`

	var wg sync.WaitGroup
	wg.Add(2)

	var scBytesResults, scCountResults []types.InstantQueryResult

	go func() {
		defer wg.Done()
		scBytesResults, _ = c.query(scBytesQuery, nil)
	}()

	go func() {
		defer wg.Done()
		scCountResults, _ = c.query(scCountQuery, nil)
	}()

	wg.Wait()

	// 合并 StorageClass 结果
	scCountMap := make(map[string]int64)
	for _, r := range scCountResults {
		if sc, ok := r.Metric["storageclass"]; ok {
			scCountMap[sc] = int64(r.Value)
		}
	}

	for _, r := range scBytesResults {
		if sc, ok := r.Metric["storageclass"]; ok {
			storage.StorageClasses = append(storage.StorageClasses, types.StorageClassUsage{
				StorageClass: sc,
				PVCCount:     scCountMap[sc],
				TotalBytes:   int64(r.Value),
			})
		}
	}

	return storage, nil
}
func (c *ClusterOperator) GetClusterNamespaces(timeRange *types.TimeRange) (*types.ClusterNamespaceMetrics, error) {
	namespaces := &types.ClusterNamespaceMetrics{
		TopByCPU:    []types.NamespaceResourceRanking{},
		TopByMemory: []types.NamespaceResourceRanking{},
		TopByPods:   []types.NamespaceResourceRanking{},
	}

	window := c.calculateRateWindow(timeRange)

	// 合并查询
	tasks := []clusterQueryTask{
		{
			name:  "total",
			query: `count(kube_namespace_created)`,
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					namespaces.Total = int64(results[0].Value)
				}
				return nil
			},
		},
		{
			name:  "active",
			query: `count(kube_namespace_status_phase{phase="Active"})`,
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					namespaces.Active = int64(results[0].Value)
				}
				return nil
			},
		},
		{
			name:  "top_cpu",
			query: fmt.Sprintf(`topk(10, sum by (namespace) (rate(container_cpu_usage_seconds_total{container!=""}[%s])))`, window),
			f: func(results []types.InstantQueryResult) error {
				for _, r := range results {
					if namespace, ok := r.Metric["namespace"]; ok {
						namespaces.TopByCPU = append(namespaces.TopByCPU, types.NamespaceResourceRanking{
							Namespace: namespace,
							Value:     r.Value,
							Unit:      "cores",
						})
					}
				}
				return nil
			},
		},
		{
			name:  "top_memory",
			query: `topk(10, sum by (namespace) (container_memory_working_set_bytes{container!=""}))`,
			f: func(results []types.InstantQueryResult) error {
				for _, r := range results {
					if namespace, ok := r.Metric["namespace"]; ok {
						namespaces.TopByMemory = append(namespaces.TopByMemory, types.NamespaceResourceRanking{
							Namespace: namespace,
							Value:     r.Value,
							Unit:      "bytes",
						})
					}
				}
				return nil
			},
		},
		{
			name:  "top_pods",
			query: `topk(10, count by (namespace) (kube_pod_info))`,
			f: func(results []types.InstantQueryResult) error {
				for _, r := range results {
					if namespace, ok := r.Metric["namespace"]; ok {
						namespaces.TopByPods = append(namespaces.TopByPods, types.NamespaceResourceRanking{
							Namespace: namespace,
							Value:     r.Value,
							Unit:      "pods",
						})
					}
				}
				return nil
			},
		},
	}

	c.executeParallelQueries(tasks)
	return namespaces, nil
}

// ==================== 辅助方法 ====================

func (c *ClusterOperator) getClusterStatefulSets() types.ClusterStatefulSetMetrics {
	sts := types.ClusterStatefulSetMetrics{}

	tasks := []clusterQueryTask{
		{
			name:  "total",
			query: `count(kube_statefulset_created)`,
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					sts.Total = int64(results[0].Value)
				}
				return nil
			},
		},
		{
			name:  "ready",
			query: `sum(kube_statefulset_status_replicas_ready)`,
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					sts.ReadyReplicas = int64(results[0].Value)
				}
				return nil
			},
		},
		{
			name:  "current",
			query: `sum(kube_statefulset_status_replicas_current)`,
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					sts.CurrentReplicas = int64(results[0].Value)
				}
				return nil
			},
		},
	}

	c.executeParallelQueries(tasks)
	return sts
}

func (c *ClusterOperator) getClusterDaemonSets() types.ClusterDaemonSetMetrics {
	ds := types.ClusterDaemonSetMetrics{}

	tasks := []clusterQueryTask{
		{
			name:  "total",
			query: `count(kube_daemonset_created)`,
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					ds.Total = int64(results[0].Value)
				}
				return nil
			},
		},
		{
			name:  "desired",
			query: `sum(kube_daemonset_status_desired_number_scheduled)`,
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					ds.DesiredPods = int64(results[0].Value)
				}
				return nil
			},
		},
		{
			name:  "available",
			query: `sum(kube_daemonset_status_number_available)`,
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					ds.AvailablePods = int64(results[0].Value)
				}
				return nil
			},
		},
		{
			name:  "unavailable",
			query: `sum(kube_daemonset_status_number_unavailable)`,
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					ds.UnavailablePods = int64(results[0].Value)
				}
				return nil
			},
		},
	}

	c.executeParallelQueries(tasks)
	return ds
}

func (c *ClusterOperator) getClusterJobs() types.ClusterJobMetrics {
	jobs := types.ClusterJobMetrics{}

	tasks := []clusterQueryTask{
		{
			name:  "active",
			query: `count(kube_job_status_active > 0)`,
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					jobs.Active = int64(results[0].Value)
				}
				return nil
			},
		},
		{
			name:  "succeeded",
			query: `count(kube_job_status_succeeded > 0)`,
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					jobs.Succeeded = int64(results[0].Value)
				}
				return nil
			},
		},
		{
			name:  "failed",
			query: `count(kube_job_status_failed > 0)`,
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					jobs.Failed = int64(results[0].Value)
				}
				return nil
			},
		},
		{
			name:  "cronjobs",
			query: `count(kube_cronjob_created)`,
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					jobs.CronJobsTotal = int64(results[0].Value)
				}
				return nil
			},
		},
	}

	c.executeParallelQueries(tasks)
	return jobs
}

func (c *ClusterOperator) getClusterServices() types.ClusterServiceMetrics {
	services := types.ClusterServiceMetrics{ByType: []types.ServiceTypeCount{}}

	tasks := []clusterQueryTask{
		{
			name:  "total",
			query: `count(kube_service_info)`,
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					services.Total = int64(results[0].Value)
				}
				return nil
			},
		},
		{
			name:  "by_type",
			query: `count by (type) (kube_service_info)`,
			f: func(results []types.InstantQueryResult) error {
				for _, r := range results {
					if serviceType, ok := r.Metric["type"]; ok {
						services.ByType = append(services.ByType, types.ServiceTypeCount{
							Type:  serviceType,
							Count: int64(r.Value),
						})
					}
				}
				return nil
			},
		},
	}

	c.executeParallelQueries(tasks)
	return services
}

func (c *ClusterOperator) getClusterContainers() types.ClusterContainerMetrics {
	containers := types.ClusterContainerMetrics{}

	tasks := []clusterQueryTask{
		{
			name:  "total",
			query: `count(kube_pod_container_info)`,
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					containers.Total = int64(results[0].Value)
				}
				return nil
			},
		},
		{
			name:  "running",
			query: `count(kube_pod_container_status_running)`,
			f: func(results []types.InstantQueryResult) error {
				if len(results) > 0 {
					containers.Running = int64(results[0].Value)
				}
				return nil
			},
		},
	}

	c.executeParallelQueries(tasks)
	return containers
}

// ==================== 查询方法 ====================

func (c *ClusterOperator) query(query string, timestamp *time.Time) ([]types.InstantQueryResult, error) {
	params := map[string]string{"query": query}
	if timestamp != nil {
		params["time"] = c.formatTimestamp(*timestamp)
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

	if err := c.doRequest("GET", "/api/v1/query", params, nil, &response); err != nil {
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

func (c *ClusterOperator) queryRange(query string, start, end time.Time, step string) ([]types.RangeQueryResult, error) {
	params := map[string]string{
		"query": query,
		"start": c.formatTimestamp(start),
		"end":   c.formatTimestamp(end),
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

	if err := c.doRequest("GET", "/api/v1/query_range", params, nil, &response); err != nil {
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

func (c *ClusterOperator) formatTimestamp(t time.Time) string {
	return fmt.Sprintf("%.3f", float64(t.UnixNano())/1e9)
}

func (c *ClusterOperator) addStatusCode(dist *types.StatusCodeDistribution, code string, value float64) {
	if len(code) < 1 {
		return
	}
	switch code[0] {
	case '2':
		dist.Status2xx[code] = value
	case '3':
		dist.Status3xx[code] = value
	case '4':
		dist.Status4xx[code] = value
	case '5':
		dist.Status5xx[code] = value
	}
}

func (c *ClusterOperator) parseAPIServerTrend(values []types.MetricValue) []types.APIServerDataPoint {
	trend := make([]types.APIServerDataPoint, 0, len(values))
	for _, v := range values {
		trend = append(trend, types.APIServerDataPoint{
			Timestamp:         v.Timestamp,
			RequestsPerSecond: v.Value,
		})
	}
	return trend
}

func (c *ClusterOperator) parseEtcdTrend(values []types.MetricValue) []types.EtcdDataPoint {
	trend := make([]types.EtcdDataPoint, 0, len(values))
	for _, v := range values {
		trend = append(trend, types.EtcdDataPoint{
			Timestamp:   v.Timestamp,
			DBSizeBytes: int64(v.Value),
		})
	}
	return trend
}

func (c *ClusterOperator) parseSchedulerTrend(values []types.MetricValue) []types.SchedulerDataPoint {
	trend := make([]types.SchedulerDataPoint, 0, len(values))
	for _, v := range values {
		trend = append(trend, types.SchedulerDataPoint{
			Timestamp:           v.Timestamp,
			ScheduleSuccessRate: v.Value,
		})
	}
	return trend
}

func (c *ClusterOperator) parseControllerManagerTrend(values []types.MetricValue) []types.ControllerManagerDataPoint {
	trend := make([]types.ControllerManagerDataPoint, 0, len(values))
	for _, v := range values {
		trend = append(trend, types.ControllerManagerDataPoint{
			Timestamp:            v.Timestamp,
			DeploymentQueueDepth: int64(v.Value),
		})
	}
	return trend
}

func (c *ClusterOperator) parsePodTrend(values []types.MetricValue) []types.ClusterPodDataPoint {
	trend := make([]types.ClusterPodDataPoint, 0, len(values))
	for _, v := range values {
		trend = append(trend, types.ClusterPodDataPoint{
			Timestamp: v.Timestamp,
			Running:   int64(v.Value),
		})
	}
	return trend
}
