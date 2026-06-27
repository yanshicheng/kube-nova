package jenkins

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestStageLogUsesChildLogsWhenParentOnlyHasPipelineScaffold(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/job/demo/65/execution/node/45/wfapi/log":
			_ = json.NewEncoder(w).Encode(stageLogResp{
				Text:    "[Pipeline] { (Sonar扫描前端项目)\n[Pipeline] stage\n",
				HasMore: false,
				Length:  42,
			})
		case "/job/demo/65/execution/node/45/wfapi/describe":
			_, _ = w.Write([]byte(`{"stageFlowNodes":[{"id":"451","name":"script","status":"SUCCESS"}]}`))
		case "/job/demo/65/execution/node/451/wfapi/log":
			_ = json.NewEncoder(w).Encode(stageLogResp{
				Text:    "17:13:23 INFO sonar real log\n",
				HasMore: false,
				Length:  27,
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	manager := NewManager(ClientConfig{Endpoint: server.URL})
	content, err := manager.StageLog(context.Background(), "demo", 65, "45")
	if err != nil {
		t.Fatalf("stage log should succeed: %v", err)
	}
	if !contains(content, "sonar real log") {
		t.Fatalf("stage log should use child content when parent only has scaffold, got: %q", content)
	}
	if contains(content, "{ (Sonar扫描前端项目)") {
		t.Fatalf("stage log should not keep scaffold-only parent wrapper, got: %q", content)
	}
}

func TestStageLogKeepsDirectLogsWhenParentAlreadyHasRealOutput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/job/demo/65/execution/node/47/wfapi/log":
			_ = json.NewEncoder(w).Encode(stageLogResp{
				Text:    "added 69 packages in 35s\n",
				HasMore: false,
				Length:  24,
			})
		case "/job/demo/65/execution/node/47/wfapi/describe":
			_, _ = w.Write([]byte(`{"stageFlowNodes":[{"id":"471","name":"script","status":"SUCCESS"}]}`))
		case "/job/demo/65/execution/node/471/wfapi/log":
			_ = json.NewEncoder(w).Encode(stageLogResp{
				Text:    "[Pipeline] echo\n",
				HasMore: false,
				Length:  16,
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	manager := NewManager(ClientConfig{Endpoint: server.URL})
	content, err := manager.StageLog(context.Background(), "demo", 65, "47")
	if err != nil {
		t.Fatalf("stage log should succeed: %v", err)
	}
	if !contains(content, "added 69 packages in 35s") {
		t.Fatalf("stage log should keep direct node output, got: %q", content)
	}
}

func TestStageChildStatusesReturnsRuntimeFields(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/job/demo/66/execution/node/48/wfapi/describe":
			_, _ = w.Write([]byte(`{"stageFlowNodes":[{"id":"481","name":"npm编译","status":"IN_PROGRESS","type":"STAGE","durationMillis":35000,"startTimeMillis":1710000003000}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	manager := NewManager(ClientConfig{Endpoint: server.URL})
	items, err := manager.StageChildStatuses(context.Background(), "demo", 66, "48")
	if err != nil {
		t.Fatalf("stage child statuses should succeed: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected one child stage, got %d", len(items))
	}
	if items[0].ID != "481" || items[0].Duration != 35000 || items[0].Start != 1710000003000 {
		t.Fatalf("stage child runtime fields should be preserved, got %#v", items[0])
	}
}
