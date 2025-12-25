package pod

import (
	"context"
	"net/http"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/mcuadros/go-defaults"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/common/wsutil"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/logic/pod"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
	"github.com/yanshicheng/kube-nova/common/verify"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// 交互式执行命令（WebSocket）
func PodExecHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1. 解析参数
		var req types.PodExecReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		// 2. 设置默认值并验证
		defaults.SetDefaults(&req)
		if err := svcCtx.Validator.Validate.StructCtx(r.Context(), &req); err != nil {
			strErr := verify.RemoveTopSaStr(err.(validator.ValidationErrors), svcCtx.Validator.Translator)
			httpx.ErrorCtx(r.Context(), w, errorx.New(40020, strErr))
			return
		}

		logx.Infof("[终端] 请求参数: workloadId=%d, pod=%s, container=%s, command=%v, rows=%d, cols=%d",
			req.WorkloadId, req.PodName, req.Container, req.Command, req.Rows, req.Cols)

		// 3. 升级为 WebSocket
		ws, err := wsutil.UpgradeWebSocket(w, r)
		if err != nil {
			logx.Errorf("[终端] WebSocket 升级失败: %v", err)
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		defer func() {
			if !ws.IsClosed() {
				logx.Info("[终端] 强制关闭 WebSocket 连接")
				ws.Close()
			}
		}()

		logx.Infof("[终端] WebSocket 连接已建立: %s", r.RemoteAddr)

		// 4. 启动心跳
		ws.StartPingPong(30 * time.Second)

		ctx, cancel := context.WithTimeout(r.Context(), 1*time.Hour)
		defer cancel()

		go func() {
			<-ctx.Done()
			if !ws.IsClosed() {
				logx.Info("[终端] 上下文取消，关闭 WebSocket")
				ws.Close()
			}
		}()

		go func() {
			<-ws.CloseChan()
			logx.Info("[终端] WebSocket 关闭，取消上下文")
			cancel()
		}()

		// 5. 调用 Logic（使用带超时的上下文）
		l := pod.NewPodExecLogic(ctx, svcCtx, ws)

		func() {
			defer func() {
				if r := recover(); r != nil {
					logx.Errorf("[终端] Logic 发生 panic: %v", r)
					if !ws.IsClosed() {
						ws.SendErrorWithCode("PANIC", "服务异常")
					}
				}
			}()

			if err := l.PodExec(&req); err != nil {
				logx.Errorf("[终端] 执行失败: %v", err)
				if !ws.IsClosed() {
					ws.SendError(err)
				}
			}
		}()

		select {
		case <-ws.CloseChan():
			logx.Info("[终端] WebSocket 正常关闭")
		case <-ctx.Done():
			if ctx.Err() == context.DeadlineExceeded {
				logx.Info("[终端] 连接超时（1小时限制）")
				if !ws.IsClosed() {
					ws.SendErrorWithCode("TIMEOUT", "连接超时，已自动断开")
				}
			} else {
				logx.Info("[终端] 上下文取消")
			}
		}

		logx.Infof("[终端] 会话结束: %s", r.RemoteAddr)
	}
}
