package main

import (
	"common_library/logging"
	"common_library/metadata"
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"schedule_service/internal/config"
	"schedule_service/internal/database/postgres"
	"schedule_service/internal/kafka"
	service "schedule_service/internal/service/service"
	pb "schedule_service/pkg/api"
	"strings"
	"syscall"
	"time"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

func main() {
	ctx := context.Background()
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	zapLogger, err := zap.NewDevelopment()
	if err != nil {
		panic(fmt.Sprintf("cannot create logger: %v", err))
	}

	logger := logging.New(zapLogger)

	ctx = logging.ContextWithLogger(ctx, logger)

	cfg := config.GetConfig()

	database, err := postgres.New(ctx, cfg)
	if err != nil {
		logger.Fatal(ctx, "cannot create db", zap.Error(err))
	}
	userClient, err := service.NewUserClient(cfg.UserClientDNS)
	if err != nil {
		logger.Fatal(ctx, "failed to create UserClient")
	}

	rawBrokers := strings.Split(cfg.KafkaBrokers, ",")
	brokers := make([]string, 0, len(rawBrokers))
	for _, b := range rawBrokers {
		if trimmed := strings.TrimSpace(b); trimmed != "" {
			brokers = append(brokers, trimmed)
		}
	}
	eventSender := kafka.NewEventSender(brokers, cfg.KafkaReminderTopic)

	schedule_service := service.NewScheduleServer(database, userClient, eventSender, logger)

	listener, err := net.Listen("tcp", fmt.Sprintf(":%s", cfg.GRPCPort))
	if err != nil {
		logger.Fatal(ctx, "cannot create listener", zap.Error(err))
	}

	server := grpc.NewServer(
		grpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer(
			metadata.NewMetadataUnaryInterceptor(),
			logging.NewUnaryLoggingInterceptor(logger),
		)),
	)
	database.RegisterHealthService(server) // readiness probe
	pb.RegisterScheduleServiceServer(server, schedule_service)

	logger.Info(ctx, "Starting gRPC server...", zap.String("port", cfg.GRPCPort),
		zap.String("user_service_addr", cfg.UserClientDNS))
	go func() {
		if err := server.Serve(listener); err != nil {
			logger.Fatal(ctx, "failed to serve", zap.Error(err))
		}
	}()

	<-ctx.Done()

	shutdownDone := make(chan struct{})
	go func() {
		server.GracefulStop()
		close(shutdownDone)
	}()
	select {
	case <-shutdownDone:
	case <-time.After(10 * time.Second):
		logger.Info(ctx, "GracefulStop timed out, forcing Stop")
		server.Stop()
	}

	if err := eventSender.Close(); err != nil {
		logger.Error(ctx, "failed to close event sender", zap.Error(err))
	}
	database.Close()
	userClient.Close()
	logger.Info(ctx, "Server Stopped")
}
