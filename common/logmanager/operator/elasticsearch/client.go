package elasticsearch

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	baseop "github.com/yanshicheng/kube-nova/common/logmanager/operator"
	"github.com/yanshicheng/kube-nova/common/logmanager/types"
)

type Client struct {
	*baseop.BaseOperator
	uuid     string
	name     string
	endpoint string
	config   types.ElasticsearchConfig
}

type CountResult struct {
	Count int64
	Err   error
}

var quotedWildcardQueryPattern = regexp.MustCompile(`([A-Za-z0-9_.]+):"([^"]*[*?][^"]*)"`)

func NewClient(ctx context.Context, config *types.LogConfig) (types.LogClient, error) {
	base, err := baseop.NewBaseOperator(ctx, config.Endpoint, config.Username, config.Password, config.Token, config.Insecure, config.Timeout, config.HTTPPool)
	if err != nil {
		return nil, err
	}
	return &Client{BaseOperator: base, uuid: config.UUID, name: config.Name, endpoint: config.Endpoint, config: types.NormalizeElasticsearchConfig(config.Elasticsearch)}, nil
}

func (c *Client) GetUUID() string        { return c.uuid }
func (c *Client) GetName() string        { return c.name }
func (c *Client) GetEndpoint() string    { return c.endpoint }
func (c *Client) GetBackendType() string { return "elasticsearch" }

func mappedField(mapping, fallback string) string {
	if strings.TrimSpace(mapping) != "" {
		return strings.TrimSpace(mapping)
	}
	return fallback
}

func buildSearchPath(req *types.SearchRequest) string {
	if strings.TrimSpace(req.DataStream) != "" {
		return "/" + strings.TrimSpace(req.DataStream) + "/_search"
	}
	if strings.TrimSpace(req.IndexPattern) != "" {
		return "/" + strings.TrimSpace(req.IndexPattern) + "/_search"
	}
	return "/_search"
}

func buildTermClause(field, value string) map[string]any {
	return map[string]any{"term": map[string]any{field: value}}
}

func buildWildcardClause(field, value string) map[string]any {
	return map[string]any{"wildcard": map[string]any{field: value}}
}

func buildFuzzyPattern(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || trimmed == "*" {
		return ""
	}
	return "*" + strings.Trim(trimmed, "*") + "*"
}

func buildPodNameClause(field, value string) map[string]any {
	pattern := strings.TrimSpace(value)
	return map[string]any{
		"bool": map[string]any{
			"should": []map[string]any{
				buildWildcardClause(field, pattern),
				{"query_string": map[string]any{"query": fmt.Sprintf("%s:%s", field, pattern)}},
			},
			"minimum_should_match": 1,
		},
	}
}

func normalizeMessageSearchPattern(input string) string {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" || trimmed == "*" {
		return ""
	}
	if _, err := regexp.Compile(trimmed); err == nil {
		return trimmed
	}
	escaped := regexp.QuoteMeta(trimmed)
	escaped = strings.ReplaceAll(escaped, `\*`, `.*`)
	escaped = strings.ReplaceAll(escaped, `\?`, `.`)
	return escaped
}

func escapeQueryStringTerm(value string) string {
	replacer := strings.NewReplacer(
		`\\`, `\\\\`,
		`+`, `\+`,
		`-`, `\-`,
		`=`, `\=`,
		`&&`, `\&&`,
		`||`, `\||`,
		`>`, `\>`,
		`<`, `\<`,
		`!`, `\!`,
		`(`, `\(`,
		`)`, `\)`,
		`{`, `\{`,
		`}`, `\}`,
		`[`, `\[`,
		`]`, `\]`,
		`^`, `\^`,
		`"`, `\"`,
		`~`, `\~`,
		`:`, `\:`,
		`/`, `\/`,
	)
	return replacer.Replace(value)
}

