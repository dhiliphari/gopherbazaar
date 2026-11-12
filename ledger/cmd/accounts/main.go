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

	"github.com/gopherbazaar/ledger/internal/accounts"
	"github.com/gopherbazaar/ledger/internal/dbx"
	"github.com/gopherbazaar/ledger/internal/httpx"
)

type application struct {
	logger *slog.Logger
	store  *accounts.Store
}

func main() {
	addr := flag.String("addr", ":4001", "HTTP listen address")
	dsn := flag.String("db-dsn", os.Getenv("ACCOUNTS_DB_DSN"), "Postgres DSN")
	maxConns := flag.Int("db-max-conns", 25, "maximum open DB connections")
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil)).With("service", "accounts")

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := dbx.Open(*dsn, *maxConns)
	if err != nil {
		logger.Error("open db", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	app := &application{logger: logger, store: accounts.NewStore(pool)}

	srv := &http.Server{
		Addr:         *addr,
		Handler:      app.routes(),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  time.Minute,
	}

	if err := httpx.Serve(ctx, logger, srv); err != nil {
		logger.Error("server", "error", err)
		os.Exit(1)
	}
}
