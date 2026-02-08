package types

import "time"

// ==================== Ingress 级别类型 ====================

// IngressMetrics Ingress 综合指标
type IngressMetrics struct {
	Namespace    string                    `json:"namespace"`
	IngressName  string                    `json:"ingressName"`
	Timestamp    time.Time                 `json:"timestamp"`
	Controller   IngressControllerHealth   `json:"controller"`
	Traffic      IngressTrafficMetrics     `json:"traffic"`
	Performance  IngressPerformanceMetrics `json:"performance"`
	Errors       IngressErrorMetrics       `json:"errors"`
	Backends     IngressBackendMetrics     `json:"backends"`
	Certificates IngressCertificateMetrics `json:"certificates"`
}

// ==================== Controller 健康 ====================

// IngressControllerHealth Controller 健康状态
type IngressControllerHealth struct {
	ControllerName    string                       `json:"controllerName"`
	TotalPods         int64                        `json:"totalPods"`
	RunningPods       int64                        `json:"runningPods"`
	ReadyPods         int64                        `json:"readyPods"`
	CPUUsage          float64                      `json:"cpuUsage"`
	MemoryUsage       int64                        `json:"memoryUsage"`
	LastReloadSuccess bool                         `json:"lastReloadSuccess"`
	LastReloadTime    time.Time                    `json:"lastReloadTime"`
	ReloadRate        float64                      `json:"reloadRate"`
	ReloadFailRate    float64                      `json:"reloadFailRate"`
	Connections       IngressConnectionMetrics     `json:"connections"`
	Trend             []IngressControllerDataPoint `json:"trend"`
}

// IngressConnectionMetrics 连接指标
type IngressConnectionMetrics struct {
	Active  int64 `json:"active"`
	Reading int64 `json:"reading"`
	Writing int64 `json:"writing"`
	Waiting int64 `json:"waiting"`
	Total   int64 `json:"total"`
}

// IngressControllerDataPoint Controller 数据点
type IngressControllerDataPoint struct {
	Timestamp         time.Time `json:"timestamp"`
	ActiveConnections int64     `json:"activeConnections"`
	ReloadRate        float64   `json:"reloadRate"`
	CPUUsage          float64   `json:"cpuUsage"`
	MemoryUsage       int64     `json:"memoryUsage"`
}

// ==================== 流量指标 ====================

// IngressTrafficMetrics 流量指标
type IngressTrafficMetrics struct {
	Current   IngressTrafficSnapshot    `json:"current"`
	Trend     []IngressTrafficDataPoint `json:"trend"`
	ByHost    []TrafficByDimension      `json:"byHost"`
	ByPath    []TrafficByDimension      `json:"byPath"`
	ByService []TrafficByDimension      `json:"byService"`
	ByMethod  []TrafficByMethod         `json:"byMethod"`
	Summary   IngressTrafficSummary     `json:"summary"`
}

// IngressTrafficSnapshot 流量快照
type IngressTrafficSnapshot struct {
	Timestamp          time.Time `json:"timestamp"`
	RequestsPerSecond  float64   `json:"requestsPerSecond"`
	ActiveConnections  int64     `json:"activeConnections"`
	IngressBytesPerSec float64   `json:"ingressBytesPerSec"`
	EgressBytesPerSec  float64   `json:"egressBytesPerSec"`
}

// IngressTrafficDataPoint 流量数据点
type IngressTrafficDataPoint struct {
	Timestamp          time.Time `json:"timestamp"`
	RequestsPerSecond  float64   `json:"requestsPerSecond"`
	IngressBytesPerSec float64   `json:"ingressBytesPerSec"`
	EgressBytesPerSec  float64   `json:"egressBytesPerSec"`
}

// TrafficByDimension 按维度的流量
type TrafficByDimension struct {
	Name              string  `json:"name"` // Host/Path/Service 名称
	Namespace         string  `json:"namespace,omitempty"`
	RequestsPerSecond float64 `json:"requestsPerSecond"`
	BytesPerSecond    float64 `json:"bytesPerSecond"`
}

// TrafficByMethod 按 HTTP 方法的流量
type TrafficByMethod struct {
	Method            string  `json:"method"` // GET, POST, PUT, DELETE, PATCH
	RequestsPerSecond float64 `json:"requestsPerSecond"`
}

