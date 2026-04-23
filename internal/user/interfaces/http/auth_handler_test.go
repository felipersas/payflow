package http

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/felipersas/payflow/internal/user/application/services"
	"github.com/felipersas/payflow/internal/user/domain/entities"
	"github.com/felipersas/payflow/internal/user/domain/repositories"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func setupAuthHandler(ctrl *gomock.Controller, mockRepo *repositories.MockUserRepository) *AuthHandler {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := services.NewAuthService(mockRepo, "test-secret-key-at-least-32b", logger)
	return NewAuthHandler(svc)
}

func setupAuthRouter(h *AuthHandler) *chi.Mux {
	r := chi.NewRouter()
	r.Route("/auth", h.Routes)
	return r
}

// bcrypt hash of "password123"
var hashedPassword = "$2a$10$O83PGgbOzzRbN8WyUsiiZ.nK8G/USIUIgKAYmdKAqaDUQ6nBWyXlC"


func TestRegister_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repositories.NewMockUserRepository(ctrl)
	mockRepo.EXPECT().GetByEmail(gomock.Any(), "test@test.com").Return(nil, fmt.Errorf("not found"))
	mockRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)

	h := setupAuthHandler(ctrl, mockRepo)
	r := setupAuthRouter(h)

	body := map[string]string{
		"email":    "test@test.com",
		"password": "password123",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/auth/register", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)

	var resp map[string]any
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.NotNil(t, resp["token"])
	assert.NotNil(t, resp["user"])
}

func TestRegister_InvalidBody(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repositories.NewMockUserRepository(ctrl)
	h := setupAuthHandler(ctrl, mockRepo)
	r := setupAuthRouter(h)

	req := httptest.NewRequest("POST", "/auth/register", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestRegister_DuplicateEmail(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repositories.NewMockUserRepository(ctrl)
	existingUser, err := entities.NewUser("test@test.com", "password123")
	require.NoError(t, err)

	// Second registration finds existing user
	mockRepo.EXPECT().GetByEmail(gomock.Any(), "test@test.com").Return(existingUser, nil)

	h := setupAuthHandler(ctrl, mockRepo)
	r := setupAuthRouter(h)

	body := map[string]string{
		"email":    "test@test.com",
		"password": "password123",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/auth/register", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusConflict, rec.Code)

	var resp map[string]string
	err = json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.NotEmpty(t, resp["error"])
}

func TestLogin_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repositories.NewMockUserRepository(ctrl)
	existingUser, _ := entities.NewUser("test@test.com", hashedPassword)

	mockRepo.EXPECT().GetByEmail(gomock.Any(), "test@test.com").Return(existingUser, nil)

	h := setupAuthHandler(ctrl, mockRepo)
	r := setupAuthRouter(h)

	loginBody := map[string]string{
		"email":    "test@test.com",
		"password": "password123",
	}
	jsonLoginBody, _ := json.Marshal(loginBody)
	req := httptest.NewRequest("POST", "/auth/login", bytes.NewReader(jsonLoginBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]any
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.NotNil(t, resp["token"])
	assert.NotNil(t, resp["user"])
}

func TestLogin_WrongPassword(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repositories.NewMockUserRepository(ctrl)
	existingUser, _ := entities.NewUser("test@test.com", hashedPassword)

	mockRepo.EXPECT().GetByEmail(gomock.Any(), "test@test.com").Return(existingUser, nil)

	h := setupAuthHandler(ctrl, mockRepo)
	r := setupAuthRouter(h)

	loginBody := map[string]string{
		"email":    "test@test.com",
		"password": "wrongpassword",
	}
	jsonLoginBody, _ := json.Marshal(loginBody)
	req := httptest.NewRequest("POST", "/auth/login", bytes.NewReader(jsonLoginBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestLogin_InvalidBody(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repositories.NewMockUserRepository(ctrl)
	h := setupAuthHandler(ctrl, mockRepo)
	r := setupAuthRouter(h)

	req := httptest.NewRequest("POST", "/auth/login", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestRegister_ValidationErrors(t *testing.T) {
	tests := []struct {
		name     string
		body     map[string]string
		wantCode int
	}{
		{"empty email", map[string]string{"email": "", "password": "password123"}, http.StatusUnprocessableEntity},
		{"invalid email", map[string]string{"email": "not-an-email", "password": "password123"}, http.StatusUnprocessableEntity},
		{"empty password", map[string]string{"email": "test@test.com", "password": ""}, http.StatusUnprocessableEntity},
		{"short password", map[string]string{"email": "test@test.com", "password": "12345"}, http.StatusUnprocessableEntity},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockRepo := repositories.NewMockUserRepository(ctrl)
			h := setupAuthHandler(ctrl, mockRepo)
			r := setupAuthRouter(h)

			jsonBody, _ := json.Marshal(tt.body)
			req := httptest.NewRequest("POST", "/auth/register", bytes.NewReader(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			r.ServeHTTP(rec, req)

			assert.Equal(t, tt.wantCode, rec.Code)

			var resp map[string]any
			err := json.Unmarshal(rec.Body.Bytes(), &resp)
			require.NoError(t, err)
			assert.Contains(t, resp, "fields")
		})
	}
}

func TestLogin_ValidationErrors(t *testing.T) {
	tests := []struct {
		name     string
		body     map[string]string
		wantCode int
	}{
		{"empty email", map[string]string{"email": "", "password": "password123"}, http.StatusUnprocessableEntity},
		{"empty password", map[string]string{"email": "test@test.com", "password": ""}, http.StatusUnprocessableEntity},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockRepo := repositories.NewMockUserRepository(ctrl)
			h := setupAuthHandler(ctrl, mockRepo)
			r := setupAuthRouter(h)

			jsonBody, _ := json.Marshal(tt.body)
			req := httptest.NewRequest("POST", "/auth/login", bytes.NewReader(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			r.ServeHTTP(rec, req)

			assert.Equal(t, tt.wantCode, rec.Code)

			var resp map[string]any
			err := json.Unmarshal(rec.Body.Bytes(), &resp)
			require.NoError(t, err)
			assert.Contains(t, resp, "fields")
		})
	}
}
