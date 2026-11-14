package logx

import (
	"context"
	"io"
	"log/slog"

	"github.com/gopherbazaar/ledger/internal/reqid"
)

func New(w io.Writer, service string) *slog.Logger {
	h := ContextHandler{Handler: slog.NewJSONHandler(w, nil)}
	return slog.New(h).With("service", service)
}

// ContextHandler wraps another slog.Handler and copies request-scoped values
// from the context into each record.
type ContextHandler struct {
	slog.Handler
}

func (h ContextHandler) Handle(ctx context.Context, r slog.Record) error {
	if id := reqid.From(ctx); id != "" {
		r.AddAttrs(slog.String("request_id", id))
	}
	return h.Handler.Handle(ctx, r)
}

// WithAttrs and WithGroup must be overridden too. Otherwise the embedded
// handler's versions would return an unwrapped handler, and loggers created
// with logger.With(...) would silently lose the request ID.
func (h ContextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return ContextHandler{Handler: h.Handler.WithAttrs(attrs)}
}

func (h ContextHandler) WithGroup(name string) slog.Handler {
	return ContextHandler{Handler: h.Handler.WithGroup(name)}
}
