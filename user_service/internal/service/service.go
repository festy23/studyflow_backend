package service

import (
	"common_library/ctxdata"
	"common_library/logging"
	"context"
	"errors"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"slices"
	"strings"
	"userservice/internal/authorization"
	"userservice/internal/errdefs"
	"userservice/internal/model"
)

type UserRepository interface {
	NewUserCreationRepositoryTx(ctx context.Context) (UserCreationRepositoryTx, error)

	GetUser(ctx context.Context, id uuid.UUID) (*model.User, error)
	UpdateUser(ctx context.Context, id uuid.UUID, input *model.UpdateUserInput) (*model.User, error)
	DeleteUser(ctx context.Context, id uuid.UUID) (*model.User, error)

	GetTutorProfile(ctx context.Context, userId uuid.UUID) (*model.TutorProfile, error)
	UpdateTutorProfile(ctx context.Context, userId uuid.UUID, input *model.UpdateTutorProfileInput) (*model.TutorProfile, error)
	CreateTutorProfile(ctx context.Context, input *model.RepositoryCreateTutorProfileInput) (*model.TutorProfile, error)

	GetTelegramAccount(ctx context.Context, userId uuid.UUID) (*model.TelegramAccount, error)
	GetTelegramAccountByTelegramId(ctx context.Context, telegramId int64) (*model.TelegramAccount, error)
}

type UserCreationRepositoryTx interface {
	CreateUser(ctx context.Context, input *model.RepositoryCreateUserInput) (*model.User, error)
	CreateTutorProfile(ctx context.Context, input *model.RepositoryCreateTutorProfileInput) (*model.TutorProfile, error)
	CreateTelegramAccount(ctx context.Context, input *model.RepositoryCreateTelegramAccountInput) (*model.TelegramAccount, error)
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}

type TutorStudentsRepository interface {
	CreateTutorStudent(ctx context.Context, input *model.RepositoryCreateTutorStudentInput) (*model.TutorStudent, error)
	UpdateTutorStudent(ctx context.Context, tutorId uuid.UUID, studentId uuid.UUID, input *model.UpdateTutorStudentInput) (*model.TutorStudent, error)
	GetTutorStudent(ctx context.Context, tutorId uuid.UUID, studentId uuid.UUID) (*model.TutorStudent, error)
	DeleteTutorStudent(ctx context.Context, tutorId uuid.UUID, studentId uuid.UUID) error
	// ListTutorStudents Set tutorId or studentId to UUID.Nil to search by one parameter
	ListTutorStudents(ctx context.Context, tutorId uuid.UUID, studentId uuid.UUID) ([]*model.TutorStudent, error)
}

type InvitationRepository interface {
	CreateInvitation(ctx context.Context, inv *model.Invitation) (*model.Invitation, error)
	GetInvitationByToken(ctx context.Context, token uuid.UUID) (*model.Invitation, error)
	GetInvitationByID(ctx context.Context, id uuid.UUID) (*model.Invitation, error)
	ListInvitationsByTutor(ctx context.Context, tutorId uuid.UUID) ([]*model.Invitation, error)
	UpdateInvitationStatus(ctx context.Context, id uuid.UUID, status model.InvitationStatus) error
}

type UserService struct {
	userRepository         UserRepository
	tsRepository           TutorStudentsRepository
	invitationRepository   InvitationRepository
	telegramAuthSecret     string
	authDisableLegacyHMAC  bool
}

func NewUserService(
	userRepository UserRepository,
	tutorStudentsRepository TutorStudentsRepository,
	invitationRepository InvitationRepository,
	telegramAuthSecret string,
	authDisableLegacyHMAC bool,
) *UserService {
	return &UserService{userRepository, tutorStudentsRepository, invitationRepository, telegramAuthSecret, authDisableLegacyHMAC}
}

