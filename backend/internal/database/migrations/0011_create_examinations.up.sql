-- Feature 11: Examination Management
-- An examination (e.g. "Half-Yearly Examination 2027") belongs to one academic
-- year (Feature 05), covers one or more classes (Feature 06) and has a
-- subject-wise schedule per class, using only subjects the class takes that
-- year (Feature 07). Schedules are class-level: every section of the class
-- sits the same timetable. Marks and results (Feature 12) will reference
-- examination_schedules.id; nothing here is deleted once published.

-- Needed for the exclusion constraint that stops two exams of the same class
-- from overlapping in time.
CREATE EXTENSION IF NOT EXISTS btree_gist;

-- Examination types: a predefined list the school can extend with its own.
CREATE TABLE IF NOT EXISTS exam_types (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name         VARCHAR(60)  NOT NULL,
    description  VARCHAR(300),
    -- Predefined types can be deactivated but not renamed or deleted.
    is_system    BOOLEAN      NOT NULL DEFAULT false,
    status       VARCHAR(16)  NOT NULL DEFAULT 'active',
    sort_order   INTEGER      NOT NULL DEFAULT 100,
    created_by   UUID REFERENCES users (id) ON DELETE SET NULL,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ  NOT NULL DEFAULT now(),

    CONSTRAINT exam_types_name_present CHECK (btrim(name) <> ''),
    CONSTRAINT exam_types_status_check CHECK (status IN ('active', 'inactive'))
);
CREATE UNIQUE INDEX IF NOT EXISTS exam_types_name_key ON exam_types (lower(name));

INSERT INTO exam_types (name, is_system, sort_order) VALUES
    ('Class Test', true, 10),
    ('Monthly Test', true, 20),
    ('First Term Examination', true, 30),
    ('Half-Yearly Examination', true, 40),
    ('Second Term Examination', true, 50),
    ('Annual Examination', true, 60),
    ('Model Test', true, 70),
    ('Test Examination', true, 80),
    ('Pre-Test Examination', true, 90),
    ('Selection Examination', true, 100)
ON CONFLICT DO NOTHING;

CREATE TABLE IF NOT EXISTS examinations (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name              VARCHAR(120)  NOT NULL,
    exam_type_id      UUID          NOT NULL REFERENCES exam_types (id) ON DELETE RESTRICT,
    academic_year_id  UUID          NOT NULL REFERENCES academic_years (id) ON DELETE RESTRICT,
    start_date        DATE          NOT NULL,
    end_date          DATE          NOT NULL,
    description       VARCHAR(1000),
    instructions      VARCHAR(2000),
    status            VARCHAR(16)   NOT NULL DEFAULT 'draft',
    published_at      TIMESTAMPTZ,
    published_by      UUID REFERENCES users (id) ON DELETE SET NULL,
    completed_at      TIMESTAMPTZ,
    cancelled_at      TIMESTAMPTZ,
    cancel_reason     VARCHAR(500),
    created_by        UUID REFERENCES users (id) ON DELETE SET NULL,
    updated_by        UUID REFERENCES users (id) ON DELETE SET NULL,
    created_at        TIMESTAMPTZ   NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ   NOT NULL DEFAULT now(),

    CONSTRAINT examinations_name_present CHECK (btrim(name) <> ''),
    CONSTRAINT examinations_dates_order CHECK (start_date <= end_date),
    CONSTRAINT examinations_status_check CHECK (status IN ('draft', 'published', 'completed', 'cancelled')),
    CONSTRAINT examinations_published_stamp CHECK (status = 'draft' OR status = 'cancelled' OR published_at IS NOT NULL),
    CONSTRAINT examinations_cancel_reason CHECK ((status = 'cancelled') = (cancelled_at IS NOT NULL AND cancel_reason IS NOT NULL)),
    -- Lets classes reference (examination, academic year) together.
    CONSTRAINT examinations_id_year_key UNIQUE (id, academic_year_id)
);
-- Names are unique within an academic year.
CREATE UNIQUE INDEX IF NOT EXISTS examinations_year_name_key ON examinations (academic_year_id, lower(name));
CREATE INDEX IF NOT EXISTS examinations_year_status_idx ON examinations (academic_year_id, status, start_date);
CREATE INDEX IF NOT EXISTS examinations_type_idx ON examinations (exam_type_id);
CREATE INDEX IF NOT EXISTS examinations_dates_idx ON examinations (start_date, end_date);

-- The examination's dates must fall within its academic year.
CREATE OR REPLACE FUNCTION examinations_check_year_dates() RETURNS trigger AS $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM academic_years y
        WHERE y.id = NEW.academic_year_id AND NEW.start_date >= y.start_date AND NEW.end_date <= y.end_date
    ) THEN
        RAISE EXCEPTION 'examination dates % – % are outside the academic year', NEW.start_date, NEW.end_date
            USING ERRCODE = 'check_violation', CONSTRAINT = 'examinations_dates_in_year';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER examinations_dates_in_year
    BEFORE INSERT OR UPDATE OF start_date, end_date, academic_year_id ON examinations
    FOR EACH ROW EXECUTE FUNCTION examinations_check_year_dates();

