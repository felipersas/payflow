package http

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/felipersas/payflow/internal/user/application/services"
	"github.com/felipersas/payflow/internal/user/domain/entities"
	"github.com/go-chi/chi/v5"
)

type mockUserRepo struct {
	usersByEmail map[string]*entities.User
	usersByID    map[string]*entities.User
	mu           sync.Mutex
}

func newMockUserRepo() *mockUserRepo {
	return &mockUserRepo{
		usersByEmail: make(map[string]*entities.User),
		usersByID:    make(map[string]*entities.User),
	}
}

func (m *mockUserRepo) Create(_ context.Context, user *entities.User) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.usersByEmail[user.Email] = user
	m.usersByID[user.ID] = user
	return nil
}

func (m *mockUserRepo) GetByEmail(_ context.Context, email string) (*entities.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.usersByEmail[email]
	if !ok {
		return nil, nil
	}
	return u, nil
}

func (m *mockUserRepo) GetByID(_ context.Context, id string) (*entities.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.usersByID[id]
	if !ok {
		return nil, nil
	}
	return u, nil
}

func setupAuthHandler() *AuthHandler {
	repo := newMockUserRepo()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := services.NewAuthService(repo, "test-secret-key-at-least-32b", logger)
	return NewAuthHandler(svc)
}

func setupAuthRouter(h *AuthHandler) *chi.Mux {
	r := chi.NewRouter()
	r.Route("/auth", h.Routes)
	return r
}

func TestRegister_Success(t *testing.T) {
	h := setupAuthHandler()
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

	if rec.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusCreated)
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp["token"] == nil {
		t.Error("response missing token")
	}
	if resp["user"] == nil {
		t.Error("response missing user")
	}
}

func TestRegister_InvalidBody(t *testing.T) {
	h := setupAuthHandler()
	r := setupAuthRouter(h)

	req := httptest.NewRequest("POST", "/auth/register", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestRegister_DuplicateEmail(t *testing.T) {
	h := setupAuthHandler()
	r := setupAuthRouter(h)

	body := map[string]string{
		"email":    "test@test.com",
		"password": "password123",
	}
	jsonBody, _ := json.Marshal(body)

	// First registration
	req1 := httptest.NewRequest("POST", "/auth/register", bytes.NewReader(jsonBody))
	req1.Header.Set("Content-Type", "application/json")
	rec1 := httptest.NewRecorder()
	r.ServeHTTP(rec1, req1)

	if rec1.Code != http.StatusCreated {
		t.Fatalf("first registration failed with status %d", rec1.Code)
	}

	// Second registration with same email
	req2 := httptest.NewRequest("POST", "/auth/register", bytes.NewReader(jsonBody))
	req2.Header.Set("Content-Type", "application/json")
	rec2 := httptest.NewRecorder()
	r.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusBadRequest {
		t.Errorf("second registration status = %d, want %d", rec2.Code, http.StatusBadRequest)
	}

	var resp map[string]string
	if err := json.Unmarshal(rec2.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp["error"] == "" {
		t.Error("response missing error message")
	}
}

func TestLogin_Success(t *testing.T) {
	h := setupAuthHandler()
	r := setupAuthRouter(h)

	// Register first
	registerBody := map[string]string{
		"email":    "test@test.com",
		"password": "password123",
	}
	jsonBody, _ := json.Marshal(registerBody)
	reqReg := httptest.NewRequest("POST", "/auth/register", bytes.NewReader(jsonBody))
	reqReg.Header.Set("Content-Type", "application/json")
	recReg := httptest.NewRecorder()
	r.ServeHTTP(recReg, reqReg)

	if recReg.Code != http.StatusCreated {
		t.Fatalf("registration failed with status %d", recReg.Code)
	}

	// Login
	loginBody := map[string]string{
		"email":    "test@test.com",
		"password": "password123",
	}
	jsonLoginBody, _ := json.Marshal(loginBody)
	req := httptest.NewRequest("POST", "/auth/login", bytes.NewReader(jsonLoginBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp["token"] == nil {
		t.Error("response missing token")
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	h := setupAuthHandler()
	r := setupAuthRouter(h)

	// Register first
	registerBody := map[string]string{
		"email":    "test@test.com",
		"password": "password123",
	}
	jsonBody, _ := json.Marshal(registerBody)
	reqReg := httptest.NewRequest("POST", "/auth/register", bytes.NewReader(jsonBody))
	reqReg.Header.Set("Content-Type", "application/json")
	recReg := httptest.NewRecorder()
	r.ServeHTTP(recReg, reqReg)

	// Login with wrong password
	loginBody := map[string]string{
		"email":    "test@test.com",
		"password": "wrongpassword",
	}
	jsonLoginBody, _ := json.Marshal(loginBody)
	req := httptest.NewRequest("POST", "/auth/login", bytes.NewReader(jsonLoginBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestLogin_InvalidBody(t *testing.T) {
	h := setupAuthHandler()
	r := setupAuthRouter(h)

	req := httptest.NewRequest("POST", "/auth/login", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}
