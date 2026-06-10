package rbac

import (
	"context"

	"github.com/google/uuid"
)

type Role string

const (
	RoleSuperAdmin Role = "super_admin" // cross-org; org_id is nil
	RoleAdmin      Role = "admin"       // administrator of one org
	RoleUser       Role = "user"        // standard user within one org
)

// Identity is the authenticated caller derived from the access token.
type Identity struct {
	UserID uuid.UUID
	OrgID  *uuid.UUID // nil for super_admin
	Roles  []Role
}

func (i Identity) HasRole(r Role) bool {
	for _, role := range i.Roles {
		if role == r {
			return true
		}
	}
	return false
}

func (i Identity) IsSuperAdmin() bool { return i.HasRole(RoleSuperAdmin) }

// CanAccessOrg reports whether the caller may act within the given org.
func (i Identity) CanAccessOrg(orgID uuid.UUID) bool {
	if i.IsSuperAdmin() {
		return true
	}
	return i.OrgID != nil && *i.OrgID == orgID
}

// EffectiveOrgFilter returns the org id that repository queries must filter by,
// or nil to mean "no filter" (super-admin sees all orgs).
func (i Identity) EffectiveOrgFilter() *uuid.UUID {
	if i.IsSuperAdmin() {
		return nil
	}
	return i.OrgID
}

type ctxKey struct{}

var ContextKey = ctxKey{}

func WithIdentity(ctx context.Context, id Identity) context.Context {
	return context.WithValue(ctx, ContextKey, id)
}

func FromContext(ctx context.Context) (Identity, bool) {
	id, ok := ctx.Value(ContextKey).(Identity)
	return id, ok
}
