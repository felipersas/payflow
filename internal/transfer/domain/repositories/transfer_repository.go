package repositories

import (
	"context"

	"github.com/felipersas/payflow/internal/transfer/domain/entities"
	"github.com/felipersas/payflow/pkg/pagination"
)

//go:generate mockgen -source=transfer_repository.go -destination=mock_transfer_repository.go -package=repositories

// TransferRepository é o contrato (porta) do DDD para transferências.
// A camada de domínio define a interface; a infraestrutura implementa.
type TransferRepository interface {
	// Create registra uma nova transferência no banco.
	Create(ctx context.Context, transfer *entities.Transfer) error

	// GetByID busca transferência pelo ID.
	GetByID(ctx context.Context, id string) (*entities.Transfer, error)

	// GetByReference busca transferência pela referência de idempotência.
	// Retorna nil se não existe (primeira tentativa).
	GetByReference(ctx context.Context, reference string) (*entities.Transfer, error)

	// UpdateStatus atualiza o status da transferência (e.g., "pending", "completed", "failed").
	UpdateStatus(ctx context.Context, id string, status string) error

	// ListByAccountID retorna transferências paginadas por cursor para uma conta.
	ListByAccountID(ctx context.Context, accountID string, params pagination.Params) ([]*entities.Transfer, error)
}
