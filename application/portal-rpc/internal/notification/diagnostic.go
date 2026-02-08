package notification

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
)

// DiagnosticInfo 诊断信息
type DiagnosticInfo struct {
	// 系统信息
	SystemInfo SystemInfo `json:"systemInfo"`
	// 聚合器信息
	AggregatorInfo *AggregatorDiagnostic `json:"aggregatorInfo,omitempty"`
	// 通道信息
	ChannelsInfo []ChannelDiagnostic `json:"channelsInfo"`
	// 指标信息
	MetricsInfo MetricsInfo `json:"metricsInfo"`
	// 健康状态
	OverallHealth HealthSummary `json:"overallHealth"`
	// 诊断时间
	DiagnosticTime time.Time `json:"diagnosticTime"`
}

// SystemInfo 系统信息
type SystemInfo struct {
	GoVersion    string    `json:"goVersion"`
	NumGoroutine int       `json:"numGoroutine"`
	NumCPU       int       `json:"numCpu"`
	MemStats     MemStats  `json:"memStats"`
	StartTime    time.Time `json:"startTime"`
	Uptime       string    `json:"uptime"`
}

// MemStats 内存统计
type MemStats struct {
	Alloc        uint64 `json:"alloc"`        // 当前分配的内存 (bytes)
	TotalAlloc   uint64 `json:"totalAlloc"`   // 累计分配的内存 (bytes)
	Sys          uint64 `json:"sys"`          // 系统分配的内存 (bytes)
	NumGC        uint32 `json:"numGC"`        // GC 次数
	PauseTotalNs uint64 `json:"pauseTotalNs"` // GC 暂停总时间 (nanoseconds)
}

// AggregatorDiagnostic 聚合器诊断信息
type AggregatorDiagnostic struct {
	Enabled             bool                   `json:"enabled"`
	IsLeader            bool                   `json:"isLeader"`
	LeaderID            string                 `json:"leaderId"`
	LeaderType          string                 `json:"leaderType"` // "k8s", "redis", "standalone"
	LastLeadershipCheck time.Time              `json:"lastLeadershipCheck"`
	LeadershipFailures  int                    `json:"leadershipFailures"`
	MaxLeaderFailures   int                    `json:"maxLeaderFailures"`
	ProcessInterval     string                 `json:"processInterval"`
	SeverityWindows     map[string]string      `json:"severityWindows"`
	Stats               map[string]int64       `json:"stats"`
	HealthInfo          map[string]interface{} `json:"healthInfo"`
}

// ChannelDiagnostic 通道诊断信息
type ChannelDiagnostic struct {
	UUID         string                 `json:"uuid"`
	Name         string                 `json:"name"`
	Type         string                 `json:"type"`
	Enabled      bool                   `json:"enabled"`
	Healthy      bool                   `json:"healthy"`
	LastCheck    time.Time              `json:"lastCheck"`
	ErrorMessage string                 `json:"errorMessage,omitempty"`
	Config       map[string]interface{} `json:"config,omitempty"`
	Stats        map[string]interface{} `json:"stats,omitempty"`
}

// MetricsInfo 指标信息
type MetricsInfo struct {
	InstanceStats    map[string]*InstanceStats `json:"instanceStats"`
	PrometheusLabels []string                  `json:"prometheusLabels"`
	MetricsEnabled   bool                      `json:"metricsEnabled"`
}

// HealthSummary 健康状态摘要
type HealthSummary struct {
	Overall          string            `json:"overall"` // "healthy", "degraded", "unhealthy"
	ComponentsHealth map[string]string `json:"componentsHealth"`
	Issues           []string          `json:"issues,omitempty"`
	Recommendations  []string          `json:"recommendations,omitempty"`
}

// DiagnosticService 诊断服务
type DiagnosticService struct {
	manager           *Manager
	aggregatorService *AggregatorService
	startTime         time.Time
	logger            logx.Logger
}

// NewDiagnosticService 创建诊断服务
func NewDiagnosticService(manager *Manager, aggregatorService *AggregatorService) *DiagnosticService {
	return &DiagnosticService{
		manager:           manager,
		aggregatorService: aggregatorService,
		startTime:         time.Now(),
		logger:            logx.WithContext(context.Background()),
	}
}