-- The classes an examination covers (all their sections).
CREATE TABLE IF NOT EXISTS examination_classes (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    examination_id    UUID         NOT NULL,
    academic_year_id  UUID         NOT NULL,
    class_id          UUID         NOT NULL REFERENCES classes (id) ON DELETE RESTRICT,
    created_by        UUID REFERENCES users (id) ON DELETE SET NULL,
    created_at        TIMESTAMPTZ  NOT NULL DEFAULT now(),

    CONSTRAINT examination_classes_exam_fkey FOREIGN KEY (examination_id, academic_year_id)
        REFERENCES examinations (id, academic_year_id) ON DELETE RESTRICT,
    CONSTRAINT examination_classes_exam_class_key UNIQUE (examination_id, class_id),
    -- Lets schedules reference (examination class, class, academic year) together.
    CONSTRAINT examination_classes_id_class_year_key UNIQUE (id, class_id, academic_year_id)
);
CREATE INDEX IF NOT EXISTS examination_classes_class_idx ON examination_classes (class_id, academic_year_id);

-- One row per subject of a class in an examination.
CREATE TABLE IF NOT EXISTS examination_schedules (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    examination_id        UUID          NOT NULL REFERENCES examinations (id) ON DELETE RESTRICT,
    examination_class_id  UUID          NOT NULL,
    academic_year_id      UUID          NOT NULL,
    class_id              UUID          NOT NULL,
    subject_id            UUID          NOT NULL REFERENCES subjects (id) ON DELETE RESTRICT,
    exam_date             DATE          NOT NULL,
    start_time            TIME          NOT NULL,
    end_time              TIME          NOT NULL,
    room_number           VARCHAR(40),
    instructions          VARCHAR(500),
    -- Optimistic concurrency: bumped on every change, checked by writers.
    version               INTEGER       NOT NULL DEFAULT 1,
    created_by            UUID REFERENCES users (id) ON DELETE SET NULL,
    updated_by            UUID REFERENCES users (id) ON DELETE SET NULL,
    created_at            TIMESTAMPTZ   NOT NULL DEFAULT now(),
    updated_at            TIMESTAMPTZ   NOT NULL DEFAULT now(),

    CONSTRAINT examination_schedules_class_fkey FOREIGN KEY (examination_class_id, class_id, academic_year_id)
        REFERENCES examination_classes (id, class_id, academic_year_id) ON DELETE RESTRICT,
    -- The subject must be one the class takes that academic year.
    CONSTRAINT examination_schedules_class_subject_fkey FOREIGN KEY (class_id, academic_year_id, subject_id)
        REFERENCES class_subjects (class_id, academic_year_id, subject_id) ON DELETE RESTRICT,
    CONSTRAINT examination_schedules_subject_key UNIQUE (examination_class_id, subject_id),
    CONSTRAINT examination_schedules_times_order CHECK (start_time < end_time),
    CONSTRAINT examination_schedules_version_positive CHECK (version >= 1),
    -- A class can't sit two papers of the same examination at once.
    CONSTRAINT examination_schedules_no_overlap EXCLUDE USING gist (
        examination_class_id WITH =,
        tsrange(exam_date + start_time, exam_date + end_time) WITH &&
    ) DEFERRABLE INITIALLY IMMEDIATE
);
CREATE INDEX IF NOT EXISTS examination_schedules_exam_idx ON examination_schedules (examination_id, class_id, exam_date, start_time);
CREATE INDEX IF NOT EXISTS examination_schedules_year_date_idx ON examination_schedules (academic_year_id, exam_date);
CREATE INDEX IF NOT EXISTS examination_schedules_subject_idx ON examination_schedules (subject_id);
CREATE INDEX IF NOT EXISTS examination_schedules_room_idx ON examination_schedules (exam_date, lower(room_number)) WHERE room_number IS NOT NULL;

-- A paper's date must fall within its examination's date range.
CREATE OR REPLACE FUNCTION examination_schedules_check_date() RETURNS trigger AS $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM examinations e
        WHERE e.id = NEW.examination_id AND e.academic_year_id = NEW.academic_year_id
          AND NEW.exam_date BETWEEN e.start_date AND e.end_date
    ) THEN
        RAISE EXCEPTION 'exam date % is outside the examination dates', NEW.exam_date
            USING ERRCODE = 'check_violation', CONSTRAINT = 'examination_schedules_date_in_exam';
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM examination_classes ec WHERE ec.id = NEW.examination_class_id AND ec.examination_id = NEW.examination_id
    ) THEN
        RAISE EXCEPTION 'schedule class belongs to another examination'
            USING ERRCODE = 'foreign_key_violation', CONSTRAINT = 'examination_schedules_exam_match';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER examination_schedules_date_in_exam
    BEFORE INSERT OR UPDATE ON examination_schedules
    FOR EACH ROW EXECUTE FUNCTION examination_schedules_check_date();
