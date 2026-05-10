package service

import (
	"context"

	"github.com/google/uuid"

	"homework_service/internal/domain"
)

type UserClient interface {
	IsPair(ctx context.Context, tutorID, studentID uuid.UUID) (bool, error)
}

type FileClient interface {
	GetFileURL(ctx context.Context, fileID uuid.UUID) (string, error)
	// EnsureFileExists verifies a file_id refers to an existing, accessible file
	// in file_service. Implementations MUST return ErrInvalidFileID for
	// NotFound/PermissionDenied responses from file_service.
	EnsureFileExists(ctx context.Context, fileID uuid.UUID) error
}

// SubmissionRepo is the subset of repository.SubmissionRepository used by
// submission/feedback services. Defined as an interface to enable mocking.
type SubmissionRepo interface {
	Create(ctx context.Context, submission *domain.Submission) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Submission, error)
	ListByAssignment(ctx context.Context, assignmentID uuid.UUID) ([]*domain.Submission, error)
}

// FeedbackRepo is the subset of repository.FeedbackRepository used by the
// feedback / submission services.
type FeedbackRepo interface {
	Create(ctx context.Context, feedback *domain.Feedback) error
	Update(ctx context.Context, feedback *domain.Feedback) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Feedback, error)
	ListByAssignment(ctx context.Context, assignmentID uuid.UUID) ([]*domain.Feedback, error)
	HasFeedbackForAssignmentStudent(ctx context.Context, assignmentID, studentID uuid.UUID) (bool, error)
}
