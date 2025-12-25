package types

import "time"

// ==================== Flagger 级别类型 ====================

// FlaggerMetrics Flagger 综合指标
type FlaggerMetrics struct {
	Timestamp  time.Time               `json:"timestamp"`
	Canaries   []CanaryMetrics         `json:"canaries"`
	Statistics FlaggerStatistics       `json:"statistics"`
	Controller FlaggerControllerHealth `json:"controller"`
}

// ==================== Canary 指标 ====================

// CanaryMetrics 单个 Canary 指标
type CanaryMetrics struct {
	Name           string            `json:"name"`
	Namespace      string            `json:"namespace"`
	Target         string            `json:"target"`
	Status         CanaryStatus      `json:"status"`
	Progress       CanaryProgress    `json:"progress"`
	Analysis       CanaryAnalysis    `json:"analysis"`
	Comparison     CanaryComparison  `json:"comparison"` // Canary vs Primary 对比
	Trend          []CanaryDataPoint `json:"trend"`
	Duration       CanaryDuration    `json:"duration"`
	LastUpdateTime time.Time         `json:"lastUpdateTime"`
}

// CanaryStatus Canary 状态
type CanaryStatus struct {
	Phase          string    `json:"phase"`     // Initialized, Waiting, Progressing, Promoting, Finalising, Succeeded, Failed
	PhaseCode      int       `json:"phaseCode"` // 0-6
	LastTransition time.Time `json:"lastTransition"`
}

// CanaryProgress Canary 进度
type CanaryProgress struct {
	CurrentWeight   float64                   `json:"currentWeight"`   // 当前流量权重 (0-100)
	TargetWeight    float64                   `json:"targetWeight"`    // 目标流量权重
	Iteration       int64                     `json:"iteration"`       // 当前迭代次数
	ProgressPercent float64                   `json:"progressPercent"` // 进度百分比
	Trend           []CanaryProgressDataPoint `json:"trend"`           // 权重变化趋势
}

// CanaryProgressDataPoint 进度数据点
type CanaryProgressDataPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Weight    float64   `json:"weight"`
	Iteration int64     `json:"iteration"`
}

// CanaryAnalysis Canary 分析结果
type CanaryAnalysis struct {
	MetricChecks     []MetricCheckResult       `json:"metricChecks"`
	CheckSuccessRate float64                   `json:"checkSuccessRate"`
	TotalChecks      int64                     `json:"totalChecks"`
	FailedChecks     int64                     `json:"failedChecks"`
	Trend            []CanaryAnalysisDataPoint `json:"trend"`
}

// MetricCheckResult 指标检查结果
type MetricCheckResult struct {
	MetricName   string    `json:"metricName"`
	ChecksPassed int64     `json:"checksPassed"`
	ChecksFailed int64     `json:"checksFailed"`
	SuccessRate  float64   `json:"successRate"`
	LastCheck    time.Time `json:"lastCheck"`
}

// CanaryAnalysisDataPoint 分析数据点
type CanaryAnalysisDataPoint struct {
	Timestamp        time.Time `json:"timestamp"`
	CheckSuccessRate float64   `json:"checkSuccessRate"`
	FailedChecks     int64     `json:"failedChecks"`
}

// CanaryComparison Canary vs Primary 对比
type CanaryComparison struct {
	CanaryMetrics  WorkloadMetrics             `json:"canaryMetrics"`
	PrimaryMetrics WorkloadMetrics             `json:"primaryMetrics"`
	Trend          []CanaryComparisonDataPoint `json:"trend"`
}

// WorkloadMetrics 工作负载指标（从 Mesh/Ingress 获取）
type WorkloadMetrics struct {
	SuccessRate float64 `json:"successRate"`
	ErrorRate   float64 `json:"errorRate"`
	P50Latency  float64 `json:"p50Latency"`
	P95Latency  float64 `json:"p95Latency"`
	P99Latency  float64 `json:"p99Latency"`
	RequestRate float64 `json:"requestRate"`
}

// CanaryComparisonDataPoint 对比数据点
type CanaryComparisonDataPoint struct {
	Timestamp          time.Time `json:"timestamp"`
	CanarySuccessRate  float64   `json:"canarySuccessRate"`
	PrimarySuccessRate float64   `json:"primarySuccessRate"`
	CanaryP95Latency   float64   `json:"canaryP95Latency"`
	PrimaryP95Latency  float64   `json:"primaryP95Latency"`
	CanaryErrorRate    float64   `json:"canaryErrorRate"`
	PrimaryErrorRate   float64   `json:"primaryErrorRate"`
}

// CanaryDuration Canary 持续时间
type CanaryDuration struct {
	StartTime       time.Time  `json:"startTime"`
	EndTime         *time.Time `json:"endTime,omitempty"`
	DurationSeconds float64    `json:"durationSeconds"`
}

// CanaryDataPoint Canary 数据点（用于状态趋势）
type CanaryDataPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Phase     string    `json:"phase"`
	PhaseCode int       `json:"phaseCode"`
	Weight    float64   `json:"weight"`
	Iteration int64     `json:"iteration"`
}