// IngressTrafficSummary 流量统计
type IngressTrafficSummary struct {
	TotalRequests     int64   `json:"totalRequests"`
	TotalIngressBytes int64   `json:"totalIngressBytes"`
	TotalEgressBytes  int64   `json:"totalEgressBytes"`
	AvgRequestsPerSec float64 `json:"avgRequestsPerSec"`
	MaxRequestsPerSec float64 `json:"maxRequestsPerSec"`
	AvgRequestSize    float64 `json:"avgRequestSize"`
	AvgResponseSize   float64 `json:"avgResponseSize"`
}

// ==================== 性能指标 ====================

// IngressPerformanceMetrics 性能指标
type IngressPerformanceMetrics struct {
	Overall         IngressLatencyStats       `json:"overall"`
	ByHost          []LatencyByDimension      `json:"byHost"`
	ByPath          []LatencyByDimension      `json:"byPath"`
	UpstreamLatency IngressLatencyStats       `json:"upstreamLatency"`
	Trend           []IngressLatencyDataPoint `json:"trend"`
}

// IngressLatencyStats 延迟统计
type IngressLatencyStats struct {
	P50 float64 `json:"p50"`
	P95 float64 `json:"p95"`
	P99 float64 `json:"p99"`
	Avg float64 `json:"avg"`
	Max float64 `json:"max"`
}

// LatencyByDimension 按维度的延迟
type LatencyByDimension struct {
	Name      string              `json:"name"`
	Namespace string              `json:"namespace,omitempty"`
	Latency   IngressLatencyStats `json:"latency"`
}

// IngressLatencyDataPoint 延迟数据点
type IngressLatencyDataPoint struct {
	Timestamp time.Time `json:"timestamp"`
	P50       float64   `json:"p50"`
	P95       float64   `json:"p95"`
	P99       float64   `json:"p99"`
}

// ==================== 错误指标 ====================

// IngressErrorMetrics 错误指标
type IngressErrorMetrics struct {
	Overall     IngressErrorRateStats         `json:"overall"`
	StatusCodes IngressStatusCodeDistribution `json:"statusCodes"`
	ByHost      []ErrorRateByDimension        `json:"byHost"`
	ByPath      []ErrorRateByDimension        `json:"byPath"`
	Trend       []IngressErrorDataPoint       `json:"trend"`
}

// IngressErrorRateStats 错误率统计
type IngressErrorRateStats struct {
	TotalErrorRate float64 `json:"totalErrorRate"`
	Error4xxRate   float64 `json:"error4xxRate"`
	Error5xxRate   float64 `json:"error5xxRate"`
}

// IngressStatusCodeDistribution 状态码分布
type IngressStatusCodeDistribution struct {
	Status2xx map[string]float64 `json:"status2xx"` // "200": rate, "201": rate
	Status3xx map[string]float64 `json:"status3xx"` // "301": rate, "302": rate
	Status4xx map[string]float64 `json:"status4xx"` // "400": rate, "404": rate
	Status5xx map[string]float64 `json:"status5xx"` // "500": rate, "502": rate
}

// ErrorRateByDimension 按维度的错误率
type ErrorRateByDimension struct {
	Name      string                `json:"name"`
	Namespace string                `json:"namespace,omitempty"`
	ErrorRate IngressErrorRateStats `json:"errorRate"`
	TopErrors []StatusCodeRate      `json:"topErrors"`
}

// StatusCodeRate 状态码速率
type StatusCodeRate struct {
	StatusCode string  `json:"statusCode"`
	Rate       float64 `json:"rate"`
	Percent    float64 `json:"percent"`
}

// IngressErrorDataPoint 错误数据点
type IngressErrorDataPoint struct {
	Timestamp      time.Time `json:"timestamp"`
	TotalErrorRate float64   `json:"totalErrorRate"`
	Error4xxRate   float64   `json:"error4xxRate"`
	Error5xxRate   float64   `json:"error5xxRate"`
}

// ==================== 后端健康 ====================

// IngressBackendMetrics 后端指标
type IngressBackendMetrics struct {
	UpstreamLatency    float64               `json:"upstreamLatency"`
	EndpointsByService []ServiceEndpoints    `json:"endpointsByService"`
	BackendHealth      []BackendHealthStatus `json:"backendHealth"`
}

