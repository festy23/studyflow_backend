package main

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"homework_service/internal/domain"
	"homework_service/pkg/logger"
)

type fakeReminderRepo struct {
	assignments []*domain.Assignment
	marked      []uuid.UUID
}

func (f *fakeReminderRepo) FindAssignmentsDueSoon(_ context.Context, _ time.Duration) ([]*domain.Assignment, error) {
	return f.assignments, nil
}

func (f *fakeReminderRepo) MarkReminderSent(_ context.Context, id uuid.UUID) error {
	f.marked = append(f.marked, id)
	return nil
}

type fakeProducer struct{ sent int }

func (f *fakeProducer) Send(_ context.Context, _ string, _ interface{}) error {
	f.sent++
	return nil
}

// BUG-021: every successful Kafka send must be followed by MarkReminderSent so
// the worker does not duplicate notifications across 1-minute ticks.
func TestReminderWorker_MarksReminderSentAfterSend(t *testing.T) {
	id := uuid.New()
	due := time.Now().Add(time.Hour)
	repo := &fakeReminderRepo{
		assignments: []*domain.Assignment{{
			ID:        id,
			TutorID:   uuid.New(),
			StudentID: uuid.New(),
			DueDate:   &due,
		}},
	}
	prod := &fakeProducer{}
	w := NewReminderWorker(repo, prod, logger.New())

	w.processReminders(context.Background())

	if prod.sent != 1 {
		t.Fatalf("expected 1 Kafka send, got %d", prod.sent)
	}
	if len(repo.marked) != 1 || repo.marked[0] != id {
		t.Fatalf("expected MarkReminderSent for %s, got %v", id, repo.marked)
	}
}
