package domain

import (
	"time"

	"github.com/google/uuid"
)

type FAQ struct {
	ID        uuid.UUID
	Question  string
	Answer    string
	Category  *string
	SortOrder int32
	CreatedAt time.Time
	EditedAt  time.Time
}
