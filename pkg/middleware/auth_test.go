package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/felipersas/payflow/pkg/auth"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	require.NoError(t, err, "GenerateToken failed")

	r := setupAuthMiddleware(secret)
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code, "status code mismatch")
	body := rr.Body.String()
	assert.Equal(t, userID, body, "response body mismatch")
}

func TestAuth_MissingHeader(t *testing.T) {
	secret := "test-secret"
	r := setupAuthMiddleware(secret)

	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code, "status code mismatch")
	body := rr.Body.String()
	assert.Contains(t, body, "missing authorization header", "error message not found in response")
}

func TestAuth_BadFormat(t *testing.T) {
	secret := "test-secret"
	r := setupAuthMiddleware(secret)

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "NotBearer token")
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code, "status code mismatch")
}

func TestAuth_InvalidToken(t *testing.T) {
	secret := "test-secret"
	r := setupAuthMiddleware(secret)

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code, "status code mismatch")
	body := rr.Body.String()
	assert.Contains(t, body, "invalid or expired token", "error message not found in response")
}

func TestGetUserID_EmptyContext(t *testing.T) {
	userID := GetUserID(context.Background())
	assert.Equal(t, "", userID, "GetUserID with empty context should return empty string")
}
