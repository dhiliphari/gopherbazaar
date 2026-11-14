package grpcx

import (
	"context"

	"github.com/gopherbazaar/ledger/internal/reqid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// RequestID takes the request ID from the incoming metadata (or makes a new
// one if the caller didn't send any) and puts it in the context.
func RequestID() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		id := ""
		if md, ok := metadata.FromIncomingContext(ctx); ok {
			if vals := md.Get(reqid.MetadataKey); len(vals) > 0 {
				id = vals[0]
			}
		}
		if id == "" {
			id = reqid.New()
		}
		return handler(reqid.With(ctx, id), req)
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
