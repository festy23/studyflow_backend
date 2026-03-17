package worker

import (
	"context"
	"strings"
	"sync"
	"time"

	"common_library/logging"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/timestamppb"
	schedulepb "schedule_service/pkg/api"
)

type ReminderRepository interface {
	PaymentReminderSent(ctx context.Context, lessonID uuid.UUID) (bool, error)
	MarkPaymentReminderSent(ctx context.Context, lessonID uuid.UUID) error
}

type ScheduleClient interface {
	ListCompletedUnpaidLessons(ctx context.Context, req *schedulepb.ListCompletedUnpaidLessonsRequest, opts ...grpc.CallOption) (*schedulepb.ListLessonsResponse, error)
	GetSlot(ctx context.Context, req *schedulepb.GetSlotRequest, opts ...grpc.CallOption) (*schedulepb.Slot, error)
}

type ReminderProducer interface {
	Send(ctx context.Context, topic string, message any) error
}

type PaymentReminderWorker struct {
	repo           ReminderRepository
	scheduleClient ScheduleClient
	producer       ReminderProducer
	topic          string
	interval       time.Duration
	remindAfter    time.Duration
	logger         *logging.Logger
}

func NewPaymentReminderWorker(repo ReminderRepository, scheduleClient ScheduleClient, producer ReminderProducer, topic string, interval, remindAfter time.Duration, logger *logging.Logger) *PaymentReminderWorker {
	return &PaymentReminderWorker{
		repo:           repo,
		scheduleClient: scheduleClient,
		producer:       producer,
		topic:          topic,
		interval:       interval,
		remindAfter:    remindAfter,
		logger:         logger,
	}
}

func (w *PaymentReminderWorker) Start(ctx context.Context, wg *sync.WaitGroup) {
	if w.interval <= 0 || w.remindAfter <= 0 || strings.TrimSpace(w.topic) == "" || w.producer == nil {
		if w.logger != nil {
			w.logger.Warn(ctx, "payment reminder worker disabled")
		}
		return
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(w.interval)
		defer ticker.Stop()

		w.process(ctx)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				w.process(ctx)
			}
		}
	}()
}

func (w *PaymentReminderWorker) process(ctx context.Context) {
	cutoff := time.Now().Add(-w.remindAfter)
	reqCtx := metadata.NewOutgoingContext(ctx, metadata.Pairs(
		"x-user-id", "payment_service",
		"x-user-role", "service",
	))
	resp, err := w.scheduleClient.ListCompletedUnpaidLessons(reqCtx, &schedulepb.ListCompletedUnpaidLessonsRequest{
		Before: timestamppb.New(cutoff),
	})
	if err != nil {
		w.logError(ctx, "failed to list unpaid lessons", err)
		return
	}

	for _, lesson := range resp.GetLessons() {
		lessonID, err := uuid.Parse(lesson.GetId())
		if err != nil {
			w.logError(ctx, "invalid lesson id in schedule response", err)
			continue
		}
		sent, err := w.repo.PaymentReminderSent(ctx, lessonID)
		if err != nil {
			w.logError(ctx, "failed to check payment reminder state", err)
			continue
		}
		if sent {
			continue
		}

		slot, err := w.scheduleClient.GetSlot(reqCtx, &schedulepb.GetSlotRequest{Id: lesson.GetSlotId()})
		if err != nil {
			w.logError(ctx, "failed to resolve lesson slot", err)
			continue
		}

		message := map[string]any{
			"event_type": "payment_due",
			"lesson_id":  lesson.GetId(),
			"slot_id":    lesson.GetSlotId(),
			"student_id": lesson.GetStudentId(),
			"tutor_id":   slot.GetTutorId(),
			"price_rub":  lesson.GetPriceRub(),
		}
		if err := w.producer.Send(ctx, w.topic, message); err != nil {
			w.logError(ctx, "failed to send payment reminder", err)
			continue
		}
		if err := w.repo.MarkPaymentReminderSent(ctx, lessonID); err != nil {
			w.logError(ctx, "failed to mark payment reminder sent", err)
		}
	}
}

func (w *PaymentReminderWorker) logError(ctx context.Context, msg string, err error) {
	if w.logger != nil {
		w.logger.Error(ctx, msg, zap.Error(err))
	}
}
