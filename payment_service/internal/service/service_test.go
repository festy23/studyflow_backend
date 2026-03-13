package service_test

import (
	"common_library/ctxdata"
	"context"
	"errors"
	api2 "fileservice/pkg/api"
	errdefs "paymentservice/internal/errors"
	"paymentservice/internal/mocks"
	"paymentservice/internal/models"
	"paymentservice/internal/service"
	api "schedule_service/pkg/api"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

func setup(t *testing.T) (*gomock.Controller, *service.PaymentService, *mocks.MockIPaymentRepo, *mocks.MockUserServiceClient, *mocks.MockFileServiceClient, *mocks.MockScheduleServiceClient) {
	ctrl := gomock.NewController(t)

	mockRepo := mocks.NewMockIPaymentRepo(ctrl)
	mockUserClient := mocks.NewMockUserServiceClient(ctrl)
	mockFileClient := mocks.NewMockFileServiceClient(ctrl)
	mockScheduleClient := mocks.NewMockScheduleServiceClient(ctrl)

	svc := service.NewPaymentService(mockRepo, mockUserClient, mockFileClient, mockScheduleClient)
	return ctrl, svc, mockRepo, mockUserClient, mockFileClient, mockScheduleClient
}

func studentCtx() context.Context {
	ctx := context.Background()
	ctx = ctxdata.WithUserRole(ctx, string(models.RoleStudent))
	ctx = ctxdata.WithUserID(ctx, uuid.New().String())
	return ctx
}

func tutorCtx() context.Context {
	ctx := context.Background()
	ctx = ctxdata.WithUserRole(ctx, string(models.RoleTutor))
	ctx = ctxdata.WithUserID(ctx, uuid.New().String())
	return ctx
}

func tutorCtxWithID(id string) context.Context {
	ctx := context.Background()
	ctx = ctxdata.WithUserRole(ctx, string(models.RoleTutor))
	ctx = ctxdata.WithUserID(ctx, id)
	return ctx
}

func studentCtxWithID(id string) context.Context {
	ctx := context.Background()
	ctx = ctxdata.WithUserRole(ctx, string(models.RoleStudent))
	ctx = ctxdata.WithUserID(ctx, id)
	return ctx
}

func TestSubmitPaymentReceipt(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		ctrl, svc, mockRepo, _, mockFileClient, mockSchedule := setup(t)
		defer ctrl.Finish()

		lessonID := uuid.New()
		fileID := uuid.New()
		receiptID := uuid.New()

		// Get existing lesson
		mockSchedule.EXPECT().GetLesson(gomock.Any(), &api.GetLessonRequest{Id: lessonID.String()}).
			Return(&api.Lesson{
				Id:             lessonID.String(),
				ConnectionLink: proto.String("link"),
				PriceRub:       proto.Int32(100),
				PaymentInfo:    proto.String("info"),
			}, nil)

		// Validate file exists in file_service.
		mockFileClient.EXPECT().GetFileMeta(gomock.Any(), &api2.GetFileMetaRequest{FileId: fileID.String()}).
			Return(&api2.File{}, nil)

		// Resolve tutor from slot.
		mockSchedule.EXPECT().GetSlot(gomock.Any(), gomock.Any()).
			Return(&api.Slot{TutorId: uuid.New().String()}, nil)

		// Create receipt.
		mockRepo.EXPECT().CreateReceipt(gomock.Any(), gomock.AssignableToTypeOf(&models.PaymentReceiptCreateInput{})).
			Return(&models.PaymentReceipt{ID: receiptID, LessonID: lessonID, FileID: fileID, IsVerified: false}, nil)

		// Mark lesson as paid.
		mockSchedule.EXPECT().MarkAsPaid(gomock.Any(), &api.MarkAsPaidRequest{Id: lessonID.String()}).
			Return(&api.Lesson{Id: lessonID.String(), IsPaid: true}, nil)

		ctx := studentCtx()
		result, err := svc.SubmitPaymentReceipt(ctx, &models.SubmitPaymentReceiptInput{LessonId: lessonID, FileId: fileID})
		assert.NoError(t, err)
		assert.Equal(t, receiptID, result.ID)
	})

	t.Run("Error_InvalidInput", func(t *testing.T) {
		_, svc, _, _, _, _ := setup(t)

		testCases := []struct {
			name  string
			input *models.SubmitPaymentReceiptInput
		}{
			{"EmptyLessonID", &models.SubmitPaymentReceiptInput{FileId: uuid.New()}},
			{"EmptyFileID", &models.SubmitPaymentReceiptInput{LessonId: uuid.New()}},
			{"BothEmpty", &models.SubmitPaymentReceiptInput{}},
		}

		ctx := studentCtx()
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				_, err := svc.SubmitPaymentReceipt(ctx, tc.input)
				assert.True(t, errors.Is(err, errdefs.ErrInvalidArgument))
			})
		}
	})

	t.Run("Error_LessonAlreadyPaid", func(t *testing.T) {
		ctrl, svc, _, _, _, mockSchedule := setup(t)
		defer ctrl.Finish()

		lessonID := uuid.New()
		// Lesson already marked paid
		mockSchedule.EXPECT().GetLesson(gomock.Any(), gomock.Any()).Return(&api.Lesson{IsPaid: true}, nil)

		ctx := studentCtx()
		_, err := svc.SubmitPaymentReceipt(ctx, &models.SubmitPaymentReceiptInput{LessonId: lessonID, FileId: uuid.New()})
		assert.True(t, errors.Is(err, errdefs.ErrAlreadyExists))
	})

	t.Run("Error_MarkAsPaid", func(t *testing.T) {
		ctrl, svc, mockRepo, _, mockFileClient, mockSchedule := setup(t)
		defer ctrl.Finish()

		// Get lesson unpaid
		mockSchedule.EXPECT().GetLesson(gomock.Any(), gomock.Any()).Return(&api.Lesson{IsPaid: false}, nil)
		mockFileClient.EXPECT().GetFileMeta(gomock.Any(), gomock.Any()).Return(&api2.File{}, nil)
		mockSchedule.EXPECT().GetSlot(gomock.Any(), gomock.Any()).Return(&api.Slot{TutorId: uuid.New().String()}, nil)
		// Create receipt succeeds
		mockRepo.EXPECT().CreateReceipt(gomock.Any(), gomock.Any()).Return(&models.PaymentReceipt{}, nil)
		// Fail to mark paid
		mockSchedule.EXPECT().MarkAsPaid(gomock.Any(), gomock.Any()).Return(nil, errors.New("mark error"))

		ctx := studentCtx()
		_, err := svc.SubmitPaymentReceipt(ctx, &models.SubmitPaymentReceiptInput{LessonId: uuid.New(), FileId: uuid.New()})
		assert.EqualError(t, err, "mark error")
	})

	t.Run("Error_CreateReceipt", func(t *testing.T) {
		ctrl, svc, mockRepo, _, mockFileClient, mockSchedule := setup(t)
		defer ctrl.Finish()

		// Get lesson unpaid
		mockSchedule.EXPECT().GetLesson(gomock.Any(), gomock.Any()).Return(&api.Lesson{IsPaid: false}, nil)
		mockFileClient.EXPECT().GetFileMeta(gomock.Any(), gomock.Any()).Return(&api2.File{}, nil)
		mockSchedule.EXPECT().GetSlot(gomock.Any(), gomock.Any()).Return(&api.Slot{TutorId: uuid.New().String()}, nil)
		// DB error on create (non-retriable, returned as-is)
		mockRepo.EXPECT().CreateReceipt(gomock.Any(), gomock.Any()).Return(nil, errors.New("db error"))

		ctx := studentCtx()
		_, err := svc.SubmitPaymentReceipt(ctx, &models.SubmitPaymentReceiptInput{LessonId: uuid.New(), FileId: uuid.New()})
		assert.EqualError(t, err, "db error")
	})

	t.Run("Error_RequiresStudentRole", func(t *testing.T) {
		_, svc, _, _, _, _ := setup(t)

		// No role in context
		_, err := svc.SubmitPaymentReceipt(context.Background(), &models.SubmitPaymentReceiptInput{LessonId: uuid.New(), FileId: uuid.New()})
		assert.True(t, errors.Is(err, errdefs.ErrPermissionDenied))
	})

	t.Run("Error_TutorCannotSubmit", func(t *testing.T) {
		_, svc, _, _, _, _ := setup(t)

		ctx := tutorCtx()
		_, err := svc.SubmitPaymentReceipt(ctx, &models.SubmitPaymentReceiptInput{LessonId: uuid.New(), FileId: uuid.New()})
		assert.True(t, errors.Is(err, errdefs.ErrPermissionDenied))
	})

	t.Run("RetryLogic_SucceedsAfterRetries", func(t *testing.T) {
		ctrl, svc, mockRepo, _, mockFileClient, mockSchedule := setup(t)
		defer ctrl.Finish()

		lessonID := uuid.New()
		// Get lesson unpaid
		mockSchedule.EXPECT().GetLesson(gomock.Any(), gomock.Any()).Return(&api.Lesson{Id: lessonID.String(), IsPaid: false}, nil)
		mockFileClient.EXPECT().GetFileMeta(gomock.Any(), gomock.Any()).Return(&api2.File{}, nil)
		mockSchedule.EXPECT().GetSlot(gomock.Any(), gomock.Any()).Return(&api.Slot{TutorId: uuid.New().String()}, nil)

		retriable := status.Error(codes.Unavailable, "unavailable")
		mockRepo.EXPECT().CreateReceipt(gomock.Any(), gomock.Any()).Return(nil, retriable).Times(4)
		mockRepo.EXPECT().CreateReceipt(gomock.Any(), gomock.Any()).Return(&models.PaymentReceipt{}, nil).Times(1)

		// MarkAsPaid after successful create
		mockSchedule.EXPECT().MarkAsPaid(gomock.Any(), gomock.Any()).Return(&api.Lesson{IsPaid: true}, nil)

		ctx := studentCtx()
		_, err := svc.SubmitPaymentReceipt(ctx, &models.SubmitPaymentReceiptInput{LessonId: lessonID, FileId: uuid.New()})
		assert.NoError(t, err)
	})

	// BUG-017: file_id must be validated against file_service before persisting.
	t.Run("Error_UnknownFileID_InvalidArgument", func(t *testing.T) {
		ctrl, svc, mockRepo, _, mockFileClient, mockSchedule := setup(t)
		defer ctrl.Finish()

		mockSchedule.EXPECT().GetLesson(gomock.Any(), gomock.Any()).Return(&api.Lesson{IsPaid: false}, nil)
		mockFileClient.EXPECT().GetFileMeta(gomock.Any(), gomock.Any()).
			Return(nil, status.Error(codes.NotFound, "file not found"))
		// repo.CreateReceipt MUST NOT be called.
		_ = mockRepo

		ctx := studentCtx()
		_, err := svc.SubmitPaymentReceipt(ctx, &models.SubmitPaymentReceiptInput{LessonId: uuid.New(), FileId: uuid.New()})
		assert.True(t, errors.Is(err, errdefs.ErrInvalidArgument))
	})

	t.Run("Error_FilePermissionDenied", func(t *testing.T) {
		ctrl, svc, _, _, mockFileClient, mockSchedule := setup(t)
		defer ctrl.Finish()

		mockSchedule.EXPECT().GetLesson(gomock.Any(), gomock.Any()).Return(&api.Lesson{IsPaid: false}, nil)
		mockFileClient.EXPECT().GetFileMeta(gomock.Any(), gomock.Any()).
			Return(nil, status.Error(codes.PermissionDenied, "not the uploader"))

		ctx := studentCtx()
		_, err := svc.SubmitPaymentReceipt(ctx, &models.SubmitPaymentReceiptInput{LessonId: uuid.New(), FileId: uuid.New()})
		assert.True(t, errors.Is(err, errdefs.ErrPermissionDenied))
	})

	// BUG-008: concurrent duplicate submissions — second insert hits the UNIQUE
	// constraint and the repo returns ErrAlreadyExists which the service surfaces.
	t.Run("Error_DuplicateReceipt_UniqueViolation", func(t *testing.T) {
		ctrl, svc, mockRepo, _, mockFileClient, mockSchedule := setup(t)
		defer ctrl.Finish()

		lessonID := uuid.New()
		fileID := uuid.New()

		// First submission succeeds.
		mockSchedule.EXPECT().GetLesson(gomock.Any(), gomock.Any()).Return(&api.Lesson{Id: lessonID.String(), IsPaid: false}, nil)
		mockFileClient.EXPECT().GetFileMeta(gomock.Any(), gomock.Any()).Return(&api2.File{}, nil)
		mockSchedule.EXPECT().GetSlot(gomock.Any(), gomock.Any()).Return(&api.Slot{TutorId: uuid.New().String()}, nil)
		mockRepo.EXPECT().CreateReceipt(gomock.Any(), gomock.Any()).
			Return(&models.PaymentReceipt{ID: uuid.New(), LessonID: lessonID, FileID: fileID}, nil)
		mockSchedule.EXPECT().MarkAsPaid(gomock.Any(), gomock.Any()).Return(&api.Lesson{IsPaid: true}, nil)

		// Second submission races: lesson still appears unpaid (race window),
		// repo INSERT hits unique constraint -> ErrAlreadyExists.
		mockSchedule.EXPECT().GetLesson(gomock.Any(), gomock.Any()).Return(&api.Lesson{Id: lessonID.String(), IsPaid: false}, nil)
		mockFileClient.EXPECT().GetFileMeta(gomock.Any(), gomock.Any()).Return(&api2.File{}, nil)
		mockSchedule.EXPECT().GetSlot(gomock.Any(), gomock.Any()).Return(&api.Slot{TutorId: uuid.New().String()}, nil)
		mockRepo.EXPECT().CreateReceipt(gomock.Any(), gomock.Any()).Return(nil, errdefs.ErrAlreadyExists)

		ctx := studentCtx()
		_, err := svc.SubmitPaymentReceipt(ctx, &models.SubmitPaymentReceiptInput{LessonId: lessonID, FileId: fileID})
		assert.NoError(t, err)

		_, err = svc.SubmitPaymentReceipt(ctx, &models.SubmitPaymentReceiptInput{LessonId: lessonID, FileId: fileID})
		assert.True(t, errors.Is(err, errdefs.ErrAlreadyExists))
	})
}

