package sitemessage

import (
	"net/http"
	"time"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/logic/common/wsutil"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/logic/sitemessage"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func SiteMessageWSConnectHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.SiteMessageWSConnectRequest
		if err := httpx.Parse(r, &req); err != nil {
			logx.Errorf("解析请求失败: %v", err)
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		ws, err := wsutil.UpgradeWebSocket(w, r)
		if err != nil {
			logx.Errorf("WebSocket 升级失败: %v", err)
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		logx.Info("====== [DEBUG] WebSocket 升级成功！ ======")

		defer func() {
			if !ws.IsClosed() {
				ws.Close()
			}
		}()

		ws.StartPingPong(30 * time.Second)

		l := sitemessage.NewSiteMessageWSConnectLogic(r.Context(), svcCtx, ws)

		if err := l.SiteMessageWSConnect(&req); err != nil {
			if !ws.IsClosed() && !ws.IsClientClosed() {
				logx.Errorf("WebSocket 处理错误: %v", err)
				ws.SendError(err)
			}
		}

	}
}
