package main

import "net/http"

func (app *application) routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /v1/healthz", app.healthz)

	mux.HandleFunc("POST /v1/accounts", app.createAccount)
	mux.HandleFunc("GET /v1/accounts/{id}", app.getAccount)

	return mux
}
