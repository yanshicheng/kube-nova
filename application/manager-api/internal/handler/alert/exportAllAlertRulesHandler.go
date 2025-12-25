package alert

import (
	"fmt"
	"net/http"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/logic/alert"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// ExportAllAlertRulesHandler 导出全部告警规则为 ZIP
func ExportAllAlertRulesHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := alert.NewExportAllAlertRulesLogic(r.Context(), svcCtx)

		// 生成 ZIP 文件
		zipData, fileName, err := l.ExportAllAlertRules()
		if err != nil {
			// 错误时返回 JSON（这是唯一使用 httpx 的地方）
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		// 直接设置响应头并写入二进制数据

		// 设置响应头
		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", fileName))
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(zipData)))
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")

		// 写入状态码
		w.WriteHeader(http.StatusOK)

		// 直接写入二进制数据
		if _, err := w.Write(zipData); err != nil {
			// 此时响应已经开始发送，无法再返回 JSON 错误
			// 只能记录日志
			fmt.Printf("写入 ZIP 数据失败: %v\n", err)
		}
	}
}
