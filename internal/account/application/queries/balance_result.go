package queries

// BalanceResult é o resultado da query de saldo.
// DTO separado da entidade — não expõe dados internos como Version.
type BalanceResult struct {
	AccountID string `json:"account_id"`
	Balance   int64  `json:"balance"` // centavos
	Currency  string `json:"currency"`
	IsActive  bool   `json:"is_active"`
}
