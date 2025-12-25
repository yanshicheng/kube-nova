package menu

import (
	"net/http"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/logic/menu"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func GetRolesMenuTreeHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := menu.NewGetRolesMenuTreeLogic(r.Context(), svcCtx)
		resp, err := l.GetRolesMenuTree()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
