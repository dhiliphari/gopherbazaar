package transfers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
)

var (
	ErrInvalidAmount     = errors.New("transfers: amount must be positive")
	ErrSameAccount       = errors.New("transfers: cannot transfer to the same account")
	ErrAccountNotFound   = errors.New("transfers: account not found")
	ErrInsufficientFunds = errors.New("transfers: insufficient funds")
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

	t, err := s.store.Create(ctx, fromID, toID, amount)
	if err != nil {
		return Transfer{}, err
	}

	if err := s.accounts.Debit(ctx, fromID, t.ID, amount); err != nil {
		return Transfer{}, fmt.Errorf("debit: %w", err)
	}
	if err := s.accounts.Credit(ctx, toID, t.ID, amount); err != nil {
		return Transfer{}, fmt.Errorf("credit: %w", err)
	}

	return s.store.SetStatus(ctx, t.ID, StatusCompleted, "")
}
