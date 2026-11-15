package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"

	accountsv1 "github.com/gopherbazaar/ledger/gen/ledger/accounts/v1"
	transfersv1 "github.com/gopherbazaar/ledger/gen/ledger/transfers/v1"
	"github.com/gopherbazaar/ledger/internal/accountsclient"
	"github.com/gopherbazaar/ledger/internal/dbx"
	"github.com/gopherbazaar/ledger/internal/debugx"
	"github.com/gopherbazaar/ledger/internal/grpcx"
	"github.com/gopherbazaar/ledger/internal/httpx"
	"github.com/gopherbazaar/ledger/internal/logx"
	"github.com/gopherbazaar/ledger/internal/transfers"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"
)

func main() {
	addr := flag.String("addr", ":9002", "gRPC listen address")
	dsn := flag.String("db-dsn", os.Getenv("TRANSFERS_DB_DSN"), "Postgres DSN")
	maxConns := flag.Int("db-max-conns", 25, "maximum open DB connections")
	accountsURL := flag.String("accounts-addr", "localhost:9001", "accounts service base URL")
	debugAddr := flag.String("debug-addr", "localhost:6002", "debug (expvar, pprof) listen address")
	accountsTimeout := flag.Duration("accounts-timeout", time.Second, "timeout for each call to accounts")
	flag.Parse()

	logger := logx.New(os.Stdout, "transfers")

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

	conn, err := grpc.NewClient(*accountsURL,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithChainUnaryInterceptor(
			grpcx.PropagateRequestID(),
			grpcx.CallTimeout(*accountsTimeout),
		),
	)
	if err != nil {
		logger.Error("accounts client", "error", err)
		os.Exit(1)
	}
	defer conn.Close()

	store := transfers.NewStore(pool)
	accountsClient := accountsclient.New(accountsv1.NewAccountsServiceClient(conn))
	service := transfers.NewService(store, accountsClient, logger)

	srv := grpc.NewServer(grpcx.ServerInterceptors(logger))
	transfersv1.RegisterTransfersServiceServer(srv, transfers.NewGRPCServer(service, store, logger))
	reflection.Register(srv)

	if err := grpcx.Serve(ctx, logger, srv, *addr); err != nil {
		logger.Error("server", "error", err)
		os.Exit(1)
	}
}
