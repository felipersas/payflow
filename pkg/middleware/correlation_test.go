package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
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

	if rr.Code != http.StatusOK {
		t.Errorf("status code: got %d, want %d", rr.Code, http.StatusOK)
	}

	responseID := rr.Header().Get("X-Correlation-ID")
	if responseID != correlationID {
		t.Errorf("response header: got %s, want %s", responseID, correlationID)
	}

	body := rr.Body.String()
	if body != correlationID {
		t.Errorf("response body: got %s, want %s", body, correlationID)
	}
}

func TestCorrelationID_GenerateNew(t *testing.T) {
	r := setupCorrelationMiddleware()

	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status code: got %d, want %d", rr.Code, http.StatusOK)
	}

	responseID := rr.Header().Get("X-Correlation-ID")
	if responseID == "" {
		t.Error("expected non-empty X-Correlation-ID header")
	}

	_, err := uuid.Parse(responseID)
	if err != nil {
		t.Errorf("X-Correlation-ID is not a valid UUID: %v", err)
	}

	body := rr.Body.String()
	if body != responseID {
		t.Errorf("response body mismatch: got %s, want %s", body, responseID)
	}
}

func TestGetCorrelationID_EmptyContext(t *testing.T) {
	id := GetCorrelationID(context.Background())
	if id != "" {
		t.Errorf("GetCorrelationID with empty context: got %s, want empty string", id)
	}
}
