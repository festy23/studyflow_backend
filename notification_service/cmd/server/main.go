package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

func main() {
	logger, err := zap.NewProduction()
	if err != nil {
		panic(fmt.Sprintf("cannot create logger: %v", err))
	}
	defer func() { _ = logger.Sync() }()

	brokers := getEnv("KAFKA_BROKERS", "kafka:9092")
	topics := getEnv("KAFKA_TOPICS", "lesson-reminders,assignment-reminders")
	groupID := getEnv("KAFKA_GROUP_ID", "notification-service")

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	topicList := splitAndTrim(topics)
	brokerList := splitAndTrim(brokers)

	logger.Info("Starting notification consumer",
		zap.Strings("topics", topicList),
		zap.Strings("brokers", brokerList),
		zap.String("group_id", groupID),
	)

	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     brokerList,
		GroupID:     groupID,
		GroupTopics: topicList,
	})
	defer func() { _ = reader.Close() }()

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

		processMessage(logger, msg)

		// Commit unconditionally: malformed messages are logged and skipped.
		// When real dispatch logic is added, consider skipping commit on processing errors.
		if err := reader.CommitMessages(ctx, msg); err != nil {
			logger.Error("Failed to commit message", zap.Error(err))
		}
	}
}

func processMessage(logger *zap.Logger, msg kafka.Message) {
	var payload map[string]any
	if jsonErr := json.Unmarshal(msg.Value, &payload); jsonErr != nil {
		truncated := truncateBytes(msg.Value, 256)
		logger.Warn("Failed to unmarshal message",
			zap.String("topic", msg.Topic),
			zap.Int("value_len", len(msg.Value)),
			zap.ByteString("value_head", truncated),
			zap.Error(jsonErr),
		)
	} else {
		logger.Info("Received event",
			zap.String("topic", msg.Topic),
			zap.Int("partition", msg.Partition),
			zap.Int64("offset", msg.Offset),
			zap.Any("payload", payload),
		)
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
