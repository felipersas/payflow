package services

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"testing"

	"github.com/felipersas/payflow/internal/transfer/application/commands"
	"github.com/felipersas/payflow/internal/transfer/domain/entities"
)

type mockTransferRepo struct {
	transfers map[string]*entities.Transfer
}

func newMockTransferRepo() *mockTransferRepo {
	return &mockTransferRepo{
		transfers: make(map[string]*entities.Transfer),
	}
}

func (m *mockTransferRepo) Create(_ context.Context, t *entities.Transfer) error {
	m.transfers[t.ID] = t
	return nil
}

func (m *mockTransferRepo) GetByID(_ context.Context, id string) (*entities.Transfer, error) {
	t, ok := m.transfers[id]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	return t, nil
}

func (m *mockTransferRepo) GetByReference(_ context.Context, ref string) (*entities.Transfer, error) {
	return nil, nil
}

func (m *mockTransferRepo) UpdateStatus(_ context.Context, id string, status string) error {
	t, ok := m.transfers[id]
	if !ok {
		return fmt.Errorf("not found")
	}
	t.Status = status
	return nil
}

type publishedMsg struct {
	routingKey string
	event      any
}

type mockPublisher struct {
	messages []publishedMsg
	err      error
}

func (m *mockPublisher) Publish(_ context.Context, routingKey string, event any) error {
	m.messages = append(m.messages, publishedMsg{routingKey, event})
	return m.err
}

func (m *mockPublisher) Close() error {
	return nil
}

func setupService() (*TransferService, *mockTransferRepo, *mockPublisher) {
	repo := newMockTransferRepo()
	pub := &mockPublisher{messages: make([]publishedMsg, 0)}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewTransferService(repo, pub, logger), repo, pub
}

func findMessageByKey(pub *mockPublisher, key string) *publishedMsg {
	for _, msg := range pub.messages {
		if msg.routingKey == key {
			return &msg
		}
	}
	return nil
}

func TestCreateTransfer_Valid(t *testing.T) {
	svc, repo, pub := setupService()

	result, err := svc.CreateTransfer(context.Background(), commands.CreateTransferCommand{
		FromAccountID: "acc-1",
		ToAccountID:   "acc-2",
		Amount:        100.50,
		Currency:      "BRL",
	})
	if err != nil {
		t.Fatalf("CreateTransfer() error = %v", err)
	}
	if result.Status != "pending" {
		t.Errorf("Status = %q, want %q", result.Status, "pending")
	}
	if result.TransferID == "" {
		t.Error("TransferID should not be empty")
	}
	if result.Amount != 100 {
		t.Errorf("Amount = %d, want 100", result.Amount)
	}

	transfer, _ := repo.GetByID(context.Background(), result.TransferID)
	if transfer == nil {
		t.Error("transfer not saved in repo")
	}

	msg := findMessageByKey(pub, "account.debit.cmd")
	if msg == nil {
		t.Error("expected debit command message")
	}
}

func TestCreateTransfer_InvalidAmount(t *testing.T) {
	svc, _, _ := setupService()

	_, err := svc.CreateTransfer(context.Background(), commands.CreateTransferCommand{
		FromAccountID: "acc-1",
		ToAccountID:   "acc-2",
		Amount:        0,
		Currency:      "BRL",
	})
	if err == nil {
		t.Error("expected error for zero amount")
	}
}

func TestHandleDebitConfirmed(t *testing.T) {
	svc, repo, pub := setupService()

	result, _ := svc.CreateTransfer(context.Background(), commands.CreateTransferCommand{
		FromAccountID: "acc-1",
		ToAccountID:   "acc-2",
		Amount:        100,
		Currency:      "BRL",
	})

	pub.messages = nil // clear previous messages

	err := svc.HandleDebitConfirmed(context.Background(), result.TransferID)
	if err != nil {
		t.Fatalf("HandleDebitConfirmed() error = %v", err)
	}

	transfer, _ := repo.GetByID(context.Background(), result.TransferID)
	if transfer.Status != "processing" {
		t.Errorf("Status = %q, want %q", transfer.Status, "processing")
	}

	msg := findMessageByKey(pub, "account.credit.cmd")
	if msg == nil {
		t.Error("expected credit command message")
	}
}

func TestHandleDebitConfirmed_AlreadyProcessed(t *testing.T) {
	svc, repo, pub := setupService()

	result, _ := svc.CreateTransfer(context.Background(), commands.CreateTransferCommand{
		FromAccountID: "acc-1",
		ToAccountID:   "acc-2",
		Amount:        100,
		Currency:      "BRL",
	})

	transfer, _ := repo.GetByID(context.Background(), result.TransferID)
	transfer.MarkCompleted()

	pub.messages = nil

	err := svc.HandleDebitConfirmed(context.Background(), result.TransferID)
	if err != nil {
		t.Fatalf("HandleDebitConfirmed() error = %v", err)
	}

	msg := findMessageByKey(pub, "account.credit.cmd")
	if msg != nil {
		t.Error("should not send credit command for already processed transfer")
	}
}

