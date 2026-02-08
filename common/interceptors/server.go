package interceptors

import (
	"context"
	"strconv"
	"strings"

	"github.com/yanshicheng/kube-nova/common/handler/errorx"
	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// ServerMetadataInterceptor 从 gRPC metadata 中提取用户信息并注入到 context
func ServerMetadataInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		// 从 metadata 中提取用户信息
		if md, ok := metadata.FromIncomingContext(ctx); ok {
			// 提取 userId
			if userIds := md.Get("user-id"); len(userIds) > 0 {
				if userId, err := strconv.ParseUint(userIds[0], 10, 64); err == nil {
					ctx = context.WithValue(ctx, "userId", userId)
				}
			}

			// 提取 username
			if usernames := md.Get("username"); len(usernames) > 0 {
				ctx = context.WithValue(ctx, "username", usernames[0])
			}

			// 提取 roles
			if roles := md.Get("roles"); len(roles) > 0 {
				// roles 是逗号分隔的字符串，需要拆分
				rolesList := strings.Split(roles[0], ",")
				ctx = context.WithValue(ctx, "roles", rolesList)
			}

			// 提取 nickName
			if nickNames := md.Get("nick-name"); len(nickNames) > 0 {
				ctx = context.WithValue(ctx, "nickName", nickNames[0])
			}

			// 提取 uuid
			if uuids := md.Get("uuid"); len(uuids) > 0 {
				ctx = context.WithValue(ctx, "uuid", uuids[0])
			}
		}

		// 调用实际的 RPC 方法
		return handler(ctx, req)
	}
}

// ServerErrorInterceptor 服务端错误处理拦截器
func ServerErrorInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		resp, err = handler(ctx, req)
		if err != nil {
			// 跳过健康检查请求的错误日志
			if !isHealthCheckRequest(info.FullMethod, req) {
				logx.WithContext(ctx).Errorf("【RPC SRV ERR】 %v", err)
			}
		}
		// TODO
		return resp, errorx.FromError(err).Err()
	}
}

// isHealthCheckRequest 判断是否为健康检查请求
func isHealthCheckRequest(method string, req interface{}) bool {
	// 检查是否为 GetClusterAuthInfo 方法且 ClusterUuid 为 __health_check__
	if strings.HasSuffix(method, "/GetClusterAuthInfo") {
		if r, ok := req.(interface{ GetClusterUuid() string }); ok {
			return r.GetClusterUuid() == "__health_check__"
		}
	}
	return false
}
