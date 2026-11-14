// Package reqid carries a request ID through a context, so every log line
// written while handling one request, in any service, can be found together.
package reqid

import (
	"context"
	"crypto/rand"
	"encoding/hex"
)

const (
	// Header is the HTTP header that carries the ID.
	Header = "X-Request-Id"
	// MetadataKey is the gRPC metadata key that carries the ID. gRPC
	// metadata keys are always lower case.
	MetadataKey = "x-request-id"
)

type ctxKey struct{}

// New returns a random 16-character hex ID.
func New() string {
	var b [8]byte
	rand.Read(b[:]) // crypto/rand.Read never returns an error
	return hex.EncodeToString(b[:])
}

// With returns a copy of ctx that carries id.
func With(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ctxKey{}, id)
}

// From returns the ID carried by ctx, or "" if there is none.
func From(ctx context.Context) string {
	id, _ := ctx.Value(ctxKey{}).(string)
	return id
}
