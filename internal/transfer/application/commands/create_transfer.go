package commands

type CreateTransferCommand struct {
	FromAccountID string  `json:"from_account_id"`
	ToAccountID   string  `json:"to_account_id"`
	Amount        float64 `json:"amount"` // em centavos
	Currency      string  `json:"currency"`
}
