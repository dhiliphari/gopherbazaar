package grpcx

import (
	"context"
	"time"

	"github.com/gopherbazaar/ledger/internal/reqid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// CallTimeout gives each outgoing RPC at most d. If the caller's context
// already has a sooner deadline, that one wins: a timeout can only ever
// shorten the time available, never extend it.
func CallTimeout(d time.Duration) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		ctx, cancel := context.WithTimeout(ctx, d)
		defer cancel()
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

// PropagateRequestID copies the request ID from the context into outgoing
// metadata, so the next service in the chain logs the same ID.
func PropagateRequestID() grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		if id := reqid.From(ctx); id != "" {
			ctx = metadata.AppendToOutgoingContext(ctx, reqid.MetadataKey, id)
		}
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}
