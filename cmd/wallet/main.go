package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type config struct {
	addr string
	dsn  string
}

type application struct {
	logger *slog.Logger
}

func main() {
	var cfg config
	flag.StringVar(&cfg.addr, "port", ":4000", "HTTP listen address")
	flag.StringVar(&cfg.dsn, "db-dsn", os.Getenv("LEDGER_DB_DSN"), "Postgres DSN")
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	db, err := openDB(cfg.dsn)
	if err != nil {
		logger.Error("open db", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	logger.Info("database connected")

	app := &application{logger: logger}

	logger.Info("starting server", "addr", ":4000")
	err = http.ListenAndServe(cfg.addr, app.routes())
	logger.Error("server stopped", "error", err)
	os.Exit(1)
}

func openDB(dsn string) (*pgxpool.Pool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, err
	}

	err = pool.Ping(ctx)
	if err != nil {
		pool.Close()
		return nil, err
	}

	return pool, nil
}
