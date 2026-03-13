ALTER TABLE receipts
    DROP COLUMN IF EXISTS tutor_id,
    DROP COLUMN IF EXISTS student_id;
