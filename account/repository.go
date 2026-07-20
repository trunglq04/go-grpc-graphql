package account

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/lib/pq"
)

type Repository interface {
	Close()
	PutAccount(ctx context.Context, a Account) error
	GetAccountByID(ctx context.Context, id string) (*Account, error)
	ListAccounts(ctx context.Context, skip uint64, take uint64) ([]Account, error)
}

type postgresRepository struct {
	db *sql.DB
}

func NewPostgresRepository(url string) (Repository, error) {
	db, err := sql.Open("postgres", url)
	if err != nil {
		return nil, err
	}

	err = db.Ping()
	if err != nil {
		return nil, err
	}

	return &postgresRepository{db}, nil
}

func (r *postgresRepository) Close() {
	r.db.Close()
}

func (r *postgresRepository) Ping() error {
	return r.db.Ping()
}

func (r *postgresRepository) PutAccount(ctx context.Context, a Account) error {
	query := `
		INSERT INTO accounts(id, name)
		VALUES ($1, $2)
	`
	_, err := r.db.ExecContext(ctx, query, a.ID, a.Name)
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) {
			return fmt.Errorf("PutAccount: SQL error code %s - %s: %w", pqErr.Code, pqErr.Message, err)
		}
		return fmt.Errorf("PutAccount: failed to insert account (id=%s): %w", a.ID, err)
	}

	return nil
}

func (r *postgresRepository) GetAccountByID(ctx context.Context, id string) (*Account, error) {
	query := `	
		SELECT id, name FROM accounts
		WHERE id = $1
	`

	a := &Account{}
	row := r.db.QueryRowContext(ctx, query, id)
	if err := row.Scan(&a.ID, &a.Name); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("GetAccountByID: account not found (id=%s)", id)
		}
		return nil, fmt.Errorf("GetAccountByID: scan error: %w", err)
	}
	return a, nil
}

func (r *postgresRepository) ListAccounts(ctx context.Context, skip uint64, take uint64) ([]Account, error) {
	query := `	
		SELECT * FROM accounts
		ORDER BY id DESC
		OFFSET $1 LIMIT $2
	`
	rows, err := r.db.QueryContext(ctx, query, skip, take)
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) {
			return nil, fmt.Errorf("ListAccounts: SQL error code %s - %s: %w", pqErr.Code, pqErr.Message, err)
		}
		return nil, fmt.Errorf("ListAccounts: query failed (skip=%d, take=%d): %w", skip, take, err)
	}
	defer rows.Close()

	accounts := []Account{}
	for rows.Next() {
		a := &Account{}
		if err := rows.Scan(&a.ID, &a.Name); err == nil {
			accounts = append(accounts, *a)
		}
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("ListAccounts: iteration error: %w", err)
	}
	return accounts, nil
}
