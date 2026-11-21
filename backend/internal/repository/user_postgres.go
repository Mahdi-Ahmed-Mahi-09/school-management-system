package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/models"
)

const userColumns = `id, username, email, first_name, last_name, phone, password_hash,
	role, status, token_version, must_change_password, last_login_at,
	password_changed_at, created_by, created_at, updated_at`

// PostgresUserRepository implements UserRepository using pgx.
type PostgresUserRepository struct {
	db *pgxpool.Pool
}

// NewPostgresUserRepository creates a repository backed by the pool.
func NewPostgresUserRepository(db *pgxpool.Pool) *PostgresUserRepository {
	return &PostgresUserRepository{db: db}
}

func scanUser(row pgx.Row) (*models.User, error) {
	var (
		u      models.User
		role   string
		status string
	)
	err := row.Scan(&u.ID, &u.Username, &u.Email, &u.FirstName, &u.LastName, &u.Phone,
		&u.PasswordHash, &role, &status, &u.TokenVersion, &u.MustChangePassword,
		&u.LastLoginAt, &u.PasswordChangedAt, &u.CreatedBy, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	u.Role = models.Role(role)
	u.Status = models.Status(status)
	return &u, nil
}

func mapWriteError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		switch pgErr.ConstraintName {
		case "users_email_key":
			return ErrDuplicateEmail
		case "users_username_key":
			return ErrDuplicateUsername
		}
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

// Create inserts a new user.
func (r *PostgresUserRepository) Create(ctx context.Context, u *models.User) (*models.User, error) {
	row := r.db.QueryRow(ctx, `
		INSERT INTO users (username, email, first_name, last_name, phone, password_hash,
			role, status, must_change_password, created_by, updated_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $10)
		RETURNING `+userColumns,
		u.Username, u.Email, u.FirstName, u.LastName, u.Phone, u.PasswordHash,
		string(u.Role), string(u.Status), u.MustChangePassword, u.CreatedBy)
	created, err := scanUser(row)
	if err != nil {
		return nil, mapWriteError(err)
	}
	return created, nil
}

// GetByID loads a user by primary key.
func (r *PostgresUserRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	return scanUser(r.db.QueryRow(ctx, `SELECT `+userColumns+` FROM users WHERE id = $1`, id))
}

// GetByIdentifier loads a user by email or username.
// Usernames cannot contain "@", so the two never collide.
func (r *PostgresUserRepository) GetByIdentifier(ctx context.Context, identifier string) (*models.User, error) {
	return scanUser(r.db.QueryRow(ctx,
		`SELECT `+userColumns+` FROM users WHERE lower(email) = lower($1) OR lower(username) = lower($1) LIMIT 1`,
		identifier))
}

var sortExpressions = map[string][]string{
	SortCreatedAt: {"created_at"},
	SortName:      {"lower(first_name)", "lower(last_name)"},
	SortEmail:     {"lower(email)"},
	SortRole:      {"role"},
	SortLastLogin: {"last_login_at"},
}

// escapeLike escapes LIKE wildcards so user input is matched literally.
func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

// List returns a filtered, sorted page of users and the total match count.
func (r *PostgresUserRepository) List(ctx context.Context, f UserFilter) ([]models.User, int, error) {
	var (
		conditions []string
		args       []any
	)
	if f.Search != "" {
		args = append(args, "%"+escapeLike(f.Search)+"%")
		n := len(args)
		conditions = append(conditions, fmt.Sprintf(
			"((first_name || ' ' || last_name) ILIKE $%[1]d OR email ILIKE $%[1]d OR username ILIKE $%[1]d)", n))
	}
	if f.Role != nil {
		args = append(args, string(*f.Role))
		conditions = append(conditions, fmt.Sprintf("role = $%d", len(args)))
	}
	if f.Status != nil {
		args = append(args, string(*f.Status))
		conditions = append(conditions, fmt.Sprintf("status = $%d", len(args)))
	}
	where := ""
	if len(conditions) > 0 {
		where = " WHERE " + strings.Join(conditions, " AND ")
	}

	var total int
	if err := r.db.QueryRow(ctx, "SELECT count(*) FROM users"+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	exprs, ok := sortExpressions[f.SortBy]
	if !ok {
		exprs = sortExpressions[SortCreatedAt]
	}
	direction := " ASC NULLS LAST"
	if f.SortDesc {
		direction = " DESC NULLS LAST"
	}
	order := make([]string, 0, len(exprs)+1)
	for _, e := range exprs {
		order = append(order, e+direction)
	}
	order = append(order, "id ASC")

	args = append(args, f.PageSize, (f.Page-1)*f.PageSize)
	query := fmt.Sprintf("SELECT %s FROM users%s ORDER BY %s LIMIT $%d OFFSET $%d",
		userColumns, where, strings.Join(order, ", "), len(args)-1, len(args))

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	users := []models.User{}
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, 0, err
		}
		users = append(users, *u)
	}
	return users, total, rows.Err()
}

