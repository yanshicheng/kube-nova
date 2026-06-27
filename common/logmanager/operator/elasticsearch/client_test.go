package elasticsearch

import (
	"testing"
	"time"

	"github.com/yanshicheng/kube-nova/common/logmanager/types"
)

func TestBuildSearchPathPrefersDataStream(t *testing.T) {
	req := &types.SearchRequest{DataStream: "logs-kube-nova-default", IndexPattern: "logs-*"}
	if got := buildSearchPath(req); got != "/logs-kube-nova-default/_search" {
		t.Fatalf("expected data stream path, got %q", got)
	}
}

func TestBuildSearchPathFallsBackToIndexPattern(t *testing.T) {
	req := &types.SearchRequest{IndexPattern: "logs-*"}
	if got := buildSearchPath(req); got != "/logs-*/_search" {
		t.Fatalf("expected index pattern path, got %q", got)
	}
}

func TestBuildSearchBodyUsesUnifiedFields(t *testing.T) {
	req := &types.SearchRequest{
		ProjectUuid:   "project-1",
		ClusterUuid:   "cluster-1",
		NamespaceName: "ns-1",
		ResourceName:  "deploy-1",
		PodName:       "pod-1",
		ContainerName: "container-1",
		PodIp:         "10.0.0.2",
		Host:          "node-1",
		SourceType:    "kernel",
		LogType:       "container",
		Level:         "error",
		QueryMode:     types.QueryModePlatform,
		QueryText:     "timeout",
		Start:         time.UnixMilli(1000),
		End:           time.UnixMilli(2000),
		Limit:         50,
		Direction:     "backward",
	}
	cfg := types.ElasticsearchConfig{
		FieldMapping: types.ElasticsearchFieldMapping{
			Timestamp:     "@timestamp",
			Message:       "message",
			ProjectUuid:   "project_uuid",
			ClusterUuid:   "cluster_uuid",
			NamespaceName: "namespace_name",
			ResourceName:  "resource_name",
			PodName:       "pod_name",
			ContainerName: "container_name",
			PodIp:         "pod_ip",
			Host:          "node_name",
			SourceType:    "source_type",
			LogType:       "log_type",
			Level:         "level",
		},
	}

	body := buildSearchBody(req, cfg)
	query := body["query"].(map[string]any)["bool"].(map[string]any)
	filters := query["filter"].([]map[string]any)
	if len(filters) < 11 {
		t.Fatalf("expected unified filters, got %#v", filters)
	}
	must := query["must"].([]map[string]any)
	if len(must) != 1 {
		t.Fatalf("expected one platform text clause, got %#v", must)
	}
	queryString := must[0]["query_string"].(map[string]any)
	if queryString["query"] != `message:(*timeout*)` {
		t.Fatalf("expected fuzzy message query_string, got %#v", queryString)
	}
	foundProject := false
	for _, filter := range filters {
		if term, ok := filter["term"].(map[string]any); ok {
			if term["project_uuid"] == "project-1" {
				foundProject = true
			}
		}
	}
	if !foundProject {
		t.Fatalf("expected projectUuid exact filter, got %#v", filters)
	}
}

func TestParseTimestampStringSupportsRFC3339Nano(t *testing.T) {
	got := parseTimestampString("2026-04-22T12:34:56.123456789Z")
	want := time.Date(2026, 4, 22, 12, 34, 56, 123456789, time.UTC).UnixMilli()
	if got != want {
		t.Fatalf("parseTimestampString()=%d, want %d", got, want)
	}
}

func TestExtractSourceLabelsPreservesStructuredFields(t *testing.T) {
	labels := extractSourceLabels(map[string]any{
		"@timestamp":   "2026-04-22T12:34:56.123456789Z",
		"message":      "hello",
		"cluster_uuid": "cluster-1",
		"node_name":    "node-1",
		"source_host":  "10.0.0.1",
		"count":        3,
	}, types.ElasticsearchConfig{
		FieldMapping: types.ElasticsearchFieldMapping{
			Timestamp: "@timestamp",
			Message:   "message",
		},
	})
	if labels["cluster_uuid"] != "cluster-1" || labels["node_name"] != "node-1" || labels["source_host"] != "10.0.0.1" || labels["count"] != "3" {
		t.Fatalf("unexpected labels: %#v", labels)
	}
	if _, ok := labels["@timestamp"]; ok {
		t.Fatalf("timestamp should not be copied into labels: %#v", labels)
	}
	if _, ok := labels["message"]; ok {
		t.Fatalf("message should not be copied into labels: %#v", labels)
	}
}

