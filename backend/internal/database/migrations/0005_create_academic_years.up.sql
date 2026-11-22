-- Feature 05: Academic Year Management
-- One row per academic year. Status is changed only by explicit admin actions
-- (activate / complete); is_current is derived from it so the two can never
-- disagree. Future modules (classes, enrollment, exams, fees) reference
-- academic_years.id with ON DELETE RESTRICT, which blocks deleting a year that
-- is still in use (reported to clients as ACADEMIC_YEAR_IN_USE).

CREATE TABLE IF NOT EXISTS academic_years (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name          VARCHAR(50)  NOT NULL,
    start_date    DATE         NOT NULL,
    end_date      DATE         NOT NULL,
    status        VARCHAR(16)  NOT NULL DEFAULT 'upcoming',
    is_current    BOOLEAN      GENERATED ALWAYS AS (status = 'active') STORED,
    description   VARCHAR(500),
    activated_at  TIMESTAMPTZ,
    completed_at  TIMESTAMPTZ,
    created_by    UUID REFERENCES users (id) ON DELETE SET NULL,
    updated_by    UUID REFERENCES users (id) ON DELETE SET NULL,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT now(),

    CONSTRAINT academic_years_name_present CHECK (btrim(name) <> ''),
    CONSTRAINT academic_years_dates_order CHECK (start_date < end_date),
    CONSTRAINT academic_years_status_check CHECK (status IN ('upcoming', 'active', 'completed')),
    -- No two academic years may share a calendar day (inclusive ranges).
    CONSTRAINT academic_years_no_overlap EXCLUDE USING gist (daterange(start_date, end_date, '[]') WITH &&)
);

-- Names are unique regardless of case.
CREATE UNIQUE INDEX IF NOT EXISTS academic_years_name_key ON academic_years (lower(name));
-- At most one current (active) academic year.
CREATE UNIQUE INDEX IF NOT EXISTS academic_years_one_current ON academic_years (is_current) WHERE is_current;
CREATE INDEX IF NOT EXISTS academic_years_status_idx ON academic_years (status);
CREATE INDEX IF NOT EXISTS academic_years_start_date_idx ON academic_years (start_date DESC);
