package user

import (
	"net/http"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/logic/user"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func GetSysUserInfoHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := user.NewGetSysUserInfoLogic(r.Context(), svcCtx)
		resp, err := l.GetSysUserInfo()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
