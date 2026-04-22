package commands

// DebitAccountCommand representa a intenção de debitar valor de uma conta.
type DebitAccountCommand struct {
	AccountID string `json:"account_id"`
	Amount    int64  `json:"amount"` // centavos
	Reference string `json:"reference"`
}
