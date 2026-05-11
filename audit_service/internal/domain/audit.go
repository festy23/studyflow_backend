package domain

import (
	"time"

	"github.com/google/uuid"
)

// AuditEvent represents a single audit log entry.
type AuditEvent struct {
	ID           uuid.UUID `db:"id"`
	UserID       string    `db:"user_id"`
	Action       string    `db:"action"`
	ResourceType string    `db:"resource_type"`
	ResourceID   *string   `db:"resource_id"`
	Details      *string   `db:"details"`
	IPAddress    *string   `db:"ip_address"`
	CreatedAt    time.Time `db:"created_at"`
}

// RecordEventInput contains the data needed to record a new audit event.
type RecordEventInput struct {
	UserID       string
	Action       string
	ResourceType string
	ResourceID   *string
	Details      *string
	IPAddress    *string
}

// ListEventsInput contains filtering and pagination parameters for listing events.
type ListEventsInput struct {
	UserID       string
	ResourceType string
	ResourceID   string
	Page         int32
	PageSize     int32
}
