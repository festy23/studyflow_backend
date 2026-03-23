package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"faq_service/config"
	"faq_service/internal/db"
	"faq_service/internal/handler"
	"faq_service/internal/repository"
	"faq_service/internal/service"
	pb "faq_service/pkg/api"
)

func main() {
	logger, _ := zap.NewProduction()
	defer func() { _ = logger.Sync() }()

	cfg, err := config.Load()
	if err != nil {
		logger.Fatal("failed to load config", zap.Error(err))
	}

	ctx := context.Background()
	pool, err := db.NewPool(ctx, cfg.DSN())
	if err != nil {
		logger.Fatal("failed to connect to db", zap.Error(err))
	}
	defer pool.Close()

	if err := db.RunMigrations(cfg.DSN()); err != nil {
		logger.Fatal("failed to run migrations", zap.Error(err))
	}

	repo := repository.NewFAQRepo(pool)
	svc := service.NewFAQService(repo)
	srv := handler.NewFAQServer(svc)

	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", cfg.GRPCPort))
	if err != nil {
		logger.Fatal("failed to listen", zap.Error(err))
	}

	grpcServer := grpc.NewServer()
	pb.RegisterFAQServiceServer(grpcServer, srv)
	reflection.Register(grpcServer)

	go func() {
		logger.Info("FAQ service started", zap.String("port", cfg.GRPCPort))
		if err := grpcServer.Serve(lis); err != nil {
			logger.Fatal("failed to serve", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down...")
	grpcServer.GracefulStop()
}