// GetDiagnosticInfo 获取完整的诊断信息
func (ds *DiagnosticService) GetDiagnosticInfo(ctx context.Context) (*DiagnosticInfo, error) {
	info := &DiagnosticInfo{
		DiagnosticTime: time.Now(),
	}

	// 收集系统信息
	info.SystemInfo = ds.collectSystemInfo()

	// 收集聚合器信息
	if ds.aggregatorService != nil {
		info.AggregatorInfo = ds.collectAggregatorInfo()
	}

	// 收集通道信息
	info.ChannelsInfo = ds.collectChannelsInfo(ctx)

	// 收集指标信息
	info.MetricsInfo = ds.collectMetricsInfo()

	// 评估整体健康状态
	info.OverallHealth = ds.evaluateOverallHealth(info)

	return info, nil
}

// collectSystemInfo 收集系统信息
func (ds *DiagnosticService) collectSystemInfo() SystemInfo {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	return SystemInfo{
		GoVersion:    runtime.Version(),
		NumGoroutine: runtime.NumGoroutine(),
		NumCPU:       runtime.NumCPU(),
		MemStats: MemStats{
			Alloc:        memStats.Alloc,
			TotalAlloc:   memStats.TotalAlloc,
			Sys:          memStats.Sys,
			NumGC:        memStats.NumGC,
			PauseTotalNs: memStats.PauseTotalNs,
		},
		StartTime: ds.startTime,
		Uptime:    time.Since(ds.startTime).String(),
	}
}

// collectAggregatorInfo 收集聚合器信息
func (ds *DiagnosticService) collectAggregatorInfo() *AggregatorDiagnostic {
	if ds.aggregatorService == nil {
		return nil
	}

	healthInfo := ds.aggregatorService.GetHealthInfo()
	aggregator := ds.aggregatorService.GetAggregator()

	diagnostic := &AggregatorDiagnostic{
		Enabled:    ds.aggregatorService.config.Enabled,
		LeaderType: "unknown",
	}

	if ds.aggregatorService.useK8sLeader {
		diagnostic.LeaderType = "k8s"
	} else if ds.aggregatorService.useRedisLeader {
		diagnostic.LeaderType = "redis"
	} else {
		diagnostic.LeaderType = "standalone"
	}

	if aggregator != nil {
		stats := aggregator.GetStats()
		diagnostic.Stats = stats
		diagnostic.IsLeader = stats["isLeader"] == 1
		diagnostic.LeaderID = aggregator.leaderID
		diagnostic.LastLeadershipCheck = aggregator.lastLeadershipCheck
		diagnostic.LeadershipFailures = aggregator.leadershipFailures
		diagnostic.MaxLeaderFailures = aggregator.maxLeaderFailures
		diagnostic.ProcessInterval = aggregator.processInterval.String()
		diagnostic.SeverityWindows = map[string]string{
			"critical": aggregator.config.SeverityWindows.Critical.String(),
			"warning":  aggregator.config.SeverityWindows.Warning.String(),
			"info":     aggregator.config.SeverityWindows.Info.String(),
			"default":  aggregator.config.SeverityWindows.Default.String(),
		}
	}

	diagnostic.HealthInfo = healthInfo

	return diagnostic
}

// collectChannelsInfo 收集通道信息
func (ds *DiagnosticService) collectChannelsInfo(ctx context.Context) []ChannelDiagnostic {
	var channels []ChannelDiagnostic

	if ds.manager == nil {
		return channels
	}

	// 这里需要根据实际的 Manager 实现来获取通道信息
	// 由于我们没有看到 Manager 的完整实现，这里提供一个框架
	// 实际实现时需要遍历所有注册的通道并收集其诊断信息

	// 示例代码（需要根据实际实现调整）:
	/*
		for _, channel := range ds.manager.GetAllChannels() {
			diagnostic := ChannelDiagnostic{
				UUID:    channel.GetUUID(),
				Name:    channel.GetName(),
				Type:    channel.GetType(),
				Enabled: channel.IsEnabled(),
			}

			// 执行健康检查
			if healthChecker, ok := channel.(HealthChecker); ok {
				health, err := healthChecker.HealthCheck(ctx)
				if err != nil {
					diagnostic.Healthy = false
					diagnostic.ErrorMessage = err.Error()
				} else {
					diagnostic.Healthy = health.Healthy
					diagnostic.LastCheck = health.LastCheckTime
				}
			}

			channels = append(channels, diagnostic)
		}
	*/

	return channels
}

