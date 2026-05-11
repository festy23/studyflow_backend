package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/georgysavva/scany/v2/pgxscan"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"faq_service/internal/domain"
	errdefs "faq_service/internal/errors"
)

type FAQRepo struct {
	pool *pgxpool.Pool
}

func NewFAQRepo(pool *pgxpool.Pool) *FAQRepo {
	return &FAQRepo{pool: pool}
}

func (r *FAQRepo) Create(ctx context.Context, faq *domain.FAQ) error {
	query := `
		INSERT INTO faqs (id, question, answer, category, sort_order, created_at, edited_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	id, err := uuid.NewV7()
	if err != nil {
		return fmt.Errorf("generate id: %w", err)
	}
	now := time.Now()
	faq.ID = id
	faq.CreatedAt = now
	faq.EditedAt = now

	_, err = r.pool.Exec(ctx, query, faq.ID, faq.Question, faq.Answer, faq.Category, faq.SortOrder, faq.CreatedAt, faq.EditedAt)
	if err != nil {
		return fmt.Errorf("create faq: %w", err)
	}
	return nil
}

func (r *FAQRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.FAQ, error) {
	query := `SELECT id, question, answer, category, sort_order, created_at, edited_at FROM faqs WHERE id = $1`
	var faq domain.FAQ
	if err := pgxscan.Get(ctx, r.pool, &faq, query, id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errdefs.ErrNotFound
		}
		return nil, fmt.Errorf("get faq: %w", err)
	}
	return &faq, nil
}

func (r *FAQRepo) Update(ctx context.Context, faq *domain.FAQ) error {
	query := `UPDATE faqs SET question = $1, answer = $2, category = $3, sort_order = $4, edited_at = $5 WHERE id = $6`
	faq.EditedAt = time.Now()
	res, err := r.pool.Exec(ctx, query, faq.Question, faq.Answer, faq.Category, faq.SortOrder, faq.EditedAt, faq.ID)
	if err != nil {
		return fmt.Errorf("update faq: %w", err)
	}
	if res.RowsAffected() == 0 {
		return errdefs.ErrNotFound
	}
	return nil
}

func (r *FAQRepo) Delete(ctx context.Context, id uuid.UUID) error {
	res, err := r.pool.Exec(ctx, `DELETE FROM faqs WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete faq: %w", err)
	}
	if res.RowsAffected() == 0 {
		return errdefs.ErrNotFound
	}
	return nil
}

func (r *FAQRepo) List(ctx context.Context, category *string, limit, offset int) ([]*domain.FAQ, int64, error) {
	var countQuery string
	var countArgs []interface{}
	where := ""
	if category != nil {
		where = " WHERE category = $1"
		countArgs = append(countArgs, *category)
	}
	countQuery = "SELECT COUNT(*) FROM faqs" + where
	var totalCount int64
	if err := r.pool.QueryRow(ctx, countQuery, countArgs...).Scan(&totalCount); err != nil {
		return nil, 0, fmt.Errorf("count faqs: %w", err)
	}

	query := "SELECT id, question, answer, category, sort_order, created_at, edited_at FROM faqs" + where + " ORDER BY sort_order ASC, created_at DESC"
	args := countArgs
	if limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", len(args)+1, len(args)+2)
		args = append(args, limit, offset)
	}

	var faqs []*domain.FAQ
	if err := pgxscan.Select(ctx, r.pool, &faqs, query, args...); err != nil {
		return nil, 0, fmt.Errorf("list faqs: %w", err)
	}
	return faqs, totalCount, nil
}

func (r *FAQRepo) ListCategories(ctx context.Context) ([]string, error) {
	query := `SELECT DISTINCT category FROM faqs WHERE category IS NOT NULL ORDER BY category`
	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list categories: %w", err)
	}
	defer rows.Close()

	var categories []string
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			return nil, fmt.Errorf("scan category: %w", err)
		}
		categories = append(categories, c)
	}
	return categories, rows.Err()
}

func appendPagination(query string, args []interface{}, limit, offset int) (string, []interface{}) {
	if limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", len(args)+1, len(args)+2)
		args = append(args, limit, offset)
	}
	return query, args
}

var _ = strings.Join // suppress unused import warning
