package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/felipersas/payflow/internal/transfer/domain/entities"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TransferRepositoryImpl struct {
	pool *pgxpool.Pool
}

func NewTransferRepository(pool *pgxpool.Pool) *TransferRepositoryImpl {
	return &TransferRepositoryImpl{pool: pool}
}

func (r *TransferRepositoryImpl) InitSchema(ctx context.Context) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS transfers (
			id VARCHAR(36) PRIMARY KEY,
			from_account_id VARCHAR(36) NOT NULL,
			to_account_id VARCHAR(36) NOT NULL,
			amount BIGINT NOT NULL,
			currency VARCHAR(3) NOT NULL,
			status VARCHAR(20) NOT NULL,
			version INTEGER NOT NULL DEFAULT 1,
			created_at TIMESTAMPTZ NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL,
			reference VARCHAR(255) UNIQUE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_transfers_from_account_id ON transfers(from_account_id)`,
		`CREATE INDEX IF NOT EXISTS idx_transfers_to_account_id ON transfers(to_account_id)`,
	}
	for _, q := range queries {
		if _, err := r.pool.Exec(ctx, q); err != nil {
			return fmt.Errorf("executing schema migration: %w", err)
		}
	}
	return nil
}

func (r *TransferRepositoryImpl) Create(ctx context.Context, transfer *entities.Transfer) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO transfers (id, from_account_id, to_account_id, amount, currency, status, version, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		transfer.ID,
		transfer.FromAccountID,
		transfer.ToAccountID,
		transfer.Amount,
		transfer.Currency,
		transfer.Status,
		transfer.Version,
		transfer.CreatedAt,
		transfer.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("inserting transfer: %w", err)
	}
	return nil
}

func (r *TransferRepositoryImpl) GetByID(ctx context.Context, id string) (*entities.Transfer, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, from_account_id, to_account_id, amount, currency, status, version, created_at, updated_at
		 FROM transfers WHERE id = $1`, id)

	var t entities.Transfer
	err := row.Scan(
		&t.ID,
		&t.FromAccountID,
		&t.ToAccountID,
		&t.Amount,
		&t.Currency,
		&t.Status,
		&t.Version,
		&t.CreatedAt,
		&t.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil // Not found
		}
		return nil, fmt.Errorf("querying transfer by ID: %w", err)
	}
	return &t, nil
}

func (r *TransferRepositoryImpl) GetByReference(ctx context.Context, reference string) (*entities.Transfer, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, from_account_id, to_account_id, amount, currency, status, version, created_at, updated_at
		 FROM transfers WHERE reference = $1`, reference)

	var t entities.Transfer
	err := row.Scan(
		&t.ID,
		&t.FromAccountID,
		&t.ToAccountID,
		&t.Amount,
		&t.Currency,
		&t.Status,
		&t.Version,
		&t.CreatedAt,
		&t.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil // Not found
		}
		return nil, fmt.Errorf("querying transfer by reference: %w", err)
	}
	return &t, nil
}

func (r *TransferRepositoryImpl) UpdateStatus(ctx context.Context, id string, status string) error {
	cmdTag, err := r.pool.Exec(ctx,
		`UPDATE transfers SET status = $1, updated_at = $2 WHERE id = $3`,
		status, time.Now().UTC(), id,
	)
	if err != nil {
		return fmt.Errorf("updating transfer status: %w", err)
	}
	if cmdTag.RowsAffected() == 0 {
		return fmt.Errorf("no transfer found with ID %s", id)
	}
	return nil
}

func (r *TransferRepositoryImpl) ListByAccountID(ctx context.Context, accountID string) ([]*entities.Transfer, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, from_account_id, to_account_id, amount, currency, status, version, created_at, updated_at
		 FROM transfers WHERE from_account_id = $1 OR to_account_id = $1 ORDER BY created_at DESC`, accountID,
	)
	if err != nil {
		return nil, fmt.Errorf("querying transfers by account ID: %w", err)
	}
	defer rows.Close()

	var transfers []*entities.Transfer
	for rows.Next() {
		var t entities.Transfer
		err := rows.Scan(
			&t.ID,
			&t.FromAccountID,
			&t.ToAccountID,
			&t.Amount,
			&t.Currency,
			&t.Status,
			&t.Version,
			&t.CreatedAt,
			&t.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning transfer row: %w", err)
		}
		transfers = append(transfers, &t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating transfer rows: %w", err)
	}
	return transfers, nil
}
