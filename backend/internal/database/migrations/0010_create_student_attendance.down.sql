DROP TABLE IF EXISTS student_attendance_changes;
DROP TABLE IF EXISTS student_attendance;
DROP FUNCTION IF EXISTS student_attendance_check_date();
ALTER TABLE student_enrollments DROP CONSTRAINT IF EXISTS student_enrollments_id_student_year_key;
