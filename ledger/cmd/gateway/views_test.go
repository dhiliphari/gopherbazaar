package main

import (
	"net/http"
	"testing"

	"google.golang.org/grpc/codes"

	transfersv1 "github.com/gopherbazaar/ledger/gen/ledger/transfers/v1"
)

func TestHTTPStatus(t *testing.T) {
	tests := []struct {
		code codes.Code
		want int
	}{
		{codes.InvalidArgument, http.StatusUnprocessableEntity},
		{codes.FailedPrecondition, http.StatusUnprocessableEntity},
		{codes.NotFound, http.StatusNotFound},
		{codes.Unavailable, http.StatusServiceUnavailable},
		{codes.DeadlineExceeded, http.StatusGatewayTimeout},
		{codes.Internal, http.StatusInternalServerError},
		{codes.Unknown, http.StatusInternalServerError},
	}
	for _, tt := range tests {
		t.Run(tt.code.String(), func(t *testing.T) {
			if got := httpStatus(tt.code); got != tt.want {
				t.Errorf("httpStatus(%v) = %d, want %d", tt.code, got, tt.want)
			}
		})
	}
}

func TestStatusName(t *testing.T) {
	if got := statusName(transfersv1.TransferStatus_TRANSFER_STATUS_COMPLETED); got != "completed" {
		t.Errorf("got %q, want %q", got, "completed")
	}
}
