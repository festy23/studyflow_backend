package main

import (
	"apigateway/internal/cache"
	"apigateway/internal/client"
	"apigateway/internal/config"
	"apigateway/internal/handler"
	"apigateway/internal/middleware"
	auditpb "audit_service/pkg/api"
	"common_library/logging"
	"context"
	faqpb "faq_service/pkg/api"
	filepb "fileservice/pkg/api"
	"fmt"
	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	homeworkpb "homework_service/pkg/api"
	"net/http"
	"os"
	"os/signal"
	paymentpb "paymentservice/pkg/api"
	schedulepb "schedule_service/pkg/api"
	"strings"
	"syscall"
	"time"
	userpb "userservice/pkg/api"
)

func newZapLogger(appEnv string) (*zap.Logger, error) {
	switch strings.ToLower(strings.TrimSpace(appEnv)) {
	case "dev", "development":
		return zap.NewDevelopment()
	default:
		return zap.NewProduction()
	}
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Build a temporary bootstrap logger before config is loaded so we can
	// report config errors. It defaults to production unless APP_ENV opts in.
	bootstrapLogger, err := newZapLogger(os.Getenv("APP_ENV"))
	if err != nil {
		panic(err)
	}
	logger := logging.New(bootstrapLogger)

	cfg, err := config.New()
	if err != nil {
		logger.Fatal(ctx, "cannot create config", zap.Error(err))
	}

	zapLogger, err := newZapLogger(cfg.AppEnv)
	if err != nil {
		logger.Fatal(ctx, "cannot create zap logger", zap.Error(err))
	}
	logger = logging.New(zapLogger)

	userGrpcClient, closeFunc := client.New(ctx, cfg.UserServiceURL)
	defer closeFunc()

	fileGrpcClient, closeFunc := client.New(ctx, cfg.FileServiceURL)
	defer closeFunc()

	homeworkGrpcClient, closeFunc := client.New(ctx, cfg.HomeworkServiceURL)
	defer closeFunc()

	paymentGrpcClient, closeFunc := client.New(ctx, cfg.PaymentServiceURL)
	defer closeFunc()

	scheduleGrpcClient, closeFunc := client.New(ctx, cfg.ScheduleServiceURL)
	defer closeFunc()

	faqGrpcClient, closeFunc := client.New(ctx, cfg.FAQServiceURL)
	defer closeFunc()

	auditGrpcClient, closeFunc := client.New(ctx, cfg.AuditServiceURL)
	defer closeFunc()

	redisConn := redis.NewClient(&redis.Options{
		Addr: cfg.RedisURL,
	})

	redisCache := cache.NewRedisCache(redisConn, logger)

	userClient := userpb.NewUserServiceClient(userGrpcClient)
	userHandler := handler.NewUserHandler(userClient, redisCache)
	authHandler := handler.NewSignUpHandler(userClient)

	fileClient := filepb.NewFileServiceClient(fileGrpcClient)
	fileHandler := handler.NewFileHandler(fileClient, cfg.MinioURL)

	homeworkClient := homeworkpb.NewHomeworkServiceClient(homeworkGrpcClient)
	homeworkHandler := handler.NewHomeworkHandler(homeworkClient)

	paymentClient := paymentpb.NewPaymentServiceClient(paymentGrpcClient)
	paymentHandler := handler.NewPaymentHandler(paymentClient)

	scheduleClient := schedulepb.NewScheduleServiceClient(scheduleGrpcClient)
	scheduleHandler := handler.NewScheduleHandler(scheduleClient)

	faqClient := faqpb.NewFAQServiceClient(faqGrpcClient)
	faqHandler := handler.NewFAQHandler(faqClient, redisCache)

	auditClient := auditpb.NewAuditServiceClient(auditGrpcClient)
	auditMiddleware := middleware.NewAuditMiddleware(auditClient, zapLogger)

	authMiddleware := middleware.NewAuthMiddleware(userClient)
	r := chi.NewRouter()
	r.Use(middleware.NewCORSMiddleware())
	r.Use(middleware.NewLoggingMiddleware(logger))
	r.Use(func(next http.Handler) http.Handler {
		return http.MaxBytesHandler(next, 10<<20) // 10 MB
	})
	r.Use(auditMiddleware)
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	r.Route("/users", func(r chi.Router) {
		authHandler.RegisterRoutes(r)
		userHandler.RegisterRoutes(r, authMiddleware)
	})

	r.Route("/files", func(r chi.Router) {
		fileHandler.RegisterRoutes(r, authMiddleware)
	})

	r.Route("/schedule", func(r chi.Router) {
		scheduleHandler.RegisterRoutes(r, authMiddleware)
	})

	r.Route("/payment", func(r chi.Router) {
		paymentHandler.RegisterRoutes(r, authMiddleware)
	})

	r.Route("/homework", func(r chi.Router) {
		homeworkHandler.RegisterRoutes(r, authMiddleware)
	})

	faqHandler.RegisterRoutes(r)

	port := fmt.Sprintf(":%d", cfg.HTTPPort)
	logger.Info(ctx, "Starting server", zap.String("port", port))

	srv := &http.Server{
		Addr:              port,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal(ctx, "cannot start http server", zap.Error(err))
		}
	}()

	<-ctx.Done()
	logger.Info(ctx, "Shutting down server...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Fatal(ctx, "server forced to shutdown", zap.Error(err))
	}
	logger.Info(ctx, "Server stopped")
}
