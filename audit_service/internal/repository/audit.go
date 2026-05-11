package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/georgysavva/scany/v2/pgxscan"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"audit_service/internal/domain"
)

// Querier defines pgxpool.Pool + pgxscan-compatible interface.
type Querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// AuditRepo stores audit log entries.
type AuditRepo struct {
	db Querier
}

// NewAuditRepository creates a new AuditRepo.
func NewAuditRepository(db Querier) *AuditRepo {
	return &AuditRepo{db: db}
}

const selectEvent = `SELECT id, user_id, action, resource_type, resource_id, details, ip_address, created_at FROM audit_log`

// CreateEvent inserts a new audit event and returns it.
func (r *AuditRepo) CreateEvent(ctx context.Context, input *domain.RecordEventInput) (*domain.AuditEvent, error) {
	query := `
		INSERT INTO audit_log (id, user_id, action, resource_type, resource_id, details, ip_address, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, user_id, action, resource_type, resource_id, details, ip_address, created_at
	`
	now := time.Now()
	eventID := uuid.New()

	event := &domain.AuditEvent{}
	err := pgxscan.Get(ctx, r.db, event, query,
		eventID,
		input.UserID,
		input.Action,
		input.ResourceType,
		input.ResourceID,
		input.Details,
		input.IPAddress,
		now,
	)
	if err != nil {
		return nil, fmt.Errorf("create event: %w", err)
	}
	return event, nil
}

// ListEvents returns audit events matching the filter with pagination.
func (r *AuditRepo) ListEvents(ctx context.Context, filter *domain.ListEventsInput) ([]*domain.AuditEvent, int64, error) {
	var where []string
	args := []any{}
	argIdx := 1

	if filter.UserID != "" {
		where = append(where, fmt.Sprintf("user_id = $%d", argIdx))
		args = append(args, filter.UserID)
		argIdx++
	}
	if filter.ResourceType != "" {
		where = append(where, fmt.Sprintf("resource_type = $%d", argIdx))
		args = append(args, filter.ResourceType)
		argIdx++
	}
	if filter.ResourceID != "" {
		where = append(where, fmt.Sprintf("resource_id = $%d", argIdx))
		args = append(args, filter.ResourceID)
		argIdx++
	}

	whereClause := ""
	if len(where) > 0 {
		whereClause = " WHERE " + strings.Join(where, " AND ")
	}

	// Count query
	countQuery := "SELECT COUNT(*) FROM audit_log" + whereClause
	var totalCount int64
	if err := r.db.QueryRow(ctx, countQuery, args...).Scan(&totalCount); err != nil {
		return nil, 0, fmt.Errorf("count events: %w", err)
	}

	// Data query
	query := selectEvent + whereClause + " ORDER BY created_at DESC"

	limit := filter.PageSize
	if limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
		args = append(args, limit, int32(filter.Page)*limit)
	}

	var events []*domain.AuditEvent
	if err := pgxscan.Select(ctx, r.db, &events, query, args...); err != nil {
		return nil, 0, fmt.Errorf("list events: %w", err)
	}
	return events, totalCount, nil
}
