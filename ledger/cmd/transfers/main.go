package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	accountsv1 "github.com/gopherbazaar/ledger/gen/ledger/accounts/v1"
	transfersv1 "github.com/gopherbazaar/ledger/gen/ledger/transfers/v1"
	"github.com/gopherbazaar/ledger/internal/accountsclient"
	"github.com/gopherbazaar/ledger/internal/dbx"
	"github.com/gopherbazaar/ledger/internal/grpcx"
	"github.com/gopherbazaar/ledger/internal/transfers"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"
)

func main() {
	addr := flag.String("addr", ":9002", "gRPC listen address")
	dsn := flag.String("db-dsn", os.Getenv("TRANSFERS_DB_DSN"), "Postgres DSN")
	maxConns := flag.Int("db-max-conns", 25, "maximum open DB connections")
	accountsURL := flag.String("accounts-url", "localhost:9001", "accounts service base URL")
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil)).With("service", "transfers")

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := dbx.Open(*dsn, *maxConns)
	if err != nil {
		logger.Error("open db", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	conn, err := grpc.NewClient(*accountsURL, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logger.Error("accounts client", "error", err)
		os.Exit(1)
	}
	defer conn.Close()

	store := transfers.NewStore(pool)
	accountsClient := accountsclient.New(accountsv1.NewAccountsServiceClient(conn))
	service := transfers.NewService(store, accountsClient, logger)

	srv := grpc.NewServer()
	transfersv1.RegisterTransfersServiceServer(srv, transfers.NewGRPCServer(service, store, logger))
	reflection.Register(srv)

	if err := grpcx.Serve(ctx, logger, srv, *addr); err != nil {
		logger.Error("server", "error", err)
		os.Exit(1)
	}
}
