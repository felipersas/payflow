package http

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/felipersas/payflow/internal/transfer/application/services"
	"github.com/felipersas/payflow/internal/transfer/domain/entities"
	"github.com/felipersas/payflow/internal/transfer/domain/repositories"
	"github.com/felipersas/payflow/pkg/messaging"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func setupTransferHandler(ctrl *gomock.Controller, mockRepo *repositories.MockTransferRepository, mockPub *messaging.MockMessagePublisher) *TransferHandler {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := services.NewTransferService(mockRepo, mockPub, logger)
	return NewTransferHandler(svc)
}

func TestCreateTransfer_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repositories.NewMockTransferRepository(ctrl)
	mockPub := messaging.NewMockMessagePublisher(ctrl)

	fromAccountID := uuid.New().String()
	toAccountID := uuid.New().String()

	mockRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
	mockPub.EXPECT().Publish(gomock.Any(), "account.debit.cmd", gomock.Any()).Return(nil)

	h := setupTransferHandler(ctrl, mockRepo, mockPub)
	r := chi.NewRouter()
	r.Route("/", h.Routes)

	body := map[string]any{
		"from_account_id": fromAccountID,
		"to_account_id":   toAccountID,
		"amount":          10050,
		"currency":        "BRL",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusAccepted, rec.Code)

	var resp map[string]any
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.NotNil(t, resp["transfer_id"])
	assert.NotNil(t, resp["status"])
	assert.Equal(t, "pending", resp["status"])
}

func TestCreateTransfer_InvalidBody(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repositories.NewMockTransferRepository(ctrl)
	mockPub := messaging.NewMockMessagePublisher(ctrl)

	h := setupTransferHandler(ctrl, mockRepo, mockPub)
	r := chi.NewRouter()
	r.Route("/", h.Routes)

	req := httptest.NewRequest("POST", "/", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestCreateTransfer_InvalidInput(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repositories.NewMockTransferRepository(ctrl)
	mockPub := messaging.NewMockMessagePublisher(ctrl)

	h := setupTransferHandler(ctrl, mockRepo, mockPub)
	r := chi.NewRouter()
	r.Route("/", h.Routes)

	body := map[string]any{
		"from_account_id": "",
		"to_account_id":   uuid.New().String(),
		"amount":          10050,
		"currency":        "BRL",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
}

func TestGetTransfer_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repositories.NewMockTransferRepository(ctrl)
	mockPub := messaging.NewMockMessagePublisher(ctrl)

	transfer, _ := entities.NewTransfer(
		uuid.New().String(),
		uuid.New().String(),
		10050,
		"BRL",
	)

	mockRepo.EXPECT().GetByID(gomock.Any(), transfer.ID).Return(transfer, nil)

	h := setupTransferHandler(ctrl, mockRepo, mockPub)
	r := chi.NewRouter()
	r.Route("/", h.Routes)

	req := httptest.NewRequest("GET", "/"+transfer.ID, nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]any
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.NotNil(t, resp["transfer_id"])
	assert.Equal(t, transfer.ID, resp["transfer_id"])
}

func TestGetTransfer_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repositories.NewMockTransferRepository(ctrl)
	mockPub := messaging.NewMockMessagePublisher(ctrl)

	mockRepo.EXPECT().GetByID(gomock.Any(), "nonexistent-id").Return(nil, nil)

	h := setupTransferHandler(ctrl, mockRepo, mockPub)
	r := chi.NewRouter()
	r.Route("/", h.Routes)

	req := httptest.NewRequest("GET", "/nonexistent-id", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code, "Body: %s", rec.Body.String())
}
