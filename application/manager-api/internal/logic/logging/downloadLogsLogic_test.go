package logging

import (
	"bytes"
	"context"
	"net/http"
	"testing"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	logservice "github.com/yanshicheng/kube-nova/application/manager-rpc/client/logservice"
	"google.golang.org/grpc"
)

type fakeLogRPC struct {
	logservice.LogService
	queryResp *logservice.QueryLogsResp
	queryErr  error
	queryReq  *logservice.QueryLogsReq
}

func (f *fakeLogRPC) QueryLogs(ctx context.Context, in *logservice.QueryLogsReq, opts ...grpc.CallOption) (*logservice.QueryLogsResp, error) {
	f.queryReq = in
	return f.queryResp, f.queryErr
}

func TestDownloadLogsUsesQueryLogsRPC(t *testing.T) {
	var body bytes.Buffer
	fake := &fakeLogRPC{queryResp: &logservice.QueryLogsResp{List: []*logservice.LogQueryRecord{{Message: "line-1"}, {Message: "line-2"}}}}
	logic := NewDownloadLogsLogic(context.Background(), &svc.ServiceContext{
		LogRpc: fake,
	}, nopResponseWriter{headers: http.Header{}, b: &body})

	err := logic.DownloadLogs(&types.LogDownloadRequest{
		ClusterUuid: "cluster-1",
		Start:       "2026-03-20T10:00:00Z",
		End:         "2026-03-20T11:00:00Z",
	})
	if err != nil {
		t.Fatalf("DownloadLogs() returned error: %v", err)
	}
	if body.String() != "line-1\nline-2\n" {
		t.Fatalf("expected downloaded body, got %q", body.String())
	}
	if fake.queryReq == nil {
		t.Fatal("expected QueryLogs rpc request to be captured")
	}
	if fake.queryReq.LogType != "container" {
		t.Fatalf("expected default logType container for download, got %q", fake.queryReq.LogType)
	}
}

type nopResponseWriter struct {
	headers http.Header
	b       *bytes.Buffer
}

func (n nopResponseWriter) Header() http.Header         { return n.headers }
func (n nopResponseWriter) Write(p []byte) (int, error) { return n.b.Write(p) }
func (n nopResponseWriter) WriteHeader(statusCode int)  {}
