package httpx_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gopherbazaar/ledger/internal/httpx"
	"github.com/gopherbazaar/ledger/internal/reqid"
)

func TestRequestID(t *testing.T) {
	tests := []struct {
		name     string
		incoming string
		wantSame bool
	}{
		{"reuses caller's id", "abc123", true},
		{"generates when missing", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var seen string
			h := httpx.RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				seen = reqid.From(r.Context())
			}))

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.incoming != "" {
				req.Header.Set(reqid.Header, tt.incoming)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if seen == "" {
				t.Fatal("handler saw no request id")
			}
			if tt.wantSame && seen != tt.incoming {
				t.Errorf("id = %q, want %q", seen, tt.incoming)
			}
			if got := rec.Header().Get(reqid.Header); got != seen {
				t.Errorf("response header %q, want %q", got, seen)
			}
		})
	}
}
