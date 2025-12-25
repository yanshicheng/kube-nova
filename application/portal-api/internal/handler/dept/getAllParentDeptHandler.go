package dept

import (
	"net/http"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/logic/dept"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func GetAllParentDeptHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := dept.NewGetAllParentDeptLogic(r.Context(), svcCtx)
		resp, err := l.GetAllParentDept()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
