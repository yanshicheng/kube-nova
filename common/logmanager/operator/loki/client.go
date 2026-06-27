package loki

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
}

func NewClient(ctx context.Context, config *types.LogConfig) (types.LogClient, error) {
	base, err := baseop.NewBaseOperator(ctx, config.Endpoint, config.Username, config.Password, config.Token, config.Insecure, config.Timeout, config.HTTPPool)
	if err != nil {
		return nil, err
	}
	return &Client{BaseOperator: base, uuid: config.UUID, name: config.Name, endpoint: config.Endpoint}, nil
}

func (c *Client) GetUUID() string        { return c.uuid }
func (c *Client) GetName() string        { return c.name }
func (c *Client) GetEndpoint() string    { return c.endpoint }
func (c *Client) GetBackendType() string { return "loki" }

func appendLabel(parts []string, key, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return parts
	}
	if strings.Contains(value, "*") {
		pattern := regexp.QuoteMeta(value)
		pattern = strings.ReplaceAll(pattern, `\*`, `.*`)
		return append(parts, fmt.Sprintf(`%s=~"%s"`, key, pattern))
	}
	return append(parts, fmt.Sprintf(`%s="%s"`, key, strings.ReplaceAll(value, `"`, `\"`)))
}

func appendJSONFilter(parts []string, key, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return parts
	}
	if strings.Contains(value, "*") {
		pattern := regexp.QuoteMeta(value)
		pattern = strings.ReplaceAll(pattern, `\*`, `.*`)
		return append(parts, fmt.Sprintf(`%s=~"%s"`, key, pattern))
	}
	return append(parts, fmt.Sprintf(`%s="%s"`, key, strings.ReplaceAll(value, `"`, `\"`)))
}

func normalizeSearchPattern(input string) string {
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

func buildCaseInsensitiveLineRegex(input string) string {
	pattern := normalizeSearchPattern(input)
	if pattern == "" {
		return ""
	}
	if strings.HasPrefix(pattern, "(?i)") {
		return pattern
	}
	return "(?i)" + pattern
}

func looksLikeLokiRawFragment(input string) bool {
	trimmed := strings.TrimSpace(input)
	return strings.HasPrefix(trimmed, "|")
}

func buildContainerMessageFilter(input string) string {
	pattern := buildCaseInsensitiveLineRegex(input)
	if pattern == "" {
		return ""
	}
	return fmt.Sprintf(` |~ "%s"`, strings.ReplaceAll(pattern, `"`, `\"`))
}

func buildQuery(req *types.SearchRequest) string {
	if req.SearchMode == types.SearchModeCode && strings.TrimSpace(req.QueryExpr) != "" {
		return strings.TrimSpace(req.QueryExpr)
	}
	parts := make([]string, 0, 10)
	parts = appendLabel(parts, "namespace_name", req.NamespaceName)
	parts = appendLabel(parts, "node_name", req.Host)
	parts = appendLabel(parts, "source_type", req.SourceType)
	parts = appendLabel(parts, "log_type", req.LogType)
	parts = appendLabel(parts, "level", req.Level)

	query := "{" + strings.Join(parts, ",") + "}"
	jsonFilters := make([]string, 0, 6)
	jsonFilters = appendJSONFilter(jsonFilters, "cluster_uuid", req.ClusterUuid)
	jsonFilters = appendJSONFilter(jsonFilters, "project_uuid", req.ProjectUuid)
	jsonFilters = appendJSONFilter(jsonFilters, "app", req.Application)
	jsonFilters = appendJSONFilter(jsonFilters, "resource_name", req.ResourceName)
	jsonFilters = appendJSONFilter(jsonFilters, "pod_name", req.PodName)
	jsonFilters = appendJSONFilter(jsonFilters, "container_name", req.ContainerName)
	jsonFilters = appendJSONFilter(jsonFilters, "pod_ip", req.PodIp)
	if len(jsonFilters) > 0 {
		query += " | json | " + strings.Join(jsonFilters, " | ")
	}
	text := strings.TrimSpace(req.QueryText)
	if text == "" {
		text = strings.TrimSpace(req.Keyword)
	}
	if req.QueryMode == types.QueryModeLoki && looksLikeLokiRawFragment(text) {
		query += " " + strings.TrimSpace(text)
	} else if strings.TrimSpace(req.LogType) == "container" {
		if !strings.Contains(query, " | json") {
			query += " | json"
		}
		query += buildContainerMessageFilter(text)
	} else if regexText := buildCaseInsensitiveLineRegex(text); regexText != "" {
		query += fmt.Sprintf(` |~ "%s"`, strings.ReplaceAll(regexText, `"`, `\"`))
	}
	if command := strings.TrimSpace(req.Command); command != "" {
		query += " " + command
	}
	return query
}

func extractLokiMessage(line string) string {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return ""
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(trimmed), &payload); err != nil {
		return trimmed
	}

	if message := strings.TrimSpace(stringify(payload["message"])); message != "" && message != "null" {
		return message
	}
	if message := strings.TrimSpace(stringify(payload["log"])); message != "" && message != "null" {
		return message
	}
	return trimmed
}

func stringify(v any) string {
	switch val := v.(type) {
	case string:
		return val
	case nil:
		return ""
	default:
		data, _ := json.Marshal(val)
		return string(data)
	}
}

