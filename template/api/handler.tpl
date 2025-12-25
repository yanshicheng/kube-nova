// Code scaffolded by goctl. Safe to edit.
// goctl {{.version}}

package {{.PkgName}}

import (
	"net/http"
	"github.com/go-playground/validator/v10"
	"github.com/mcuadros/go-defaults"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
	"github.com/yanshicheng/kube-nova/common/verify"
	"github.com/zeromicro/go-zero/rest/httpx"
	{{.ImportPackages}}
)

{{if .HasDoc}}{{.Doc}}{{end}}
func {{.HandlerName}}(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		{{if .HasRequest}}var req types.{{.RequestType}}
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
		{{end}}l := {{.LogicName}}.New{{.LogicType}}(r.Context(), svcCtx)
		{{if .HasResp}}resp, {{end}}err := l.{{.Call}}({{if .HasRequest}}&req{{end}})
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			{{if .HasResp}}httpx.OkJsonCtx(r.Context(), w, resp){{else}}httpx.Ok(w){{end}}
		}
	}
}