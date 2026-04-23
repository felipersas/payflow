package entities

import (
	"time"

	apperrors "github.com/felipersas/payflow/pkg/errors"
	"github.com/felipersas/payflow/pkg/events"
	"github.com/google/uuid"
)

type TransferStatus string

const (
	TransferPending    TransferStatus = "pending"
	TransferProcessing TransferStatus = "processing"
	TransferCompleted  TransferStatus = "completed"
	TransferFailed     TransferStatus = "failed"
)

// Transfer é a entidade raiz do agregado Transfer.
// Contém todas as regras de negócio relacionadas a transferências entre contas.
type Transfer struct {
	ID            string
	FromAccountID string
	ToAccountID   string
	Amount        int64 // centavos
	Currency      string
	Status        TransferStatus
	Version       int
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func NewTransfer(fromAccountID, toAccountID string, amount int64, currency string) (*Transfer, error) {
	t := &Transfer{
		ID:            uuid.Must(uuid.NewV7()).String(),
		FromAccountID: fromAccountID,
		ToAccountID:   toAccountID,
		Amount:        amount,
		Currency:      currency,
		Status:        TransferPending,
		Version:       1,
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}
	if err := t.validate(); err != nil {
		return nil, err
	}
	return t, nil
}

func (t *Transfer) validate() error {
	if t.FromAccountID == "" {
		return apperrors.BusinessRule("from account ID is required")
	}
	if t.ToAccountID == "" {
		return apperrors.BusinessRule("to account ID is required")
	}
	if t.Amount <= 0 {
		return apperrors.BusinessRule("amount must be positive, got %d", t.Amount)
	}
	if t.Currency == "" {
		return apperrors.BusinessRule("currency is required")
	}
	return nil
}

func (t *Transfer) MarkCompleted() (*events.TransferOcurred, error) {
	t.Status = TransferCompleted
	t.Version++
	t.UpdatedAt = time.Now().UTC()

	event := &events.TransferOcurred{
		TransferID:    t.ID,
		FromAccountID: t.FromAccountID,
		ToAccountID:   t.ToAccountID,
		Amount:        t.Amount,
		Currency:      t.Currency,
		Status:        string(t.Status),
	}

	return event, nil
}

func (t *Transfer) MarkFailed() (*events.TransferOcurred, error) {
	t.Status = TransferFailed
	t.Version++
	t.UpdatedAt = time.Now().UTC()

	event := &events.TransferOcurred{
		TransferID:    t.ID,
		FromAccountID: t.FromAccountID,
		ToAccountID:   t.ToAccountID,
		Amount:        t.Amount,
		Currency:      t.Currency,
		Status:        string(t.Status),
	}

	return event, nil
}

func (t *Transfer) IsPending() bool {
	return t.Status == TransferPending
}

func (t *Transfer) IsCompleted() bool {
	return t.Status == TransferCompleted
}