func buildMessageQueryClause(field, value string) map[string]any {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}

	if strings.ContainsAny(trimmed, `\[]()|+.^$*?`) {
		pattern := strings.ToLower(normalizeMessageSearchPattern(trimmed))
		return map[string]any{
			"query_string": map[string]any{
				"query":                  fmt.Sprintf("%s:/%s/", field, pattern),
				"allow_leading_wildcard": true,
			},
		}
	}

	terms := strings.Fields(strings.ToLower(trimmed))
	if len(terms) == 0 {
		return nil
	}
	parts := make([]string, 0, len(terms))
	for _, term := range terms {
		parts = append(parts, fmt.Sprintf("%s:(*%s*)", field, escapeQueryStringTerm(term)))
	}
	return map[string]any{
		"query_string": map[string]any{
			"query":                  strings.Join(parts, " AND "),
			"allow_leading_wildcard": true,
			"analyze_wildcard":       true,
			"default_operator":       "AND",
		},
	}
}

func buildSearchBody(req *types.SearchRequest, cfg types.ElasticsearchConfig) map[string]any {
	mapping := cfg.FieldMapping
	timestampField := mappedField(mapping.Timestamp, "@timestamp")
	messageField := mappedField(mapping.Message, "message")
	projectField := mappedField(mapping.ProjectUuid, "project_uuid")
	_ = projectField
	clusterField := mappedField(mapping.ClusterUuid, "cluster_uuid")
	namespaceField := mappedField(mapping.NamespaceName, "namespace_name")
	applicationField := mappedField(mapping.Application, "app")
	resourceField := mappedField(mapping.ResourceName, "resource_name")
	podField := mappedField(mapping.PodName, "pod_name")
	containerField := mappedField(mapping.ContainerName, "container_name")
	podIPField := mappedField(mapping.PodIp, "pod_ip")
	hostField := mappedField(mapping.Host, "node_name")
	sourceTypeField := mappedField(mapping.SourceType, "source_type")
	logTypeField := mappedField(mapping.LogType, "log_type")
	levelField := mappedField(mapping.Level, "level")

	if req.SearchMode == types.SearchModeCode && strings.TrimSpace(req.QueryExpr) != "" {
		normalizedExpr := normalizeCodeExpr(strings.TrimSpace(req.QueryExpr))
		body := map[string]any{
			"size": req.Limit,
			"sort": []map[string]any{{timestampField: map[string]any{"order": sortDirection(req.Direction)}}},
			"query": map[string]any{
				"bool": map[string]any{
					"filter": []map[string]any{{"range": map[string]any{timestampField: map[string]any{"gte": req.Start.Format(time.RFC3339), "lte": req.End.Format(time.RFC3339)}}}},
					"must":   []map[string]any{{"query_string": map[string]any{"query": normalizedExpr}}},
				},
			},
		}
		return body
	}

	filters := []map[string]any{
		buildTermClause(clusterField, req.ClusterUuid),
		{"range": map[string]any{timestampField: map[string]any{"gte": req.Start.Format(time.RFC3339), "lte": req.End.Format(time.RFC3339)}}},
	}
	if req.ProjectUuid != "" {
		filters = append(filters, buildTermClause(projectField, req.ProjectUuid))
	}
	if req.NamespaceName != "" {
		filters = append(filters, buildTermClause(namespaceField, req.NamespaceName))
	}
	if req.Application != "" {
		filters = append(filters, buildTermClause(applicationField, req.Application))
	}
	if req.ResourceName != "" {
		filters = append(filters, buildTermClause(resourceField, req.ResourceName))
	}
	if req.PodName != "" {
		filters = append(filters, buildPodNameClause(podField, req.PodName))
	}
	if req.ContainerName != "" {
		filters = append(filters, buildTermClause(containerField, req.ContainerName))
	}
	if req.PodIp != "" {
		filters = append(filters, buildTermClause(podIPField, req.PodIp))
	}
	if req.Host != "" {
		filters = append(filters, buildTermClause(hostField, req.Host))
	}
	if req.SourceType != "" {
		filters = append(filters, buildTermClause(sourceTypeField, req.SourceType))
	}
	if req.LogType != "" {
		filters = append(filters, buildTermClause(logTypeField, req.LogType))
	}
	if req.Level != "" {
		filters = append(filters, buildTermClause(levelField, req.Level))
	}

	must := []map[string]any{}
	text := strings.TrimSpace(req.QueryText)
	if text == "" {
		text = strings.TrimSpace(req.Keyword)
	}
	if text == "*" {
		text = ""
	}
	if text != "" {
		switch req.QueryMode {
		case types.QueryModeES:
			must = append(must, map[string]any{"query_string": map[string]any{"query": text}})
		default:
			if clause := buildMessageQueryClause(messageField, text); clause != nil {
				must = append(must, clause)
			}
		}
	}
	if command := strings.TrimSpace(req.Command); command != "" {
		switch req.QueryMode {
		case types.QueryModeES:
			must = append(must, map[string]any{"query_string": map[string]any{"query": command}})
		default:
			must = append(must, map[string]any{"simple_query_string": map[string]any{"query": command, "fields": []string{messageField}}})
		}
	}

	return map[string]any{
		"size":  req.Limit,
		"sort":  []map[string]any{{timestampField: map[string]any{"order": sortDirection(req.Direction)}}},
		"query": map[string]any{"bool": map[string]any{"filter": filters, "must": must}},
	}
}

