package service_test

import (
	"common_library/ctxdata"
	"context"
	"errors"
	"testing"
	"userservice/internal/errdefs"
	"userservice/internal/model"
	"userservice/internal/service"
	"userservice/internal/service/mocks"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func setup(t *testing.T) (
	*service.UserService,
	*mocks.MockUserRepository,
	*mocks.MockTutorStudentsRepository,
	*gomock.Controller,
) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	mockUserRepo := mocks.NewMockUserRepository(ctrl)
	mockTSRepo := mocks.NewMockTutorStudentsRepository(ctrl)
	svc := service.NewUserService(mockUserRepo, mockTSRepo, "test-secret")

	return svc, mockUserRepo, mockTSRepo, ctrl
}

func userCtx(userID uuid.UUID, role model.Role) context.Context {
	ctx := context.Background()
	ctx = ctxdata.WithUserID(ctx, userID.String())
	ctx = ctxdata.WithUserRole(ctx, string(role))
	return ctx
}

// ── GetMe ───────────────────────────────────────────────────────────

func TestGetMe(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		svc, mockUserRepo, _, _ := setup(t)
		userID := uuid.New()
		ctx := userCtx(userID, model.RoleTutor)

		expected := &model.User{
			Id:   userID,
			Role: model.RoleTutor,
		}
		mockUserRepo.EXPECT().GetUser(gomock.Any(), userID).Return(expected, nil)

		result, err := svc.GetMe(ctx)
		require.NoError(t, err)
		assert.Equal(t, userID, result.Id)
	})

	t.Run("NoUserIDInContext", func(t *testing.T) {
		svc, _, _, _ := setup(t)

		_, err := svc.GetMe(context.Background())
		assert.ErrorIs(t, err, errdefs.ErrNotFound)
	})

	t.Run("InvalidUUIDInContext", func(t *testing.T) {
		svc, _, _, _ := setup(t)
		ctx := ctxdata.WithUserID(context.Background(), "not-a-uuid")

		_, err := svc.GetMe(ctx)
		assert.ErrorIs(t, err, errdefs.ErrAuthentication)
	})

	t.Run("UserNotFoundInRepo", func(t *testing.T) {
		svc, mockUserRepo, _, _ := setup(t)
		userID := uuid.New()
		ctx := userCtx(userID, model.RoleTutor)

		mockUserRepo.EXPECT().GetUser(gomock.Any(), userID).Return(nil, errdefs.ErrNotFound)

		_, err := svc.GetMe(ctx)
		assert.ErrorIs(t, err, errdefs.ErrNotFound)
	})
}

// ── GetUserPublic ───────────────────────────────────────────────────

func TestGetUserPublic(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		svc, mockUserRepo, _, _ := setup(t)
		userID := uuid.New()
		firstName := "John"

		mockUserRepo.EXPECT().GetUser(gomock.Any(), userID).Return(&model.User{
			Id:        userID,
			Role:      model.RoleTutor,
			FirstName: &firstName,
		}, nil)

		result, err := svc.GetUserPublic(context.Background(), userID)
		require.NoError(t, err)
		assert.Equal(t, userID, result.Id)
		assert.Equal(t, model.RoleTutor, result.Role)
		assert.Equal(t, &firstName, result.FirstName)
	})

	t.Run("NotFound", func(t *testing.T) {
		svc, mockUserRepo, _, _ := setup(t)
		userID := uuid.New()

		mockUserRepo.EXPECT().GetUser(gomock.Any(), userID).Return(nil, errdefs.ErrNotFound)

		_, err := svc.GetUserPublic(context.Background(), userID)
		assert.ErrorIs(t, err, errdefs.ErrNotFound)
	})
}

// ── UpdateUser ──────────────────────────────────────────────────────

