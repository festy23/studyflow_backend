package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

// messageDispatcher consumes a parsed Kafka message and turns it into
// a Telegram notification (or just logs in test/local mode).
// Returns non-nil only for retriable failures.
type messageDispatcher interface {
	Dispatch(ctx context.Context, msg kafka.Message) error
}

// logOnlyDispatcher is used when TELEGRAM_BOT_TOKEN is unset — it just logs
// the event type and structural fields. Same as the pre-Telegram behaviour.
type logOnlyDispatcher struct{ logger *zap.Logger }

func (d logOnlyDispatcher) Dispatch(_ context.Context, msg kafka.Message) error {
	var payload map[string]any
	if err := json.Unmarshal(msg.Value, &payload); err != nil {
		d.logger.Warn("Failed to unmarshal message",
			zap.String("topic", msg.Topic),
			zap.Int("partition", msg.Partition),
			zap.Int64("offset", msg.Offset),
			zap.Error(err),
		)
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
	d.logger.Info("Received event (log-only mode)", fields...)
	return nil
}

// telegramDispatcher resolves the recipient's chat_id via user_service and
// sends a formatted message via the Telegram Bot API.
type telegramDispatcher struct {
	tg       telegramSender
	resolver chatIDResolver
	logger   *zap.Logger
}

func (d *telegramDispatcher) Dispatch(ctx context.Context, msg kafka.Message) error {
	var payload map[string]any
	if err := json.Unmarshal(msg.Value, &payload); err != nil {
		d.logger.Warn("Failed to unmarshal message",
			zap.String("topic", msg.Topic),
			zap.Int64("offset", msg.Offset),
			zap.Error(err),
		)
		// Malformed — terminal; commit and move on.
		return nil
	}

	recipient, text, ok := buildMessage(msg.Topic, payload)
	if !ok {
		d.logger.Warn("Unknown event shape; skipping",
			zap.String("topic", msg.Topic),
			zap.Int64("offset", msg.Offset),
		)
		return nil
	}

	chatID, err := d.resolver.GetChatID(ctx, recipient)
	if err != nil {
		if errors.Is(err, errChatIDNotFound) {
			d.logger.Info("Recipient has no Telegram account; skipping",
				zap.String("topic", msg.Topic),
				zap.Int64("offset", msg.Offset),
			)
			return nil
		}
		// gRPC failure — treat as retriable so we don't lose the event.
		return fmt.Errorf("resolve chat_id: %w", err)
	}

	if err := d.tg.SendMessage(ctx, chatID, text); err != nil {
		if errors.Is(err, errRetriableSend) {
			return err
		}
		// Terminal Telegram error (e.g. bot blocked by user, bad chat_id).
		d.logger.Warn("Terminal Telegram send error; committing",
			zap.String("topic", msg.Topic),
			zap.Int64("offset", msg.Offset),
			zap.Error(err),
		)
		return nil
	}

	d.logger.Info("Delivered telegram notification",
		zap.String("topic", msg.Topic),
		zap.Int64("offset", msg.Offset),
	)
	return nil
}

// buildMessage returns (recipient_user_id, text, ok).
// Notifications target the student by default — the audience that benefits
// most from reminders/notifications. Returning ok=false means the event
// shape is unknown and the message will be committed-and-skipped.
func buildMessage(topic string, payload map[string]any) (string, string, bool) {
	switch topic {
	case "lesson-reminders":
		eventType, _ := payload["event_type"].(string)
		studentID, _ := payload["student_id"].(string)
		if studentID == "" {
			return "", "", false
		}
		startsAt := parseTime(payload["starts_at"])
		when := ""
		if !startsAt.IsZero() {
			when = " at " + startsAt.UTC().Format("2006-01-02 15:04 UTC")
		}
		switch eventType {
		case "booked":
			return studentID, "New lesson booked" + when + ".", true
		case "cancelled":
			return studentID, "Your lesson" + when + " was cancelled.", true
		default:
			return studentID, "Lesson reminder" + when + ".", true
		}
	case "assignment-reminders":
		studentID, _ := payload["student_id"].(string)
		if studentID == "" {
			return "", "", false
		}
		title, _ := payload["title"].(string)
		due := parseTime(payload["due_date"])
		var b strings.Builder
		b.WriteString("Assignment reminder")
		if title != "" {
			b.WriteString(": ")
			b.WriteString(title)
		}
		if !due.IsZero() {
			b.WriteString(" — due ")
			b.WriteString(due.UTC().Format("2006-01-02 15:04 UTC"))
		}
		return studentID, b.String(), true
	default:
		return "", "", false
	}
}

func parseTime(v any) time.Time {
	s, ok := v.(string)
	if !ok || s == "" {
		return time.Time{}
	}
	if parsed, err := time.Parse(time.RFC3339, s); err == nil {
		return parsed
	}
	return time.Time{}
}
