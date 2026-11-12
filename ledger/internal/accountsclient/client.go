// Package accountsclient lets the transfers service call the accounts service.
package accountsclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gopherbazaar/ledger/internal/transfers"
)

// Client implements transfers.AccountsClient over HTTP.
type Client struct {
	base string
	http *http.Client
}

// This line fails to compile if Client stops satisfying the interface.
var _ transfers.AccountsClient = (*Client)(nil)

func New(baseURL string) *Client {
	return &Client{base: baseURL, http: &http.Client{}}
}

func (c *Client) GetAccount(ctx context.Context, id int64) (transfers.AccountInfo, error) {
	var out struct {
		Account struct {
			ID      int64  `json:"id"`
			Owner   string `json:"owner"`
			Balance int64  `json:"balance"`
		} `json:"account"`
	}
	if err := c.do(ctx, http.MethodGet, fmt.Sprintf("/v1/accounts/%d", id), nil, &out); err != nil {
		return transfers.AccountInfo{}, err
	}
	return transfers.AccountInfo{
		ID:      out.Account.ID,
		Owner:   out.Account.Owner,
		Balance: out.Account.Balance,
	}, nil
}

func (c *Client) Debit(ctx context.Context, accountID, transferID, amount int64) error {
	return c.movement(ctx, "debit", accountID, transferID, amount)
}

func (c *Client) Credit(ctx context.Context, accountID, transferID, amount int64) error {
	return c.movement(ctx, "credit", accountID, transferID, amount)
}

func (c *Client) Refund(ctx context.Context, accountID, transferID, amount int64) error {
	return c.movement(ctx, "refund", accountID, transferID, amount)
}

func (c *Client) movement(ctx context.Context, op string, accountID, transferID, amount int64) error {
	in := map[string]int64{"transfer_id": transferID, "amount": amount}
	path := fmt.Sprintf("/internal/v1/accounts/%d/%s", accountID, op)
	return c.do(ctx, http.MethodPost, path, in, nil)
}

// do sends a JSON request and translates the response status into the
// errors the transfers package understands.
func (c *Client) do(ctx context.Context, method, path string, in, out any) error {
	var body bytes.Buffer
	if in != nil {
		if err := json.NewEncoder(&body).Encode(in); err != nil {
			return err
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, &body)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("accounts %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusNotFound:
		return transfers.ErrAccountNotFound
	case resp.StatusCode == http.StatusUnprocessableEntity:
		return transfers.ErrInsufficientFunds
	case resp.StatusCode >= 300:
		return fmt.Errorf("accounts %s %s: unexpected status %d", method, path, resp.StatusCode)
	}
	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}
