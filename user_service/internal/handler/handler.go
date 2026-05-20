package handler

import (
	"common_library/logging"
	"context"
	"errors"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
	"slices"
	"userservice/internal/errdefs"
	"userservice/internal/model"
	pb "userservice/pkg/api"
)

type UserService interface {
	RegisterViaTelegram(ctx context.Context, input *model.RegisterViaTelegramInput) (*model.User, error)
	Authorize(ctx context.Context, input *model.AuthorizeInput) (*model.User, error)
	GetMe(ctx context.Context) (*model.User, error)
	GetUserPublic(ctx context.Context, id uuid.UUID) (*model.UserPublic, error)
	UpdateUser(ctx context.Context, id uuid.UUID, input *model.UpdateUserInput) (*model.User, error)
	DeleteUser(ctx context.Context, id uuid.UUID) (*model.User, error)
	GetTutorProfile(ctx context.Context, userId uuid.UUID) (*model.TutorProfile, error)
	UpdateTutorProfile(ctx context.Context, userId uuid.UUID, input *model.UpdateTutorProfileInput) (*model.TutorProfile, error)
	CreateTutorStudent(ctx context.Context, input *model.CreateTutorStudentInput) (*model.TutorStudent, error)
	GetTutorStudent(ctx context.Context, tutorId uuid.UUID, studentId uuid.UUID) (*model.TutorStudent, error)
	GetTelegramChatId(ctx context.Context, userId uuid.UUID) (int64, error)
	UpdateTutorStudent(ctx context.Context, tutorId uuid.UUID, studentId uuid.UUID, input *model.UpdateTutorStudentInput) (*model.TutorStudent, error)
	DeleteTutorStudent(ctx context.Context, tutorId uuid.UUID, studentId uuid.UUID) error
	ListTutorStudents(ctx context.Context, tutorId uuid.UUID) ([]*model.TutorStudent, error)
	ListTutorStudentsForStudent(ctx context.Context, studentId uuid.UUID) ([]*model.TutorStudent, error)
	ResolveTutorStudentContext(ctx context.Context, tutorId uuid.UUID, studentId uuid.UUID) (*model.TutorStudentContext, error)
	AcceptInvitationFromTutor(ctx context.Context, tutorId uuid.UUID) error

	CreateInvitation(ctx context.Context) (*model.Invitation, error)
	ListInvitations(ctx context.Context) ([]*model.Invitation, error)
	RevokeInvitation(ctx context.Context, id uuid.UUID) error
	AcceptInvitation(ctx context.Context, token uuid.UUID) (*model.TutorStudent, error)
}

type UserServiceServer struct {
	pb.UnimplementedUserServiceServer
	service UserService
}

func NewUserServiceServer(userService UserService) *UserServiceServer {
	return &UserServiceServer{service: userService}
}

func (h *UserServiceServer) RegisterViaTelegram(ctx context.Context, req *pb.RegisterViaTelegramRequest) (*pb.User, error) {
	input := &model.RegisterViaTelegramInput{
		TelegramId: req.TelegramId,
		Role:       model.Role(req.Role),
		Username:   req.Username,
		FirstName:  req.FirstName,
		LastName:   req.LastName,
		Timezone:   req.Timezone,
	}
	logger, hasLogger := logging.GetFromContext(ctx)
	if hasLogger {
		logger.Info(ctx, "registering user",
			zap.String("role", string(input.Role)),
			zap.Stringp("timezone", input.Timezone),
		)
	}
	user, err := h.service.RegisterViaTelegram(ctx, input)
	if err != nil {
		return nil, mapError(err, errdefs.ErrAlreadyExists, errdefs.ErrValidation)
	}
	if hasLogger {
		logger.Info(ctx, "registered user", zap.String("user_id", user.Id.String()))
	}

	return toPbUser(user), nil
}

func (h *UserServiceServer) AuthorizeByAuthHeader(ctx context.Context, req *pb.AuthorizeByAuthHeaderRequest) (*pb.User, error) {
	input := &model.AuthorizeInput{
		AuthorizationHeader: req.GetAuthorizationHeader(),
	}

	user, err := h.service.Authorize(ctx, input)
	if err != nil {
		return nil, mapError(err, errdefs.ErrValidation, errdefs.ErrAuthentication, errdefs.ErrUserDeleted)
	}

	return toPbUser(user), nil
}

func (h *UserServiceServer) GetMe(ctx context.Context, _ *pb.Empty) (*pb.User, error) {
	user, err := h.service.GetMe(ctx)

	if err != nil {
		return nil, mapError(err, errdefs.ErrNotFound)
	}

	return toPbUser(user), nil
}