func TestUpdateUser(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		svc, mockUserRepo, _, _ := setup(t)
		userID := uuid.New()
		ctx := userCtx(userID, model.RoleTutor)
		newName := "Updated"

		input := &model.UpdateUserInput{FirstName: &newName}
		mockUserRepo.EXPECT().UpdateUser(gomock.Any(), userID, input).Return(&model.User{
			Id:        userID,
			FirstName: &newName,
		}, nil)

		result, err := svc.UpdateUser(ctx, userID, input)
		require.NoError(t, err)
		assert.Equal(t, &newName, result.FirstName)
	})

	t.Run("PermissionDenied_DifferentUser", func(t *testing.T) {
		svc, _, _, _ := setup(t)
		currentUserID := uuid.New()
		otherUserID := uuid.New()
		ctx := userCtx(currentUserID, model.RoleTutor)

		_, err := svc.UpdateUser(ctx, otherUserID, &model.UpdateUserInput{})
		assert.ErrorIs(t, err, errdefs.ErrPermissionDenied)
	})

	t.Run("Unauthenticated", func(t *testing.T) {
		svc, _, _, _ := setup(t)

		_, err := svc.UpdateUser(context.Background(), uuid.New(), &model.UpdateUserInput{})
		assert.ErrorIs(t, err, errdefs.ErrAuthentication)
	})
}

// ── GetTutorProfile ─────────────────────────────────────────────────

func TestGetTutorProfile(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		svc, mockUserRepo, _, _ := setup(t)
		userID := uuid.New()
		ctx := userCtx(userID, model.RoleTutor)

		expected := &model.TutorProfile{UserId: userID}
		mockUserRepo.EXPECT().GetTutorProfile(gomock.Any(), userID).Return(expected, nil)

		result, err := svc.GetTutorProfile(ctx, userID)
		require.NoError(t, err)
		assert.Equal(t, userID, result.UserId)
	})

	t.Run("PermissionDenied", func(t *testing.T) {
		svc, _, _, _ := setup(t)
		currentUserID := uuid.New()
		otherUserID := uuid.New()
		ctx := userCtx(currentUserID, model.RoleTutor)

		_, err := svc.GetTutorProfile(ctx, otherUserID)
		assert.ErrorIs(t, err, errdefs.ErrPermissionDenied)
	})
}

// ── UpdateTutorProfile ──────────────────────────────────────────────

func TestUpdateTutorProfile(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		svc, mockUserRepo, _, _ := setup(t)
		userID := uuid.New()
		ctx := userCtx(userID, model.RoleTutor)
		price := int32(1500)

		input := &model.UpdateTutorProfileInput{LessonPriceRub: &price}
		mockUserRepo.EXPECT().UpdateTutorProfile(gomock.Any(), userID, input).Return(&model.TutorProfile{
			UserId:         userID,
			LessonPriceRub: &price,
		}, nil)

		result, err := svc.UpdateTutorProfile(ctx, userID, input)
		require.NoError(t, err)
		assert.Equal(t, &price, result.LessonPriceRub)
	})

	t.Run("PermissionDenied", func(t *testing.T) {
		svc, _, _, _ := setup(t)
		currentUserID := uuid.New()
		otherUserID := uuid.New()
		ctx := userCtx(currentUserID, model.RoleTutor)

		_, err := svc.UpdateTutorProfile(ctx, otherUserID, &model.UpdateTutorProfileInput{})
		assert.ErrorIs(t, err, errdefs.ErrPermissionDenied)
	})
}

// ── CreateTutorStudent ──────────────────────────────────────────────

func TestCreateTutorStudent(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		svc, _, mockTSRepo, _ := setup(t)
		tutorID := uuid.New()
		studentID := uuid.New()
		ctx := userCtx(tutorID, model.RoleTutor)

		mockTSRepo.EXPECT().CreateTutorStudent(gomock.Any(), gomock.Any()).Return(&model.TutorStudent{
			TutorId:   tutorID,
			StudentId: studentID,
			Status:    model.TutorStudentStatusInvited,
		}, nil)

		result, err := svc.CreateTutorStudent(ctx, &model.CreateTutorStudentInput{
			TutorId:   tutorID,
			StudentId: studentID,
		})
		require.NoError(t, err)
		assert.Equal(t, model.TutorStudentStatusInvited, result.Status)
	})

	t.Run("PermissionDenied_NotTheTutor", func(t *testing.T) {
		svc, _, _, _ := setup(t)
		currentUserID := uuid.New()
		otherTutorID := uuid.New()
		ctx := userCtx(currentUserID, model.RoleTutor)

		_, err := svc.CreateTutorStudent(ctx, &model.CreateTutorStudentInput{
			TutorId:   otherTutorID,
			StudentId: uuid.New(),
		})
		assert.ErrorIs(t, err, errdefs.ErrPermissionDenied)
	})

	t.Run("PermissionDenied_StudentRole", func(t *testing.T) {
		svc, _, _, _ := setup(t)
		userID := uuid.New()
		ctx := userCtx(userID, model.RoleStudent)

		_, err := svc.CreateTutorStudent(ctx, &model.CreateTutorStudentInput{
			TutorId:   userID,
			StudentId: uuid.New(),
		})
		assert.ErrorIs(t, err, errdefs.ErrPermissionDenied)
	})
}

