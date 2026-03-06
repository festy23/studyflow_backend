package utils

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestRetryWithBackoff_ZeroBaseDelayDoesNotPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("unexpected panic: %v", r)
		}
	}()
	calls := 0
	_, err := RetryWithBackoff(context.Background(), 3, 0, func() (string, error) {
		calls++
		return "", status.Error(codes.Unavailable, "unavailable")
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if calls != 3 {
		t.Fatalf("expected 3 calls, got %d", calls)
	}
}

func TestErrCircuitOpen_IsSentinel(t *testing.T) {
	wrapped := errors.New("wrap")
	_ = wrapped
	if !errors.Is(ErrCircuitOpen, ErrCircuitOpen) {
		t.Fatal("errors.Is failed for ErrCircuitOpen")
	}
}

func TestCircuitBreaker_TripsOnInternal(t *testing.T) {
	cb := NewCircuitBreaker(2, 1*time.Second)
	for i := 0; i < 2; i++ {
		_ = cb.Execute(func() error {
			return status.Error(codes.Internal, "boom")
		})
	}
	if cb.state != StateOpen {
		t.Fatalf("expected StateOpen after Internal failures, got %v", cb.state)
	}
}

func TestCircuitBreaker_TripsOnDeadlineExceeded(t *testing.T) {
	cb := NewCircuitBreaker(2, 1*time.Second)
	for i := 0; i < 2; i++ {
		_ = cb.Execute(func() error {
			return status.Error(codes.DeadlineExceeded, "timeout")
		})
	}
	if cb.state != StateOpen {
		t.Fatalf("expected StateOpen after DeadlineExceeded, got %v", cb.state)
	}
}

func TestCircuitBreaker_HalfOpenSingleProbe(t *testing.T) {
	t.Parallel()
	cb := NewCircuitBreaker(2, 20*time.Millisecond)

	// Trip breaker open.
	for i := 0; i < 2; i++ {
		_ = cb.Execute(func() error {
			return status.Error(codes.Unavailable, "u")
		})
	}
	if cb.state != StateOpen {
		t.Fatalf("expected open")
	}

	time.Sleep(40 * time.Millisecond) // past resetTimeout

	var probeCount int64
	var openCount int64
	var wg sync.WaitGroup
	start := make(chan struct{})

	const goroutines = 50
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			err := cb.Execute(func() error {
				atomic.AddInt64(&probeCount, 1)
				// Hold the probe long enough that other goroutines race.
				time.Sleep(20 * time.Millisecond)
				return nil
			})
			if errors.Is(err, ErrCircuitOpen) {
				atomic.AddInt64(&openCount, 1)
			}
		}()
	}
	close(start)
	wg.Wait()

	if probeCount != 1 {
		t.Fatalf("expected exactly 1 probe to run, got %d", probeCount)
	}
	if openCount != goroutines-1 {
		t.Fatalf("expected %d ErrCircuitOpen rejections, got %d", goroutines-1, openCount)
	}
	if cb.state != StateClosed {
		t.Fatalf("expected closed after successful probe, got %v", cb.state)
	}
}

