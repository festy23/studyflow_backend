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

// fakeAssignmentRepo is a hand-rolled stub satisfying AssignmentRepo. Phase 4
// will migrate the suite to go.uber.org/mock-generated mocks; for now this
// keeps the test self-contained without pulling in additional generated code.
type fakeAssignmentRepo struct {
	getByID     func(ctx context.Context, id uuid.UUID) (*domain.Assignment, error)
	create      func(ctx context.Context, a *domain.Assignment) error
	update      func(ctx context.Context, a *domain.Assignment) error
	deleteFn    func(ctx context.Context, id uuid.UUID) error
	listByFltr  func(ctx context.Context, f domain.AssignmentFilter) ([]*domain.Assignment, error)
	updateCalls int
	createCalls int
}

func (f *fakeAssignmentRepo) Create(ctx context.Context, a *domain.Assignment) error {
	f.createCalls++
	if f.create != nil {
		return f.create(ctx, a)
	}
	return nil
}
func (f *fakeAssignmentRepo) Update(ctx context.Context, a *domain.Assignment) error {
	f.updateCalls++
	if f.update != nil {
		return f.update(ctx, a)
	}
	return nil
}
func (f *fakeAssignmentRepo) Delete(ctx context.Context, id uuid.UUID) error {
	if f.deleteFn != nil {
		return f.deleteFn(ctx, id)
	}
	return nil
}
func (f *fakeAssignmentRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Assignment, error) {
	if f.getByID != nil {
		return f.getByID(ctx, id)
	}
	return nil, repository.ErrNotFound
}
func (f *fakeAssignmentRepo) ListByFilter(ctx context.Context, fltr domain.AssignmentFilter) ([]*domain.Assignment, error) {
	if f.listByFltr != nil {
		return f.listByFltr(ctx, fltr)
	}
	return nil, nil
}

type fakeUserClient struct {
	isPair func(ctx context.Context, tutorID, studentID uuid.UUID) (bool, error)
}

func (f *fakeUserClient) IsPair(ctx context.Context, tutorID, studentID uuid.UUID) (bool, error) {
	if f.isPair != nil {
		return f.isPair(ctx, tutorID, studentID)
	}
	return true, nil
}

type fakeFileClient struct {
	ensure func(ctx context.Context, fileID uuid.UUID) error
}

func (fakeFileClient) GetFileURL(ctx context.Context, fileID uuid.UUID) (string, error) {
	return "", nil
}

func (f fakeFileClient) EnsureFileExists(ctx context.Context, fileID uuid.UUID) error {
	if f.ensure != nil {
		return f.ensure(ctx, fileID)
	}
	return nil
}

func tutorCtx(userID string) context.Context {
	ctx := context.Background()
	ctx = ctxdata.WithUserID(ctx, userID)
	ctx = ctxdata.WithUserRole(ctx, "tutor")
	return ctx
}

// BUG-015: CreateAssignment must bind TutorID to the authenticated caller.
func TestCreateAssignment_RejectsForeignTutorID(t *testing.T) {
	callerID := uuid.New()
	otherTutor := uuid.New()
	repo := &fakeAssignmentRepo{}
	user := &fakeUserClient{
		isPair: func(ctx context.Context, tutorID, studentID uuid.UUID) (bool, error) {
			t.Fatalf("IsPair must not be called when TutorID does not match caller")
			return false, nil
		},
	}
	svc := NewAssignmentService(repo, user, fakeFileClient{})

	req := &domain.Assignment{TutorID: otherTutor, StudentID: uuid.New()}
	_, err := svc.CreateAssignment(tutorCtx(callerID.String()), req)
	if !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("expected ErrPermissionDenied, got %v", err)
	}
	if repo.createCalls != 0 {
		t.Fatalf("repo.Create must not be called on auth failure (calls=%d)", repo.createCalls)
	}
}

func TestCreateAssignment_AllowsMatchingTutorID(t *testing.T) {
	callerID := uuid.New()
	repo := &fakeAssignmentRepo{}
	user := &fakeUserClient{}
	svc := NewAssignmentService(repo, user, fakeFileClient{})

	req := &domain.Assignment{TutorID: callerID, StudentID: uuid.New()}
	_, err := svc.CreateAssignment(tutorCtx(callerID.String()), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.createCalls != 1 {
		t.Fatalf("expected Create to be called once, got %d", repo.createCalls)
	}
}

// BUG-016: UpdateAssignment must fetch the existing record and compare its
// TutorID — not the caller-supplied TutorID — against the authenticated user.
func TestUpdateAssignment_RejectsCallerSuppliedTutorID(t *testing.T) {
	callerID := uuid.New()         // caller is logged in as this user
	realOwner := uuid.New()        // the actual TutorID in the DB
	assignmentID := uuid.New()

	repo := &fakeAssignmentRepo{
		getByID: func(ctx context.Context, id uuid.UUID) (*domain.Assignment, error) {
			return &domain.Assignment{
				ID:        assignmentID,
				TutorID:   realOwner,
				StudentID: uuid.New(),
			}, nil
		},
	}
	svc := NewAssignmentService(repo, &fakeUserClient{}, fakeFileClient{})

	// Caller spoofs TutorID to their own ID, but is NOT the real owner.
	bogus := &domain.Assignment{ID: assignmentID, TutorID: callerID}
	err := svc.UpdateAssignment(tutorCtx(callerID.String()), bogus)
	if !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("expected ErrPermissionDenied, got %v", err)
	}
	if repo.updateCalls != 0 {
		t.Fatalf("repo.Update must not be called on auth failure (calls=%d)", repo.updateCalls)
	}
}

func TestUpdateAssignment_AllowsRealOwner(t *testing.T) {
	owner := uuid.New()
	assignmentID := uuid.New()
	repo := &fakeAssignmentRepo{
		getByID: func(ctx context.Context, id uuid.UUID) (*domain.Assignment, error) {
			return &domain.Assignment{ID: assignmentID, TutorID: owner, StudentID: uuid.New()}, nil
		},
	}
	svc := NewAssignmentService(repo, &fakeUserClient{}, fakeFileClient{})

	err := svc.UpdateAssignment(tutorCtx(owner.String()), &domain.Assignment{ID: assignmentID, TutorID: owner})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.updateCalls != 1 {
		t.Fatalf("expected Update to be called once, got %d", repo.updateCalls)
	}
}

func TestUpdateAssignment_NotFound(t *testing.T) {
	owner := uuid.New()
	repo := &fakeAssignmentRepo{
		getByID: func(ctx context.Context, id uuid.UUID) (*domain.Assignment, error) {
			return nil, repository.ErrNotFound
		},
	}
	svc := NewAssignmentService(repo, &fakeUserClient{}, fakeFileClient{})

	err := svc.UpdateAssignment(tutorCtx(owner.String()), &domain.Assignment{ID: uuid.New(), TutorID: owner})
	if !errors.Is(err, ErrAssignmentNotFound) {
		t.Fatalf("expected ErrAssignmentNotFound, got %v", err)
	}
}
