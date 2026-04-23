package http

import (
	"errors"
	"net/http"

	"github.com/felipersas/payflow/internal/account/application/commands"
	"github.com/felipersas/payflow/internal/account/application/queries"
	"github.com/felipersas/payflow/internal/account/application/services"
	"github.com/felipersas/payflow/pkg/httputil"
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
		httputil.WriteError(w, err)
		return false
	}
	return true
}

func (h *AccountHandler) CreateAccount(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	var req createAccountRequest
	if err := httputil.DecodeAndValidate(r, &req); err != nil {
		writeHandlerError(w, err)
		return
	}

	account, err := h.service.CreateAccount(r.Context(), commands.CreateAccountCommand{
		UserID:   userID,
		Currency: req.Currency,
	})
	if err != nil {
		httputil.WriteError(w, err)
		return
	}

	httputil.WriteJSON(w, http.StatusCreated, accountResponse{
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
		httputil.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "account id is required"})
		return
	}

	if !h.verifyOwnership(w, r, accountID) {
		return
	}

	result, err := h.service.GetBalance(r.Context(), queries.GetBalanceQuery{
		AccountID: accountID,
	})
	if err != nil {
		httputil.WriteError(w, err)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, result)
}

type creditDebitRequest struct {
	Amount    int64  `json:"amount" validate:"required,gt=0"`
	Reference string `json:"reference" validate:"required"`
}

func (h *AccountHandler) CreditAccount(w http.ResponseWriter, r *http.Request) {
	accountID := chi.URLParam(r, "id")
	if accountID == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "account id is required"})
		return
	}

	if !h.verifyOwnership(w, r, accountID) {
		return
	}

	var req creditDebitRequest
	if err := httputil.DecodeAndValidate(r, &req); err != nil {
		writeHandlerError(w, err)
		return
	}

	account, err := h.service.CreditAccount(r.Context(), commands.CreditAccountCommand{
		AccountID: accountID,
		Amount:    req.Amount,
		Reference: req.Reference,
	})
	if err != nil {
		httputil.WriteError(w, err)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, accountResponse{
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
		httputil.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "account id is required"})
		return
	}

	if !h.verifyOwnership(w, r, accountID) {
		return
	}

	var req creditDebitRequest
	if err := httputil.DecodeAndValidate(r, &req); err != nil {
		writeHandlerError(w, err)
		return
	}

	account, err := h.service.DebitAccount(r.Context(), commands.DebitAccountCommand{
		AccountID: accountID,
		Amount:    req.Amount,
		Reference: req.Reference,
	})
	if err != nil {
		httputil.WriteError(w, err)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, accountResponse{
		ID:       account.ID,
		UserID:   account.UserID,
		Balance:  account.Balance,
		Currency: account.Currency,
		IsActive: account.IsActive,
	})
}

// writeHandlerError distinguishes decode errors (400) from validation errors (422).
func writeHandlerError(w http.ResponseWriter, err error) {
	if httputil.IsDecodeError(err) {
		httputil.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	var valErr *validation.ValidationError
	if errors.As(err, &valErr) {
		httputil.WriteValidationError(w, err)
		return
	}
	httputil.WriteError(w, err)
}
