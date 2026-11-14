// Package accountsclient lets the transfers service call the accounts service.
package accountsclient

import (
	"context"

	accountsv1 "github.com/gopherbazaar/ledger/gen/ledger/accounts/v1"
	"github.com/gopherbazaar/ledger/internal/grpcx"
	"github.com/gopherbazaar/ledger/internal/transfers"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Client implements transfers.AccountsClient over gRPC.
type Client struct {
	rpc accountsv1.AccountsServiceClient
}

// This line fails to compile if Client stops satisfying the interface.
var _ transfers.AccountsClient = (*Client)(nil)

func New(rpc accountsv1.AccountsServiceClient) *Client {
	return &Client{rpc: rpc}
}

func (c *Client) GetAccount(ctx context.Context, id int64) (transfers.AccountInfo, error) {
	res, err := c.rpc.GetAccount(ctx, &accountsv1.GetAccountRequest{Id: id})
	if err != nil {
		return transfers.AccountInfo{}, mapErr(err)
	}
	a := res.GetAccount()
	return transfers.AccountInfo{ID: a.GetId(), Owner: a.GetOwner(), Balance: a.GetBalance()}, nil
}

func (c *Client) Debit(ctx context.Context, accountID, transferID, amount int64) error {
	_, err := c.rpc.Debit(ctx, &accountsv1.DebitRequest{
		AccountId:  accountID,
		TransferId: transferID,
		Amount:     amount,
	})
	return mapErr(err)
}

func (c *Client) Credit(ctx context.Context, accountID, transferID, amount int64) error {
	_, err := c.rpc.Credit(ctx, &accountsv1.CreditRequest{
		AccountId:  accountID,
		TransferId: transferID,
		Amount:     amount,
	})
	return mapErr(err)
}

func (c *Client) Refund(ctx context.Context, accountID, transferID, amount int64) error {
	_, err := c.rpc.Refund(ctx, &accountsv1.RefundRequest{
		AccountId:  accountID,
		TransferId: transferID,
		Amount:     amount,
	})
	return mapErr(err)
}

// mapErr translates gRPC status errors into the errors the transfers package
// understands. Anything it doesn't recognise is returned unchanged, which the
// transfers service treats as "outcome unknown".
func mapErr(err error) error {
	if err == nil {
		return nil
	}
	code := status.Code(err)
	switch {
	case code == codes.NotFound:
		return transfers.ErrAccountNotFound
	case code == codes.FailedPrecondition && grpcx.Reason(err) == "INSUFFICIENT_FUNDS":
		return transfers.ErrInsufficientFunds
	}
	return err
}
