package wallet_test

import (
	"context"
	"errors"
	"math/rand/v2"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/gopherbazaar/ledger/internal/wallet"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

// newTestStore starts a disposable Postgres container, applies every up
// migration, and returns a Store connected to it. The container is removed
// when the test finishes.
func newTestStore(t *testing.T) *wallet.Store {
	t.Helper()
	if testing.Short() {
		t.Skip("integration test: skipped with -short")
	}
	ctx := context.Background()

	ctr, err := postgres.Run(ctx, "postgres:17",
		postgres.WithDatabase("ledger"),
		postgres.WithUsername("ledger"),
		postgres.WithPassword("ledger"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		t.Fatalf("start postgres: %v", err)
	}
	t.Cleanup(func() { _ = ctr.Terminate(context.Background()) })

	dsn, err := ctr.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("connection string: %v", err)
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	// Glob returns names in sorted order, so 000001 runs before 000002.
	files, err := filepath.Glob("../../migrations/*.up.sql")
	if err != nil || len(files) == 0 {
		t.Fatalf("find migrations: %v (found %d)", err, len(files))
	}
	for _, f := range files {
		sql, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		// Exec with no arguments uses Postgres's simple protocol, which
		// allows several statements in one string.
		if _, err := pool.Exec(ctx, string(sql)); err != nil {
			t.Fatalf("apply %s: %v", f, err)
		}
	}

	return wallet.NewStore(pool)
}

func TestTransferValidation(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	alice, err := store.CreateAccount(ctx, "alice", 1_000)
	if err != nil {
		t.Fatal(err)
	}
	bob, err := store.CreateAccount(ctx, "bob", 0)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		from    int64
		to      int64
		amount  int64
		wantErr error
	}{
		{"zero amount", alice.ID, bob.ID, 0, wallet.ErrInvalidAmount},
		{"negative amount", alice.ID, bob.ID, -5, wallet.ErrInvalidAmount},
		{"same account", alice.ID, alice.ID, 100, wallet.ErrSameAccount},
		{"unknown destination", alice.ID, 999_999, 100, wallet.ErrNotFound},
		{"unknown source", 999_999, bob.ID, 100, wallet.ErrNotFound},
		{"insufficient funds", alice.ID, bob.ID, 5_000, wallet.ErrInsufficientFunds},
		{"valid transfer", alice.ID, bob.ID, 400, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := store.Transfer(ctx, tt.from, tt.to, tt.amount)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("got error %v, want %v", err, tt.wantErr)
			}
		})
	}

	got, err := store.GetAccount(ctx, bob.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Balance != 400 {
		t.Errorf("bob balance = %d, want 400", got.Balance)
	}
}

func TestConcurrentTransfersConserveMoney(t *testing.T) {
	store := newTestStore(t)
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
		mu         sync.Mutex
		unexpected []error
	)
	for range workers {
		wg.Go(func() {
			for range perWorker {
				from, to := ids[rand.IntN(numAccounts)], ids[rand.IntN(numAccounts)]
				if from == to {
					continue
				}
				_, err := store.Transfer(ctx, from, to, rand.Int64N(1_000)+1)
				if err != nil && !errors.Is(err, wallet.ErrInsufficientFunds) {
					mu.Lock()
					unexpected = append(unexpected, err)
					mu.Unlock()
				}
			}
		})
	}
	wg.Wait()

	// Deadlocks and constraint violations would show up here.
	for _, err := range unexpected {
		t.Errorf("unexpected transfer error: %v", err)
	}

	total, err := store.TotalBalance(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if want := int64(numAccounts * deposit); total != want {
		t.Errorf("total balance = %d, want %d (money was created or destroyed)", total, want)
	}

	for _, id := range ids {
		a, err := store.GetAccount(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		if a.Balance < 0 {
			t.Errorf("account %d has negative balance %d", id, a.Balance)
		}
	}

	mismatches, err := store.Reconcile(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(mismatches) > 0 {
		t.Errorf("books don't balance: %+v", mismatches)
	}
}