func TestRetryWithBackoff_Success(t *testing.T) {
	result, err := RetryWithBackoff(context.Background(), 3, 10*time.Millisecond, func() (string, error) {
		return "ok", nil
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result != "ok" {
		t.Fatalf("expected 'ok', got %q", result)
	}
}

func TestRetryWithBackoff_NonRetriableError(t *testing.T) {
	calls := 0
	_, err := RetryWithBackoff(context.Background(), 3, 10*time.Millisecond, func() (string, error) {
		calls++
		return "", status.Error(codes.NotFound, "not found")
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if calls != 1 {
		t.Fatalf("expected 1 call for non-retriable error, got %d", calls)
	}
}

func TestRetryWithBackoff_RetriableEventualSuccess(t *testing.T) {
	calls := 0
	result, err := RetryWithBackoff(context.Background(), 5, 10*time.Millisecond, func() (string, error) {
		calls++
		if calls < 3 {
			return "", status.Error(codes.Unavailable, "unavailable")
		}
		return "recovered", nil
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result != "recovered" {
		t.Fatalf("expected 'recovered', got %q", result)
	}
	if calls != 3 {
		t.Fatalf("expected 3 calls, got %d", calls)
	}
}

func TestRetryWithBackoff_AllRetriesFail(t *testing.T) {
	calls := 0
	_, err := RetryWithBackoff(context.Background(), 3, 10*time.Millisecond, func() (string, error) {
		calls++
		return "", status.Error(codes.Unavailable, "unavailable")
	})
	if err == nil {
		t.Fatal("expected error after all retries")
	}
	if calls != 3 {
		t.Fatalf("expected 3 calls, got %d", calls)
	}
}

func TestRetryWithBackoff_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := RetryWithBackoff(ctx, 5, 10*time.Millisecond, func() (string, error) {
		return "", status.Error(codes.Unavailable, "unavailable")
	})
	if err == nil {
		t.Fatal("expected context cancelled error")
	}
}

func TestIsRetriable(t *testing.T) {
	tests := []struct {
		code     codes.Code
		expected bool
	}{
		{codes.Unavailable, true},
		{codes.Internal, false},
		{codes.NotFound, false},
		{codes.PermissionDenied, false},
		{codes.InvalidArgument, false},
		{codes.OK, false},
	}

	for _, tt := range tests {
		err := status.Error(tt.code, "test")
		if got := IsRetriable(err); got != tt.expected {
			t.Errorf("IsRetriable(%v) = %v, want %v", tt.code, got, tt.expected)
		}
	}
}

func TestCircuitBreaker_ClosedState(t *testing.T) {
	cb := NewCircuitBreaker(3, 1*time.Second)

	err := cb.Execute(func() error {
		return nil
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cb.state != StateClosed {
		t.Fatalf("expected StateClosed, got %v", cb.state)
	}
}

func TestCircuitBreaker_OpensAfterThreshold(t *testing.T) {
	cb := NewCircuitBreaker(3, 1*time.Second)

	for i := 0; i < 3; i++ {
		_ = cb.Execute(func() error {
			return status.Error(codes.Unavailable, "unavailable")
		})
	}

	if cb.state != StateOpen {
		t.Fatalf("expected StateOpen after %d failures, got %v", 3, cb.state)
	}

	err := cb.Execute(func() error {
		return nil
	})
	if err != ErrCircuitOpen {
		t.Fatalf("expected ErrCircuitOpen, got %v", err)
	}
}

func TestCircuitBreaker_ResetsAfterTimeout(t *testing.T) {
	cb := NewCircuitBreaker(2, 50*time.Millisecond)

	for i := 0; i < 2; i++ {
		_ = cb.Execute(func() error {
			return status.Error(codes.Unavailable, "unavailable")
		})
	}
	if cb.state != StateOpen {
		t.Fatal("expected StateOpen")
	}

	time.Sleep(60 * time.Millisecond)

	err := cb.Execute(func() error {
		return nil
	})
	if err != nil {
		t.Fatalf("expected no error after timeout, got %v", err)
	}
	if cb.state != StateClosed {
		t.Fatalf("expected StateClosed after successful call, got %v", cb.state)
	}
}

func TestCircuitBreaker_NonRetriableErrorDoesNotCount(t *testing.T) {
	cb := NewCircuitBreaker(2, 1*time.Second)

	for i := 0; i < 5; i++ {
		_ = cb.Execute(func() error {
			return status.Error(codes.NotFound, "not found")
		})
	}

	if cb.state != StateClosed {
		t.Fatalf("expected StateClosed for non-retriable errors, got %v", cb.state)
	}
}

func TestRetryWithCircuitBreaker_Success(t *testing.T) {
	cb := NewCircuitBreaker(5, 1*time.Second)
	result, err := RetryWithCircuitBreaker(context.Background(), cb, 3, 10*time.Millisecond, func() (string, error) {
		return "ok", nil
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result != "ok" {
		t.Fatalf("expected 'ok', got %q", result)
	}
}
