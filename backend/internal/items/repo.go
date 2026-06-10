package items

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("item not found")

type Repo struct {
	db *pgxpool.Pool
}

func NewRepo(db *pgxpool.Pool) *Repo { return &Repo{db: db} }

func (r *Repo) Create(ctx context.Context, in NewItem) (*Item, error) {
	it := Item{}
	err := r.db.QueryRow(ctx, `
		INSERT INTO items (org_id, name, description, created_by)
		VALUES ($1,$2,$3,$4)
		RETURNING id, org_id, name, description, created_by, created_at, updated_at`,
		in.OrgID, in.Name, in.Description, in.CreatedBy,
	).Scan(&it.ID, &it.OrgID, &it.Name, &it.Description, &it.CreatedBy, &it.CreatedAt, &it.UpdatedAt)
	return &it, err
}

func (r *Repo) GetByID(ctx context.Context, id uuid.UUID, orgFilter *uuid.UUID) (*Item, error) {
	it := Item{}
	err := r.db.QueryRow(ctx, `
		SELECT id, org_id, name, description, created_by, created_at, updated_at
		FROM items
		WHERE id=$1 AND deleted_at IS NULL AND ($2::uuid IS NULL OR org_id=$2)`,
		id, orgFilter,
	).Scan(&it.ID, &it.OrgID, &it.Name, &it.Description, &it.CreatedBy, &it.CreatedAt, &it.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &it, err
}

func (r *Repo) List(ctx context.Context, orgFilter *uuid.UUID) ([]Item, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, org_id, name, description, created_by, created_at, updated_at
		FROM items
		WHERE deleted_at IS NULL AND ($1::uuid IS NULL OR org_id=$1)
		ORDER BY created_at DESC`,
		orgFilter,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Item
	for rows.Next() {
		it := Item{}
		if err := rows.Scan(&it.ID, &it.OrgID, &it.Name, &it.Description, &it.CreatedBy, &it.CreatedAt, &it.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

func (r *Repo) Update(ctx context.Context, id uuid.UUID, orgFilter *uuid.UUID, p Patch) (*Item, error) {
	ct, err := r.db.Exec(ctx, `
		UPDATE items SET
		  name = COALESCE($3, name),
		  description = COALESCE($4, description),
		  updated_at = now()
		WHERE id=$1 AND deleted_at IS NULL AND ($2::uuid IS NULL OR org_id=$2)`,
		id, orgFilter, p.Name, p.Description)
	if err != nil {
		return nil, err
	}
	if ct.RowsAffected() == 0 {
		return nil, ErrNotFound
	}
	return r.GetByID(ctx, id, orgFilter)
}

func (r *Repo) Delete(ctx context.Context, id uuid.UUID, orgFilter *uuid.UUID) error {
	ct, err := r.db.Exec(ctx, `
		UPDATE items SET deleted_at=now(), updated_at=now()
		WHERE id=$1 AND deleted_at IS NULL AND ($2::uuid IS NULL OR org_id=$2)`,
		id, orgFilter)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
