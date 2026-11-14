package accountsclient_test

import (
	"context"
	"io"
	"log/slog"
	"net"
	"testing"

	accountsv1 "github.com/gopherbazaar/ledger/gen/ledger/accounts/v1"
	"github.com/gopherbazaar/ledger/internal/accounts"
	"github.com/gopherbazaar/ledger/internal/accountsclient"
	"github.com/gopherbazaar/ledger/internal/pgtest"
	"github.com/gopherbazaar/ledger/internal/transfers"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

// newClient runs the real accounts gRPC server (backed by a real Postgres)
// on an in-memory connection, and returns our client connected to it.
func newClient(t *testing.T) (*accountsclient.Client, *accounts.Store) {
	t.Helper()

	store := accounts.NewStore(pgtest.New(t, "../../migrations/accounts"))
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	lis := bufconn.Listen(1 << 20)
	srv := grpc.NewServer()
	accountsv1.RegisterAccountsServiceServer(srv, accounts.NewGRPCServer(store, logger))
	go srv.Serve(lis)
	t.Cleanup(srv.Stop)

	conn, err := grpc.NewClient("passthrough:///bufnet", grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
		return lis.DialContext(ctx)
	}), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })

	return accountsclient.New(accountsv1.NewAccountsServiceClient(conn)), store
}

func TestErrorContract(t *testing.T) {
	client, store := newClient(t)
	ctx := context.Background()

	alice, err := store.CreateAccount(ctx, "alice", 1_000)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		call    func() error
		wantErr error
	}{
		{"debit ok", func() error { return client.Debit(ctx, alice.ID, 1, 100) }, nil},
		{"debit too much", func() error { return client.Debit(ctx, alice.ID, 2, 99_999) }, transfers.ErrInsufficientFunds},
		{"debit unknown account", func() error { return client.Debit(ctx, 999_999, 3, 100) }, transfers.ErrAccountNotFound},
		{"credit unknown account", func() error { return client.Credit(ctx, 999_999, 4, 100) }, transfers.ErrAccountNotFound},
		{"get unknown account", func() error { _, err := client.GetAccount(ctx, 999_999); return err }, transfers.ErrAccountNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.call(); err != tt.wantErr {
				t.Fatalf("got %v, want %v", err, tt.wantErr)
			}
		})
	}

	got, err := client.GetAccount(ctx, alice.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Balance != 900 {
		t.Errorf("balance = %d, want 900", got.Balance)
	}
}
