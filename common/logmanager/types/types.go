package types

import (
	"fmt"
	"io"
	"strings"
	"time"
)

const (
	QueryModePlatform      = "platform"
	QueryModeLoki          = "loki"
	QueryModeES            = "es"
	DefaultSearchQueryMode = QueryModePlatform
	SearchModeForm         = "form"
	SearchModeCode         = "code"
)

type ElasticsearchFieldMapping struct {
	Timestamp     string `json:"timestamp,omitempty"`
	Message       string `json:"message,omitempty"`
	ProjectUuid   string `json:"projectUuid,omitempty"`
	ClusterUuid   string `json:"clusterUuid,omitempty"`
	NamespaceName string `json:"namespaceName,omitempty"`
	Application   string `json:"application,omitempty"`
	ResourceName  string `json:"resourceName,omitempty"`
	PodName       string `json:"podName,omitempty"`
	ContainerName string `json:"containerName,omitempty"`
	PodIp         string `json:"podIp,omitempty"`
	Host          string `json:"host,omitempty"`
	SourceType    string `json:"sourceType,omitempty"`
	LogType       string `json:"logType,omitempty"`
	Level         string `json:"level,omitempty"`
}

type ElasticsearchConfig struct {
	DataStream   string                    `json:"dataStream,omitempty"`
	IndexPattern string                    `json:"indexPattern,omitempty"`
	FieldMapping ElasticsearchFieldMapping `json:"fieldMapping,omitempty"`
}

type QueryDefaults struct {
	QueryMode string `json:"queryMode,omitempty"`
}

func NormalizeQueryMode(mode string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "", QueryModePlatform:
		return QueryModePlatform, nil
	case QueryModeLoki:
		return QueryModeLoki, nil
	case QueryModeES:
		return QueryModeES, nil
	default:
		return "", fmt.Errorf("unsupported query mode: %s", mode)
	}
}

func NormalizeSearchMode(mode string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "", SearchModeForm:
		return SearchModeForm, nil
	case SearchModeCode:
		return SearchModeCode, nil
	default:
		return "", fmt.Errorf("unsupported search mode: %s", mode)
	}
}

func normalizeFieldMapping(mapping ElasticsearchFieldMapping) ElasticsearchFieldMapping {
	mapping.Timestamp = strings.TrimSpace(mapping.Timestamp)
	mapping.Message = strings.TrimSpace(mapping.Message)
	mapping.ProjectUuid = strings.TrimSpace(mapping.ProjectUuid)
	mapping.ClusterUuid = strings.TrimSpace(mapping.ClusterUuid)
	mapping.NamespaceName = strings.TrimSpace(mapping.NamespaceName)
	mapping.Application = strings.TrimSpace(mapping.Application)
	mapping.ResourceName = strings.TrimSpace(mapping.ResourceName)
	mapping.PodName = strings.TrimSpace(mapping.PodName)
	mapping.ContainerName = strings.TrimSpace(mapping.ContainerName)
	mapping.PodIp = strings.TrimSpace(mapping.PodIp)
	mapping.Host = strings.TrimSpace(mapping.Host)
	mapping.SourceType = strings.TrimSpace(mapping.SourceType)
	mapping.LogType = strings.TrimSpace(mapping.LogType)
	mapping.Level = strings.TrimSpace(mapping.Level)
	return mapping
}

func NormalizeElasticsearchConfig(cfg ElasticsearchConfig) ElasticsearchConfig {
	return ElasticsearchConfig{
		DataStream:   strings.TrimSpace(cfg.DataStream),
		IndexPattern: strings.TrimSpace(cfg.IndexPattern),
		FieldMapping: normalizeFieldMapping(cfg.FieldMapping),
	}
}

func (d QueryDefaults) Normalize() (QueryDefaults, error) {
	queryMode, err := NormalizeQueryMode(d.QueryMode)
	if err != nil {
		return QueryDefaults{}, err
	}
	return QueryDefaults{QueryMode: queryMode}, nil
}

