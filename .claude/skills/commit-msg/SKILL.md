---
description: Draft compact and verbose commit message options for staged or branch changes. Follows XYZMed conventional-commits style.
context: fork
---

Draft two commit message options for the changes below. Do not create the commit.

Staged changes:
```
!`git diff --cached`
```

If nothing is staged, use the branch diff instead:
```
!`git diff master...HEAD`
```

## Rules

- **Conventional commits** type prefix: `feat`, `fix`, `chore`, `refactor`, `docs`, `test`, `perf`, `ci`.
- Scope in parentheses reflects the affected subsystem: `ehr`, `auth`, `rbac`, `billing`, `api`, `frontend`, `db`, `deploy`, etc.
- Subject line: imperative mood, lowercase after the colon, no trailing period, 72 chars max.
- No em dashes anywhere. Use a colon, comma, or parentheses instead.
- No `Co-Authored-By` trailer.

## Output

**Compact:**
```
type(scope): subject line only
```

**Verbose:**
```
type(scope): subject line

Body: 2-4 sentences explaining *why* the change was made and any
non-obvious details. Wrap at 72 chars.
```

Output only the two code blocks above, nothing else.
