package main

import (
	"context"
	"time"

	"github.com/google/uuid"

	"homework_service/internal/domain"
	"homework_service/pkg/logger"
)

// reminderProducer is the slice of kafka.Producer used by the reminder worker.
type reminderProducer interface {
	Send(ctx context.Context, topic string, message interface{}) error
}

// reminderAssignmentRepo is the slice of repository.AssignmentRepository used
// by the reminder worker. It exists primarily to enable mocking in tests.
type reminderAssignmentRepo interface {
	FindAssignmentsDueSoon(ctx context.Context, duration time.Duration) ([]*domain.Assignment, error)
	MarkReminderSent(ctx context.Context, id uuid.UUID) error
}

type ReminderWorker struct {
	assignmentRepo reminderAssignmentRepo
	kafkaProducer  reminderProducer
	logger         *logger.Logger
	interval       time.Duration
}

func NewReminderWorker(
	assignmentRepo reminderAssignmentRepo,
	kafkaProducer reminderProducer,
	logger *logger.Logger,
) *ReminderWorker {
	return &ReminderWorker{
		assignmentRepo: assignmentRepo,
		kafkaProducer:  kafkaProducer,
		logger:         logger,
		interval:       time.Minute,
	}
}

func (w *ReminderWorker) Start(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			w.logger.Info("Reminder worker stopped")
			return
		case <-ticker.C:
			w.processReminders(ctx)
		}
	}
}

func (w *ReminderWorker) processReminders(ctx context.Context) {
	assignments, err := w.assignmentRepo.FindAssignmentsDueSoon(ctx, 24*time.Hour)
	if err != nil {
		w.logger.Errorf("Failed to get assignments due soon: %v", err)
		return
	}

	for _, assignment := range assignments {
		message := map[string]interface{}{
			"assignment_id": assignment.ID,
			"student_id":    assignment.StudentID,
			"tutor_id":      assignment.TutorID,
			"due_date":      assignment.DueDate,
			"title":         assignment.Title,
		}

		if err := w.kafkaProducer.Send(ctx, "assignment-reminders", message); err != nil {
			w.logger.Errorf("Failed to send reminder for assignment %s: %v", assignment.ID, err)
			continue
		}

		// BUG-021: stamp the row so this assignment is excluded from subsequent
		// 1-minute ticks within the same 24h window.
		if err := w.assignmentRepo.MarkReminderSent(ctx, assignment.ID); err != nil {
			w.logger.Errorf("Failed to mark reminder sent for assignment %s: %v", assignment.ID, err)
			continue
		}

		w.logger.Infof("Sent reminder for assignment %s", assignment.ID)
	}
}
