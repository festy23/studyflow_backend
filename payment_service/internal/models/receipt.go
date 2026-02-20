package models

import (
	"time"

	"github.com/google/uuid"
)

type PaymentReceipt struct {
	ID         uuid.UUID `db:"id"`
	LessonID   uuid.UUID `db:"lesson_id"`
	FileID     uuid.UUID `db:"file_id"`
	IsVerified bool      `db:"is_verified"`
	CreatedAt  time.Time `db:"created_at"`
	EditedAt   time.Time `db:"edited_at"`
}

type PaymentReceiptCreateInput struct {
	ID         uuid.UUID
	LessonID   uuid.UUID
	FileID     uuid.UUID
	IsVerified bool
}

type PaymentReceiptUpdateInput struct {
	ID         uuid.UUID
	IsVerified bool
}

type ReceiptFileUrl struct {
	URL string
}
