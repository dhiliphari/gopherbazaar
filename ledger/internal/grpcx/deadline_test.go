package grpcx_test

import (
	"context"
	"testing"
	"time"

	"github.com/gopherbazaar/ledger/internal/grpcx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestDeadline(t *testing.T) {
	const limit = time.Second

	tests := []struct {
		name    string
		ctx     func() (context.Context, context.CancelFunc)
		wantMax time.Duration // the handler must see a deadline no later than this
	}{
		{
			name:    "no deadline gets the limit",
			ctx:     func() (context.Context, context.CancelFunc) { return context.WithCancel(context.Background()) },
			wantMax: limit,
		},
		{
			name: "long deadline is capped",
			ctx: func() (context.Context, context.CancelFunc) {
				return context.WithTimeout(context.Background(), time.Hour)
			},
			wantMax: limit,
		},
		{
			name: "short deadline is kept",
			ctx: func() (context.Context, context.CancelFunc) {
				return context.WithTimeout(context.Background(), 100*time.Millisecond)
			},
			wantMax: 100 * time.Millisecond,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := tt.ctx()
			defer cancel()

			var remaining time.Duration
			_, err := grpcx.Deadline(limit)(ctx, nil, info, func(ctx context.Context, req any) (any, error) {
				deadline, ok := ctx.Deadline()
				if !ok {
					t.Fatal("handler ran without a deadline")
				}
				remaining = time.Until(deadline)
				return nil, nil
			})
			if err != nil {
				t.Fatal(err)
			}
			if remaining > tt.wantMax {
				t.Errorf("handler had %v left, want at most %v", remaining, tt.wantMax)
			}
		})
	}
}

func TestDeadlineRejectsExpiredContext(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancel()
	time.Sleep(time.Millisecond)

	called := false
	_, err := grpcx.Deadline(time.Second)(ctx, nil, info, func(ctx context.Context, req any) (any, error) {
		called = true
		return nil, nil
	})
	if called {
		t.Error("handler ran even though the deadline had already passed")
	}
	if code := status.Code(err); code != codes.DeadlineExceeded {
		t.Errorf("code = %v, want DeadlineExceeded", code)
	}
}

func TestCallTimeoutNeverExtends(t *testing.T) {
	invoker := func(want time.Duration) grpc.UnaryInvoker {
		return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
			deadline, ok := ctx.Deadline()
			if !ok {
				t.Fatal("outgoing call has no deadline")
			}
			if left := time.Until(deadline); left > want {
				t.Errorf("outgoing call has %v, want at most %v", left, want)
			}
			return nil
		}
	}

	// No caller deadline: the call gets the per-call timeout.
	_ = grpcx.CallTimeout(time.Second)(context.Background(), "/x/Y", nil, nil, nil, invoker(time.Second))

	// A sooner caller deadline wins over the per-call timeout.
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_ = grpcx.CallTimeout(time.Second)(ctx, "/x/Y", nil, nil, nil, invoker(50*time.Millisecond))
}
