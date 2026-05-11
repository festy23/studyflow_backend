package service

import (
	"context"
	"errors"
	"fmt"

	"audit_service/internal/domain"
)

var (
	ErrInvalidArgument = errors.New("invalid argument")
	ErrNotFound        = errors.New("not found")
)

// IAuditRepo defines the repository interface needed by the audit service.
type IAuditRepo interface {
	CreateEvent(ctx context.Context, input *domain.RecordEventInput) (*domain.AuditEvent, error)
	ListEvents(ctx context.Context, filter *domain.ListEventsInput) ([]*domain.AuditEvent, int64, error)
}

// AuditService implements audit business logic.
type AuditService struct {
	repo IAuditRepo
}

// NewAuditService creates a new AuditService.
func NewAuditService(repo IAuditRepo) *AuditService {
	return &AuditService{repo: repo}
}

// RecordEvent records a new audit event. Empty required fields return ErrInvalidArgument.
func (s *AuditService) RecordEvent(ctx context.Context, input *domain.RecordEventInput) (*domain.AuditEvent, error) {
	if input == nil {
		return nil, fmt.Errorf("%w: input is nil", ErrInvalidArgument)
	}
	if input.UserID == "" {
		return nil, fmt.Errorf("%w: user_id is required", ErrInvalidArgument)
	}
	if input.Action == "" {
		return nil, fmt.Errorf("%w: action is required", ErrInvalidArgument)
	}
	if input.ResourceType == "" {
		return nil, fmt.Errorf("%w: resource_type is required", ErrInvalidArgument)
	}

	event, err := s.repo.CreateEvent(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("record event: %w", err)
	}
	return event, nil
}

// ListEvents returns audit events matching the filter with pagination.
// Default page_size is 20, max is 100.
func (s *AuditService) ListEvents(ctx context.Context, input *domain.ListEventsInput) ([]*domain.AuditEvent, int64, error) {
	if input == nil {
		input = &domain.ListEventsInput{}
	}
	pageSize := input.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	page := input.Page
	if page < 0 {
		page = 0
	}

	filter := &domain.ListEventsInput{
		UserID:       input.UserID,
		ResourceType: input.ResourceType,
		ResourceID:   input.ResourceID,
		Page:         page,
		PageSize:     pageSize,
	}

	events, totalCount, err := s.repo.ListEvents(ctx, filter)
	if err != nil {
		return nil, 0, fmt.Errorf("list events: %w", err)
	}
	return events, totalCount, nil
}
