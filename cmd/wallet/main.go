package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gopherbazaar/ledger/internal/wallet"
	"github.com/jackc/pgx/v5/pgxpool"
)

type config struct {
	addr       string
	dsn        string
	dbMaxConns int
}

type application struct {
	logger *slog.Logger
	wallet *wallet.Store
}

func main() {
	var cfg config
	flag.StringVar(&cfg.addr, "port", ":4000", "HTTP listen address")
	flag.StringVar(&cfg.dsn, "db-dsn", os.Getenv("LEDGER_DB_DSN"), "Postgres DSN")
	flag.IntVar(&cfg.dbMaxConns, "db-max-conns", 25, "maximum open DB connections")
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	if err := run(logger, cfg); err != nil {
		logger.Error("fatal", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger, cfg config) error {
	// ctx is cancelled when we receive SIGINT (Ctrl+C) or SIGTERM
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	db, err := openDB(cfg)
	if err != nil {
		logger.Error("open db", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	logger.Info("database connected", "max_conns", cfg.dbMaxConns)

	app := &application{
		logger: logger,
		wallet: wallet.NewStore(db),
	}

	srv := &http.Server{
		Addr:         cfg.addr,
		Handler:      app.routes(),
		IdleTimeout:  time.Minute,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		ErrorLog:     slog.NewLogLogger(logger.Handler(), slog.LevelError),
	}

	srvErr := make(chan error, 1)
	go func() {
		logger.Info("starting server", "addr", cfg.addr)
		srvErr <- srv.ListenAndServe()
	}()

	select {
	case err := <-srvErr:
		return err
	case <-ctx.Done():
		logger.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shutdown: %w", err)
		}
		if err := <-srvErr; !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		logger.Info("stopped")
		return nil
	}
}

func openDB(cfg config) (*pgxpool.Pool, error) {
	pcfg, err := pgxpool.ParseConfig(cfg.dsn)
	if err != nil {
		return nil, fmt.Errorf("parse dsn: %w", err)
	}
	pcfg.MaxConns = int32(cfg.dbMaxConns)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(ctx, pcfg)
	if err != nil {
		return nil, err
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}

	return pool, nil
}
