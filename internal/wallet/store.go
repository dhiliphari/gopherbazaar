package wallet

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrInvalidAmount = errors.New("wallet: amount must be positive")
	ErrNotFound      = errors.New("wallet: account not found")
)

type Account struct {
	ID        int64     `json:"id"`
	Owner     string    `json:"owner"`
	Balance   int64     `json:"balance"`
	Currency  string    `json:"currency"`
	CreatedAt time.Time `json:"created_at"`
}

type Store struct {
	db *pgxpool.Pool
}

func NewStore(db *pgxpool.Pool) *Store {
	return &Store{db: db}
}

func (s *Store) CreateAccount(ctx context.Context, owner string, openingDeposit int64) (Account, error) {
	if openingDeposit < 0 {
		return Account{}, ErrInvalidAmount
	}

	var a Account
	err := s.db.QueryRow(ctx, `
		INSERT INTO accounts (owner, balance)
		VALUES ($1, $2)
		RETURNING id, owner, balance, currency, created_at`,
		owner, openingDeposit,
	).Scan(&a.ID, &a.Owner, &a.Balance, &a.Currency, &a.CreatedAt)

	if err != nil {
		return Account{}, fmt.Errorf("insert amount: %w", err)
	}

	return a, nil
}

func (s *Store) GetAccount(ctx context.Context, id int64) (Account, error) {
	var a Account

	err := s.db.QueryRow(ctx, `
		SELECT id, owner, balance, currency, created_at
		FROM accounts
		WHERE id = $1`,
		id,
	).Scan(&a.ID, &a.Owner, &a.Balance, &a.Currency, &a.CreatedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		return Account{}, ErrNotFound
	}
	if err != nil {
		return Account{}, fmt.Errorf("get account %d: %w", id, err)
	}

	return a, nil
}
