# Multica API Docs (unofficial, hand-derived)

Multica ships no official API spec. This folder is our own contract snapshot,
built for external integrations (e.g. tracking / monitoring systems) that
need to notice upstream drift.

| File | What it is |
| --- | --- |
| `openapi.yaml` | OpenAPI 3.0 spec, hand-derived from source at commit `5ed381a9d` (2026-07-02) |
| `routes.json` | Auto-generated route inventory (`METHOD /path`, sorted) — the diffable surface |
| `index.html` | Static rendered docs (Redoc), generated from `openapi.yaml` |

---

## 1. Build and view the API docs

```bash
# Build the static HTML (self-contained, no server needed)
npx @redocly/cli build-docs api-docs/openapi.yaml -o api-docs/index.html

# Open it
open api-docs/index.html
```

Alternatives:

```bash
# Live-reload preview while editing openapi.yaml
npx @redocly/cli preview-docs api-docs/openapi.yaml

# Swagger UI (has try-it-out: fire real requests with your PAT)
docker run -p 8081:8080 -e SWAGGER_JSON=/spec/openapi.yaml \
  -v $PWD/api-docs:/spec swaggerapi/swagger-ui
```

`routes.json` needs no viewer — it is a plain sorted list; read or diff it
directly.

---

## 2. Regenerate when upstream updates

`openapi.yaml` is a snapshot; it does not update itself. After every upstream
pull:

```bash
# Step 1 — endpoint changes (exact adds/removes/renames)
make routedoc && git diff api-docs/routes.json

# Step 2 — request/response shape changes
git diff <last-synced-commit>..HEAD -- \
  packages/core/api/schemas.ts \
  packages/core/api/client.ts
```

- Both diffs empty → docs still accurate, done.
- `routes.json` diff → add/remove the affected paths in `openapi.yaml`.
- `schemas.ts` / `client.ts` diff → update the affected schemas in
  `openapi.yaml`.
- Then bump `info.version` in `openapi.yaml` to the new commit hash, rebuild
  `index.html` (section 1), and commit all three files.

Sources of truth in the repo, in case you need to check a shape by hand:

| File | Defines |
| --- | --- |
| `server/cmd/server/router.go` | Every endpoint: path, method, auth middleware |
| `packages/core/api/client.ts` | Request shapes the official frontend sends |
| `packages/core/api/schemas.ts` | zod response schemas (lenient contract) |
| `packages/core/api/ws-client.ts` | WebSocket event contract (`/ws`) |
| `packages/core/types/*.ts` | TS response types |
| `apps/docs/content/docs/developers/auth-tokens.mdx` | PAT documentation |

---

## 3. How the docgen generator works

Generator: `server/cmd/server/routedoc_gen_test.go`.

How it works:

1. Builds the real production router (`NewRouter(...)`) with nil DB pool —
   construction only wires dependencies, it never touches the database.
2. Walks every registered route with `chi.Walk` (built into chi — no extra
   dependency, no handler annotations needed).
3. Deduplicates, sorts, and writes `METHOD /path` lines to
   `api-docs/routes.json`.

It lives as a Go test because `NewRouter` is in `package main` and cannot be
imported by a separate command. It is guarded by the `ROUTEDOC` env var, so a
plain `go test ./...` skips it and never rewrites the file.

Run it:

```bash
make routedoc
```

The make target boots the postgres container and applies migrations first —
not because the generator needs the DB, but because the test package's
`TestMain` (in `integration_test.go`) exits early when the DB is unreachable,
which would skip the generator too.

Manual invocation (postgres already running):

```bash
cd server && ROUTEDOC=1 go test ./cmd/server -run TestGenerateRouteDocs -v
```

Limitation: routes only. Request/response schemas cannot be auto-extracted —
handlers decode/encode JSON inline with no annotations for a tool to reflect
on. Schemas in `openapi.yaml` are maintained by hand from `schemas.ts`.

---

## Auth quickstart

```bash
# 1. Create a PAT in Settings > API tokens (or POST /api/tokens with a session)
# 2. Workspace-scoped calls need the workspace header:
curl -H "Authorization: Bearer mul_..." \
     -H "X-Workspace-ID: <workspace-uuid>" \
     https://<host>/api/issues
```

## Endpoints most useful for tracking / monitoring

- `GET /api/workspaces/{id}/members` — member roster (join `actor_id` against it)
- `GET /api/issues` / `GET /api/issues/grouped` — who is assigned what
- `GET /api/issues/{id}/timeline` — activity log + comments per issue; status
  and assignee changes carry `actor_type`/`actor_id`/`created_at`, the raw
  material for time-in-status per person
- `GET /api/issues/{id}/task-runs` + `GET /api/issues/{id}/usage` — agent time
  and token cost per issue
- `GET /api/dashboard/agent-runtime`, `/api/dashboard/runtime/daily`,
  `/api/dashboard/usage/*` — workspace rollups (agents only)
- `GET /api/squads/{id}/members/status` — live presence + `last_active_at`
- `GET /ws` — realtime event push instead of polling

Note: Multica does NOT track human time natively. Human time-on-task must be
derived from timeline status transitions.

## Coverage policy

Every route in `router.go` is listed in `openapi.yaml` (diffable surface).
Full schemas are modeled only for tracking-relevant endpoints; the rest
return `GenericObject` — consult `schemas.ts`/`client.ts` when you need their
exact shape. Server enums are documented as open strings because upstream
adds values without notice (their own frontend parses leniently on purpose).
