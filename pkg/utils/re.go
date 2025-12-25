package utils

import (
	"errors"
	"regexp"
	"strings"
)

// 去掉结尾 ADN

func RemoveQueryADN(queryParts []string) string {
	// 将条件拼接成一个字符串
	query := strings.Join(queryParts, " ")

	// 使用正则表达式去除末尾的 AND
	re := regexp.MustCompile(`(?i)\s+AND\s*$`)
	return re.ReplaceAllString(query, "")
}

// ValidateNamespaceName 验证字符串是否符合规则
func ValidateNamespaceName(input string) error {
	// 检查长度是否超过 20 位
	if len(input) > 30 {
		return errors.New("长度超过 30 个字符")
	}

	// 检查是否符合规则：开头为数字，包含小写字母和中横线，中横线不能结尾
	matched, err := regexp.MatchString(`^[a-z][a-z0-9-]*[a-z0-9]$`, input)
	if err != nil {
		return err
	}
	if !matched {
		return errors.New("字符串不符合规则：开头必须是数字，结尾不能是中横线，只能包含小写字母、数字和中横线")
	}

	return nil
}
