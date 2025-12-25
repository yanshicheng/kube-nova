package middleware

import (
	"bytes"
	"fmt"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/yanshicheng/kube-nova/common/handler/errorx"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// PanicRecoveryMiddleware 捕获并恢复panic
func PanicRecoveryMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				// 获取堆栈信息
				stack := debug.Stack()

				// 捕获的错误类型
				var errMsg string
				switch e := err.(type) {
				case string:
					errMsg = e
				case error:
					errMsg = e.Error()
				default:
					errMsg = fmt.Sprintf("%v", err)
				}

				// 记录详细日志
				logx.Errorw("Panic error",
					logx.Field("time", time.Now().Format(time.RFC3339)),
					logx.Field("error", errMsg),
					logx.Field("stack", string(stack)),
					logx.Field("url", r.URL.String()),
					logx.Field("method", r.Method),
					logx.Field("headers", formatHeaders(r.Header)),
				)

				// 统一返回友好的错误响应
				httpx.ErrorCtx(r.Context(), w, errorx.ServerErr)
			}
		}()

		// 调用下一个处理函数
		next(w, r)
	}
}

// formatHeaders 格式化HTTP头部为字符串，便于日志记录
func formatHeaders(headers http.Header) string {
	var buffer bytes.Buffer
	for key, values := range headers {
		for _, value := range values {
			buffer.WriteString(fmt.Sprintf("%s: %s; ", key, value))
		}
	}
	return buffer.String()
}
