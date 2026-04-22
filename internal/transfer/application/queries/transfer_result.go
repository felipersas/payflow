package queries

type TransferResult struct {
	TransferID    string `json:"transfer_id"`
	FromAccountID string `json:"from_account_id"`
	ToAccountID   string `json:"to_account_id"`
	Amount        int64  `json:"amount"` // em centavos
	Currency      string `json:"currency"`
	Status        string `json:"status"` // e.g., "completed", "failed"
}
