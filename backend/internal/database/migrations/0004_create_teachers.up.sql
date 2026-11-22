-- Feature 04: Teacher Management
-- Teacher records are HR/profile data. A record may optionally be linked to
-- one login account (users.role = 'teacher'); authentication stays in users.
-- Future modules (subject assignment, routines, attendance, payroll)
-- reference teachers.id from their own tables.

CREATE TABLE IF NOT EXISTS teachers (
    id                          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    employee_id                 VARCHAR(32)   NOT NULL,
    full_name                   VARCHAR(120)  NOT NULL,
    gender                      VARCHAR(16)   NOT NULL,
    date_of_birth               DATE,
    phone                       VARCHAR(24)   NOT NULL,
    email                       VARCHAR(254),
    address                     VARCHAR(500),
    qualification               VARCHAR(200),
    specialization              VARCHAR(120),
    designation                 VARCHAR(80)   NOT NULL,
    employment_type             VARCHAR(16)   NOT NULL,
    joining_date                DATE          NOT NULL,
    -- Sensitive: only returned to holders of teachers:view_salary.
    salary                      NUMERIC(12, 2),
    status                      VARCHAR(16)   NOT NULL DEFAULT 'active',
    emergency_contact_name      VARCHAR(120),
    emergency_contact_relation  VARCHAR(40),
    emergency_contact_phone     VARCHAR(24),
    notes                       VARCHAR(1000),
    -- Optional link to the teacher's login account (one account per record).
    user_id                     UUID REFERENCES users (id) ON DELETE SET NULL,
    photo_updated_at            TIMESTAMPTZ,
    created_by                  UUID REFERENCES users (id) ON DELETE SET NULL,
    updated_by                  UUID REFERENCES users (id) ON DELETE SET NULL,
    created_at                  TIMESTAMPTZ   NOT NULL DEFAULT now(),
    updated_at                  TIMESTAMPTZ   NOT NULL DEFAULT now(),

    CONSTRAINT teachers_employee_id_format CHECK (employee_id ~ '^[A-Z0-9][A-Z0-9/_-]{1,31}$'),
    CONSTRAINT teachers_full_name_present CHECK (btrim(full_name) <> ''),
    CONSTRAINT teachers_gender_check CHECK (gender IN ('male', 'female', 'other')),
    CONSTRAINT teachers_phone_present CHECK (btrim(phone) <> ''),
    CONSTRAINT teachers_email_lowercase CHECK (email IS NULL OR (email = lower(email) AND position('@' IN email) > 1)),
    CONSTRAINT teachers_designation_present CHECK (btrim(designation) <> ''),
    CONSTRAINT teachers_employment_type_check CHECK (employment_type IN ('full_time', 'part_time')),
    CONSTRAINT teachers_status_check CHECK (status IN ('active', 'inactive')),
    CONSTRAINT teachers_salary_non_negative CHECK (salary IS NULL OR salary >= 0),
    CONSTRAINT teachers_birth_before_joining CHECK (date_of_birth IS NULL OR date_of_birth < joining_date),
    CONSTRAINT teachers_birth_reasonable CHECK (date_of_birth IS NULL OR date_of_birth >= DATE '1900-01-01'),
    CONSTRAINT teachers_emergency_contact_complete CHECK (
        (emergency_contact_name IS NULL) = (emergency_contact_phone IS NULL)
    )
);

CREATE UNIQUE INDEX IF NOT EXISTS teachers_employee_id_key ON teachers (upper(employee_id));
-- A login account can be linked to at most one teacher record.
CREATE UNIQUE INDEX IF NOT EXISTS teachers_user_id_key ON teachers (user_id) WHERE user_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS teachers_status_idx ON teachers (status);
CREATE INDEX IF NOT EXISTS teachers_employment_type_idx ON teachers (employment_type);
CREATE INDEX IF NOT EXISTS teachers_designation_idx ON teachers (lower(designation));
CREATE INDEX IF NOT EXISTS teachers_joining_date_idx ON teachers (joining_date DESC, created_at DESC);
CREATE INDEX IF NOT EXISTS teachers_full_name_idx ON teachers (lower(full_name));

CREATE TABLE IF NOT EXISTS teacher_photos (
    teacher_id   UUID PRIMARY KEY REFERENCES teachers (id) ON DELETE CASCADE,
    content_type VARCHAR(32) NOT NULL,
    data         BYTEA       NOT NULL,
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT teacher_photos_content_type CHECK (content_type IN ('image/jpeg', 'image/png', 'image/webp')),
    CONSTRAINT teacher_photos_size CHECK (octet_length(data) BETWEEN 1 AND 1048576)
);
