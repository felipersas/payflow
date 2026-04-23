package services

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/felipersas/payflow/internal/transfer/application/commands"
	"github.com/felipersas/payflow/internal/transfer/domain/entities"
	"github.com/felipersas/payflow/internal/transfer/domain/repositories"
	"github.com/felipersas/payflow/pkg/messaging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gomock "go.uber.org/mock/gomock"
)

func setupService(t *testing.T) (*TransferService, *repositories.MockTransferRepository, *messaging.MockMessagePublisher) {
	ctrl := gomock.NewController(t)
	mockRepo := repositories.NewMockTransferRepository(ctrl)
	mockPub := messaging.NewMockMessagePublisher(ctrl)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewTransferService(mockRepo, mockPub, logger), mockRepo, mockPub
}

func TestCreateTransfer_Valid(t *testing.T) {
	svc, mockRepo, mockPub := setupService(t)

	mockRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
	mockPub.EXPECT().Publish(gomock.Any(), "account.debit.cmd", gomock.Any()).Return(nil)

	result, err := svc.CreateTransfer(context.Background(), commands.CreateTransferCommand{
		FromAccountID: "acc-1",
		ToAccountID:   "acc-2",
		Amount:        10050,
		Currency:      "BRL",
	})
	require.NoError(t, err)
	assert.Equal(t, entities.TransferPending, result.Status)
	assert.NotEmpty(t, result.TransferID)
	assert.Equal(t, int64(10050), result.Amount)
}

func TestCreateTransfer_InvalidAmount(t *testing.T) {
	svc, _, _ := setupService(t)

	_, err := svc.CreateTransfer(context.Background(), commands.CreateTransferCommand{
		FromAccountID: "acc-1",
		ToAccountID:   "acc-2",
		Amount:        0,
		Currency:      "BRL",
	})
	require.Error(t, err)
}

func TestHandleDebitConfirmed(t *testing.T) {
	svc, mockRepo, mockPub := setupService(t)

	transfer, _ := entities.NewTransfer("acc-1", "acc-2", 100, "BRL")

	mockRepo.EXPECT().GetByID(gomock.Any(), gomock.Any()).Return(transfer, nil)
	mockRepo.EXPECT().UpdateStatus(gomock.Any(), gomock.Any(), string(entities.TransferProcessing)).Return(nil)
	mockPub.EXPECT().Publish(gomock.Any(), "account.credit.cmd", gomock.Any()).Return(nil)

	err := svc.HandleDebitConfirmed(context.Background(), transfer.ID)
	require.NoError(t, err)
}

func TestHandleDebitConfirmed_AlreadyProcessed(t *testing.T) {
	svc, mockRepo, _ := setupService(t)

	transfer, _ := entities.NewTransfer("acc-1", "acc-2", 100, "BRL")
	transfer.MarkCompleted()

	mockRepo.EXPECT().GetByID(gomock.Any(), gomock.Any()).Return(transfer, nil)

	err := svc.HandleDebitConfirmed(context.Background(), transfer.ID)
	require.NoError(t, err)
}

func TestHandleCreditConfirmed(t *testing.T) {
	svc, mockRepo, mockPub := setupService(t)

	transfer, _ := entities.NewTransfer("acc-1", "acc-2", 100, "BRL")

	mockRepo.EXPECT().GetByID(gomock.Any(), gomock.Any()).Return(transfer, nil)
	mockRepo.EXPECT().UpdateStatus(gomock.Any(), gomock.Any(), string(entities.TransferCompleted)).Return(nil)
	mockPub.EXPECT().Publish(gomock.Any(), "transfer.completed", gomock.Any()).Return(nil)

	err := svc.HandleCreditConfirmed(context.Background(), transfer.ID)
	require.NoError(t, err)
	assert.Equal(t, entities.TransferCompleted, transfer.Status)
}

func TestHandleDebitFailed(t *testing.T) {
	svc, mockRepo, mockPub := setupService(t)

	transfer, _ := entities.NewTransfer("acc-1", "acc-2", 100, "BRL")

	mockRepo.EXPECT().GetByID(gomock.Any(), gomock.Any()).Return(transfer, nil)
	mockRepo.EXPECT().UpdateStatus(gomock.Any(), gomock.Any(), string(entities.TransferFailed)).Return(nil)
	mockPub.EXPECT().Publish(gomock.Any(), "transfer.failed", gomock.Any()).Return(nil)

	err := svc.HandleDebitFailed(context.Background(), transfer.ID, "insufficient funds")
	require.NoError(t, err)
	assert.Equal(t, entities.TransferFailed, transfer.Status)
}

func TestHandleCreditFailed(t *testing.T) {
	svc, mockRepo, mockPub := setupService(t)

	transfer, _ := entities.NewTransfer("acc-1", "acc-2", 100, "BRL")

	mockRepo.EXPECT().GetByID(gomock.Any(), gomock.Any()).Return(transfer, nil)
	mockRepo.EXPECT().UpdateStatus(gomock.Any(), gomock.Any(), string(entities.TransferFailed)).Return(nil)
	mockPub.EXPECT().Publish(gomock.Any(), "account.compensate.cmd", gomock.Any()).Return(nil)
	mockPub.EXPECT().Publish(gomock.Any(), "transfer.failed", gomock.Any()).Return(nil)

	err := svc.HandleCreditFailed(context.Background(), transfer.ID, "account not found")
	require.NoError(t, err)
	assert.Equal(t, entities.TransferFailed, transfer.Status)
}

func TestGetTransfer(t *testing.T) {
	svc, mockRepo, _ := setupService(t)

	transfer, _ := entities.NewTransfer("acc-1", "acc-2", 100, "BRL")

	mockRepo.EXPECT().GetByID(gomock.Any(), gomock.Any()).Return(transfer, nil)

	result, err := svc.GetTransfer(context.Background(), transfer.ID)
	require.NoError(t, err)
	assert.Equal(t, transfer.ID, result.TransferID)
	assert.Equal(t, "acc-1", result.FromAccountID)
	assert.Equal(t, "acc-2", result.ToAccountID)
	assert.Equal(t, int64(100), result.Amount)
	assert.Equal(t, "BRL", result.Currency)
}

func TestGetTransfer_NotFound(t *testing.T) {
	svc, mockRepo, _ := setupService(t)

	mockRepo.EXPECT().GetByID(gomock.Any(), "nonexistent-id").Return(nil, nil)

	_, err := svc.GetTransfer(context.Background(), "nonexistent-id")
	require.Error(t, err)
}
