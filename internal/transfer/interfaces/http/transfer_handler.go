package http

import (
	"errors"
	"net/http"

	"github.com/felipersas/payflow/internal/transfer/application/commands"
	"github.com/felipersas/payflow/internal/transfer/application/services"
	"github.com/felipersas/payflow/pkg/httputil"
	"github.com/felipersas/payflow/pkg/pagination"
	"github.com/felipersas/payflow/pkg/validation"
	"github.com/go-chi/chi/v5"
)

type TransferHandler struct {
	service *services.TransferService
}

func NewTransferHandler(service *services.TransferService) *TransferHandler {
	return &TransferHandler{service: service}
}

func (h *TransferHandler) Routes(r chi.Router) {
	r.Post("/", h.CreateTransfer)
	r.Get("/", h.ListTransfers)
	r.Get("/{id}", h.GetTransfer)
}

type createTransferRequest struct {
	FromAccountID string `json:"from_account_id" validate:"required"`
	ToAccountID   string `json:"to_account_id" validate:"required"`
	Amount        int64  `json:"amount" validate:"required,gt=0"`
	Currency      string `json:"currency" validate:"required,len=3"`
}

func (h *TransferHandler) CreateTransfer(w http.ResponseWriter, r *http.Request) {
	var req createTransferRequest
	if err := httputil.DecodeAndValidate(r, &req); err != nil {
		writeHandlerError(w, err)
		return
	}

	result, err := h.service.CreateTransfer(r.Context(), commands.CreateTransferCommand{
		FromAccountID: req.FromAccountID,
		ToAccountID:   req.ToAccountID,
		Amount:        req.Amount,
		Currency:      req.Currency,
	})
	if err != nil {
		httputil.WriteError(w, err)
		return
	}

	httputil.WriteJSON(w, http.StatusAccepted, result)
}

func (h *TransferHandler) GetTransfer(w http.ResponseWriter, r *http.Request) {
	transferID := chi.URLParam(r, "id")
	if transferID == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "transfer id is required"})
		return
	}

	result, err := h.service.GetTransfer(r.Context(), transferID)
	if err != nil {
		httputil.WriteError(w, err)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, result)
}

func (h *TransferHandler) ListTransfers(w http.ResponseWriter, r *http.Request) {
	accountID := r.URL.Query().Get("account_id")
	if accountID == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "account_id is required"})
		return
	}

	params, err := pagination.ParseParams(r.URL.Query().Get("cursor"), r.URL.Query().Get("limit"))
	if err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	result, err := h.service.ListTransfers(r.Context(), accountID, params)
	if err != nil {
		httputil.WriteError(w, err)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, result)
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
