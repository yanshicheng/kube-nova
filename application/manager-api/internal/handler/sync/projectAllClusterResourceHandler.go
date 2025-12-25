package sync

import (
	"net/http"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/logic/sync"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// 同步所有项目集群配额
func ProjectAllClusterResourceHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := sync.NewProjectAllClusterResourceLogic(r.Context(), svcCtx)
		resp, err := l.ProjectAllClusterResource()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
