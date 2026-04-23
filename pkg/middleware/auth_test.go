package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/felipersas/payflow/pkg/auth"
	"github.com/go-chi/chi/v5"
)

func setupAuthMiddleware(secret string) http.Handler {
	r := chi.NewRouter()
	r.Use(Auth(secret))
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		userID := GetUserID(r.Context())
		w.Write([]byte(userID))
	})
	return r
}

func TestAuth_ValidToken(t *testing.T) {
	secret := "test-secret"
	userID := "user-123"
	token, err := auth.GenerateToken(secret, userID)
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	r := setupAuthMiddleware(secret)
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status code: got %d, want %d", rr.Code, http.StatusOK)
	}
	body := rr.Body.String()
	if body != userID {
		t.Errorf("response body: got %s, want %s", body, userID)
	}
}

func TestAuth_MissingHeader(t *testing.T) {
	secret := "test-secret"
	r := setupAuthMiddleware(secret)

	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status code: got %d, want %d", rr.Code, http.StatusUnauthorized)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "missing authorization header") {
		t.Errorf("error message not found in response: %s", body)
	}
}

func TestAuth_BadFormat(t *testing.T) {
	secret := "test-secret"
	r := setupAuthMiddleware(secret)

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "NotBearer token")
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status code: got %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestAuth_InvalidToken(t *testing.T) {
	secret := "test-secret"
	r := setupAuthMiddleware(secret)

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status code: got %d, want %d", rr.Code, http.StatusUnauthorized)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "invalid or expired token") {
		t.Errorf("error message not found in response: %s", body)
	}
}

func TestGetUserID_EmptyContext(t *testing.T) {
	userID := GetUserID(context.Background())
	if userID != "" {
		t.Errorf("GetUserID with empty context: got %s, want empty string", userID)
	}
}
