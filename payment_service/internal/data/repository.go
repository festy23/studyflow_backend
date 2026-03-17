package data

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/georgysavva/scany/v2/pgxscan"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	_ "github.com/jackc/pgx/v5/pgxpool"

	errdefs "paymentservice/internal/errors"
	"paymentservice/internal/models"
)

// Querier defines pgxpool.Pool + pgxscan-compatible interface.
type Querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// PaymentRepo stores receipts.
type PaymentRepo struct {
	db Querier
}

// NewPaymentRepository creates a new PaymentRepo.
func NewPaymentRepository(db Querier) *PaymentRepo {
	return &PaymentRepo{db: db}
}

const selectReceipt = `SELECT id, lesson_id, file_id, tutor_id, student_id, is_verified, price_rub, created_at, edited_at FROM receipts`

// CreateReceipt inserts a new receipt and returns it.
func (r *PaymentRepo) CreateReceipt(ctx context.Context, input *models.PaymentReceiptCreateInput) (*models.PaymentReceipt, error) {
	query := `
		INSERT INTO receipts (id, lesson_id, file_id, tutor_id, student_id, is_verified, price_rub, created_at, edited_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, lesson_id, file_id, tutor_id, student_id, is_verified, price_rub, created_at, edited_at
	`
	now := time.Now()
	pr := &models.PaymentReceipt{}
	err := pgxscan.Get(ctx, r.db, pr, query,
		input.ID,
		input.LessonID,
		input.FileID,
		input.TutorID,
		input.StudentID,
		input.IsVerified,
		input.PriceRub,
		now,
		now,
	)
	if err != nil {
		return nil, handleError(err)
	}
	return pr, nil
}

// GetReceiptByID retrieves a receipt by ID.
func (r *PaymentRepo) GetReceiptByID(ctx context.Context, id uuid.UUID) (*models.PaymentReceipt, error) {
	query := selectReceipt + ` WHERE id = $1`
	pr := &models.PaymentReceipt{}
	err := pgxscan.Get(ctx, r.db, pr, query, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errdefs.ErrNotFound
		}
		return nil, handleError(err)
	}
	return pr, nil
}

// UpdateReceipt updates verification and returns receipt.
func (r *PaymentRepo) UpdateReceipt(ctx context.Context, id uuid.UUID, isVerified bool) (*models.PaymentReceipt, error) {
	query := `UPDATE receipts SET is_verified = $1, edited_at = $2 WHERE id = $3`
	now := time.Now()
	cmdTag, err := r.db.Exec(ctx, query, isVerified, now, id)
	if err != nil {
		return nil, handleError(err)
	}
	if cmdTag.RowsAffected() == 0 {
		return nil, errdefs.ErrNotFound
	}
	return r.GetReceiptByID(ctx, id)
}

// ExistsByID checks existence by ID.
func (r *PaymentRepo) ExistsByID(ctx context.Context, id uuid.UUID) (bool, error) {
	query := `SELECT EXISTS (SELECT 1 FROM receipts WHERE id = $1)`
	var exists bool
	err := r.db.QueryRow(ctx, query, id).Scan(&exists)
	if err != nil {
		return false, handleError(err)
	}
	return exists, nil
}

// GetReceiptByLessonID retrieves a receipt by lesson ID.
func (r *PaymentRepo) GetReceiptByLessonID(ctx context.Context, lessonID uuid.UUID) (*models.PaymentReceipt, error) {
	query := selectReceipt + ` WHERE lesson_id = $1`
	pr := &models.PaymentReceipt{}
	err := pgxscan.Get(ctx, r.db, pr, query, lessonID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errdefs.ErrNotFound
		}
		return nil, handleError(err)
	}
	return pr, nil
}

// ListReceiptsByTutor returns all receipts for a tutor ordered by created_at desc.
func (r *PaymentRepo) ListReceiptsByTutor(ctx context.Context, tutorID string) ([]*models.PaymentReceipt, error) {
	query := selectReceipt + ` WHERE tutor_id = $1 ORDER BY created_at DESC`
	var receipts []*models.PaymentReceipt
	if err := pgxscan.Select(ctx, r.db, &receipts, query, tutorID); err != nil {
		return nil, handleError(err)
	}
	return receipts, nil
}

// ListReceiptsByStudent returns all receipts for a student ordered by created_at desc.
func (r *PaymentRepo) ListReceiptsByStudent(ctx context.Context, studentID string) ([]*models.PaymentReceipt, error) {
	query := selectReceipt + ` WHERE student_id = $1 ORDER BY created_at DESC`
	var receipts []*models.PaymentReceipt
	if err := pgxscan.Select(ctx, r.db, &receipts, query, studentID); err != nil {
		return nil, handleError(err)
	}
	return receipts, nil
}

func (r *PaymentRepo) GetTutorRevenue(ctx context.Context, tutorID string, from, to *time.Time) (int64, error) {
	query := `SELECT COALESCE(SUM(price_rub), 0) FROM receipts WHERE tutor_id = $1`
	args := []any{tutorID}
	if from != nil {
		args = append(args, *from)
		query += ` AND created_at >= $` + strconv.Itoa(len(args))
	}
	if to != nil {
		args = append(args, *to)
		query += ` AND created_at <= $` + strconv.Itoa(len(args))
	}

	var revenue int64
	if err := r.db.QueryRow(ctx, query, args...).Scan(&revenue); err != nil {
		return 0, handleError(err)
	}
	return revenue, nil
}

func (r *PaymentRepo) PaymentReminderSent(ctx context.Context, lessonID uuid.UUID) (bool, error) {
	query := `SELECT EXISTS (SELECT 1 FROM payment_reminders WHERE lesson_id = $1)`
	var exists bool
	if err := r.db.QueryRow(ctx, query, lessonID).Scan(&exists); err != nil {
		return false, handleError(err)
	}
	return exists, nil
}

func (r *PaymentRepo) MarkPaymentReminderSent(ctx context.Context, lessonID uuid.UUID) error {
	query := `
		INSERT INTO payment_reminders (lesson_id, sent_at)
		VALUES ($1, NOW())
		ON CONFLICT (lesson_id) DO UPDATE SET sent_at = EXCLUDED.sent_at
	`
	if _, err := r.db.Exec(ctx, query, lessonID); err != nil {
		return handleError(err)
	}
	return nil
}
