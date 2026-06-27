package loki

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/yanshicheng/kube-nova/common/logmanager/types"
)

func TestBuildQueryUsesUnifiedSelectors(t *testing.T) {
	req := &types.SearchRequest{
		ProjectUuid:   "project-1",
		ClusterUuid:   "cluster-1",
		NamespaceName: "ns-1",
		ResourceName:  "deploy-1",
		PodName:       "pod-1",
		ContainerName: "container-1",
		Host:          "node-1",
		SourceType:    "kernel",
		LogType:       "container",
		Level:         "error",
		QueryMode:     types.QueryModePlatform,
		QueryText:     "timeout",
	}

	query := buildQuery(req)
	for _, want := range []string{
		`namespace_name="ns-1"`,
		`node_name="node-1"`,
		`source_type="kernel"`,
		`log_type="container"`,
		`level="error"`,
		`| json | cluster_uuid="cluster-1"`,
		`resource_name="deploy-1"`,
		`pod_name="pod-1"`,
		`container_name="container-1"`,
		`|~ "(?i)timeout"`,
	} {
		if !strings.Contains(query, want) {
			t.Fatalf("expected query %q to contain %q", query, want)
		}
	}
	if !strings.Contains(query, `project_uuid="project-1"`) {
		t.Fatalf("expected projectUuid json filter in query, got %q", query)
	}
}

func TestBuildQueryTreatsWildcardMessageAsNoTextFilter(t *testing.T) {
	req := &types.SearchRequest{
		ClusterUuid: "cluster-1",
		LogType:     "container",
		QueryMode:   types.QueryModePlatform,
		QueryText:   "*",
	}

	query := buildQuery(req)
	if strings.Contains(query, `|= "*"`) {
		t.Fatalf("expected wildcard message to be ignored, got %q", query)
	}
	if !strings.Contains(query, `log_type="container"`) {
		t.Fatalf("expected exact log_type selector, got %q", query)
	}
}

func TestBuildQueryUsesWildcardSelectors(t *testing.T) {
	req := &types.SearchRequest{
		ClusterUuid: "cluster-1",
		PodName:     "test-deployment*",
		QueryMode:   types.QueryModePlatform,
		QueryText:   "panic",
	}

	query := buildQuery(req)
	if !strings.Contains(query, `pod_name=~"test-deployment.*"`) {
		t.Fatalf("expected wildcard pod selector, got %q", query)
	}
}

func TestBuildQueryUsesRawLokiFragment(t *testing.T) {
	req := &types.SearchRequest{
		ProjectUuid:   "project-1",
		ClusterUuid:   "cluster-1",
		NamespaceName: "ns-1",
		QueryMode:     types.QueryModeLoki,
		QueryText:     `|= "panic"`,
		Command:       `| json | pod_ip="10.0.0.2"`,
	}

	query := buildQuery(req)
	if !strings.Contains(query, `cluster_uuid="cluster-1"`) {
		t.Fatalf("expected platform boundary selectors in query: %q", query)
	}
	if !strings.Contains(query, `project_uuid="project-1"`) {
		t.Fatalf("expected projectUuid json filter in query: %q", query)
	}
	if !strings.Contains(query, `|= "panic"`) {
		t.Fatalf("expected loki query text in query: %q", query)
	}
	if !strings.Contains(query, `| json | pod_ip="10.0.0.2"`) {
		t.Fatalf("expected loki command fragment in query: %q", query)
	}
}

func TestBuildQueryUsesCodeExprDirectly(t *testing.T) {
	expr := `{cluster_uuid="cluster-1",log_type="system"} |= "panic"`
	req := &types.SearchRequest{
		ClusterUuid: "cluster-1",
		SearchMode:  types.SearchModeCode,
		QueryExpr:   expr,
		QueryText:   "ignored",
	}

	if got := buildQuery(req); got != expr {
		t.Fatalf("expected code mode expr to be used directly, got %q", got)
	}
}

func TestBuildQueryEscapesPlatformText(t *testing.T) {
	req := &types.SearchRequest{
		ProjectUuid:   "project-1",
		ClusterUuid:   "cluster-1",
		NamespaceName: "ns-1",
		QueryMode:     types.QueryModePlatform,
		QueryText:     `error \"quoted\"`,
		Start:         time.Now(),
	}

	query := buildQuery(req)
	if !strings.Contains(query, `|~ "(?i)error `) || !strings.Contains(query, `quoted`) {
		t.Fatalf("expected escaped platform query text, got %q", query)
	}
}

func TestBuildQueryTreatsWildcardWrappedKeywordAsRegex(t *testing.T) {
	req := &types.SearchRequest{
		ClusterUuid: "cluster-1",
		LogType:     "container",
		QueryMode:   types.QueryModePlatform,
		QueryText:   "*running*",
	}

	query := buildQuery(req)
	if !strings.Contains(query, `|~ "(?i).*running.*"`) {
		t.Fatalf("expected wildcard keyword to become case-insensitive regex, got %q", query)
	}
}

func TestExtractLokiMessagePrefersMessageThenLog(t *testing.T) {
	rawWithMessage, _ := json.Marshal(map[string]any{"message": "hello world", "log": "fallback"})
	if got := extractLokiMessage(string(rawWithMessage)); got != "hello world" {
		t.Fatalf("expected message field, got %q", got)
	}

	rawWithLog, _ := json.Marshal(map[string]any{"log": "daemonset running"})
	if got := extractLokiMessage(string(rawWithLog)); got != "daemonset running" {
		t.Fatalf("expected log field fallback, got %q", got)
	}

	rawPlain := "plain raw line"
	if got := extractLokiMessage(rawPlain); got != rawPlain {
		t.Fatalf("expected raw line fallback, got %q", got)
	}
}
