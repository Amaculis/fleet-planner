# Bus Fleet & Trip Planning

Internal system for a passenger bus company: plan trips, assign a bus and a driver to
each, and see the day on a timeline. Roles: **admin**, **dispatcher**, **driver**.
It holds drivers' personal data, so GDPR applies (erasure by anonymisation, EU-only
hosting, encrypted backups). Worked time is recorded per trip so payroll can be added
later without a schema change.

## Stack

| Layer | Choice |
|---|---|
| API | Go, `chi`, layered `handler → service → repository`; DB access only through `sqlc` |
| Database | PostgreSQL 17 via `pgx`; migrations with `golang-migrate` |
| Portal (main UI) | Vue 3 + [`@dativa-lv/lx-ui`](https://github.com/dativa-lv/lx-ui), served under `/app/`, installable as a PWA |
| Server-rendered UI | `templ` + `htmx` (older, smaller UI; see [docs/lx-ui-integration.md](docs/lx-ui-integration.md)) |
| Edge | Caddy (automatic TLS, HSTS) |
| Tests | Go unit + integration (real Postgres), Vitest, Playwright e2e |

Key design decisions:

- **Double booking is impossible by construction.** A bus and a driver can't be on two
  trips whose time ranges overlap: a Postgres `EXCLUDE USING gist` constraint enforces
  it, and the service layer re-checks it to return a friendly error.
- **Secure by default.** Opaque server-side sessions in `__Host-` cookies, CSRF tokens on
  every state-changing request, argon2id password hashing, per-IP and per-account login
  rate limits, strict CSP with per-request nonce, role checks on the server for every route.
- **Every mutation is audited** (who, what, when, before, after). The audit log is
  append-only for the app's database role and is purged after two years.
- **Least privilege.** The app connects as `fleet_app` (no DDL); migrations run as a
  separate `fleet_owner`.

## Quick start (Docker)

```sh
cp .env.example .env
# Fill in every empty value; generate secrets with: openssl rand -hex 32
docker compose up --build
bash scripts/create-admin.sh     # dev only: creates the first admin from .env
```

Open `https://$APP_DOMAIN/app/`. The stack refuses to start while a secret is empty,
and the app rejects a placeholder `APP_SECRET`.

## Development

```sh
make test          # unit + integration tests against a throwaway Postgres (needs Docker)
make test-unit     # unit tests only; the database-backed tests are SKIPPED
make vet fmt-check # static checks
make vuln          # govulncheck
make generate      # sqlc + templ (sqlc runs from its Docker image)
make hooks         # enable git hooks: gofmt, vet, gosec before commit/push
make portal-test   # portal unit tests
make verify-e2e    # Playwright against the running stack (docker compose up -d first)
```

Portal development: `cd portal && npm ci && npm run dev` (proxies `/api` to
`localhost:8080`). Lint with `npm run lint`. The package manager is npm, deliberately.

## Layout

```
cmd/server           the binary: HTTP server plus the create-admin subcommand
internal/domain      entities, roles, statuses, transition rules (no I/O)
internal/service     business logic and validation, audit recording
internal/repo        sqlc-generated queries + transaction helper
internal/httpapi     routes, middleware, handlers (server-rendered and JSON API)
internal/auth        sessions, password hashing, CSRF
internal/inttest     integration tests against real Postgres
db/migrations        paired up/down migrations; db/queries holds the sqlc SQL
portal/              Vue 3 + lx-ui single-page app (main UI) and its Playwright e2e
web/                 server-rendered templates and static assets
deploy/              Postgres init (roles) and the encrypted-backup service
```

## Operations

- Backups: `pg_dump`, encrypted with `age`, restore verified by `scripts/verify-backup.sh`.
  See [deploy/backup/README.md](deploy/backup/README.md). The off-site destination is
  not configured yet.
- Project rules, security baseline and the definition of done live in
  [CLAUDE.md](CLAUDE.md).
