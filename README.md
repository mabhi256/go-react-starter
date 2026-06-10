# go-react-starter

A forkable monorepo starter for Go + React SaaS projects.

**Stack:** Go (Echo + Huma), PostgreSQL, Valkey/Redis, asynq, OpenTelemetry, React (TanStack Start + Query + Zustand + Tailwind v4 + shadcn/ui), docker-compose.

**Includes:** JWT auth + refresh tokens, multi-tenant org isolation, RBAC (super_admin/admin/user), append-only audit trail, background tasks with OTel trace propagation, a demo `items` CRUD API + frontend page.

## Quick start

```bash
git clone https://github.com/your-org/go-react-starter
cd go-react-starter
cp .env.example .env
docker compose -f docker-compose.yml -f docker-compose.dev.yml up --build
```

API: http://localhost:8080 | Scalar docs: http://localhost:8080/docs | Grafana: http://localhost:3001

## Fork checklist

When using this as a starting point for a real project:

1. **Rename the Go module:**
   ```bash
   cd backend
   go mod edit -module github.com/your-org/your-project/backend
   find . -name "*.go" | xargs sed -i 's|github.com/your-org/go-react-starter/backend|github.com/your-org/your-project/backend|g'
   ```
2. **Replace `internal/items/`** with your actual domain package(s).
3. **Add your domain migration** in `backend/db/migrations/`.
4. **Wire your handler** in `backend/cmd/api/main.go`.
5. **Fill in `PRODUCT.md`** with your product description.
6. **Install code-review-graph** (one-time per clone):
   ```bash
   pip install code-review-graph
   code-review-graph install
   ```

## Running tests

```bash
cd backend
go test ./tests/...    # integration (requires Docker)
go test ./internal/... # unit
```

## Project layout

```
backend/   Go service (API + worker + migrate)
frontend/  React app
deploy/    AWS prod notes
docs/      Architecture map + spec workflow
```

See `backend/CLAUDE.md` and `frontend/CLAUDE.md` for development guides.
