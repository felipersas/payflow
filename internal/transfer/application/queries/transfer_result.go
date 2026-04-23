package queries

import "github.com/felipersas/payflow/internal/transfer/domain/entities"

type TransferResult struct {
	TransferID    string                  `json:"transfer_id"`
	FromAccountID string                  `json:"from_account_id"`
	ToAccountID   string                  `json:"to_account_id"`
	Amount        int64                   `json:"amount"` // em centavos
	Currency      string                  `json:"currency"`
	Status        entities.TransferStatus `json:"status"`
}
