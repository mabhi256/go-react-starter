// Package apiutil holds small helpers shared by every domain handler: reading the request
// identity and emitting audit entries. Domain packages import it; it imports only the
// cross-cutting packages (rbac, reqctx, audit).
package apiutil

import (
	"context"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"

	"github.com/your-org/go-react-starter/backend/internal/audit"
	"github.com/your-org/go-react-starter/backend/internal/rbac"
	"github.com/your-org/go-react-starter/backend/internal/reqctx"
)

// Identity returns the authenticated caller. On protected routes the auth middleware
// guarantees it is present; on public routes the zero value is returned.
func Identity(ctx context.Context) rbac.Identity {
	id, _ := rbac.FromContext(ctx)
	return id
}

// Audit records a best-effort access entry built from the request identity + metadata.
// Enqueue failures are intentionally swallowed so auditing never breaks the request path;
// the worker logs persistence failures.
func Audit(ctx context.Context, rec audit.Recorder, action, resourceType, resourceID string) {
	e := audit.FromIdentity(Identity(ctx), action, resourceType, resourceID)
	m := reqctx.FromContext(ctx)
	e.IP, e.UserAgent = m.IP, m.UserAgent
	_ = rec.Record(ctx, e)
}

// RequireSuperAdmin returns a 403 error unless the caller is a platform super-admin.
func RequireSuperAdmin(ctx context.Context) error {
	if !Identity(ctx).IsSuperAdmin() {
		return huma.Error403Forbidden("super-admin role required")
	}
	return nil
}

// RequireAnyRole returns a 403 error unless the caller holds at least one of the roles.
func RequireAnyRole(ctx context.Context, roles ...rbac.Role) error {
	id := Identity(ctx)
	for _, r := range roles {
		if id.HasRole(r) {
			return nil
		}
	}
	return huma.Error403Forbidden("insufficient role")
}

// ResolveOrg determines which org a request acts on and authorizes access.
//
// For a super-admin, requested (if non-nil) is used as-is (they may target any org); a nil
// requested for a super-admin is an error since they have no implicit org. For everyone
// else the caller's own org is used and must match requested when one is supplied.
func ResolveOrg(ctx context.Context, requested *uuid.UUID) (uuid.UUID, error) {
	id := Identity(ctx)
	if id.IsSuperAdmin() {
		if requested == nil {
			return uuid.Nil, huma.Error400BadRequest("org_id is required for super-admin")
		}
		return *requested, nil
	}
	if id.OrgID == nil {
		return uuid.Nil, huma.Error403Forbidden("no organization on identity")
	}
	if requested != nil && *requested != *id.OrgID {
		return uuid.Nil, huma.Error403Forbidden("cannot act outside your organization")
	}
	return *id.OrgID, nil
}

