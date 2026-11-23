-- Feature 06: Class & Section Management
-- A class is a reusable academic level (e.g. "Class 6"). Sections belong to
-- exactly one class AND one academic year, so a class can have different
-- sections in different years. Foreign keys use ON DELETE RESTRICT: a class or
-- academic year that still has sections cannot be deleted. Future modules
-- (enrollment, teacher assignment, routines, attendance, exams) reference
-- sections.id / classes.id the same way.

CREATE TABLE IF NOT EXISTS classes (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name           VARCHAR(60)  NOT NULL,
    code           VARCHAR(20),
    display_order  INTEGER      NOT NULL,
    description    VARCHAR(500),
    status         VARCHAR(16)  NOT NULL DEFAULT 'active',
    created_by     UUID REFERENCES users (id) ON DELETE SET NULL,
    updated_by     UUID REFERENCES users (id) ON DELETE SET NULL,
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ  NOT NULL DEFAULT now(),

    CONSTRAINT classes_name_present CHECK (btrim(name) <> ''),
    CONSTRAINT classes_code_format CHECK (code IS NULL OR code ~ '^[A-Z0-9][A-Z0-9-]{0,19}$'),
    CONSTRAINT classes_display_order_range CHECK (display_order BETWEEN 1 AND 9999),
    CONSTRAINT classes_status_check CHECK (status IN ('active', 'inactive'))
);

CREATE UNIQUE INDEX IF NOT EXISTS classes_name_key ON classes (lower(name));
CREATE UNIQUE INDEX IF NOT EXISTS classes_code_key ON classes (code) WHERE code IS NOT NULL;
CREATE INDEX IF NOT EXISTS classes_display_order_idx ON classes (display_order, lower(name));
CREATE INDEX IF NOT EXISTS classes_status_idx ON classes (status);

CREATE TABLE IF NOT EXISTS sections (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    class_id          UUID         NOT NULL REFERENCES classes (id) ON DELETE RESTRICT,
    academic_year_id  UUID         NOT NULL REFERENCES academic_years (id) ON DELETE RESTRICT,
    name              VARCHAR(30)  NOT NULL,
    code              VARCHAR(20),
    room_number       VARCHAR(40),
    capacity          INTEGER,
    description       VARCHAR(500),
    status            VARCHAR(16)  NOT NULL DEFAULT 'active',
    created_by        UUID REFERENCES users (id) ON DELETE SET NULL,
    updated_by        UUID REFERENCES users (id) ON DELETE SET NULL,
    created_at        TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ  NOT NULL DEFAULT now(),

    CONSTRAINT sections_name_present CHECK (btrim(name) <> ''),
    CONSTRAINT sections_code_format CHECK (code IS NULL OR code ~ '^[A-Z0-9][A-Z0-9-]{0,19}$'),
    CONSTRAINT sections_capacity_positive CHECK (capacity IS NULL OR capacity BETWEEN 1 AND 1000),
    CONSTRAINT sections_status_check CHECK (status IN ('active', 'inactive'))
);

-- The same section name may exist in other classes or years, never twice in
-- one class and year.
CREATE UNIQUE INDEX IF NOT EXISTS sections_class_year_name_key ON sections (class_id, academic_year_id, lower(name));
-- Section codes (e.g. "6A") identify a section within an academic year.
CREATE UNIQUE INDEX IF NOT EXISTS sections_year_code_key ON sections (academic_year_id, code) WHERE code IS NOT NULL;
CREATE INDEX IF NOT EXISTS sections_academic_year_idx ON sections (academic_year_id, class_id);
CREATE INDEX IF NOT EXISTS sections_status_idx ON sections (status);
