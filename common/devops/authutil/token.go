package authutil

import (
	"strings"
)

// NormalizeBearerToken 兼容直接 token 和 Bearer token。
func NormalizeBearerToken(_ string, token string) string {
	token = strings.TrimSpace(token)
	if token == "" {
		return ""
	}
	token = stripBearerToken(token)
	return token
}

func stripBearerToken(token string) string {
	token = strings.Trim(strings.TrimSpace(token), `"'`)
	if strings.HasPrefix(strings.ToLower(token), "authorization:") {
		token = strings.TrimSpace(token[len("authorization:"):])
	}
	if strings.HasPrefix(strings.ToLower(token), "bearer ") {
		token = strings.TrimSpace(token[7:])
	}
	return strings.Trim(strings.TrimSpace(token), `"'`)
}
