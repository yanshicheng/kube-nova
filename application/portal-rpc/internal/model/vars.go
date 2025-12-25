package model

import "github.com/zeromicro/go-zero/core/stores/sqlx"

var ErrNotFound = sqlx.ErrNotFound

// 辅助函数：检查值是否为零值
func isZeroValue(value any) bool {
	switch v := value.(type) {
	case string:
		return v == ""
	case int, int32, int64, uint, uint32, uint64:
		return v != 0
	default:
		return value == nil
	}
}
