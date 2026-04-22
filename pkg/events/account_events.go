package events

// Eventos que o Account Service publica após alterações de saldo.
// Outros serviços (Transaction, Notification) consomem estes eventos.

const (
	AccountCreatedEvent  = "account.created"
	AccountCreditedEvent = "account.credited"
	AccountDebitedEvent  = "account.debited"
	AccountBlockedEvent  = "account.blocked"

	AccountCompensateCmd = "account.compensate.cmd"
)

type AccountCreated struct {
	BaseEvent
	AccountID string `json:"account_id"`
	UserID    string `json:"user_id"`
	Currency  string `json:"currency"`
}

type AccountCredited struct {
	BaseEvent
	AccountID    string `json:"account_id"`
	Amount       int64  `json:"amount"` // centavos
	Reference    string `json:"reference"`
	BalanceAfter int64  `json:"balance_after"`
}

type AccountDebited struct {
	BaseEvent
	AccountID    string `json:"account_id"`
	Amount       int64  `json:"amount"` // centavos
	Reference    string `json:"reference"`
	BalanceAfter int64  `json:"balance_after"`
}

type AccountBlocked struct {
	BaseEvent
	AccountID string `json:"account_id"`
	Reason    string `json:"reason"`
}
