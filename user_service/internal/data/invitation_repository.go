package data

import (
	"context"

	"github.com/georgysavva/scany/v2/pgxscan"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"userservice/internal/errdefs"
	"userservice/internal/model"
)

type InvitationRepository struct {
	db *pgxpool.Pool
}

func NewInvitationRepository(db *pgxpool.Pool) *InvitationRepository {
	return &InvitationRepository{db: db}
}

func (r *InvitationRepository) CreateInvitation(ctx context.Context, inv *model.Invitation) (*model.Invitation, error) {
	query := `
	INSERT INTO invitations (id, tutor_id, token, status)
	VALUES ($1, $2, $3, $4)
	RETURNING id, tutor_id, token, status, created_at, edited_at
	`
	var result model.Invitation
	err := pgxscan.Get(ctx, r.db, &result, query,
		inv.Id,
		inv.TutorId,
		inv.Token,
		inv.Status,
	)
	if err != nil {
		return nil, handleError(err)
	}
	return &result, nil
}

func (r *InvitationRepository) GetInvitationByID(ctx context.Context, id uuid.UUID) (*model.Invitation, error) {
	query := `
	SELECT id, tutor_id, token, status, created_at, edited_at
	FROM invitations
	WHERE id = $1
	`
	var inv model.Invitation
	err := pgxscan.Get(ctx, r.db, &inv, query, id)
	if err != nil {
		return nil, handleError(err)
	}
	return &inv, nil
}

func (r *InvitationRepository) GetInvitationByToken(ctx context.Context, token uuid.UUID) (*model.Invitation, error) {
	query := `
	SELECT id, tutor_id, token, status, created_at, edited_at
	FROM invitations
	WHERE token = $1
	`
	var inv model.Invitation
	err := pgxscan.Get(ctx, r.db, &inv, query, token)
	if err != nil {
		return nil, handleError(err)
	}
	return &inv, nil
}

func (r *InvitationRepository) ListInvitationsByTutor(ctx context.Context, tutorId uuid.UUID) ([]*model.Invitation, error) {
	query := `
	SELECT id, tutor_id, token, status, created_at, edited_at
	FROM invitations
	WHERE tutor_id = $1
	ORDER BY created_at DESC
	`
	var rows []*model.Invitation
	err := pgxscan.Select(ctx, r.db, &rows, query, tutorId)
	if err != nil {
		return nil, handleError(err)
	}
	return rows, nil
}

// MarkInvitationUsedIfActive atomically transitions an invitation from
// "active" to "used". It returns true only if this call performed the
// transition, so concurrent accepts of the same token cannot both succeed.
func (r *InvitationRepository) MarkInvitationUsedIfActive(ctx context.Context, id uuid.UUID) (bool, error) {
	query := `
	UPDATE invitations
	SET status = $1, edited_at = NOW()
	WHERE id = $2 AND status = $3
	`
	result, err := r.db.Exec(ctx, query, model.InvitationStatusUsed, id, model.InvitationStatusActive)
	if err != nil {
		return false, handleError(err)
	}
	return result.RowsAffected() > 0, nil
}

func (r *InvitationRepository) UpdateInvitationStatus(ctx context.Context, id uuid.UUID, status model.InvitationStatus) error {
	query := `
	UPDATE invitations
	SET status = $1, edited_at = NOW()
	WHERE id = $2
	`
	result, err := r.db.Exec(ctx, query, status, id)
	if err != nil {
		return handleError(err)
	}
	if result.RowsAffected() == 0 {
		return errdefs.ErrNotFound
	}
	return nil
}
