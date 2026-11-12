package main

import (
	"net/http"

	"github.com/gopherbazaar/ledger/internal/httpx"
)

func (app *application) routes() http.Handler {
	mux := http.NewServeMux()

	// Public API
	mux.HandleFunc("POST /v1/accounts", app.createAccount)
	mux.HandleFunc("GET /v1/accounts/{id}", app.getAccount)
	mux.HandleFunc("GET /v1/stats/total", app.totalBalance)
	mux.HandleFunc("GET /v1/stats/reconcile", app.reconcile)

	// Internal API, called by the transfers service
	mux.HandleFunc("POST /internal/v1/accounts/{id}/debit", app.movement(app.store.Debit))
	mux.HandleFunc("POST /internal/v1/accounts/{id}/credit", app.movement(app.store.Credit))
	mux.HandleFunc("POST /internal/v1/accounts/{id}/refund", app.movement(app.store.Refund))

	return httpx.LogRequests(app.logger, mux)
}
