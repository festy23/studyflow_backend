//go:generate mockgen -source=service.go -destination=../mocks/payment_mocks.go -package=mocks

package service

import (
	"common_library/ctxdata"
	"common_library/utils"
	"context"
	api2 "fileservice/pkg/api"
	"fmt"
	"github.com/google/uuid"
	"google.golang.org/grpc/metadata"
	"paymentservice/internal/clients"
	errdefs "paymentservice/internal/errors"
	"paymentservice/internal/models"
	api3 "schedule_service/pkg/api"
	"time"
)

const maxRetries = 6                      // Максимальное количество попыток
const retryDelay = 100 * time.Millisecond // Задержка между попытками

type IPaymentRepo interface {
	CreateReceipt(ctx context.Context, receipt *models.PaymentReceiptCreateInput) (*models.PaymentReceipt, error)

	GetReceiptByID(ctx context.Context, id uuid.UUID) (*models.PaymentReceipt, error)

	UpdateReceipt(ctx context.Context, id uuid.UUID, isVerified bool) (*models.PaymentReceipt, error)

	ExistsByID(ctx context.Context, id uuid.UUID) (bool, error)

	GetReceiptByLessonID(ctx context.Context, lessonID uuid.UUID) (*models.PaymentReceipt, error)
}

type PaymentService struct {
	repo           IPaymentRepo
	userClient     clients.UserServiceClient
	fileClient     clients.FileServiceClient
	scheduleClient clients.ScheduleServiceClient
}

func NewPaymentService(
	repo IPaymentRepo,
	userClient clients.UserServiceClient,
	fileClient clients.FileServiceClient,
	scheduleClient clients.ScheduleServiceClient,
) *PaymentService {

	return &PaymentService{
		repo:           repo,
		userClient:     userClient,
		fileClient:     fileClient,
		scheduleClient: scheduleClient,
	}
}

func requireRole(ctx context.Context, role models.Role) error {
	userRole, ok := ctxdata.GetUserRole(ctx)
	if !ok {
		return errdefs.ErrPermissionDenied
	}
	if models.Role(userRole) != role {
		return errdefs.ErrPermissionDenied
	}
	return nil
}

func (s *PaymentService) SubmitPaymentReceipt(ctx context.Context, input *models.SubmitPaymentReceiptInput) (*models.PaymentReceipt, error) {
	if err := requireRole(ctx, models.RoleStudent); err != nil {
		return nil, err
	}
	if input.FileId == uuid.Nil || input.LessonId == uuid.Nil {
		return nil, errdefs.ErrInvalidArgument
	}

	getLessonRequest := &api3.GetLessonRequest{
		Id: input.LessonId.String(),
	}

	lesson, err := utils.RetryWithBackoff[*api3.Lesson](ctx, maxRetries, retryDelay, func() (*api3.Lesson, error) {
		return s.scheduleClient.GetLesson(ctxWithMetadata(ctx), getLessonRequest)
	})
	if err != nil {
		return nil, err
	}

	if lesson.IsPaid {
		return nil, errdefs.ErrAlreadyExists
	}

	newReceiptID, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("failed to generate receipt ID: %w", err)
	}

	createReceiptInput := &models.PaymentReceiptCreateInput{
		ID:         newReceiptID,
		LessonID:   input.LessonId,
		FileID:     input.FileId,
		IsVerified: false,
	}
	receipt, err := utils.RetryWithBackoff(ctx, maxRetries, retryDelay, func() (*models.PaymentReceipt, error) {
		return s.repo.CreateReceipt(ctxWithMetadata(ctx), createReceiptInput)
	})
	if err != nil {
		return nil, err
	}

	req := &api3.MarkAsPaidRequest{
		Id: input.LessonId.String(),
	}
	_, err = utils.RetryWithBackoff[*api3.Lesson](ctx, maxRetries, retryDelay, func() (*api3.Lesson, error) {
		return s.scheduleClient.MarkAsPaid(ctxWithMetadata(ctx), req)
	})
	if err != nil {
		return nil, err
	}

	// отправить ивент уведомление

	return receipt, nil
}