func (s *UserService) RegisterViaTelegram(ctx context.Context, input *model.RegisterViaTelegramInput) (*model.User, error) {
	if !input.Role.IsValid() {
		return nil, errdefs.ErrValidation
	}

	// Check if this telegram account already exists
	tgAccount, err := s.userRepository.GetTelegramAccountByTelegramId(ctx, input.TelegramId)
	if err != nil && !errors.Is(err, errdefs.ErrNotFound) {
		return nil, err
	}

	if err == nil {
		// Telegram account exists — check the associated user
		user, err := s.userRepository.GetUser(ctx, tgAccount.UserId)
		if err != nil {
			return nil, err
		}

		switch user.Status {
		case model.UserStatusDeleted:
			return s.reactivateDeletedUser(ctx, user, input)
		case model.UserStatusActive:
			return nil, errdefs.ErrAlreadyExists
		default:
			return nil, errdefs.ErrAlreadyExists
		}
	}

	// No existing telegram account — normal registration flow
	repo, err := s.userRepository.NewUserCreationRepositoryTx(ctx)
	if err != nil {
		return nil, err
	}

	defer func(repo UserCreationRepositoryTx, ctx context.Context) {
		err := repo.Rollback(ctx)
		if err != nil {
			logger, ok := logging.GetFromContext(ctx)
			if ok {
				logger.Error(ctx, "Failed to Rollback", zap.Error(err))
			}
		}
	}(repo, ctx)

	id, err := uuid.NewV7()
	if err != nil {
		return nil, err
	}

	userInput := &model.RepositoryCreateUserInput{
		Id:           id,
		Role:         input.Role,
		AuthProvider: model.AuthProviderTelegram,
		Status:       model.UserStatusActive,
		FirstName:    input.FirstName,
		LastName:     input.LastName,
		Timezone:     input.Timezone,
	}

	user, err := repo.CreateUser(ctx, userInput)
	if err != nil {
		return nil, err
	}

	id, err = uuid.NewV7()
	if err != nil {
		return nil, err
	}

	tgAccountInput := &model.RepositoryCreateTelegramAccountInput{
		Id:         id,
		UserId:     user.Id,
		TelegramId: input.TelegramId,
		Username:   input.Username,
	}

	_, err = repo.CreateTelegramAccount(ctx, tgAccountInput)
	if err != nil {
		return nil, err
	}

	if user.Role == model.RoleTutor {
		id, err = uuid.NewV7()
		if err != nil {
			return nil, err
		}
		tutorProfileInput := &model.RepositoryCreateTutorProfileInput{
			Id:     id,
			UserId: user.Id,
		}

		_, err := repo.CreateTutorProfile(ctx, tutorProfileInput)
		if err != nil {
			return nil, err
		}
	}

	err = repo.Commit(ctx)
	if err != nil {
		return nil, err
	}

	return user, nil
}

func (s *UserService) reactivateDeletedUser(ctx context.Context, user *model.User, input *model.RegisterViaTelegramInput) (*model.User, error) {
	activeStatus := model.UserStatusActive
	updateInput := &model.UpdateUserInput{
		FirstName: input.FirstName,
		LastName:  input.LastName,
		Timezone:  input.Timezone,
		Role:      &input.Role,
		Status:    &activeStatus,
	}

	oldRole := user.Role
	updatedUser, err := s.userRepository.UpdateUser(ctx, user.Id, updateInput)
	if err != nil {
		return nil, err
	}

	if input.Role == model.RoleTutor && oldRole != model.RoleTutor {
		_, err := s.userRepository.GetTutorProfile(ctx, user.Id)
		if err != nil {
			if errors.Is(err, errdefs.ErrNotFound) {
				profileID, err := uuid.NewV7()
				if err != nil {
					return nil, err
				}
				_, err = s.userRepository.CreateTutorProfile(ctx, &model.RepositoryCreateTutorProfileInput{
					Id:     profileID,
					UserId: user.Id,
				})
				if err != nil {
					return nil, err
				}
			} else {
				return nil, err
			}
		}
	}

	return updatedUser, nil
}

func (s *UserService) Authorize(ctx context.Context, input *model.AuthorizeInput) (*model.User, error) {
	header := strings.TrimSpace(input.AuthorizationHeader)
	if strings.HasPrefix(header, "telegram ") {
		if s.authDisableLegacyHMAC {
			return nil, errdefs.ErrAuthentication
		}
		return s.authorizeWithTelegram(ctx, strings.TrimSpace(strings.TrimPrefix(header, "telegram ")))
	}
	if strings.HasPrefix(header, "tma ") {
		return s.authorizeWithTelegram(ctx, strings.TrimSpace(strings.TrimPrefix(header, "tma ")))
	}
	if strings.HasPrefix(header, "Bearer ") {
		return s.authorizeWithTelegram(ctx, strings.TrimSpace(strings.TrimPrefix(header, "Bearer ")))
	}

	return nil, errdefs.ErrAuthentication
}