func (c *Client) Search(req *types.SearchRequest) (*types.SearchResponse, error) {
	cfg := c.config
	if req.DataStream != "" || req.IndexPattern != "" {
		cfg.DataStream = req.DataStream
		cfg.IndexPattern = req.IndexPattern
	}
	body := buildSearchBody(req, cfg)
	path := buildSearchPath(&types.SearchRequest{DataStream: cfg.DataStream, IndexPattern: cfg.IndexPattern})
	var response struct {
		Hits struct {
			Hits []struct {
				Source map[string]any `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err := c.BaseOperator.DoJSONRequest(http.MethodPost, path, nil, body, &response); err != nil {
		return nil, err
	}
	timestampField := mappedField(cfg.FieldMapping.Timestamp, "@timestamp")
	messageField := mappedField(cfg.FieldMapping.Message, "message")
	resp := &types.SearchResponse{BackendType: "elasticsearch", QueryMode: req.QueryMode, DataStream: cfg.DataStream, IndexPattern: cfg.IndexPattern, List: make([]types.LogRecord, 0, len(response.Hits.Hits))}
	for _, hit := range response.Hits.Hits {
		timestamp := parseTimestampValue(hit.Source[timestampField])
		message := stringify(hit.Source[messageField])
		labels := extractSourceLabels(hit.Source, cfg)
		rawBytes, _ := json.Marshal(hit.Source)
		resp.List = append(resp.List, types.LogRecord{Timestamp: timestamp, Message: message, BackendType: "elasticsearch", Labels: labels, Raw: string(rawBytes)})
	}
	return resp, nil
}

func parseTimestampValue(value any) int64 {
	switch ts := value.(type) {
	case string:
		return parseTimestampString(ts)
	case json.Number:
		if millis, err := ts.Int64(); err == nil {
			return millis
		}
	case float64:
		return int64(ts)
	case int64:
		return ts
	case int:
		return int64(ts)
	}
	return 0
}

func parseTimestampString(value string) int64 {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return 0
	}
	if millis, err := strconv.ParseInt(trimmed, 10, 64); err == nil {
		return millis
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		if ts, err := time.Parse(layout, trimmed); err == nil {
			return ts.UnixMilli()
		}
	}
	return 0
}

func extractSourceLabels(source map[string]any, cfg types.ElasticsearchConfig) map[string]string {
	if len(source) == 0 {
		return map[string]string{}
	}
	skipFields := map[string]struct{}{
		mappedField(cfg.FieldMapping.Timestamp, "@timestamp"): {},
		mappedField(cfg.FieldMapping.Message, "message"):      {},
	}
	labels := make(map[string]string, len(source))
	for key, value := range source {
		if _, skip := skipFields[key]; skip {
			continue
		}
		text := strings.TrimSpace(stringify(value))
		if text == "" || text == "null" {
			continue
		}
		labels[key] = text
	}
	return labels
}

func normalizeCodeExpr(expr string) string {
	return quotedWildcardQueryPattern.ReplaceAllString(expr, `$1:$2`)
}

func stringify(v any) string {
	switch val := v.(type) {
	case string:
		return val
	default:
		data, _ := json.Marshal(val)
		return string(data)
	}
}

func sortDirection(direction string) string {
	if strings.EqualFold(direction, "forward") {
		return "asc"
	}
	return "desc"
}

func (c *Client) Download(req *types.SearchRequest, writer io.Writer) error {
	resp, err := c.Search(req)
	if err != nil {
		return err
	}
	for _, item := range resp.List {
		if _, err := io.WriteString(writer, item.Message+"\n"); err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) Count(req *types.SearchRequest) (int64, error) {
	cfg := c.config
	if req.DataStream != "" || req.IndexPattern != "" {
		cfg.DataStream = req.DataStream
		cfg.IndexPattern = req.IndexPattern
	}
	countBody, err := buildCountBody(req, cfg)
	if err != nil {
		return 0, err
	}

	path := buildSearchPath(&types.SearchRequest{DataStream: cfg.DataStream, IndexPattern: cfg.IndexPattern})
	var response struct {
		Hits struct {
			Total struct {
				Value int64 `json:"value"`
			} `json:"total"`
		} `json:"hits"`
	}
	if err := c.BaseOperator.DoJSONRequest(http.MethodPost, path, nil, countBody, &response); err != nil {
		return 0, err
	}
	return response.Hits.Total.Value, nil
}

func buildCountBody(req *types.SearchRequest, cfg types.ElasticsearchConfig) (map[string]any, error) {
	body := buildSearchBody(req, cfg)
	query, ok := body["query"]
	if !ok {
		return nil, fmt.Errorf("count query body invalid")
	}
	return map[string]any{
		"query":            query,
		"size":             0,
		"track_total_hits": true,
	}, nil
}

func (c *Client) MultiCount(reqs []*types.SearchRequest) ([]CountResult, error) {
	if len(reqs) == 0 {
		return []CountResult{}, nil
	}

	var bodyBuilder strings.Builder
	for _, req := range reqs {
		cfg := c.config
		if req.DataStream != "" || req.IndexPattern != "" {
			cfg.DataStream = req.DataStream
			cfg.IndexPattern = req.IndexPattern
		}

		header := make(map[string]any)
		if strings.TrimSpace(cfg.DataStream) != "" {
			header["index"] = []string{strings.TrimSpace(cfg.DataStream)}
		} else if strings.TrimSpace(cfg.IndexPattern) != "" {
			header["index"] = []string{strings.TrimSpace(cfg.IndexPattern)}
		}
		headerBytes, err := json.Marshal(header)
		if err != nil {
			return nil, err
		}

		countBody, err := buildCountBody(req, cfg)
		if err != nil {
			return nil, err
		}
		bodyBytes, err := json.Marshal(countBody)
		if err != nil {
			return nil, err
		}

		bodyBuilder.Write(headerBytes)
		bodyBuilder.WriteByte('\n')
		bodyBuilder.Write(bodyBytes)
		bodyBuilder.WriteByte('\n')
	}

	respBytes, _, err := c.BaseOperator.DoTextRequest(http.MethodPost, "/_msearch", nil, bodyBuilder.String(), "application/x-ndjson")
	if err != nil {
		return nil, err
	}

	var response struct {
		Responses []struct {
			Error any `json:"error"`
			Hits  struct {
				Total any `json:"total"`
			} `json:"hits"`
		} `json:"responses"`
	}
	if err := json.Unmarshal(respBytes, &response); err != nil {
		return nil, fmt.Errorf("解析 msearch 响应失败: %v", err)
	}
	if len(response.Responses) != len(reqs) {
		return nil, fmt.Errorf("msearch 响应数量不匹配: expect=%d actual=%d", len(reqs), len(response.Responses))
	}

	result := make([]CountResult, 0, len(response.Responses))
	for idx, item := range response.Responses {
		if item.Error != nil {
			errBytes, _ := json.Marshal(item.Error)
			result = append(result, CountResult{Err: fmt.Errorf("msearch 响应失败: idx=%d err=%s", idx, string(errBytes))})
			continue
		}
		total, err := parseHitsTotal(item.Hits.Total)
		if err != nil {
			result = append(result, CountResult{Err: fmt.Errorf("解析 msearch total 失败: idx=%d err=%v", idx, err)})
			continue
		}
		result = append(result, CountResult{Count: total})
	}
	return result, nil
}

func parseHitsTotal(value any) (int64, error) {
	switch v := value.(type) {
	case map[string]any:
		raw, ok := v["value"]
		if !ok {
			return 0, fmt.Errorf("missing total.value")
		}
		return parseHitsTotal(raw)
	case float64:
		return int64(v), nil
	case int64:
		return v, nil
	case int:
		return int64(v), nil
	case json.Number:
		return v.Int64()
	default:
		return 0, fmt.Errorf("unsupported total type: %T", value)
	}
}

func (c *Client) CreateAlert(req *types.AlertSyncRequest) (string, string, error) {
	watchID := fmt.Sprintf("log-rule-%d", req.RuleID)
	indices := []string{"*"}
	if c.config.DataStream != "" {
		indices = []string{c.config.DataStream}
	} else if c.config.IndexPattern != "" {
		indices = []string{c.config.IndexPattern}
	}
	body := map[string]any{
		"trigger": map[string]any{"schedule": map[string]any{"interval": req.Window}},
		"input": map[string]any{
			"search": map[string]any{
				"request": map[string]any{
					"indices": indices,
					"body": map[string]any{
						"query": map[string]any{
							"bool": map[string]any{
								"filter": []map[string]any{{
									"term": map[string]any{"project_uuid.keyword": req.ProjectUuid},
								}, {
									"term": map[string]any{"namespace.keyword": req.Namespace},
								}},
							},
						},
					},
				},
			},
		},
		"condition": map[string]any{"compare": map[string]any{"ctx.payload.hits.total.value": map[string]any{"gt": 0}}},
		"actions": map[string]any{
			"webhook_manager": map[string]any{
				"webhook": map[string]any{
					"method": "POST",
					"url":    req.WebhookURL + "?token=" + req.WebhookToken,
					"body": fmt.Sprintf(`{"receiver":"log-rule","status":"firing","alerts":[{"status":"firing","labels":{"cluster_uuid":"%s","project_uuid":"%s","namespace":"%s","application":"%s","resource_name":"%s","alertname":"%s","severity":"%s"},"annotations":{"description":"%s"},"startsAt":"{{ctx.execution_time}}","endsAt":"","generatorURL":"watcher","fingerprint":"%s"}]}`,
						req.ClusterUuid, req.ProjectUuid, req.Namespace, req.Application, req.ResourceName, req.Name, req.Severity, req.Description, watchID),
				},
			},
		},
	}
	payloadBytes, _ := json.Marshal(body)
	if err := c.BaseOperator.DoJSONRequest(http.MethodPut, "/_watcher/watch/"+watchID, nil, body, nil); err != nil {
		return "", string(payloadBytes), err
	}
	return watchID, string(payloadBytes), nil
}

func (c *Client) UpdateAlert(_ string, req *types.AlertSyncRequest) (string, error) {
	_, payload, err := c.CreateAlert(req)
	return payload, err
}

func (c *Client) DeleteAlert(backendRuleID string) error {
	_, _, err := c.BaseOperator.DoTextRequest(http.MethodDelete, "/_watcher/watch/"+backendRuleID, nil, "", "")
	return err
}

func (c *Client) EnableAlert(req *types.AlertSyncRequest, backendRuleID string) (string, string, error) {
	_, _, err := c.BaseOperator.DoTextRequest(http.MethodPut, "/_watcher/watch/"+backendRuleID+"/_activate", nil, "", "")
	if err != nil {
		return "", "", err
	}
	return backendRuleID, "", nil
}

func (c *Client) DisableAlert(backendRuleID string) error {
	_, _, err := c.BaseOperator.DoTextRequest(http.MethodPut, "/_watcher/watch/"+backendRuleID+"/_deactivate", nil, "", "")
	return err
}

func (c *Client) Ping() error {
	_, status, err := c.BaseOperator.DoTextRequest(http.MethodGet, "/", nil, "", "")
	if err != nil {
		return err
	}
	if status < 200 || status >= 300 {
		return fmt.Errorf("es ping failed: %d", status)
	}
	return nil
}

func (c *Client) Close() error { return nil }
