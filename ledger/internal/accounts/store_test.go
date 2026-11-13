// File: internal/accounts/store_test.go
package accounts_test

import (
	"context"
	"errors"
	"math/rand/v2"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/gopherbazaar/ledger/internal/accounts"
	"github.com/gopherbazaar/ledger/internal/pgtest"
)

func TestMovements(t *testing.T) {
	store := accounts.NewStore(pgtest.New(t, "../../migrations/accounts"))
	ctx := context.Background()

	alice, err := store.CreateAccount(ctx, "alice", 1_000)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		op      func(ctx context.Context, accountID, transferID, amount int64) error
		account int64
		amount  int64
		wantErr error
	}{
		{"debit zero", store.Debit, alice.ID, 0, accounts.ErrInvalidAmount},
		{"debit unknown account", store.Debit, 999_999, 10, accounts.ErrNotFound},
		{"debit too much", store.Debit, alice.ID, 5_000, accounts.ErrInsufficientFunds},
		{"debit ok", store.Debit, alice.ID, 300, nil},
		{"credit ok", store.Credit, alice.ID, 50, nil},
		{"refund ok", store.Refund, alice.ID, 300, nil},
	}
	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.op(ctx, tt.account, int64(i+1), tt.amount)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("got %v, want %v", err, tt.wantErr)
			}
		})
	}

	got, err := store.GetAccount(ctx, alice.ID)
	if err != nil {
		t.Fatal(err)
	}
	if want := int64(1_000 - 300 + 50 + 300); got.Balance != want {
		t.Errorf("balance = %d, want %d", got.Balance, want)
	}
}

// TestConcurrentMovementsConserveMoney does what the transfers service does,
// a debit followed by a credit, from many goroutines at once.
func TestConcurrentMovementsConserveMoney(t *testing.T) {
	store := accounts.NewStore(pgtest.New(t, "../../migrations/accounts"))
	ctx := context.Background()

	const (
		numAccounts = 5
		deposit     = 10_000
		workers     = 50
		perWorker   = 40
	)

	ids := make([]int64, numAccounts)
	for i := range ids {
		a, err := store.CreateAccount(ctx, "acct", deposit)
		if err != nil {
			t.Fatal(err)
		}
		ids[i] = a.ID
	}

	var (
		wg         sync.WaitGroup
		transferID atomic.Int64
		mu         sync.Mutex
		unexpected []error
	)
	record := func(err error) {
		mu.Lock()
		unexpected = append(unexpected, err)
		mu.Unlock()
	}

	for range workers {
		wg.Go(func() {
			for range perWorker {
				from, to := ids[rand.IntN(numAccounts)], ids[rand.IntN(numAccounts)]
				if from == to {
					continue
				}
				tid, amount := transferID.Add(1), rand.Int64N(1_000)+1

				err := store.Debit(ctx, from, tid, amount)
				if errors.Is(err, accounts.ErrInsufficientFunds) {
					continue
				}
				if err != nil {
					record(err)
					continue
				}
				if err := store.Credit(ctx, to, tid, amount); err != nil {
					record(err)
				}
			}
		})
	}
	wg.Wait()

	for _, err := range unexpected {
		t.Errorf("unexpected error: %v", err)
	}

	total, err := store.TotalBalance(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if want := int64(numAccounts * deposit); total != want {
		t.Errorf("total = %d, want %d", total, want)
	}

	mismatches, err := store.Reconcile(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(mismatches) > 0 {
		t.Errorf("books don't balance: %+v", mismatches)
	}
}