func (c *Client) Search(req *types.SearchRequest) (*types.SearchResponse, error) {
	params := map[string]string{
		"query":     buildQuery(req),
		"start":     strconv.FormatInt(req.Start.UnixNano(), 10),
		"end":       strconv.FormatInt(req.End.UnixNano(), 10),
		"limit":     strconv.FormatInt(req.Limit, 10),
		"direction": req.Direction,
	}
	var response struct {
		Data struct {
			Result []struct {
				Stream map[string]string `json:"stream"`
				Values [][]string        `json:"values"`
			} `json:"result"`
		} `json:"data"`
	}
	if err := c.BaseOperator.DoJSONRequest(http.MethodGet, "/loki/api/v1/query_range", params, nil, &response); err != nil {
		return nil, err
	}
	resp := &types.SearchResponse{BackendType: "loki", QueryMode: req.QueryMode, List: make([]types.LogRecord, 0)}
	for _, item := range response.Data.Result {
		for _, value := range item.Values {
			if len(value) < 2 {
				continue
			}
			ts, _ := strconv.ParseInt(value[0], 10, 64)
			resp.List = append(resp.List, types.LogRecord{
				Timestamp:   ts / int64(time.Millisecond),
				Message:     extractLokiMessage(value[1]),
				BackendType: "loki",
				Labels:      item.Stream,
				Raw:         value[1],
			})
		}
	}
	return resp, nil
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

func alertRuleNamespace(req *types.AlertSyncRequest) string {
	if value := strings.TrimSpace(req.ProjectUuid); value != "" {
		return value
	}
	if value := strings.TrimSpace(req.ClusterUuid); value != "" {
		return value
	}
	return "global"
}

func buildAlertSearchQuery(req *types.AlertSyncRequest) string {
	searchReq := &types.SearchRequest{
		ClusterUuid:   req.ClusterUuid,
		ProjectUuid:   req.ProjectUuid,
		NamespaceName: req.Namespace,
		Namespace:     req.Namespace,
		Application:   req.Application,
		ResourceName:  req.ResourceName,
		LogType:       "container",
		QueryText:     req.QueryText,
		QueryMode:     types.QueryModePlatform,
		SearchMode:    types.SearchModeForm,
	}
	return buildQuery(searchReq)
}

func parseThreshold(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "0"
	}
	return trimmed
}

func buildAlertExpr(req *types.AlertSyncRequest) string {
	query := buildAlertSearchQuery(req)
	window := strings.TrimSpace(req.Window)
	if window == "" {
		window = "5m"
	}
	threshold := parseThreshold(req.Threshold)
	base := fmt.Sprintf("sum(count_over_time(%s[%s]))", query, window)

	switch strings.ToLower(strings.TrimSpace(req.ConditionType)) {
	case "count_gt":
		return fmt.Sprintf("%s > %s", base, threshold)
	case "count_gte", "match_any":
		return fmt.Sprintf("%s >= %s", base, threshold)
	case "count_lt":
		return fmt.Sprintf("%s < %s", base, threshold)
	case "count_lte":
		return fmt.Sprintf("%s <= %s", base, threshold)
	case "no_data", "flatline":
		return fmt.Sprintf("%s == 0", base)
	default:
		return fmt.Sprintf("%s > 0", base)
	}
}

func (c *Client) CreateAlert(req *types.AlertSyncRequest) (string, string, error) {
	namespace := alertRuleNamespace(req)
	groupName := fmt.Sprintf("log-rule-%d", req.RuleID)
	yaml := fmt.Sprintf("name: %s\ninterval: 1m\nrules:\n  - alert: %s\n    expr: >-\n      %s\n    for: %s\n    labels:\n      severity: %s\n      cluster_uuid: %s\n      project_uuid: %s\n      namespace: %s\n      application: %s\n      resource_name: %s\n    annotations:\n      description: %s\n", groupName, req.Name, buildAlertExpr(req), req.Window, req.Severity, req.ClusterUuid, req.ProjectUuid, req.Namespace, req.Application, req.ResourceName, req.Description)
	if _, _, err := c.BaseOperator.DoTextRequest(http.MethodPost, "/loki/api/v1/rules/"+namespace, nil, yaml, "application/yaml"); err != nil {
		return "", "", err
	}
	return namespace + "/" + groupName, yaml, nil
}

func (c *Client) UpdateAlert(_ string, req *types.AlertSyncRequest) (string, error) {
	_, payload, err := c.CreateAlert(req)
	return payload, err
}

func splitRuleID(backendRuleID string) (string, string) {
	parts := strings.SplitN(backendRuleID, "/", 2)
	if len(parts) != 2 {
		return "", ""
	}
	return parts[0], parts[1]
}

func (c *Client) DeleteAlert(backendRuleID string) error {
	namespace, group := splitRuleID(backendRuleID)
	if namespace == "" || group == "" {
		return fmt.Errorf("无效的 Loki 规则ID: %s", backendRuleID)
	}
	_, _, err := c.BaseOperator.DoTextRequest(http.MethodDelete, "/loki/api/v1/rules/"+namespace+"/"+group, nil, "", "")
	return err
}

func (c *Client) EnableAlert(req *types.AlertSyncRequest, _ string) (string, string, error) {
	return c.CreateAlert(req)
}

func (c *Client) DisableAlert(backendRuleID string) error {
	return c.DeleteAlert(backendRuleID)
}

func (c *Client) Ping() error {
	_, status, err := c.BaseOperator.DoTextRequest(http.MethodGet, "/ready", nil, "", "")
	if err != nil {
		return err
	}
	if status != 200 {
		return fmt.Errorf("loki ping failed: %d", status)
	}
	return nil
}

func (c *Client) Close() error { return nil }