type LogConfig struct {
	UUID          string              `json:"uuid"`
	Name          string              `json:"name"`
	AppCode       string              `json:"appCode"`
	Endpoint      string              `json:"endpoint"`
	Username      string              `json:"username,omitempty"`
	Password      string              `json:"password,omitempty"`
	Token         string              `json:"token,omitempty"`
	Insecure      bool                `json:"insecure"`
	Timeout       int                 `json:"timeout"`
	HTTPPool      HTTPPoolConfig      `json:"httpPool,omitempty"`
	Elasticsearch ElasticsearchConfig `json:"elasticsearch,omitempty"`
}

type HTTPPoolConfig struct {
	MaxIdleConns          int           `json:"maxIdleConns,omitempty"`
	MaxIdleConnsPerHost   int           `json:"maxIdleConnsPerHost,omitempty"`
	MaxConnsPerHost       int           `json:"maxConnsPerHost,omitempty"`
	IdleConnTimeout       time.Duration `json:"idleConnTimeout,omitempty"`
	ResponseHeaderTimeout time.Duration `json:"responseHeaderTimeout,omitempty"`
	TLSHandshakeTimeout   time.Duration `json:"tlsHandshakeTimeout,omitempty"`
	ExpectContinueTimeout time.Duration `json:"expectContinueTimeout,omitempty"`
}

type SearchRequest struct {
	ClusterUuid   string    `json:"clusterUuid"`
	ProjectUuid   string    `json:"projectUuid"`
	NamespaceName string    `json:"namespaceName,omitempty"`
	Namespace     string    `json:"namespace,omitempty"`
	Application   string    `json:"application,omitempty"`
	ResourceName  string    `json:"resourceName,omitempty"`
	PodName       string    `json:"podName,omitempty"`
	ContainerName string    `json:"containerName,omitempty"`
	PodIp         string    `json:"podIp,omitempty"`
	Hosts         []string  `json:"hosts,omitempty"`
	Host          string    `json:"host,omitempty"`
	SourceType    string    `json:"sourceType,omitempty"`
	LogType       string    `json:"logType,omitempty"`
	Level         string    `json:"level,omitempty"`
	QueryText     string    `json:"queryText,omitempty"`
	Command       string    `json:"command,omitempty"`
	QueryMode     string    `json:"queryMode,omitempty"`
	QueryExpr     string    `json:"queryExpr,omitempty"`
	SearchMode    string    `json:"searchMode,omitempty"`
	Keyword       string    `json:"keyword,omitempty"`
	DataStream    string    `json:"dataStream,omitempty"`
	IndexPattern  string    `json:"indexPattern,omitempty"`
	Start         time.Time `json:"start"`
	End           time.Time `json:"end"`
	Limit         int64     `json:"limit"`
	Direction     string    `json:"direction"`
	NextToken     string    `json:"nextToken,omitempty"`
}

