package client

import (
	"common_library/utils"
	"context"
	"time"

	"google.golang.org/grpc"
)

// RetryUnaryInterceptor returns a unary client interceptor that retries the
// invocation using exponential backoff. Only codes.Unavailable is retriable
// (as enforced by utils.RetryWithBackoff / utils.IsRetriable).
func RetryUnaryInterceptor(maxAttempts int, baseDelay time.Duration) grpc.UnaryClientInterceptor {
	return func(
		ctx context.Context,
		method string,
		req, reply any,
		cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker,
		opts ...grpc.CallOption,
	) error {
		_, err := utils.RetryWithBackoff(ctx, maxAttempts, baseDelay, func() (any, error) {
			return nil, invoker(ctx, method, req, reply, cc, opts...)
		})
		return err
	}
}
