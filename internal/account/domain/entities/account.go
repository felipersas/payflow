package entities

import (
	"time"

	apperrors "github.com/felipersas/payflow/pkg/errors"
	"github.com/felipersas/payflow/pkg/events"
	"github.com/google/uuid"
)

// Account é a entidade raiz do agregado Account.
// Contém todas as regras de negócio relacionadas a saldo e bloqueio.
// O saldo é armazenado em centavos (int64) para evitar problemas com ponto flutuante.
type Account struct {
	ID        string
	UserID    string
	Balance   int64 // centavos
	Currency  string
	IsActive  bool
	Version   int
	CreatedAt time.Time
	UpdatedAt time.Time
}

// NewAccount cria uma nova conta com UUID v7 (time-ordered) e validação.
// Retorna erro se userID ou currency forem inválidos.
func NewAccount(userID, currency string) (*Account, error) {
	a := &Account{
		ID:        uuid.Must(uuid.NewV7()).String(),
		UserID:    userID,
		Balance:   0,
		Currency:  currency,
		IsActive:  true,
		Version:   1,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := a.validate(); err != nil {
		return nil, err
	}
	return a, nil
}

// Debit remove valor da conta. Regras:
// - Conta deve estar ativa
// - Amount deve ser positivo
// - Saldo deve ser suficiente
// - Reference é usada para idempotência
func (a *Account) Debit(amount int64, reference string) (*events.AccountDebited, error) {
	if !a.IsActive {
		return nil, apperrors.BusinessRule("account %s is blocked", a.ID)
	}
	if amount <= 0 {
		return nil, apperrors.BusinessRule("debit amount must be positive, got %d", amount)
	}
	if a.Balance < amount {
		return nil, apperrors.BusinessRule("insufficient balance: have %d, need %d", a.Balance, amount)
	}
	if reference == "" {
		return nil, apperrors.BusinessRule("reference is required for debit operations")
	}

	a.Balance -= amount
	a.Version++
	a.UpdatedAt = time.Now().UTC()

	event := &events.AccountDebited{
		BaseEvent:    events.NewBaseEvent(events.AccountDebitedEvent, 1),
		AccountID:    a.ID,
		Amount:       amount,
		Reference:    reference,
		BalanceAfter: a.Balance,
	}
	return event, nil
}

// Credit adiciona valor à conta. Regras:
// - Conta deve estar ativa
// - Amount deve ser positivo
// - Reference é usada para idempotência
func (a *Account) Credit(amount int64, reference string) (*events.AccountCredited, error) {
	if !a.IsActive {
		return nil, apperrors.BusinessRule("account %s is blocked", a.ID)
	}
	if amount <= 0 {
		return nil, apperrors.BusinessRule("credit amount must be positive, got %d", amount)
	}
	if reference == "" {
		return nil, apperrors.BusinessRule("reference is required for credit operations")
	}

	a.Balance += amount
	a.Version++
	a.UpdatedAt = time.Now().UTC()

	event := &events.AccountCredited{
		BaseEvent:    events.NewBaseEvent(events.AccountCreditedEvent, 1),
		AccountID:    a.ID,
		Amount:       amount,
		Reference:    reference,
		BalanceAfter: a.Balance,
	}
	return event, nil
}

// Block desativa a conta. Operações de débito/crédito serão recusadas.
func (a *Account) Block(reason string) *events.AccountBlocked {
	a.IsActive = false
	a.UpdatedAt = time.Now().UTC()

	return &events.AccountBlocked{
		BaseEvent: events.NewBaseEvent(events.AccountBlockedEvent, 1),
		AccountID: a.ID,
		Reason:    reason,
	}
}

// Unblock reativa a conta.
func (a *Account) Unblock() {
	a.IsActive = true
	a.UpdatedAt = time.Now().UTC()
}

func (a *Account) validate() error {
	if a.UserID == "" {
		return apperrors.BusinessRule("user_id is required")
	}
	if a.Currency == "" {
		return apperrors.BusinessRule("currency is required")
	}
	if len(a.Currency) != 3 {
		return apperrors.BusinessRule("currency must be 3 characters (ISO 4217), got %q", a.Currency)
	}
	return nil
}
