package main

import (
	"net/http"

	transfersv1 "github.com/gopherbazaar/ledger/gen/ledger/transfers/v1"
	"github.com/gopherbazaar/ledger/internal/httpx"
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

	resp, err := app.transfers.CreateTransfer(r.Context(), &transfersv1.CreateTransferRequest{
		FromAccountId: input.FromAccountID,
		ToAccountId:   input.ToAccountID,
		Amount:        input.Amount,
	})
	if err != nil {
		app.grpcError(w, r, err)
		return
	}

	t := resp.GetTransfer()
	if t.GetStatus() != transfersv1.TransferStatus_TRANSFER_STATUS_COMPLETED {
		httpx.WriteJSON(w, http.StatusAccepted, httpx.Envelope{
			"transfer": transferFromProto(t),
			"message":  "transfer accepted but its outcome is not yet known; poll GET /v1/transfers/{id}",
		})
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, httpx.Envelope{"transfer": transferFromProto(t)})
}

func (app *application) getTransfer(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.IDParam(r, "id")
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "transfer not found")
		return
	}

	resp, err := app.transfers.GetTransfer(r.Context(), &transfersv1.GetTransferRequest{Id: id})
	if err != nil {
		app.grpcError(w, r, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, httpx.Envelope{"transfer": transferFromProto(resp.GetTransfer())})
}

func (app *application) transferStats(w http.ResponseWriter, r *http.Request) {
	resp, err := app.transfers.GetTransferStats(r.Context(), &transfersv1.GetTransferStatsRequest{})
	if err != nil {
		app.grpcError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, httpx.Envelope{
		"by_status": resp.GetByStatus(),
		"in_flight": resp.GetInFlight(),
	})
}