// ServiceEndpoints Service Endpoints
type ServiceEndpoints struct {
	ServiceName        string `json:"serviceName"`
	Namespace          string `json:"namespace"`
	AvailableEndpoints int64  `json:"availableEndpoints"`
	HasEndpoints       bool   `json:"hasEndpoints"`
}

// BackendHealthStatus 后端健康状态
type BackendHealthStatus struct {
	Upstream          string  `json:"upstream"`
	ResponseLatency   float64 `json:"responseLatency"`
	SuccessRate       float64 `json:"successRate"`
	ActiveConnections int64   `json:"activeConnections"`
}

// ==================== 证书指标 ====================

// IngressCertificateMetrics 证书指标
type IngressCertificateMetrics struct {
	HTTPSRequestPercent float64           `json:"httpsRequestPercent"`
	Certificates        []CertificateInfo `json:"certificates"`
	ExpiringCount       int64             `json:"expiringCount"` // 即将过期（< 30 天）
	ExpiredCount        int64             `json:"expiredCount"`
}

// CertificateInfo 证书信息
type CertificateInfo struct {
	Name           string    `json:"name"`
	Namespace      string    `json:"namespace"`
	ExpirationTime time.Time `json:"expirationTime"`
	DaysRemaining  int       `json:"daysRemaining"`
	IsExpiring     bool      `json:"isExpiring"` // < 30 天
	IsExpired      bool      `json:"isExpired"`
}

// ==================== Ingress 对象状态 ====================

