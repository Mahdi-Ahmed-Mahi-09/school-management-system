-- Feature 08: Student Enrollment & Promotion
-- An enrollment places an existing student (Feature 03) into a section
-- (Feature 06) for one academic year (Feature 05). The class is the section's
-- class; it is not stored again. Enrollments are never removed by status
-- changes, so every year's placement stays in the student's history.

-- Lets enrollments reference (section, academic year) together, so the
-- database guarantees an enrollment's section belongs to its academic year.
ALTER TABLE sections ADD CONSTRAINT sections_id_year_key UNIQUE (id, academic_year_id);

CREATE TABLE IF NOT EXISTS student_enrollments (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    student_id         UUID         NOT NULL REFERENCES students (id) ON DELETE RESTRICT,
    academic_year_id   UUID         NOT NULL REFERENCES academic_years (id) ON DELETE RESTRICT,
    section_id         UUID         NOT NULL,
    roll_number        INTEGER      NOT NULL,
    admission_type     VARCHAR(16)  NOT NULL DEFAULT 'new',
    enrollment_date    DATE         NOT NULL,
    status             VARCHAR(16)  NOT NULL DEFAULT 'enrolled',
    -- Why the status last changed (withdrawal reason, promotion batch …).
    status_note        VARCHAR(500),
    status_changed_at  TIMESTAMPTZ,
    remarks            VARCHAR(500),
    created_by         UUID REFERENCES users (id) ON DELETE SET NULL,
    updated_by         UUID REFERENCES users (id) ON DELETE SET NULL,
    created_at         TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ  NOT NULL DEFAULT now(),

    CONSTRAINT student_enrollments_section_year_fkey FOREIGN KEY (section_id, academic_year_id)
        REFERENCES sections (id, academic_year_id) ON DELETE RESTRICT,
    -- One enrollment per student per academic year (withdrawn records are
    -- re-activated rather than duplicated).
    CONSTRAINT student_enrollments_student_year_key UNIQUE (student_id, academic_year_id),
    CONSTRAINT student_enrollments_roll_range CHECK (roll_number BETWEEN 1 AND 9999),
    CONSTRAINT student_enrollments_admission_type_check CHECK (admission_type IN ('new', 'continuing', 'readmission')),
    CONSTRAINT student_enrollments_status_check CHECK (status IN ('enrolled', 'promoted', 'repeated', 'completed', 'withdrawn', 'transferred'))
);

-- Roll numbers are unique within a section (sections belong to one year)
-- among students who still hold a place; withdrawn/transferred students free theirs.
CREATE UNIQUE INDEX IF NOT EXISTS student_enrollments_section_roll_key
    ON student_enrollments (section_id, roll_number) WHERE status NOT IN ('withdrawn', 'transferred');
CREATE INDEX IF NOT EXISTS student_enrollments_year_section_idx ON student_enrollments (academic_year_id, section_id, roll_number);
CREATE INDEX IF NOT EXISTS student_enrollments_student_idx ON student_enrollments (student_id);
CREATE INDEX IF NOT EXISTS student_enrollments_status_idx ON student_enrollments (academic_year_id, status);

-- A promotion batch records one confirmed promotion run.
CREATE TABLE IF NOT EXISTS promotion_batches (
    id                       UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    -- Client-generated key: resubmitting the same confirmation never runs twice.
    request_id               UUID         NOT NULL,
    source_academic_year_id  UUID         NOT NULL REFERENCES academic_years (id) ON DELETE RESTRICT,
    target_academic_year_id  UUID         NOT NULL REFERENCES academic_years (id) ON DELETE RESTRICT,
    roll_strategy            VARCHAR(16)  NOT NULL,
    total_students           INTEGER      NOT NULL DEFAULT 0,
    promoted_count           INTEGER      NOT NULL DEFAULT 0,
    repeated_count           INTEGER      NOT NULL DEFAULT 0,
    completed_count          INTEGER      NOT NULL DEFAULT 0,
    excluded_count           INTEGER      NOT NULL DEFAULT 0,
    skipped_count            INTEGER      NOT NULL DEFAULT 0,
    remarks                  VARCHAR(500),
    status                   VARCHAR(16)  NOT NULL DEFAULT 'completed',
    processed_by             UUID REFERENCES users (id) ON DELETE SET NULL,
    processed_at             TIMESTAMPTZ  NOT NULL DEFAULT now(),

    CONSTRAINT promotion_batches_request_key UNIQUE (request_id),
    CONSTRAINT promotion_batches_years_differ CHECK (source_academic_year_id <> target_academic_year_id),
    CONSTRAINT promotion_batches_roll_strategy_check CHECK (roll_strategy IN ('keep', 'sequential')),
    CONSTRAINT promotion_batches_status_check CHECK (status IN ('completed')),
    CONSTRAINT promotion_batches_counts_check CHECK (
        total_students = promoted_count + repeated_count + completed_count + excluded_count + skipped_count)
);

CREATE INDEX IF NOT EXISTS promotion_batches_processed_idx ON promotion_batches (processed_at DESC);

-- One row per student considered in a batch, with where they came from and went.
CREATE TABLE IF NOT EXISTS promotion_batch_items (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    batch_id                UUID         NOT NULL REFERENCES promotion_batches (id) ON DELETE RESTRICT,
    student_id              UUID         NOT NULL REFERENCES students (id) ON DELETE RESTRICT,
    previous_enrollment_id  UUID         NOT NULL REFERENCES student_enrollments (id) ON DELETE RESTRICT,
    new_enrollment_id       UUID REFERENCES student_enrollments (id) ON DELETE RESTRICT,
    decision                VARCHAR(16)  NOT NULL,
    status                  VARCHAR(16)  NOT NULL,
    message                 VARCHAR(300),
    previous_class_id       UUID         NOT NULL REFERENCES classes (id) ON DELETE RESTRICT,
    previous_section_id     UUID         NOT NULL REFERENCES sections (id) ON DELETE RESTRICT,
    previous_roll_number    INTEGER      NOT NULL,
    new_class_id            UUID REFERENCES classes (id) ON DELETE RESTRICT,
    new_section_id          UUID REFERENCES sections (id) ON DELETE RESTRICT,
    new_roll_number         INTEGER,

    CONSTRAINT promotion_batch_items_decision_check CHECK (decision IN ('promote', 'repeat', 'complete', 'exclude')),
    CONSTRAINT promotion_batch_items_status_check CHECK (status IN ('processed', 'skipped', 'excluded')),
    CONSTRAINT promotion_batch_items_batch_enrollment_key UNIQUE (batch_id, previous_enrollment_id),
    -- Promoted/repeated students always get a destination enrollment.
    CONSTRAINT promotion_batch_items_destination_check CHECK (
        (status = 'processed' AND decision IN ('promote', 'repeat')) = (new_enrollment_id IS NOT NULL))
);

-- An enrollment is processed by at most one promotion, ever.
CREATE UNIQUE INDEX IF NOT EXISTS promotion_batch_items_processed_key
    ON promotion_batch_items (previous_enrollment_id) WHERE status = 'processed';
CREATE INDEX IF NOT EXISTS promotion_batch_items_batch_idx ON promotion_batch_items (batch_id);
CREATE INDEX IF NOT EXISTS promotion_batch_items_student_idx ON promotion_batch_items (student_id);
