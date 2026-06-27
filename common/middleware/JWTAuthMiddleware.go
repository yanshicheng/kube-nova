package middleware

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/client/sysauthservice"
)

type Response struct {
	Code    int64  `json:"code"`    // 应用自定义状态码
	Data    any    `json:"data"`    // 响应数据
	Message string `json:"message"` // 消息描述
}

type JWTAuthMiddleware struct {
	auth sysauthservice.SysAuthService
}

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

func (m *JWTAuthMiddleware) Handle(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		//// 获取Token并校验
		token := r.Header.Get("Authorization")
		if token == "" {
			// 从 url 中获取 token
			urlToken := r.URL.Query().Get("token")
			// 组装Bearer
			token = "Bearer " + urlToken
		}

		res, err := m.auth.VerifyToken(r.Context(), &sysauthservice.VerifyTokenRequest{
			Token: token,
		})
		if err != nil {
			w.WriteHeader(http.StatusOK) // 200
			writeJSONResponse(w, 100002, "Token验证失败: "+err.Error(), nil)
			return
		}
		if !res.IsValid {
			w.WriteHeader(http.StatusOK) // 200
			writeJSONResponse(w, res.ErrorType, res.ErrorMessage, nil)
			return
		}

		ctx := context.WithValue(r.Context(), "username", res.Username)
		ctx = context.WithValue(ctx, "userId", res.UserId)
		ctx = context.WithValue(ctx, "roles", res.Roles)
		ctx = context.WithValue(ctx, "nickName", res.NickName)
		ctx = context.WithValue(ctx, "uuid", res.Uuid)

		// 获取请求的 url
		url := r.URL.Path
		// 获取 请求的方法
		method := r.Method
		isAuth, authErr := m.auth.ApiAuth(ctx, &sysauthservice.ApiAuthRequest{
			ApiPath:   url,
			ApiMethod: method,
			UserRoles: res.Roles,
		})
		if authErr != nil {
			w.WriteHeader(http.StatusUnauthorized) // 401
			writeJSONResponse(w, 100999, authErr.Error(), nil)
			return
		}
		if !isAuth.IsAuth {
			w.WriteHeader(http.StatusUnauthorized) // 401
			writeJSONResponse(w, 100999, "无权限访问!", nil)
			return
		}
		next(w, r.WithContext(ctx))
	}
}
