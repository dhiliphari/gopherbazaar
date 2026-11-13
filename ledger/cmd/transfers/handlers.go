package main

import (
	"errors"
	"net/http"

	"github.com/gopherbazaar/ledger/internal/httpx"
	"github.com/gopherbazaar/ledger/internal/transfers"
)

func (app *application) createTransfer(w http.ResponseWriter, r *http.Request) {
	var input struct {
		FromAccountID int64 `json:"from_account_id"`
		ToAccountID   int64 `json:"to_account_id"`
		Amount        int64 `json:"amount"`
	}
	if err := httpx.ReadJSON(w, r, &input); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	t, err := app.service.Transfer(r.Context(), input.FromAccountID, input.ToAccountID, input.Amount)
	switch {
	case errors.Is(err, transfers.ErrOutcomeUnknown):
		httpx.WriteJSON(w, http.StatusAccepted, httpx.Envelope{
			"transfer": t,
			"message":  "transfer accepted but its outcome is not yet known; poll GET /v1/transfers/{id}",
		})
	case errors.Is(err, transfers.ErrAccountNotFound):
		httpx.Error(w, http.StatusNotFound, err.Error())
	case errors.Is(err, transfers.ErrInvalidAmount),
		errors.Is(err, transfers.ErrSameAccount),
		errors.Is(err, transfers.ErrInsufficientFunds):
		httpx.Error(w, http.StatusUnprocessableEntity, err.Error())
	case err != nil:
		httpx.ServerError(app.logger, w, r, err)
	default:
		httpx.WriteJSON(w, http.StatusCreated, httpx.Envelope{"transfer": t})
	}
}

func (app *application) getTransfer(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.IDParam(r, "id")
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "transfer not found")
		return
	}

	t, err := app.store.Get(r.Context(), id)
	switch {
	case errors.Is(err, transfers.ErrNotFound):
		httpx.Error(w, http.StatusNotFound, "transfer not found")
	case err != nil:
		httpx.ServerError(app.logger, w, r, err)
	default:
		httpx.WriteJSON(w, http.StatusOK, httpx.Envelope{"transfer": t})
	}
}

func (app *application) stats(w http.ResponseWriter, r *http.Request) {
	st, err := app.store.Stats(r.Context())
	if err != nil {
		httpx.ServerError(app.logger, w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, st)
}
