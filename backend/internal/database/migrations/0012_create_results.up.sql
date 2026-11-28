-- Feature 12: Marks, Grades & Results
-- Marks are entered per paper of an examination (examination_schedules,
-- Feature 11) for students identified by their enrollment (Feature 08), one
-- mark sheet per paper and section. Results are snapshots: subject and
-- overall results copy the marks, grades and names they were computed from,
-- and every publication stores the published results, so published results
-- stay accurate even if configurations change later.

-- Marking scheme of one paper: full and pass marks, optional components.
CREATE TABLE IF NOT EXISTS exam_mark_schemes (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    schedule_id   UUID          NOT NULL REFERENCES examination_schedules (id) ON DELETE RESTRICT,
    full_marks    NUMERIC(8,2)  NOT NULL,
    pass_marks    NUMERIC(8,2)  NOT NULL,
    instructions  VARCHAR(500),
    version       INTEGER       NOT NULL DEFAULT 1,
    created_by    UUID REFERENCES users (id) ON DELETE SET NULL,
    updated_by    UUID REFERENCES users (id) ON DELETE SET NULL,
    created_at    TIMESTAMPTZ   NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ   NOT NULL DEFAULT now(),

    CONSTRAINT exam_mark_schemes_schedule_key UNIQUE (schedule_id),
    CONSTRAINT exam_mark_schemes_full_positive CHECK (full_marks > 0),
    CONSTRAINT exam_mark_schemes_pass_range CHECK (pass_marks >= 0 AND pass_marks <= full_marks)
);

CREATE TABLE IF NOT EXISTS exam_mark_components (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    scheme_id      UUID          NOT NULL REFERENCES exam_mark_schemes (id) ON DELETE RESTRICT,
    name           VARCHAR(40)   NOT NULL,
    max_marks      NUMERIC(8,2)  NOT NULL,
    display_order  INTEGER       NOT NULL DEFAULT 0,
    created_at     TIMESTAMPTZ   NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ   NOT NULL DEFAULT now(),

    CONSTRAINT exam_mark_components_name_present CHECK (btrim(name) <> ''),
    CONSTRAINT exam_mark_components_max_positive CHECK (max_marks > 0)
);
CREATE UNIQUE INDEX IF NOT EXISTS exam_mark_components_name_key ON exam_mark_components (scheme_id, lower(name));

-- When a scheme has components, their maximums must add up to its full
-- marks. Checked at commit, so a scheme and its components can change together.
CREATE OR REPLACE FUNCTION exam_mark_scheme_check_components() RETURNS trigger AS $$
DECLARE
    sid UUID;
    total NUMERIC;
    n INTEGER;
    full_m NUMERIC;
BEGIN
    IF TG_TABLE_NAME = 'exam_mark_components' THEN
        sid := COALESCE(NEW.scheme_id, OLD.scheme_id);
    ELSE
        sid := NEW.id;
    END IF;
    SELECT full_marks INTO full_m FROM exam_mark_schemes WHERE id = sid;
    IF full_m IS NULL THEN
        RETURN NULL;
    END IF;
    SELECT count(*), COALESCE(sum(max_marks), 0) INTO n, total FROM exam_mark_components WHERE scheme_id = sid;
    IF n > 0 AND total <> full_m THEN
        RAISE EXCEPTION 'component maximums (%) must add up to the full marks (%)', total, full_m
            USING ERRCODE = 'check_violation', CONSTRAINT = 'exam_mark_components_sum';
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

CREATE CONSTRAINT TRIGGER exam_mark_components_sum
    AFTER INSERT OR UPDATE OR DELETE ON exam_mark_components
    DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION exam_mark_scheme_check_components();
CREATE CONSTRAINT TRIGGER exam_mark_schemes_components_sum
    AFTER INSERT OR UPDATE OF full_marks ON exam_mark_schemes
    DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION exam_mark_scheme_check_components();

