DROP INDEX IF EXISTS idx_files_orphan_uploads;

ALTER TABLE files
    DROP COLUMN IF EXISTS is_uploaded;