func TestGetPaymentInfo(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		ctrl, svc, _, _, _, mockScheduleClient := setup(t)
		defer ctrl.Finish()

		lessonID := uuid.New()
		input := &models.GetPaymentInfoInput{LessonId: lessonID}

		mockScheduleClient.EXPECT().GetLesson(gomock.Any(), &api.GetLessonRequest{
			Id: lessonID.String(),
		}).Return(&api.Lesson{
			Id:          lessonID.String(),
			PriceRub:    proto.Int32(1500),
			PaymentInfo: proto.String("Payment instructions"),
		}, nil)

		info, err := svc.GetPaymentInfo(context.Background(), input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if info == nil || info.LessonID != lessonID || info.PriceRUB != 1500 {
			t.Fatal("invalid payment info returned")
		}
	})

	t.Run("Error_InvalidInput", func(t *testing.T) {
		_, svc, _, _, _, _ := setup(t)

		_, err := svc.GetPaymentInfo(context.Background(), &models.GetPaymentInfoInput{})
		if err == nil {
			t.Fatal("expected error for empty lesson ID")
		}
	})

	t.Run("Error_LessonNotFound", func(t *testing.T) {
		ctrl, svc, _, _, _, mockScheduleClient := setup(t)
		defer ctrl.Finish()

		lessonID := uuid.New()
		mockScheduleClient.EXPECT().GetLesson(gomock.Any(), gomock.Any()).Return(nil, errors.New("not found"))

		_, err := svc.GetPaymentInfo(context.Background(), &models.GetPaymentInfoInput{LessonId: lessonID})
		if err == nil {
			t.Fatal("expected error when lesson not found")
		}
	})
}

