package handler

import (
	"context"
	"fmt"
	"testing"
	"userservice/internal/errdefs"
	"userservice/internal/model"
	pb "userservice/pkg/api"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// stubUserService implements UserService; only methods used in tests are non-trivial.
type stubUserService struct {
	updateTutorStudentErr error
}

func (s *stubUserService) RegisterViaTelegram(ctx context.Context, input *model.RegisterViaTelegramInput) (*model.User, error) {
	return nil, nil
}
func (s *stubUserService) Authorize(ctx context.Context, input *model.AuthorizeInput) (*model.User, error) {
	return nil, nil
}
func (s *stubUserService) GetMe(ctx context.Context) (*model.User, error) { return nil, nil }
func (s *stubUserService) GetUserPublic(ctx context.Context, id uuid.UUID) (*model.UserPublic, error) {
	return nil, nil
}
func (s *stubUserService) UpdateUser(ctx context.Context, id uuid.UUID, input *model.UpdateUserInput) (*model.User, error) {
	return nil, nil
}
func (s *stubUserService) GetTutorProfile(ctx context.Context, userId uuid.UUID) (*model.TutorProfile, error) {
	return nil, nil
}
func (s *stubUserService) UpdateTutorProfile(ctx context.Context, userId uuid.UUID, input *model.UpdateTutorProfileInput) (*model.TutorProfile, error) {
	return nil, nil
}
func (s *stubUserService) CreateTutorStudent(ctx context.Context, input *model.CreateTutorStudentInput) (*model.TutorStudent, error) {
	return nil, nil
}
func (s *stubUserService) GetTutorStudent(ctx context.Context, tutorId uuid.UUID, studentId uuid.UUID) (*model.TutorStudent, error) {
	return nil, nil
}
func (s *stubUserService) UpdateTutorStudent(ctx context.Context, tutorId uuid.UUID, studentId uuid.UUID, input *model.UpdateTutorStudentInput) (*model.TutorStudent, error) {
	return nil, s.updateTutorStudentErr
}
func (s *stubUserService) DeleteTutorStudent(ctx context.Context, tutorId uuid.UUID, studentId uuid.UUID) error {
	return nil
}
func (s *stubUserService) ListTutorStudents(ctx context.Context, tutorId uuid.UUID) ([]*model.TutorStudent, error) {
	return nil, nil
}
func (s *stubUserService) ListTutorStudentsForStudent(ctx context.Context, studentId uuid.UUID) ([]*model.TutorStudent, error) {
	return nil, nil
}
func (s *stubUserService) ResolveTutorStudentContext(ctx context.Context, tutorId uuid.UUID, studentId uuid.UUID) (*model.TutorStudentContext, error) {
	return nil, nil
}
func (s *stubUserService) AcceptInvitationFromTutor(ctx context.Context, tutorId uuid.UUID) error {
	return nil
}

func TestUpdateTutorStudent_NotFoundIsMappedToNotFound(t *testing.T) {
	svc := &stubUserService{updateTutorStudentErr: fmt.Errorf("wrap: %w", errdefs.ErrNotFound)}
	h := NewUserServiceServer(svc)

	_, err := h.UpdateTutorStudent(context.Background(), &pb.UpdateTutorStudentRequest{
		TutorId:   uuid.NewString(),
		StudentId: uuid.NewString(),
	})
	if err == nil {
		t.Fatal("expected error")
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("not a status err: %v", err)
	}
	if st.Code() != codes.NotFound {
		t.Fatalf("expected NotFound, got %s: %v", st.Code(), err)
	}
}

func TestUpdateTutorStudent_ValidationMappedToInvalidArgument(t *testing.T) {
	svc := &stubUserService{updateTutorStudentErr: fmt.Errorf("wrap: %w", errdefs.ErrValidation)}
	h := NewUserServiceServer(svc)

	_, err := h.UpdateTutorStudent(context.Background(), &pb.UpdateTutorStudentRequest{
		TutorId:   uuid.NewString(),
		StudentId: uuid.NewString(),
	})
	if err == nil {
		t.Fatal("expected error")
	}
	st, _ := status.FromError(err)
	if st.Code() != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %s", st.Code())
	}
}
