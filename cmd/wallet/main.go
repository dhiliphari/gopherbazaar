package main

import (
	"log/slog"
	"net/http"
	"os"
)

type application struct {
	logger *slog.Logger
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	app := &application{logger: logger}

	logger.Info("starting server", "addr", ":4000")
	err := http.ListenAndServe(":4000", app.routes())
	logger.Error("server stopped", "error", err)
	os.Exit(1)
}
