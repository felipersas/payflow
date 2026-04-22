package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/felipersas/payflow/pkg/telemetry"
)

// Metrics records HTTP request duration and count.
func Metrics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		path := r.URL.Path

		ww := &responseWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(ww, r)

		duration := time.Since(start).Seconds()
		telemetry.HTTPRequestDuration.WithLabelValues(r.Method, path).Observe(duration)
		telemetry.HTTPRequestsTotal.WithLabelValues(r.Method, path, strconv.Itoa(ww.status)).Inc()
	})
}
