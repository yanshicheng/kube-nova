package app

import (
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/mcuadros/go-defaults"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/logic/app"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
	"github.com/yanshicheng/kube-nova/common/verify"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// 添加或更新集群应用配置，如果应用已存在则更新，否则创建新应用
func AddClusterAppHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AddClusterAppRequest
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		// 设置默认值
		defaults.SetDefaults(&req)
		// validator验证
		if err := svcCtx.Validator.Validate.StructCtx(r.Context(), &req); err != nil {
			strErr := verify.RemoveTopSaStr(err.(validator.ValidationErrors), svcCtx.Validator.Translator)
			httpx.ErrorCtx(r.Context(), w, errorx.New(40020, strErr))
			return
		}
		l := app.NewAddClusterAppLogic(r.Context(), svcCtx)
		resp, err := l.AddClusterApp(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
