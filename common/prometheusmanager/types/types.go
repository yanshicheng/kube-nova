package types

import (
	"time"
)

// PrometheusConfig Prometheus 连接配置
type PrometheusConfig struct {
	UUID     string `json:"uuid"`
	Name     string `json:"name"`
	Endpoint string `json:"endpoint"` // http://prometheus.example.com:9090
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	Insecure bool   `json:"insecure"`
	Timeout  int    `json:"timeout"` // 超时时间（秒）
}

// QueryRequest 通用查询请求
type QueryRequest struct {
	Query     string            `json:"query"`      // PromQL 查询语句
	Time      *time.Time        `json:"time"`       // 即时查询的时间点
	TimeRange *TimeRange        `json:"time_range"` // 范围查询的时间范围
	Labels    map[string]string `json:"labels"`     // 标签过滤器
}

// MetricSeries 指标序列
type MetricSeries struct {
	Metric map[string]string `json:"metric"` // 标签集合
	Values []MetricValue     `json:"values"` // 值序列
}

// QueryResult 查询结果
type QueryResult struct {
	ResultType string         `json:"result_type"` // "matrix" 或 "vector"
	Series     []MetricSeries `json:"series"`
}

type PrometheusResponse struct {
	Status string      `json:"status"`
	Data   interface{} `json:"data"`
	Error  string      `json:"error,omitempty"`
}

// PrometheusClient Prometheus 客户端接口
type PrometheusClient interface {
	GetUUID() string
	GetName() string
	GetEndpoint() string

	// 即时查询
	Query(query string, timestamp *time.Time) ([]InstantQueryResult, error)

	// 范围查询
	QueryRange(query string, start, end time.Time, step string) ([]RangeQueryResult, error)

	// Pod 操作器
	Pod() PodOperator
	Cluster() ClusterOperator
	Node() NodeOperator
	Ingress() IngressOperator
	Flagger() FlaggerOperator
	Namespace() NamespaceOperator
	// 健康检查
	Ping() error
	Close() error
}
