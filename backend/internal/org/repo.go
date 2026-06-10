package org

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("organization not found")

type Repo struct {
	db *pgxpool.Pool
}

func NewRepo(db *pgxpool.Pool) *Repo { return &Repo{db: db} }

func (r *Repo) Create(ctx context.Context, in NewOrg) (*Organization, error) {
	addr, err := json.Marshal(in.Address)
	if err != nil {
		return nil, err
	}
	o := Organization{}
	var raw []byte
	err = r.db.QueryRow(ctx, `
		INSERT INTO organizations (name, type, address, contact_email, contact_phone)
		VALUES ($1,$2,$3,$4,$5)
		RETURNING id, name, type, status, address, contact_email, contact_phone, created_at, updated_at`,
		in.Name, in.Type, addr, in.ContactEmail, in.ContactPhone,
	).Scan(&o.ID, &o.Name, &o.Type, &o.Status, &raw, &o.ContactEmail, &o.ContactPhone, &o.CreatedAt, &o.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &o, json.Unmarshal(raw, &o.Address)
}

func (r *Repo) GetByID(ctx context.Context, id uuid.UUID) (*Organization, error) {
	o := Organization{}
	var raw []byte
	err := r.db.QueryRow(ctx, `
		SELECT id, name, type, status, address, contact_email, contact_phone, created_at, updated_at
		FROM organizations WHERE id=$1 AND deleted_at IS NULL`, id,
	).Scan(&o.ID, &o.Name, &o.Type, &o.Status, &raw, &o.ContactEmail, &o.ContactPhone, &o.CreatedAt, &o.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &o, json.Unmarshal(raw, &o.Address)
}

func (r *Repo) List(ctx context.Context, q string) ([]Organization, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, name, type, status, address, contact_email, contact_phone, created_at, updated_at
		FROM organizations WHERE deleted_at IS NULL AND ($1 = '' OR name ILIKE '%' || $1 || '%') ORDER BY created_at`,
		q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Organization
	for rows.Next() {
		o := Organization{}
		var raw []byte
		if err := rows.Scan(&o.ID, &o.Name, &o.Type, &o.Status, &raw, &o.ContactEmail, &o.ContactPhone, &o.CreatedAt, &o.UpdatedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(raw, &o.Address); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

func (r *Repo) Update(ctx context.Context, id uuid.UUID, p Patch) (*Organization, error) {
	var addr []byte
	if p.Address != nil {
		b, err := json.Marshal(*p.Address)
		if err != nil {
			return nil, err
		}
		addr = b
	}
	ct, err := r.db.Exec(ctx, `
		UPDATE organizations SET
		  name = COALESCE($2, name),
		  type = COALESCE($3, type),
		  status = COALESCE($4::entity_status, status),
		  address = COALESCE($5, address),
		  contact_email = COALESCE($6, contact_email),
		  contact_phone = COALESCE($7, contact_phone),
		  updated_at = now()
		WHERE id=$1 AND deleted_at IS NULL`,
		id, p.Name, p.Type, p.Status, addr, p.ContactEmail, p.ContactPhone)
	if err != nil {
		return nil, err
	}
	if ct.RowsAffected() == 0 {
		return nil, ErrNotFound
	}
	return r.GetByID(ctx, id)
}

// SoftDelete sets deleted_at; the row and its data are retained.
func (r *Repo) SoftDelete(ctx context.Context, id uuid.UUID) error {
	ct, err := r.db.Exec(ctx, `UPDATE organizations SET deleted_at=now() WHERE id=$1 AND deleted_at IS NULL`, id)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
