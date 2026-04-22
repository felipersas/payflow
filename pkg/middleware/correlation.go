package middleware

import (
	"context"
	"net/http"

	"github.com/google/uuid"
)

type contextKey string

const CorrelationIDKey contextKey = "correlation_id"

// CorrelationID injeta X-Correlation-ID em cada request.
// Se não veio no header, gera um novo UUID.
// Permite rastrear a jornada completa de uma operação entre serviços.
func CorrelationID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		correlationID := r.Header.Get("X-Correlation-ID")
		if correlationID == "" {
			correlationID = uuid.Must(uuid.NewV7()).String()
		}
		w.Header().Set("X-Correlation-ID", correlationID)
		ctx := context.WithValue(r.Context(), CorrelationIDKey, correlationID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func GetCorrelationID(ctx context.Context) string {
	if id, ok := ctx.Value(CorrelationIDKey).(string); ok {
		return id
	}
	return ""
}
