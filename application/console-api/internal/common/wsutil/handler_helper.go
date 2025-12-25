package wsutil

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// WSHandlerFunc WebSocket 处理函数类型
type WSHandlerFunc func(ws *WSConnection) error

func HandleWebSocket(handler WSHandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 1*time.Hour)
		defer cancel()

		// 升级为 WebSocket 连接
		ws, err := UpgradeWebSocket(w, r)
		if err != nil {
			logx.Errorf("websocket upgrade failed: %v", err)
			httpx.ErrorCtx(r.Context(), w, fmt.Errorf("websocket升级失败"))
			return
		}

		defer func() {
			if !ws.IsClosed() {
				logx.Info("force closing websocket connection in defer")
				err := ws.Close()
				if err != nil {
					return
				}
			}
		}()

		ws.StartPingPong(30 * time.Second)

		go func() {
			<-ctx.Done()
			if !ws.IsClosed() {
				logx.Info("context cancelled, closing websocket")
				err := ws.Close()
				if err != nil {
					return
				}
			}
		}()

		// 执行业务处理（带 panic 恢复）
		func() {
			defer func() {
				if r := recover(); r != nil {
					logx.Errorf("websocket handler panic: %v", r)
					if !ws.IsClosed() {
						err := ws.SendErrorWithCode("PANIC", fmt.Sprintf("服务异常: %v", r))
						if err != nil {
							return
						}
					}
				}
			}()

			if err := handler(ws); err != nil {
				if !ws.IsClosed() {
					logx.Errorf("websocket handler error: %v", err)
					err := ws.SendError(err)
					if err != nil {
						return
					}
				}
			}
		}()

		// 等待连接关闭或超时
		select {
		case <-ws.CloseChan():
			logx.Info("websocket connection closed normally")
		case <-ctx.Done():
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				logx.Info("websocket connection timeout (1 hour limit)")
				if !ws.IsClosed() {
					err := ws.SendErrorWithCode("TIMEOUT", "连接超时，已自动断开")
					if err != nil {
						return
					}
				}
			} else {
				logx.Info("websocket connection cancelled")
			}
		}

		logx.Infof("websocket connection finished for %s", r.RemoteAddr)
	}
}

type WSHandlerWithContextFunc func(ctx context.Context, ws *WSConnection) error

func HandleWebSocketWithContext(handler WSHandlerWithContextFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 1*time.Hour)
		defer cancel()

		ws, err := UpgradeWebSocket(w, r)
		if err != nil {
			logx.Errorf("websocket upgrade failed: %v", err)
			httpx.ErrorCtx(r.Context(), w, fmt.Errorf("websocket升级失败"))
			return
		}

		defer func() {
			if !ws.IsClosed() {
				logx.Info("force closing websocket connection in defer")
				err := ws.Close()
				if err != nil {
					return
				}
			}
		}()

		ws.StartPingPong(30 * time.Second)

		go func() {
			<-ctx.Done()
			if !ws.IsClosed() {
				logx.Info("context cancelled, closing websocket")
				err := ws.Close()
				if err != nil {
					return
				}
			}
		}()

		func() {
			defer func() {
				if r := recover(); r != nil {
					logx.Errorf("websocket handler panic: %v", r)
					if !ws.IsClosed() {
						err := ws.SendErrorWithCode("PANIC", fmt.Sprintf("服务异常: %v", r))
						if err != nil {
							return
						}
					}
				}
			}()

			if err := handler(ctx, ws); err != nil {
				if !ws.IsClosed() {
					logx.Errorf("websocket handler error: %v", err)
					err := ws.SendError(err)
					if err != nil {
						return
					}
				}
			}
		}()

		select {
		case <-ws.CloseChan():
			logx.Info("websocket connection closed normally")
		case <-ctx.Done():
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				logx.Info("websocket connection timeout")
				if !ws.IsClosed() {
					err := ws.SendErrorWithCode("TIMEOUT", "连接超时，已自动断开")
					if err != nil {
						return
					}
				}
			} else {
				logx.Info("websocket connection cancelled")
			}
		}

		logx.Infof("websocket connection finished for %s", r.RemoteAddr)
	}
}
