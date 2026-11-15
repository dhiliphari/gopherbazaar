package transfers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/gopherbazaar/ledger/internal/reqid"
)

var (
	ErrInvalidAmount     = errors.New("transfers: amount must be positive")
	ErrSameAccount       = errors.New("transfers: cannot transfer to the same account")
	ErrAccountNotFound   = errors.New("transfers: account not found")
	ErrInsufficientFunds = errors.New("transfers: insufficient funds")

	// ErrOutcomeUnknown means a call to accounts failed in a way that doesn't
	// tell us whether it took effect (a timeout, a reset connection, a 5xx).
	ErrOutcomeUnknown = errors.New("transfers: outcome unknown")
)

// AccountInfo is the part of an account the transfers service cares about.
type AccountInfo struct {
	ID      int64
	Owner   string
	Balance int64
}

// AccountsClient is what the transfers service needs from the accounts
// service. Implementations must return ErrAccountNotFound and
// ErrInsufficientFunds for those answers. Any other error means the outcome
// of the call is unknown.
type AccountsClient interface {
	GetAccount(ctx context.Context, id int64) (AccountInfo, error)
	Debit(ctx context.Context, accountID, transferID, amount int64) error
	Credit(ctx context.Context, accountID, transferID, amount int64) error
	Refund(ctx context.Context, accountID, transferID, amount int64) error
}

type Service struct {
	store    *Store
	accounts AccountsClient
	logger   *slog.Logger
}

func NewService(store *Store, accounts AccountsClient, logger *slog.Logger) *Service {
	return &Service{store: store, accounts: accounts, logger: logger}
}

func (s *Service) Transfer(ctx context.Context, fromID, toID, amount int64) (Transfer, error) {
	if amount <= 0 {
		return Transfer{}, ErrInvalidAmount
	}
	if fromID == toID {
		return Transfer{}, ErrSameAccount
	}

	if _, err := s.accounts.GetAccount(ctx, toID); err != nil {
		if errors.Is(err, ErrAccountNotFound) {
			return Transfer{}, err
		}
		return Transfer{}, fmt.Errorf("check destination: %w", err)
	}

	t, err := s.store.Create(ctx, fromID, toID, amount)
	if err != nil {
		return Transfer{}, err
	}

	return s.execute(ctx, t)
}

// execute drives a transfer forward from whatever state it is in, recording
// the outcome of every step before taking the next one.
func (s *Service) execute(ctx context.Context, t Transfer) (Transfer, error) {
	if t.Status == StatusPending {
		err := s.accounts.Debit(ctx, t.FromAccountID, t.ID, t.Amount)
		switch {
		case isDefinite(err):
			// Accounts said no. No money moved.
			return s.finish(ctx, t, StatusFailed, err)
		case err != nil:
			return s.unknown(ctx, t, "debit", err)
		}
		if t, err = s.store.SetStatus(ctx, t.ID, StatusDebited, ""); err != nil {
			return t, err
		}
	}

	if t.Status == StatusDebited {
		err := s.accounts.Credit(ctx, t.ToAccountID, t.ID, t.Amount)
		switch {
		case isDefinite(err):
			if rerr := s.accounts.Refund(ctx, t.FromAccountID, t.ID, t.Amount); rerr != nil {
				return s.unknown(ctx, t, "refund", rerr)
			}
			return s.finish(ctx, t, StatusRefunded, err)
		case err != nil:
			return s.unknown(ctx, t, "credit", err)
		}
		return s.store.SetStatus(ctx, t.ID, StatusCompleted, "")
	}

	return t, nil
}

// isDefinite reports whether err is an answer from accounts, as opposed to a
// failure to get an answer.
func isDefinite(err error) bool {
	return errors.Is(err, ErrAccountNotFound) || errors.Is(err, ErrInsufficientFunds)
}

// finish moves the transfer to a final status and returns cause to the caller.
func (s *Service) finish(ctx context.Context, t Transfer, status Status, cause error) (Transfer, error) {
	t, err := s.store.SetStatus(ctx, t.ID, status, cause.Error())
	if err != nil {
		return t, err
	}
	return t, cause
}

// unknown records an ambiguous failure without changing the status.
func (s *Service) unknown(ctx context.Context, t Transfer, step string, cause error) (Transfer, error) {
	s.logger.WarnContext(ctx, "transfer outcome unknown",
		"transfer_id", t.ID, "status", t.Status, "step", step, "error", cause)

	// The caller's context may be the thing that failed (it was cancelled,
	// say), so record the error using a short context of our own.
	recCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
	defer cancel()
	if updated, err := s.store.SetStatus(recCtx, t.ID, t.Status, step+": "+cause.Error()); err == nil {
		t = updated
	}
	return t, fmt.Errorf("%w: %s for transfer %d: %w", ErrOutcomeUnknown, step, t.ID, cause)
}

// ResumeStuck claims up to limit transfers that have been unfinished for at
// least olderThan and drives each one forward. It returns how many it
// claimed. Because every accounts operation is idempotent, resuming a
// transfer that another request is still working on is harmless.
func (s *Service) ResumeStuck(ctx context.Context, olderThan, lease time.Duration, limit int) (int, error) {
	stuck, err := s.store.ClaimStuck(ctx, olderThan, lease, limit)
	if err != nil {
		return 0, err
	}

	for _, t := range stuck {
		tctx := reqid.With(ctx, reqid.New())
		resumed, err := s.execute(tctx, t)
		s.logger.InfoContext(ctx, "transfer resumed",
			"transfer_id", t.ID, "from_status", t.Status, "to_status", resumed.Status, "error", err)
	}

	return len(stuck), nil
}

// RunResumer calls ResumeStuck every interval until ctx is cancelled.
func (s *Service) RunResumer(ctx context.Context, interval, olderThan time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			rctx, cancel := context.WithTimeout(ctx, interval)
			if _, err := s.ResumeStuck(rctx, olderThan, 30*time.Second, 20); err != nil {
				s.logger.ErrorContext(rctx, "resume stuck transfers", "error", err)
			}
			cancel()
		}
	}
}
