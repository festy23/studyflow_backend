CREATE INDEX idx_receipts_lesson_id ON receipts(lesson_id);
CREATE INDEX idx_receipts_is_verified ON receipts(is_verified) WHERE is_verified = false;
