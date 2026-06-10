# Implementation Spec: [Feature Name]

**Status:** Draft | Review | Approved
**Date:** YYYY-MM-DD

## Files to create or modify

| File | Change |
|------|--------|
| `backend/internal/foo/model.go` | Create |
| `backend/db/migrations/NNN_foo.sql` | Create |

## Migration SQL

```sql
-- up
CREATE TABLE ...;

---- create above / drop below ----

DROP TABLE ...;
```

## Function signatures

```go
func (r *Repo) Create(ctx context.Context, in NewFoo) (*Foo, error)
```

## Test plan

- [ ] Unit: [what to unit test]
- [ ] Integration: [what to integration test]
- [ ] Manual: [what to verify by hand]

## Rollout notes

[Feature flags, data backfill, migration safety notes]
