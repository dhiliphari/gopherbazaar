package httpx_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gopherbazaar/ledger/internal/httpx"
)

func TestTimeoutSetDeadline(t *testing.T) {
	var remaining time.Duration
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		deadline, ok := r.Context().Deadline()
		if !ok {
			t.Fatal("request context has no deadline")
		}
		remaining = time.Until(deadline)
	})

	httpx.Timeout(2*time.Second, next).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	if remaining <= 0 || remaining > 2*time.Second {
		t.Errorf("remaining = %v, want between 0 and 2s", remaining)
	}
}