// ── GetTutorStudent ─────────────────────────────────────────────────

func TestGetTutorStudent(t *testing.T) {
	t.Run("Success_AsTutor", func(t *testing.T) {
		svc, _, mockTSRepo, _ := setup(t)
		tutorID := uuid.New()
		studentID := uuid.New()
		ctx := userCtx(tutorID, model.RoleTutor)

		mockTSRepo.EXPECT().GetTutorStudent(gomock.Any(), tutorID, studentID).Return(&model.TutorStudent{
			TutorId:   tutorID,
			StudentId: studentID,
		}, nil)

		result, err := svc.GetTutorStudent(ctx, tutorID, studentID)
		require.NoError(t, err)
		assert.Equal(t, tutorID, result.TutorId)
	})

	t.Run("Success_AsStudent", func(t *testing.T) {
		svc, _, mockTSRepo, _ := setup(t)
		tutorID := uuid.New()
		studentID := uuid.New()
		ctx := userCtx(studentID, model.RoleStudent)

		mockTSRepo.EXPECT().GetTutorStudent(gomock.Any(), tutorID, studentID).Return(&model.TutorStudent{
			TutorId:   tutorID,
			StudentId: studentID,
		}, nil)

		result, err := svc.GetTutorStudent(ctx, tutorID, studentID)
		require.NoError(t, err)
		assert.Equal(t, studentID, result.StudentId)
	})

	t.Run("PermissionDenied_UnrelatedUser", func(t *testing.T) {
		svc, _, _, _ := setup(t)
		unrelatedID := uuid.New()
		ctx := userCtx(unrelatedID, model.RoleTutor)

		_, err := svc.GetTutorStudent(ctx, uuid.New(), uuid.New())
		assert.ErrorIs(t, err, errdefs.ErrPermissionDenied)
	})
}

// ── DeleteTutorStudent ──────────────────────────────────────────────

func TestDeleteTutorStudent(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		svc, _, mockTSRepo, _ := setup(t)
		tutorID := uuid.New()
		studentID := uuid.New()
		ctx := userCtx(tutorID, model.RoleTutor)

		mockTSRepo.EXPECT().DeleteTutorStudent(gomock.Any(), tutorID, studentID).Return(nil)

		err := svc.DeleteTutorStudent(ctx, tutorID, studentID)
		assert.NoError(t, err)
	})

	t.Run("PermissionDenied", func(t *testing.T) {
		svc, _, _, _ := setup(t)
		currentUserID := uuid.New()
		ctx := userCtx(currentUserID, model.RoleTutor)

		err := svc.DeleteTutorStudent(ctx, uuid.New(), uuid.New())
		assert.ErrorIs(t, err, errdefs.ErrPermissionDenied)
	})
}

// ── ListTutorStudents ───────────────────────────────────────────────

func TestListTutorStudents(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		svc, _, mockTSRepo, _ := setup(t)
		tutorID := uuid.New()
		ctx := userCtx(tutorID, model.RoleTutor)

		mockTSRepo.EXPECT().ListTutorStudents(gomock.Any(), tutorID, uuid.Nil).Return([]*model.TutorStudent{
			{TutorId: tutorID, StudentId: uuid.New()},
			{TutorId: tutorID, StudentId: uuid.New()},
		}, nil)

		result, err := svc.ListTutorStudents(ctx, tutorID)
		require.NoError(t, err)
		assert.Len(t, result, 2)
	})

	t.Run("PermissionDenied", func(t *testing.T) {
		svc, _, _, _ := setup(t)
		currentUserID := uuid.New()
		ctx := userCtx(currentUserID, model.RoleTutor)

		_, err := svc.ListTutorStudents(ctx, uuid.New())
		assert.ErrorIs(t, err, errdefs.ErrPermissionDenied)
	})
}

