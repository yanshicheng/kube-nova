package pod

import (
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/mcuadros/go-defaults"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/logic/pod"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
	"github.com/yanshicheng/kube-nova/common/verify"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// 下载 Pod 日志
func PodLogsDownloadHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1. 解析参数
		var req types.PodLogsDownloadReq
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

		// 3. 调用 Logic（传入 ResponseWriter 用于流式传输）
		l := pod.NewPodLogsDownloadLogic(r.Context(), svcCtx, w, r)
		if err := l.PodLogsDownload(&req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		}
		// 注意：不需要调用 httpx.Ok()，因为 Logic 已经写入了响应
	}
}
