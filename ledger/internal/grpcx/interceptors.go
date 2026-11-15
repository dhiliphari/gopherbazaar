package grpcx

import (
	"context"
	"log/slog"
	"runtime/debug"
	"time"

	"github.com/gopherbazaar/ledger/internal/reqid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
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

// Logging logs one line per RPC: method, status code, duration, and how much
// of the caller's deadline was left when the RPC arrived.
func Logging(logger *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()
		attrs := []slog.Attr{slog.String("method", info.FullMethod)}
		if deadline, ok := ctx.Deadline(); ok {
			attrs = append(attrs, slog.Int64("deadline_ms", int64(time.Until(deadline).Milliseconds())))
		}

		resp, err := handler(ctx, req)

		code := status.Code(err)
		attrs = append(attrs,
			slog.String("code", code.String()),
			slog.Int64("duration_ms", int64(time.Since(start).Milliseconds())),
		)

		level := slog.LevelInfo
		if isServerFault(code) {
			level = slog.LevelError
		}

		logger.LogAttrs(ctx, level, "rpc", attrs...)
		return resp, err
	}
}

// isServerFault reports whether code means "the server got it wrong", as
// opposed to "the caller asked for something that can't be done".
func isServerFault(code codes.Code) bool {
	switch code {
	case codes.Internal, codes.Unknown, codes.DataLoss, codes.Unimplemented:
		return true
	}
	return false
}

// Recovery turns a panic in a handler into an Internal error, instead of
// letting it crash the whole process (and every other in-flight RPC).
func Recovery(logger *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		defer func() {
			if p := recover(); p != nil {
				logger.ErrorContext(ctx, "panic recovered",
					"method", info.FullMethod,
					"panic", p,
					"stack", string(debug.Stack()),
				)
				resp, err = nil, status.Error(codes.Internal, "internal error")
			}
		}()

		return handler(ctx, req)
	}
}

// ServerInterceptors returns the interceptor chain every server uses. The
// first interceptor is the outermost: it runs first on the way in and last
// on the way out.
//
//	RequestID → Logging → Recovery → handler
//
// Recovery sits inside Logging so that a recovered panic is logged as an
// Internal error like any other, with the request ID attached.
func ServerInterceptors(logger *slog.Logger) grpc.ServerOption {
	return grpc.ChainUnaryInterceptor(
		RequestID(),
		Logging(logger),
		Recovery(logger),
	)
}
