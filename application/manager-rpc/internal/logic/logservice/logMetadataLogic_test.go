package logservicelogic

import (
	"context"
	"testing"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
)

func TestListLogLabelKeysSupportsContainerAndSystemProviders(t *testing.T) {
	logic := NewListLogLabelKeysLogic(context.Background(), &svc.ServiceContext{})

	tests := []struct {
		name       string
		req        *pb.ListLogLabelKeysReq
		wantKey    string
		wantErrMsg string
	}{
		{
			name:    "container loki",
			req:     &pb.ListLogLabelKeysReq{SourceType: "container", ClusterUuid: "cluster-1", QueryMode: "loki"},
			wantKey: "namespace_name",
		},
		{
			name:    "container es",
			req:     &pb.ListLogLabelKeysReq{SourceType: "container", ClusterUuid: "cluster-1", QueryMode: "es"},
			wantKey: "container_name",
		},
		{
			name:    "system loki",
			req:     &pb.ListLogLabelKeysReq{SourceType: "system", ClusterUuid: "cluster-1", QueryMode: "loki"},
			wantKey: "node_name",
		},
		{
			name:    "system es",
			req:     &pb.ListLogLabelKeysReq{SourceType: "system", ClusterUuid: "cluster-1", QueryMode: "es"},
			wantKey: "source_host",
		},
		{
			name:       "platform unsupported",
			req:        &pb.ListLogLabelKeysReq{SourceType: "container", ClusterUuid: "cluster-1", QueryMode: "platform"},
			wantErrMsg: "当前模式不支持 labels 元数据",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := logic.ListLogLabelKeys(tt.req)
			if tt.wantErrMsg != "" {
				if err == nil || err.Error() != tt.wantErrMsg {
					t.Fatalf("expected error %q, got %v", tt.wantErrMsg, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("ListLogLabelKeys() returned error: %v", err)
			}
			if len(resp.Keys) == 0 {
				t.Fatal("expected non-empty keys")
			}
			found := false
			for _, key := range resp.Keys {
				if key == tt.wantKey {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("expected key %q in %+v", tt.wantKey, resp.Keys)
			}
		})
	}
}

func TestListLogLabelValuesSupportsContainerAndSystemProviders(t *testing.T) {
	logic := NewListLogLabelValuesLogic(context.Background(), &svc.ServiceContext{})

	tests := []struct {
		name       string
		req        *pb.ListLogLabelValuesReq
		wantValue  string
		wantErrMsg string
	}{
		{
			name:      "container loki",
			req:       &pb.ListLogLabelValuesReq{SourceType: "container", ClusterUuid: "cluster-1", QueryMode: "loki", LabelKey: "namespace_name", Namespace: "pay-sit"},
			wantValue: "pay-sit",
		},
		{
			name:      "container es",
			req:       &pb.ListLogLabelValuesReq{SourceType: "container", ClusterUuid: "cluster-1", QueryMode: "es", LabelKey: "log_type"},
			wantValue: "container",
		},
		{
			name:      "system loki",
			req:       &pb.ListLogLabelValuesReq{SourceType: "system", ClusterUuid: "cluster-1", QueryMode: "loki", LabelKey: "node_name", Hosts: []string{"node-a", "node-b"}},
			wantValue: "node-a",
		},
		{
			name:      "system es",
			req:       &pb.ListLogLabelValuesReq{SourceType: "system", ClusterUuid: "cluster-1", QueryMode: "es", LabelKey: "log_type"},
			wantValue: "system",
		},
		{
			name:       "platform unsupported",
			req:        &pb.ListLogLabelValuesReq{SourceType: "container", ClusterUuid: "cluster-1", QueryMode: "platform", LabelKey: "namespace_name"},
			wantErrMsg: "当前模式不支持 labels 元数据",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := logic.ListLogLabelValues(tt.req)
			if tt.wantErrMsg != "" {
				if err == nil || err.Error() != tt.wantErrMsg {
					t.Fatalf("expected error %q, got %v", tt.wantErrMsg, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("ListLogLabelValues() returned error: %v", err)
			}
			if len(resp.Values) == 0 {
				t.Fatal("expected non-empty values")
			}
			found := false
			for _, value := range resp.Values {
				if value == tt.wantValue {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("expected value %q in %+v", tt.wantValue, resp.Values)
			}
		})
	}
}