// ==================== Flagger 统计 ====================

// FlaggerStatistics Flagger 统计
type FlaggerStatistics struct {
	TotalCanaries   int64                        `json:"totalCanaries"`
	ActiveCanaries  int64                        `json:"activeCanaries"`  // Progressing 状态
	WaitingCanaries int64                        `json:"waitingCanaries"` // Waiting 状态
	SuccessCanaries int64                        `json:"successCanaries"` // Succeeded 状态
	FailedCanaries  int64                        `json:"failedCanaries"`  // Failed 状态
	SuccessRate     float64                      `json:"successRate"`
	FailureRate     float64                      `json:"failureRate"`
	Duration        FlaggerDurationStatistics    `json:"duration"`
	WebhookStats    FlaggerWebhookStatistics     `json:"webhookStats"`
	Trend           []FlaggerStatisticsDataPoint `json:"trend"`
}

// FlaggerDurationStatistics 持续时间统计
type FlaggerDurationStatistics struct {
	AvgDurationSeconds float64                    `json:"avgDurationSeconds"`
	MinDurationSeconds float64                    `json:"minDurationSeconds"`
	MaxDurationSeconds float64                    `json:"maxDurationSeconds"`
	P50DurationSeconds float64                    `json:"p50DurationSeconds"`
	P95DurationSeconds float64                    `json:"p95DurationSeconds"`
	P99DurationSeconds float64                    `json:"p99DurationSeconds"`
	Trend              []FlaggerDurationDataPoint `json:"trend"`
}

// FlaggerDurationDataPoint 持续时间数据点
type FlaggerDurationDataPoint struct {
	Timestamp          time.Time `json:"timestamp"`
	AvgDurationSeconds float64   `json:"avgDurationSeconds"`
	P95DurationSeconds float64   `json:"p95DurationSeconds"`
}

// FlaggerWebhookStatistics Webhook 统计
type FlaggerWebhookStatistics struct {
	TotalRequests   int64                     `json:"totalRequests"`
	SuccessRequests int64                     `json:"successRequests"`
	FailedRequests  int64                     `json:"failedRequests"`
	SuccessRate     float64                   `json:"successRate"`
	FailureRate     float64                   `json:"failureRate"`
	Trend           []FlaggerWebhookDataPoint `json:"trend"`
}

// FlaggerWebhookDataPoint Webhook 数据点
type FlaggerWebhookDataPoint struct {
	Timestamp     time.Time `json:"timestamp"`
	TotalRequests int64     `json:"totalRequests"`
	SuccessRate   float64   `json:"successRate"`
}

// FlaggerStatisticsDataPoint 统计数据点
type FlaggerStatisticsDataPoint struct {
	Timestamp       time.Time `json:"timestamp"`
	TotalCanaries   int64     `json:"totalCanaries"`
	ActiveCanaries  int64     `json:"activeCanaries"`
	SuccessCanaries int64     `json:"successCanaries"`
	FailedCanaries  int64     `json:"failedCanaries"`
	SuccessRate     float64   `json:"successRate"`
}

// ==================== Flagger Controller 健康 ====================

// FlaggerControllerHealth Flagger Controller 健康
type FlaggerControllerHealth struct {
	ReconcileTotal     int64                        `json:"reconcileTotal"`
	ReconcileErrors    int64                        `json:"reconcileErrors"`
	ReconcileErrorRate float64                      `json:"reconcileErrorRate"`
	AvgReconcileTime   float64                      `json:"avgReconcileTime"` // 平均 Reconcile 时间（秒）
	P95ReconcileTime   float64                      `json:"p95ReconcileTime"` // P95 Reconcile 时间（秒）
	Trend              []FlaggerControllerDataPoint `json:"trend"`
}

// FlaggerControllerDataPoint Controller 数据点
type FlaggerControllerDataPoint struct {
	Timestamp          time.Time `json:"timestamp"`
	ReconcileTotal     int64     `json:"reconcileTotal"`
	ReconcileErrorRate float64   `json:"reconcileErrorRate"`
	AvgReconcileTime   float64   `json:"avgReconcileTime"`
}

// ==================== Canary 列表和摘要 ====================

// CanaryBrief Canary 简要信息
type CanaryBrief struct {
	Name          string    `json:"name"`
	Namespace     string    `json:"namespace"`
	Target        string    `json:"target"`
	Phase         string    `json:"phase"`
	CurrentWeight float64   `json:"currentWeight"`
	Iteration     int64     `json:"iteration"`
	LastUpdate    time.Time `json:"lastUpdate"`
}

// CanaryListMetrics Canary 列表指标
type CanaryListMetrics struct {
	Timestamp time.Time         `json:"timestamp"`
	Canaries  []CanaryBrief     `json:"canaries"`
	Summary   CanaryListSummary `json:"summary"`
}

