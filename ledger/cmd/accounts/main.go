package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"

	accountsv1 "github.com/gopherbazaar/ledger/gen/ledger/accounts/v1"
	"github.com/gopherbazaar/ledger/internal/accounts"
	"github.com/gopherbazaar/ledger/internal/dbx"
	"github.com/gopherbazaar/ledger/internal/grpcx"
	"github.com/gopherbazaar/ledger/internal/logx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	addr := flag.String("addr", ":9001", "gRPC listen address")
	dsn := flag.String("db-dsn", os.Getenv("ACCOUNTS_DB_DSN"), "Postgres DSN")
	maxConns := flag.Int("db-max-conns", 25, "maximum open DB connections")
	flag.Parse()

	logger := logx.New(os.Stdout, "accounts")

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := dbx.Open(*dsn, *maxConns)
	if err != nil {
		logger.Error("open db", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	srv := grpc.NewServer(grpc.ChainUnaryInterceptor(grpcx.RequestID()))
	accountsv1.RegisterAccountsServiceServer(srv, accounts.NewGRPCServer(accounts.NewStore(pool), logger))
	// Reflection lets tools like grpcurl discover the API at runtime.
	reflection.Register(srv)

	if err := grpcx.Serve(ctx, logger, srv, *addr); err != nil {
		logger.Error("server", "error", err)
		os.Exit(1)
	}
}
