package operator

import (
	"fmt"
	"strconv"
	"strings"
)

// FormatBytes 格式化字节数为可读格式
func FormatBytes(bytes int64) string {
	if bytes < 0 {
		return "无限制"
	}
	if bytes == 0 {
		return "0B"
	}

	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%dB", bytes)
	}

	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	units := []string{"KB", "MB", "GB", "TB", "PB", "EB"}
	if exp >= len(units) {
		exp = len(units) - 1
	}

	return fmt.Sprintf("%.2f%s", float64(bytes)/float64(div), units[exp])
}

// ParseStorageSize 解析存储大小字符串，支持单位: "10M", "5G", "1TB" 或纯数字（字节）
// 返回字节数
func ParseStorageSize(sizeStr string) (int64, error) {
	if sizeStr == "" {
		return 0, nil
	}

	sizeStr = strings.TrimSpace(strings.ToUpper(sizeStr))

	// 如果是纯数字，直接返回
	if num, err := strconv.ParseInt(sizeStr, 10, 64); err == nil {
		return num, nil
	}

	// 解析带单位的字符串
	units := map[string]int64{
		"B":  1,
		"KB": 1024,
		"MB": 1024 * 1024,
		"GB": 1024 * 1024 * 1024,
		"TB": 1024 * 1024 * 1024 * 1024,
		"PB": 1024 * 1024 * 1024 * 1024 * 1024,
		"K":  1024,
		"M":  1024 * 1024,
		"G":  1024 * 1024 * 1024,
		"T":  1024 * 1024 * 1024 * 1024,
		"P":  1024 * 1024 * 1024 * 1024 * 1024,
	}

	for unit, multiplier := range units {
		if strings.HasSuffix(sizeStr, unit) {
			numStr := strings.TrimSuffix(sizeStr, unit)
			numStr = strings.TrimSpace(numStr)

			// 支持浮点数
			num, err := strconv.ParseFloat(numStr, 64)
			if err != nil {
				return 0, fmt.Errorf("无效的大小格式: %s", sizeStr)
			}

			return int64(num * float64(multiplier)), nil
		}
	}

	return 0, fmt.Errorf("不支持的单位: %s", sizeStr)
}

// ValidateRoleID 验证角色ID是否有效
// Harbor角色: 1=管理员, 2=开发者, 3=访客, 4=维护者
func ValidateRoleID(roleID int64) bool {
	return roleID >= 1 && roleID <= 4
}

// GetRoleName 根据角色ID获取角色名称
func GetRoleName(roleID int64) string {
	roles := map[int64]string{
		1: "projectAdmin", // 项目管理员
		2: "developer",    // 开发者
		3: "guest",        // 访客
		4: "maintainer",   // 维护者
	}

	if name, ok := roles[roleID]; ok {
		return name
	}
	return "unknown"
}