func (r *SearchRequest) Normalize(defaults ...QueryDefaults) error {
	if r == nil {
		return fmt.Errorf("search request is nil")
	}
	cfg := QueryDefaults{QueryMode: DefaultSearchQueryMode}
	if len(defaults) > 0 {
		cfg = defaults[0]
	}
	normalizedDefaults, err := cfg.Normalize()
	if err != nil {
		return err
	}
	if strings.TrimSpace(r.NamespaceName) == "" {
		r.NamespaceName = strings.TrimSpace(r.Namespace)
	}
	if strings.TrimSpace(r.Namespace) == "" {
		r.Namespace = r.NamespaceName
	}
	queryText := strings.TrimSpace(r.QueryText)
	if queryText == "" {
		queryText = strings.TrimSpace(r.Keyword)
	}
	if queryText == "*" {
		queryText = ""
	}
	queryMode := strings.TrimSpace(r.QueryMode)
	if queryMode == "" {
		queryMode = normalizedDefaults.QueryMode
	}
	if queryMode == "" {
		queryMode = DefaultSearchQueryMode
	}
	normalizedMode, err := NormalizeQueryMode(queryMode)
	if err != nil {
		return err
	}
	r.ClusterUuid = strings.TrimSpace(r.ClusterUuid)
	r.ProjectUuid = strings.TrimSpace(r.ProjectUuid)
	r.NamespaceName = strings.TrimSpace(r.NamespaceName)
	r.Namespace = strings.TrimSpace(r.Namespace)
	r.Application = strings.TrimSpace(r.Application)
	r.ResourceName = strings.TrimSpace(r.ResourceName)
	r.PodName = strings.TrimSpace(r.PodName)
	r.ContainerName = strings.TrimSpace(r.ContainerName)
	r.PodIp = strings.TrimSpace(r.PodIp)
	r.Host = strings.TrimSpace(r.Host)
	r.SourceType = strings.TrimSpace(r.SourceType)
	r.LogType = strings.TrimSpace(r.LogType)
	r.Level = strings.TrimSpace(r.Level)
	r.QueryText = queryText
	r.Command = strings.TrimSpace(r.Command)
	r.QueryExpr = strings.TrimSpace(r.QueryExpr)
	searchMode, err := NormalizeSearchMode(r.SearchMode)
	if err != nil {
		return err
	}
	r.SearchMode = searchMode
	r.Keyword = queryText
	r.QueryMode = normalizedMode
	r.DataStream = strings.TrimSpace(r.DataStream)
	r.IndexPattern = strings.TrimSpace(r.IndexPattern)
	r.Direction = strings.TrimSpace(r.Direction)
	r.NextToken = strings.TrimSpace(r.NextToken)
	return nil
}

type LogRecord struct {
	Timestamp   int64             `json:"timestamp"`
	Message     string            `json:"message"`
	BackendType string            `json:"backendType"`
	Labels      map[string]string `json:"labels"`
	Raw         string            `json:"raw"`
}

type SearchResponse struct {
	BackendType  string      `json:"backendType"`
	QueryMode    string      `json:"queryMode,omitempty"`
	DataStream   string      `json:"dataStream,omitempty"`
	IndexPattern string      `json:"indexPattern,omitempty"`
	NextToken    string      `json:"nextToken,omitempty"`
	List         []LogRecord `json:"list"`
}

type AlertSyncRequest struct {
	RuleID        uint64            `json:"ruleId"`
	ClusterUuid   string            `json:"clusterUuid"`
	ProjectUuid   string            `json:"projectUuid"`
	Namespace     string            `json:"namespace"`
	Application   string            `json:"application,omitempty"`
	ResourceName  string            `json:"resourceName,omitempty"`
	Name          string            `json:"name"`
	Description   string            `json:"description,omitempty"`
	QueryText     string            `json:"queryText"`
	ConditionType string            `json:"conditionType"`
	Threshold     string            `json:"threshold"`
	Window        string            `json:"window"`
	Severity      string            `json:"severity"`
	Labels        map[string]string `json:"labels,omitempty"`
	WebhookURL    string            `json:"webhookUrl,omitempty"`
	WebhookToken  string            `json:"webhookToken,omitempty"`
}

type LogClient interface {
	GetUUID() string
	GetName() string
	GetEndpoint() string
	GetBackendType() string
	Search(req *SearchRequest) (*SearchResponse, error)
	Download(req *SearchRequest, writer io.Writer) error
	CreateAlert(req *AlertSyncRequest) (string, string, error)
	UpdateAlert(backendRuleID string, req *AlertSyncRequest) (string, error)
	DeleteAlert(backendRuleID string) error
	EnableAlert(req *AlertSyncRequest, backendRuleID string) (string, string, error)
	DisableAlert(backendRuleID string) error
	Ping() error
	Close() error
}
