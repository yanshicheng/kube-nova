package logservicelogic

import (
	"context"
	"testing"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	logtypes "github.com/yanshicheng/kube-nova/common/logmanager/types"
)

func TestQueryLogsRequiresClusterUuid(t *testing.T) {
	logic := NewQueryLogsLogic(context.Background(), &svc.ServiceContext{})
	_, err := logic.QueryLogs(&pb.QueryLogsReq{})
	if err == nil || err.Error() != "clusterUuid 不能为空" {
		t.Fatalf("expected clusterUuid required error, got %v", err)
	}
}

func TestBuildQueryLogsRespIncludesSearchTargets(t *testing.T) {
	resp := buildQueryLogsResp(&logtypes.SearchResponse{
		BackendType:  "elasticsearch",
		QueryMode:    logtypes.QueryModeES,
		DataStream:   "logs-kube-nova-default",
		IndexPattern: "logs-*",
		NextToken:    "next-1",
		List:         []logtypes.LogRecord{{Timestamp: 1, Message: "line-1", BackendType: "elasticsearch"}},
	})
	if resp.QueryMode != logtypes.QueryModeES {
		t.Fatalf("expected queryMode to be propagated, got %q", resp.QueryMode)
	}
	if resp.DataStream != "logs-kube-nova-default" || resp.IndexPattern != "logs-*" {
		t.Fatalf("expected search targets to be propagated, got dataStream=%q indexPattern=%q", resp.DataStream, resp.IndexPattern)
	}
}

func TestBuildSearchRequestUsesCodeMode(t *testing.T) {
	searchReq, err := buildSearchRequest(&pb.QueryLogsReq{
		ClusterUuid: "cluster-1",
		LogType:     "container",
		Mode:        "code",
		Expr:        `{cluster_uuid="cluster-1"} |= "panic"`,
		StartTime:   1000,
		EndTime:     2000,
		Limit:       100,
		Direction:   "backward",
	}, "container", "loki", searchDefaults{QueryMode: logtypes.QueryModePlatform, DataStream: "logs-default", IndexPattern: "logs-*"})
	if err != nil {
		t.Fatalf("buildSearchRequest() returned error: %v", err)
	}
	if searchReq.SearchMode != logtypes.SearchModeCode {
		t.Fatalf("expected code mode, got %q", searchReq.SearchMode)
	}
	if searchReq.QueryExpr == "" {
		t.Fatal("expected expr propagated")
	}
	if searchReq.QueryMode != logtypes.QueryModeLoki {
		t.Fatalf("expected loki query mode for loki backend, got %q", searchReq.QueryMode)
	}
}

func TestBuildSearchRequestMergesHostFilters(t *testing.T) {
	searchReq, err := buildSearchRequest(&pb.QueryLogsReq{
		ClusterUuid: "cluster-1",
		LogType:     "system",
		Namespace:   "ignored",
		Host:        "node-a",
		Hosts:       []string{"node-a", "node-b"},
		SourceType:  "kernel",
		StartTime:   1000,
		EndTime:     2000,
		Limit:       100,
		Direction:   "backward",
	}, "system", "elasticsearch", searchDefaults{QueryMode: logtypes.QueryModePlatform})
	if err != nil {
		t.Fatalf("buildSearchRequest() returned error: %v", err)
	}
	if searchReq.Host != "node-a" {
		t.Fatalf("expected primary host, got %q", searchReq.Host)
	}
	if len(searchReq.Hosts) != 2 {
		t.Fatalf("expected merged hosts, got %#v", searchReq.Hosts)
	}
	if searchReq.SourceType != "kernel" {
		t.Fatalf("expected sourceType propagated, got %q", searchReq.SourceType)
	}
}

func TestBuildSearchRequestKeepsProjectUuidButDoesNotAffectFiltersHere(t *testing.T) {
	searchReq, err := buildSearchRequest(&pb.QueryLogsReq{
		ClusterUuid: "cluster-1",
		LogType:     "container",
		ProjectUuid: "project-1",
		Namespace:   "pay-sit",
		PodName:     "test-deployment*",
		QueryText:   "学习啊",
		StartTime:   1000,
		EndTime:     2000,
		Limit:       100,
		Direction:   "backward",
	}, "container", "elasticsearch", searchDefaults{QueryMode: logtypes.QueryModePlatform})
	if err != nil {
		t.Fatalf("buildSearchRequest() returned error: %v", err)
	}
	if searchReq.ProjectUuid != "project-1" {
		t.Fatalf("expected projectUuid propagated, got %q", searchReq.ProjectUuid)
	}
	if searchReq.PodName != "test-deployment*" {
		t.Fatalf("expected wildcard podName propagated, got %q", searchReq.PodName)
	}
}

func TestResolveLogBackendAppPrefersRequestedMode(t *testing.T) {
	apps := []*model.OnecClusterApp{{AppCode: "loki", IsDefault: 0}, {AppCode: "elasticsearch", IsDefault: 1}}
	picked := resolveLogBackendApp(apps, logtypes.QueryModeLoki)
	if picked == nil || picked.AppCode != "loki" {
		t.Fatalf("expected loki backend, got %+v", picked)
	}
}

func TestResolveLogBackendAppFallsBackToAvailableBackendWhenNoDefault(t *testing.T) {
	apps := []*model.OnecClusterApp{{AppCode: "loki", IsDefault: 0}}
	picked := resolveLogBackendApp(apps, logtypes.QueryModePlatform)
	if picked == nil || picked.AppCode != "loki" {
		t.Fatalf("expected available loki backend, got %+v", picked)
	}
}

func TestResolveElasticsearchDataStreamByLogType(t *testing.T) {
	tests := []struct {
		name       string
		backend    string
		logType    string
		fallback   string
		wantStream string
	}{
		{name: "es container stream", backend: "elasticsearch", logType: "container", fallback: "logs-kube-nova-default", wantStream: "logs-container-default"},
		{name: "es system stream", backend: "elasticsearch", logType: "system", fallback: "logs-kube-nova-default", wantStream: "logs-system-default"},
		{name: "non es keeps fallback", backend: "loki", logType: "container", fallback: "logs-kube-nova-default", wantStream: "logs-kube-nova-default"},
		{name: "es unknown keeps fallback", backend: "elasticsearch", logType: "unknown", fallback: "logs-kube-nova-default", wantStream: "logs-kube-nova-default"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveElasticsearchDataStream(tt.backend, tt.logType, tt.fallback)
			if got != tt.wantStream {
				t.Fatalf("resolveElasticsearchDataStream()=%q, want %q", got, tt.wantStream)
			}
		})
	}
}

func TestResolveElasticsearchIndexPatternByLogType(t *testing.T) {
	tests := []struct {
		name        string
		backend     string
		logType     string
		fallback    string
		wantPattern string
	}{
		{name: "es container pattern", backend: "elasticsearch", logType: "container", fallback: "logs-kube-nova-*", wantPattern: "logs-container-*"},
		{name: "es system pattern", backend: "elasticsearch", logType: "system", fallback: "logs-kube-nova-*", wantPattern: "logs-system-*"},
		{name: "non es keeps fallback", backend: "loki", logType: "container", fallback: "logs-kube-nova-*", wantPattern: "logs-kube-nova-*"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveElasticsearchIndexPattern(tt.backend, tt.logType, tt.fallback)
			if got != tt.wantPattern {
				t.Fatalf("resolveElasticsearchIndexPattern()=%q, want %q", got, tt.wantPattern)
			}
		})
	}
}
