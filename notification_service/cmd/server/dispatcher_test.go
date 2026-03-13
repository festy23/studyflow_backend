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

func TestBuildMessages_LessonReminders(t *testing.T) {
	t.Run("booked notifies student and tutor", func(t *testing.T) {
		ns, ok := buildMessages("lesson-reminders", map[string]any{
			"event_type": "booked",
			"student_id": "stu-1",
			"tutor_id":   "tut-1",
			"starts_at":  "2026-05-12T10:00:00Z",
		})
		if !ok || len(ns) != 2 {
			t.Fatalf("expected 2 notifications, got ok=%v len=%d", ok, len(ns))
		}
		if ns[0].userID != "stu-1" || !strings.Contains(ns[0].text, "booked") {
			t.Fatalf("unexpected student notification: %+v", ns[0])
		}
		if ns[1].userID != "tut-1" || !strings.Contains(ns[1].text, "booked") {
			t.Fatalf("unexpected tutor notification: %+v", ns[1])
		}
	})

	t.Run("cancelled: student and tutor get different texts", func(t *testing.T) {
		ns, ok := buildMessages("lesson-reminders", map[string]any{
			"event_type": "cancelled",
			"student_id": "stu-1",
			"tutor_id":   "tut-1",
		})
		if !ok || len(ns) != 2 {
			t.Fatalf("expected 2 notifications, got ok=%v len=%d", ok, len(ns))
		}
		if !strings.Contains(ns[0].text, "cancelled") {
			t.Fatalf("expected 'cancelled' in student text, got %q", ns[0].text)
		}
		if !strings.Contains(ns[1].text, "cancelled") {
			t.Fatalf("expected 'cancelled' in tutor text, got %q", ns[1].text)
		}
	})

	t.Run("missing student_id is not deliverable", func(t *testing.T) {
		_, ok := buildMessages("lesson-reminders", map[string]any{"event_type": "booked", "tutor_id": "tut-1"})
		if ok {
			t.Fatalf("expected ok=false for missing student_id")
		}
	})

	t.Run("missing tutor_id is not deliverable", func(t *testing.T) {
		_, ok := buildMessages("lesson-reminders", map[string]any{"event_type": "booked", "student_id": "stu-1"})
		if ok {
			t.Fatalf("expected ok=false for missing tutor_id")
		}
	})
}

func TestBuildMessages_AssignmentReminders(t *testing.T) {
	t.Run("with tutor_id sends to both", func(t *testing.T) {
		ns, ok := buildMessages("assignment-reminders", map[string]any{
			"student_id": "stu-1",
			"tutor_id":   "tut-1",
			"title":      "Algebra worksheet",
			"due_date":   "2026-05-15T18:00:00Z",
		})
		if !ok || len(ns) != 2 {
			t.Fatalf("expected 2 notifications, got ok=%v len=%d", ok, len(ns))
		}
		if ns[0].userID != "stu-1" || !strings.Contains(ns[0].text, "Algebra worksheet") {
			t.Fatalf("unexpected student notification: %+v", ns[0])
		}
		if ns[1].userID != "tut-1" || !strings.Contains(ns[1].text, "Algebra worksheet") {
			t.Fatalf("unexpected tutor notification: %+v", ns[1])
		}
	})

	t.Run("without tutor_id sends to student only", func(t *testing.T) {
		ns, ok := buildMessages("assignment-reminders", map[string]any{
			"student_id": "stu-1",
			"title":      "hw",
		})
		if !ok || len(ns) != 1 || ns[0].userID != "stu-1" {
			t.Fatalf("expected 1 student notification, got ok=%v len=%d", ok, len(ns))
		}
	})
}

func TestBuildMessages_UnknownTopic(t *testing.T) {
	if _, ok := buildMessages("unknown", map[string]any{"student_id": "x"}); ok {
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
		Value: []byte(`{"event_type":"booked","student_id":"stu-1","tutor_id":"tut-1","starts_at":"2026-05-12T10:00:00Z"}`),
	}
	if err := d.Dispatch(context.Background(), msg); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	// Two recipients: student and tutor.
	if sender.calls != 2 {
		t.Fatalf("expected 2 calls (student+tutor), got %d", sender.calls)
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
		Value: []byte(`{"student_id":"stu-1","tutor_id":"tut-1","title":"hw"}`),
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
		Value: []byte(`{"event_type":"booked","student_id":"stu-1","tutor_id":"tut-1"}`),
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
		Value: []byte(`{"event_type":"booked","student_id":"stu-1","tutor_id":"tut-1"}`),
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
		Value: []byte(`{"event_type":"booked","student_id":"stu-1","tutor_id":"tut-1"}`),
	}
	if err := d.Dispatch(context.Background(), msg); err != nil {
		t.Fatalf("expected nil (commit), got %v", err)
	}
}
