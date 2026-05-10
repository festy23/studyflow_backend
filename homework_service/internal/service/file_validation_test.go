package service

import (
	"common_library/ctxdata"
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"homework_service/internal/domain"
	"homework_service/internal/repository"
)

// fakeSubmissionRepo / fakeFeedbackRepo are hand-rolled stubs satisfying
// SubmissionRepo and FeedbackRepo, matching the Phase 1 pattern.

type fakeSubmissionRepo struct {
	getByID      func(ctx context.Context, id uuid.UUID) (*domain.Submission, error)
	create       func(ctx context.Context, s *domain.Submission) error
	listByAssgnm func(ctx context.Context, id uuid.UUID) ([]*domain.Submission, error)
	createCalls  int
}

func (f *fakeSubmissionRepo) Create(ctx context.Context, s *domain.Submission) error {
	f.createCalls++
	if f.create != nil {
		return f.create(ctx, s)
	}
	return nil
}
func (f *fakeSubmissionRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Submission, error) {
	if f.getByID != nil {
		return f.getByID(ctx, id)
	}
	return nil, repository.ErrNotFound
}
func (f *fakeSubmissionRepo) ListByAssignment(ctx context.Context, id uuid.UUID) ([]*domain.Submission, error) {
	if f.listByAssgnm != nil {
		return f.listByAssgnm(ctx, id)
	}
	return nil, nil
}

type fakeFeedbackRepo struct {
	getByID      func(ctx context.Context, id uuid.UUID) (*domain.Feedback, error)
	create       func(ctx context.Context, f *domain.Feedback) error
	update       func(ctx context.Context, f *domain.Feedback) error
	listByAssgnm func(ctx context.Context, id uuid.UUID) ([]*domain.Feedback, error)
	hasFeedback  func(ctx context.Context, aID, sID uuid.UUID) (bool, error)
	createCalls  int
	updateCalls  int
}

func (f *fakeFeedbackRepo) Create(ctx context.Context, fb *domain.Feedback) error {
	f.createCalls++
	if f.create != nil {
		return f.create(ctx, fb)
	}
	return nil
}
func (f *fakeFeedbackRepo) Update(ctx context.Context, fb *domain.Feedback) error {
	f.updateCalls++
	if f.update != nil {
		return f.update(ctx, fb)
	}
	return nil
}
func (f *fakeFeedbackRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Feedback, error) {
	if f.getByID != nil {
		return f.getByID(ctx, id)
	}
	return nil, repository.ErrNotFound
}
func (f *fakeFeedbackRepo) ListByAssignment(ctx context.Context, id uuid.UUID) ([]*domain.Feedback, error) {
	if f.listByAssgnm != nil {
		return f.listByAssgnm(ctx, id)
	}
	return nil, nil
}
func (f *fakeFeedbackRepo) HasFeedbackForAssignmentStudent(ctx context.Context, aID, sID uuid.UUID) (bool, error) {
	if f.hasFeedback != nil {
		return f.hasFeedback(ctx, aID, sID)
	}
	return false, nil
}

func studentCtx(userID string) context.Context {
	ctx := context.Background()
	ctx = ctxdata.WithUserID(ctx, userID)
	ctx = ctxdata.WithUserRole(ctx, "student")
	return ctx
}

// BUG-017: CreateAssignment must reject unknown file_id.
func TestCreateAssignment_RejectsBogusFileID(t *testing.T) {
	caller := uuid.New()
	bogusFile := uuid.New()
	repo := &fakeAssignmentRepo{}
	user := &fakeUserClient{}
	files := fakeFileClient{ensure: func(ctx context.Context, id uuid.UUID) error {
		return ErrInvalidFileID
	}}
	svc := NewAssignmentService(repo, user, files)

	req := &domain.Assignment{TutorID: caller, StudentID: uuid.New(), FileID: &bogusFile}
	_, err := svc.CreateAssignment(tutorCtx(caller.String()), req)
	if !errors.Is(err, ErrInvalidFileID) {
		t.Fatalf("expected ErrInvalidFileID, got %v", err)
	}
	if repo.createCalls != 0 {
		t.Fatalf("repo.Create must not be called when file_id is invalid (calls=%d)", repo.createCalls)
	}
}

// BUG-017: CreateSubmission must reject unknown file_id.
func TestCreateSubmission_RejectsBogusFileID(t *testing.T) {
	student := uuid.New()
	assignmentID := uuid.New()
	bogusFile := uuid.New()
	aRepo := &fakeAssignmentRepo{
		getByID: func(ctx context.Context, id uuid.UUID) (*domain.Assignment, error) {
			return &domain.Assignment{ID: assignmentID, StudentID: student, TutorID: uuid.New()}, nil
		},
	}
	sRepo := &fakeSubmissionRepo{}
	fRepo := &fakeFeedbackRepo{}
	files := fakeFileClient{ensure: func(ctx context.Context, id uuid.UUID) error {
		return ErrInvalidFileID
	}}
	svc := NewSubmissionService(sRepo, aRepo, fRepo, files)

	_, err := svc.CreateSubmission(studentCtx(student.String()),
		&domain.Submission{AssignmentID: assignmentID, FileID: &bogusFile})
	if !errors.Is(err, ErrInvalidFileID) {
		t.Fatalf("expected ErrInvalidFileID, got %v", err)
	}
	if sRepo.createCalls != 0 {
		t.Fatalf("submission repo.Create must not be called (calls=%d)", sRepo.createCalls)
	}
}

