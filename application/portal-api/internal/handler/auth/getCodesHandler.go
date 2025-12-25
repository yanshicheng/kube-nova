package auth

import (
	"net/http"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/logic/auth"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func GetCodesHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := auth.NewGetCodesLogic(r.Context(), svcCtx)
		resp, err := l.GetCodes()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
