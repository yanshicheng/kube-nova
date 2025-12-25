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

// 上传文件
// 上传文件
func PodFileUploadHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1. 解析表单参数（文件上传使用 multipart/form-data）
		var req types.PodFileUploadReq
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

		// 3. 调用 Logic（传入 Request 用于读取上传文件）
		l := pod.NewPodFileUploadLogic(r.Context(), svcCtx, r)
		resp, err := l.PodFileUpload(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
