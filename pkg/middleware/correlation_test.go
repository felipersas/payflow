package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupCorrelationMiddleware() http.Handler {
	r := chi.NewRouter()
	r.Use(CorrelationID)
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		id := GetCorrelationID(r.Context())
		w.Write([]byte(id))
	})
	return r
}

func TestCorrelationID_PassThrough(t *testing.T) {
	r := setupCorrelationMiddleware()
	correlationID := uuid.Must(uuid.NewV7()).String()

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Correlation-ID", correlationID)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code, "status code mismatch")

	responseID := rr.Header().Get("X-Correlation-ID")
	assert.Equal(t, correlationID, responseID, "response header mismatch")

	body := rr.Body.String()
	assert.Equal(t, correlationID, body, "response body mismatch")
}

func TestCorrelationID_GenerateNew(t *testing.T) {
	r := setupCorrelationMiddleware()

	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code, "status code mismatch")

	responseID := rr.Header().Get("X-Correlation-ID")
	assert.NotEmpty(t, responseID, "expected non-empty X-Correlation-ID header")

	_, err := uuid.Parse(responseID)
	require.NoError(t, err, "X-Correlation-ID is not a valid UUID")

	body := rr.Body.String()
	assert.Equal(t, responseID, body, "response body mismatch")
}

func TestGetCorrelationID_EmptyContext(t *testing.T) {
	id := GetCorrelationID(context.Background())
	assert.Equal(t, "", id, "GetCorrelationID with empty context should return empty string")
}
