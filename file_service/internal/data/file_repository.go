package data

import (
	"context"
	"fileservice/internal/model"
	"github.com/georgysavva/scany/v2/pgxscan"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"time"
)

type FileRepository struct {
	db *pgxpool.Pool
}

func NewFileRepository(db *pgxpool.Pool) *FileRepository {
	return &FileRepository{db: db}
}

func (r *FileRepository) CreateFile(ctx context.Context, input *model.RepositoryCreateFileInput) (*model.File, error) {
	query := `
INSERT INTO files (
 id, extension, uploaded_by, filename
)
VALUES ($1, $2, $3, $4)
RETURNING id, extension, uploaded_by, filename, created_at, is_uploaded
`
	var file model.File
	err := pgxscan.Get(ctx, r.db, &file, query,
		input.Id,
		input.Extension,
		input.UploadedBy,
		input.Filename,
	)
	if err != nil {
		return nil, err
	}
	return &file, nil
}

func (r *FileRepository) GetFile(ctx context.Context, fileId uuid.UUID) (*model.File, error) {
	query := `
SELECT id, extension, uploaded_by, filename, created_at, is_uploaded
FROM files
WHERE id = $1
`
	var file model.File
	err := pgxscan.Get(ctx, r.db, &file, query, fileId)
	if err != nil {
		return nil, err
	}

	return &file, nil
}

func (r *FileRepository) ConfirmUpload(ctx context.Context, fileID uuid.UUID) (*model.File, error) {
	query := `
UPDATE files
SET is_uploaded = TRUE
WHERE id = $1
RETURNING id, extension, uploaded_by, filename, created_at, is_uploaded
`
	var file model.File
	if err := pgxscan.Get(ctx, r.db, &file, query, fileID); err != nil {
		return nil, err
	}
	return &file, nil
}

func (r *FileRepository) ListOrphanUploads(ctx context.Context, olderThan time.Time) ([]*model.File, error) {
	query := `
SELECT id, extension, uploaded_by, filename, created_at, is_uploaded
FROM files
WHERE is_uploaded = FALSE AND created_at < $1
`
	var files []*model.File
	if err := pgxscan.Select(ctx, r.db, &files, query, olderThan); err != nil {
		return nil, err
	}
	return files, nil
}

func (r *FileRepository) DeleteFile(ctx context.Context, fileID uuid.UUID) error {
	_, err := r.db.Exec(ctx, `DELETE FROM files WHERE id = $1`, fileID)
	return err
}
