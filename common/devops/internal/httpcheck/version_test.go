package httpcheck

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	devopstypes "github.com/yanshicheng/kube-nova/common/devops/types"
)

func TestKubeNovaVersionExtraction(t *testing.T) {
	// 模拟 portal-api 的 /portal/version 响应（经过 OkHandler 包装）
	versionServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"code":    0,
			"data":    map[string]any{"version": "1.0.0"},
			"message": "OK",
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer versionServer.Close()

	sp := ServiceProvider{
		ProductName: "kube-nova",
		Probes: []Probe{
			{Path: "/portal/version", PlainVersion: false, Capability: "versionApi"},
			{Path: "/healthz", Capability: "healthApi"},
		},
	}

	req := devopstypes.Request{
		Channel: devopstypes.Channel{
			Endpoint: versionServer.URL,
		},
	}
	result := sp.TestConnection(context.Background(), req)

	t.Logf("Success: %v", result.Success)
	t.Logf("Message: %s", result.Message)
	t.Logf("Metadata.ProductVersion: %q", result.Metadata.ProductVersion)
	t.Logf("Metadata.Version: %q", result.Metadata.Version)

	metadataJSON := devopstypes.MetadataJSON(result.Metadata)
	t.Logf("Metadata JSON: %s", metadataJSON)

	var parsed map[string]any
	if err := json.Unmarshal([]byte(metadataJSON), &parsed); err != nil {
		t.Fatalf("metadata JSON 解析失败: %v", err)
	}

	// 关键断言：metadata JSON 中必须有 version 字段
	version, hasVersion := parsed["version"]
	if !hasVersion {
		t.Fatal("metadata JSON 中缺少 version 字段")
	}
	if version != "1.0.0" {
		t.Fatalf("version = %q, want %q", version, "1.0.0")
	}
	t.Logf("version 字段正确: %v", version)
}

func TestKubeNovaVersionExtraction_HealthzFails(t *testing.T) {
	// 模拟 portal-api 有 /portal/version 但没有 /healthz
	versionServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/portal/version" {
			resp := map[string]any{
				"code":    0,
				"data":    map[string]any{"version": "1.0.0"},
				"message": "OK",
			}
			json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(404)
	}))
	defer versionServer.Close()

	sp := ServiceProvider{
		ProductName: "kube-nova",
		Probes: []Probe{
			{Path: "/portal/version", PlainVersion: false, Capability: "versionApi"},
			{Path: "/healthz", Capability: "healthApi"},
		},
	}

	req := devopstypes.Request{
		Channel: devopstypes.Channel{
			Endpoint: versionServer.URL,
		},
	}
	result := sp.TestConnection(context.Background(), req)

	t.Logf("Success: %v", result.Success)
	t.Logf("Message: %s", result.Message)
	t.Logf("Metadata.ProductVersion: %q", result.Metadata.ProductVersion)
	t.Logf("Metadata.Version: %q", result.Metadata.Version)

	metadataJSON := devopstypes.MetadataJSON(result.Metadata)
	t.Logf("Metadata JSON: %s", metadataJSON)

	var parsed map[string]any
	if err := json.Unmarshal([]byte(metadataJSON), &parsed); err != nil {
		t.Fatalf("metadata JSON 解析失败: %v", err)
	}

	version, hasVersion := parsed["version"]
	if !hasVersion {
		t.Fatal("metadata JSON 中缺少 version 字段")
	}
	if version != "1.0.0" {
		t.Fatalf("version = %q, want %q", version, "1.0.0")
	}
	t.Logf("healthz 失败时 version 字段仍然正确: %v", version)
}
