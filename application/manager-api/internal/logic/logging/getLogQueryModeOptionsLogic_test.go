package logging

import (
	"context"
	"testing"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	logservice "github.com/yanshicheng/kube-nova/application/manager-rpc/client/logservice"
	"google.golang.org/grpc"
)

type fakeLogQueryModeRPC struct {
	logservice.LogService
	req  *logservice.LogQueryModeOptionsReq
	resp *logservice.LogQueryModeOptionsResp
	err  error
}

func (f *fakeLogQueryModeRPC) GetLogQueryModeOptions(ctx context.Context, in *logservice.LogQueryModeOptionsReq, opts ...grpc.CallOption) (*logservice.LogQueryModeOptionsResp, error) {
	f.req = in
	return f.resp, f.err
}

func TestGetLogQueryModeOptionsCallsLogRPC(t *testing.T) {
	fake := &fakeLogQueryModeRPC{resp: &logservice.LogQueryModeOptionsResp{Loki: true, Elasticsearch: false}}
	logic := NewGetLogQueryModeOptionsLogic(context.Background(), &svc.ServiceContext{LogRpc: fake})

	resp, err := logic.GetLogQueryModeOptions(&types.LogQueryModeOptionsRequest{ClusterUuid: "cluster-1"})
	if err != nil {
		t.Fatalf("GetLogQueryModeOptions() returned error: %v", err)
	}
	if fake.req == nil || fake.req.ClusterUuid != "cluster-1" {
		t.Fatalf("expected clusterUuid to be passed to LogRpc, got %+v", fake.req)
	}
	if !resp.Loki || resp.Elasticsearch {
		t.Fatalf("unexpected response: %+v", resp)
	}
}