// collectMetricsInfo 收集指标信息
func (ds *DiagnosticService) collectMetricsInfo() MetricsInfo {
	return MetricsInfo{
		InstanceStats:    GlobalMetricsCollector.GetAllInstanceStats(),
		PrometheusLabels: []string{"channel_type", "status", "project_name", "instance_id"},
		MetricsEnabled:   true,
	}
}

// evaluateOverallHealth 评估整体健康状态
func (ds *DiagnosticService) evaluateOverallHealth(info *DiagnosticInfo) HealthSummary {
	summary := HealthSummary{
		Overall:          "healthy",
		ComponentsHealth: make(map[string]string),
		Issues:           []string{},
		Recommendations:  []string{},
	}

	// 检查聚合器健康状态
	if info.AggregatorInfo != nil {
		if info.AggregatorInfo.Enabled {
			if info.AggregatorInfo.LeadershipFailures > 0 {
				summary.ComponentsHealth["aggregator"] = "degraded"
				summary.Issues = append(summary.Issues, fmt.Sprintf("聚合器 Leader 选举失败 %d 次", info.AggregatorInfo.LeadershipFailures))
				if info.AggregatorInfo.LeadershipFailures >= info.AggregatorInfo.MaxLeaderFailures {
					summary.Overall = "degraded"
					summary.Recommendations = append(summary.Recommendations, "检查 Redis 连接或 K8s 集群状态")
				}
			} else {
				summary.ComponentsHealth["aggregator"] = "healthy"
			}
		} else {
			summary.ComponentsHealth["aggregator"] = "disabled"
		}
	}

	// 检查通道健康状态
	healthyChannels := 0
	totalChannels := len(info.ChannelsInfo)
	for _, channel := range info.ChannelsInfo {
		if channel.Healthy {
			healthyChannels++
		} else {
			summary.Issues = append(summary.Issues, fmt.Sprintf("通道 %s (%s) 不健康: %s", channel.Name, channel.Type, channel.ErrorMessage))
		}
	}

	if totalChannels > 0 {
		healthRatio := float64(healthyChannels) / float64(totalChannels)
		if healthRatio == 1.0 {
			summary.ComponentsHealth["channels"] = "healthy"
		} else if healthRatio >= 0.5 {
			summary.ComponentsHealth["channels"] = "degraded"
			summary.Overall = "degraded"
		} else {
			summary.ComponentsHealth["channels"] = "unhealthy"
			summary.Overall = "unhealthy"
		}
	}

	// 检查系统资源
	memUsageMB := float64(info.SystemInfo.MemStats.Alloc) / 1024 / 1024
	if memUsageMB > 1000 { // 超过 1GB
		summary.Issues = append(summary.Issues, fmt.Sprintf("内存使用量较高: %.2f MB", memUsageMB))
		summary.Recommendations = append(summary.Recommendations, "考虑优化内存使用或增加系统内存")
	}

	if info.SystemInfo.NumGoroutine > 1000 {
		summary.Issues = append(summary.Issues, fmt.Sprintf("Goroutine 数量较多: %d", info.SystemInfo.NumGoroutine))
		summary.Recommendations = append(summary.Recommendations, "检查是否存在 Goroutine 泄漏")
	}

	// 根据问题数量调整整体状态
	if len(summary.Issues) == 0 {
		summary.Overall = "healthy"
	} else if len(summary.Issues) <= 2 {
		if summary.Overall == "healthy" {
			summary.Overall = "degraded"
		}
	} else {
		summary.Overall = "unhealthy"
	}

	return summary
}

// GetQuickHealthCheck 快速健康检查
func (ds *DiagnosticService) GetQuickHealthCheck(ctx context.Context) (*HealthSummary, error) {
	info, err := ds.GetDiagnosticInfo(ctx)
	if err != nil {
		return nil, err
	}

	return &info.OverallHealth, nil
}
