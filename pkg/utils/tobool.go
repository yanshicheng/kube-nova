package utils

// IntToBool 根据 MySQL 中的 TINYINT(1) 值返回布尔值
func IntToBool(num int64) bool {
	// 在 MySQL 中，1 表示 true，0 表示 false
	return num == 1
}

// BoolToInt 根据布尔值返回 MySQL 中的 TINYINT(1) 值
func BoolToInt(b bool) int64 {
	if b {
		return 1 // true
	}
	return 0 // false
}
