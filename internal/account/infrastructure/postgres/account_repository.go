package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/felipersas/payflow/internal/account/domain/entities"
	"github.com/felipersas/payflow/internal/account/domain/repositories"
	apperrors "github.com/felipersas/payflow/pkg/errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type txKeyType struct{}

var txKey = txKeyType{}

// querier abstracts the common DB operations between pgxpool.Pool and pgx.Tx.
type querier interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// verify interface compliance at compile time
var _ repositories.AccountRepository = (*AccountRepositoryImpl)(nil)

// AccountRepositoryImpl é a implementação concreta do contrato do domínio.
// Usa pgx para acesso ao PostgreSQL com connection pool.
type AccountRepositoryImpl struct {
	pool *pgxpool.Pool
}

func NewAccountRepository(pool *pgxpool.Pool) *AccountRepositoryImpl {
	return &AccountRepositoryImpl{pool: pool}
}

// getQuerier returns pgx.Tx from context (if inside a transaction) or falls back to the pool.
func (r *AccountRepositoryImpl) getQuerier(ctx context.Context) querier {
	if tx, ok := ctx.Value(txKey).(pgx.Tx); ok {
		return tx
	}
	return r.pool
}

func (r *AccountRepositoryImpl) RunInTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	txCtx := context.WithValue(ctx, txKey, tx)
	if err := fn(txCtx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *AccountRepositoryImpl) Create(ctx context.Context, account *entities.Account) error {
	_, err := r.getQuerier(ctx).Exec(ctx,
		`INSERT INTO accounts (id, user_id, balance, currency, is_active, version, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		account.ID, account.UserID, account.Balance, account.Currency,
		account.IsActive, account.Version, account.CreatedAt, account.UpdatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return apperrors.Conflict("account already exists for user %s", account.UserID)
		}
		return fmt.Errorf("inserting account: %w", err)
	}
	return nil
}

func (r *AccountRepositoryImpl) GetByID(ctx context.Context, id string) (*entities.Account, error) {
	var a entities.Account
	err := r.getQuerier(ctx).QueryRow(ctx,
		`SELECT id, user_id, balance, currency, is_active, version, created_at, updated_at
		 FROM accounts WHERE id = $1`, id,
	).Scan(&a.ID, &a.UserID, &a.Balance, &a.Currency, &a.IsActive, &a.Version, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NotFound("account %s not found", id)
		}
		return nil, fmt.Errorf("querying account: %w", err)
	}
	return &a, nil
}

func (r *AccountRepositoryImpl) GetByUserID(ctx context.Context, userID string) (*entities.Account, error) {
	var a entities.Account
	err := r.getQuerier(ctx).QueryRow(ctx,
		`SELECT id, user_id, balance, currency, is_active, version, created_at, updated_at
		 FROM accounts WHERE user_id = $1`, userID,
	).Scan(&a.ID, &a.UserID, &a.Balance, &a.Currency, &a.IsActive, &a.Version, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NotFound("account for user %s not found", userID)
		}
		return nil, fmt.Errorf("querying account by user_id: %w", err)
	}
	return &a, nil
}

// Update usa optimistic locking: só atualiza se a versão no DB bate com a da entidade.
// Se outro processo modificou a conta, a versão diverge e retorna erro.
func (r *AccountRepositoryImpl) Update(ctx context.Context, account *entities.Account) error {
	tag, err := r.getQuerier(ctx).Exec(ctx,
		`UPDATE accounts
		 SET balance = $1, is_active = $2, version = $3, updated_at = $4
		 WHERE id = $5 AND version = $6`,
		account.Balance, account.IsActive, account.Version, account.UpdatedAt,
		account.ID, account.Version-1, // version-1 = versão antes do débito/crédito
	)
	if err != nil {
		return fmt.Errorf("updating account: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("optimistic lock conflict for account %s: version mismatch", account.ID)
	}
	return nil
}

func (r *AccountRepositoryImpl) GetByReference(ctx context.Context, reference string) (*repositories.Transaction, error) {
	var tx repositories.Transaction
	var createdAt time.Time
	err := r.getQuerier(ctx).QueryRow(ctx,
		`SELECT id, account_id, reference, amount, type, balance_after, created_at
		 FROM transactions WHERE reference = $1`, reference,
	).Scan(&tx.ID, &tx.AccountID, &tx.Reference, &tx.Amount, &tx.Type, &tx.BalanceAfter, &createdAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil // Não existe = primeira tentativa
		}
		return nil, fmt.Errorf("querying transaction by reference: %w", err)
	}
	tx.CreatedAt = createdAt.Format(time.RFC3339)
	return &tx, nil
}

func (r *AccountRepositoryImpl) SaveTransaction(ctx context.Context, tx *repositories.Transaction) error {
	createdAt, err := time.Parse(time.RFC3339, tx.CreatedAt)
	if err != nil {
		createdAt = time.Now().UTC()
	}
	_, err = r.getQuerier(ctx).Exec(ctx,
		`INSERT INTO transactions (id, account_id, reference, amount, type, balance_after, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		tx.ID, tx.AccountID, tx.Reference, tx.Amount, tx.Type, tx.BalanceAfter, createdAt,
	)
	if err != nil {
		return fmt.Errorf("inserting transaction: %w", err)
	}
	return nil
}