// CanaryListSummary Canary 列表摘要
type CanaryListSummary struct {
	Total       int64 `json:"total"`
	Active      int64 `json:"active"`
	Waiting     int64 `json:"waiting"`
	Succeeded   int64 `json:"succeeded"`
	Failed      int64 `json:"failed"`
	Initialized int64 `json:"initialized"`
}

// ==================== Namespace 级别 Canary 统计 ====================

// NamespaceCanaryMetrics Namespace 级别 Canary 指标
type NamespaceCanaryMetrics struct {
	Namespace  string                    `json:"namespace"`
	Timestamp  time.Time                 `json:"timestamp"`
	Canaries   []CanaryBrief             `json:"canaries"`
	Statistics NamespaceCanaryStatistics `json:"statistics"`
}

// NamespaceCanaryStatistics Namespace Canary 统计
type NamespaceCanaryStatistics struct {
	Total       int64   `json:"total"`
	Active      int64   `json:"active"`
	SuccessRate float64 `json:"successRate"`
	AvgDuration float64 `json:"avgDuration"`
}

// ==================== Canary 排行 ====================

// CanaryRanking Canary 排行
type CanaryRanking struct {
	TopByDuration     []CanaryRankingItem `json:"topByDuration"`
	TopByIterations   []CanaryRankingItem `json:"topByIterations"`
	TopByFailedChecks []CanaryRankingItem `json:"topByFailedChecks"`
	RecentFailed      []CanaryBrief       `json:"recentFailed"`
	RecentSucceeded   []CanaryBrief       `json:"recentSucceeded"`
}

// CanaryRankingItem Canary 排行项
type CanaryRankingItem struct {
	Namespace string  `json:"namespace"`
	Name      string  `json:"name"`
	Value     float64 `json:"value"`
	Unit      string  `json:"unit"`
}

// ==================== 历史记录 ====================

// CanaryHistory Canary 历史记录
type CanaryHistory struct {
	Namespace string                `json:"namespace"`
	Name      string                `json:"name"`
	Records   []CanaryHistoryRecord `json:"records"`
}

// CanaryHistoryRecord Canary 历史记录项
type CanaryHistoryRecord struct {
	StartTime       time.Time `json:"startTime"`
	EndTime         time.Time `json:"endTime"`
	DurationSeconds float64   `json:"durationSeconds"`
	FinalPhase      string    `json:"finalPhase"` // Succeeded, Failed
	MaxWeight       float64   `json:"maxWeight"`
	Iterations      int64     `json:"iterations"`
	FailedChecks    int64     `json:"failedChecks"`
}

// ==================== 实时监控 ====================

// CanaryRealtimeMetrics Canary 实时监控指标
type CanaryRealtimeMetrics struct {
	Namespace     string              `json:"namespace"`
	Name          string              `json:"name"`
	Timestamp     time.Time           `json:"timestamp"`
	CurrentStatus CanaryStatus        `json:"currentStatus"`
	CurrentWeight float64             `json:"currentWeight"`
	Comparison    CanaryComparison    `json:"comparison"`
	RecentChecks  []MetricCheckResult `json:"recentChecks"`
}

// ==================== FlaggerOperator 接口 ====================

// FlaggerOperator Flagger 操作器接口
type FlaggerOperator interface {
	// 综合查询
	GetFlaggerMetrics(timeRange *TimeRange) (*FlaggerMetrics, error)

	// Canary 查询
	GetCanaryMetrics(namespace, name string, timeRange *TimeRange) (*CanaryMetrics, error)
	GetCanaryStatus(namespace, name string) (*CanaryStatus, error)
	GetCanaryProgress(namespace, name string, timeRange *TimeRange) (*CanaryProgress, error)
	GetCanaryAnalysis(namespace, name string, timeRange *TimeRange) (*CanaryAnalysis, error)
	GetCanaryComparison(namespace, name string, timeRange *TimeRange) (*CanaryComparison, error)
	GetCanaryDuration(namespace, name string) (*CanaryDuration, error)

	// Canary 列表
	ListCanaries(namespace string) (*CanaryListMetrics, error)
	ListAllCanaries() (*CanaryListMetrics, error)

	// 统计查询
	GetFlaggerStatistics(timeRange *TimeRange) (*FlaggerStatistics, error)
	GetFlaggerDurationStatistics(timeRange *TimeRange) (*FlaggerDurationStatistics, error)
	GetFlaggerWebhookStatistics(timeRange *TimeRange) (*FlaggerWebhookStatistics, error)

	// Controller 查询
	GetFlaggerControllerHealth(timeRange *TimeRange) (*FlaggerControllerHealth, error)

	// Namespace 级别查询
	GetNamespaceCanaryMetrics(namespace string, timeRange *TimeRange) (*NamespaceCanaryMetrics, error)

	// 排行查询
	GetCanaryRanking(limit int, timeRange *TimeRange) (*CanaryRanking, error)

	// 历史记录
	GetCanaryHistory(namespace, name string, timeRange *TimeRange) (*CanaryHistory, error)

	// 实时监控
	GetCanaryRealtimeMetrics(namespace, name string) (*CanaryRealtimeMetrics, error)
}
