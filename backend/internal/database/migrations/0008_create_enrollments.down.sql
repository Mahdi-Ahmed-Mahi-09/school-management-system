-- Rolls back Feature 08. WARNING: this deletes all enrollments and promotion history.
DROP TABLE IF EXISTS promotion_batch_items;
DROP TABLE IF EXISTS promotion_batches;
DROP TABLE IF EXISTS student_enrollments;
ALTER TABLE sections DROP CONSTRAINT IF EXISTS sections_id_year_key;
