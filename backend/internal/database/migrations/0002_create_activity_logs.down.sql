-- Rolls back Feature 02. WARNING: this deletes the activity history.
DROP INDEX IF EXISTS users_role_created_at_idx;
DROP TABLE IF EXISTS activity_logs;
