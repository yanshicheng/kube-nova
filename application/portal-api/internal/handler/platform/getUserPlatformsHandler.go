// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package platform

import (
	"net/http"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/logic/platform"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func GetUserPlatformsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.GetUserPlatformsRequest
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := platform.NewGetUserPlatformsLogic(r.Context(), svcCtx)
		resp, err := l.GetUserPlatforms(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
