package pod

import (
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
	"github.com/zeromicro/go-zero/rest/httpx"
)

// 实时日志流（WebSocket）
// 实时日志流（WebSocket）
func PodLogsStreamHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1. 解析参数
		var req types.PodLogsStreamReq
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

		// 3. 升级为 WebSocket
		ws, err := wsutil.UpgradeWebSocket(w, r)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		defer ws.Close()

		// 4. 启动心跳
		ws.StartPingPong(30 * time.Second)

		// 5. 调用 Logic
		l := pod.NewPodLogsStreamLogic(r.Context(), svcCtx, ws)
		if err := l.PodLogsStream(&req); err != nil {
			ws.SendError(err)
		}
	}
}
