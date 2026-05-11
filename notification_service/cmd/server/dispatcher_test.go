package main

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

type stubResolver struct {
	chatID int64
	err    error
}

func (s stubResolver) GetChatID(_ context.Context, _ string) (int64, error) {
	return s.chatID, s.err
}

type recordingSender struct {
	chatID int64
	text   string
	err    error
	calls  int
}

func (r *recordingSender) SendMessage(_ context.Context, chatID int64, text string) error {
	r.calls++
	r.chatID = chatID
	r.text = text
	return r.err
}

func TestBuildMessage_LessonReminders(t *testing.T) {
	t.Run("booked", func(t *testing.T) {
		rec, text, ok := buildMessage("lesson-reminders", map[string]any{
			"event_type": "booked",
			"student_id": "stu-1",
			"starts_at":  "2026-05-12T10:00:00Z",
		})
		if !ok || rec != "stu-1" || !strings.Contains(text, "booked") {
			t.Fatalf("unexpected result: rec=%q text=%q ok=%v", rec, text, ok)
		}
	})

	t.Run("missing student_id is not deliverable", func(t *testing.T) {
		_, _, ok := buildMessage("lesson-reminders", map[string]any{"event_type": "booked"})
		if ok {
			t.Fatalf("expected ok=false for missing student_id")
		}
	})
}

func TestBuildMessage_AssignmentReminders(t *testing.T) {
	rec, text, ok := buildMessage("assignment-reminders", map[string]any{
		"student_id": "stu-1",
		"title":      "Algebra worksheet",
		"due_date":   "2026-05-15T18:00:00Z",
	})
	if !ok || rec != "stu-1" {
		t.Fatalf("unexpected: rec=%q ok=%v", rec, ok)
	}
	if !strings.Contains(text, "Algebra worksheet") {
		t.Fatalf("expected title in text, got %q", text)
	}
}

func TestBuildMessage_UnknownTopic(t *testing.T) {
	if _, _, ok := buildMessage("unknown", map[string]any{"student_id": "x"}); ok {
		t.Fatalf("expected ok=false for unknown topic")
	}
}

func TestTelegramDispatcher_HappyPath(t *testing.T) {
	sender := &recordingSender{}
	d := &telegramDispatcher{
		tg:       sender,
		resolver: stubResolver{chatID: 42},
		logger:   zap.NewNop(),
	}
	msg := kafka.Message{
		Topic: "lesson-reminders",
		Value: []byte(`{"event_type":"booked","student_id":"stu-1","starts_at":"2026-05-12T10:00:00Z"}`),
	}
	if err := d.Dispatch(context.Background(), msg); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if sender.calls != 1 || sender.chatID != 42 {
		t.Fatalf("expected 1 call to chat 42, got calls=%d chat=%d", sender.calls, sender.chatID)
	}
}

func TestTelegramDispatcher_NoTelegramAccount_Commits(t *testing.T) {
	sender := &recordingSender{}
	d := &telegramDispatcher{
		tg:       sender,
		resolver: stubResolver{err: errChatIDNotFound},
		logger:   zap.NewNop(),
	}
	msg := kafka.Message{
		Topic: "assignment-reminders",
		Value: []byte(`{"student_id":"stu-1","title":"hw"}`),
	}
	if err := d.Dispatch(context.Background(), msg); err != nil {
		t.Fatalf("expected nil (commit), got %v", err)
	}
	if sender.calls != 0 {
		t.Fatalf("expected no telegram send, got %d", sender.calls)
	}
}

func TestTelegramDispatcher_ResolverError_Retries(t *testing.T) {
	d := &telegramDispatcher{
		tg:       &recordingSender{},
		resolver: stubResolver{err: errors.New("boom")},
		logger:   zap.NewNop(),
	}
	msg := kafka.Message{
		Topic: "lesson-reminders",
		Value: []byte(`{"event_type":"booked","student_id":"stu-1"}`),
	}
	if err := d.Dispatch(context.Background(), msg); err == nil {
		t.Fatalf("expected retriable error, got nil")
	}
}

func TestTelegramDispatcher_RetriableSendError(t *testing.T) {
	sender := &recordingSender{err: errRetriableSend}
	d := &telegramDispatcher{
		tg:       sender,
		resolver: stubResolver{chatID: 7},
		logger:   zap.NewNop(),
	}
	msg := kafka.Message{
		Topic: "lesson-reminders",
		Value: []byte(`{"event_type":"booked","student_id":"stu-1"}`),
	}
	err := d.Dispatch(context.Background(), msg)
	if !errors.Is(err, errRetriableSend) {
		t.Fatalf("expected errRetriableSend, got %v", err)
	}
}

func TestTelegramDispatcher_TerminalSendError_Commits(t *testing.T) {
	sender := &recordingSender{err: errors.New("bot was blocked by user")}
	d := &telegramDispatcher{
		tg:       sender,
		resolver: stubResolver{chatID: 7},
		logger:   zap.NewNop(),
	}
	msg := kafka.Message{
		Topic: "lesson-reminders",
		Value: []byte(`{"event_type":"booked","student_id":"stu-1"}`),
	}
	if err := d.Dispatch(context.Background(), msg); err != nil {
		t.Fatalf("expected nil (commit), got %v", err)
	}
}
