package domain

import (
	"github.com/google/uuid"
	"time"
)

type Assignment struct {
	ID          uuid.UUID
	TutorID     uuid.UUID
	StudentID   uuid.UUID
	Title       *string
	Description *string
	FileID      *uuid.UUID
	DueDate     *time.Time
	CreatedAt   time.Time
	EditedAt    time.Time
}

type AssignmentStatus string

const (
	AssignmentStatusUnspecified AssignmentStatus = "UNSPECIFIED"
	AssignmentStatusUnsent      AssignmentStatus = "UNSENT"
	AssignmentStatusUnreviewed  AssignmentStatus = "UNREVIEWED"
	AssignmentStatusReviewed    AssignmentStatus = "REVIEWED"
	AssignmentStatusOverdue     AssignmentStatus = "OVERDUE"
)

type AssignmentFilter struct {
	TutorID   uuid.UUID
	StudentID uuid.UUID
	Statuses  []AssignmentStatus
	Page      *int32
	PageSize  *int32
}

func (f *AssignmentFilter) Paginate() (limit, offset int) {
	if f.Page == nil {
		return 0, 0
	}
	p := int(*f.Page)
	if p < 1 {
		p = 1
	}
	ps := 20
	if f.PageSize != nil && *f.PageSize > 0 {
		ps = int(*f.PageSize)
	}
	if ps > 100 {
		ps = 100
	}
	return ps, (p - 1) * ps
}
