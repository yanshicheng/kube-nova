package logging

import (
	"context"
	"testing"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	logservice "github.com/yanshicheng/kube-nova/application/manager-rpc/client/logservice"
	"google.golang.org/grpc"
)

type fakeLogMetadataRPC struct {
	logservice.LogService
	listKeysReq    *logservice.ListLogLabelKeysReq
	listKeysResp   *logservice.ListLogLabelKeysResp
	listKeysErr    error
	listValuesReq  *logservice.ListLogLabelValuesReq
	listValuesResp *logservice.ListLogLabelValuesResp
	listValuesErr  error
}

func (f *fakeLogMetadataRPC) ListLogLabelKeys(ctx context.Context, in *logservice.ListLogLabelKeysReq, opts ...grpc.CallOption) (*logservice.ListLogLabelKeysResp, error) {
	f.listKeysReq = in
	return f.listKeysResp, f.listKeysErr
}

func (f *fakeLogMetadataRPC) ListLogLabelValues(ctx context.Context, in *logservice.ListLogLabelValuesReq, opts ...grpc.CallOption) (*logservice.ListLogLabelValuesResp, error) {
	f.listValuesReq = in
	return f.listValuesResp, f.listValuesErr
}

func TestListLogLabelKeysCallsLogRPC(t *testing.T) {
	fake := &fakeLogMetadataRPC{listKeysResp: &logservice.ListLogLabelKeysResp{Keys: []string{"namespace_name", "pod_name"}}}
	logic := NewListLogLabelKeysLogic(context.Background(), &svc.ServiceContext{LogRpc: fake})

	resp, err := logic.ListLogLabelKeys(&types.LogLabelKeysRequest{
		SourceType:   "container",
		ClusterUuid:  "cluster-1",
		QueryMode:    "loki",
		WorkspaceId:  74,
		Namespace:    "pay-sit",
		Application:  "order-api",
		ResourceName: "order-api",
		Hosts:        []string{"node-a"},
	})
	if err != nil {
		t.Fatalf("ListLogLabelKeys() returned error: %v", err)
	}
	if fake.listKeysReq == nil {
		t.Fatal("expected ListLogLabelKeys rpc request to be captured")
	}
	if fake.listKeysReq.SourceType != "container" || fake.listKeysReq.QueryMode != "loki" || fake.listKeysReq.ClusterUuid != "cluster-1" {
		t.Fatalf("unexpected rpc request: %+v", fake.listKeysReq)
	}
	if len(resp.Keys) != 2 || resp.Keys[0] != "namespace_name" {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestListLogLabelValuesCallsLogRPC(t *testing.T) {
	fake := &fakeLogMetadataRPC{listValuesResp: &logservice.ListLogLabelValuesResp{Values: []string{"node-a", "node-b"}}}
	logic := NewListLogLabelValuesLogic(context.Background(), &svc.ServiceContext{LogRpc: fake})

	resp, err := logic.ListLogLabelValues(&types.LogLabelValuesRequest{
		SourceType:  "system",
		ClusterUuid: "cluster-1",
		QueryMode:   "es",
		LabelKey:    "node_name",
		Hosts:       []string{"node-a", "node-b"},
	})
	if err != nil {
		t.Fatalf("ListLogLabelValues() returned error: %v", err)
	}
	if fake.listValuesReq == nil {
		t.Fatal("expected ListLogLabelValues rpc request to be captured")
	}
	if fake.listValuesReq.SourceType != "system" || fake.listValuesReq.QueryMode != "es" || fake.listValuesReq.LabelKey != "node_name" {
		t.Fatalf("unexpected rpc request: %+v", fake.listValuesReq)
	}
	if len(resp.Values) != 2 || resp.Values[0] != "node-a" {
		t.Fatalf("unexpected response: %+v", resp)
	}
}