// ── ListTutorStudentsForStudent ─────────────────────────────────────

func TestListTutorStudentsForStudent(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		svc, _, mockTSRepo, _ := setup(t)
		studentID := uuid.New()
		ctx := userCtx(studentID, model.RoleStudent)

		mockTSRepo.EXPECT().ListTutorStudents(gomock.Any(), uuid.Nil, studentID).Return([]*model.TutorStudent{
			{TutorId: uuid.New(), StudentId: studentID},
		}, nil)

		result, err := svc.ListTutorStudentsForStudent(ctx, studentID)
		require.NoError(t, err)
		assert.Len(t, result, 1)
	})
}

// ── AcceptInvitationFromTutor ───────────────────────────────────────

func TestAcceptInvitationFromTutor(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		svc, _, mockTSRepo, _ := setup(t)
		studentID := uuid.New()
		tutorID := uuid.New()
		ctx := userCtx(studentID, model.RoleStudent)

		activeStatus := model.TutorStudentStatusActive
		mockTSRepo.EXPECT().UpdateTutorStudent(gomock.Any(), tutorID, studentID, &model.UpdateTutorStudentInput{
			Status: &activeStatus,
		}).Return(&model.TutorStudent{Status: model.TutorStudentStatusActive}, nil)

		err := svc.AcceptInvitationFromTutor(ctx, tutorID)
		assert.NoError(t, err)
	})

	t.Run("PermissionDenied_TutorRole", func(t *testing.T) {
		svc, _, _, _ := setup(t)
		userID := uuid.New()
		ctx := userCtx(userID, model.RoleTutor)

		err := svc.AcceptInvitationFromTutor(ctx, uuid.New())
		assert.ErrorIs(t, err, errdefs.ErrPermissionDenied)
	})

	t.Run("Unauthenticated", func(t *testing.T) {
		svc, _, _, _ := setup(t)

		err := svc.AcceptInvitationFromTutor(context.Background(), uuid.New())
		assert.ErrorIs(t, err, errdefs.ErrAuthentication)
	})
}

// ── ResolveTutorStudentContext ───────────────────────────────────────

func TestResolveTutorStudentContext(t *testing.T) {
	t.Run("Success_PairOverridesTutorDefaults", func(t *testing.T) {
		svc, mockUserRepo, mockTSRepo, _ := setup(t)
		tutorID := uuid.New()
		studentID := uuid.New()
		ctx := userCtx(tutorID, model.RoleTutor)

		tutorPrice := int32(1000)
		tutorLink := "https://meet.example.com/default"
		pairPrice := int32(1500)

		mockUserRepo.EXPECT().GetTutorProfile(gomock.Any(), tutorID).Return(&model.TutorProfile{
			UserId:               tutorID,
			LessonPriceRub:       &tutorPrice,
			LessonConnectionLink: &tutorLink,
		}, nil)

		mockTSRepo.EXPECT().GetTutorStudent(gomock.Any(), tutorID, studentID).Return(&model.TutorStudent{
			TutorId:        tutorID,
			StudentId:      studentID,
			LessonPriceRub: &pairPrice,
			Status:         model.TutorStudentStatusActive,
		}, nil)

		result, err := svc.ResolveTutorStudentContext(ctx, tutorID, studentID)
		require.NoError(t, err)
		assert.Equal(t, &pairPrice, result.LessonPriceRub)
		assert.Equal(t, &tutorLink, result.LessonConnectionLink)
		assert.Equal(t, model.TutorStudentStatusActive, result.RelationshipStatus)
	})

	t.Run("PermissionDenied", func(t *testing.T) {
		svc, _, _, _ := setup(t)
		unrelatedID := uuid.New()
		ctx := userCtx(unrelatedID, model.RoleTutor)

		_, err := svc.ResolveTutorStudentContext(ctx, uuid.New(), uuid.New())
		assert.ErrorIs(t, err, errdefs.ErrPermissionDenied)
	})
}

// ── RegisterViaTelegram ─────────────────────────────────────────────

