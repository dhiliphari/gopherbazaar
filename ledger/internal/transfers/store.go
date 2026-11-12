package transfers

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("transfers: transfer not found")

type Status string

const (
	StatusPending   Status = "pending"   // created; debit not yet confirmed
	StatusDebited   Status = "debited"   // debit confirmed; credit not yet confirmed
	StatusCompleted Status = "completed" // credit confirmed
	StatusFailed    Status = "failed"    // debit definitely rejected; no money moved
	StatusRefunded  Status = "refunded"  // credit rejected; debit refunded
)

type Transfer struct {
	ID            int64     `json:"id"`
	FromAccountID int64     `json:"from_account_id"`
	ToAccountID   int64     `json:"to_account_id"`
	Amount        int64     `json:"amount"`
	Status        Status    `json:"status"`
	LastError     string    `json:"last_error,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// transferColumns is the column list every query returns, in scan order.
const transferColumns = `id, from_account_id, to_account_id, amount, status,
	COALESCE(last_error, ''), created_at, updated_at`

func scanTransfer(row pgx.Row) (Transfer, error) {
	var t Transfer
	err := row.Scan(&t.ID, &t.FromAccountID, &t.ToAccountID, &t.Amount, &t.Status,
		&t.LastError, &t.CreatedAt, &t.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Transfer{}, ErrNotFound
	}
	return t, err
}

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// Create records a new transfer in the pending state.
func (s *Store) Create(ctx context.Context, fromID, toID, amount int64) (Transfer, error) {
	t, err := scanTransfer(s.pool.QueryRow(ctx, `
		INSERT INTO transfers (from_account_id, to_account_id, amount)
		VALUES ($1, $2, $3)
		RETURNING `+transferColumns,
		fromID, toID, amount))
	if err != nil {
		return Transfer{}, fmt.Errorf("insert transfer: %w", err)
	}
	return t, nil
}

func (s *Store) Get(ctx context.Context, id int64) (Transfer, error) {
	t, err := scanTransfer(s.pool.QueryRow(ctx,
		`SELECT `+transferColumns+` FROM transfers WHERE id = $1`, id))
	if err != nil && !errors.Is(err, ErrNotFound) {
		return Transfer{}, fmt.Errorf("select transfer %d: %w", id, err)
	}
	return t, err
}

// SetStatus records a transfer's status and its most recent error, if any.
func (s *Store) SetStatus(ctx context.Context, id int64, status Status, lastErr string) (Transfer, error) {
	t, err := scanTransfer(s.pool.QueryRow(ctx, `
		UPDATE transfers
		SET status = $2, last_error = NULLIF($3, ''), updated_at = now()
		WHERE id = $1
		RETURNING `+transferColumns,
		id, status, lastErr))
	if err != nil {
		return Transfer{}, fmt.Errorf("update transfer %d: %w", id, err)
	}
	return t, nil
}

// Stats summarises transfers by status, plus the money currently "in flight":
// debited from a sender but not yet credited to a receiver.
type Stats struct {
	ByStatus map[Status]int64 `json:"by_status"`
	InFlight int64            `json:"in_flight"`
}

func (s *Store) Stats(ctx context.Context) (Stats, error) {
	st := Stats{ByStatus: map[Status]int64{}}

	rows, err := s.pool.Query(ctx, `SELECT status, count(*) FROM transfers GROUP BY status`)
	if err != nil {
		return Stats{}, fmt.Errorf("count transfers: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var status Status
		var n int64
		if err := rows.Scan(&status, &n); err != nil {
			return Stats{}, fmt.Errorf("scan count: %w", err)
		}
		st.ByStatus[status] = n
	}
	if err := rows.Err(); err != nil {
		return Stats{}, fmt.Errorf("count transfers: %w", err)
	}

	err = s.pool.QueryRow(ctx,
		`SELECT COALESCE(SUM(amount), 0)::bigint FROM transfers WHERE status = 'debited'`,
	).Scan(&st.InFlight)
	if err != nil {
		return Stats{}, fmt.Errorf("sum in-flight: %w", err)
	}
	return st, nil
}