func (s *UserService) authorizeWithTelegram(ctx context.Context, header string) (*model.User, error) {
	telegramId, err := authorization.GetTelegramId(s.telegramAuthSecret, header)
	if err != nil {
		return nil, err
	}

	tgAccount, err := s.userRepository.GetTelegramAccountByTelegramId(ctx, telegramId)
	if err != nil {
		return nil, err
	}

	user, err := s.userRepository.GetUser(ctx, tgAccount.UserId)
	if err != nil {
		return nil, err
	}

	if user.Status == model.UserStatusDeleted {
		return nil, errdefs.ErrUserDeleted
	}

	return user, nil
}

func (s *UserService) GetMe(ctx context.Context) (*model.User, error) {
	userId, ok := ctxdata.GetUserID(ctx)
	if !ok {
		return nil, errdefs.ErrNotFound
	}

	id, err := uuid.Parse(userId)
	if err != nil {
		return nil, errdefs.ErrAuthentication
	}

	user, err := s.userRepository.GetUser(ctx, id)
	if err != nil {
		return nil, err
	}

	return user, nil
}

func (s *UserService) GetUserPublic(ctx context.Context, id uuid.UUID) (*model.UserPublic, error) {
	user, err := s.userRepository.GetUser(ctx, id)
	if err != nil {
		return nil, err
	}

	resp := &model.UserPublic{
		Id:        user.Id,
		Role:      user.Role,
		FirstName: user.FirstName,
		LastName:  user.LastName,
	}
	return resp, nil
}

func (s *UserService) UpdateUser(ctx context.Context, id uuid.UUID, input *model.UpdateUserInput) (*model.User, error) {
	if err := ensureCurrentUserIs(ctx, id); err != nil {
		return nil, err
	}

	var oldRole model.Role
	if input.Role != nil {
		currentUser, err := s.userRepository.GetUser(ctx, id)
		if err != nil {
			return nil, err
		}
		if currentUser.Status == model.UserStatusDeleted {
			return nil, errdefs.ErrNotFound
		}
		oldRole = currentUser.Role
	}

	user, err := s.userRepository.UpdateUser(ctx, id, input)
	if err != nil {
		return nil, err
	}

	if input.Role != nil && *input.Role == model.RoleTutor && oldRole != model.RoleTutor {
		_, err := s.userRepository.GetTutorProfile(ctx, id)
		if err != nil {
			if errors.Is(err, errdefs.ErrNotFound) {
				profileID, err := uuid.NewV7()
				if err != nil {
					return nil, err
				}
				_, err = s.userRepository.CreateTutorProfile(ctx, &model.RepositoryCreateTutorProfileInput{
					Id:     profileID,
					UserId: id,
				})
				if err != nil {
					return nil, err
				}
			} else {
				return nil, err
			}
		}
	}

	return user, nil
}

func (s *UserService) DeleteUser(ctx context.Context, id uuid.UUID) (*model.User, error) {
	if err := ensureCurrentUserIs(ctx, id); err != nil {
		return nil, err
	}

	user, err := s.userRepository.DeleteUser(ctx, id)
	if err != nil {
		return nil, err
	}

	return user, nil
}

// GetTelegramChatId returns the Telegram chat_id (== telegram_id) for the given user.
// Intended for internal service-to-service callers (notification_service).
// Returns errdefs.ErrNotFound if the user has no linked Telegram account.
func (s *UserService) GetTelegramChatId(ctx context.Context, userId uuid.UUID) (int64, error) {
	acc, err := s.userRepository.GetTelegramAccount(ctx, userId)
	if err != nil {
		return 0, err
	}
	return acc.TelegramId, nil
}

func (s *UserService) GetTutorProfile(ctx context.Context, userId uuid.UUID) (*model.TutorProfile, error) {
	if err := ensureCurrentUserIs(ctx, userId); err != nil {
		return nil, err
	}

	profile, err := s.userRepository.GetTutorProfile(ctx, userId)
	if err != nil {
		return nil, err
	}

	return profile, nil
}

func (s *UserService) UpdateTutorProfile(ctx context.Context, userId uuid.UUID, input *model.UpdateTutorProfileInput) (*model.TutorProfile, error) {
	if err := ensureCurrentUserIs(ctx, userId); err != nil {
		return nil, err
	}

	profile, err := s.userRepository.UpdateTutorProfile(ctx, userId, input)
	if err != nil {
		return nil, err
	}

	return profile, nil
}

