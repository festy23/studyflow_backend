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

func TestSubmitPaymentReceipt(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		ctrl, svc, mockRepo, _, _, mockSchedule := setup(t)
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

		// Create receipt (now happens BEFORE marking as paid)
		mockRepo.EXPECT().CreateReceipt(gomock.Any(), gomock.AssignableToTypeOf(&models.PaymentReceiptCreateInput{})).
			Return(&models.PaymentReceipt{ID: receiptID, LessonID: lessonID, FileID: fileID, IsVerified: false}, nil)

		// Mark lesson as paid (now happens AFTER creating receipt)
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
		ctrl, svc, mockRepo, _, _, mockSchedule := setup(t)
		defer ctrl.Finish()

		// Get lesson unpaid
		mockSchedule.EXPECT().GetLesson(gomock.Any(), gomock.Any()).Return(&api.Lesson{IsPaid: false}, nil)
		// Create receipt succeeds
		mockRepo.EXPECT().CreateReceipt(gomock.Any(), gomock.Any()).Return(&models.PaymentReceipt{}, nil)
		// Fail to mark paid
		mockSchedule.EXPECT().MarkAsPaid(gomock.Any(), gomock.Any()).Return(nil, errors.New("mark error"))

		ctx := studentCtx()
		_, err := svc.SubmitPaymentReceipt(ctx, &models.SubmitPaymentReceiptInput{LessonId: uuid.New(), FileId: uuid.New()})
		assert.EqualError(t, err, "mark error")
	})

	t.Run("Error_CreateReceipt", func(t *testing.T) {
		ctrl, svc, mockRepo, _, _, mockSchedule := setup(t)
		defer ctrl.Finish()

		// Get lesson unpaid
		mockSchedule.EXPECT().GetLesson(gomock.Any(), gomock.Any()).Return(&api.Lesson{IsPaid: false}, nil)
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
		ctrl, svc, mockRepo, _, _, mockSchedule := setup(t)
		defer ctrl.Finish()

		lessonID := uuid.New()
		// Get lesson unpaid
		mockSchedule.EXPECT().GetLesson(gomock.Any(), gomock.Any()).Return(&api.Lesson{Id: lessonID.String(), IsPaid: false}, nil)

		retriable := status.Error(codes.Unavailable, "unavailable")
		mockRepo.EXPECT().CreateReceipt(gomock.Any(), gomock.Any()).Return(nil, retriable).Times(4)
		mockRepo.EXPECT().CreateReceipt(gomock.Any(), gomock.Any()).Return(&models.PaymentReceipt{}, nil).Times(1)

		// MarkAsPaid after successful create
		mockSchedule.EXPECT().MarkAsPaid(gomock.Any(), gomock.Any()).Return(&api.Lesson{IsPaid: true}, nil)

		ctx := studentCtx()
		_, err := svc.SubmitPaymentReceipt(ctx, &models.SubmitPaymentReceiptInput{LessonId: lessonID, FileId: uuid.New()})
		assert.NoError(t, err)
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
	t.Run("Success", func(t *testing.T) {
		ctrl, svc, mockRepo, _, _, _ := setup(t)
		defer ctrl.Finish()

		receiptID := uuid.New()
		input := &models.GetReceiptInput{ReceiptId: receiptID}

		receipt := &models.PaymentReceipt{
			ID:         receiptID,
			LessonID:   uuid.New(),
			FileID:     uuid.New(),
			IsVerified: true,
			CreatedAt:  time.Now(),
			EditedAt:   time.Now(),
		}

		mockRepo.EXPECT().GetReceiptByID(gomock.Any(), receiptID).Return(receipt, nil)

		result, err := svc.GetReceipt(context.Background(), input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result == nil || result.ID != receiptID {
			t.Fatal("invalid receipt returned")
		}
	})

	t.Run("Error_InvalidInput", func(t *testing.T) {
		_, svc, _, _, _, _ := setup(t)

		_, err := svc.GetReceipt(context.Background(), &models.GetReceiptInput{})
		if err == nil {
			t.Fatal("expected error for empty receipt ID")
		}
	})

	t.Run("Error_ReceiptNotFound", func(t *testing.T) {
		ctrl, svc, mockRepo, _, _, _ := setup(t)
		defer ctrl.Finish()

		receiptID := uuid.New()
		mockRepo.EXPECT().GetReceiptByID(gomock.Any(), receiptID).Return(nil, errors.New("not found"))

		_, err := svc.GetReceipt(context.Background(), &models.GetReceiptInput{ReceiptId: receiptID})
		if err == nil {
			t.Fatal("expected error when receipt not found")
		}
	})
}

func TestVerifyReceipt(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		ctrl, svc, mockRepo, _, _, _ := setup(t)
		defer ctrl.Finish()

		receiptID := uuid.New()
		input := &models.VerifyReceiptInput{ReceiptId: receiptID}

		updatedReceipt := &models.PaymentReceipt{
			ID:         receiptID,
			IsVerified: true,
		}

		mockRepo.EXPECT().UpdateReceipt(gomock.Any(), receiptID, true).Return(updatedReceipt, nil)

		ctx := tutorCtx()
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
		mockRepo.EXPECT().UpdateReceipt(gomock.Any(), receiptID, true).Return(nil, errors.New("not found"))

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
}

func TestGetReceiptFile(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		ctrl, svc, mockRepo, _, mockFileClient, _ := setup(t)
		defer ctrl.Finish()

		receiptID := uuid.New()
		fileID := uuid.New()
		input := &models.GetReceiptFileInput{ReceiptId: receiptID}

		receipt := &models.PaymentReceipt{
			ID:         receiptID,
			FileID:     fileID,
			IsVerified: true,
		}

		mockRepo.EXPECT().GetReceiptByID(gomock.Any(), receiptID).Return(receipt, nil)
		mockFileClient.EXPECT().GenerateDownloadURL(gomock.Any(), &api2.GenerateDownloadURLRequest{
			FileId: fileID.String(),
		}).Return(&api2.DownloadURL{
			Url: "https://storage.example.com/file123",
		}, nil)

		result, err := svc.GetReceiptFile(context.Background(), input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result == nil || result.URL != "https://storage.example.com/file123" {
			t.Fatal("invalid download URL returned")
		}
	})

	t.Run("Error_InvalidInput", func(t *testing.T) {
		_, svc, _, _, _, _ := setup(t)

		_, err := svc.GetReceiptFile(context.Background(), &models.GetReceiptFileInput{})
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

		_, err := svc.GetReceiptFile(context.Background(), &models.GetReceiptFileInput{ReceiptId: uuid.New()})
		if err == nil {
			t.Fatal("expected error when receipt missing")
		}
	})

	t.Run("Error_FileServiceUnavailable", func(t *testing.T) {
		ctrl, svc, mockRepo, _, mockFileClient, _ := setup(t)
		defer ctrl.Finish()

		receiptID := uuid.New()
		fileID := uuid.New()
		receipt := &models.PaymentReceipt{
			ID:         receiptID,
			FileID:     fileID,
			IsVerified: true,
		}

		mockRepo.EXPECT().GetReceiptByID(gomock.Any(), receiptID).Return(receipt, nil)
		mockFileClient.EXPECT().GenerateDownloadURL(gomock.Any(), gomock.Any()).Return(nil, errors.New("service unavailable"))

		_, err := svc.GetReceiptFile(context.Background(), &models.GetReceiptFileInput{ReceiptId: receiptID})
		if err == nil {
			t.Fatal("expected error when file service unavailable")
		}
	})
}
