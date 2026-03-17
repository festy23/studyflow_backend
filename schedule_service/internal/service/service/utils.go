package service

import (
	"common_library/ctxdata"
	"common_library/utils"
	"context"
	"errors"
	"fmt"
	"schedule_service/internal/database/repo"
	pb "schedule_service/pkg/api"
	"time"

	"google.golang.org/grpc/metadata"

	userpb "userservice/pkg/api"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type IUserClient interface {
	Close()
	GetTutorStudent(ctx context.Context, tutorID, studentID string) (*userpb.TutorStudent, error)
	ResolveTutorStudentContext(ctx context.Context, tutorID, studentID string) (*userpb.ResolvedTutorStudentContext, error)
}

type UserClient struct {
	conn   *grpc.ClientConn
	client userpb.UserServiceClient
}

func NewUserClient(adress string) (*UserClient, error) {
	conn, err := grpc.NewClient(adress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	return &UserClient{
		conn:   conn,
		client: userpb.NewUserServiceClient(conn),
	}, nil

}
func (c *UserClient) Close() {
	_ = c.conn.Close()
}

func (c *UserClient) GetTutorStudent(ctx context.Context, tutorID, studentID string) (*userpb.TutorStudent, error) {
	return utils.RetryWithBackoff(ctx, 3, 100*time.Millisecond, func() (*userpb.TutorStudent, error) {
		return c.client.GetTutorStudent(ctx, &userpb.GetTutorStudentRequest{
			TutorId:   tutorID,
			StudentId: studentID,
		})
	})
}

func (c *UserClient) ResolveTutorStudentContext(ctx context.Context, tutorID, studentID string) (*userpb.ResolvedTutorStudentContext, error) {
	return utils.RetryWithBackoff(ctx, 3, 100*time.Millisecond, func() (*userpb.ResolvedTutorStudentContext, error) {
		return c.client.ResolveTutorStudentContext(ctx, &userpb.ResolveTutorStudentContextRequest{
			TutorId:   tutorID,
			StudentId: studentID,
		})
	})
}

// applyResolvedDefaults fills nil fields on proto from the resolved context,
// implementing the documented precedence: lesson > tutor-student pair > tutor defaults.
// user_service already merges pair-overrides on top of tutor defaults; this function
// only fills lesson-level nils, so lesson-specific values always win.
func applyResolvedDefaults(proto *pb.Lesson, resolved *userpb.ResolvedTutorStudentContext) {
	if proto == nil || resolved == nil {
		return
	}
	if proto.ConnectionLink == nil && resolved.LessonConnectionLink != nil {
		v := *resolved.LessonConnectionLink
		proto.ConnectionLink = &v
	}
	if proto.PriceRub == nil && resolved.LessonPriceRub != nil {
		v := *resolved.LessonPriceRub
		proto.PriceRub = &v
	}
	if proto.PaymentInfo == nil && resolved.PaymentInfo != nil {
		v := *resolved.PaymentInfo
		proto.PaymentInfo = &v
	}
}

func convertrepoLessonToProto(lesson *repo.Lesson) *pb.Lesson {
	protoLesson := &pb.Lesson{
		Id:        lesson.ID,
		SlotId:    lesson.SlotID,
		StudentId: lesson.StudentID,
		Status:    lesson.Status,
		IsPaid:    lesson.IsPaid,
		CreatedAt: timestamppb.New(lesson.CreatedAt),
		EditedAt:  timestamppb.New(lesson.EditedAt),
	}

	if lesson.ConnectionLink != nil {
		protoLesson.ConnectionLink = lesson.ConnectionLink
	}

	if lesson.PriceRub != nil {
		protoLesson.PriceRub = lesson.PriceRub
	}

	if lesson.PaymentInfo != nil {
		protoLesson.PaymentInfo = lesson.PaymentInfo
	}

	return protoLesson
}

func createListLessonsResponse(lessons []repo.Lesson) *pb.ListLessonsResponse {
	protoLessons := make([]*pb.Lesson, 0, len(lessons))

	for _, lesson := range lessons {
		lessonCopy := lesson
		protoLesson := convertrepoLessonToProto(&lessonCopy)
		protoLessons = append(protoLessons, protoLesson)
	}

	return &pb.ListLessonsResponse{
		Lessons: protoLessons,
	}
}

// parseFromTo parses optional RFC3339 from/to strings into *time.Time.
// Returns codes.InvalidArgument if either string is non-nil but not valid RFC3339.
func parseFromTo(fromStr, toStr *string) (*time.Time, *time.Time, error) {
	var from, to *time.Time
	if fromStr != nil {
		t, err := time.Parse(time.RFC3339, *fromStr)
		if err != nil {
			return nil, nil, status.Errorf(codes.InvalidArgument, "invalid from: %v", err)
		}
		from = &t
	}
	if toStr != nil {
		t, err := time.Parse(time.RFC3339, *toStr)
		if err != nil {
			return nil, nil, status.Errorf(codes.InvalidArgument, "invalid to: %v", err)
		}
		to = &t
	}
	return from, to, nil
}

func validateTimeRange(start, end time.Time) bool {
	return start.Before(end)
}

func (s *ScheduleServer) ValidateTutorStudentPair(ctx context.Context, tutorID, studentID string) (bool, error) {
	currentUserID, ok := ctxdata.GetUserID(ctx)
	if !ok {
		return false, errors.New("user ID not found in context")
	}

	currentUserRole, ok := ctxdata.GetUserRole(ctx)
	if !ok {
		return false, errors.New("user role not found in context")
	}

	reqCtx := metadata.NewOutgoingContext(ctx, metadata.Pairs("x-user-id", currentUserID, "x-user-role", currentUserRole))
	tutorStudent, err := s.UserClient.GetTutorStudent(reqCtx, tutorID, studentID)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return false, nil
		}
		return false, fmt.Errorf("failed to verify tutor-student pair: %w", err)
	}
	if tutorStudent.GetStatus() != "active" {
		return false, nil
	}

	switch currentUserRole {
	case "tutor":
		return currentUserID == tutorID, nil
	case "student":
		return currentUserID == studentID, nil
	default:
		return false, nil
	}

}
func IsTutor(ctx context.Context, userID string) (bool, error) {
	currentUserID, ok := ctxdata.GetUserID(ctx)
	if !ok {
		return false, errors.New("user ID not found in context")
	}

	if currentUserID != userID {
		return false, nil
	}

	role, ok := ctxdata.GetUserRole(ctx)
	if !ok {
		return false, errors.New("user role not found in context")
	}

	return role == "tutor", nil
}
