package repositories

import (
	"context"

	"github.com/felipersas/payflow/internal/account/domain/entities"
)

// AccountRepository é o contrato (porta) do DDD.
// A camada de domínio define a interface; a infraestrutura implementa.
type AccountRepository interface {
	// Create insere uma nova conta no banco.
	Create(ctx context.Context, account *entities.Account) error

	// GetByID busca conta pelo ID.
	GetByID(ctx context.Context, id string) (*entities.Account, error)

	// GetByUserID busca conta pelo ID do usuário.
	GetByUserID(ctx context.Context, userID string) (*entities.Account, error)

	// Update atualiza a conta com optimistic locking via Version.
	// Retorna erro se a versão no DB for diferente (conflito).
	Update(ctx context.Context, account *entities.Account) error

	// GetByReference busca transação pela referência de idempotência.
	// Retorna nil se não existe (primeira tentativa).
	GetByReference(ctx context.Context, reference string) (*Transaction, error)

	// SaveTransaction registra uma transação para idempotência.
	SaveTransaction(ctx context.Context, tx *Transaction) error
}

// Transaction representa o registro de idempotência.
// Se uma transação com a mesma referência já existe, a operação é ignorada.
type Transaction struct {
	ID          string
	AccountID   string
	Reference   string
	Amount      int64
	Type        string // "credit" ou "debit"
	BalanceAfter int64
	CreatedAt   string
}