func (s *UserService) CreateTutorStudent(ctx context.Context, input *model.CreateTutorStudentInput) (*model.TutorStudent, error) {
	if err := ensureCurrentUserIs(ctx, input.TutorId); err != nil {
		return nil, err
	}

	if err := ensureCurrentUserRole(ctx, model.RoleTutor); err != nil {
		return nil, err
	}

	id, err := uuid.NewV7()
	if err != nil {
		return nil, err
	}

	tsInput := &model.RepositoryCreateTutorStudentInput{
		Id:                   id,
		TutorId:              input.TutorId,
		StudentId:            input.StudentId,
		LessonPriceRub:       input.LessonPriceRub,
		LessonConnectionLink: input.LessonConnectionLink,
		Status:               model.TutorStudentStatusInvited,
	}
	ts, err := s.tsRepository.CreateTutorStudent(ctx, tsInput)
	if err != nil {
		return nil, err
	}

	return ts, nil
}

func (s *UserService) GetTutorStudent(ctx context.Context, tutorId uuid.UUID, studentId uuid.UUID) (*model.TutorStudent, error) {
	if err := ensureCurrentUserIs(ctx, tutorId, studentId); err != nil {
		return nil, err
	}
	ts, err := s.tsRepository.GetTutorStudent(ctx, tutorId, studentId)
	if err != nil {
		return nil, err
	}

	return ts, nil
}

func (s *UserService) UpdateTutorStudent(ctx context.Context, tutorId uuid.UUID, studentId uuid.UUID, input *model.UpdateTutorStudentInput) (*model.TutorStudent, error) {
	if err := ensureCurrentUserIs(ctx, tutorId); err != nil {
		return nil, err
	}
	ts, err := s.tsRepository.UpdateTutorStudent(ctx, tutorId, studentId, input)
	if err != nil {
		return nil, err
	}

	return ts, nil
}

func (s *UserService) DeleteTutorStudent(ctx context.Context, tutorId uuid.UUID, studentId uuid.UUID) error {
	if err := ensureCurrentUserIs(ctx, tutorId); err != nil {
		return err
	}

	if err := s.tsRepository.DeleteTutorStudent(ctx, tutorId, studentId); err != nil {
		return err
	}

	return nil
}

