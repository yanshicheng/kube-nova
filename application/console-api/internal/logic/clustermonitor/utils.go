package clustermonitor

import (
	"math"
)

// sanitizeFloat64 将 NaN 和 Inf 转换为 0
func sanitizeFloat64(value float64) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}
	return value
}

// sanitizeInt 确保整数有效
func sanitizeInt64(value int64) int64 {
	if value < 0 {
		return 0
	}
	return value
}

// sanitizeFloat64Slice 清理 float64 切片中的 NaN 值
func sanitizeFloat64Slice(values []float64) []float64 {
	result := make([]float64, len(values))
	for i, v := range values {
		result[i] = sanitizeFloat64(v)
	}
	return result
}
