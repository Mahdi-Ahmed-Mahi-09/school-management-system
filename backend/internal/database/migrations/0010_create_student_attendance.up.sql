-- Feature 10: Student Attendance
-- One row per student per day, linked to the student's enrollment (Feature 08)
-- for that academic year. The class and section the student was in when the
-- attendance was taken are stored on the row, so a later section change never
-- rewrites history. Records are corrected in place (never deleted); every
-- change after the first save is written to student_attendance_changes.

-- Lets attendance reference (enrollment, student, academic year) together, so
-- the database guarantees the row's student and year match its enrollment.
ALTER TABLE student_enrollments ADD CONSTRAINT student_enrollments_id_student_year_key UNIQUE (id, student_id, academic_year_id);

CREATE TABLE IF NOT EXISTS student_attendance (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    enrollment_id     UUID         NOT NULL,
    student_id        UUID         NOT NULL REFERENCES students (id) ON DELETE RESTRICT,
    academic_year_id  UUID         NOT NULL REFERENCES academic_years (id) ON DELETE RESTRICT,
    -- Where the student was (and their roll number) when attendance was taken.
    class_id          UUID         NOT NULL REFERENCES classes (id) ON DELETE RESTRICT,
    section_id        UUID         NOT NULL,
    roll_number       INTEGER      NOT NULL,
    attendance_date   DATE         NOT NULL,
    status            VARCHAR(8)   NOT NULL,
    remarks           VARCHAR(300),
    -- Optimistic concurrency: bumped on every change, checked by writers.
    version           INTEGER      NOT NULL DEFAULT 1,
    marked_by         UUID REFERENCES users (id) ON DELETE SET NULL,
    updated_by        UUID REFERENCES users (id) ON DELETE SET NULL,
    created_at        TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ  NOT NULL DEFAULT now(),

    CONSTRAINT student_attendance_enrollment_fkey FOREIGN KEY (enrollment_id, student_id, academic_year_id)
        REFERENCES student_enrollments (id, student_id, academic_year_id) ON DELETE RESTRICT,
    CONSTRAINT student_attendance_section_fkey FOREIGN KEY (section_id, class_id, academic_year_id)
        REFERENCES sections (id, class_id, academic_year_id) ON DELETE RESTRICT,
    CONSTRAINT student_attendance_status_check CHECK (status IN ('present', 'absent', 'late', 'leave')),
    CONSTRAINT student_attendance_roll_range CHECK (roll_number BETWEEN 1 AND 9999),
    CONSTRAINT student_attendance_version_positive CHECK (version >= 1),
    -- One record per student per day (and so per enrollment per day).
    CONSTRAINT student_attendance_student_date_key UNIQUE (student_id, attendance_date),
    CONSTRAINT student_attendance_enrollment_date_key UNIQUE (enrollment_id, attendance_date)
);

CREATE INDEX IF NOT EXISTS student_attendance_section_date_idx ON student_attendance (section_id, attendance_date);
CREATE INDEX IF NOT EXISTS student_attendance_year_date_idx ON student_attendance (academic_year_id, attendance_date, status);
CREATE INDEX IF NOT EXISTS student_attendance_class_date_idx ON student_attendance (class_id, attendance_date);
CREATE INDEX IF NOT EXISTS student_attendance_date_idx ON student_attendance (attendance_date);

-- The attendance date must fall within the row's academic year. A CHECK
-- constraint can't see another table, so a trigger enforces it on every write.
CREATE OR REPLACE FUNCTION student_attendance_check_date() RETURNS trigger AS $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM academic_years y
        WHERE y.id = NEW.academic_year_id AND NEW.attendance_date BETWEEN y.start_date AND y.end_date
    ) THEN
        RAISE EXCEPTION 'attendance date % is outside its academic year', NEW.attendance_date
            USING ERRCODE = 'check_violation', CONSTRAINT = 'student_attendance_date_in_year';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER student_attendance_date_in_year
    BEFORE INSERT OR UPDATE OF attendance_date, academic_year_id ON student_attendance
    FOR EACH ROW EXECUTE FUNCTION student_attendance_check_date();

-- Audit trail: one row per change to a saved attendance record.
CREATE TABLE IF NOT EXISTS student_attendance_changes (
    id                BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    attendance_id     UUID         NOT NULL REFERENCES student_attendance (id) ON DELETE RESTRICT,
    previous_status   VARCHAR(8)   NOT NULL,
    new_status        VARCHAR(8)   NOT NULL,
    previous_remarks  VARCHAR(300),
    new_remarks       VARCHAR(300),
    reason            VARCHAR(300),
    changed_by        UUID REFERENCES users (id) ON DELETE SET NULL,
    changed_at        TIMESTAMPTZ  NOT NULL DEFAULT now(),

    CONSTRAINT student_attendance_changes_previous_check CHECK (previous_status IN ('present', 'absent', 'late', 'leave')),
    CONSTRAINT student_attendance_changes_new_check CHECK (new_status IN ('present', 'absent', 'late', 'leave'))
);

CREATE INDEX IF NOT EXISTS student_attendance_changes_attendance_idx ON student_attendance_changes (attendance_id, changed_at DESC);
