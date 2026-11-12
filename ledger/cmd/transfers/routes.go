package main

import (
	"net/http"

	"github.com/gopherbazaar/ledger/internal/httpx"
)

func (app *application) routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /v1/transfers", app.createTransfer)
	mux.HandleFunc("GET /v1/transfers/{id}", app.getTransfer)
	mux.HandleFunc("GET /v1/stats/transfers", app.stats)

	return httpx.LogRequests(app.logger, mux)
}
