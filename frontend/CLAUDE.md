# Frontend: Engineering Guide

React app: TanStack Router + TanStack Query + Zustand + Tailwind v4 + shadcn/ui.

Follow `andrej-karpathy-skills:karpathy-guidelines`: surgical edits, no speculative features.

## Key files

| File | Purpose |
|------|---------|
| `src/styles.css` | Design tokens, Tailwind v4 theme, base layer |
| `src/lib/utils.ts` | `cn()` helper (clsx + tailwind-merge) |
| `src/lib/api.ts` | Axios instance; base URL from `VITE_API_URL`; auth header interceptor |
| `src/lib/store.ts` | Zustand auth store (access/refresh token persistence) |
| `src/lib/query.ts` | TanStack Query client with shared defaults |
| `src/routes/__root.tsx` | Root layout: wraps with QueryClientProvider |
| `src/routes/items.tsx` | Demo page: live CRUD against the items API |

## UI workflow

1. Use the `frontend-design` skill when designing a new screen or component.
2. After implementing, use `impeccable` to critique and refine.
3. Before reading large UI code, use `code-review-graph` MCP tools to locate relevant nodes.

## Design system

- **Primary colour:** teal (oklch 0.48 0.14 195 light / 0.60 dark)
- **Fonts:** Inter (body) + Plus Jakarta Sans (headings)
- **Components:** shadcn/ui (Radix-based). Add via `bunx shadcn add <name>`.
- **Dark mode:** toggle `.dark` class on a wrapper element.

## Adding a new route

```bash
touch src/routes/my-feature.tsx
bun run dev   # TanStack Router auto-generates routeTree.gen.ts on change
```

## Dev

```bash
bun install
VITE_API_URL=http://localhost:8080 bun run dev
# Open http://localhost:3000/items
```

## Prod (AWS Amplify)

Set `VITE_API_URL` to the ALB URL in Amplify env vars. Build: `bun run build`.
