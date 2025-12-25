// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package alert

import (
	"net/http"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/logic/alert"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// 搜索告警规则文件列表
func SearchAlertRuleFileHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.SearchAlertRuleFileRequest
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := alert.NewSearchAlertRuleFileLogic(r.Context(), svcCtx)
		resp, err := l.SearchAlertRuleFile(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
