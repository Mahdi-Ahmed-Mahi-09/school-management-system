-- Feature 03: Student Management
-- Student records are separate from login accounts (users): a student may
-- never sign in. Future modules (enrollment, attendance, fees, guardian
-- portal) reference students.id from their own tables.

CREATE TABLE IF NOT EXISTS students (
    id                          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    admission_number            VARCHAR(32)  NOT NULL,
    full_name                   VARCHAR(120) NOT NULL,
    date_of_birth               DATE         NOT NULL,
    gender                      VARCHAR(16)  NOT NULL,
    blood_group                 VARCHAR(3),
    phone                       VARCHAR(24),
    email                       VARCHAR(254),
    address                     VARCHAR(500) NOT NULL,
    father_name                 VARCHAR(120),
    father_phone                VARCHAR(24),
    mother_name                 VARCHAR(120),
    mother_phone                VARCHAR(24),
    guardian_name               VARCHAR(120) NOT NULL,
    guardian_relation           VARCHAR(40),
    guardian_phone              VARCHAR(24)  NOT NULL,
    emergency_contact_name      VARCHAR(120) NOT NULL,
    emergency_contact_relation  VARCHAR(40),
    emergency_contact_phone     VARCHAR(24)  NOT NULL,
    admission_date              DATE         NOT NULL,
    status                      VARCHAR(16)  NOT NULL DEFAULT 'active',
    -- Set when a photo is uploaded; used for cache-busting the photo URL.
    photo_updated_at            TIMESTAMPTZ,
    created_by                  UUID REFERENCES users (id) ON DELETE SET NULL,
    updated_by                  UUID REFERENCES users (id) ON DELETE SET NULL,
    created_at                  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at                  TIMESTAMPTZ  NOT NULL DEFAULT now(),

    CONSTRAINT students_admission_number_format CHECK (admission_number ~ '^[A-Z0-9][A-Z0-9/_-]{1,31}$'),
    CONSTRAINT students_full_name_present CHECK (btrim(full_name) <> ''),
    CONSTRAINT students_gender_check CHECK (gender IN ('male', 'female', 'other')),
    CONSTRAINT students_blood_group_check CHECK (blood_group IS NULL OR blood_group IN ('A+', 'A-', 'B+', 'B-', 'AB+', 'AB-', 'O+', 'O-')),
    CONSTRAINT students_email_lowercase CHECK (email IS NULL OR (email = lower(email) AND position('@' IN email) > 1)),
    CONSTRAINT students_address_present CHECK (btrim(address) <> ''),
    CONSTRAINT students_guardian_present CHECK (btrim(guardian_name) <> '' AND btrim(guardian_phone) <> ''),
    CONSTRAINT students_emergency_present CHECK (btrim(emergency_contact_name) <> '' AND btrim(emergency_contact_phone) <> ''),
    CONSTRAINT students_status_check CHECK (status IN ('active', 'inactive')),
    CONSTRAINT students_birth_before_admission CHECK (date_of_birth < admission_date),
    CONSTRAINT students_birth_reasonable CHECK (date_of_birth >= DATE '1900-01-01')
);

CREATE UNIQUE INDEX IF NOT EXISTS students_admission_number_key ON students (upper(admission_number));
CREATE INDEX IF NOT EXISTS students_status_idx ON students (status);
CREATE INDEX IF NOT EXISTS students_admission_date_idx ON students (admission_date DESC, created_at DESC);
CREATE INDEX IF NOT EXISTS students_full_name_idx ON students (lower(full_name));
CREATE INDEX IF NOT EXISTS students_gender_idx ON students (gender);

-- Photos live in their own table so list queries never load image bytes.
CREATE TABLE IF NOT EXISTS student_photos (
    student_id   UUID PRIMARY KEY REFERENCES students (id) ON DELETE CASCADE,
    content_type VARCHAR(32) NOT NULL,
    data         BYTEA       NOT NULL,
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT student_photos_content_type CHECK (content_type IN ('image/jpeg', 'image/png', 'image/webp')),
    CONSTRAINT student_photos_size CHECK (octet_length(data) BETWEEN 1 AND 1048576)
);
