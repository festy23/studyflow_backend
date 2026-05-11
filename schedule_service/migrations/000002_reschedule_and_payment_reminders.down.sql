DROP INDEX IF EXISTS idx_lessons_rescheduled_from;

ALTER TABLE lessons
    DROP COLUMN IF EXISTS rescheduled_from_lesson_id;
