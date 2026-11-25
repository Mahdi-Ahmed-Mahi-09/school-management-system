-- Rolls back Feature 09. WARNING: this deletes all teacher assignments and their history.
DROP TABLE IF EXISTS teacher_assignments;
ALTER TABLE sections DROP CONSTRAINT IF EXISTS sections_id_class_year_key;
