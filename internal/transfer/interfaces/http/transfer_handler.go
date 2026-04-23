package http

import (
	"encoding/json"
	"net/http"

	"github.com/felipersas/payflow/internal/transfer/application/commands"
	"github.com/felipersas/payflow/internal/transfer/application/services"
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
	r.Get("/{id}", h.GetTransfer)
}

type createTransferRequest struct {
	FromAccountID string  `json:"from_account_id" validate:"required"`
	ToAccountID   string  `json:"to_account_id" validate:"required"`
	Amount        float64 `json:"amount" validate:"required,gt=0"`
	Currency      string  `json:"currency" validate:"required,len=3"`
}

func (h *TransferHandler) CreateTransfer(w http.ResponseWriter, r *http.Request) {
	var req createTransferRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if err := validation.Validate(&req); err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"error": err.Error(), "fields": err.(*validation.ValidationError).Fields})
		return
	}

	result, err := h.service.CreateTransfer(r.Context(), commands.CreateTransferCommand{
		FromAccountID: req.FromAccountID,
		ToAccountID:   req.ToAccountID,
		Amount:        req.Amount,
		Currency:      req.Currency,
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusAccepted, result)
}

func (h *TransferHandler) GetTransfer(w http.ResponseWriter, r *http.Request) {
	transferID := chi.URLParam(r, "id")
	if transferID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "transfer id is required"})
		return
	}

	result, err := h.service.GetTransfer(r.Context(), transferID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
