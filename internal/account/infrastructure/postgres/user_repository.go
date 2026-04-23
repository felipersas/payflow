package postgres

import (
	"context"
	"fmt"

	"github.com/felipersas/payflow/internal/account/domain/entities"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// UserRepositoryImpl implementa repositories.UserRepository com pgx.
type UserRepositoryImpl struct {
	pool *pgxpool.Pool
}

func NewUserRepository(pool *pgxpool.Pool) *UserRepositoryImpl {
	return &UserRepositoryImpl{pool: pool}
}

func (r *UserRepositoryImpl) Create(ctx context.Context, user *entities.User) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO users (id, email, password_hash, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5)`,
		user.ID, user.Email, user.PasswordHash, user.CreatedAt, user.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("inserting user: %w", err)
	}
	return nil
}

func (r *UserRepositoryImpl) GetByEmail(ctx context.Context, email string) (*entities.User, error) {
	var u entities.User
	err := r.pool.QueryRow(ctx,
		`SELECT id, email, password_hash, created_at, updated_at
		 FROM users WHERE email = $1`, email,
	).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("querying user by email: %w", err)
	}
	return &u, nil
}

func (r *UserRepositoryImpl) GetByID(ctx context.Context, id string) (*entities.User, error) {
	var u entities.User
	err := r.pool.QueryRow(ctx,
		`SELECT id, email, password_hash, created_at, updated_at
		 FROM users WHERE id = $1`, id,
	).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("querying user by id: %w", err)
	}
	return &u, nil
}
