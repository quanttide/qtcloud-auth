package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/quanttide/qtcloud-auth/auth"
)

type contextKey string

const ClaimsKey contextKey = "auth_claims"

func AuthMiddleware(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
				WriteOAuthError(w, "invalid_token", "missing or invalid authorization header", http.StatusUnauthorized)
				return
			}

			token := strings.TrimPrefix(authHeader, "Bearer ")
			claims, err := auth.Verify(token, secret)
			if err != nil {
				WriteOAuthError(w, "invalid_token", "invalid or expired token", http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), ClaimsKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
