package accounts

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrInvalidAmount     = errors.New("accounts: amount must be positive")
	ErrNotFound          = errors.New("accounts: account not found")
	ErrInsufficientFunds = errors.New("accounts: insufficient funds")
)

// EntryKind says why a ledger entry exists.
type EntryKind string

const (
	KindDeposit EntryKind = "deposit"
	KindDebit   EntryKind = "debit"
	KindCredit  EntryKind = "credit"
	KindRefund  EntryKind = "refund"
)

type Account struct {
	ID        int64     `json:"id"`
	Owner     string    `json:"owner"`
	Balance   int64     `json:"balance"`
	Currency  string    `json:"currency"`
	CreatedAt time.Time `json:"created_at"`
}

// Mismatch is an account whose cached balance disagrees with its ledger entries.
type Mismatch struct {
	AccountID     int64 `json:"account_id"`
	Balance       int64 `json:"balance"`
	LedgerBalance int64 `json:"ledger_balance"`
}

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func (s *Store) CreateAccount(ctx context.Context, owner string, openingDeposit int64) (Account, error) {
	if openingDeposit < 0 {
		return Account{}, ErrInvalidAmount
	}

	var a Account
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `
			INSERT INTO accounts (owner, balance)
			VALUES ($1, $2)
			RETURNING id, owner, balance, currency, created_at`,
			owner, openingDeposit,
		).Scan(&a.ID, &a.Owner, &a.Balance, &a.Currency, &a.CreatedAt)
		if err != nil {
			return fmt.Errorf("insert account: %w", err)
		}

		if openingDeposit > 0 {
			_, err = tx.Exec(ctx,
				`INSERT INTO entries (account_id, kind, amount) VALUES ($1, $2, $3)`,
				a.ID, KindDeposit, openingDeposit)
			if err != nil {
				return fmt.Errorf("insert deposit entry: %w", err)
			}
		}
		return nil
	})
	return a, err
}

func (s *Store) GetAccount(ctx context.Context, id int64) (Account, error) {
	var a Account
	err := s.pool.QueryRow(ctx, `
		SELECT id, owner, balance, currency, created_at
		FROM accounts
		WHERE id = $1`,
		id,
	).Scan(&a.ID, &a.Owner, &a.Balance, &a.Currency, &a.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Account{}, ErrNotFound
	}
	if err != nil {
		return Account{}, fmt.Errorf("select account %d: %w", id, err)
	}
	return a, nil
}

// Debit removes amount from an account on behalf of a transfer.
func (s *Store) Debit(ctx context.Context, accountID, transferID, amount int64) error {
	if amount <= 0 {
		return ErrInvalidAmount
	}
	return s.apply(ctx, accountID, transferID, -amount, KindDebit)
}

// Credit adds amount to an account on behalf of a transfer.
func (s *Store) Credit(ctx context.Context, accountID, transferID, amount int64) error {
	if amount <= 0 {
		return ErrInvalidAmount
	}
	return s.apply(ctx, accountID, transferID, amount, KindCredit)
}

// Refund gives back an amount previously debited for a transfer.
func (s *Store) Refund(ctx context.Context, accountID, transferID, amount int64) error {
	if amount <= 0 {
		return ErrInvalidAmount
	}
	return s.apply(ctx, accountID, transferID, amount, KindRefund)
}

// apply locks one account, changes its balance by delta and records a ledger
// entry, all in one local transaction. Each call locks exactly one row, so
// two calls can never deadlock with each other.
func (s *Store) apply(ctx context.Context, accountId, transferID, delta int64, kind EntryKind) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		var balance int64
		err := tx.QueryRow(ctx,
			`SELECT balance FROM accounts WHERE id = $1 FOR UPDATE`, accountId).Scan(&balance)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("lock account %d: %w", accountId, err)
		}

		if balance+delta < 0 {
			return ErrInsufficientFunds
		}

		_, err = tx.Exec(ctx,
			`UPDATE accounts SET balance = balance + $1 WHERE id = $2`, delta, accountId)
		if err != nil {
			return fmt.Errorf("update balance %d: %w", accountId, err)
		}

		_, err = tx.Exec(ctx,
			`INSERT INTO entries (account_id, transfer_id, kind, amount) 
			VALUES ($1, $2, $3, $4)`,
			accountId, transferID, kind, delta)
		if err != nil {
			return fmt.Errorf("insert entry: %w", err)
		}

		return nil
	})
}

// TotalBalance returns the sum of every balance.
func (s *Store) TotalBalance(ctx context.Context) (int64, error) {
	var total int64
	err := s.pool.QueryRow(ctx,
		`SELECT COALESCE(SUM(balance), 0)::bigint FROM accounts`,
	).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("sum balances: %w", err)
	}
	return total, nil
}

// Reconcile returns every account whose cached balance disagrees with the
// sum of its ledger entries. An empty result means the books balance.
func (s *Store) Reconcile(ctx context.Context) ([]Mismatch, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT a.id, a.balance, COALESCE(SUM(e.amount), 0)::bigint
		FROM accounts a
		LEFT JOIN entries e ON e.account_id = a.id
		GROUP BY a.id
		HAVING a.balance <> COALESCE(SUM(e.amount), 0)`)
	if err != nil {
		return nil, fmt.Errorf("reconcile: %w", err)
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[Mismatch])
}
