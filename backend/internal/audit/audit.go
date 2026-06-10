// Package audit records an append-only trail of access to patient / EHR data, as required
// by the Health Data Management Policy. Services call Recorder.Record; in the API process
// that enqueues an asynq task which the worker inserts via Repo. Rows are never updated or
// deleted.
package audit

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/your-org/go-react-starter/backend/internal/rbac"
)

// Entry is one audit record. OrgID is nil for super-admin / cross-org actions.
type Entry struct {
	OrgID        *uuid.UUID      `json:"org_id"`
	ActorID      uuid.UUID       `json:"actor_id"`
	Action       string          `json:"action"`        // e.g. "patient.read"
	ResourceType string          `json:"resource_type"` // e.g. "patient"
	ResourceID   string          `json:"resource_id"`
	Purpose      string          `json:"purpose,omitempty"`
	IP           string          `json:"ip,omitempty"`
	UserAgent    string          `json:"user_agent,omitempty"`
	Metadata     json.RawMessage `json:"metadata,omitempty"`
}

// FromIdentity pre-fills actor + org from the request identity.
func FromIdentity(id rbac.Identity, action, resourceType, resourceID string) Entry {
	return Entry{
		OrgID:        id.OrgID,
		ActorID:      id.UserID,
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
	}
}

// Repo performs the append-only insert.
type Repo struct {
	db *pgxpool.Pool
}

func NewRepo(db *pgxpool.Pool) *Repo { return &Repo{db: db} }

func (r *Repo) Insert(ctx context.Context, e Entry) error {
	const q = `
		INSERT INTO audit_logs
		  (org_id, actor_id, action, resource_type, resource_id, purpose, ip, user_agent, metadata, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`
	_, err := r.db.Exec(ctx, q,
		e.OrgID, e.ActorID, e.Action, e.ResourceType, e.ResourceID,
		nullStr(e.Purpose), nullStr(e.IP), nullStr(e.UserAgent), e.Metadata, time.Now().UTC())
	return err
}

func nullStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

