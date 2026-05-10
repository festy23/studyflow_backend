package main

import (
	"context"
	"encoding/json"
	"errors"
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

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	topicList := splitAndTrim(topics)
	brokerList := splitAndTrim(brokers)

	logger.Info("Starting notification consumer",
		zap.Strings("topics", topicList),
		zap.String("group_id", groupID),
	)
	// Broker endpoints are infra detail; keep them out of INFO logs.
	logger.Debug("Kafka broker list", zap.Strings("brokers", brokerList))

	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     brokerList,
		GroupID:     groupID,
		GroupTopics: topicList,
		// For new consumer groups (no committed offsets), start at the latest
		// offset rather than replaying the entire topic history. Previously
		// committed offsets for the group still take precedence.
		StartOffset: kafka.LastOffset,
	})
	defer func() { _ = reader.Close() }()

	runConsumer(ctx, logger, reader)
}

func runConsumer(ctx context.Context, logger *zap.Logger, reader messageCommitter) {
	for {
		msg, err := reader.FetchMessage(ctx)
		if err != nil {
			// If the signal context was cancelled, there is no fetched
			// message to commit; exit cleanly without touching the reader.
			if ctx.Err() != nil {
				logger.Info("Consumer shutting down")
				return
			}
			logger.Error("Failed to fetch message", zap.Error(err))
			continue
		}

		// processMessage contract:
		//   nil error  -> success or non-retriable malformed payload; commit.
		//   non-nil    -> retriable/transient failure; do NOT commit so the
		//                 same offset is re-fetched on the next iteration.
		// Today there are no retriable failures, but the contract is wired
		// so future Telegram-dispatch errors can opt into at-least-once
		// delivery without losing offsets.
		if perr := processMessage(logger, msg); perr != nil {
			logger.Error("Processing failed; will retry without commit",
				zap.String("topic", msg.Topic),
				zap.Int("partition", msg.Partition),
				zap.Int64("offset", msg.Offset),
				zap.Error(perr),
			)
			continue
		}

		// Use a fresh deadline so the final commit succeeds even when ctx
		// has already been cancelled by SIGTERM (graceful drain window).
		commitCtx, cancelCommit := context.WithTimeout(context.Background(), commitTimeout)
		if err := reader.CommitMessages(commitCtx, msg); err != nil {
			logger.Error("Failed to commit message", zap.Error(err))
		}
		cancelCommit()

		// After a successful commit, honor shutdown if it was requested
		// while we were processing.
		if ctx.Err() != nil {
			logger.Info("Consumer shutting down")
			return
		}
	}
}

// processMessage logs the event without exposing PII from the payload.
// Returns a non-nil error only for retriable failures; malformed payloads
// are treated as terminal (logged and skipped) so they don't block the
// partition.
func processMessage(logger *zap.Logger, msg kafka.Message) error {
	var payload map[string]any
	if jsonErr := json.Unmarshal(msg.Value, &payload); jsonErr != nil {
		truncated := truncateBytes(msg.Value, 256)
		logger.Warn("Failed to unmarshal message",
			zap.String("topic", msg.Topic),
			zap.Int("partition", msg.Partition),
			zap.Int64("offset", msg.Offset),
			zap.Int("value_len", len(msg.Value)),
			zap.ByteString("value_head", truncated),
			zap.Error(jsonErr),
		)
		// Non-retriable: skip and commit.
		return nil
	}

	fields := []zap.Field{
		zap.String("topic", msg.Topic),
		zap.Int("partition", msg.Partition),
		zap.Int64("offset", msg.Offset),
	}
	if et, ok := payload["event_type"].(string); ok && et != "" {
		fields = append(fields, zap.String("event_type", et))
	}
	// IMPORTANT: never log the full payload — it contains PII
	// (telegram_id, chat_id, message text, etc.).
	logger.Info("Received event", fields...)
	return nil
}

// errRetriable is reserved for future use by processMessage callers that
// want to signal a transient failure. Kept here so the contract is visible.
var errRetriable = errors.New("retriable processing failure") //nolint:unused // reserved for future Telegram dispatcher

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
