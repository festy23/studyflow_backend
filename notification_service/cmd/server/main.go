package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

// commitTimeout is the deadline applied to the final CommitMessages call
// during graceful shutdown. It is independent of the signal context so the
// in-flight message can be acknowledged even after SIGTERM cancels ctx.
const commitTimeout = 5 * time.Second

// messageCommitter is the subset of *kafka.Reader used by the consumer loop.
// It is defined as an interface so the loop can be unit-tested with a fake.
type messageCommitter interface {
	FetchMessage(ctx context.Context) (kafka.Message, error)
	CommitMessages(ctx context.Context, msgs ...kafka.Message) error
}

func main() {
	logger, err := zap.NewProduction()
	if err != nil {
		panic(fmt.Sprintf("cannot create logger: %v", err))
	}
	defer func() { _ = logger.Sync() }()

	brokers := getEnv("KAFKA_BROKERS", "kafka:9092")
	topics := getEnv("KAFKA_TOPICS", "lesson-reminders,assignment-reminders")
	groupID := getEnv("KAFKA_GROUP_ID", "notification-service")
	telegramToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	userServiceAddr := getEnv("USER_SERVICE_ADDR", "user_service:50051")

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	topicList := splitAndTrim(topics)
	brokerList := splitAndTrim(brokers)

	logger.Info("Starting notification consumer",
		zap.Strings("topics", topicList),
		zap.String("group_id", groupID),
		zap.Bool("telegram_enabled", telegramToken != ""),
	)
	logger.Debug("Kafka broker list", zap.Strings("brokers", brokerList))

	var dispatcher messageDispatcher
	if telegramToken != "" {
		resolver, dialErr := dialUserService(userServiceAddr)
		if dialErr != nil {
			logger.Fatal("Failed to dial user_service", zap.Error(dialErr))
		}
		defer resolver.Close()
		dispatcher = &telegramDispatcher{
			tg:       newTelegramClient(telegramToken),
			resolver: resolver,
			logger:   logger,
		}
	} else {
		logger.Warn("TELEGRAM_BOT_TOKEN not set — running in log-only mode (no message delivery)")
		dispatcher = logOnlyDispatcher{logger: logger}
	}

	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     brokerList,
		GroupID:     groupID,
		GroupTopics: topicList,
		StartOffset: kafka.LastOffset,
	})
	defer func() { _ = reader.Close() }()

	runConsumer(ctx, logger, reader, dispatcher)
}

func runConsumer(ctx context.Context, logger *zap.Logger, reader messageCommitter, dispatcher messageDispatcher) {
	for {
		msg, err := reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				logger.Info("Consumer shutting down")
				return
			}
			logger.Error("Failed to fetch message", zap.Error(err))
			continue
		}

		// Dispatcher contract:
		//   nil error  -> commit (success or terminal failure that should not block partition).
		//   non-nil    -> retriable; do NOT commit, re-fetch on next iteration.
		if perr := dispatcher.Dispatch(ctx, msg); perr != nil {
			logger.Error("Dispatch failed; will retry without commit",
				zap.String("topic", msg.Topic),
				zap.Int("partition", msg.Partition),
				zap.Int64("offset", msg.Offset),
				zap.Error(perr),
			)
			continue
		}

		// Fresh deadline so the final commit succeeds even when ctx was
		// cancelled by SIGTERM during dispatch (graceful drain window).
		commitCtx, cancelCommit := context.WithTimeout(context.Background(), commitTimeout)
		if err := reader.CommitMessages(commitCtx, msg); err != nil {
			logger.Error("Failed to commit message", zap.Error(err))
		}
		cancelCommit()

		if ctx.Err() != nil {
			logger.Info("Consumer shutting down")
			return
		}
	}
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

func splitAndTrim(csv string) []string {
	raw := strings.Split(csv, ",")
	result := make([]string, 0, len(raw))
	for _, s := range raw {
		if trimmed := strings.TrimSpace(s); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func truncateBytes(data []byte, max int) []byte {
	if len(data) <= max {
		return data
	}
	return data[:max]
}
