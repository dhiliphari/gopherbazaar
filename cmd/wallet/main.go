package main

import (
	"log/slog"
	"net/http"
	"os"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	mux := http.NewServeMux()

	mux.HandleFunc("GET /v1/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "application/json")
		w.Write([]byte(`{"status": "ok"}` + "\n"))
	})

	logger.Info("starting server", "addr", ":4000")
	err := http.ListenAndServe(":4000", mux)
	logger.Error("server stopped", "error", err)
	os.Exit(1)
}
