# CLAUDE.md — Bus Fleet & Trip Planning System

Project constitution. Read fully at the start of every session. When this file and a
request conflict, ask — do not silently override these rules.

## Product in one paragraph

Internal (no passenger-facing part) system for a passenger bus company to plan trips
and assign buses + drivers, with a timeline view. Roles: admin, dispatcher, driver.
Modern, secure-by-default, and designed so **payroll** can be added later without a
schema rewrite. Handles personal data of drivers → GDPR applies.

## Golden rules

1. **Security is not a feature to add later — it is the default.** Every endpoint is
   authenticated and authorized server-side before anything else runs.
2. **Never trust the client.** Validate and authorize on the server for every request,
   every field. The frontend hiding a button is not access control.
3. **No raw SQL string building, ever.** All queries go through `sqlc` (compile-checked,
   parameterized). This eliminates SQL injection by construction.
4. **Make illegal states unrepresentable.** Prefer DB constraints and Go types over
   runtime checks alone. Example: the no-double-booking rule is a Postgres `EXCLUDE`
   constraint, not just app logic.
5. **Small, reviewable changes.** Propose schema/architecture and stop for review before
   implementing. Do not generate the whole app in one shot.
6. **Every mutation is audited.** who / what / when / before / after.

## Architecture

- **Go API** (1.22+), `chi` router (or stdlib mux), layered:
  `handler → service → repository (sqlc)`. Handlers do HTTP + validation only; business
  logic lives in services; DB access only in repositories.
- **PostgreSQL** via `pgx`. Migrations via `golang-migrate` (see `db-migrations` skill).
- **Frontend:** server-rendered `templ` + `htmx` + Tailwind. Minimal client JS. Driver
  view is a PWA. (A small SPA is allowed only after explicit approval.)
- **Auth:** opaque server-side sessions, `HttpOnly; Secure; SameSite=Lax` cookies,
  session rotation on login, idle + absolute expiry, CSRF tokens on all state-changing
  requests. Passwords hashed with **argon2id**.
- **Reverse proxy:** Caddy (automatic TLS / Let's Encrypt, HSTS).
- **Config:** everything via environment variables. Secrets never committed — only
  `.env.example` with dummy values.

## Data model (core)

- `users(id, email, password_hash, role, driver_id?, is_active, created_at, updated_at)`
- `drivers(id, full_name, phone, license_number, license_expiry, hourly_rate?, pay_type?, is_active, ...)`
  — pay fields nullable + unused in MVP; present so payroll needs no migration later.
- `buses(id, plate, model, seats, status, insurance_expiry?, inspection_expiry?, ...)`
- `trips(id, origin, destination, scheduled_start, scheduled_end, actual_start?, actual_end?, status, notes?, ...)`
- `assignments(id, trip_id, bus_id, driver_id, created_at)`
- `audit_log(id, actor_user_id, action, entity, entity_id, before jsonb?, after jsonb?, ip?, created_at)`

**Invariants:**
- A bus and a driver may not each be on two assignments whose trips' `[scheduled_start,
  scheduled_end)` ranges overlap. Enforced by `EXCLUDE USING gist` (needs `btree_gist`),
  **and** re-checked in the service layer for a friendly error.
- `trips.status ∈ {planned, in_progress, completed, cancelled}`; `users.role ∈ {admin,
  dispatcher, driver}` — as Postgres enums or checked constraints, not free text.

**Payroll seam:** worked time = `actual_start`/`actual_end` per trip, tied to a driver via
`assignments`. Never delete this linkage. Comment it as `// payroll-seam` where relevant.

## Security baseline (must all be present before "MVP done")

- argon2id password hashing; constant-time comparison; no password in logs.
- Session-based auth as above; logout invalidates server-side session.
- RBAC middleware; a `driver` can reach only their own data (verify by `driver_id`, not by
  a client-supplied id).
- CSRF protection on every non-GET request.
- Server-side input validation on every field; `templ` auto-escaping for output.
- Rate limiting: strict on auth endpoints (brute-force), sane global default.
- Security headers: CSP, `X-Content-Type-Options: nosniff`, `Referrer-Policy`,
  `X-Frame-Options: DENY`, HSTS (via Caddy).
- Least-privilege Postgres role for the app (no superuser; only needed grants).
- `govulncheck` clean; dependencies pinned; no secrets in the repo.
- Structured logs with **no PII**; audit log separate from app logs.

## GDPR baseline

- Data minimization: store only what the system needs.
- Right to erasure: an admin endpoint to delete/anonymize a driver's personal data
  (respecting audit/retention needs — anonymize rather than break referential integrity).
- Data stays in the EU (VPS region + backup destination).
- Encrypted, off-site, **tested** DB backups (`pg_dump`, scheduled, restore verified).

## Testing (see `testing` conventions if present)

Minimum before merge:
- Conflict detection (overlap, adjacency edge `[start, end)`, cancelled trips excluded).
- Role boundaries: driver cannot read/write another driver's or admin's data.
- Auth: login, logout, session expiry, CSRF rejection, rate-limit trigger.

## Commands (fill in as they stabilize)

- Run: `docker compose up --build`
- Migrate: `migrate -path db/migrations -database "$DATABASE_URL" up`
- Generate queries: `sqlc generate`
- Test: `go test ./...`
- Vuln scan: `govulncheck ./...`

## Definition of done for any task

Compiles, tests pass, `govulncheck` clean, `security-review` checklist run, migration is
reversible, no secret or PII committed, audit log covers new mutations.
