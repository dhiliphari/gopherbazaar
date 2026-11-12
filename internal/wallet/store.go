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

type lockedAccount struct {
	ID      int64
	Balance int64
}

func (s *Store) Transfer(ctx context.Context, fromID, toID, amount int64) error {
	if amount < 0 {
		return ErrInvalidAmount
	}

	if fromID == toID {
		return ErrSameAccount
	}

	return pgx.BeginFunc(ctx, s.db, func(tx pgx.Tx) error {
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

	_, err = tx.Exec(ctx, `		UPDATE accounts SET balance = balance - $1 		WHERE id = $2`, amount, 		fromID	)
	if err != nil {
		return fmt.Errorf("debit %d: %w", fromID, err)
	}

	_, err = tx.Exec(ctx, `		UPDATE accounts SET balance = balance + $1 		WHERE id = $2`, amount, toID	)
	if err != nil {
		return fmt.Errorf("credit %d: %w", fromID, err)
	}

	return nil
})
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