func TestHandleCreditConfirmed(t *testing.T) {
	svc, repo, pub := setupService()

	result, _ := svc.CreateTransfer(context.Background(), commands.CreateTransferCommand{
		FromAccountID: "acc-1",
		ToAccountID:   "acc-2",
		Amount:        100,
		Currency:      "BRL",
	})

	_ = svc.HandleDebitConfirmed(context.Background(), result.TransferID)
	pub.messages = nil

	err := svc.HandleCreditConfirmed(context.Background(), result.TransferID)
	if err != nil {
		t.Fatalf("HandleCreditConfirmed() error = %v", err)
	}

	transfer, _ := repo.GetByID(context.Background(), result.TransferID)
	if transfer.Status != "completed" {
		t.Errorf("Status = %q, want %q", transfer.Status, "completed")
	}

	msg := findMessageByKey(pub, "transfer.completed")
	if msg == nil {
		t.Error("expected transfer.completed message")
	}
}

func TestHandleDebitFailed(t *testing.T) {
	svc, repo, pub := setupService()

	result, _ := svc.CreateTransfer(context.Background(), commands.CreateTransferCommand{
		FromAccountID: "acc-1",
		ToAccountID:   "acc-2",
		Amount:        100,
		Currency:      "BRL",
	})

	pub.messages = nil

	err := svc.HandleDebitFailed(context.Background(), result.TransferID, "insufficient funds")
	if err != nil {
		t.Fatalf("HandleDebitFailed() error = %v", err)
	}

	transfer, _ := repo.GetByID(context.Background(), result.TransferID)
	if transfer.Status != "failed" {
		t.Errorf("Status = %q, want %q", transfer.Status, "failed")
	}

	msg := findMessageByKey(pub, "transfer.failed")
	if msg == nil {
		t.Error("expected transfer.failed message")
	}
}

func TestHandleCreditFailed(t *testing.T) {
	svc, repo, pub := setupService()

	result, _ := svc.CreateTransfer(context.Background(), commands.CreateTransferCommand{
		FromAccountID: "acc-1",
		ToAccountID:   "acc-2",
		Amount:        100,
		Currency:      "BRL",
	})

	_ = svc.HandleDebitConfirmed(context.Background(), result.TransferID)
	pub.messages = nil

	err := svc.HandleCreditFailed(context.Background(), result.TransferID, "account not found")
	if err != nil {
		t.Fatalf("HandleCreditFailed() error = %v", err)
	}

	transfer, _ := repo.GetByID(context.Background(), result.TransferID)
	if transfer.Status != "failed" {
		t.Errorf("Status = %q, want %q", transfer.Status, "failed")
	}

	compensateMsg := findMessageByKey(pub, "account.compensate.cmd")
	if compensateMsg == nil {
		t.Error("expected account.compensate.cmd message")
	}

	failMsg := findMessageByKey(pub, "transfer.failed")
	if failMsg == nil {
		t.Error("expected transfer.failed message")
	}
}

func TestGetTransfer(t *testing.T) {
	svc, _, _ := setupService()

	created, _ := svc.CreateTransfer(context.Background(), commands.CreateTransferCommand{
		FromAccountID: "acc-1",
		ToAccountID:   "acc-2",
		Amount:        100,
		Currency:      "BRL",
	})

	result, err := svc.GetTransfer(context.Background(), created.TransferID)
	if err != nil {
		t.Fatalf("GetTransfer() error = %v", err)
	}
	if result.TransferID != created.TransferID {
		t.Errorf("TransferID = %q, want %q", result.TransferID, created.TransferID)
	}
	if result.FromAccountID != "acc-1" {
		t.Errorf("FromAccountID = %q, want %q", result.FromAccountID, "acc-1")
	}
	if result.ToAccountID != "acc-2" {
		t.Errorf("ToAccountID = %q, want %q", result.ToAccountID, "acc-2")
	}
	if result.Amount != 100 {
		t.Errorf("Amount = %d, want 100", result.Amount)
	}
	if result.Currency != "BRL" {
		t.Errorf("Currency = %q, want %q", result.Currency, "BRL")
	}
}

func TestGetTransfer_NotFound(t *testing.T) {
	svc, _, _ := setupService()

	_, err := svc.GetTransfer(context.Background(), "nonexistent-id")
	if err == nil {
		t.Error("expected error for nonexistent transfer")
	}
}
