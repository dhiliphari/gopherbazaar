package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

type client struct {
	base string
	http *http.Client
}

// do sends a JSON request and decodes a JSON response into out (if out is
// non-nil and the request succeeded). It returns the HTTP status code.
func (c *client) do(method, path string, in, out any) (int, error) {
	var body bytes.Buffer
	if in != nil {
		if err := json.NewEncoder(&body).Encode(in); err != nil {
			return 0, err
		}
	}

	req, err := http.NewRequest(method, c.base+path, &body)
	if err != nil {
		return 0, err
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if out != nil && resp.StatusCode < 300 {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return 0, err
		}
	}

	return resp.StatusCode, nil
}

func (c *client) createAccount(owner string, openingDeposit int64) (int64, error) {
	var out struct {
		Account struct {
			Id int64 `json:"id"`
		} `json:"account"`
	}

	status, err := c.do(http.MethodPost, "/v1/accounts", map[string]any{
		"owner":           owner,
		"opening_deposit": openingDeposit,
	}, &out)
	if err != nil {
		return 0, err
	}
	if status != http.StatusCreated {
		return 0, fmt.Errorf("create account: unexpected status %d", status)
	}

	return out.Account.Id, nil
}

// transfer returns the HTTP status code, or 0 if the request failed outright.
func (c *client) transfer(fromID, toID, amount int64) int {
	in := map[string]any{
		"from_account_id": fromID,
		"to_account_id":   toID,
		"amount":          amount,
	}
	status, err := c.do(http.MethodPost, "/v1/transfers", in, nil)
	if err != nil {
		return 0
	}
	return status
}

func (c *client) total() (int64, error) {
	var out struct {
		Total int64 `json:"total"`
	}
	if _, err := c.do(http.MethodGet, "/v1/stats/total", nil, &out); err != nil {
		return 0, err
	}
	return out.Total, nil
}

func (c *client) balance(id int64) (int64, error) {
	var out struct {
		Account struct {
			Balance int64 `json:"balance"`
		} `json:"account"`
	}
	if _, err := c.do(http.MethodGet, fmt.Sprintf("/v1/accounts/%d", id), nil, &out); err != nil {
		return 0, err
	}
	return out.Account.Balance, nil
}
