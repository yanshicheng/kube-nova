package utils

import (
	"errors"
	"regexp"
	"strings"
)

// ValidateVersionName 验证版本名称格式
// 规则：
// 1. 只能英文开头
// 2. 包含英文、中文、'-'
// 3. 不能以 '-' 结尾
func ValidateVersionName(name string) error {
	if name == "" {
		return errors.New("版本名称不能为空")
	}

	// 检查是否以英文字母开头
	if !regexp.MustCompile(`^[a-zA-Z]`).MatchString(name) {
		return errors.New("版本名称必须以英文字母开头")
	}

	// 检查是否以 '-' 结尾
	if strings.HasSuffix(name, "-") {
		return errors.New("版本名称不能以 '-' 结尾")
	}

	// 检查是否只包含英文、中文、'-'
	// 英文: a-zA-Z
	// 中文: \x{4e00}-\x{9fa5}
	// 连字符: -
	validPattern := regexp.MustCompile(`^[a-zA-Z][\x{4e00}-\x{9fa5}a-zA-Z0-9\-]*$`)
	if !validPattern.MatchString(name) {
		return errors.New("版本名称只能包含英文、中文、数字和 '-'")
	}

	return nil
}
