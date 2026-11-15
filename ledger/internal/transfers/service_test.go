package transfers_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"slices"
	"testing"
	"time"

	"github.com/gopherbazaar/ledger/internal/pgtest"
	"github.com/gopherbazaar/ledger/internal/transfers"
)

// fakeAccounts is a hand-written fake of the accounts service. Each method
// returns a scripted error and records that it was called.
type fakeAccounts struct {
	debitErr, creditErr, refundErr error
	debitHangs                     bool // Debit blocks until its context is done
	calls                          []string
}

func (f *fakeAccounts) GetAccount(_ context.Context, id int64) (transfers.AccountInfo, error) {
	f.calls = append(f.calls, "get")
	return transfers.AccountInfo{ID: id}, nil
}

func (f *fakeAccounts) Debit(ctx context.Context, _, _, _ int64) error {
	f.calls = append(f.calls, "debit")
	if f.debitHangs {
		<-ctx.Done()
		return ctx.Err()
	}
	return f.debitErr
}

func (f *fakeAccounts) Credit(context.Context, int64, int64, int64) error {
	f.calls = append(f.calls, "credit")
	return f.creditErr
}

func (f *fakeAccounts) Refund(context.Context, int64, int64, int64) error {
	f.calls = append(f.calls, "refund")
	return f.refundErr
}

// errNetwork stands in for any failure that leaves the outcome unknown.
var errNetwork = errors.New("connection reset by peer")

func TestTransferStateMachine(t *testing.T) {
	store := transfers.NewStore(pgtest.New(t, "../../migrations/transfers"))
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	tests := []struct {
		name       string
		fake       fakeAccounts
		wantErr    error
		wantStatus transfers.Status
		wantCalls  []string
	}{
		{
			name:       "happy path",
			wantStatus: transfers.StatusCompleted,
			wantCalls:  []string{"get", "debit", "credit"},
		},
		{
			name:       "debit rejected: no money moved",
			fake:       fakeAccounts{debitErr: transfers.ErrInsufficientFunds},
			wantErr:    transfers.ErrInsufficientFunds,
			wantStatus: transfers.StatusFailed,
			wantCalls:  []string{"get", "debit"},
		},
		{
			name:       "credit rejected: sender refunded",
			fake:       fakeAccounts{creditErr: transfers.ErrAccountNotFound},
			wantErr:    transfers.ErrAccountNotFound,
			wantStatus: transfers.StatusRefunded,
			wantCalls:  []string{"get", "debit", "credit", "refund"},
		},
		{
			name:       "debit ambiguous: stays pending",
			fake:       fakeAccounts{debitErr: errNetwork},
			wantErr:    transfers.ErrOutcomeUnknown,
			wantStatus: transfers.StatusPending,
			wantCalls:  []string{"get", "debit"},
		},
		{
			name:       "credit ambiguous: stays debited",
			fake:       fakeAccounts{creditErr: errNetwork},
			wantErr:    transfers.ErrOutcomeUnknown,
			wantStatus: transfers.StatusDebited,
			wantCalls:  []string{"get", "debit", "credit"},
		},
		{
			name:       "refund ambiguous: stays debited",
			fake:       fakeAccounts{creditErr: transfers.ErrAccountNotFound, refundErr: errNetwork},
			wantErr:    transfers.ErrOutcomeUnknown,
			wantStatus: transfers.StatusDebited,
			wantCalls:  []string{"get", "debit", "credit", "refund"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := tt.fake
			svc := transfers.NewService(store, &fake, logger)

			got, err := svc.Transfer(context.Background(), 1, 2, 100)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v, want %v", err, tt.wantErr)
			}
			if got.Status != tt.wantStatus {
				t.Errorf("returned status = %q, want %q", got.Status, tt.wantStatus)
			}

			// What's stored is what matters after a crash.
			stored, err := store.Get(context.Background(), got.ID)
			if err != nil {
				t.Fatal(err)
			}
			if stored.Status != tt.wantStatus {
				t.Errorf("stored status = %q, want %q", stored.Status, tt.wantStatus)
			}
			if !slices.Equal(fake.calls, tt.wantCalls) {
				t.Errorf("calls = %v, want %v", fake.calls, tt.wantCalls)
			}
		})
	}
}

// TestTransferRespectsDeadline checks two things when accounts never answers:
// the call returns soon after the caller's deadline, and the transfer's state
// is still recorded even though the caller's context has already expired.
func TestTransferRespectsDeadline(t *testing.T) {
	store := transfers.NewStore(pgtest.New(t, "../../migrations/transfers"))
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := transfers.NewService(store, &fakeAccounts{debitHangs: true}, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	start := time.Now()
	got, err := svc.Transfer(ctx, 1, 2, 100)
	elapsed := time.Since(start)

	if !errors.Is(err, transfers.ErrOutcomeUnknown) {
		t.Fatalf("error = %v, want ErrOutcomeUnknown", err)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("error = %v, want it to wrap context.DeadlineExceeded", err)
	}
	if elapsed > time.Second {
		t.Errorf("Transfer took %v; it should give up soon after the 200ms deadline", elapsed)
	}

	stored, err := store.Get(context.Background(), got.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Status != transfers.StatusPending || stored.LastError == "" {
		t.Errorf("stored = %+v, want pending with last_error recorded", stored)
	}
}
