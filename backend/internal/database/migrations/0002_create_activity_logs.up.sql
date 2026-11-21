-- Feature 02: Admin Dashboard
-- A lightweight, append-only activity log that powers the dashboard's
-- "Recent activity" feed. Later modules record their own actions here.

CREATE TABLE IF NOT EXISTS activity_logs (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    -- NULL when performed by the system (e.g. seeding) or the actor was removed.
    actor_id    UUID REFERENCES users (id) ON DELETE SET NULL,
    action      VARCHAR(64)  NOT NULL,
    entity_type VARCHAR(32)  NOT NULL,
    -- Polymorphic reference to the affected record; intentionally no FK.
    entity_id   UUID,
    summary     VARCHAR(255) NOT NULL,
    metadata    JSONB        NOT NULL DEFAULT '{}'::jsonb,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),

    CONSTRAINT activity_logs_action_format CHECK (action ~ '^[a-z][a-z_]*\.[a-z][a-z_]*$'),
    CONSTRAINT activity_logs_entity_type_format CHECK (entity_type ~ '^[a-z][a-z_]*$'),
    CONSTRAINT activity_logs_summary_present CHECK (btrim(summary) <> ''),
    CONSTRAINT activity_logs_metadata_object CHECK (jsonb_typeof(metadata) = 'object')
);

CREATE INDEX IF NOT EXISTS activity_logs_created_at_idx ON activity_logs (created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS activity_logs_entity_idx ON activity_logs (entity_type, entity_id);
CREATE INDEX IF NOT EXISTS activity_logs_actor_idx ON activity_logs (actor_id);

-- Supports the dashboard's per-month account growth aggregation.
CREATE INDEX IF NOT EXISTS users_role_created_at_idx ON users (role, created_at);

-- Backfill: every existing account was genuinely created at users.created_at,
-- so record that real event. No other history is invented.
INSERT INTO activity_logs (actor_id, action, entity_type, entity_id, summary, metadata, created_at)
SELECT u.created_by,
       'user.created',
       'user',
       u.id,
       left('Created ' || u.role || ' account for ' || u.first_name || ' ' || u.last_name, 255),
       jsonb_build_object('role', u.role, 'backfilled', true),
       u.created_at
FROM users u
WHERE NOT EXISTS (
    SELECT 1 FROM activity_logs l WHERE l.entity_type = 'user' AND l.entity_id = u.id AND l.action = 'user.created'
);