func (s *PaymentService) GetPaymentInfo(ctx context.Context, input *models.GetPaymentInfoInput) (*models.PaymentInfo, error) {
	if input.LessonId == uuid.Nil {
		return nil, errdefs.ErrInvalidArgument
	}

	getLessonRequest := &api3.GetLessonRequest{
		Id: input.LessonId.String(),
	}

	lesson, err := utils.RetryWithBackoff(ctx, maxRetries, retryDelay, func() (*api3.Lesson, error) {
		return s.scheduleClient.GetLesson(ctxWithMetadata(ctx), getLessonRequest)
	})
	if err != nil {
		return nil, err
	}

	paymentInfo := &models.PaymentInfo{
		LessonID: input.LessonId,
	}

	if lesson.PriceRub != nil {
		paymentInfo.PriceRUB = *lesson.PriceRub
	}
	if lesson.PaymentInfo != nil {
		paymentInfo.PaymentDetails = *lesson.PaymentInfo
	}
	return paymentInfo, nil
}
func (s *PaymentService) GetReceipt(ctx context.Context, input *models.GetReceiptInput) (*models.PaymentReceipt, error) {
	if input.ReceiptId == uuid.Nil {
		return nil, errdefs.ErrInvalidArgument
	}

	receipt, err := utils.RetryWithBackoff[*models.PaymentReceipt](ctx, maxRetries, retryDelay, func() (*models.PaymentReceipt, error) {
		return s.repo.GetReceiptByID(ctxWithMetadata(ctx), input.ReceiptId)
	})
	if err != nil {
		return nil, err
	}
	return receipt, nil
}

func (s *PaymentService) VerifyReceipt(ctx context.Context, input *models.VerifyReceiptInput) (*models.PaymentReceipt, error) {
	if err := requireRole(ctx, models.RoleTutor); err != nil {
		return nil, err
	}
	if input.ReceiptId == uuid.Nil {
		return nil, errdefs.ErrInvalidArgument
	}
	receipt, err := utils.RetryWithBackoff(ctx, maxRetries, retryDelay, func() (*models.PaymentReceipt, error) {
		return s.repo.UpdateReceipt(ctx, input.ReceiptId, true)
	})
	if err != nil {
		return nil, err
	}
	return receipt, nil
}

func (s *PaymentService) GetReceiptFile(ctx context.Context, input *models.GetReceiptFileInput) (*models.ReceiptFileUrl, error) {
	if input.ReceiptId == uuid.Nil {
		return nil, errdefs.ErrInvalidArgument
	}
	receipt, err := utils.RetryWithBackoff[*models.PaymentReceipt](ctx, maxRetries, retryDelay, func() (*models.PaymentReceipt, error) {
		return s.repo.GetReceiptByID(ctx, input.ReceiptId)
	})
	if err != nil {
		return nil, err
	}
	generateDownloadURLRequest := &api2.GenerateDownloadURLRequest{FileId: receipt.FileID.String()}
	url, err := s.fileClient.GenerateDownloadURL(ctxWithMetadata(ctx), generateDownloadURLRequest)
	if err != nil {
		return nil, err
	}
	receiptFileURL := &models.ReceiptFileUrl{
		URL: url.GetUrl(),
	}
	return receiptFileURL, nil
}

func ctxWithMetadata(ctx context.Context) context.Context {
	reqCtx := metadata.NewOutgoingContext(ctx, metadata.Pairs())
	if userId, ok := ctxdata.GetUserID(ctx); ok {
		reqCtx = metadata.AppendToOutgoingContext(reqCtx, "x-user-id", userId)
	}
	if userRole, ok := ctxdata.GetUserRole(ctx); ok {
		reqCtx = metadata.AppendToOutgoingContext(reqCtx, "x-user-role", userRole)
	}

	return reqCtx
}
