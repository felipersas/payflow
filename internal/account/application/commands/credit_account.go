package commands

// CreditAccountCommand representa a intenção de creditar valor em uma conta.
type CreditAccountCommand struct {
	AccountID string `json:"account_id"`
	Amount    int64  `json:"amount"` // centavos
	Reference string `json:"reference"`
}
