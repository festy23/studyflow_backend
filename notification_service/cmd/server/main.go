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

	rawTopics := strings.Split(topics, ",")
	topicList := make([]string, 0, len(rawTopics))
	for _, t := range rawTopics {
		if trimmed := strings.TrimSpace(t); trimmed != "" {
			topicList = append(topicList, trimmed)
		}
	}

	brokerList := strings.Split(brokers, ",")
	for i, b := range brokerList {
		brokerList[i] = strings.TrimSpace(b)
	}

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

		var payload map[string]interface{}
		if jsonErr := json.Unmarshal(msg.Value, &payload); jsonErr != nil {
			logger.Warn("Failed to unmarshal message",
				zap.String("topic", msg.Topic),
				zap.ByteString("value", msg.Value),
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

		// Commit unconditionally: malformed messages are logged and skipped.
		// When real dispatch logic is added, consider skipping commit on processing errors.
		if err := reader.CommitMessages(ctx, msg); err != nil {
			logger.Error("Failed to commit message", zap.Error(err))
		}
	}
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
