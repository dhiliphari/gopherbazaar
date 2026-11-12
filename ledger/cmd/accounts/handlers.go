package main

import (
	"context"
	"errors"
	"net/http"

	"github.com/gopherbazaar/ledger/internal/accounts"
	"github.com/gopherbazaar/ledger/internal/httpx"
)

func (app *application) createAccount(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Owner          string `json:"owner"`
		OpeningDeposit int64  `json:"opening_deposit"`
	}
	if err := httpx.ReadJSON(w, r, &input); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	if input.Owner == "" {
		httpx.Error(w, http.StatusUnprocessableEntity, "owner must be provided")
		return
	}

	acct, err := app.store.CreateAccount(r.Context(), input.Owner, input.OpeningDeposit)
	switch {
	case errors.Is(err, accounts.ErrInvalidAmount):
		httpx.Error(w, http.StatusUnprocessableEntity, err.Error())
	case err != nil:
		httpx.ServerError(app.logger, w, r, err)
	default:
		httpx.WriteJSON(w, http.StatusCreated, httpx.Envelope{"account": acct})
	}
}

func (app *application) getAccount(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.IDParam(r, "id")
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "account not found")
		return
	}

	acct, err := app.store.GetAccount(r.Context(), id)
	switch {
	case errors.Is(err, accounts.ErrNotFound):
		httpx.Error(w, http.StatusNotFound, "account not found")
	case err != nil:
		httpx.ServerError(app.logger, w, r, err)
	default:
		httpx.WriteJSON(w, http.StatusOK, httpx.Envelope{"account": acct})
	}
}

// movementFunc is the shared signature of Store.Debit, Credit and Refund.
type movementFunc func(ctx context.Context, accountID, transferID, amount int64) error

// movement turns a Debit, Credit or Refund method into an HTTP handler.
func (app *application) movement(op movementFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := httpx.IDParam(r, "id")
		if err != nil {
			httpx.Error(w, http.StatusNotFound, "account not found")
			return
		}
		var input struct {
			TransferID int64 `json:"transfer_id"`
			Amount     int64 `json:"amount"`
		}
		if err := httpx.ReadJSON(w, r, &input); err != nil {
			httpx.Error(w, http.StatusBadRequest, err.Error())
			return
		}

		err = op(r.Context(), id, input.TransferID, input.Amount)
		switch {
		case errors.Is(err, accounts.ErrNotFound):
			httpx.Error(w, http.StatusNotFound, err.Error())
		case errors.Is(err, accounts.ErrInvalidAmount):
			httpx.Error(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, accounts.ErrInsufficientFunds):
			httpx.Error(w, http.StatusUnprocessableEntity, err.Error())
		case err != nil:
			httpx.ServerError(app.logger, w, r, err)
		default:
			httpx.WriteJSON(w, http.StatusOK, httpx.Envelope{"status": "applied"})
		}
	}
}

func (app *application) totalBalance(w http.ResponseWriter, r *http.Request) {
	total, err := app.store.TotalBalance(r.Context())
	if err != nil {
		httpx.ServerError(app.logger, w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, httpx.Envelope{"total": total})
}

func (app *application) reconcile(w http.ResponseWriter, r *http.Request) {
	mismatches, err := app.store.Reconcile(r.Context())
	if err != nil {
		httpx.ServerError(app.logger, w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, httpx.Envelope{
		"balanced":   len(mismatches) == 0,
		"mismatches": mismatches,
	})
}
