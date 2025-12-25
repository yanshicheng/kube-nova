package middleware

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/client/sysauthservice"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"
)

// JWTAuthMiddleware 处理JWT鉴权中间件
type JWTAuthMiddleware struct {
	auth sysauthservice.SysAuthService
}

// Response 定义返回的结构体，仅包含业务状态码和消息
type Response struct {
	Code    int64  `json:"code"`    // 应用自定义状态码
	Data    any    `json:"data"`    // 响应数据
	Message string `json:"message"` // 消息描述
}

// NewJWTAuthMiddleware 创建一个JWTAuthMiddleware实例
func NewJWTAuthMiddleware(auth sysauthservice.SysAuthService) *JWTAuthMiddleware {
	return &JWTAuthMiddleware{
		auth: auth,
	}
}

// writeJSONResponse 统一封装JSON响应，不包含HTTP状态码
func writeJSONResponse(w http.ResponseWriter, code int64, message string, data any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(Response{
		Code:    code,
		Message: message,
		Data:    data,
	})
}

// Handle 处理JWT鉴权逻辑
func (m *JWTAuthMiddleware) Handle(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		//// 获取Token并校验
		token := r.Header.Get("Authorization")

		// 调用RPC服务验证Token
		res, err := m.auth.VerifyToken(r.Context(), &pb.VerifyTokenRequest{
			Token: token,
		})
		if err != nil {
			w.WriteHeader(http.StatusUnauthorized) // 设置 HTTP 状态码
			writeJSONResponse(w, 100002, "Token验证失败: "+err.Error(), nil)
			return
		}

		if !res.IsValid {
			w.WriteHeader(http.StatusUnauthorized) // 设置 HTTP 状态码
			writeJSONResponse(w, res.ErrorType, res.ErrorMessage, nil)
			return
		}
		// token 验证通过 设置 ctx 吧 账号信息添加到 ctx 中 account ，等
		ctx := context.WithValue(r.Context(), "username", res.Username)
		ctx = context.WithValue(ctx, "userId", res.UserId)
		ctx = context.WithValue(ctx, "roles", res.Roles)
		ctx = context.WithValue(ctx, "nickName", res.NickName)
		ctx = context.WithValue(ctx, "uuid", res.Uuid)
		next(w, r.WithContext(ctx))
	}
}
