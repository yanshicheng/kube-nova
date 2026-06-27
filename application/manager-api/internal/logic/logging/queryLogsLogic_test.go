package logging

import (
	"context"
	"testing"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	logservice "github.com/yanshicheng/kube-nova/application/manager-rpc/client/logservice"
	"google.golang.org/grpc"
)

type fakeQueryLogRPC struct {
	logservice.LogService
	queryResp *logservice.QueryLogsResp
	queryErr  error
	queryReq  *logservice.QueryLogsReq
}

func (f *fakeQueryLogRPC) QueryLogs(ctx context.Context, in *logservice.QueryLogsReq, opts ...grpc.CallOption) (*logservice.QueryLogsResp, error) {
	f.queryReq = in
	return f.queryResp, f.queryErr
}

func TestQueryLogsMapsFullRequestFields(t *testing.T) {
	fakeRPC := &fakeQueryLogRPC{queryResp: &logservice.QueryLogsResp{}}
	logic := NewQueryLogsLogic(context.Background(), &svc.ServiceContext{LogRpc: fakeRPC})

	_, err := logic.QueryLogs(&types.LogQueryRequest{
		LogType:       "container",
		ClusterUuid:   "684f1e17-d82d-498f-9a95-6de9eb84b8bf",
		ProjectUuid:   "86c7ff28-659b-4501-ae48-f4acb2de435f",
		Namespace:     "pay-sit",
		PodName:       "test-deployment*",
		ContainerName: "test-container",
		Host:          "node-a",
		Message:       "学习啊",
		Mode:          "form",
		Start:         "1774921958492",
		End:           "1774923758492",
		Direction:     "backward",
		Limit:         100,
	})
	if err != nil {
		t.Fatalf("QueryLogs() returned error: %v", err)
	}
	if fakeRPC.queryReq == nil {
		t.Fatal("expected QueryLogs rpc request to be captured")
	}
	if fakeRPC.queryReq.LogType != "container" {
		t.Fatalf("expected logType mapped, got %q", fakeRPC.queryReq.LogType)
	}
	if fakeRPC.queryReq.ProjectUuid != "86c7ff28-659b-4501-ae48-f4acb2de435f" {
		t.Fatalf("expected projectUuid mapped, got %q", fakeRPC.queryReq.ProjectUuid)
	}
	if fakeRPC.queryReq.Host != "node-a" {
		t.Fatalf("expected host mapped, got %q", fakeRPC.queryReq.Host)
	}
	if fakeRPC.queryReq.QueryText != "学习啊" || fakeRPC.queryReq.Keyword != "学习啊" {
		t.Fatalf("expected message mapped to query text and keyword, got queryText=%q keyword=%q", fakeRPC.queryReq.QueryText, fakeRPC.queryReq.Keyword)
	}
	if fakeRPC.queryReq.Mode != "form" {
		t.Fatalf("expected mode mapped, got %q", fakeRPC.queryReq.Mode)
	}
	if fakeRPC.queryReq.StartTime != 1774921958492 || fakeRPC.queryReq.EndTime != 1774923758492 {
		t.Fatalf("expected millis timestamps mapped, got start=%d end=%d", fakeRPC.queryReq.StartTime, fakeRPC.queryReq.EndTime)
	}
}

func TestQueryLogsUsesLegacySourceTypeAsLogType(t *testing.T) {
	fakeRPC := &fakeQueryLogRPC{queryResp: &logservice.QueryLogsResp{}}
	logic := NewQueryLogsLogic(context.Background(), &svc.ServiceContext{LogRpc: fakeRPC})

	_, err := logic.QueryLogs(&types.LogQueryRequest{
		LegacyLogType: "system",
		ClusterUuid:   "cluster-1",
		SourceType:    "kernel",
		Mode:          "form",
		Start:         "1774921958492",
		End:           "1774923758492",
	})
	if err != nil {
		t.Fatalf("QueryLogs() returned error: %v", err)
	}
	if fakeRPC.queryReq.LogType != "system" {
		t.Fatalf("expected legacy sourceType to map to logType, got %q", fakeRPC.queryReq.LogType)
	}
	if fakeRPC.queryReq.SourceType != "kernel" {
		t.Fatalf("expected logSourceType to stay on sourceType, got %q", fakeRPC.queryReq.SourceType)
	}
}

func TestQueryLogsCodeModeRequiresExpr(t *testing.T) {
	logic := NewQueryLogsLogic(context.Background(), &svc.ServiceContext{LogRpc: &fakeQueryLogRPC{queryResp: &logservice.QueryLogsResp{}}})

	_, err := logic.QueryLogs(&types.LogQueryRequest{
		LogType:     "system",
		ClusterUuid: "cluster-1",
		Mode:        "code",
		Start:       "1774921958492",
		End:         "1774923758492",
	})
	if err == nil || err.Error() != "code 模式下 expr 不能为空" {
		t.Fatalf("expected code mode expr error, got %v", err)
	}
}

func TestQueryLogsPromotesStructuredFormQueryToCode(t *testing.T) {
	fakeRPC := &fakeQueryLogRPC{queryResp: &logservice.QueryLogsResp{}}
	logic := NewQueryLogsLogic(context.Background(), &svc.ServiceContext{LogRpc: fakeRPC})

	_, err := logic.QueryLogs(&types.LogQueryRequest{
		LogType:     "container",
		ClusterUuid: "cluster-1",
		QueryMode:   "loki",
		Mode:        "form",
		QueryText:   `{log_type="container"} | json | pod_name="test-deployment-1"`,
		Start:       "1774921958492",
		End:         "1774923758492",
	})
	if err != nil {
		t.Fatalf("QueryLogs() returned error: %v", err)
	}
	if fakeRPC.queryReq == nil {
		t.Fatal("expected QueryLogs rpc request to be captured")
	}
	if fakeRPC.queryReq.Mode != "code" {
		t.Fatalf("expected mode promoted to code, got %q", fakeRPC.queryReq.Mode)
	}
	if fakeRPC.queryReq.Expr != `{log_type="container"} | json | pod_name="test-deployment-1"` {
		t.Fatalf("expected expr promoted from queryText, got %q", fakeRPC.queryReq.Expr)
	}
}
