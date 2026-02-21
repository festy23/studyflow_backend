package main

import (
	"os"
	"testing"

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
		os.Setenv("NOTIFICATION_TEST_EMPTY_VAR", "")
		defer os.Unsetenv("NOTIFICATION_TEST_EMPTY_VAR")
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

func TestProcessMessage(t *testing.T) {
	logger := zap.NewNop()

	t.Run("valid JSON payload", func(t *testing.T) {
		msg := kafka.Message{
			Topic:     "lesson-reminders",
			Partition: 0,
			Offset:    42,
			Value:     []byte(`{"lesson_id":"abc","event_type":"booked"}`),
		}
		// Should not panic
		processMessage(logger, msg)
	})

	t.Run("invalid JSON payload", func(t *testing.T) {
		msg := kafka.Message{
			Topic: "lesson-reminders",
			Value: []byte("not-json"),
		}
		// Should not panic
		processMessage(logger, msg)
	})

	t.Run("empty payload", func(t *testing.T) {
		msg := kafka.Message{
			Topic: "test-topic",
			Value: []byte{},
		}
		processMessage(logger, msg)
	})
}
