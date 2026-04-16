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

// notification pairs a user ID with the message text to send them.
type notification struct {
	userID string
	text   string
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

	notifications, ok := buildMessages(msg.Topic, payload)
	if !ok {
		d.logger.Warn("Unknown event shape; skipping",
			zap.String("topic", msg.Topic),
			zap.Int64("offset", msg.Offset),
		)
		return nil
	}

	for _, n := range notifications {
		chatID, err := d.resolver.GetChatID(ctx, n.userID)
		if err != nil {
			if errors.Is(err, errChatIDNotFound) {
				d.logger.Info("Recipient has no Telegram account; skipping",
					zap.String("topic", msg.Topic),
					zap.Int64("offset", msg.Offset),
					zap.String("user_id", n.userID),
				)
				continue
			}
			// gRPC failure — treat as retriable so we don't lose the event.
			return fmt.Errorf("resolve chat_id for %s: %w", n.userID, err)
		}

		if err := d.tg.SendMessage(ctx, chatID, n.text); err != nil {
			if errors.Is(err, errRetriableSend) {
				return err
			}
			// Terminal Telegram error (e.g. bot blocked by user, bad chat_id).
			d.logger.Warn("Terminal Telegram send error; skipping recipient",
				zap.String("topic", msg.Topic),
				zap.Int64("offset", msg.Offset),
				zap.String("user_id", n.userID),
				zap.Error(err),
			)
			continue
		}

		d.logger.Info("Delivered telegram notification",
			zap.String("topic", msg.Topic),
			zap.Int64("offset", msg.Offset),
			zap.String("user_id", n.userID),
		)
	}
	return nil
}

// buildMessages returns the list of notifications to send for a Kafka message.
// ok=false means the event shape is unknown and the message will be committed-and-skipped.
func buildMessages(topic string, payload map[string]any) ([]notification, bool) {
	switch topic {
	case "lesson-reminders":
		eventType, _ := payload["event_type"].(string)
		studentID, _ := payload["student_id"].(string)
		tutorID, _ := payload["tutor_id"].(string)
		if studentID == "" || tutorID == "" {
			return nil, false
		}
		when := lessonWhenClause(parseTime(payload["starts_at"]))
		var studentText, tutorText string
		switch eventType {
		case "booked":
			studentText = fmt.Sprintf(msgLessonBooked, when)
			tutorText = fmt.Sprintf(msgLessonBooked, when)
		case "cancelled":
			studentText = fmt.Sprintf(msgLessonCancelledStudent, when)
			tutorText = fmt.Sprintf(msgLessonCancelledTutor, when)
		case "rescheduled":
			studentText = fmt.Sprintf(msgLessonRescheduledStudent, when)
			tutorText = fmt.Sprintf(msgLessonRescheduledTutor, when)
		default:
			studentText = fmt.Sprintf(msgLessonReminder, when)
			tutorText = fmt.Sprintf(msgLessonReminder, when)
		}
		return []notification{
			{userID: studentID, text: studentText},
			{userID: tutorID, text: tutorText},
		}, true

	case "assignment-reminders":
		studentID, _ := payload["student_id"].(string)
		tutorID, _ := payload["tutor_id"].(string)
		if studentID == "" {
			return nil, false
		}
		title, _ := payload["title"].(string)
		due := parseTime(payload["due_date"])
		var sb strings.Builder
		sb.WriteString(msgAssignmentStudent)
		if title != "" {
			sb.WriteString(assignmentTitleSep)
			sb.WriteString(title)
		}
		if !due.IsZero() {
			sb.WriteString(assignmentDuePrefix)
			sb.WriteString(due.UTC().Format(dateLayoutRU))
		}
		studentText := sb.String()

		result := []notification{{userID: studentID, text: studentText}}
		if tutorID != "" {
			var tb strings.Builder
			tb.WriteString(msgAssignmentTutor)
			if title != "" {
				tb.WriteString(assignmentTitleSep)
				tb.WriteString(title)
			}
			if !due.IsZero() {
				tb.WriteString(assignmentDuePrefix)
				tb.WriteString(due.UTC().Format(dateLayoutRU))
			}
			result = append(result, notification{userID: tutorID, text: tb.String()})
		}
		return result, true

	case "payment-reminders":
		studentID, _ := payload["student_id"].(string)
		tutorID, _ := payload["tutor_id"].(string)
		if studentID == "" || tutorID == "" {
			return nil, false
		}
		price := ""
		if v, ok := payload["price_rub"].(float64); ok {
			price = paymentPriceClause(v)
		}
		return []notification{
			{userID: studentID, text: fmt.Sprintf(msgPaymentStudent, price)},
			{userID: tutorID, text: fmt.Sprintf(msgPaymentTutor, price)},
		}, true

	default:
		return nil, false
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
