package main

import (
	"net/http"

	"github.com/gopherbazaar/ledger/internal/httpx"
)

func (app *application) routes() http.Handler {
	mux := http.NewServeMux()

	// accounts
	mux.HandleFunc("POST /v1/accounts", app.createAccount)
	mux.HandleFunc("GET /v1/accounts/{id}", app.getAccount)
	mux.HandleFunc("GET /v1/stats/total", app.totalBalance)
	mux.HandleFunc("GET /v1/stats/reconcile", app.reconcile)

	// transfers
	mux.HandleFunc("POST /v1/transfers", app.createTransfer)
	mux.HandleFunc("GET /v1/transfers/{id}", app.getTransfer)
	mux.HandleFunc("GET /v1/stats/transfers", app.transferStats)

	return httpx.RequestID(httpx.LogRequests(app.logger, httpx.Timeout(app.requestTimeout, mux)))
}
