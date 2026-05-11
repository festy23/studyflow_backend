package domain

import (
	"github.com/google/uuid"
	"time"
)

type Feedback struct {
	ID           uuid.UUID
	SubmissionID uuid.UUID
	FileID       *uuid.UUID
	Comment      *string
	Grade        *int32
	CreatedAt    time.Time
	EditedAt     time.Time
}
