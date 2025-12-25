package webhook

import (
	"context"
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/mcuadros/go-defaults"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/logic/webhook"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
	"github.com/yanshicheng/kube-nova/common/verify"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// 接收 Alertmanager 告警
func ReceiveAlertmanagerWebhookHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AlertmanagerWebhook
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		// 设置默认值
		defaults.SetDefaults(&req)
		// validator验证
		if err := svcCtx.Validator.Validate.StructCtx(r.Context(), &req); err != nil {
			strErr := verify.RemoveTopSaStr(err.(validator.ValidationErrors), svcCtx.Validator.Translator)
			httpx.ErrorCtx(r.Context(), w, errorx.New(40020, strErr))
			return
		}
		// 优先从 URL 参数获取 Token
		token := r.URL.Query().Get("token")
		if token == "" {
			// 如果 URL 参数没有，尝试从 Header 获取
			token = r.Header.Get("Authorization")
		}

		ctx := context.WithValue(r.Context(), "token", token)

		l := webhook.NewReceiveAlertmanagerWebhookLogic(ctx, svcCtx)
		resp, err := l.ReceiveAlertmanagerWebhook(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
