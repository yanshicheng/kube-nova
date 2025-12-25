package utils

import (
	"math"
	"time"

	"github.com/yanshicheng/kube-nova/common/prometheusmanager/types"
)

// ParseTimeRange 解析时间范围参数
func ParseTimeRange(start, end, step string) *types.TimeRange {
	var timeRange *types.TimeRange

	if start != "" || end != "" {
		timeRange = &types.TimeRange{
			Step: step,
		}

		if start != "" {
			if t, err := time.Parse(time.RFC3339, start); err == nil {
				timeRange.Start = t
			}
		}

		if end != "" {
			if t, err := time.Parse(time.RFC3339, end); err == nil {
				timeRange.End = t
			}
		}

		// 如果没有提供时间范围，使用默认值（最近1小时）
		if timeRange.Start.IsZero() && timeRange.End.IsZero() {
			timeRange.End = time.Now()
			timeRange.Start = timeRange.End.Add(-1 * time.Hour)
		}

		// 设置默认步长
		if timeRange.Step == "" {
			timeRange.Step = "15s"
		}
	}

	return timeRange
}

// FormatTimestamp 格式化时间戳
func FormatTimestamp(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}

// sanitizeFloat64 将 NaN 和 Inf 转换为 0
func sanitizeFloat64(value float64) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}
	return value
}

// sanitizeInt 确保整数有效
func sanitizeInt(value int) int {
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