-- One mark sheet per paper and section: its marks move through
-- draft → submitted → verified → finalized.
CREATE TABLE IF NOT EXISTS exam_mark_sheets (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    examination_id  UUID          NOT NULL REFERENCES examinations (id) ON DELETE RESTRICT,
    schedule_id     UUID          NOT NULL REFERENCES examination_schedules (id) ON DELETE RESTRICT,
    section_id      UUID          NOT NULL REFERENCES sections (id) ON DELETE RESTRICT,
    status          VARCHAR(16)   NOT NULL DEFAULT 'draft',
    submitted_at    TIMESTAMPTZ,
    submitted_by    UUID REFERENCES users (id) ON DELETE SET NULL,
    verified_at     TIMESTAMPTZ,
    verified_by     UUID REFERENCES users (id) ON DELETE SET NULL,
    finalized_at    TIMESTAMPTZ,
    finalized_by    UUID REFERENCES users (id) ON DELETE SET NULL,
    -- Why the sheet was last returned or reopened.
    reopen_reason   VARCHAR(500),
    reopened_at     TIMESTAMPTZ,
    reopened_by     UUID REFERENCES users (id) ON DELETE SET NULL,
    created_at      TIMESTAMPTZ   NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ   NOT NULL DEFAULT now(),

    CONSTRAINT exam_mark_sheets_schedule_section_key UNIQUE (schedule_id, section_id),
    CONSTRAINT exam_mark_sheets_status_check CHECK (status IN ('draft', 'submitted', 'verified', 'finalized'))
);
CREATE INDEX IF NOT EXISTS exam_mark_sheets_exam_idx ON exam_mark_sheets (examination_id, status);

-- One marks record per student (enrollment) per paper.
CREATE TABLE IF NOT EXISTS student_exam_marks (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    sheet_id        UUID          NOT NULL REFERENCES exam_mark_sheets (id) ON DELETE RESTRICT,
    schedule_id     UUID          NOT NULL REFERENCES examination_schedules (id) ON DELETE RESTRICT,
    enrollment_id   UUID          NOT NULL REFERENCES student_enrollments (id) ON DELETE RESTRICT,
    student_id      UUID          NOT NULL REFERENCES students (id) ON DELETE RESTRICT,
    attendance      VARCHAR(8)    NOT NULL,
    total_marks     NUMERIC(8,2)  NOT NULL DEFAULT 0,
    version         INTEGER       NOT NULL DEFAULT 1,
    entered_by      UUID REFERENCES users (id) ON DELETE SET NULL,
    updated_by      UUID REFERENCES users (id) ON DELETE SET NULL,
    created_at      TIMESTAMPTZ   NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ   NOT NULL DEFAULT now(),

    CONSTRAINT student_exam_marks_schedule_enrollment_key UNIQUE (schedule_id, enrollment_id),
    CONSTRAINT student_exam_marks_attendance_check CHECK (attendance IN ('present', 'absent')),
    CONSTRAINT student_exam_marks_total_non_negative CHECK (total_marks >= 0),
    -- An absent student keeps 0 marks; the absence is recorded separately.
    CONSTRAINT student_exam_marks_absent_zero CHECK (attendance = 'present' OR total_marks = 0)
);
CREATE INDEX IF NOT EXISTS student_exam_marks_sheet_idx ON student_exam_marks (sheet_id);
CREATE INDEX IF NOT EXISTS student_exam_marks_enrollment_idx ON student_exam_marks (enrollment_id);

-- Marks must fit the paper's full marks, and the student must be enrolled
-- in the paper's class and academic year.
CREATE OR REPLACE FUNCTION student_exam_marks_check() RETURNS trigger AS $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM exam_mark_schemes m WHERE m.schedule_id = NEW.schedule_id AND NEW.total_marks <= m.full_marks
    ) THEN
        RAISE EXCEPTION 'marks % exceed the full marks (or the paper has no marking scheme)', NEW.total_marks
            USING ERRCODE = 'check_violation', CONSTRAINT = 'student_exam_marks_within_full';
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM student_enrollments e
        JOIN sections s ON s.id = e.section_id
        JOIN examination_schedules es ON es.id = NEW.schedule_id
        WHERE e.id = NEW.enrollment_id AND e.student_id = NEW.student_id
          AND e.academic_year_id = es.academic_year_id AND s.class_id = es.class_id
    ) THEN
        RAISE EXCEPTION 'the student is not enrolled in the class of this paper'
            USING ERRCODE = 'foreign_key_violation', CONSTRAINT = 'student_exam_marks_enrollment_class';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER student_exam_marks_check
    BEFORE INSERT OR UPDATE ON student_exam_marks
    FOR EACH ROW EXECUTE FUNCTION student_exam_marks_check();

