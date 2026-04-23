package entities

import (
	"time"

	apperrors "github.com/felipersas/payflow/pkg/errors"
	"github.com/google/uuid"
)

// User representa um usuário autenticado do sistema.
type User struct {
	ID           string
	Email        string
	PasswordHash string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// NewUser cria um novo usuário com UUID v7 e validação.
func NewUser(email, passwordHash string) (*User, error) {
	if email == "" {
		return nil, apperrors.BusinessRule("email is required")
	}
	if passwordHash == "" {
		return nil, apperrors.BusinessRule("password is required")
	}

	now := time.Now().UTC()
	return &User{
		ID:           uuid.Must(uuid.NewV7()).String(),
		Email:        email,
		PasswordHash: passwordHash,
		CreatedAt:    now,
		UpdatedAt:    now,
	}, nil
}
