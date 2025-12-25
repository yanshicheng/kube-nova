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

// 读取文件内容
func PodFileReadHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1. 解析参数
		var req types.PodFileReadReq
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

		// 3. 调用 Logic
		l := pod.NewPodFileReadLogic(r.Context(), svcCtx)
		resp, err := l.PodFileRead(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
