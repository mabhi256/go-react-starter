# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## [Project Name]: Engineering Guide

[One paragraph: what this project does, who it is for.]

Monorepo: Go backend (Echo + Huma), React frontend (TanStack Start), docker-compose for local dev.

## Writing style

Never use em dashes (-- or the long dash character) in documentation, comments, or text output. Use a colon, semicolon, comma, or parentheses instead.

## Working agreement

Follow the **`andrej-karpathy-skills:karpathy-guidelines`** skill on every change:
think before coding, simplest thing that works, surgical edits, define a verifiable success
criterion and loop until it passes. Do not add speculative abstractions.

## Layout

- `backend/`: Go service. See `backend/CLAUDE.md`.
- `frontend/`: React app (TanStack Start). See `frontend/CLAUDE.md`.
- `deploy/`: prod AWS notes.
- `docs/`: architecture doc, spec workflow.

## Run it locally

```bash
cp .env.example .env
docker compose -f docker-compose.yml -f docker-compose.dev.yml up --build
```

Services: API (`:8080`, Scalar at `/docs` dev-only), worker, Postgres, Valkey, Mailhog (`:8025`),
Grafana LGTM (`:3001`).

## Dev vs prod

One `APP_ENV` (`dev` | `prod`) read via koanf. `dev` uses the compose stack; `prod` points
the same binaries at RDS / ElastiCache / SES. `config.IsProd()` gates Scalar docs.

## Cross-cutting architecture

### Auth

JWT access tokens (short-lived, HS256, carry `org_id` + `roles`) + opaque refresh tokens in
Redis (one-time use, rotated on refresh). Module: `internal/auth`.

### RBAC and tenancy

Three roles: `super_admin` (cross-org), `admin` (org-level admin), `user` (standard org user).
`rbac.Identity.EffectiveOrgFilter()` returns `nil` for super-admin or the caller's `org_id`
for everyone else. Every repo query must pass this filter.

### Audit trail

`apiutil.Audit(ctx, rec, action, resourceType, resourceID)` in every domain handler. Enqueues
an `audit:write` asynq task; the worker persists it. The DB trigger enforces append-only.

### Background tasks (asynq)

Worker: `cmd/worker`. Naming: `domain:action` (e.g. `audit:write`, `notify:email`). Payloads
wrapped with OTel trace context via `platform.WrapTaskPayload` / `platform.UnwrapTaskPayload`.

### IDs

All primary keys are UUID v7 (time-ordered). Use `uuid.NewV7()`.

---

## Skill Guide

### context7 (library docs): ALWAYS use before implementing against a library

Before using any library, framework, or SDK, call:
1. `mcp__plugin_context7_context7__resolve-library-id` to find the library.
2. `mcp__plugin_context7_context7__query-docs` to fetch current docs.

Do not rely on training-data memory for library APIs.

### code-review-graph (code navigation): install once per clone

```bash
pip install code-review-graph
code-review-graph install
```

This adds ~30 MCP tools to Claude Code. Rules:
- Before reading large codebases, use `query_graph_tool` or `semantic_search_nodes_tool` to locate relevant nodes. Only open source files when the graph query identifies something worth inspecting.
- Before any review, run `detect_changes_tool` to get the blast radius of changed files.
- Keep `docs/ARCHITECTURE.md` updated when adding new packages or changing boundaries.

### frontend-design + impeccable (UI workflow)

1. Use the `frontend-design` skill when designing a new screen or component from scratch.
2. After implementation, use the `impeccable` skill to critique and refine.

### simplify (code cleanup)

After completing a feature, run `/simplify` on changed files before marking work done.

### code-review (pre-merge review)

Run `/code-review` before any merge. Use `--fix` to apply findings.

### superpowers (use sparingly)

Invoke `superpowers:*` skills ONLY when:
- The user explicitly asks (e.g. "use brainstorming", "write a plan", "debug this systematically").
- The task involves new infrastructure, a multi-system integration, or an architectural decision.

Do NOT auto-trigger brainstorming, writing-plans, systematic-debugging, or TDD on routine feature work.

---

## Feature Development Gate (HARD RULE)

Before implementing any new feature or slice, Claude must:

1. Check `docs/specs/{feature-slug}/` for all four documents:
   - `prd.md`: product requirements and user stories
   - `design.md`: UX + data model design spec
   - `tsd.md`: high-level technical spec (architecture, APIs, decisions)
   - `impl.md`: low-level spec (exact files, functions, migration SQL)

2. If ANY are missing: stop, list which are absent, and offer to create them collaboratively before writing any code.

3. Proceed only when all four docs exist.

**Exceptions** (no spec docs needed): bugfixes, chores, dependency upgrades, CI/infra-only changes.

**Example path:** `docs/specs/billing-subscription/prd.md`

---

## Commit messages

After completing work that is ready to commit, suggest two options:

- **Compact**: single `type(scope): subject` line (<=72 chars), conventional commits style.
- **Verbose**: compact line + 2-4 sentences explaining the *why* and non-obvious details.

Do not create the commit automatically; suggest the messages and let the user decide.
