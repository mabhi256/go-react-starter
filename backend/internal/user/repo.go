package user

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/your-org/go-react-starter/backend/internal/rbac"
)

var (
	ErrNotFound = errors.New("user not found")
	ErrConflict = errors.New("email or phone already in use")
)

type Repo struct {
	db *pgxpool.Pool
}

func NewRepo(db *pgxpool.Pool) *Repo { return &Repo{db: db} }

// CreateWithIdentity inserts a user, its roles, and an optional login identity atomically.
func (r *Repo) CreateWithIdentity(ctx context.Context, in NewUser, id *Identity) (*User, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after commit

	var u User
	err = tx.QueryRow(ctx, `
		INSERT INTO users (org_id, name, email, phone, password_hash)
		VALUES ($1,$2,$3,$4,$5)
		RETURNING id, org_id, name, email, phone, status, created_at, updated_at`,
		in.OrgID, in.Name, in.Email, in.Phone, in.PasswordHash,
	).Scan(&u.ID, &u.OrgID, &u.Name, &u.Email, &u.Phone, &u.Status, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, mapErr(err)
	}

	for _, role := range in.Roles {
		if _, err := tx.Exec(ctx, `INSERT INTO user_roles (user_id, role_key) VALUES ($1,$2)`, u.ID, role); err != nil {
			return nil, mapErr(err)
		}
	}
	if id != nil {
		if _, err := tx.Exec(ctx, `INSERT INTO auth_identities (user_id, provider, provider_subject) VALUES ($1,$2,$3)`,
			u.ID, id.Provider, id.Subject); err != nil {
			return nil, mapErr(err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	u.Roles = in.Roles
	return &u, nil
}

// AddIdentity attaches another login provider to an existing user.
func (r *Repo) AddIdentity(ctx context.Context, userID uuid.UUID, id Identity) error {
	_, err := r.db.Exec(ctx, `INSERT INTO auth_identities (user_id, provider, provider_subject) VALUES ($1,$2,$3)`,
		userID, id.Provider, id.Subject)
	return mapErr(err)
}

// FindUserIDByIdentity resolves a (provider, subject) login to its user id.
func (r *Repo) FindUserIDByIdentity(ctx context.Context, provider, subject string) (uuid.UUID, error) {
	var id uuid.UUID
	err := r.db.QueryRow(ctx, `
		SELECT ai.user_id FROM auth_identities ai
		JOIN users u ON u.id = ai.user_id
		WHERE ai.provider=$1 AND ai.provider_subject=$2
		  AND u.status='active' AND u.deleted_at IS NULL`,
		provider, subject).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, ErrNotFound
	}
	return id, err
}

// SuperAdminExists reports whether at least one super_admin user exists.
func (r *Repo) SuperAdminExists(ctx context.Context) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM user_roles WHERE role_key='super_admin')`).Scan(&exists)
	return exists, err
}

// GetAuthByEmail returns the user id and password hash for password login.
func (r *Repo) GetAuthByEmail(ctx context.Context, email string) (uuid.UUID, *string, error) {
	var id uuid.UUID
	var hash *string
	err := r.db.QueryRow(ctx,
		`SELECT id, password_hash FROM users WHERE lower(email)=lower($1) AND status='active' AND deleted_at IS NULL`,
		email).Scan(&id, &hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, nil, ErrNotFound
	}
	return id, hash, err
}

// GetPasswordHashByID returns the stored password hash for a user (for re-verification).
func (r *Repo) GetPasswordHashByID(ctx context.Context, userID uuid.UUID) (*string, error) {
	var hash *string
	err := r.db.QueryRow(ctx,
		`SELECT password_hash FROM users WHERE id=$1 AND status='active' AND deleted_at IS NULL`,
		userID).Scan(&hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return hash, err
}

// UpdatePasswordHash sets a new bcrypt hash for the given user.
func (r *Repo) UpdatePasswordHash(ctx context.Context, userID uuid.UUID, hash string) error {
	ct, err := r.db.Exec(ctx,
		`UPDATE users SET password_hash=$2, updated_at=now() WHERE id=$1 AND deleted_at IS NULL`,
		userID, hash)
	if err != nil {
		return mapErr(err)
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// GetUserIDByPhone resolves a phone to a user id (OTP login).
func (r *Repo) GetUserIDByPhone(ctx context.Context, phone string) (uuid.UUID, error) {
	var id uuid.UUID
	err := r.db.QueryRow(ctx,
		`SELECT id FROM users WHERE phone=$1 AND status='active' AND deleted_at IS NULL`, phone).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, ErrNotFound
	}
	return id, err
}

// LoadIdentity builds the authorization identity (org + roles) for a user.
func (r *Repo) LoadIdentity(ctx context.Context, userID uuid.UUID) (rbac.Identity, error) {
	var orgID *uuid.UUID
	err := r.db.QueryRow(ctx,
		`SELECT org_id FROM users WHERE id=$1 AND deleted_at IS NULL`, userID).Scan(&orgID)
	if errors.Is(err, pgx.ErrNoRows) {
		return rbac.Identity{}, ErrNotFound
	}
	if err != nil {
		return rbac.Identity{}, err
	}
	roles, err := r.roles(ctx, userID)
	if err != nil {
		return rbac.Identity{}, err
	}
	return rbac.Identity{UserID: userID, OrgID: orgID, Roles: roles}, nil
}

// GetByID returns a user with roles, scoped by the caller's org filter (nil = any org).
func (r *Repo) GetByID(ctx context.Context, id uuid.UUID, orgFilter *uuid.UUID) (*User, error) {
	u := User{}
	err := r.db.QueryRow(ctx, `
		SELECT id, org_id, name, email, phone, status, created_at, updated_at
		FROM users
		WHERE id=$1 AND deleted_at IS NULL AND ($2::uuid IS NULL OR org_id=$2)`,
		id, orgFilter,
	).Scan(&u.ID, &u.OrgID, &u.Name, &u.Email, &u.Phone, &u.Status, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if u.Roles, err = r.roles(ctx, id); err != nil {
		return nil, err
	}
	return &u, nil
}

// List returns live users in an org.
func (r *Repo) List(ctx context.Context, orgID uuid.UUID) ([]User, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, org_id, name, email, phone, status, created_at, updated_at
		FROM users WHERE org_id=$1 AND deleted_at IS NULL ORDER BY created_at`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.OrgID, &u.Name, &u.Email, &u.Phone, &u.Status, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// Update applies non-nil fields. Returns ErrNotFound if absent/out of scope.
func (r *Repo) Update(ctx context.Context, id uuid.UUID, orgFilter *uuid.UUID, name, status *string) (*User, error) {
	ct, err := r.db.Exec(ctx, `
		UPDATE users SET
		  name = COALESCE($3, name),
		  status = COALESCE($4, status),
		  updated_at = now()
		WHERE id=$1 AND deleted_at IS NULL AND ($2::uuid IS NULL OR org_id=$2)`,
		id, orgFilter, name, status)
	if err != nil {
		return nil, mapErr(err)
	}
	if ct.RowsAffected() == 0 {
		return nil, ErrNotFound
	}
	return r.GetByID(ctx, id, orgFilter)
}

// SoftDelete sets deleted_at. Returns ErrNotFound if absent/out of scope.
func (r *Repo) SoftDelete(ctx context.Context, id uuid.UUID, orgFilter *uuid.UUID) error {
	ct, err := r.db.Exec(ctx,
		`UPDATE users SET deleted_at=now() WHERE id=$1 AND deleted_at IS NULL AND ($2::uuid IS NULL OR org_id=$2)`,
		id, orgFilter)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repo) roles(ctx context.Context, userID uuid.UUID) ([]rbac.Role, error) {
	rows, err := r.db.Query(ctx, `SELECT role_key FROM user_roles WHERE user_id=$1`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var roles []rbac.Role
	for rows.Next() {
		var role string
		if err := rows.Scan(&role); err != nil {
			return nil, err
		}
		roles = append(roles, rbac.Role(role))
	}
	return roles, rows.Err()
}

func mapErr(err error) error {
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" { // unique_violation
		return ErrConflict
	}
	return fmt.Errorf("user repo: %w", err)
}

