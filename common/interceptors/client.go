package interceptors

import (
	"context"
	"fmt"
	"strings"

	"github.com/yanshicheng/kube-nova/common/handler/errorx"
	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// ClientMetadataInterceptor 将 context 中的用户信息注入到 gRPC metadata
func ClientMetadataInterceptor() grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {

		// 提取 userId
		if userId, ok := ctx.Value("userId").(uint64); ok {
			ctx = metadata.AppendToOutgoingContext(ctx, "user-id", fmt.Sprintf("%d", userId))
		}

		// 提取 username
		if username, ok := ctx.Value("username").(string); ok {
			ctx = metadata.AppendToOutgoingContext(ctx, "username", username)
		}

		// 提取 roles
		if roles, ok := ctx.Value("roles").([]string); ok {
			rolesStr := strings.Join(roles, ",")
			ctx = metadata.AppendToOutgoingContext(ctx, "roles", rolesStr)
		}

		// 调用下一个拦截器
		err := invoker(ctx, method, req, reply, cc, opts...)
		if err != nil {
			logx.Errorf("❌ [ClientMetadata] 调用失败: %v", err)
		}
		return err
	}
}

// ClientErrorInterceptor 错误处理拦截器
func ClientErrorInterceptor() grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		err := invoker(ctx, method, req, reply, cc, opts...)
		if err != nil {
			grpcStatus, _ := status.FromError(err)
			xc := errorx.GrpcStatusToErrorX(grpcStatus)
			return xc
		}
		return nil
	}
}
