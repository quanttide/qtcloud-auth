package api

import (
	"context"
	"net/http"
)

type contextKey string

const ClaimsKey contextKey = "auth_claims"

// AuthMiddleware 解析 Bearer JWT 并注入 claims 到请求上下文.
func (h *AuthHandler) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenStr := extractBearerToken(r)
		if tokenStr == "" {
			WriteOAuthError(w, "invalid_token", "missing or invalid authorization header", http.StatusUnauthorized)
			return
		}

		claims, err := h.parseToken(tokenStr)
		if err != nil {
			WriteOAuthError(w, "invalid_token", "invalid or expired token", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), ClaimsKey, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