func (h *UserServiceServer) GetUser(ctx context.Context, req *pb.GetUserRequest) (*pb.UserPublic, error) {
	id, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}

	user, err := h.service.GetUserPublic(ctx, id)
	if err != nil {
		return nil, mapError(err, errdefs.ErrNotFound)
	}

	userPb := &pb.UserPublic{
		Id:        user.Id.String(),
		Role:      user.Role.String(),
		FirstName: user.FirstName,
		LastName:  user.LastName,
	}

	return userPb, nil
}

func (h *UserServiceServer) UpdateUser(ctx context.Context, req *pb.UpdateUserRequest) (*pb.User, error) {
	id, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}

	input := &model.UpdateUserInput{
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Timezone:  req.Timezone,
	}
	if req.Role != nil {
		role := model.Role(*req.Role)
		if !role.IsValid() {
			return nil, status.Errorf(codes.InvalidArgument, "invalid role: %s", *req.Role)
		}
		input.Role = &role
	}

	user, err := h.service.UpdateUser(ctx, id, input)

	if err != nil {
		return nil, mapError(err, errdefs.ErrNotFound, errdefs.ErrValidation, errdefs.ErrPermissionDenied)
	}

	return toPbUser(user), nil
}

func (h *UserServiceServer) DeleteUser(ctx context.Context, req *pb.DeleteUserRequest) (*pb.User, error) {
	id, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}

	user, err := h.service.DeleteUser(ctx, id)
	if err != nil {
		return nil, mapError(err, errdefs.ErrNotFound, errdefs.ErrPermissionDenied)
	}

	return toPbUser(user), nil
}

func (h *UserServiceServer) GetTutorProfileByUserId(ctx context.Context, req *pb.GetTutorProfileByUserIdRequest) (*pb.TutorProfile, error) {
	id, err := uuid.Parse(req.UserId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}

	profile, err := h.service.GetTutorProfile(ctx, id)

	if err != nil {
		return nil, mapError(err, errdefs.ErrNotFound, errdefs.ErrPermissionDenied)
	}

	return toPbTutorProfile(profile), nil
}

func (h *UserServiceServer) UpdateTutorProfile(ctx context.Context, req *pb.UpdateTutorProfileRequest) (*pb.TutorProfile, error) {
	id, err := uuid.Parse(req.UserId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}

	input := &model.UpdateTutorProfileInput{
		PaymentInfo:          req.PaymentInfo,
		LessonPriceRub:       req.LessonPriceRub,
		LessonConnectionLink: req.LessonConnectionLink,
	}

	profile, err := h.service.UpdateTutorProfile(ctx, id, input)
	if err != nil {
		return nil, mapError(err, errdefs.ErrNotFound, errdefs.ErrValidation, errdefs.ErrPermissionDenied)
	}

	return toPbTutorProfile(profile), nil
}

func (h *UserServiceServer) CreateTutorStudent(ctx context.Context, req *pb.CreateTutorStudentRequest) (*pb.TutorStudent, error) {
	tutorId, err := uuid.Parse(req.TutorId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}

	studentId, err := uuid.Parse(req.StudentId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}

	input := &model.CreateTutorStudentInput{
		TutorId:        tutorId,
		StudentId:      studentId,
		LessonPriceRub: req.LessonPriceRub,
	}

	tutorStudent, err := h.service.CreateTutorStudent(ctx, input)
	if err != nil {
		return nil, mapError(err, errdefs.ErrAlreadyExists, errdefs.ErrValidation, errdefs.ErrPermissionDenied)
	}

	return toPbTutorStudent(tutorStudent), nil
}

func (h *UserServiceServer) GetTelegramChatId(ctx context.Context, req *pb.GetTelegramChatIdRequest) (*pb.GetTelegramChatIdResponse, error) {
	userId, err := uuid.Parse(req.UserId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}
	telegramId, err := h.service.GetTelegramChatId(ctx, userId)
	if err != nil {
		return nil, mapError(err, errdefs.ErrNotFound)
	}
	return &pb.GetTelegramChatIdResponse{TelegramId: telegramId}, nil
}

func (h *UserServiceServer) GetTutorStudent(ctx context.Context, req *pb.GetTutorStudentRequest) (*pb.TutorStudent, error) {
	tutorId, err := uuid.Parse(req.TutorId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}
	studentId, err := uuid.Parse(req.StudentId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}

	tutorStudent, err := h.service.GetTutorStudent(ctx, tutorId, studentId)
	if err != nil {
		return nil, mapError(err, errdefs.ErrNotFound, errdefs.ErrPermissionDenied, errdefs.ErrAuthentication)
	}
	return toPbTutorStudent(tutorStudent), nil
}

