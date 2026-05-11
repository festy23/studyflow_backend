package main

import (
	"context"
	"errors"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

func TestGetEnv(t *testing.T) {
	t.Run("returns fallback when env not set", func(t *testing.T) {
		got := getEnv("NOTIFICATION_TEST_UNSET_VAR_12345", "default-val")
		if got != "default-val" {
			t.Errorf("expected %q, got %q", "default-val", got)
		}
	})

	t.Run("returns env value when set", func(t *testing.T) {
		t.Setenv("NOTIFICATION_TEST_VAR", "custom-val")
		got := getEnv("NOTIFICATION_TEST_VAR", "default-val")
		if got != "custom-val" {
			t.Errorf("expected %q, got %q", "custom-val", got)
		}
	})

	t.Run("returns fallback when env is empty string", func(t *testing.T) {
		_ = os.Setenv("NOTIFICATION_TEST_EMPTY_VAR", "")
		defer func() { _ = os.Unsetenv("NOTIFICATION_TEST_EMPTY_VAR") }()
		got := getEnv("NOTIFICATION_TEST_EMPTY_VAR", "fallback")
		if got != "fallback" {
			t.Errorf("expected %q, got %q", "fallback", got)
		}
	})
}

func TestSplitAndTrim(t *testing.T) {
	tests := []struct {
		name string
		csv  string
		want []string
	}{
		{"single value", "kafka:9092", []string{"kafka:9092"}},
		{"multiple values", "broker1:9092,broker2:9092", []string{"broker1:9092", "broker2:9092"}},
		{"with spaces", " broker1:9092 , broker2:9092 ", []string{"broker1:9092", "broker2:9092"}},
		{"trailing comma", "broker1:9092,", []string{"broker1:9092"}},
		{"empty string", "", []string{}},
		{"only commas", ",,", []string{}},
		{"spaces only entries", " , , ", []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitAndTrim(tt.csv)
			if len(got) != len(tt.want) {
				t.Fatalf("len mismatch: got %d, want %d (%v vs %v)", len(got), len(tt.want), got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("index %d: got %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestTruncateBytes(t *testing.T) {
	t.Run("short data unchanged", func(t *testing.T) {
		data := []byte("hello")
		got := truncateBytes(data, 256)
		if string(got) != "hello" {
			t.Errorf("expected %q, got %q", "hello", string(got))
		}
	})

	t.Run("exact length unchanged", func(t *testing.T) {
		data := make([]byte, 256)
		got := truncateBytes(data, 256)
		if len(got) != 256 {
			t.Errorf("expected len 256, got %d", len(got))
		}
	})

	t.Run("long data truncated", func(t *testing.T) {
		data := make([]byte, 512)
		for i := range data {
			data[i] = byte(i % 256)
		}
		got := truncateBytes(data, 256)
		if len(got) != 256 {
			t.Errorf("expected len 256, got %d", len(got))
		}
	})

	t.Run("nil data", func(t *testing.T) {
		got := truncateBytes(nil, 256)
		if got != nil {
			t.Errorf("expected nil, got %v", got)
		}
	})
}

func TestLogOnlyDispatcher(t *testing.T) {
	logger := zap.NewNop()
	d := logOnlyDispatcher{logger: logger}
	ctx := context.Background()

	t.Run("valid JSON payload commits", func(t *testing.T) {
		msg := kafka.Message{
			Topic:  "lesson-reminders",
			Offset: 42,
			Value:  []byte(`{"lesson_id":"abc","event_type":"booked"}`),
		}
		if err := d.Dispatch(ctx, msg); err != nil {
			t.Errorf("expected nil, got %v", err)
		}
	})

	t.Run("malformed JSON commits (non-retriable)", func(t *testing.T) {
		msg := kafka.Message{Topic: "lesson-reminders", Value: []byte("not-json")}
		if err := d.Dispatch(ctx, msg); err != nil {
			t.Errorf("expected nil, got %v", err)
		}
	})
}

// fakeReader implements messageCommitter for testing the consumer loop.
type fakeReader struct {
	mu           sync.Mutex
	messages     []kafka.Message
	fetchIdx     int
	commits      []kafka.Message
	commitErr    error
	// blockOnEmpty: when no more messages, block until ctx is cancelled,
	// mimicking real kafka.Reader behavior.
	blockOnEmpty bool
	// observedCommitCtxErr captures ctx.Err() seen by CommitMessages.
	observedCommitCtxErr atomic.Value // error
}

func (f *fakeReader) FetchMessage(ctx context.Context) (kafka.Message, error) {
	f.mu.Lock()
	if f.fetchIdx < len(f.messages) {
		msg := f.messages[f.fetchIdx]
		f.fetchIdx++
		f.mu.Unlock()
		return msg, nil
	}
	f.mu.Unlock()
	if !f.blockOnEmpty {
		return kafka.Message{}, errors.New("no more messages")
	}
	<-ctx.Done()
	return kafka.Message{}, ctx.Err()
}

func (f *fakeReader) CommitMessages(ctx context.Context, msgs ...kafka.Message) error {
	f.observedCommitCtxErr.Store(errOrNil(ctx.Err()))
	if f.commitErr != nil {
		return f.commitErr
	}
	f.mu.Lock()
	f.commits = append(f.commits, msgs...)
	f.mu.Unlock()
	return nil
}

// errOrNil wraps so atomic.Value never stores untyped nil.
type errBox struct{ err error }

func errOrNil(err error) errBox { return errBox{err: err} }

// TestRunConsumer_GracefulDrain verifies that when SIGTERM (ctx cancel)
// occurs after a message has been fetched, the final commit still
// succeeds because it uses an independent deadline.
func TestRunConsumer_GracefulDrain(t *testing.T) {
	logger := zap.NewNop()
	fr := &fakeReader{
		messages: []kafka.Message{
			{Topic: "lesson-reminders", Partition: 0, Offset: 1, Value: []byte(`{"event_type":"booked"}`)},
		},
		blockOnEmpty: true,
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		runConsumer(ctx, logger, fr, logOnlyDispatcher{logger: logger})
		close(done)
	}()

	// Wait for the message to be processed and committed.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		fr.mu.Lock()
		n := len(fr.commits)
		fr.mu.Unlock()
		if n == 1 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	fr.mu.Lock()
	if len(fr.commits) != 1 {
		fr.mu.Unlock()
		t.Fatalf("expected 1 commit before shutdown, got %d", len(fr.commits))
	}
	if fr.commits[0].Offset != 1 {
		fr.mu.Unlock()
		t.Fatalf("expected offset 1 committed, got %d", fr.commits[0].Offset)
	}
	fr.mu.Unlock()

	// Verify commit context was NOT cancelled (independent deadline).
	if v := fr.observedCommitCtxErr.Load(); v != nil {
		if box, ok := v.(errBox); ok && box.err != nil {
			t.Errorf("expected commit ctx not cancelled, got %v", box.err)
		}
	}

	// Now cancel — consumer should exit cleanly without panicking and
	// without trying to commit a never-fetched message.
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("consumer did not shut down within 2s of cancellation")
	}
}

// TestRunConsumer_CancelDuringFetch verifies that cancelling ctx while
// the consumer is blocked in FetchMessage causes a clean exit with no
// commit attempt for a non-existent message.
func TestRunConsumer_CancelDuringFetch(t *testing.T) {
	logger := zap.NewNop()
	fr := &fakeReader{blockOnEmpty: true}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		runConsumer(ctx, logger, fr, logOnlyDispatcher{logger: logger})
		close(done)
	}()

	time.Sleep(50 * time.Millisecond) // let it enter FetchMessage
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("consumer did not exit on cancel")
	}

	fr.mu.Lock()
	defer fr.mu.Unlock()
	if len(fr.commits) != 0 {
		t.Errorf("expected no commits on cancel-during-fetch, got %d", len(fr.commits))
	}
}
