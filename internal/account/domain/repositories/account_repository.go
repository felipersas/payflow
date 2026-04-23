package repositories

import (
	"context"

	"github.com/felipersas/payflow/internal/account/domain/entities"
)

//go:generate mockgen -source=account_repository.go -destination=mock_account_repository.go -package=repositories

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

	// RunInTransaction executa fn dentro de uma transação DB atômica.
	// O ctx passado para fn carrega a transação; todas as operações
	// do repositório chamadas com esse ctx usam a mesma transação.
	RunInTransaction(ctx context.Context, fn func(ctx context.Context) error) error
}

// TransactionType representa o tipo de transação.
type TransactionType string

const (
	TransactionCredit TransactionType = "credit"
	TransactionDebit  TransactionType = "debit"
)

// Transaction representa o registro de idempotência.
// Se uma transação com a mesma referência já existe, a operação é ignorada.
type Transaction struct {
	ID           string
	AccountID    string
	Reference    string
	Amount       int64
	Type         TransactionType
	BalanceAfter int64
	CreatedAt    string
}