// IngressObjectMetrics Ingress 对象指标
type IngressObjectMetrics struct {
	Namespace   string            `json:"namespace"`
	IngressName string            `json:"ingressName"`
	PathCount   int64             `json:"pathCount"`
	RuleCount   int64             `json:"ruleCount"`
	TLSEnabled  bool              `json:"tlsEnabled"`
	Hosts       []string          `json:"hosts"`
	Paths       []IngressPathInfo `json:"paths"`
	CreatedAt   time.Time         `json:"createdAt"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
}

// IngressPathInfo Ingress 路径信息
type IngressPathInfo struct {
	Host        string `json:"host"`
	Path        string `json:"path"`
	PathType    string `json:"pathType"`
	ServiceName string `json:"serviceName"`
	ServicePort int32  `json:"servicePort"`
}

// ==================== 限流指标 ====================

// IngressRateLimitMetrics 限流指标
type IngressRateLimitMetrics struct {
	Namespace        string                      `json:"namespace"`
	IngressName      string                      `json:"ingressName"`
	LimitedRequests  float64                     `json:"limitedRequests"`  // 被限流的请求速率
	LimitTriggerRate float64                     `json:"limitTriggerRate"` // 限流触发率
	ByPath           []RateLimitByPath           `json:"byPath"`
	Trend            []IngressRateLimitDataPoint `json:"trend"`
}

// RateLimitByPath 按路径的限流情况
type RateLimitByPath struct {
	Path                string  `json:"path"`
	LimitedRequests     float64 `json:"limitedRequests"`
	TotalRequests       float64 `json:"totalRequests"`
	LimitTriggerPercent float64 `json:"limitTriggerPercent"`
}

// IngressRateLimitDataPoint 限流数据点
type IngressRateLimitDataPoint struct {
	Timestamp        time.Time `json:"timestamp"`
	LimitedRequests  float64   `json:"limitedRequests"`
	LimitTriggerRate float64   `json:"limitTriggerRate"`
}

// ==================== Ingress 排行 ====================

// IngressRanking Ingress 排行
type IngressRanking struct {
	TopByQPS       []IngressRankingItem `json:"topByQPS"`
	TopByErrorRate []IngressRankingItem `json:"topByErrorRate"`
	TopByLatency   []IngressRankingItem `json:"topByLatency"`
	TopByTraffic   []IngressRankingItem `json:"topByTraffic"`
}

// IngressRankingItem Ingress 排行项
type IngressRankingItem struct {
	Namespace   string  `json:"namespace"`
	IngressName string  `json:"ingressName"`
	Value       float64 `json:"value"`
	Unit        string  `json:"unit"`
}

// PathRanking Path 排行
type PathRanking struct {
	TopByQPS       []PathRankingItem `json:"topByQPS"`
	TopByErrorRate []PathRankingItem `json:"topByErrorRate"`
	TopByLatency   []PathRankingItem `json:"topByLatency"`
}

// PathRankingItem Path 排行项
type PathRankingItem struct {
	Host  string  `json:"host"`
	Path  string  `json:"path"`
	Value float64 `json:"value"`
	Unit  string  `json:"unit"`
}

// HostRanking Host 排行
type HostRanking struct {
	TopByQPS       []HostRankingItem `json:"topByQPS"`
	TopByErrorRate []HostRankingItem `json:"topByErrorRate"`
	TopByLatency   []HostRankingItem `json:"topByLatency"`
}

// HostRankingItem Host 排行项
type HostRankingItem struct {
	Host  string  `json:"host"`
	Value float64 `json:"value"`
	Unit  string  `json:"unit"`
}

// ==================== Host 级别监控 ====================

// HostMetricsDetail Host 级别监控详情
type HostMetricsDetail struct {
	Host         string                        `json:"host"`
	Namespace    string                        `json:"namespace"`
	IngressNames []string                      `json:"ingressNames"`
	Traffic      IngressTrafficMetrics         `json:"traffic"`
	Performance  IngressLatencyStats           `json:"performance"`
	Errors       IngressErrorRateStats         `json:"errors"`
	Backends     []BackendHealthStatus         `json:"backends"`
	StatusCodes  IngressStatusCodeDistribution `json:"statusCodes"`
}

// ==================== Host 级别监控新类型 ====================

// HostMetrics Host 级别综合指标
type HostMetrics struct {
	Host        string                     `json:"host"`
	Start       int64                      `json:"start"`
	End         int64                      `json:"end"`
	Current     HostMetricSnapshot         `json:"current"`
	StatusCodes HostStatusCodeDistribution `json:"statusCodes"`
	ErrorRate   HostErrorRateStats         `json:"errorRate"`
	Latency     HostLatencyStats           `json:"latency"`
	Summary     HostMetricsSummary         `json:"summary"`
	Trend       []HostMetricDataPoint      `json:"trend"`
	ByPath      []HostPathMetrics          `json:"byPath"`
}

// HostMetricSnapshot Host 当前快照
type HostMetricSnapshot struct {
	Timestamp          int64   `json:"timestamp"`
	RequestsPerSecond  float64 `json:"requestsPerSecond"`
	IngressBytesPerSec float64 `json:"ingressBytesPerSec"`
	EgressBytesPerSec  float64 `json:"egressBytesPerSec"`
	ActiveConnections  int64   `json:"activeConnections"`
}

// HostStatusCodeDistribution Host 状态码分布（返回数量而非速率）
type HostStatusCodeDistribution struct {
	Status2xx map[string]int64 `json:"status2xx"`
	Status3xx map[string]int64 `json:"status3xx"`
	Status4xx map[string]int64 `json:"status4xx"`
	Status5xx map[string]int64 `json:"status5xx"`
}

// HostErrorRateStats Host 错误率统计
type HostErrorRateStats struct {
	TotalErrorRate float64        `json:"totalErrorRate"`
	Error4xxRate   float64        `json:"error4xxRate"`
	Error5xxRate   float64        `json:"error5xxRate"`
	TopErrors      []HostTopError `json:"topErrors"`    // 所有错误的 Top 列表
	Top4xxErrors   []HostTopError `json:"top4xxErrors"` // 4xx 错误的 Top 列表
	Top5xxErrors   []HostTopError `json:"top5xxErrors"` // 5xx 错误的 Top 列表
}

// HostTopError Host Top 错误
type HostTopError struct {
	StatusCode string  `json:"statusCode"`
	Count      int64   `json:"count"`
	Percent    float64 `json:"percent"`
}

// HostLatencyStats Host 延迟统计
type HostLatencyStats struct {
	P50 float64 `json:"p50"`
	P95 float64 `json:"p95"`
	P99 float64 `json:"p99"`
	Avg float64 `json:"avg"`
	Max float64 `json:"max"`
}

// HostMetricsSummary Host 汇总统计
type HostMetricsSummary struct {
	TotalRequests     int64   `json:"totalRequests"`
	TotalIngressBytes int64   `json:"totalIngressBytes"`
	TotalEgressBytes  int64   `json:"totalEgressBytes"`
	AvgRequestsPerSec float64 `json:"avgRequestsPerSec"`
	MaxRequestsPerSec float64 `json:"maxRequestsPerSec"`
	AvgRequestSize    float64 `json:"avgRequestSize"`
	AvgResponseSize   float64 `json:"avgResponseSize"`
}

// HostMetricDataPoint Host 趋势数据点
type HostMetricDataPoint struct {
	Timestamp          int64   `json:"timestamp"`
	RequestsPerSecond  float64 `json:"requestsPerSecond"`
	IngressBytesPerSec float64 `json:"ingressBytesPerSec"`
	EgressBytesPerSec  float64 `json:"egressBytesPerSec"`
	Error4xxRate       float64 `json:"error4xxRate"`
	Error5xxRate       float64 `json:"error5xxRate"`
	P95Latency         float64 `json:"p95Latency"`
}

// HostPathMetrics Host 按 Path 分组指标
type HostPathMetrics struct {
	Path              string  `json:"path"`
	RequestsPerSecond float64 `json:"requestsPerSecond"`
	ErrorRate         float64 `json:"errorRate"`
	ErrorRatePercent  float64 `json:"errorRatePercent"` // 错误率百分比（用于显示）
	P95Latency        float64 `json:"p95Latency"`
}

// ==================== IngressOperator 接口 ====================

// IngressOperator Ingress 操作器接口
type IngressOperator interface {
	// 综合查询
	GetIngressMetrics(namespace, ingressName string, timeRange *TimeRange) (*IngressMetrics, error)

	// Controller 健康查询
	GetControllerHealth(timeRange *TimeRange) (*IngressControllerHealth, error)

	// 流量查询
	GetIngressTraffic(namespace, ingressName string, timeRange *TimeRange) (*IngressTrafficMetrics, error)
	GetIngressTrafficByHost(host string, timeRange *TimeRange) (*IngressTrafficMetrics, error)
	GetIngressTrafficByPath(path string, timeRange *TimeRange) (*IngressTrafficMetrics, error)

	// 性能查询
	GetIngressPerformance(namespace, ingressName string, timeRange *TimeRange) (*IngressPerformanceMetrics, error)
	GetIngressLatencyByHost(host string, timeRange *TimeRange) (*IngressLatencyStats, error)
	GetIngressLatencyByPath(path string, timeRange *TimeRange) (*IngressLatencyStats, error)

	// 错误查询
	GetIngressErrors(namespace, ingressName string, timeRange *TimeRange) (*IngressErrorMetrics, error)
	GetIngressStatusCodes(namespace, ingressName string, timeRange *TimeRange) (*IngressStatusCodeDistribution, error)

	// 后端健康查询
	GetIngressBackends(namespace, ingressName string, timeRange *TimeRange) (*IngressBackendMetrics, error)

	// 证书查询
	GetIngressCertificates(namespace, ingressName string) (*IngressCertificateMetrics, error)
	GetExpiringCertificates(daysThreshold int) ([]CertificateInfo, error)

	// Ingress 对象查询
	GetIngressObject(namespace, ingressName string) (*IngressObjectMetrics, error)

	// 限流查询
	GetIngressRateLimit(namespace, ingressName string, timeRange *TimeRange) (*IngressRateLimitMetrics, error)

	// 排行查询
	GetIngressRanking(limit int, timeRange *TimeRange) (*IngressRanking, error)
	GetPathRanking(limit int, timeRange *TimeRange) (*PathRanking, error)
	GetHostRanking(limit int, timeRange *TimeRange) (*HostRanking, error)

	// 列表查询
	ListIngressMetrics(namespace string, timeRange *TimeRange) ([]IngressMetrics, error)

	// Host 级别查询（旧版，保留兼容）
	GetHostMetrics(namespace, host string, timeRange *TimeRange) (*HostMetricsDetail, error)

	// Host 级别监控查询（新版）
	GetHostMetricsDetail(host string, timeRange *TimeRange) (*HostMetrics, error)
	GetMultiHostMetrics(hosts []string, timeRange *TimeRange) ([]HostMetrics, error)
}