func TestBuildSearchBodyUsesWildcardPodFilter(t *testing.T) {
	req := &types.SearchRequest{
		ClusterUuid: "cluster-1",
		PodName:     "test-deployment*",
		Start:       time.UnixMilli(1000),
		End:         time.UnixMilli(2000),
		Limit:       10,
		Direction:   "backward",
	}
	cfg := types.ElasticsearchConfig{FieldMapping: types.ElasticsearchFieldMapping{Timestamp: "@timestamp", PodName: "pod_name", ClusterUuid: "cluster_uuid"}}

	body := buildSearchBody(req, cfg)
	filters := body["query"].(map[string]any)["bool"].(map[string]any)["filter"].([]map[string]any)
	found := false
	for _, filter := range filters {
		boolClause, ok := filter["bool"].(map[string]any)
		if !ok {
			continue
		}
		should, ok := boolClause["should"].([]map[string]any)
		if !ok || len(should) == 0 {
			continue
		}
		hasWildcard := false
		hasQueryString := false
		for _, clause := range should {
			if wildcard, ok := clause["wildcard"].(map[string]any); ok && wildcard["pod_name"] == "test-deployment*" {
				hasWildcard = true
			}
			if queryString, ok := clause["query_string"].(map[string]any); ok && queryString["query"] == "pod_name:test-deployment*" {
				hasQueryString = true
			}
		}
		if hasWildcard && hasQueryString {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected pod filter to include wildcard and query_string clauses, got %#v", filters)
	}
}

func TestBuildSearchBodyTreatsWildcardMessageAsNoTextFilter(t *testing.T) {
	req := &types.SearchRequest{
		ClusterUuid: "cluster-1",
		QueryText:   "*",
		Start:       time.UnixMilli(1000),
		End:         time.UnixMilli(2000),
		Limit:       10,
		Direction:   "backward",
	}
	cfg := types.ElasticsearchConfig{FieldMapping: types.ElasticsearchFieldMapping{Timestamp: "@timestamp", Message: "message", ClusterUuid: "cluster_uuid"}}

	body := buildSearchBody(req, cfg)
	must := body["query"].(map[string]any)["bool"].(map[string]any)["must"].([]map[string]any)
	if len(must) != 0 {
		t.Fatalf("expected no message filter for wildcard text, got %#v", must)
	}
}

func TestBuildSearchBodyUsesCodeExpr(t *testing.T) {
	req := &types.SearchRequest{
		ClusterUuid: "cluster-1",
		SearchMode:  types.SearchModeCode,
		QueryExpr:   `log_type:system AND message:*panic*`,
		Start:       time.UnixMilli(1000),
		End:         time.UnixMilli(2000),
		Limit:       10,
		Direction:   "forward",
	}
	cfg := types.ElasticsearchConfig{FieldMapping: types.ElasticsearchFieldMapping{Timestamp: "@timestamp"}}

	body := buildSearchBody(req, cfg)
	query := body["query"].(map[string]any)["bool"].(map[string]any)
	must := query["must"].([]map[string]any)
	if len(must) != 1 {
		t.Fatalf("expected code mode to produce one raw expr clause, got %#v", must)
	}
	queryString := must[0]["query_string"].(map[string]any)
	if queryString["query"] != `log_type:system AND message:*panic*` {
		t.Fatalf("expected expr propagated, got %#v", queryString)
	}
}

func TestBuildSearchBodyNormalizesQuotedWildcardExpr(t *testing.T) {
	req := &types.SearchRequest{
		ClusterUuid: "cluster-1",
		SearchMode:  types.SearchModeCode,
		QueryExpr:   `cluster_uuid:"cluster-1" AND pod_name:"dep-web-test*"`,
		Start:       time.UnixMilli(1000),
		End:         time.UnixMilli(2000),
		Limit:       10,
		Direction:   "forward",
	}
	cfg := types.ElasticsearchConfig{FieldMapping: types.ElasticsearchFieldMapping{Timestamp: "@timestamp"}}

	body := buildSearchBody(req, cfg)
	query := body["query"].(map[string]any)["bool"].(map[string]any)
	must := query["must"].([]map[string]any)
	queryString := must[0]["query_string"].(map[string]any)
	if queryString["query"] != `cluster_uuid:"cluster-1" AND pod_name:dep-web-test*` {
		t.Fatalf("expected quoted wildcard normalized, got %#v", queryString)
	}
}

func TestBuildSearchBodyUsesRawESQuery(t *testing.T) {
	req := &types.SearchRequest{
		ProjectUuid:   "project-1",
		ClusterUuid:   "cluster-1",
		NamespaceName: "ns-1",
		QueryMode:     types.QueryModeES,
		QueryText:     `level:error AND message:timeout`,
		Command:       `AND pod_ip:10.0.0.2`,
		Start:         time.UnixMilli(1000),
		End:           time.UnixMilli(2000),
		Limit:         10,
		Direction:     "forward",
	}
	cfg := types.ElasticsearchConfig{FieldMapping: types.ElasticsearchFieldMapping{Timestamp: "@timestamp", ProjectUuid: "project_uuid", ClusterUuid: "cluster_uuid", NamespaceName: "namespace_name"}}
	body := buildSearchBody(req, cfg)
	query := body["query"].(map[string]any)["bool"].(map[string]any)
	must := query["must"].([]map[string]any)
	if len(must) != 2 {
		t.Fatalf("expected raw ES clauses for query text and command, got %#v", must)
	}
}

func TestBuildMessageQueryClauseSupportsWildcardRegex(t *testing.T) {
	clause := buildMessageQueryClause("message", "*Running*")
	queryString := clause["query_string"].(map[string]any)
	if queryString["query"] != `message:/.*running.*/` {
		t.Fatalf("expected wildcard text converted to regex query, got %#v", queryString)
	}
}
