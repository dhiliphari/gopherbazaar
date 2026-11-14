package main

import (
	"net/http"
	"strings"
	"time"

	accountsv1 "github.com/gopherbazaar/ledger/gen/ledger/accounts/v1"
	transfersv1 "github.com/gopherbazaar/ledger/gen/ledger/transfers/v1"
	"github.com/gopherbazaar/ledger/internal/httpx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type accountView struct {
	ID        int64     `json:"id"`
	Owner     string    `json:"owner"`
	Balance   int64     `json:"balance"`
	Currency  string    `json:"currency"`
	CreatedAt time.Time `json:"created_at"`
}

func accountFromProto(a *accountsv1.Account) accountView {
	return accountView{
		ID:        a.GetId(),
		Owner:     a.GetOwner(),
		Balance:   a.GetBalance(),
		Currency:  a.GetCurrency(),
		CreatedAt: a.GetCreateTime().AsTime(),
	}
}

type transferView struct {
	ID            int64     `json:"id"`
	FromAccountID int64     `json:"from_account_id"`
	ToAccountID   int64     `json:"to_account_id"`
	Amount        int64     `json:"amount"`
	Status        string    `json:"status"`
	LastError     string    `json:"last_error,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func transferFromProto(t *transfersv1.Transfer) transferView {
	return transferView{
		ID:            t.GetId(),
		FromAccountID: t.GetFromAccountId(),
		ToAccountID:   t.GetToAccountId(),
		Amount:        t.GetAmount(),
		Status:        statusName(t.GetStatus()),
		LastError:     t.GetLastError(),
		CreatedAt:     t.GetCreateTime().AsTime(),
		UpdatedAt:     t.GetUpdateTime().AsTime(),
	}
}

// statusName turns TRANSFER_STATUS_COMPLETED into "completed".
func statusName(s transfersv1.TransferStatus) string {
	return strings.ToLower(strings.TrimPrefix(s.String(), "TRANSFER_STATUS_"))
}

// httpStatus maps a gRPC status code to the closest HTTP status code.
func httpStatus(code codes.Code) int {
	switch code {
	case codes.InvalidArgument, codes.FailedPrecondition, codes.OutOfRange:
		return http.StatusUnprocessableEntity
	case codes.NotFound:
		return http.StatusNotFound
	case codes.AlreadyExists, codes.Aborted:
		return http.StatusConflict
	case codes.Unauthenticated:
		return http.StatusUnauthorized
	case codes.PermissionDenied:
		return http.StatusForbidden
	case codes.ResourceExhausted:
		return http.StatusTooManyRequests
	case codes.Unavailable:
		return http.StatusServiceUnavailable
	case codes.DeadlineExceeded:
		return http.StatusGatewayTimeout
	case codes.Canceled:
		return 499 // client closed request (nginx convention)
	default:
		return http.StatusInternalServerError
	}
}

// grpcError writes an HTTP error response for an error returned by a backend.
func (app *application) grpcError(w http.ResponseWriter, r *http.Request, err error) {
	st := status.Convert(err)
	code := httpStatus(st.Code())
	if code == http.StatusInternalServerError {
		httpx.ServerError(app.logger, w, r, err)
		return
	}
	httpx.Error(w, code, st.Message())
}
