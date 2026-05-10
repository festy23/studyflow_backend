package logging

import (
	"context"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
)

func NewUnaryLoggingInterceptor(logger *Logger) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		start := time.Now()

		clientIP := "unknown"
		if p, ok := peer.FromContext(ctx); ok {
			clientIP = p.Addr.String()
		}

		fields := []zap.Field{
			zap.String("method", info.FullMethod),
			zap.String("client_ip", clientIP),
		}

		if md, ok := metadata.FromIncomingContext(ctx); ok {
			if vals := md.Get("x-trace-id"); len(vals) > 0 && vals[0] != "" {
				fields = append(fields, zap.String("trace_id", vals[0]))
			}
		}

		logger.Info(ctx, "grpc unary request", fields...)

		ctx = ContextWithLogger(ctx, logger)

		resp, err := handler(ctx, req)

		duration := time.Since(start)

		fields = []zap.Field{
			zap.String("method", info.FullMethod),
			zap.Duration("duration", duration),
		}

		if err != nil {
			fields = append(fields, zap.Error(err))
			logger.Error(ctx, "request failed", fields...)
		} else {
			logger.Info(ctx, "request handled", fields...)
		}

		return resp, err
	}
}
