package {{.pkg}}

import "github.com/zeromicro/go-zero/core/stores/sqlx"

var ErrNotFound = sqlx.ErrNotFound

// isZeroValue 辅助函数：检查值是否为零值
func isZeroValue(value any) bool {
    if value == nil {
        return true
    }
    switch v := value.(type) {
    case string:
        return v == ""
    case int:
        return v == 0
    case int8:
        return v == 0
    case int16:
        return v == 0
    case int32:
        return v == 0
    case int64:
        return v == 0
    case uint:
        return v == 0
    case uint8:
        return v == 0
    case uint16:
        return v == 0
    case uint32:
        return v == 0
    case uint64:
        return v == 0
    case float32:
        return v == 0
    case float64:
        return v == 0
    case bool:
        return !v
    default:
        return false
    }
}
