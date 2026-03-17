ALTER TABLE lessons
    ADD COLUMN IF NOT EXISTS rescheduled_from_lesson_id UUID REFERENCES lessons(id);

CREATE INDEX IF NOT EXISTS idx_lessons_rescheduled_from
    ON lessons(rescheduled_from_lesson_id)
    WHERE rescheduled_from_lesson_id IS NOT NULL;
