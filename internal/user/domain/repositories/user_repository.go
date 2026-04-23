package repositories

import (
	"context"

	"github.com/felipersas/payflow/internal/user/domain/entities"
)

// UserRepository é o contrato para persistência de usuários.
type UserRepository interface {
	Create(ctx context.Context, user *entities.User) error
	GetByEmail(ctx context.Context, email string) (*entities.User, error)
	GetByID(ctx context.Context, id string) (*entities.User, error)
}
