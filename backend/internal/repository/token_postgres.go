package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresTokenRepository implements TokenRepository using pgx.
type PostgresTokenRepository struct {
	db *pgxpool.Pool
}

// NewPostgresTokenRepository creates a repository backed by the pool.
func NewPostgresTokenRepository(db *pgxpool.Pool) *PostgresTokenRepository {
	return &PostgresTokenRepository{db: db}
}

// Revoke marks a token ID as revoked until it would have expired anyway.
func (r *PostgresTokenRepository) Revoke(ctx context.Context, jti, userID uuid.UUID, expiresAt time.Time) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO revoked_tokens (jti, user_id, expires_at) VALUES ($1, $2, $3)
		ON CONFLICT (jti) DO NOTHING`, jti, userID, expiresAt)
	return err
}

// IsRevoked reports whether a token ID has been revoked.
func (r *PostgresTokenRepository) IsRevoked(ctx context.Context, jti uuid.UUID) (bool, error) {
	var revoked bool
	err := r.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM revoked_tokens WHERE jti = $1)`, jti).Scan(&revoked)
	return revoked, err
}

// DeleteExpired removes revocation records for tokens that have expired.
func (r *PostgresTokenRepository) DeleteExpired(ctx context.Context) (int64, error) {
	tag, err := r.db.Exec(ctx, `DELETE FROM revoked_tokens WHERE expires_at < now()`)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}
