package utils

import (
	"fmt"
	"time"
)

// 时间戳 转为 格式化
// 将时间戳转换为 time.Time 类型，并返回零时分秒的日期时间
func FormattedDate(timestamp int64) time.Time {
	// 将时间戳转换为 time.Time 类型
	convertedTime := time.Unix(timestamp, 0)

	// 将时间的时分秒置为 00:00:00
	formattedTime := time.Date(
		convertedTime.Year(),
		convertedTime.Month(),
		convertedTime.Day(),
		0, 0, 0, 0,
		convertedTime.Location(),
	)

	return formattedTime
}

// ConvertTimestampToFormattedTime 将时间戳转换为格式化时间
func ConvertTimestampToFormattedTime(timestamp int64, layout ...string) string {
	// 默认格式
	defaultLayout := "2006-01-02 15:04:05"

	// 如果提供了自定义格式，则使用第一个
	if len(layout) > 0 && layout[0] != "" {
		defaultLayout = layout[0]
	}

	// 将时间戳转换为 time.Time 对象
	t := time.Unix(timestamp, 0).Local()

	// 返回格式化时间
	return t.Format(defaultLayout)
}

// ParseStringToTime 将字符串转换为 time.Time，支持默认 layout
func ParseStringToTime(dateStr string, layout ...string) (time.Time, error) {
	// 默认 layout 格式
	defaultLayout := "2006-01-02 15:04:05"

	// 如果用户提供了 layout，则使用用户提供的格式；否则使用默认格式
	if len(layout) > 0 {
		defaultLayout = layout[0]
	}

	// 解析时间
	t, err := time.Parse(defaultLayout, dateStr)
	if err != nil {
		return time.Time{}, fmt.Errorf("解析时间失败: %w", err)
	}

	return t, nil
}
