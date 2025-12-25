package utils

import (
	"errors"
	"fmt"
	"net"
	"regexp"
	"strings"
)

// ValidateAndCleanAddresses 验证并清理地址列表
// 支持: IPv4, IPv6, 域名
func ValidateAndCleanAddresses(input string) (string, error) {
	if input == "" {
		return "", errors.New("输入不能为空")
	}

	// 分隔并清理
	parts := strings.Split(input, ",")
	validAddresses := make([]string, 0, len(parts))

	for idx, part := range parts {
		trimmed := strings.TrimSpace(part)

		// 跳过空项
		if trimmed == "" {
			return "", fmt.Errorf("第 %d 个位置存在空地址", idx+1)
		}

		// 验证地址
		if !isValidAddress(trimmed) {
			return "", fmt.Errorf("第 %d 个位置的地址无效: %s", idx+1, trimmed)
		}

		validAddresses = append(validAddresses, trimmed)
	}

	return strings.Join(validAddresses, ","), nil
}

// isValidAddress 检查是否为有效的 IP 地址或域名
func isValidAddress(addr string) bool {
	// 1. 检查是否为有效的 IP 地址 (IPv4 或 IPv6)
	if net.ParseIP(addr) != nil {
		return true
	}

	// 2. 检查是否为有效的域名
	return isValidDomain(addr)
}

// isValidDomain 检查是否为有效的域名
func isValidDomain(domain string) bool {
	// 域名长度限制
	if len(domain) == 0 || len(domain) > 253 {
		return false
	}

	// 域名正则表达式 (RFC 1035)
	// 支持: example.com, sub.example.com, example.co.uk 等
	domainRegex := regexp.MustCompile(
		`^([a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?\.)*` + // 子域名
			`[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?$`, // 顶级域名
	)

	if !domainRegex.MatchString(domain) {
		return false
	}

	// 检查每个标签的长度 (不超过 63 个字符)
	labels := strings.Split(domain, ".")
	for _, label := range labels {
		if len(label) > 63 {
			return false
		}
	}

	return true
}

// ValidateAndCleanDomainSuffixes 验证并清理域名后缀列表
// 输入: 逗号分隔的域名后缀字符串
// 返回: 清理后的字符串(确保每个后缀都以.开头)和错误信息
func ValidateAndCleanDomainSuffixes(input string) (string, error) {
	if input == "" {
		return "", errors.New("输入不能为空")
	}

	// 按逗号分隔
	parts := strings.Split(input, ",")
	var validSuffixes []string

	for i, part := range parts {
		// 去除左右空格
		trimmed := strings.TrimSpace(part)

		if trimmed == "" {
			return "", errors.New("存在空的后缀项")
		}

		// 如果以点开头，保持原样；否则添加点
		if !strings.HasPrefix(trimmed, ".") {
			trimmed = "." + trimmed
		}

		// 检查是否只是一个点
		if trimmed == "." {
			return "", errors.New("域名后缀不能只是一个点")
		}

		// 检查点后面的第一个字符是否为英文字母
		if len(trimmed) > 1 {
			firstChar := rune(trimmed[1]) // 跳过开头的点
			if !((firstChar >= 'a' && firstChar <= 'z') || (firstChar >= 'A' && firstChar <= 'Z')) {
				return "", fmt.Errorf("域名后缀必须以英文字母开头: %s (位置: %d)", trimmed, i+1)
			}
		}

		validSuffixes = append(validSuffixes, trimmed)
	}

	// 用逗号重新组合
	return strings.Join(validSuffixes, ","), nil
}
