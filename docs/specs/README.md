# Feature Spec Workflow

Every new feature goes through four documents before any code is written. Copy `_template/` into `{feature-slug}/` and fill each file in order.

## Documents

| File | Purpose | Who writes it |
|------|---------|---------------|
| `prd.md` | Product requirements: problem, users, stories, success metrics, out-of-scope | PM / product owner |
| `design.md` | UX + data model: wireframes, flows, error states, data schema sketch | Designer / lead dev |
| `tsd.md` | High-level technical spec: architecture, API contracts, key decisions | Tech lead |
| `impl.md` | Low-level implementation spec: exact files, SQL, function signatures, test plan | Implementing dev |

## Folder naming

```
docs/specs/
  {feature-slug}/
    prd.md
    design.md
    tsd.md
    impl.md
```

Example: `docs/specs/billing-subscription/prd.md`

## Enforcement

Claude Code is configured to check for all four documents before implementing any feature.
If any are missing, it stops and offers to create them collaboratively.

Exceptions: bugfixes, chores, dependency upgrades, CI/infra-only changes.
