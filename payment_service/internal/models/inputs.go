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
	Page      *int32
	PageSize  *int32
}

func (i *ListReceiptsInput) Paginate() (limit, offset int) {
	if i.Page == nil {
		return 0, 0
	}
	p := int(*i.Page)
	if p < 1 {
		p = 1
	}
	ps := 20
	if i.PageSize != nil && *i.PageSize > 0 {
		ps = int(*i.PageSize)
	}
	if ps > 100 {
		ps = 100
	}
	return ps, (p - 1) * ps
}

type GetTutorAnalyticsInput struct {
	TutorID string
	From    *time.Time
	To      *time.Time
}
