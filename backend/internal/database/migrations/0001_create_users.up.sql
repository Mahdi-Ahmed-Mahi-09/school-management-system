-- Feature 01: Authentication & User Management
-- Creates the users table and the revoked token list used for logout.

CREATE TABLE IF NOT EXISTS users (
    id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username             VARCHAR(32)  NOT NULL,
    email                VARCHAR(254) NOT NULL,
    first_name           VARCHAR(80)  NOT NULL,
    last_name            VARCHAR(80)  NOT NULL,
    phone                VARCHAR(24),
    password_hash        TEXT         NOT NULL,
    role                 VARCHAR(16)  NOT NULL,
    status               VARCHAR(16)  NOT NULL DEFAULT 'active',
    -- Incremented whenever existing sessions must be invalidated (e.g. password change).
    token_version        INTEGER      NOT NULL DEFAULT 0,
    must_change_password BOOLEAN      NOT NULL DEFAULT FALSE,
    last_login_at        TIMESTAMPTZ,
    password_changed_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    created_by           UUID REFERENCES users (id) ON DELETE SET NULL,
    updated_by           UUID REFERENCES users (id) ON DELETE SET NULL,
    created_at           TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ  NOT NULL DEFAULT now(),

    CONSTRAINT users_role_check CHECK (role IN ('admin', 'teacher', 'student', 'guardian')),
    CONSTRAINT users_status_check CHECK (status IN ('active', 'inactive')),
    CONSTRAINT users_username_format CHECK (username ~ '^[a-z0-9][a-z0-9._-]{2,31}$'),
    CONSTRAINT users_email_lowercase CHECK (email = lower(email) AND position('@' IN email) > 1),
    CONSTRAINT users_first_name_present CHECK (btrim(first_name) <> ''),
    CONSTRAINT users_last_name_present CHECK (btrim(last_name) <> ''),
    CONSTRAINT users_password_hash_bcrypt CHECK (password_hash LIKE '$2%'),
    CONSTRAINT users_token_version_positive CHECK (token_version >= 0)
);

CREATE UNIQUE INDEX IF NOT EXISTS users_email_key ON users (lower(email));
CREATE UNIQUE INDEX IF NOT EXISTS users_username_key ON users (lower(username));
CREATE INDEX IF NOT EXISTS users_role_status_idx ON users (role, status);
CREATE INDEX IF NOT EXISTS users_status_idx ON users (status);
CREATE INDEX IF NOT EXISTS users_created_at_idx ON users (created_at DESC);
CREATE INDEX IF NOT EXISTS users_name_idx ON users (lower(first_name), lower(last_name));

CREATE TABLE IF NOT EXISTS revoked_tokens (
    jti        UUID PRIMARY KEY,
    user_id    UUID        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS revoked_tokens_expires_at_idx ON revoked_tokens (expires_at);
CREATE INDEX IF NOT EXISTS revoked_tokens_user_id_idx ON revoked_tokens (user_id);
