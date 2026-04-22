package queries

// GetBalanceQuery representa a intenção de consultar o saldo de uma conta.
// Queries representam "o que o usuário quer saber" (leitura).
type GetBalanceQuery struct {
	AccountID string `json:"account_id"`
}
