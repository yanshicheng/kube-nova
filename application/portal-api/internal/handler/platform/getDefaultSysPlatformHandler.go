// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package platform

import (
	"net/http"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/logic/platform"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func GetDefaultSysPlatformHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := platform.NewGetDefaultSysPlatformLogic(r.Context(), svcCtx)
		resp, err := l.GetDefaultSysPlatform()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
