package logx_test

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/gopherbazaar/ledger/internal/logx"
	"github.com/gopherbazaar/ledger/internal/reqid"
)

func TestRequestIDIsAdded(t *testing.T) {
	var buf bytes.Buffer
	// logger.With(...) must keep the context handler; this is the case that
	// breaks if WithAttrs isn't overridden.
	logger := logx.New(&buf, "test").With("component", "store")

	logger.InfoContext(reqid.With(context.Background(), "abc123"), "hello")

	var line map[string]any
	if err := json.Unmarshal(buf.Bytes(), &line); err != nil {
		t.Fatalf("decode: %v (%q)", err, buf.String())
	}
	for key, want := range map[string]string{
		"request_id": "abc123",
		"service":    "test",
		"component":  "store",
	} {
		if line[key] != want {
			t.Errorf("%s = %v, want %q", key, line[key], want)
		}
	}
}

func TestNoRequestIDWithoutContext(t *testing.T) {
	var buf bytes.Buffer
	logx.New(&buf, "test").Info("hello")
	if bytes.Contains(buf.Bytes(), []byte("request_id")) {
		t.Errorf("unexpected request_id in %s", buf.String())
	}
}
