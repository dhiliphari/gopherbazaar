package grpcx

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"time"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Serve runs srv on addr until ctx is cancelled, then stops it gracefully:
// no new RPCs are accepted, and in-flight RPCs get up to 10 seconds to finish.
func Serve(ctx context.Context, logger *slog.Logger, srv *grpc.Server, addr string) error {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", addr, err)
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("starting grpc server", "addr", addr, "pid", os.Getpid())
		errCh <- srv.Serve(lis)
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		logger.Info("shutting down")
		stopped := make(chan struct{})
		go func() {
			srv.GracefulStop()
			close(stopped)
		}()
		select {
		case <-stopped:
		case <-time.After(10 * time.Second):
			srv.Stop() // in-flight RPCs took too long; cut them off
		}
		logger.Info("stopped")
		return nil
	}
}

// Error builds a gRPC status error that carries a machine-readable reason
// (for example "INSUFFICIENT_FUNDS") alongside the human-readable message.
func Error(code codes.Code, msg, domain, reason string) error {
	st := status.New(code, msg)
	withInfo, err := st.WithDetails(&errdetails.ErrorInfo{Reason: reason, Domain: domain})
	if err != nil {
		return st.Err() // only fails if the detail can't be marshalled
	}
	return withInfo.Err()
}

// Reason returns the ErrorInfo reason attached to err, or "" if there is none.
func Reason(err error) string {
	for _, d := range status.Convert(err).Details() {
		if info, ok := d.(*errdetails.ErrorInfo); ok {
			return info.GetReason()
		}
	}
	return ""
}