func TestGetReceipt(t *testing.T) {
	t.Run("Success_Tutor", func(t *testing.T) {
		ctrl, svc, mockRepo, _, _, mockSchedule := setup(t)
		defer ctrl.Finish()

		tutorID := uuid.New().String()
		receiptID := uuid.New()
		lessonID := uuid.New()
		slotID := uuid.New().String()
		input := &models.GetReceiptInput{ReceiptId: receiptID}

		receipt := &models.PaymentReceipt{
			ID:         receiptID,
			LessonID:   lessonID,
			FileID:     uuid.New(),
			IsVerified: true,
			CreatedAt:  time.Now(),
			EditedAt:   time.Now(),
		}

		mockRepo.EXPECT().GetReceiptByID(gomock.Any(), receiptID).Return(receipt, nil)
		mockSchedule.EXPECT().GetLesson(gomock.Any(), &api.GetLessonRequest{Id: lessonID.String()}).
			Return(&api.Lesson{Id: lessonID.String(), SlotId: slotID, StudentId: uuid.New().String()}, nil)
		mockSchedule.EXPECT().GetSlot(gomock.Any(), &api.GetSlotRequest{Id: slotID}).
			Return(&api.Slot{Id: slotID, TutorId: tutorID}, nil)

		result, err := svc.GetReceipt(tutorCtxWithID(tutorID), input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result == nil || result.ID != receiptID {
			t.Fatal("invalid receipt returned")
		}
	})

	t.Run("Error_InvalidInput", func(t *testing.T) {
		_, svc, _, _, _, _ := setup(t)

		_, err := svc.GetReceipt(tutorCtx(), &models.GetReceiptInput{})
		if err == nil {
			t.Fatal("expected error for empty receipt ID")
		}
	})

	t.Run("Error_ReceiptNotFound", func(t *testing.T) {
		ctrl, svc, mockRepo, _, _, _ := setup(t)
		defer ctrl.Finish()

		receiptID := uuid.New()
		mockRepo.EXPECT().GetReceiptByID(gomock.Any(), receiptID).Return(nil, errors.New("not found"))

		_, err := svc.GetReceipt(tutorCtx(), &models.GetReceiptInput{ReceiptId: receiptID})
		if err == nil {
			t.Fatal("expected error when receipt not found")
		}
	})

	t.Run("Error_UnrelatedCaller_PermissionDenied", func(t *testing.T) {
		ctrl, svc, mockRepo, _, _, mockSchedule := setup(t)
		defer ctrl.Finish()

		receiptID := uuid.New()
		lessonID := uuid.New()
		slotID := uuid.New().String()
		receipt := &models.PaymentReceipt{ID: receiptID, LessonID: lessonID, FileID: uuid.New()}

		mockRepo.EXPECT().GetReceiptByID(gomock.Any(), receiptID).Return(receipt, nil)
		mockSchedule.EXPECT().GetLesson(gomock.Any(), gomock.Any()).
			Return(&api.Lesson{Id: lessonID.String(), SlotId: slotID, StudentId: uuid.New().String()}, nil)
		mockSchedule.EXPECT().GetSlot(gomock.Any(), gomock.Any()).
			Return(&api.Slot{Id: slotID, TutorId: uuid.New().String()}, nil)

		_, err := svc.GetReceipt(tutorCtxWithID(uuid.New().String()), &models.GetReceiptInput{ReceiptId: receiptID})
		assert.True(t, errors.Is(err, errdefs.ErrPermissionDenied))
	})
}

func TestVerifyReceipt(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		ctrl, svc, mockRepo, _, _, mockSchedule := setup(t)
		defer ctrl.Finish()

		tutorID := uuid.New().String()
		receiptID := uuid.New()
		lessonID := uuid.New()
		slotID := uuid.New().String()
		input := &models.VerifyReceiptInput{ReceiptId: receiptID}

		existing := &models.PaymentReceipt{ID: receiptID, LessonID: lessonID}
		updatedReceipt := &models.PaymentReceipt{ID: receiptID, LessonID: lessonID, IsVerified: true}

		mockRepo.EXPECT().GetReceiptByID(gomock.Any(), receiptID).Return(existing, nil)
		mockSchedule.EXPECT().GetLesson(gomock.Any(), gomock.Any()).
			Return(&api.Lesson{Id: lessonID.String(), SlotId: slotID, StudentId: uuid.New().String()}, nil)
		mockSchedule.EXPECT().GetSlot(gomock.Any(), gomock.Any()).
			Return(&api.Slot{Id: slotID, TutorId: tutorID}, nil)
		mockRepo.EXPECT().UpdateReceipt(gomock.Any(), receiptID, true).Return(updatedReceipt, nil)

		ctx := tutorCtxWithID(tutorID)
		result, err := svc.VerifyReceipt(ctx, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result == nil || !result.IsVerified {
			t.Fatal("receipt not verified")
		}
	})

	t.Run("Error_InvalidInput", func(t *testing.T) {
		_, svc, _, _, _, _ := setup(t)

		ctx := tutorCtx()
		_, err := svc.VerifyReceipt(ctx, &models.VerifyReceiptInput{})
		if err == nil {
			t.Fatal("expected error for empty receipt ID")
		}
	})

	t.Run("Error_ReceiptNotFound", func(t *testing.T) {
		ctrl, svc, mockRepo, _, _, _ := setup(t)
		defer ctrl.Finish()

		receiptID := uuid.New()
		mockRepo.EXPECT().GetReceiptByID(gomock.Any(), receiptID).Return(nil, errors.New("not found"))

		ctx := tutorCtx()
		_, err := svc.VerifyReceipt(ctx, &models.VerifyReceiptInput{ReceiptId: receiptID})
		if err == nil {
			t.Fatal("expected error when receipt not found")
		}
	})

	t.Run("Error_RequiresTutorRole", func(t *testing.T) {
		_, svc, _, _, _, _ := setup(t)

		// No role in context
		_, err := svc.VerifyReceipt(context.Background(), &models.VerifyReceiptInput{ReceiptId: uuid.New()})
		assert.True(t, errors.Is(err, errdefs.ErrPermissionDenied))
	})

	t.Run("Error_StudentCannotVerify", func(t *testing.T) {
		_, svc, _, _, _, _ := setup(t)

		ctx := studentCtx()
		_, err := svc.VerifyReceipt(ctx, &models.VerifyReceiptInput{ReceiptId: uuid.New()})
		assert.True(t, errors.Is(err, errdefs.ErrPermissionDenied))
	})

	// BUG-009: IDOR — tutor B must not verify tutor A's receipt
	t.Run("Error_IDOR_OtherTutorDenied", func(t *testing.T) {
		ctrl, svc, mockRepo, _, _, mockSchedule := setup(t)
		defer ctrl.Finish()

		tutorA := uuid.New().String()
		tutorB := uuid.New().String()
		receiptID := uuid.New()
		lessonID := uuid.New()
		slotID := uuid.New().String()

		existing := &models.PaymentReceipt{ID: receiptID, LessonID: lessonID}
		mockRepo.EXPECT().GetReceiptByID(gomock.Any(), receiptID).Return(existing, nil)
		mockSchedule.EXPECT().GetLesson(gomock.Any(), gomock.Any()).
			Return(&api.Lesson{Id: lessonID.String(), SlotId: slotID, StudentId: uuid.New().String()}, nil)
		mockSchedule.EXPECT().GetSlot(gomock.Any(), gomock.Any()).
			Return(&api.Slot{Id: slotID, TutorId: tutorA}, nil)
		// repo.UpdateReceipt MUST NOT be called.

		_, err := svc.VerifyReceipt(tutorCtxWithID(tutorB), &models.VerifyReceiptInput{ReceiptId: receiptID})
		assert.True(t, errors.Is(err, errdefs.ErrPermissionDenied))
	})
}

