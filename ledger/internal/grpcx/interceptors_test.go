package grpcx_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/gopherbazaar/ledger/internal/grpcx"
	"github.com/gopherbazaar/ledger/internal/logx"
	"github.com/gopherbazaar/ledger/internal/reqid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

var info = &grpc.UnaryServerInfo{FullMethod: "/test.v1.TestService/Do"}

func TestRecoveryTurnsPanicIntoInternal(t *testing.T) {
	var logs bytes.Buffer
	interceptor := grpcx.Recovery(logx.New(&logs, "test"))

	resp, err := interceptor(context.Background(), nil, info, func(ctx context.Context, req any) (any, error) {
		panic("test")
		return "unreachable", nil
	})

	if resp != nil {
		t.Errorf("resp = %v, want nil", resp)
	}
	if code := status.Code(err); code != codes.Internal {
		t.Errorf("code = %v, want Internal", code)
	}
	if !strings.Contains(logs.String(), "panic recovered") {
		t.Errorf("panic was not logged: %s", logs.String())
	}
}

func TestRequestIDUsesIncomingMetadata(t *testing.T) {
	ctx := metadata.NewIncomingContext(context.Background(),
		metadata.Pairs(reqid.MetadataKey, "abc123"))

	var seen string
	_, err := grpcx.RequestID()(ctx, nil, info, func(ctx context.Context, req any) (any, error) {
		seen = reqid.From(ctx)
		return nil, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if seen != "abc123" {
		t.Errorf("handler saw request id %q, want %q", seen, "abc123")
	}
}

func TestRequestIDGeneratesWhenMissing(t *testing.T) {
	var seen string
	_, _ = grpcx.RequestID()(context.Background(), nil, info, func(ctx context.Context, req any) (any, error) {
		seen = reqid.From(ctx)
		return nil, nil
	})
	if len(seen) != 16 {
		t.Errorf("generated id %q, want 16 hex characters", seen)
	}
}

func TestPropagateRequestID(t *testing.T) {
	ctx := reqid.With(context.Background(), "abc123")

	var sent []string
	invoker := func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
		md, _ := metadata.FromOutgoingContext(ctx)
		sent = md.Get(reqid.MetadataKey)
		return nil
	}
	if err := grpcx.PropagateRequestID()(ctx, "/x/Y", nil, nil, nil, invoker); err != nil {
		t.Fatal(err)
	}
	if len(sent) != 1 || sent[0] != "abc123" {
		t.Errorf("outgoing metadata %v, want [abc123]", sent)
	}
}

func TestLoggingIncludesRequestIDAndCode(t *testing.T) {
	var logs bytes.Buffer
	logger := logx.New(&logs, "test")
	ctx := reqid.With(context.Background(), "abc123")

	_, _ = grpcx.Logging(logger)(ctx, nil, info, func(ctx context.Context, req any) (any, error) {
		return nil, status.Error(codes.NotFound, "nope")
	})

	var line map[string]any
	if err := json.NewDecoder(io.Reader(&logs)).Decode(&line); err != nil {
		t.Fatalf("decode log line: %v (%q)", err, logs.String())
	}
	if line["request_id"] != "abc123" || line["code"] != "NotFound" || line["method"] != info.FullMethod {
		t.Errorf("unexpected log line: %v", line)
	}
	if line["level"] != "INFO" {
		t.Errorf("level = %v, want INFO (NotFound is the caller's problem, not ours)", line["level"])
	}
}
