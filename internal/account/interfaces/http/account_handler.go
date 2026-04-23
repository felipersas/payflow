package http

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/felipersas/payflow/internal/account/application/commands"
	"github.com/felipersas/payflow/internal/account/application/queries"
	"github.com/felipersas/payflow/internal/account/application/services"
	"github.com/felipersas/payflow/pkg/middleware"
	"github.com/felipersas/payflow/pkg/validation"
	"github.com/go-chi/chi/v5"
)

// AccountHandler expõe os casos de uso via REST.
// Recebe HTTP, converte para commands/queries, delega ao service.
type AccountHandler struct {
	service *services.AccountService
}

func NewAccountHandler(service *services.AccountService) *AccountHandler {
	return &AccountHandler{service: service}
}

// Routes registra as rotas no chi router.
func (h *AccountHandler) Routes(r chi.Router) {
	r.Post("/", h.CreateAccount)
	r.Get("/{id}/balance", h.GetBalance)
	r.Post("/{id}/credit", h.CreditAccount)
	r.Post("/{id}/debit", h.DebitAccount)
}

type createAccountRequest struct {
	Currency string `json:"currency" validate:"required,len=3"`
}

type accountResponse struct {
	ID       string `json:"id"`
	UserID   string `json:"user_id"`
	Balance  int64  `json:"balance"`
	Currency string `json:"currency"`
	IsActive bool   `json:"is_active"`
}

// verifyOwnership garante que a conta pertence ao usuário autenticado.
func (h *AccountHandler) verifyOwnership(w http.ResponseWriter, r *http.Request, accountID string) bool {
	userID := middleware.GetUserID(r.Context())
	if err := h.service.VerifyAccountOwner(r.Context(), accountID, userID); err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": err.Error()})
		return false
	}
	return true
}

func (h *AccountHandler) CreateAccount(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	var req createAccountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if err := validation.Validate(&req); err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"error": err.Error(), "fields": err.(*validation.ValidationError).Fields})
		return
	}

	account, err := h.service.CreateAccount(r.Context(), commands.CreateAccountCommand{
		UserID:   userID,
		Currency: req.Currency,
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusCreated, accountResponse{
		ID:       account.ID,
		UserID:   account.UserID,
		Balance:  account.Balance,
		Currency: account.Currency,
		IsActive: account.IsActive,
	})
}

func (h *AccountHandler) GetBalance(w http.ResponseWriter, r *http.Request) {
	accountID := chi.URLParam(r, "id")
	if accountID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "account id is required"})
		return
	}

	if !h.verifyOwnership(w, r, accountID) {
		return
	}

	result, err := h.service.GetBalance(r.Context(), queries.GetBalanceQuery{
		AccountID: accountID,
	})
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

type creditDebitRequest struct {
	Amount    int64  `json:"amount" validate:"required,gt=0"`
	Reference string `json:"reference" validate:"required"`
}

func (h *AccountHandler) CreditAccount(w http.ResponseWriter, r *http.Request) {
	accountID := chi.URLParam(r, "id")
	if accountID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "account id is required"})
		return
	}

	if !h.verifyOwnership(w, r, accountID) {
		return
	}

	var req creditDebitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if err := validation.Validate(&req); err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"error": err.Error(), "fields": err.(*validation.ValidationError).Fields})
		return
	}

	account, err := h.service.CreditAccount(r.Context(), commands.CreditAccountCommand{
		AccountID: accountID,
		Amount:    req.Amount,
		Reference: req.Reference,
	})
	if err != nil {
		status := http.StatusBadRequest
		if fmt.Sprint(err) != "" {
			status = http.StatusUnprocessableEntity
		}
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, accountResponse{
		ID:       account.ID,
		UserID:   account.UserID,
		Balance:  account.Balance,
		Currency: account.Currency,
		IsActive: account.IsActive,
	})
}

func (h *AccountHandler) DebitAccount(w http.ResponseWriter, r *http.Request) {
	accountID := chi.URLParam(r, "id")
	if accountID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "account id is required"})
		return
	}

	if !h.verifyOwnership(w, r, accountID) {
		return
	}

	var req creditDebitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if err := validation.Validate(&req); err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"error": err.Error(), "fields": err.(*validation.ValidationError).Fields})
		return
	}

	account, err := h.service.DebitAccount(r.Context(), commands.DebitAccountCommand{
		AccountID: accountID,
		Amount:    req.Amount,
		Reference: req.Reference,
	})
	if err != nil {
		status := http.StatusBadRequest
		if fmt.Sprint(err) != "" {
			status = http.StatusUnprocessableEntity
		}
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, accountResponse{
		ID:       account.ID,
		UserID:   account.UserID,
		Balance:  account.Balance,
		Currency: account.Currency,
		IsActive: account.IsActive,
	})
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
