package errorx

import (
	"net/http"

	"github.com/yanshicheng/kube-nova/common/handler/errorx/types"
)

func ErrHandler(err error) (int, any) {
	code := CodeFromError(err)
	return http.StatusOK, types.Status{
		Code:    int32(code.Code()),
		Message: code.Message(),
	}
}
