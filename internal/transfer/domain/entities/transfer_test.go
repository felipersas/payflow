package entities

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
				assert.Error(t, err, "expected error")
				return
			}
			require.NoError(t, err, "unexpected error")
			assert.Equal(t, "pending", transfer.Status)
			assert.Equal(t, 1, transfer.Version)
			assert.NotEmpty(t, transfer.ID)
			assert.Equal(t, tt.fromAccountID, transfer.FromAccountID)
			assert.Equal(t, tt.toAccountID, transfer.ToAccountID)
			assert.Equal(t, tt.amount, transfer.Amount)
			assert.Equal(t, tt.currency, transfer.Currency)
		})
	}
}

func TestTransfer_MarkCompleted(t *testing.T) {
	transfer, _ := NewTransfer("acc-1", "acc-2", 1000, "BRL")

	event, err := transfer.MarkCompleted()
	require.NoError(t, err)
	assert.Equal(t, "completed", transfer.Status)
	assert.Equal(t, 2, transfer.Version)
	assert.Equal(t, transfer.ID, event.TransferID)
	assert.Equal(t, "completed", event.Status)
	assert.Equal(t, "acc-1", event.FromAccountID)
	assert.Equal(t, "acc-2", event.ToAccountID)
	assert.Equal(t, int64(1000), event.Amount)
	assert.Equal(t, "BRL", event.Currency)
}

func TestTransfer_MarkFailed(t *testing.T) {
	transfer, _ := NewTransfer("acc-1", "acc-2", 1000, "BRL")

	event, err := transfer.MarkFailed()
	require.NoError(t, err)
	assert.Equal(t, "failed", transfer.Status)
	assert.Equal(t, 2, transfer.Version)
	assert.Equal(t, "failed", event.Status)
}

func TestTransfer_IsPending(t *testing.T) {
	transfer, _ := NewTransfer("acc-1", "acc-2", 1000, "BRL")

	assert.True(t, transfer.IsPending(), "new transfer should be pending")

	transfer.MarkCompleted()
	assert.False(t, transfer.IsPending(), "completed transfer should not be pending")
}

func TestTransfer_IsCompleted(t *testing.T) {
	transfer, _ := NewTransfer("acc-1", "acc-2", 1000, "BRL")

	assert.False(t, transfer.IsCompleted(), "new transfer should not be completed")

	transfer.MarkCompleted()
	assert.True(t, transfer.IsCompleted(), "completed transfer should be completed")
}

func TestTransfer_Timestamps(t *testing.T) {
	before := time.Now().UTC()
	transfer, _ := NewTransfer("acc-1", "acc-2", 1000, "BRL")
	after := time.Now().UTC()

	assert.True(t, transfer.CreatedAt.After(before) || transfer.CreatedAt.Before(after), "CreatedAt should be close to now")
	assert.True(t, transfer.UpdatedAt.After(before) || transfer.UpdatedAt.Before(after), "UpdatedAt should be close to now")
}

func TestTransfer_MarkCompletedUpdatesTimestamp(t *testing.T) {
	transfer, _ := NewTransfer("acc-1", "acc-2", 1000, "BRL")
	oldUpdatedAt := transfer.UpdatedAt

	time.Sleep(1 * time.Millisecond)
	transfer.MarkCompleted()

	assert.True(t, transfer.UpdatedAt.After(oldUpdatedAt), "UpdatedAt should be updated after MarkCompleted")
}

func TestTransfer_MarkFailedUpdatesTimestamp(t *testing.T) {
	transfer, _ := NewTransfer("acc-1", "acc-2", 1000, "BRL")
	oldUpdatedAt := transfer.UpdatedAt

	time.Sleep(1 * time.Millisecond)
	transfer.MarkFailed()

	assert.True(t, transfer.UpdatedAt.After(oldUpdatedAt), "UpdatedAt should be updated after MarkFailed")
}
