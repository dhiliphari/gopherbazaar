package main

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gopherbazaar/ledger/internal/wallet"
)

func (app *application) healthz(w http.ResponseWriter, r *http.Request) {
	app.writeJSON(w, http.StatusOK, envelope{"status": "ok"})
}

func (app *application) createAccount(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Owner          string `json:"owner"`
		OpeningDeposit int64  `json:"opening_deposit"`
	}

	if err := app.readJSON(w, r, &input); err != nil {
		app.errorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	if strings.TrimSpace(input.Owner) == "" {
		app.errorResponse(w, http.StatusBadRequest, "owner must be provided")
		return
	}

	acct, err := app.wallet.CreateAccount(r.Context(), input.Owner, input.OpeningDeposit)
	switch {
	case errors.Is(err, wallet.ErrInvalidAmount):
		app.errorResponse(w, http.StatusUnprocessableEntity, err.Error())
	case err != nil:
		app.serverError(w, r, err)
	default:
		app.writeJSON(w, http.StatusCreated, envelope{"account": acct})
	}
}

func (app *application) getAccount(w http.ResponseWriter, r *http.Request) {
	id, err := app.readIDParam(r)
	if err != nil {
		app.errorResponse(w, http.StatusNotFound, "account not found")
		return
	}

	acct, err := app.wallet.GetAccount(r.Context(), id)
	switch {
	case errors.Is(err, wallet.ErrNotFound):
		app.errorResponse(w, http.StatusNotFound, "account not found")
	case err != nil:
		app.serverError(w, r, err)
	default:
		app.writeJSON(w, http.StatusOK, envelope{"account": acct})
	}
}
