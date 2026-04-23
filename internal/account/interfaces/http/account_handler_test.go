package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/felipersas/payflow/internal/account/application/commands"
	"github.com/felipersas/payflow/internal/account/application/services"
	"github.com/felipersas/payflow/internal/account/domain/entities"
	"github.com/felipersas/payflow/internal/account/domain/repositories"
	"github.com/felipersas/payflow/pkg/middleware"
	"github.com/go-chi/chi/v5"
)

type mockRepo struct {
	accounts     map[string]*entities.Account
	transactions map[string]*repositories.Transaction
}

func newMockRepo() *mockRepo {
	return &mockRepo{
		accounts:     make(map[string]*entities.Account),
		transactions: make(map[string]*repositories.Transaction),
	}
}

func (m *mockRepo) Create(_ context.Context, account *entities.Account) error {
	m.accounts[account.ID] = account
	return nil
}

func (m *mockRepo) GetByID(_ context.Context, id string) (*entities.Account, error) {
	a, ok := m.accounts[id]
	if !ok {
		return nil, fmt.Errorf("account %s not found", id)
	}
	return a, nil
}

func (m *mockRepo) GetByUserID(_ context.Context, userID string) (*entities.Account, error) {
	for _, a := range m.accounts {
		if a.UserID == userID {
			return a, nil
		}
	}
	return nil, nil
}

func (m *mockRepo) Update(_ context.Context, account *entities.Account) error {
	m.accounts[account.ID] = account
	return nil
}

func (m *mockRepo) GetByReference(_ context.Context, reference string) (*repositories.Transaction, error) {
	tx, ok := m.transactions[reference]
	if !ok {
		return nil, nil
	}
	return tx, nil
}

func (m *mockRepo) SaveTransaction(_ context.Context, tx *repositories.Transaction) error {
	m.transactions[tx.Reference] = tx
	return nil
}

func authMiddleware(userID string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), middleware.UserIDKey, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func setupAccountHandler() (*AccountHandler, *mockRepo) {
	repo := newMockRepo()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := services.NewAccountService(repo, nil, logger)
	return NewAccountHandler(svc), repo
}

func setupAccountRouter(h *AccountHandler, userID string) *chi.Mux {
	r := chi.NewRouter()
	r.Use(authMiddleware(userID))
	r.Route("/accounts", h.Routes)
	return r
}

func TestCreateAccount_Success(t *testing.T) {
	h, _ := setupAccountHandler()
	r := setupAccountRouter(h, "user-1")

	body := map[string]string{
		"currency": "BRL",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/accounts", bytes.NewReader(jsonBody))
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

	if resp["id"] == nil {
		t.Error("response missing id")
	}
	if resp["user_id"] == nil {
		t.Error("response missing user_id")
	}
	if resp["user_id"] != "user-1" {
		t.Errorf("user_id = %v, want user-1", resp["user_id"])
	}
	if resp["balance"] == nil {
		t.Error("response missing balance")
	}
	if resp["balance"] != float64(0) {
		t.Errorf("balance = %v, want 0", resp["balance"])
	}
}

func TestGetBalance_Success(t *testing.T) {
	h, repo := setupAccountHandler()

	// Create an account directly
	account, _ := entities.NewAccount("user-1", "BRL")
	repo.Create(context.Background(), account)

	r := setupAccountRouter(h, "user-1")

	req := httptest.NewRequest("GET", "/accounts/"+account.ID+"/balance", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp["balance"] == nil {
		t.Error("response missing balance")
	}
	if resp["balance"] != float64(0) {
		t.Errorf("balance = %v, want 0", resp["balance"])
	}
}

func TestGetBalance_WrongOwner(t *testing.T) {
	h, repo := setupAccountHandler()

	// Create an account for user-1
	account, _ := entities.NewAccount("user-1", "BRL")
	repo.Create(context.Background(), account)

	// Request as user-2
	r := setupAccountRouter(h, "user-2")

	req := httptest.NewRequest("GET", "/accounts/"+account.ID+"/balance", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestGetBalance_NotFound(t *testing.T) {
	h, _ := setupAccountHandler()
	r := setupAccountRouter(h, "user-1")

	req := httptest.NewRequest("GET", "/accounts/nonexistent/balance", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	// The handler checks ownership first, which fails for non-existent accounts
	// returning 403 (Forbidden) since the account doesn't belong to the user
	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestCreditAccount_Success(t *testing.T) {
	h, repo := setupAccountHandler()

	// Create an account directly
	account, _ := entities.NewAccount("user-1", "BRL")
	repo.Create(context.Background(), account)

	r := setupAccountRouter(h, "user-1")

	body := map[string]any{
		"amount":    5000,
		"reference": "ref-1",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/accounts/"+account.ID+"/credit", bytes.NewReader(jsonBody))
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

	if resp["balance"] == nil {
		t.Error("response missing balance")
	}
	if resp["balance"] != float64(5000) {
		t.Errorf("balance = %v, want 5000", resp["balance"])
	}
}

func TestDebitAccount_Success(t *testing.T) {
	h, repo := setupAccountHandler()

	// Create and credit an account
	account, _ := entities.NewAccount("user-1", "BRL")
	repo.Create(context.Background(), account)

	svc := services.NewAccountService(repo, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	svc.CreditAccount(context.Background(), commands.CreditAccountCommand{
		AccountID: account.ID,
		Amount:    10000,
		Reference: "init-credit",
	})

	r := setupAccountRouter(h, "user-1")

	body := map[string]any{
		"amount":    3000,
		"reference": "debit-ref",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/accounts/"+account.ID+"/debit", bytes.NewReader(jsonBody))
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

	if resp["balance"] == nil {
		t.Error("response missing balance")
	}
	if resp["balance"] != float64(7000) {
		t.Errorf("balance = %v, want 7000", resp["balance"])
	}
}

func TestDebitAccount_InsufficientBalance(t *testing.T) {
	h, repo := setupAccountHandler()

	// Create an account with zero balance
	account, _ := entities.NewAccount("user-1", "BRL")
	repo.Create(context.Background(), account)

	r := setupAccountRouter(h, "user-1")

	body := map[string]any{
		"amount":    100,
		"reference": "debit-ref",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/accounts/"+account.ID+"/debit", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}

	var resp map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp["error"] == "" {
		t.Error("response missing error message")
	}
}