-- Component marks of a marks record (one per component).
CREATE TABLE IF NOT EXISTS student_exam_component_marks (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    mark_id       UUID          NOT NULL REFERENCES student_exam_marks (id) ON DELETE CASCADE,
    component_id  UUID          NOT NULL REFERENCES exam_mark_components (id) ON DELETE RESTRICT,
    obtained      NUMERIC(8,2)  NOT NULL,
    created_at    TIMESTAMPTZ   NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ   NOT NULL DEFAULT now(),

    CONSTRAINT student_exam_component_marks_key UNIQUE (mark_id, component_id),
    CONSTRAINT student_exam_component_marks_non_negative CHECK (obtained >= 0)
);

CREATE OR REPLACE FUNCTION student_exam_component_marks_check() RETURNS trigger AS $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM exam_mark_components c WHERE c.id = NEW.component_id AND NEW.obtained <= c.max_marks) THEN
        RAISE EXCEPTION 'component marks % exceed the component maximum', NEW.obtained
            USING ERRCODE = 'check_violation', CONSTRAINT = 'student_exam_component_marks_within_max';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER student_exam_component_marks_check
    BEFORE INSERT OR UPDATE ON student_exam_component_marks
    FOR EACH ROW EXECUTE FUNCTION student_exam_component_marks_check();

