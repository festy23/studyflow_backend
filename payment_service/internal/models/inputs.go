package models

import (
	"time"

	"github.com/google/uuid"
)

type GetPaymentInfoInput struct {
	LessonId uuid.UUID
}

type SubmitPaymentReceiptInput struct {
	LessonId uuid.UUID
	FileId   uuid.UUID
}

type GetReceiptInput struct {
	ReceiptId uuid.UUID
}

type VerifyReceiptInput struct {
	ReceiptId uuid.UUID
}

type GetReceiptFileInput struct {
	ReceiptId uuid.UUID
}

type ListReceiptsInput struct {
	TutorID   string
	StudentID string
}

type GetTutorAnalyticsInput struct {
	TutorID string
	From    *time.Time
	To      *time.Time
}
