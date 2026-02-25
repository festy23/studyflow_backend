package service

import (
	"common_library/ctxdata"
	"context"
	"errors"
	"github.com/google/uuid"
	"time"

	"homework_service/internal/domain"
	"homework_service/internal/repository"
)

type AssignmentServiceInterface interface {
	CreateAssignment(ctx context.Context, req *domain.Assignment) (*domain.Assignment, error)
	GetAssignment(ctx context.Context, id uuid.UUID) (*domain.Assignment, error)
	UpdateAssignment(ctx context.Context, assignment *domain.Assignment) error
	DeleteAssignment(ctx context.Context, id uuid.UUID) error
	ListAssignmentsByTutor(ctx context.Context, tutorID uuid.UUID, statuses []domain.AssignmentStatus) ([]*domain.Assignment, error)
	ListAssignmentsByStudent(ctx context.Context, studentID uuid.UUID, statuses []domain.AssignmentStatus) ([]*domain.Assignment, error)
	ListAssignmentsByPair(ctx context.Context, tutorID uuid.UUID, studentID uuid.UUID, statuses []domain.AssignmentStatus) ([]*domain.Assignment, error)
	GetAssignmentFileURL(ctx context.Context, id uuid.UUID) (string, error)
}

// AssignmentRepo is the subset of repository.AssignmentRepository used by
// AssignmentService. It exists primarily to enable mocking in unit tests.
type AssignmentRepo interface {
	Create(ctx context.Context, assignment *domain.Assignment) error
	Update(ctx context.Context, assignment *domain.Assignment) error
	Delete(ctx context.Context, id uuid.UUID) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Assignment, error)
	ListByFilter(ctx context.Context, filter domain.AssignmentFilter) ([]*domain.Assignment, error)
}

type AssignmentService struct {
	assignmentRepo AssignmentRepo
	userClient     UserClient
	fileClient     FileClient
}

func NewAssignmentService(
	assignmentRepo AssignmentRepo,
	userClient UserClient,
	fileClient FileClient,
) *AssignmentService {
	return &AssignmentService{
		assignmentRepo: assignmentRepo,
		userClient:     userClient,
		fileClient:     fileClient,
	}
}

func (s *AssignmentService) CreateAssignment(ctx context.Context, req *domain.Assignment) (*domain.Assignment, error) {

	userRole, ok := ctxdata.GetUserRole(ctx)
	if !ok || userRole != "tutor" {
		return nil, ErrPermissionDenied
	}

	userID, ok := ctxdata.GetUserID(ctx)
	if !ok || req.TutorID.String() != userID {
		return nil, ErrPermissionDenied
	}

	isPair, err := s.userClient.IsPair(ctx, req.TutorID, req.StudentID)
	if err != nil {
		return nil, err
	}
	if !isPair {
		return nil, errors.New("not a tutor-student pair")
	}

	now := time.Now()
	assignment := &domain.Assignment{
		TutorID:     req.TutorID,
		StudentID:   req.StudentID,
		Title:       req.Title,
		Description: req.Description,
		FileID:      req.FileID,
		DueDate:     req.DueDate,
		CreatedAt:   now,
		EditedAt:    now,
	}

	err = s.assignmentRepo.Create(ctx, assignment)
	if err != nil {
		return nil, err
	}

	return assignment, nil
}

func (s *AssignmentService) GetAssignment(ctx context.Context, id uuid.UUID) (*domain.Assignment, error) {
	assignment, err := s.assignmentRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}

	userID, ok := ctxdata.GetUserID(ctx)
	if !ok {
		return nil, ErrPermissionDenied
	}

	if assignment.TutorID.String() != userID && assignment.StudentID.String() != userID {
		return nil, ErrPermissionDenied
	}

	return assignment, nil
}

func (s *AssignmentService) UpdateAssignment(ctx context.Context, assignment *domain.Assignment) error {
	userID, ok := ctxdata.GetUserID(ctx)
	if !ok {
		return ErrPermissionDenied
	}

	existing, err := s.assignmentRepo.GetByID(ctx, assignment.ID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrAssignmentNotFound
		}
		return err
	}

	if existing.TutorID.String() != userID {
		return ErrPermissionDenied
	}

	// Preserve immutable ownership fields from the DB record so that a
	// caller-supplied TutorID/StudentID cannot be used to re-target the row.
	assignment.TutorID = existing.TutorID
	assignment.StudentID = existing.StudentID

	return s.assignmentRepo.Update(ctx, assignment)
}

func (s *AssignmentService) DeleteAssignment(ctx context.Context, id uuid.UUID) error {
	assignment, err := s.assignmentRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	userID, ok := ctxdata.GetUserID(ctx)
	if !ok || assignment.TutorID.String() != userID {
		return ErrPermissionDenied
	}

	return s.assignmentRepo.Delete(ctx, id)
}

func (s *AssignmentService) ListAssignmentsByTutor(ctx context.Context, tutorID uuid.UUID, statuses []domain.AssignmentStatus) ([]*domain.Assignment, error) {
	userID, ok := ctxdata.GetUserID(ctx)
	if !ok || tutorID.String() != userID {
		return nil, ErrPermissionDenied
	}

	return s.assignmentRepo.ListByFilter(ctx, domain.AssignmentFilter{TutorID: tutorID, Statuses: statuses})
}

func (s *AssignmentService) ListAssignmentsByStudent(ctx context.Context, studentID uuid.UUID, statuses []domain.AssignmentStatus) ([]*domain.Assignment, error) {
	userID, ok := ctxdata.GetUserID(ctx)
	if !ok || studentID.String() != userID {
		return nil, ErrPermissionDenied
	}

	return s.assignmentRepo.ListByFilter(ctx, domain.AssignmentFilter{StudentID: studentID, Statuses: statuses})
}

func (s *AssignmentService) ListAssignmentsByPair(ctx context.Context, tutorID uuid.UUID, studentID uuid.UUID, statuses []domain.AssignmentStatus) ([]*domain.Assignment, error) {
	userID, ok := ctxdata.GetUserID(ctx)
	if !ok || (tutorID.String() != userID && studentID.String() != userID) {
		return nil, ErrPermissionDenied
	}

	return s.assignmentRepo.ListByFilter(ctx, domain.AssignmentFilter{TutorID: tutorID, StudentID: studentID, Statuses: statuses})
}

func (s *AssignmentService) GetAssignmentFileURL(ctx context.Context, id uuid.UUID) (string, error) {
	assignment, err := s.assignmentRepo.GetByID(ctx, id)
	if err != nil {
		return "", err
	}
	if assignment.FileID == nil {
		return "", ErrFileNotFound
	}
	url, err := s.fileClient.GetFileURL(ctx, *assignment.FileID)
	if err != nil {
		return "", err
	}
	return url, nil
}