func TestRegisterViaTelegram(t *testing.T) {
	t.Run("InvalidRole", func(t *testing.T) {
		svc, _, _, _ := setup(t)

		_, err := svc.RegisterViaTelegram(context.Background(), &model.RegisterViaTelegramInput{
			TelegramId: 12345,
			Role:       model.Role("invalid"),
		})
		assert.ErrorIs(t, err, errdefs.ErrValidation)
	})

	t.Run("Success_Student", func(t *testing.T) {
		svc, mockUserRepo, _, _ := setup(t)

		mockTx := mocks.NewMockUserCreationRepositoryTx(gomock.NewController(t))
		mockUserRepo.EXPECT().NewUserCreationRepositoryTx(gomock.Any()).Return(mockTx, nil)

		studentID := uuid.New()
		mockTx.EXPECT().CreateUser(gomock.Any(), gomock.Any()).Return(&model.User{
			Id:   studentID,
			Role: model.RoleStudent,
		}, nil)
		mockTx.EXPECT().CreateTelegramAccount(gomock.Any(), gomock.Any()).Return(&model.TelegramAccount{}, nil)
		mockTx.EXPECT().Commit(gomock.Any()).Return(nil)
		mockTx.EXPECT().Rollback(gomock.Any()).Return(nil)

		result, err := svc.RegisterViaTelegram(context.Background(), &model.RegisterViaTelegramInput{
			TelegramId: 12345,
			Role:       model.RoleStudent,
		})
		require.NoError(t, err)
		assert.Equal(t, studentID, result.Id)
		assert.Equal(t, model.RoleStudent, result.Role)
	})

	t.Run("Success_Tutor_CreatesTutorProfile", func(t *testing.T) {
		svc, mockUserRepo, _, _ := setup(t)

		mockTx := mocks.NewMockUserCreationRepositoryTx(gomock.NewController(t))
		mockUserRepo.EXPECT().NewUserCreationRepositoryTx(gomock.Any()).Return(mockTx, nil)

		tutorID := uuid.New()
		mockTx.EXPECT().CreateUser(gomock.Any(), gomock.Any()).Return(&model.User{
			Id:   tutorID,
			Role: model.RoleTutor,
		}, nil)
		mockTx.EXPECT().CreateTelegramAccount(gomock.Any(), gomock.Any()).Return(&model.TelegramAccount{}, nil)
		mockTx.EXPECT().CreateTutorProfile(gomock.Any(), gomock.Any()).Return(&model.TutorProfile{}, nil)
		mockTx.EXPECT().Commit(gomock.Any()).Return(nil)
		mockTx.EXPECT().Rollback(gomock.Any()).Return(nil)

		result, err := svc.RegisterViaTelegram(context.Background(), &model.RegisterViaTelegramInput{
			TelegramId: 12345,
			Role:       model.RoleTutor,
		})
		require.NoError(t, err)
		assert.Equal(t, model.RoleTutor, result.Role)
	})

	t.Run("Error_CreateUserFails", func(t *testing.T) {
		svc, mockUserRepo, _, _ := setup(t)

		mockTx := mocks.NewMockUserCreationRepositoryTx(gomock.NewController(t))
		mockUserRepo.EXPECT().NewUserCreationRepositoryTx(gomock.Any()).Return(mockTx, nil)
		mockTx.EXPECT().CreateUser(gomock.Any(), gomock.Any()).Return(nil, errors.New("db error"))
		mockTx.EXPECT().Rollback(gomock.Any()).Return(nil)

		_, err := svc.RegisterViaTelegram(context.Background(), &model.RegisterViaTelegramInput{
			TelegramId: 12345,
			Role:       model.RoleStudent,
		})
		assert.Error(t, err)
	})
}

// ── Authorize ───────────────────────────────────────────────────────

func TestAuthorize(t *testing.T) {
	t.Run("InvalidPrefix", func(t *testing.T) {
		svc, _, _, _ := setup(t)

		_, err := svc.Authorize(context.Background(), &model.AuthorizeInput{
			AuthorizationHeader: "bearer some-token",
		})
		assert.ErrorIs(t, err, errdefs.ErrAuthentication)
	})

	t.Run("EmptyHeader", func(t *testing.T) {
		svc, _, _, _ := setup(t)

		_, err := svc.Authorize(context.Background(), &model.AuthorizeInput{
			AuthorizationHeader: "",
		})
		assert.ErrorIs(t, err, errdefs.ErrAuthentication)
	})
}