// Stats summarises account counts.
func (r *PostgresUserRepository) Stats(ctx context.Context) (*models.UserStats, error) {
	stats := &models.UserStats{ByRole: map[models.Role]int{}}
	for _, role := range models.Roles {
		stats.ByRole[role] = 0
	}
	err := r.db.QueryRow(ctx, `
		SELECT count(*),
		       count(*) FILTER (WHERE status = 'active'),
		       count(*) FILTER (WHERE status = 'inactive'),
		       count(*) FILTER (WHERE created_at >= now() - interval '30 days')
		FROM users`).Scan(&stats.Total, &stats.Active, &stats.Inactive, &stats.RecentlyAdded)
	if err != nil {
		return nil, err
	}

	rows, err := r.db.Query(ctx, `SELECT role, count(*) FROM users GROUP BY role`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			role  string
			count int
		)
		if err := rows.Scan(&role, &count); err != nil {
			return nil, err
		}
		stats.ByRole[models.Role(role)] = count
	}
	return stats, rows.Err()
}

// UpdateDetails saves a user's editable profile fields.
func (r *PostgresUserRepository) UpdateDetails(ctx context.Context, u *models.User, actorID uuid.UUID) (*models.User, error) {
	row := r.db.QueryRow(ctx, `
		UPDATE users
		SET username = $2, email = $3, first_name = $4, last_name = $5, phone = $6,
		    updated_by = $7, updated_at = now()
		WHERE id = $1
		RETURNING `+userColumns,
		u.ID, u.Username, u.Email, u.FirstName, u.LastName, u.Phone, actorID)
	updated, err := scanUser(row)
	if err != nil {
		return nil, mapWriteError(err)
	}
	return updated, nil
}

// guardedAdminUpdate runs an update in a transaction that first locks every
// active admin row, so concurrent requests cannot together remove the last admin.
func (r *PostgresUserRepository) guardedAdminUpdate(
	ctx context.Context,
	id uuid.UUID,
	removesAdmin func(target *models.User) bool,
	update func(tx pgx.Tx) (*models.User, error),
) (*models.User, error) {
	var result *models.User
	err := pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx,
			`SELECT id FROM users WHERE role = 'admin' AND status = 'active' FOR UPDATE`)
		if err != nil {
			return err
		}
		admins, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
		if err != nil {
			return err
		}

		target, err := scanUser(tx.QueryRow(ctx,
			`SELECT `+userColumns+` FROM users WHERE id = $1 FOR UPDATE`, id))
		if err != nil {
			return err
		}
		if removesAdmin(target) && len(admins) <= 1 {
			return ErrLastAdmin
		}
		result, err = update(tx)
		return err
	})
	if err != nil {
		return nil, mapWriteError(err)
	}
	return result, nil
}

func isActiveAdmin(u *models.User) bool {
	return u.Role == models.RoleAdmin && u.Status == models.StatusActive
}

// UpdateRole changes a user's role.
func (r *PostgresUserRepository) UpdateRole(ctx context.Context, id uuid.UUID, role models.Role, actorID uuid.UUID) (*models.User, error) {
	return r.guardedAdminUpdate(ctx, id,
		func(t *models.User) bool { return isActiveAdmin(t) && role != models.RoleAdmin },
		func(tx pgx.Tx) (*models.User, error) {
			return scanUser(tx.QueryRow(ctx, `
				UPDATE users SET role = $2, updated_by = $3, updated_at = now()
				WHERE id = $1 RETURNING `+userColumns, id, string(role), actorID))
		})
}

// UpdateStatus activates or deactivates an account.
func (r *PostgresUserRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status models.Status, actorID uuid.UUID) (*models.User, error) {
	return r.guardedAdminUpdate(ctx, id,
		func(t *models.User) bool { return isActiveAdmin(t) && status != models.StatusActive },
		func(tx pgx.Tx) (*models.User, error) {
			return scanUser(tx.QueryRow(ctx, `
				UPDATE users SET status = $2, updated_by = $3, updated_at = now()
				WHERE id = $1 RETURNING `+userColumns, id, string(status), actorID))
		})
}

// UpdatePassword stores a new password hash and revokes existing sessions.
func (r *PostgresUserRepository) UpdatePassword(ctx context.Context, id uuid.UUID, hash string, mustChange bool, actorID uuid.UUID) (*models.User, error) {
	row := r.db.QueryRow(ctx, `
		UPDATE users
		SET password_hash = $2, must_change_password = $3, token_version = token_version + 1,
		    password_changed_at = now(), updated_by = $4, updated_at = now()
		WHERE id = $1
		RETURNING `+userColumns, id, hash, mustChange, actorID)
	updated, err := scanUser(row)
	if err != nil {
		return nil, mapWriteError(err)
	}
	return updated, nil
}

// TouchLastLogin records a successful sign-in.
func (r *PostgresUserRepository) TouchLastLogin(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Exec(ctx, `UPDATE users SET last_login_at = now() WHERE id = $1`, id)
	return err
}