func (s *UserService) ListTutorStudents(ctx context.Context, tutorId uuid.UUID) ([]*model.TutorStudent, error) {
	if err := ensureCurrentUserIs(ctx, tutorId); err != nil {
		return nil, err
	}

	resp, err := s.tsRepository.ListTutorStudents(ctx, tutorId, uuid.Nil)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (s *UserService) ListTutorStudentsForStudent(ctx context.Context, studentId uuid.UUID) ([]*model.TutorStudent, error) {
	if err := ensureCurrentUserIs(ctx, studentId); err != nil {
		return nil, err
	}

	resp, err := s.tsRepository.ListTutorStudents(ctx, uuid.Nil, studentId)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (s *UserService) ResolveTutorStudentContext(ctx context.Context, tutorId uuid.UUID, studentId uuid.UUID) (*model.TutorStudentContext, error) {
	if err := ensureCurrentUserIs(ctx, tutorId, studentId); err != nil {
		return nil, err
	}

	tutorProfile, err := s.userRepository.GetTutorProfile(ctx, tutorId)
	if err != nil {
		return nil, err
	}

	ts, err := s.tsRepository.GetTutorStudent(ctx, tutorId, studentId)
	if err != nil {
		return nil, err
	}

	resp := &model.TutorStudentContext{
		RelationshipStatus:   ts.Status,
		LessonPriceRub:       tutorProfile.LessonPriceRub,
		LessonConnectionLink: tutorProfile.LessonConnectionLink,
		PaymentInfo:          tutorProfile.PaymentInfo,
	}

	if ts.LessonPriceRub != nil {
		resp.LessonPriceRub = ts.LessonPriceRub
	}

	if ts.LessonConnectionLink != nil {
		resp.LessonConnectionLink = ts.LessonConnectionLink
	}

	return resp, nil
}

func (s *UserService) AcceptInvitationFromTutor(ctx context.Context, tutorId uuid.UUID) error {
	id, err := getUserId(ctx)
	if err != nil {
		return err
	}

	if err := ensureCurrentUserRole(ctx, model.RoleStudent); err != nil {
		return err
	}

	status := model.TutorStudentStatusActive
	_, err = s.tsRepository.UpdateTutorStudent(ctx, tutorId, id, &model.UpdateTutorStudentInput{Status: &status})
	if err != nil {
		return err
	}

	return nil
}

func getUserId(ctx context.Context) (uuid.UUID, error) {
	id, ok := ctxdata.GetUserID(ctx)
	if !ok {
		return uuid.Nil, errdefs.ErrAuthentication
	}

	idUUID, err := uuid.Parse(id)
	if err != nil {
		return uuid.Nil, errdefs.ErrAuthentication
	}

	return idUUID, nil
}

func getRole(ctx context.Context) (model.Role, error) {
	roleString, ok := ctxdata.GetUserRole(ctx)
	if !ok {
		return "", errdefs.ErrAuthentication
	}
	role := model.Role(roleString)
	if !role.IsValid() {
		return "", errdefs.ErrAuthentication
	}

	return role, nil
}

func ensureCurrentUserIs(ctx context.Context, ids ...uuid.UUID) error {
	currentUserId, err := getUserId(ctx)
	if err != nil {
		return err
	}
	if !slices.Contains(ids, currentUserId) {
		return errdefs.ErrPermissionDenied
	}
	return nil
}

func ensureCurrentUserRole(ctx context.Context, role model.Role) error {
	userRole, err := getRole(ctx)
	if err != nil {
		return err
	}
	if userRole != role {
		return errdefs.ErrPermissionDenied
	}
	return nil
}

func (s *UserService) CreateInvitation(ctx context.Context) (*model.Invitation, error) {
	if err := ensureCurrentUserRole(ctx, model.RoleTutor); err != nil {
		return nil, err
	}

	tutorId, err := getUserId(ctx)
	if err != nil {
		return nil, err
	}

	id, err := uuid.NewV7()
	if err != nil {
		return nil, err
	}
	token, err := uuid.NewV7()
	if err != nil {
		return nil, err
	}

	inv := &model.Invitation{
		Id:      id,
		TutorId: tutorId,
		Token:   token,
		Status:  model.InvitationStatusActive,
	}

	return s.invitationRepository.CreateInvitation(ctx, inv)
}

func (s *UserService) ListInvitations(ctx context.Context) ([]*model.Invitation, error) {
	if err := ensureCurrentUserRole(ctx, model.RoleTutor); err != nil {
		return nil, err
	}

	tutorId, err := getUserId(ctx)
	if err != nil {
		return nil, err
	}

	return s.invitationRepository.ListInvitationsByTutor(ctx, tutorId)
}

func (s *UserService) RevokeInvitation(ctx context.Context, id uuid.UUID) error {
	if err := ensureCurrentUserRole(ctx, model.RoleTutor); err != nil {
		return err
	}

	inv, err := s.invitationRepository.GetInvitationByID(ctx, id)
	if err != nil {
		return err
	}

	tutorId, err := getUserId(ctx)
	if err != nil {
		return err
	}
	if inv.TutorId != tutorId {
		return errdefs.ErrPermissionDenied
	}

	return s.invitationRepository.UpdateInvitationStatus(ctx, id, model.InvitationStatusRevoked)
}

func (s *UserService) AcceptInvitation(ctx context.Context, token uuid.UUID) (*model.TutorStudent, error) {
	if err := ensureCurrentUserRole(ctx, model.RoleStudent); err != nil {
		return nil, err
	}

	studentId, err := getUserId(ctx)
	if err != nil {
		return nil, err
	}

	inv, err := s.invitationRepository.GetInvitationByToken(ctx, token)
	if err != nil {
		return nil, err
	}

	if inv.Status != model.InvitationStatusActive {
		return nil, errdefs.ErrNotFound
	}

	tsID, err := uuid.NewV7()
	if err != nil {
		return nil, err
	}

	ts, err := s.tsRepository.CreateTutorStudent(ctx, &model.RepositoryCreateTutorStudentInput{
		Id:        tsID,
		TutorId:   inv.TutorId,
		StudentId: studentId,
		Status:    model.TutorStudentStatusActive,
	})
	if err != nil {
		return nil, err
	}

	if err := s.invitationRepository.UpdateInvitationStatus(ctx, inv.Id, model.InvitationStatusUsed); err != nil {
		return nil, err
	}

	return ts, nil
}