// BUG-038/036: CreateSubmission must reject when feedback already exists.
func TestCreateSubmission_RejectsWhenAlreadyReviewed(t *testing.T) {
	student := uuid.New()
	assignmentID := uuid.New()
	aRepo := &fakeAssignmentRepo{
		getByID: func(ctx context.Context, id uuid.UUID) (*domain.Assignment, error) {
			return &domain.Assignment{ID: assignmentID, StudentID: student, TutorID: uuid.New()}, nil
		},
	}
	sRepo := &fakeSubmissionRepo{}
	fRepo := &fakeFeedbackRepo{
		hasFeedback: func(ctx context.Context, aID, sID uuid.UUID) (bool, error) { return true, nil },
	}
	svc := NewSubmissionService(sRepo, aRepo, fRepo, fakeFileClient{})

	_, err := svc.CreateSubmission(studentCtx(student.String()),
		&domain.Submission{AssignmentID: assignmentID})
	if !errors.Is(err, ErrAssignmentReviewed) {
		t.Fatalf("expected ErrAssignmentReviewed, got %v", err)
	}
	if sRepo.createCalls != 0 {
		t.Fatalf("repo.Create must not be called when assignment is reviewed (calls=%d)", sRepo.createCalls)
	}
}

// BUG-017: CreateFeedback must reject unknown file_id.
func TestCreateFeedback_RejectsBogusFileID(t *testing.T) {
	tutor := uuid.New()
	student := uuid.New()
	subID := uuid.New()
	assID := uuid.New()
	bogusFile := uuid.New()

	aRepo := &fakeAssignmentRepo{
		getByID: func(ctx context.Context, id uuid.UUID) (*domain.Assignment, error) {
			return &domain.Assignment{ID: assID, TutorID: tutor, StudentID: student}, nil
		},
	}
	sRepo := &fakeSubmissionRepo{
		getByID: func(ctx context.Context, id uuid.UUID) (*domain.Submission, error) {
			return &domain.Submission{ID: subID, AssignmentID: assID}, nil
		},
	}
	fRepo := &fakeFeedbackRepo{}
	files := fakeFileClient{ensure: func(ctx context.Context, id uuid.UUID) error {
		return ErrInvalidFileID
	}}
	svc := NewFeedbackService(fRepo, sRepo, aRepo, files)

	_, err := svc.CreateFeedback(tutorCtx(tutor.String()),
		&domain.Feedback{SubmissionID: subID, FileID: &bogusFile})
	if !errors.Is(err, ErrInvalidFileID) {
		t.Fatalf("expected ErrInvalidFileID, got %v", err)
	}
	if fRepo.createCalls != 0 {
		t.Fatalf("feedback repo.Create must not be called (calls=%d)", fRepo.createCalls)
	}
}

// BUG-017: UpdateFeedback must reject unknown file_id.
func TestUpdateFeedback_RejectsBogusFileID(t *testing.T) {
	tutor := uuid.New()
	student := uuid.New()
	fbID := uuid.New()
	subID := uuid.New()
	assID := uuid.New()
	bogusFile := uuid.New()

	aRepo := &fakeAssignmentRepo{
		getByID: func(ctx context.Context, id uuid.UUID) (*domain.Assignment, error) {
			return &domain.Assignment{ID: assID, TutorID: tutor, StudentID: student}, nil
		},
	}
	sRepo := &fakeSubmissionRepo{
		getByID: func(ctx context.Context, id uuid.UUID) (*domain.Submission, error) {
			return &domain.Submission{ID: subID, AssignmentID: assID}, nil
		},
	}
	fRepo := &fakeFeedbackRepo{
		getByID: func(ctx context.Context, id uuid.UUID) (*domain.Feedback, error) {
			return &domain.Feedback{ID: fbID, SubmissionID: subID}, nil
		},
	}
	files := fakeFileClient{ensure: func(ctx context.Context, id uuid.UUID) error {
		return ErrInvalidFileID
	}}
	svc := NewFeedbackService(fRepo, sRepo, aRepo, files)

	_, err := svc.UpdateFeedback(tutorCtx(tutor.String()),
		&domain.Feedback{ID: fbID, FileID: &bogusFile})
	if !errors.Is(err, ErrInvalidFileID) {
		t.Fatalf("expected ErrInvalidFileID, got %v", err)
	}
	if fRepo.updateCalls != 0 {
		t.Fatalf("feedback repo.Update must not be called (calls=%d)", fRepo.updateCalls)
	}
}
