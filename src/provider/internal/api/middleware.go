package api

import (
	"context"

	"github.com/golang-jwt/jwt/v5"
	"net/http"
)

type contextKey string

const ClaimsKey contextKey = "auth_claims"

// UserIDFrom 从请求上下文提取当前用户 ID（JWT sub）。
func UserIDFrom(r *http.Request) string {
	if claims, ok := r.Context().Value(ClaimsKey).(jwt.MapClaims); ok {
		if sub, ok := claims["sub"].(string); ok {
			return sub
		}
	}
	return ""
}

// WithCORS 跨域支持（浏览器 Web 客户端，如账号中心 Flutter Web）：
// 回显 Origin，允许 Authorization / X-Admin-Token 头，OPTIONS 预检直接 204。
// 注：鉴权仍靠 JWT/Token 本身，CORS 只是浏览器同源策略的配套，不是安全边界。
func WithCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if origin := r.Header.Get("Origin"); origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Admin-Token")
			w.Header().Set("Access-Control-Max-Age", "600")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

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
