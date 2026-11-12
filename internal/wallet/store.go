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

	ErrInsufficientFunds = errors.New("wallet: insufficient funds")
	ErrSameAccount       = errors.New("wallet: cannot transfer to same account")
)

type Account struct {
	ID        int64     `json:"id"`
	Owner     string    `json:"owner"`
	Balance   int64     `json:"balance"`
	Currency  string    `json:"currency"`
	CreatedAt time.Time `json:"created_at"`
}

type Transfer struct {
	ID            int64     `json:"id"`
	FromAccountID int64     `json:"from_account_id"`
	ToAccountID   int64     `json:"to_account_id"`
	Amount        int64     `json:"amount"`
	CreatedAt     time.Time `json:"created_at"`
}

// Mismatch is an account whose cached balance disagrees with its ledger entries.
type Mismatch struct {
	AccountID     int64 `json:"account_id"`
	Balance       int64 `json:"balance"`
	LedgerBalance int64 `json:"ledger_balance"`
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
	err := pgx.BeginFunc(ctx, s.db, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `
			INSERT INTO accounts (owner, balance)
			VALUES ($1, $2)
			RETURNING id, owner, balance, currency, created_at`,
			owner, openingDeposit,
		).Scan(&a.ID, &a.Owner, &a.Balance, &a.Currency, &a.CreatedAt)

		if err != nil {
			return fmt.Errorf("insert amount: %w", err)
		}

		// no entires for zero deposit
		if openingDeposit > 0 {
			_, err = tx.Exec(ctx,
				`INSERT INTO entries (account_id, amount) VALUES ($1, $2)`,
				a.ID, openingDeposit,
			)
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

type lockedAccount struct {
	ID      int64
	Balance int64
}

func (s *Store) Transfer(ctx context.Context, fromID, toID, amount int64) (Transfer, error) {
	if amount <= 0 {
		return Transfer{}, ErrInvalidAmount
	}

	if fromID == toID {
		return Transfer{}, ErrSameAccount
	}

	var t Transfer
	err := pgx.BeginFunc(ctx, s.db, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT id, balance
			FROM accounts
			WHERE id = ANY($1)
			ORDER BY id
			FOR UPDATE`,
			[]int64{fromID, toID},
		)
		if err != nil {
			return fmt.Errorf("lock accounts: %w", err)
		}

		locked, err := pgx.CollectRows(rows, pgx.RowToStructByPos[lockedAccount])
		if err != nil {
			return fmt.Errorf("lock accounts: %w", err)
		}
		if len(locked) != 2 {
			return ErrNotFound
		}

		var fromBalance int64
		for _, acc := range locked {
			if acc.ID == fromID {
				fromBalance = acc.Balance
			}
		}
		if fromBalance < amount {
			return ErrInsufficientFunds
		}

		_, err = tx.Exec(ctx, `UPDATE accounts SET balance = balance - $1 WHERE id = $2`, amount, fromID)
		if err != nil {
			return fmt.Errorf("debit %d: %w", fromID, err)
		}

		_, err = tx.Exec(ctx, `UPDATE accounts SET balance = balance + $1 WHERE id = $2`, amount, toID)
		if err != nil {
			return fmt.Errorf("credit %d: %w", fromID, err)
		}

		err = tx.QueryRow(ctx, `
			INSERT into transfers (from_account_id, to_account_id, amount)
			VALUES ($1, $2, $3)
			RETURNING id, from_account_id, to_account_id, amount, created_at`,
			fromID, toID, amount,
		).Scan(&t.ID, &t.FromAccountID, &t.ToAccountID, &t.Amount, &t.CreatedAt)
		if err != nil {
			return fmt.Errorf("insert transfer: %w", err)
		}

		_, err = tx.Exec(ctx, `
			INSERT INTO entries (account_id, transfer_id, amount)
			VALUES ($1, $3, $4), ($2, $3, $5)`,
			fromID, toID, t.ID, -amount, amount)
		if err != nil {
			return fmt.Errorf("insert entries: %w", err)
		}

		return nil
	})

	if err != nil {
		return Transfer{}, err
	}

	return t, err
}

// TotalBalance returns the sum of every balance. If money is neither created
// nor destroyed, this only changes when accounts are opened.
func (s *Store) TotalBalance(ctx context.Context) (int64, error) {
	var total int64
	// SUM(bigint) returns numeric in Postgres, so cast it back to bigint.
	err := s.db.QueryRow(ctx,
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
	// LEFT JOIN keeps accounts with no entries (an account opened with a zero deposit)
	rows, err := s.db.Query(ctx, `
		SELECT a.id, a.balance, COALESCE(SUM(e.amount), 0)::bigint
		FROM accounts a
		LEFT JOIN entries e on e.account_id = a.id
		GROUP BY a.id
		HAVING a.balance <> COALESCE(SUM(e.amount), 0)`,
	)
	if err != nil {
		return nil, fmt.Errorf("reconcile: %w", err)
	}

	return pgx.CollectRows(rows, pgx.RowToStructByPos[Mismatch])
}
