package utils

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"sync"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func IsRetriable(err error) bool {
	if s, ok := status.FromError(err); ok {
		return s.Code() == codes.Unavailable
	}
	return false
}

func RetryWithBackoff[T any](
	ctx context.Context,
	maxRetries int,
	baseDelay time.Duration,
	fn func() (T, error),
) (T, error) {
	var zero T
	if maxRetries <= 0 {
		return zero, fmt.Errorf("maxRetries must be > 0, got %d", maxRetries)
	}
	var lastErr error

	for i := range maxRetries {
		select {
		case <-ctx.Done():
			return zero, ctx.Err()
		default:
		}

		result, err := fn()
		if err == nil {
			return result, nil
		}
		lastErr = err

		if !IsRetriable(err) {
			return zero, err
		}

		if i < maxRetries-1 {
			jitter := time.Duration(rand.Int63n(int64(baseDelay))) //nolint:gosec // jitter doesn't need crypto rand
			delay := time.Duration(math.Pow(2, float64(i)))*baseDelay + jitter
			select {
			case <-ctx.Done():
				return zero, ctx.Err()
			case <-time.After(delay):
			}
		}
	}
	return zero, fmt.Errorf("after %d attempts: %w", maxRetries, lastErr)
}

type CircuitState int

const (
	StateClosed   CircuitState = iota
	StateOpen
	StateHalfOpen
)

type CircuitBreaker struct {
	mu              sync.Mutex
	state           CircuitState
	failureCount    int
	failureThreshold int
	resetTimeout    time.Duration
	lastFailureTime time.Time
}

func NewCircuitBreaker(failureThreshold int, resetTimeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		state:            StateClosed,
		failureThreshold: failureThreshold,
		resetTimeout:     resetTimeout,
	}
}

var ErrCircuitOpen = fmt.Errorf("circuit breaker is open")

func (cb *CircuitBreaker) Execute(fn func() error) error {
	cb.mu.Lock()
	if cb.state == StateOpen {
		if time.Since(cb.lastFailureTime) > cb.resetTimeout {
			cb.state = StateHalfOpen
		} else {
			cb.mu.Unlock()
			return ErrCircuitOpen
		}
	}
	cb.mu.Unlock()

	err := fn()

	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err != nil && IsRetriable(err) {
		cb.failureCount++
		cb.lastFailureTime = time.Now()
		if cb.failureCount >= cb.failureThreshold {
			cb.state = StateOpen
		}
		return err
	}

	if err == nil {
		cb.failureCount = 0
		cb.state = StateClosed
	}

	return err
}

func RetryWithCircuitBreaker[T any](
	ctx context.Context,
	cb *CircuitBreaker,
	maxRetries int,
	baseDelay time.Duration,
	fn func() (T, error),
) (T, error) {
	wrappedFn := func() (T, error) {
		var result T
		var fnErr error
		cbErr := cb.Execute(func() error {
			result, fnErr = fn()
			return fnErr
		})
		if cbErr != nil && cbErr != fnErr {
			return result, cbErr
		}
		return result, fnErr
	}
	return RetryWithBackoff(ctx, maxRetries, baseDelay, wrappedFn)
}