-- Audit trail of marks changed after they were first saved.
CREATE TABLE IF NOT EXISTS student_exam_mark_changes (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    mark_id     UUID         NOT NULL REFERENCES student_exam_marks (id) ON DELETE CASCADE,
    before      JSONB        NOT NULL,
    after       JSONB        NOT NULL,
    reason      VARCHAR(500),
    changed_by  UUID REFERENCES users (id) ON DELETE SET NULL,
    changed_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS student_exam_mark_changes_mark_idx ON student_exam_mark_changes (mark_id, changed_at DESC);

-- Result processing state of an examination.
CREATE TABLE IF NOT EXISTS exam_result_runs (
    examination_id         UUID PRIMARY KEY REFERENCES examinations (id) ON DELETE RESTRICT,
    stage                  VARCHAR(20)  NOT NULL,
    subjects_processed_at  TIMESTAMPTZ,
    subjects_processed_by  UUID REFERENCES users (id) ON DELETE SET NULL,
    processed_at           TIMESTAMPTZ,
    processed_by           UUID REFERENCES users (id) ON DELETE SET NULL,
    verified_at            TIMESTAMPTZ,
    verified_by            UUID REFERENCES users (id) ON DELETE SET NULL,
    published_at           TIMESTAMPTZ,
    published_by           UUID REFERENCES users (id) ON DELETE SET NULL,
    -- How many times the results were published (corrections republish).
    publications           INTEGER      NOT NULL DEFAULT 0,
    correction_reason      VARCHAR(500),
    updated_at             TIMESTAMPTZ  NOT NULL DEFAULT now(),

    CONSTRAINT exam_result_runs_stage_check CHECK (stage IN ('pending', 'subjects_processed', 'processed', 'verified', 'published', 'correction'))
);

-- Subject results: a snapshot of each student's paper.
CREATE TABLE IF NOT EXISTS exam_subject_results (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    examination_id  UUID          NOT NULL REFERENCES examinations (id) ON DELETE RESTRICT,
    schedule_id     UUID          NOT NULL REFERENCES examination_schedules (id) ON DELETE RESTRICT,
    enrollment_id   UUID          NOT NULL REFERENCES student_enrollments (id) ON DELETE RESTRICT,
    student_id      UUID          NOT NULL REFERENCES students (id) ON DELETE RESTRICT,
    class_id        UUID          NOT NULL REFERENCES classes (id) ON DELETE RESTRICT,
    section_id      UUID          NOT NULL REFERENCES sections (id) ON DELETE RESTRICT,
    subject_id      UUID          NOT NULL REFERENCES subjects (id) ON DELETE RESTRICT,
    subject_name    VARCHAR(100)  NOT NULL,
    subject_code    VARCHAR(20)   NOT NULL,
    full_marks      NUMERIC(8,2)  NOT NULL,
    pass_marks      NUMERIC(8,2)  NOT NULL,
    obtained_marks  NUMERIC(8,2)  NOT NULL,
    percentage      NUMERIC(6,2)  NOT NULL,
    letter_grade    VARCHAR(2)    NOT NULL,
    grade_point     NUMERIC(3,2)  NOT NULL,
    absent          BOOLEAN       NOT NULL,
    passed          BOOLEAN       NOT NULL,
    processed_at    TIMESTAMPTZ   NOT NULL DEFAULT now(),
    processed_by    UUID REFERENCES users (id) ON DELETE SET NULL,

    CONSTRAINT exam_subject_results_key UNIQUE (schedule_id, enrollment_id),
    CONSTRAINT exam_subject_results_marks CHECK (obtained_marks >= 0 AND obtained_marks <= full_marks AND full_marks > 0),
    CONSTRAINT exam_subject_results_grade CHECK (letter_grade IN ('A+', 'A', 'A-', 'B', 'C', 'D', 'F')),
    -- A failed subject is always F with 0.00.
    CONSTRAINT exam_subject_results_fail_is_f CHECK (passed OR (letter_grade = 'F' AND grade_point = 0))
);
CREATE INDEX IF NOT EXISTS exam_subject_results_exam_idx ON exam_subject_results (examination_id, enrollment_id);

-- Overall results: one per student (enrollment) per examination.
CREATE TABLE IF NOT EXISTS exam_results (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    examination_id    UUID          NOT NULL REFERENCES examinations (id) ON DELETE RESTRICT,
    enrollment_id     UUID          NOT NULL REFERENCES student_enrollments (id) ON DELETE RESTRICT,
    student_id        UUID          NOT NULL REFERENCES students (id) ON DELETE RESTRICT,
    class_id          UUID          NOT NULL REFERENCES classes (id) ON DELETE RESTRICT,
    section_id        UUID          NOT NULL REFERENCES sections (id) ON DELETE RESTRICT,
    roll_number       INTEGER       NOT NULL,
    subjects          INTEGER       NOT NULL,
    failed_subjects   INTEGER       NOT NULL,
    total_obtained    NUMERIC(10,2) NOT NULL,
    total_full        NUMERIC(10,2) NOT NULL,
    percentage        NUMERIC(6,2)  NOT NULL,
    gpa               NUMERIC(3,2)  NOT NULL,
    overall_grade     VARCHAR(2)    NOT NULL,
    passed            BOOLEAN       NOT NULL,
    class_position    INTEGER,
    section_position  INTEGER,
    processed_at      TIMESTAMPTZ   NOT NULL DEFAULT now(),
    processed_by      UUID REFERENCES users (id) ON DELETE SET NULL,
    published_at      TIMESTAMPTZ,

    CONSTRAINT exam_results_key UNIQUE (examination_id, enrollment_id),
    CONSTRAINT exam_results_gpa_range CHECK (gpa >= 0 AND gpa <= 5),
    -- The fail rule: any failed subject means GPA 0.00, grade F and no position.
    CONSTRAINT exam_results_fail_rule CHECK (passed = (failed_subjects = 0) AND (passed OR (gpa = 0 AND overall_grade = 'F' AND class_position IS NULL)))
);
CREATE INDEX IF NOT EXISTS exam_results_class_idx ON exam_results (examination_id, class_id, section_id);
CREATE INDEX IF NOT EXISTS exam_results_student_idx ON exam_results (student_id);

-- Every publication keeps a copy of the published results.
CREATE TABLE IF NOT EXISTS exam_result_publications (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    examination_id     UUID         NOT NULL REFERENCES examinations (id) ON DELETE RESTRICT,
    version            INTEGER      NOT NULL,
    published_at       TIMESTAMPTZ  NOT NULL DEFAULT now(),
    published_by       UUID REFERENCES users (id) ON DELETE SET NULL,
    correction_reason  VARCHAR(500),
    students           INTEGER      NOT NULL,
    passed             INTEGER      NOT NULL,
    snapshot           JSONB        NOT NULL,

    CONSTRAINT exam_result_publications_version_key UNIQUE (examination_id, version)
);
