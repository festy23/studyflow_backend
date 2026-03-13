ALTER TABLE receipts
    ADD COLUMN tutor_id   text NOT NULL DEFAULT '',
    ADD COLUMN student_id text NOT NULL DEFAULT '';

CREATE INDEX idx_receipts_tutor_id   ON receipts (tutor_id);
CREATE INDEX idx_receipts_student_id ON receipts (student_id);
