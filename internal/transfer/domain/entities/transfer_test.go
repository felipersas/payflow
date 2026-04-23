package entities

import (
	"testing"
	"time"
)

func TestNewTransfer(t *testing.T) {
	tests := []struct {
		name         string
		fromAccountID string
		toAccountID   string
		amount        int64
		currency      string
		wantErr       bool
	}{
		{
			name:         "valid transfer",
			fromAccountID: "acc-1",
			toAccountID:   "acc-2",
			amount:        1000,
			currency:      "BRL",
			wantErr:       false,
		},
		{
			name:         "empty from",
			fromAccountID: "",
			toAccountID:   "acc-2",
			amount:        1000,
			currency:      "BRL",
			wantErr:       true,
		},
		{
			name:         "empty to",
			fromAccountID: "acc-1",
			toAccountID:   "",
			amount:        1000,
			currency:      "BRL",
			wantErr:       true,
		},
		{
			name:         "zero amount",
			fromAccountID: "acc-1",
			toAccountID:   "acc-2",
			amount:        0,
			currency:      "BRL",
			wantErr:       true,
		},
		{
			name:         "negative amount",
			fromAccountID: "acc-1",
			toAccountID:   "acc-2",
			amount:        -100,
			currency:      "BRL",
			wantErr:       true,
		},
		{
			name:         "empty currency",
			fromAccountID: "acc-1",
			toAccountID:   "acc-2",
			amount:        1000,
			currency:      "",
			wantErr:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transfer, err := NewTransfer(tt.fromAccountID, tt.toAccountID, tt.amount, tt.currency)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if transfer.Status != "pending" {
				t.Errorf("Status = %q, want %q", transfer.Status, "pending")
			}
			if transfer.Version != 1 {
				t.Errorf("Version = %d, want 1", transfer.Version)
			}
			if transfer.ID == "" {
				t.Error("ID should not be empty")
			}
			if transfer.FromAccountID != tt.fromAccountID {
				t.Errorf("FromAccountID = %q, want %q", transfer.FromAccountID, tt.fromAccountID)
			}
			if transfer.ToAccountID != tt.toAccountID {
				t.Errorf("ToAccountID = %q, want %q", transfer.ToAccountID, tt.toAccountID)
			}
			if transfer.Amount != tt.amount {
				t.Errorf("Amount = %d, want %d", transfer.Amount, tt.amount)
			}
			if transfer.Currency != tt.currency {
				t.Errorf("Currency = %q, want %q", transfer.Currency, tt.currency)
			}
		})
	}
}

func TestTransfer_MarkCompleted(t *testing.T) {
	transfer, _ := NewTransfer("acc-1", "acc-2", 1000, "BRL")

	event, err := transfer.MarkCompleted()
	if err != nil {
		t.Fatalf("MarkCompleted() error = %v", err)
	}
	if transfer.Status != "completed" {
		t.Errorf("Status = %q, want %q", transfer.Status, "completed")
	}
	if transfer.Version != 2 {
		t.Errorf("Version = %d, want 2", transfer.Version)
	}
	if event.TransferID != transfer.ID {
		t.Errorf("event TransferID = %q, want %q", event.TransferID, transfer.ID)
	}
	if event.Status != "completed" {
		t.Errorf("event Status = %q, want %q", event.Status, "completed")
	}
	if event.FromAccountID != "acc-1" {
		t.Errorf("event FromAccountID = %q, want %q", event.FromAccountID, "acc-1")
	}
	if event.ToAccountID != "acc-2" {
		t.Errorf("event ToAccountID = %q, want %q", event.ToAccountID, "acc-2")
	}
	if event.Amount != 1000 {
		t.Errorf("event Amount = %d, want 1000", event.Amount)
	}
	if event.Currency != "BRL" {
		t.Errorf("event Currency = %q, want %q", event.Currency, "BRL")
	}
}

func TestTransfer_MarkFailed(t *testing.T) {
	transfer, _ := NewTransfer("acc-1", "acc-2", 1000, "BRL")

	event, err := transfer.MarkFailed()
	if err != nil {
		t.Fatalf("MarkFailed() error = %v", err)
	}
	if transfer.Status != "failed" {
		t.Errorf("Status = %q, want %q", transfer.Status, "failed")
	}
	if transfer.Version != 2 {
		t.Errorf("Version = %d, want 2", transfer.Version)
	}
	if event.Status != "failed" {
		t.Errorf("event Status = %q, want %q", event.Status, "failed")
	}
}

func TestTransfer_IsPending(t *testing.T) {
	transfer, _ := NewTransfer("acc-1", "acc-2", 1000, "BRL")

	if !transfer.IsPending() {
		t.Error("new transfer should be pending")
	}

	transfer.MarkCompleted()
	if transfer.IsPending() {
		t.Error("completed transfer should not be pending")
	}
}

func TestTransfer_IsCompleted(t *testing.T) {
	transfer, _ := NewTransfer("acc-1", "acc-2", 1000, "BRL")

	if transfer.IsCompleted() {
		t.Error("new transfer should not be completed")
	}

	transfer.MarkCompleted()
	if !transfer.IsCompleted() {
		t.Error("completed transfer should be completed")
	}
}

func TestTransfer_Timestamps(t *testing.T) {
	before := time.Now().UTC()
	transfer, _ := NewTransfer("acc-1", "acc-2", 1000, "BRL")
	after := time.Now().UTC()

	if transfer.CreatedAt.Before(before) || transfer.CreatedAt.After(after) {
		t.Error("CreatedAt should be close to now")
	}
	if transfer.UpdatedAt.Before(before) || transfer.UpdatedAt.After(after) {
		t.Error("UpdatedAt should be close to now")
	}
}

func TestTransfer_MarkCompletedUpdatesTimestamp(t *testing.T) {
	transfer, _ := NewTransfer("acc-1", "acc-2", 1000, "BRL")
	oldUpdatedAt := transfer.UpdatedAt

	time.Sleep(1 * time.Millisecond)
	transfer.MarkCompleted()

	if !transfer.UpdatedAt.After(oldUpdatedAt) {
		t.Error("UpdatedAt should be updated after MarkCompleted")
	}
}

func TestTransfer_MarkFailedUpdatesTimestamp(t *testing.T) {
	transfer, _ := NewTransfer("acc-1", "acc-2", 1000, "BRL")
	oldUpdatedAt := transfer.UpdatedAt

	time.Sleep(1 * time.Millisecond)
	transfer.MarkFailed()

	if !transfer.UpdatedAt.After(oldUpdatedAt) {
		t.Error("UpdatedAt should be updated after MarkFailed")
	}
}
