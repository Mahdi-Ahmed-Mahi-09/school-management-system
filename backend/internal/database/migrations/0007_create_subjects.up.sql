-- Feature 07: Subject Management
-- A subject (e.g. "Mathematics") is a reusable catalogue entry. It is offered
-- to a class in a particular academic year through class_subjects, which also
-- records whether the subject is compulsory or optional for that class.
-- Foreign keys use ON DELETE RESTRICT: subjects, classes and academic years
-- with assignments cannot be deleted. Future modules (teacher assignment,
-- routines, examinations, marks) reference class_subjects.id / subjects.id.

CREATE TABLE IF NOT EXISTS subjects (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name          VARCHAR(100) NOT NULL,
    code          VARCHAR(20)  NOT NULL,
    description   VARCHAR(500),
    subject_type  VARCHAR(16)  NOT NULL DEFAULT 'compulsory',
    full_marks    INTEGER,
    pass_marks    INTEGER,
    status        VARCHAR(16)  NOT NULL DEFAULT 'active',
    created_by    UUID REFERENCES users (id) ON DELETE SET NULL,
    updated_by    UUID REFERENCES users (id) ON DELETE SET NULL,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT now(),

    CONSTRAINT subjects_name_present CHECK (btrim(name) <> ''),
    CONSTRAINT subjects_code_format CHECK (code ~ '^[A-Z0-9][A-Z0-9-]{0,19}$'),
    CONSTRAINT subjects_type_check CHECK (subject_type IN ('compulsory', 'optional')),
    CONSTRAINT subjects_full_marks_range CHECK (full_marks IS NULL OR full_marks BETWEEN 1 AND 1000),
    -- Pass marks need full marks and must lie between 0 and the full marks.
    CONSTRAINT subjects_pass_marks_range CHECK (pass_marks IS NULL OR (full_marks IS NOT NULL AND pass_marks BETWEEN 0 AND full_marks)),
    CONSTRAINT subjects_status_check CHECK (status IN ('active', 'inactive'))
);

CREATE UNIQUE INDEX IF NOT EXISTS subjects_code_key ON subjects (code);
CREATE UNIQUE INDEX IF NOT EXISTS subjects_name_key ON subjects (lower(name));
CREATE INDEX IF NOT EXISTS subjects_status_idx ON subjects (status);
CREATE INDEX IF NOT EXISTS subjects_type_idx ON subjects (subject_type);

CREATE TABLE IF NOT EXISTS class_subjects (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    class_id          UUID        NOT NULL REFERENCES classes (id) ON DELETE RESTRICT,
    subject_id        UUID        NOT NULL REFERENCES subjects (id) ON DELETE RESTRICT,
    academic_year_id  UUID        NOT NULL REFERENCES academic_years (id) ON DELETE RESTRICT,
    is_compulsory     BOOLEAN     NOT NULL,
    created_by        UUID REFERENCES users (id) ON DELETE SET NULL,
    updated_by        UUID REFERENCES users (id) ON DELETE SET NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- A subject is assigned to a class at most once per academic year.
    CONSTRAINT class_subjects_class_year_subject_key UNIQUE (class_id, academic_year_id, subject_id)
);

CREATE INDEX IF NOT EXISTS class_subjects_year_class_idx ON class_subjects (academic_year_id, class_id);
CREATE INDEX IF NOT EXISTS class_subjects_subject_idx ON class_subjects (subject_id, academic_year_id);