func TestGetReceiptFile(t *testing.T) {
	t.Run("Success_Student", func(t *testing.T) {
		ctrl, svc, mockRepo, _, mockFileClient, mockSchedule := setup(t)
		defer ctrl.Finish()

		studentID := uuid.New().String()
		receiptID := uuid.New()
		fileID := uuid.New()
		lessonID := uuid.New()
		slotID := uuid.New().String()
		input := &models.GetReceiptFileInput{ReceiptId: receiptID}

		receipt := &models.PaymentReceipt{ID: receiptID, LessonID: lessonID, FileID: fileID, IsVerified: true}

		mockRepo.EXPECT().GetReceiptByID(gomock.Any(), receiptID).Return(receipt, nil)
		mockSchedule.EXPECT().GetLesson(gomock.Any(), gomock.Any()).
			Return(&api.Lesson{Id: lessonID.String(), SlotId: slotID, StudentId: studentID}, nil)
		mockSchedule.EXPECT().GetSlot(gomock.Any(), gomock.Any()).
			Return(&api.Slot{Id: slotID, TutorId: uuid.New().String()}, nil)
		mockFileClient.EXPECT().GenerateDownloadURL(gomock.Any(), &api2.GenerateDownloadURLRequest{
			FileId: fileID.String(),
		}).Return(&api2.DownloadURL{Url: "https://storage.example.com/file123"}, nil)

		result, err := svc.GetReceiptFile(studentCtxWithID(studentID), input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result == nil || result.URL != "https://storage.example.com/file123" {
			t.Fatal("invalid download URL returned")
		}
	})

	t.Run("Error_InvalidInput", func(t *testing.T) {
		_, svc, _, _, _, _ := setup(t)

		_, err := svc.GetReceiptFile(tutorCtx(), &models.GetReceiptFileInput{})
		if err == nil {
			t.Fatal("expected error for empty receipt ID")
		}
	})

	t.Run("Error_ReceiptNotFound", func(t *testing.T) {
		ctrl, svc, mockRepo, _, _, _ := setup(t)
		defer ctrl.Finish()

		mockRepo.EXPECT().
			GetReceiptByID(gomock.Any(), gomock.Any()).
			Return(nil, errors.New("not found"))

		_, err := svc.GetReceiptFile(tutorCtx(), &models.GetReceiptFileInput{ReceiptId: uuid.New()})
		if err == nil {
			t.Fatal("expected error when receipt missing")
		}
	})

	t.Run("Error_FileServiceUnavailable", func(t *testing.T) {
		ctrl, svc, mockRepo, _, mockFileClient, mockSchedule := setup(t)
		defer ctrl.Finish()

		tutorID := uuid.New().String()
		receiptID := uuid.New()
		fileID := uuid.New()
		lessonID := uuid.New()
		slotID := uuid.New().String()
		receipt := &models.PaymentReceipt{ID: receiptID, LessonID: lessonID, FileID: fileID, IsVerified: true}

		mockRepo.EXPECT().GetReceiptByID(gomock.Any(), receiptID).Return(receipt, nil)
		mockSchedule.EXPECT().GetLesson(gomock.Any(), gomock.Any()).
			Return(&api.Lesson{Id: lessonID.String(), SlotId: slotID, StudentId: uuid.New().String()}, nil)
		mockSchedule.EXPECT().GetSlot(gomock.Any(), gomock.Any()).
			Return(&api.Slot{Id: slotID, TutorId: tutorID}, nil)
		mockFileClient.EXPECT().GenerateDownloadURL(gomock.Any(), gomock.Any()).Return(nil, errors.New("service unavailable"))

		_, err := svc.GetReceiptFile(tutorCtxWithID(tutorID), &models.GetReceiptFileInput{ReceiptId: receiptID})
		if err == nil {
			t.Fatal("expected error when file service unavailable")
		}
	})

	t.Run("Error_UnrelatedCaller_PermissionDenied", func(t *testing.T) {
		ctrl, svc, mockRepo, _, _, mockSchedule := setup(t)
		defer ctrl.Finish()

		receiptID := uuid.New()
		lessonID := uuid.New()
		slotID := uuid.New().String()
		receipt := &models.PaymentReceipt{ID: receiptID, LessonID: lessonID, FileID: uuid.New()}

		mockRepo.EXPECT().GetReceiptByID(gomock.Any(), receiptID).Return(receipt, nil)
		mockSchedule.EXPECT().GetLesson(gomock.Any(), gomock.Any()).
			Return(&api.Lesson{Id: lessonID.String(), SlotId: slotID, StudentId: uuid.New().String()}, nil)
		mockSchedule.EXPECT().GetSlot(gomock.Any(), gomock.Any()).
			Return(&api.Slot{Id: slotID, TutorId: uuid.New().String()}, nil)

		_, err := svc.GetReceiptFile(tutorCtxWithID(uuid.New().String()), &models.GetReceiptFileInput{ReceiptId: receiptID})
		assert.True(t, errors.Is(err, errdefs.ErrPermissionDenied))
	})
}
