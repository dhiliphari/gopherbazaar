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
	"github.com/gopherbazaar/ledger/internal/debugx"
	"github.com/gopherbazaar/ledger/internal/grpcx"
	"github.com/gopherbazaar/ledger/internal/httpx"
	"github.com/gopherbazaar/ledger/internal/logx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	addr := flag.String("addr", ":9001", "gRPC listen address")
	dsn := flag.String("db-dsn", os.Getenv("ACCOUNTS_DB_DSN"), "Postgres DSN")
	maxConns := flag.Int("db-max-conns", 25, "maximum open DB connections")
	debugAddr := flag.String("debug-addr", "localhost:6001", "debug (expvar, pprof) listen address")
	flag.Parse()

	logger := logx.New(os.Stdout, "accounts")

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		if err := httpx.Serve(ctx, logger, debugx.NewServer(*debugAddr)); err != nil {
			logger.Error("debug server", "error", err)
		}
	}()

	pool, err := dbx.Open(*dsn, *maxConns)
	if err != nil {
		logger.Error("open db", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	srv := grpc.NewServer(grpcx.ServerInterceptors(logger))
	accountsv1.RegisterAccountsServiceServer(srv, accounts.NewGRPCServer(accounts.NewStore(pool), logger))
	// Reflection lets tools like grpcurl discover the API at runtime.
	reflection.Register(srv)

	if err := grpcx.Serve(ctx, logger, srv, *addr); err != nil {
		logger.Error("server", "error", err)
		os.Exit(1)
	}
}
