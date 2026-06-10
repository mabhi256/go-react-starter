# Backend: Engineering Guide

Go service: HTTP API (`cmd/api`) + background worker (`cmd/worker`) + migration runner (`cmd/migrate`).

Follow `andrej-karpathy-skills:karpathy-guidelines`: surgical edits, minimum code, verifiable success criterion first.

## Layout

```text
cmd/
  api/       HTTP server entrypoint (Echo + Huma)
  worker/    asynq worker entrypoint
  migrate/   tern migration runner
internal/
  config/    koanf config; APP_ENV=dev|prod
  platform/  pgxpool, redis, otel, logger, mailer, sms, asynq
  server/    Echo+Huma bootstrap, auth middleware, CORS
  rbac/      Identity struct, role constants (super_admin/admin/user), context helpers
  reqctx/    per-request Meta (IP, UA) on context
  apiutil/   shared handler helpers (identity, RBAC guards, org resolution, audit emit)
  audit/     append-only audit_logs (Repo + AsynqRecorder + worker handler)
  auth/      login flows, JWT tokens, OTP, handler
  org/       organizations (super-admin only), handler
  user/      users/roles/identities, handler (admin CRUD)
  items/     demo CRUD domain; replace with your actual domain when forking
db/
  migrations/ tern SQL files (NNN_name.sql)
  embed.go    embeds migrations into binary
```

## Domain layering

Each domain: `model.go` + `repo.go` + `handler.go` (no separate service layer for thin CRUD).

**Invariants every repo must enforce:**
- Filter by `org_id` unless caller is `super_admin` (use `EffectiveOrgFilter()`).
- All deletes are soft (`deleted_at = now()`); never hard-delete domain rows.
- `audit_logs` is append-only (enforced by DB trigger).

## Adding a new Huma endpoint

1. Define typed `Input` + `Output` structs.
2. Call `huma.Register(api, huma.Operation{...Security: bearer}, handler)`.
3. Check authorization: call `apiutil.Require*` helpers; return the error directly.
4. Call `apiutil.Audit(ctx, rec, action, type, id)` for domain read/writes.

## Running locally

```bash
cp ../.env.example ../.env
docker compose -f ../docker-compose.yml -f ../docker-compose.dev.yml up -d postgres valkey mailhog lgtm
go run ./cmd/migrate
go run ./cmd/api
go run ./cmd/worker
```

## Testing

```bash
go test ./tests/...   # integration tests (requires Docker)
go test ./internal/... # unit tests
```

## Config keys

All keys in `.env.example`. Groups: `APP_ENV`, `HTTP_*`, `DB_*`, `REDIS_*`, `JWT_*`, `GOOGLE_OAUTH_*`, `MAIL_*`, `SMS_*`, `OTEL_*`.

## Migrations

SQL files in `db/migrations/`. Name: `NNN_description.sql`. Up/down separator: `---- create above / drop below ----`. Run: `go run ./cmd/migrate`.

## Forking this starter

When starting a new project:
1. Replace `internal/items/` with your domain package(s).
2. Add the corresponding migration in `db/migrations/`.
3. Register the handler in `cmd/api/main.go`.
4. Update `go.mod` module path: `go mod edit -module github.com/your-org/your-project/backend`.
5. Run the bulk rename: `find . -name "*.go" | xargs sed -i 's|github.com/your-org/go-react-starter/backend|github.com/your-org/your-project/backend|g'`.
