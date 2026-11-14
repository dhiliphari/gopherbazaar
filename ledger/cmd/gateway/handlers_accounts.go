package main

import (
	"net/http"

	accountsv1 "github.com/gopherbazaar/ledger/gen/ledger/accounts/v1"
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

	resp, err := app.accounts.CreateAccount(r.Context(), &accountsv1.CreateAccountRequest{
		Owner:          input.Owner,
		OpeningDeposit: input.OpeningDeposit,
	})
	if err != nil {
		app.grpcError(w, r, err)
		return
	}

	httpx.WriteJSON(w, http.StatusCreated, httpx.Envelope{"account": accountFromProto(resp.GetAccount())})
}

func (app *application) getAccount(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.IDParam(r, "id")
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "account not found")
		return
	}

	resp, err := app.accounts.GetAccount(r.Context(), &accountsv1.GetAccountRequest{Id: id})
	if err != nil {
		app.grpcError(w, r, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, httpx.Envelope{"account": accountFromProto(resp.GetAccount())})
}

func (app *application) totalBalance(w http.ResponseWriter, r *http.Request) {
	resp, err := app.accounts.GetTotalBalance(r.Context(), &accountsv1.GetTotalBalanceRequest{})
	if err != nil {
		app.grpcError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, httpx.Envelope{"total": resp.GetTotal()})
}

func (app *application) reconcile(w http.ResponseWriter, r *http.Request) {
	resp, err := app.accounts.Reconcile(r.Context(), &accountsv1.ReconcileRequest{})
	if err != nil {
		app.grpcError(w, r, err)
		return
	}

	type mismatch struct {
		AccountID     int64 `json:"account_id"`
		Balance       int64 `json:"balance"`
		LedgerBalance int64 `json:"ledger_balance"`
	}

	mismatches := []mismatch{} // encode as [] rather than null when empty
	for _, m := range resp.GetMismatches() {
		mismatches = append(mismatches, mismatch{m.GetAccountId(), m.GetBalance(), m.GetLedgerBalance()})
	}

	httpx.WriteJSON(w, http.StatusOK, httpx.Envelope{
		"balanced":   len(mismatches) == 0,
		"mismatches": mismatches,
	})
}