func (h *UserServiceServer) UpdateTutorStudent(ctx context.Context, req *pb.UpdateTutorStudentRequest) (*pb.TutorStudent, error) {
	tutorId, err := uuid.Parse(req.TutorId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}

	studentId, err := uuid.Parse(req.StudentId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}

	input := &model.UpdateTutorStudentInput{
		LessonPriceRub:       req.LessonPriceRub,
		LessonConnectionLink: req.LessonConnectionLink,
	}

	if req.Status != nil {
		s, ok := model.TutorStudentStatusFromString(*req.Status)
		if !ok {
			return nil, status.Errorf(codes.InvalidArgument, "invalid status")
		}
		input.Status = &s
	}

	tutorStudent, err := h.service.UpdateTutorStudent(ctx, tutorId, studentId, input)
	if err != nil {
		return nil, mapError(err, errdefs.ErrNotFound, errdefs.ErrPermissionDenied, errdefs.ErrValidation)
	}

	return toPbTutorStudent(tutorStudent), nil
}

func (h *UserServiceServer) DeleteTutorStudent(ctx context.Context, req *pb.DeleteTutorStudentRequest) (*pb.Empty, error) {
	tutorId, err := uuid.Parse(req.TutorId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}

	studentId, err := uuid.Parse(req.StudentId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}

	err = h.service.DeleteTutorStudent(ctx, tutorId, studentId)
	if err != nil {
		return nil, mapError(err, errdefs.ErrNotFound, errdefs.ErrPermissionDenied)
	}

	return &pb.Empty{}, nil
}

func (h *UserServiceServer) ListTutorStudents(ctx context.Context, req *pb.ListTutorStudentsRequest) (*pb.ListTutorStudentsResponse, error) {
	tutorId, err := uuid.Parse(req.TutorId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}

	tutorStudents, err := h.service.ListTutorStudents(ctx, tutorId)
	if err != nil {
		return nil, mapError(err, errdefs.ErrNotFound, errdefs.ErrPermissionDenied)
	}

	resp := make([]*pb.TutorStudent, len(tutorStudents))
	for i, tutorStudent := range tutorStudents {
		resp[i] = toPbTutorStudent(tutorStudent)
	}

	return &pb.ListTutorStudentsResponse{Students: resp}, nil
}

func (h *UserServiceServer) ListTutorsForStudent(ctx context.Context, req *pb.ListTutorsForStudentRequest) (*pb.ListTutorsForStudentResponse, error) {
	studentId, err := uuid.Parse(req.StudentId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}

	tutorStudents, err := h.service.ListTutorStudentsForStudent(ctx, studentId)
	if err != nil {
		return nil, mapError(err, errdefs.ErrNotFound, errdefs.ErrPermissionDenied)
	}

	resp := make([]*pb.TutorStudent, len(tutorStudents))
	for i, tutorStudent := range tutorStudents {
		resp[i] = toPbTutorStudent(tutorStudent)
	}

	return &pb.ListTutorsForStudentResponse{Tutors: resp}, nil
}

func (h *UserServiceServer) ResolveTutorStudentContext(ctx context.Context, req *pb.ResolveTutorStudentContextRequest) (*pb.ResolvedTutorStudentContext, error) {
	tutorId, err := uuid.Parse(req.TutorId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}
	studentId, err := uuid.Parse(req.StudentId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}

	result, err := h.service.ResolveTutorStudentContext(ctx, tutorId, studentId)
	if err != nil {
		return nil, mapError(err, errdefs.ErrNotFound, errdefs.ErrPermissionDenied)
	}

	resp := &pb.ResolvedTutorStudentContext{
		RelationshipStatus:   result.RelationshipStatus.String(),
		LessonPriceRub:       result.LessonPriceRub,
		LessonConnectionLink: result.LessonConnectionLink,
		PaymentInfo:          result.PaymentInfo,
	}

	return resp, nil
}

func (h *UserServiceServer) AcceptInvitationFromTutor(ctx context.Context, req *pb.AcceptInvitationFromTutorRequest) (*pb.Empty, error) {
	tutorId, err := uuid.Parse(req.TutorId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}

	if err := h.service.AcceptInvitationFromTutor(ctx, tutorId); err != nil {
		return nil, mapError(err, errdefs.ErrPermissionDenied, errdefs.ErrNotFound)
	}

	return &pb.Empty{}, nil
}

func (h *UserServiceServer) CreateInvitation(ctx context.Context, _ *pb.CreateInvitationRequest) (*pb.Invitation, error) {
	inv, err := h.service.CreateInvitation(ctx)
	if err != nil {
		return nil, mapError(err, errdefs.ErrPermissionDenied)
	}
	return toPbInvitation(inv), nil
}

