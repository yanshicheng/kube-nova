package okx

import (
	"context"

	"github.com/yanshicheng/kube-nova/common/handler/errorx"
)

type Okx struct {
	Code    int    `json:"code"`
	Data    any    `json:"data"`
	Message string `json:"message"`
}

func OkHandler(ctx context.Context, data any) any {
	code := errorx.CodeFromError(errorx.OK)
	return Okx{
		Code:    code.Code(),
		Data:    data,
		Message: code.Message(),
	}
}
