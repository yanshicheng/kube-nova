package errorx

import "testing"

func TestGRPCStatusKeepsCustomBusinessMessage(t *testing.T) {
	grpcStatus := FromError(Msg("业务错误"))
	got := GrpcStatusToErrorX(grpcStatus)
	if got.Code() != 999999 {
		t.Fatalf("custom business code should be preserved, got %d", got.Code())
	}
	if got.Message() != "业务错误" {
		t.Fatalf("custom business message should be preserved, got %s", got.Message())
	}
}
