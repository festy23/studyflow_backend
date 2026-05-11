ALTER TABLE files
    ADD COLUMN IF NOT EXISTS is_uploaded BOOLEAN NOT NULL DEFAULT FALSE;

CREATE INDEX IF NOT EXISTS idx_files_orphan_uploads
    ON files(created_at)
    WHERE is_uploaded = FALSE;
