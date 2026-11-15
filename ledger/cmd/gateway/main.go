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
	"github.com/gopherbazaar/ledger/internal/debugx"
	"github.com/gopherbazaar/ledger/internal/grpcx"
	"github.com/gopherbazaar/ledger/internal/httpx"
	"github.com/gopherbazaar/ledger/internal/logx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type application struct {
	logger         *slog.Logger
	accounts       accountsv1.AccountsServiceClient
	transfers      transfersv1.TransfersServiceClient
	requestTimeout time.Duration
}

func main() {
	addr := flag.String("addr", ":4000", "HTTP listen address")
	accountsAddr := flag.String("accounts-addr", "localhost:9001", "accounts service base URL")
	transfersAddr := flag.String("transfers-addr", "localhost:9002", "transfers service base URL")
	debugAddr := flag.String("debug-addr", "localhost:6000", "debug (expvar, pprof) listen address")
	requestTimeout := flag.Duration("request-timeout", 3*time.Second, "deadline for each incoming request")
	flag.Parse()

	logger := logx.New(os.Stdout, "gateway")

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		if err := httpx.Serve(ctx, logger, debugx.NewServer(*debugAddr)); err != nil {
			logger.Error("debug server", "error", err)
		}
	}()

	accountsConn, err := dial(*accountsAddr)
	if err != nil {
		logger.Error("accounts client", "error", err)
		os.Exit(1)
	}
	defer accountsConn.Close()

	transferConn, err := dial(*transfersAddr)
	if err != nil {
		logger.Error("transfers client", "error", err)
		os.Exit(1)
	}
	defer transferConn.Close()

	app := &application{
		logger:         logger,
		accounts:       accountsv1.NewAccountsServiceClient(accountsConn),
		transfers:      transfersv1.NewTransfersServiceClient(transferConn),
		requestTimeout: *requestTimeout,
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

func dial(addr string) (*grpc.ClientConn, error) {
	return grpc.NewClient(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithChainUnaryInterceptor(grpcx.PropagateRequestID()),
	)
}
