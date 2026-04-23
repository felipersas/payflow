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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func authMiddleware(userID string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), middleware.UserIDKey, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func setupAccountHandler(ctrl *gomock.Controller, mockRepo *repositories.MockAccountRepository) *AccountHandler {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := services.NewAccountService(mockRepo, nil, logger)
	return NewAccountHandler(svc)
}

func setupAccountRouter(h *AccountHandler, userID string) *chi.Mux {
	r := chi.NewRouter()
	r.Use(authMiddleware(userID))
	r.Route("/accounts", h.Routes)
	return r
}

func TestCreateAccount_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repositories.NewMockAccountRepository(ctrl)
	mockRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)

	h := setupAccountHandler(ctrl, mockRepo)
	r := setupAccountRouter(h, "user-1")

	body := map[string]string{
		"currency": "BRL",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/accounts", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)

	var resp map[string]any
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.NotNil(t, resp["id"])
	assert.NotNil(t, resp["user_id"])
	assert.Equal(t, "user-1", resp["user_id"])
	assert.NotNil(t, resp["balance"])
	assert.Equal(t, float64(0), resp["balance"])
}

func TestGetBalance_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repositories.NewMockAccountRepository(ctrl)
	account, _ := entities.NewAccount("user-1", "BRL")

	mockRepo.EXPECT().GetByID(gomock.Any(), account.ID).Return(account, nil).Times(2)

	h := setupAccountHandler(ctrl, mockRepo)
	r := setupAccountRouter(h, "user-1")

	req := httptest.NewRequest("GET", "/accounts/"+account.ID+"/balance", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]any
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.NotNil(t, resp["balance"])
	assert.Equal(t, float64(0), resp["balance"])
}

func TestGetBalance_WrongOwner(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repositories.NewMockAccountRepository(ctrl)
	account, _ := entities.NewAccount("user-1", "BRL")

	mockRepo.EXPECT().GetByID(gomock.Any(), account.ID).Return(account, nil)

	h := setupAccountHandler(ctrl, mockRepo)
	r := setupAccountRouter(h, "user-2")

	req := httptest.NewRequest("GET", "/accounts/"+account.ID+"/balance", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestGetBalance_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repositories.NewMockAccountRepository(ctrl)

	mockRepo.EXPECT().GetByID(gomock.Any(), "nonexistent").Return(nil, fmt.Errorf("account not found"))

	h := setupAccountHandler(ctrl, mockRepo)
	r := setupAccountRouter(h, "user-1")

	req := httptest.NewRequest("GET", "/accounts/nonexistent/balance", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	// Returns 403 because the handler checks ownership first, which fails for non-existent accounts
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestCreditAccount_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repositories.NewMockAccountRepository(ctrl)
	account, _ := entities.NewAccount("user-1", "BRL")

	mockRepo.EXPECT().GetByReference(gomock.Any(), "ref-1").Return(nil, nil)
	mockRepo.EXPECT().GetByID(gomock.Any(), account.ID).Return(account, nil).AnyTimes()
	mockRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
	mockRepo.EXPECT().SaveTransaction(gomock.Any(), gomock.Any()).Return(nil)

	h := setupAccountHandler(ctrl, mockRepo)
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

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]any
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.NotNil(t, resp["balance"])
	assert.Equal(t, float64(5000), resp["balance"])
}

func TestDebitAccount_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repositories.NewMockAccountRepository(ctrl)
	account, _ := entities.NewAccount("user-1", "BRL")

	// Credit the account first
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := services.NewAccountService(mockRepo, nil, logger)

	mockRepo.EXPECT().GetByReference(gomock.Any(), "init-credit").Return(nil, nil)
	mockRepo.EXPECT().GetByID(gomock.Any(), account.ID).Return(account, nil).AnyTimes()
	mockRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	mockRepo.EXPECT().SaveTransaction(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	svc.CreditAccount(context.Background(), commands.CreditAccountCommand{
		AccountID: account.ID,
		Amount:    10000,
		Reference: "init-credit",
	})

	// Reset expectations for debit test
	ctrl.Finish()
	ctrl = gomock.NewController(t)
	mockRepo = repositories.NewMockAccountRepository(ctrl)

	mockRepo.EXPECT().GetByReference(gomock.Any(), "debit-ref").Return(nil, nil)
	mockRepo.EXPECT().GetByID(gomock.Any(), account.ID).Return(account, nil).AnyTimes()
	mockRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
	mockRepo.EXPECT().SaveTransaction(gomock.Any(), gomock.Any()).Return(nil)

	h := setupAccountHandler(ctrl, mockRepo)
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

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]any
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.NotNil(t, resp["balance"])
	assert.Equal(t, float64(7000), resp["balance"])
}

func TestDebitAccount_InsufficientBalance(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repositories.NewMockAccountRepository(ctrl)
	account, _ := entities.NewAccount("user-1", "BRL")

	mockRepo.EXPECT().GetByReference(gomock.Any(), "debit-ref").Return(nil, nil)
	mockRepo.EXPECT().GetByID(gomock.Any(), account.ID).Return(account, nil).AnyTimes()
	mockRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	mockRepo.EXPECT().SaveTransaction(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	h := setupAccountHandler(ctrl, mockRepo)
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

	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)

	var resp map[string]string
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.NotEmpty(t, resp["error"])
}
