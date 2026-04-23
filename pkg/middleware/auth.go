package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/felipersas/payflow/pkg/auth"
)

const UserIDKey contextKey = "user_id"

// Auth valida o JWT no header Authorization e injeta user_id no context.
func Auth(jwtSecret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				writeAuthError(w, "missing authorization header")
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
				writeAuthError(w, "invalid authorization format, use: Bearer <token>")
				return
			}

			claims, err := auth.ValidateToken(jwtSecret, parts[1])
			if err != nil {
				writeAuthError(w, "invalid or expired token")
				return
			}

			ctx := context.WithValue(r.Context(), UserIDKey, claims.UserID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetUserID extrai o user_id do context (injetado pelo middleware Auth).
func GetUserID(ctx context.Context) string {
	if userID, ok := ctx.Value(UserIDKey).(string); ok {
		return userID
	}
	return ""
}

func writeAuthError(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	w.Write([]byte(`{"error":"` + msg + `"}`))
}
