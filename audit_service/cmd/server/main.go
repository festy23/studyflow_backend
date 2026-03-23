package main

import (
	"context"
	"fmt"
	"net"
	"os/signal"
	"syscall"
	"time"

	grpcmiddleware "github.com/grpc-ecosystem/go-grpc-middleware"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	"audit_service/config"
	"audit_service/internal/db"
	"audit_service/internal/handler"
	"audit_service/internal/repository"
	"audit_service/internal/service"
	pb "audit_service/pkg/api"

	"common_library/logging"
	"common_library/metadata"
)

func main() {
	ctx := context.Background()
	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	defer stop()

	zapLogger, err := zap.NewProduction()
	if err != nil {
		panic(err)
	}

	logger := logging.New(zapLogger)
	ctx = logging.ContextWithLogger(ctx, logger)

	cfg, err := config.New()
	if err != nil {
		logger.Fatal(ctx, "cannot create config", zap.Error(err))
	}
	logger.Info(ctx, "created config")

	database, err := db.New(ctx, cfg.PostgresURL, cfg.PostgresMaxConn, cfg.PostgresMinConn, cfg.PostgresAutoMigrate)
	if err != nil {
		logger.Fatal(ctx, "cannot create db", zap.Error(err))
	}
	logger.Info(ctx, "connected db")

	auditRepo := repository.NewAuditRepository(database)
	auditSvc := service.NewAuditService(auditRepo)
	auditHandler := handler.NewAuditServiceServer(auditSvc)

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.GRPCPort))
	if err != nil {
		logger.Fatal(ctx, "cannot create listener", zap.Error(err))
	}

	server := grpc.NewServer(
		grpc.UnaryInterceptor(grpcmiddleware.ChainUnaryServer(
			metadata.NewMetadataUnaryInterceptor(),
			logging.NewUnaryLoggingInterceptor(logger),
		)),
	)
	pb.RegisterAuditServiceServer(server, auditHandler)

	logger.Info(ctx, "Starting gRPC server...", zap.Int("port", cfg.GRPCPort))
	go func() {
		if err := server.Serve(listener); err != nil {
			logger.Fatal(ctx, "failed to serve", zap.Error(err))
		}
	}()

	<-ctx.Done()
	logger.Info(ctx, "Shutting down gRPC server...")

	stopped := make(chan struct{})
	go func() {
		server.GracefulStop()
		close(stopped)
	}()
	gracefulTimeout := time.NewTimer(15 * time.Second)
	defer gracefulTimeout.Stop()

	select {
	case <-stopped:
		logger.Info(ctx, "Server stopped gracefully")
	case <-gracefulTimeout.C:
		server.Stop()
		logger.Info(ctx, "Server stopped forcefully")
	}
}
