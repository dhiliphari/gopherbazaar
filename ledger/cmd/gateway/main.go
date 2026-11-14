package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	accountsv1 "github.com/gopherbazaar/ledger/gen/ledger/accounts/v1"
	transfersv1 "github.com/gopherbazaar/ledger/gen/ledger/transfers/v1"
	"github.com/gopherbazaar/ledger/internal/httpx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type application struct {
	logger    *slog.Logger
	accounts  accountsv1.AccountsServiceClient
	transfers transfersv1.TransfersServiceClient
}

func main() {
	addr := flag.String("addr", ":4000", "HTTP listen address")
	accountsAddr := flag.String("accounts-addr", "localhost:9001", "accounts service base URL")
	transfersAddr := flag.String("transfers-addr", "localhost:9002", "transfers service base URL")
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil)).With("service", "gateway")

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	accountsConn, err := grpc.NewClient(*accountsAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logger.Error("accounts client", "error", err)
		os.Exit(1)
	}
	defer accountsConn.Close()

	transferConn, err := grpc.NewClient(*transfersAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logger.Error("transfers client", "error", err)
		os.Exit(1)
	}
	defer transferConn.Close()

	app := &application{
		logger:    logger,
		accounts:  accountsv1.NewAccountsServiceClient(accountsConn),
		transfers: transfersv1.NewTransfersServiceClient(transferConn),
	}

	srv := &http.Server{
		Addr:         *addr,
		Handler:      app.routes(),
		IdleTimeout:  time.Minute,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		ErrorLog:     slog.NewLogLogger(logger.Handler(), slog.LevelError),
	}

	if err := httpx.Serve(ctx, logger, srv); err != nil {
		logger.Error("server", "error", err)
		os.Exit(1)
	}
}
