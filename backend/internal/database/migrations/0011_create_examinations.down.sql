DROP TABLE IF EXISTS examination_schedules;
DROP FUNCTION IF EXISTS examination_schedules_check_date();
DROP TABLE IF EXISTS examination_classes;
DROP TABLE IF EXISTS examinations;
DROP FUNCTION IF EXISTS examinations_check_year_dates();
DROP TABLE IF EXISTS exam_types;
-- btree_gist is left installed: it is harmless and may be used elsewhere.