func (h *UserServiceServer) ListInvitations(ctx context.Context, _ *pb.ListInvitationsRequest) (*pb.ListInvitationsResponse, error) {
	invs, err := h.service.ListInvitations(ctx)
	if err != nil {
		return nil, mapError(err, errdefs.ErrPermissionDenied)
	}
	resp := make([]*pb.Invitation, len(invs))
	for i, inv := range invs {
		resp[i] = toPbInvitation(inv)
	}
	return &pb.ListInvitationsResponse{Invitations: resp}, nil
}

func (h *UserServiceServer) RevokeInvitation(ctx context.Context, req *pb.RevokeInvitationRequest) (*pb.Empty, error) {
	id, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}
	if err := h.service.RevokeInvitation(ctx, id); err != nil {
		return nil, mapError(err, errdefs.ErrNotFound, errdefs.ErrPermissionDenied)
	}
	return &pb.Empty{}, nil
}

func (h *UserServiceServer) AcceptInvitation(ctx context.Context, req *pb.AcceptInvitationRequest) (*pb.TutorStudent, error) {
	token, err := uuid.Parse(req.Token)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}
	ts, err := h.service.AcceptInvitation(ctx, token)
	if err != nil {
		return nil, mapError(err, errdefs.ErrNotFound, errdefs.ErrAlreadyExists, errdefs.ErrPermissionDenied)
	}
	return toPbTutorStudent(ts), nil
}

func toPbUser(user *model.User) *pb.User {
	userPb := pb.User{
		Id:           user.Id.String(),
		Role:         user.Role.String(),
		AuthProvider: user.AuthProvider.String(),
		Status:       user.Status.String(),
		FirstName:    user.FirstName,
		LastName:     user.LastName,
		Timezone:     user.Timezone,
		CreatedAt:    timestamppb.New(user.CreatedAt),
		EditedAt:     timestamppb.New(user.EditedAt),
	}

	return &userPb
}

func toPbTutorProfile(profile *model.TutorProfile) *pb.TutorProfile {
	return &pb.TutorProfile{
		Id:                   profile.Id.String(),
		UserId:               profile.UserId.String(),
		PaymentInfo:          profile.PaymentInfo,
		LessonPriceRub:       profile.LessonPriceRub,
		LessonConnectionLink: profile.LessonConnectionLink,
		CreatedAt:            timestamppb.New(profile.CreatedAt),
		EditedAt:             timestamppb.New(profile.EditedAt),
	}
}

func toPbTutorStudent(userStudent *model.TutorStudent) *pb.TutorStudent {
	return &pb.TutorStudent{
		Id:                   userStudent.Id.String(),
		TutorId:              userStudent.TutorId.String(),
		StudentId:            userStudent.StudentId.String(),
		Status:               userStudent.Status.String(),
		LessonPriceRub:       userStudent.LessonPriceRub,
		LessonConnectionLink: userStudent.LessonConnectionLink,
		CreatedAt:            timestamppb.New(userStudent.CreatedAt),
		EditedAt:             timestamppb.New(userStudent.EditedAt),
	}
}

func toPbInvitation(inv *model.Invitation) *pb.Invitation {
	return &pb.Invitation{
		Id:        inv.Id.String(),
		TutorId:   inv.TutorId.String(),
		Token:     inv.Token.String(),
		Status:    inv.Status.String(),
		CreatedAt: timestamppb.New(inv.CreatedAt),
		EditedAt:  timestamppb.New(inv.EditedAt),
	}
}

func mapError(err error, possibleErrors ...error) error {
	switch {
	case err == nil:
		return nil

	case errors.Is(err, errdefs.ErrAlreadyExists) && slices.Contains(possibleErrors, errdefs.ErrAlreadyExists):
		return status.Errorf(codes.AlreadyExists, "%v", err)

	case errors.Is(err, errdefs.ErrValidation) && slices.Contains(possibleErrors, errdefs.ErrValidation):
		return status.Errorf(codes.InvalidArgument, "%v", err)

	case errors.Is(err, errdefs.ErrAuthentication) && slices.Contains(possibleErrors, errdefs.ErrAuthentication):
		return status.Errorf(codes.Unauthenticated, "%v", err)

	case errors.Is(err, errdefs.ErrNotFound) && slices.Contains(possibleErrors, errdefs.ErrNotFound):
		return status.Errorf(codes.NotFound, "%v", err)

	case errors.Is(err, errdefs.ErrUserDeleted) && slices.Contains(possibleErrors, errdefs.ErrUserDeleted):
		return status.Errorf(codes.NotFound, "%v", err)

	case errors.Is(err, errdefs.ErrPermissionDenied) && slices.Contains(possibleErrors, errdefs.ErrPermissionDenied):
		return status.Errorf(codes.PermissionDenied, "%v", err)

	default:
		return status.Errorf(codes.Internal, "internal server error")
	}
}
