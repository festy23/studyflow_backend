package client

import (
	"context"
	"errors"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestRetryUnaryInterceptor_RetriesUnavailableThenSucceeds(t *testing.T) {
	interceptor := RetryUnaryInterceptor(3, time.Millisecond)

	var calls int
	invoker := func(_ context.Context, _ string, _, _ any, _ *grpc.ClientConn, _ ...grpc.CallOption) error {
		calls++
		if calls < 3 {
			return status.Error(codes.Unavailable, "unavailable")
		}
		return nil
	}

	err := interceptor(context.Background(), "/svc/Method", nil, nil, nil, invoker)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if calls != 3 {
		t.Fatalf("expected 3 invoker calls, got %d", calls)
	}
}

func TestRetryUnaryInterceptor_DoesNotRetryNonRetriable(t *testing.T) {
	interceptor := RetryUnaryInterceptor(3, time.Millisecond)

	var calls int
	wantErr := status.Error(codes.Internal, "boom")
	invoker := func(_ context.Context, _ string, _, _ any, _ *grpc.ClientConn, _ ...grpc.CallOption) error {
		calls++
		return wantErr
	}

	err := interceptor(context.Background(), "/svc/Method", nil, nil, nil, invoker)
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected wrapped Internal error, got %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected 1 invoker call (no retry), got %d", calls)
	}
}
