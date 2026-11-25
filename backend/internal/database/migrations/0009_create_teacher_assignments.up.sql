-- Feature 09: Teacher Assignment
-- Assigns existing teachers (Feature 04) to sections (Feature 06) of an
-- academic year (Feature 05), either as the subject teacher of a subject the
-- class takes that year (Feature 07), or as the section's class teacher.
-- Changing a teacher deactivates the old row and links the new one to it, so
-- the history of who taught what is kept.

-- Lets assignments reference (section, class, academic year) together, so
-- the database guarantees a section belongs to the assignment's class and year.
ALTER TABLE sections ADD CONSTRAINT sections_id_class_year_key UNIQUE (id, class_id, academic_year_id);

CREATE TABLE IF NOT EXISTS teacher_assignments (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    teacher_id              UUID         NOT NULL REFERENCES teachers (id) ON DELETE RESTRICT,
    academic_year_id        UUID         NOT NULL REFERENCES academic_years (id) ON DELETE RESTRICT,
    class_id                UUID         NOT NULL REFERENCES classes (id) ON DELETE RESTRICT,
    section_id              UUID         NOT NULL,
    -- Set for subject teachers only.
    subject_id              UUID REFERENCES subjects (id) ON DELETE RESTRICT,
    assignment_type         VARCHAR(20)  NOT NULL,
    status                  VARCHAR(16)  NOT NULL DEFAULT 'active',
    assigned_at             TIMESTAMPTZ  NOT NULL DEFAULT now(),
    -- When and why the assignment stopped (deactivated or replaced).
    ended_at                TIMESTAMPTZ,
    end_note                VARCHAR(300),
    -- The assignment this one replaced (teacher change), for history.
    previous_assignment_id  UUID REFERENCES teacher_assignments (id) ON DELETE RESTRICT,
    remarks                 VARCHAR(500),
    created_by              UUID REFERENCES users (id) ON DELETE SET NULL,
    updated_by              UUID REFERENCES users (id) ON DELETE SET NULL,
    created_at              TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ  NOT NULL DEFAULT now(),

    CONSTRAINT teacher_assignments_section_fkey FOREIGN KEY (section_id, class_id, academic_year_id)
        REFERENCES sections (id, class_id, academic_year_id) ON DELETE RESTRICT,
    -- A subject teacher's subject must be one the class takes that year.
    CONSTRAINT teacher_assignments_class_subject_fkey FOREIGN KEY (class_id, academic_year_id, subject_id)
        REFERENCES class_subjects (class_id, academic_year_id, subject_id) ON DELETE RESTRICT,
    CONSTRAINT teacher_assignments_type_check CHECK (assignment_type IN ('subject_teacher', 'class_teacher')),
    CONSTRAINT teacher_assignments_subject_matches_type CHECK ((assignment_type = 'subject_teacher') = (subject_id IS NOT NULL)),
    CONSTRAINT teacher_assignments_status_check CHECK (status IN ('active', 'inactive')),
    CONSTRAINT teacher_assignments_ended_matches_status CHECK ((status = 'inactive') = (ended_at IS NOT NULL)),
    CONSTRAINT teacher_assignments_not_self CHECK (previous_assignment_id IS NULL OR previous_assignment_id <> id)
);

-- One active teacher per subject in a section (sections belong to one year).
CREATE UNIQUE INDEX IF NOT EXISTS teacher_assignments_active_subject_key
    ON teacher_assignments (section_id, subject_id) WHERE status = 'active' AND assignment_type = 'subject_teacher';
-- One active class teacher per section.
CREATE UNIQUE INDEX IF NOT EXISTS teacher_assignments_active_class_teacher_key
    ON teacher_assignments (section_id) WHERE status = 'active' AND assignment_type = 'class_teacher';
-- An assignment is replaced at most once.
CREATE UNIQUE INDEX IF NOT EXISTS teacher_assignments_previous_key
    ON teacher_assignments (previous_assignment_id) WHERE previous_assignment_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS teacher_assignments_year_class_idx ON teacher_assignments (academic_year_id, class_id, section_id);
CREATE INDEX IF NOT EXISTS teacher_assignments_teacher_idx ON teacher_assignments (teacher_id, academic_year_id);
CREATE INDEX IF NOT EXISTS teacher_assignments_subject_idx ON teacher_assignments (subject_id);
CREATE INDEX IF NOT EXISTS teacher_assignments_status_idx ON teacher_assignments (academic_year_id, status, assignment_type);
