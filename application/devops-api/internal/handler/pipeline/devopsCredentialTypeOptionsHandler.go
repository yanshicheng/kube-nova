// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package pipeline

import (
	"net/http"

	"github.com/yanshicheng/kube-nova/application/devops-api/internal/logic/pipeline"
	"github.com/yanshicheng/kube-nova/application/devops-api/internal/svc"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// 凭证类型选项
func DevopsCredentialTypeOptionsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := pipeline.NewDevopsCredentialTypeOptionsLogic(r.Context(), svcCtx)
		resp, err := l.DevopsCredentialTypeOptions()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
