# Architecture

Living code map. Update this file when adding new packages or changing package boundaries.

## Backend packages

| Package | Path | Responsibility |
|---------|------|----------------|
| config | `internal/config/` | Load + validate all config via koanf; exposes `Config` struct |
| platform | `internal/platform/` | pgxpool, Redis, OTel, logger, mailer, SMS, asynq helpers |
| server | `internal/server/` | Echo + Huma bootstrap, auth middleware, CORS |
| rbac | `internal/rbac/` | `Identity` struct, role constants, context helpers |
| reqctx | `internal/reqctx/` | Per-request metadata (IP, user-agent) on context |
| apiutil | `internal/apiutil/` | RBAC guards, org resolver, audit emitter; shared by all handlers |
| audit | `internal/audit/` | Append-only `audit_logs`; Repo + AsynqRecorder + worker handler |
| auth | `internal/auth/` | Login, JWT issue/verify, OTP, refresh token rotation |
| org | `internal/org/` | Organizations (tenant boundaries); super-admin CRUD |
| user | `internal/user/` | Users, roles, auth identities; org-admin CRUD |
| items | `internal/items/` | Demo domain: generic CRUD resource, org-scoped |
| notify | `internal/notify/` | Email + SMS via asynq tasks |
| queueadmin | `internal/queueadmin/` | Asynq queue inspection and DLQ management (super-admin) |

## Frontend routes

| Route | File | Purpose |
|-------|------|---------|
| `/` | `src/routes/index.tsx` | Landing / redirect |
| `/items` | `src/routes/items.tsx` | Demo: live CRUD against items API |

## Key data flows

**Auth:** `POST /auth/login` returns JWT + refresh token. Client stores tokens; Axios interceptor injects `Authorization: Bearer` on every request.

**Tenant isolation:** Every handler calls `apiutil.ResolveOrg` to get `org_id`, passes it to the repo, which applies `WHERE org_id=$1` (or skips the filter for super-admin).

**Background tasks:** Handler calls `audit.NewAsynqRecorder.Record`, which enqueues an `audit:write` task. Worker processes it and calls `audit.Repo.Insert` for an append-only row.
