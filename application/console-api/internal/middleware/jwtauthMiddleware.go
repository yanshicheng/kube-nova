package middleware

import (
	"context"
	"net/http"
	"strconv"
)

type JWTAuthMiddleware struct {
}

func NewJWTAuthMiddleware() *JWTAuthMiddleware {
	return &JWTAuthMiddleware{}
}

func (m *JWTAuthMiddleware) Handle(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		if platformId := parsePlatformID(r.Header.Get("X-Platform-Id")); platformId > 0 {
			ctx = context.WithValue(ctx, "platformId", platformId)
		}
		next(w, r.WithContext(ctx))
	}
}

func parsePlatformID(raw string) uint64 {
	if raw == "" {
		return 0
	}
	id, _ := strconv.ParseUint(raw, 10, 64)
	return id
}
