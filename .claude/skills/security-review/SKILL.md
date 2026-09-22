---
name: security-review
description: Security audit checklist for the Go + Postgres + templ/htmx bus fleet system. Run before merging any change that touches auth, data access, HTTP handlers, DB queries, config, or deploy. Also run before declaring MVP done.
---

# Security review

Run this as an adversarial pass over the diff. Assume the reviewer is trying to break in.
Report findings as **BLOCKER / WARN / NOTE**. Do not approve with an open BLOCKER.

## When to run

Any change touching: authentication, sessions, authorization/RBAC, HTTP handlers, SQL/
queries, file uploads, config/secrets, logging, dependencies, or the Docker/Caddy deploy.

## Checklist

### AuthN / sessions
- [ ] Passwords hashed with **argon2id** (sane params), never MD5/SHA/plaintext.
- [ ] Password comparison is constant-time; failures don't reveal whether the email exists.
- [ ] Sessions are **server-side opaque tokens**, not JWT-in-localStorage.
- [ ] Session cookie is `HttpOnly; Secure; SameSite=Lax` (or Strict).
- [ ] Session ID rotates on login; logout invalidates server-side; idle + absolute expiry set.
- [ ] Auth endpoints are rate-limited (brute-force / credential stuffing).

### AuthZ / RBAC
- [ ] **Every** non-public handler checks role server-side, before any work.
- [ ] `driver` role can access only its **own** records — resolved from the session's
      `driver_id`, never from a client-supplied id/param.
- [ ] No IDOR: object ownership is verified, not assumed from a URL/param.
- [ ] No capability hidden only in the UI and left open on the API.

### Injection / data access
- [ ] All SQL goes through `sqlc` / parameterized queries. **Zero** string-concatenated SQL.
- [ ] No dynamic table/column names from user input.
- [ ] Output is auto-escaped by `templ`; no `templ.Raw`/`dangerouslySet…` with user data.
- [ ] `htmx` responses don't reflect unescaped user input.

### Input validation
- [ ] Every field validated server-side (type, length, range, allowed set).
- [ ] Time ranges validated (`end > start`); trip/assignment invariants enforced.
- [ ] Request body size limited; content-type checked.

### CSRF / headers / transport
- [ ] CSRF token required and verified on every state-changing (non-GET) request.
- [ ] Security headers present: CSP, `X-Content-Type-Options: nosniff`,
      `X-Frame-Options: DENY`, `Referrer-Policy`, HSTS (Caddy).
- [ ] HTTPS enforced; no mixed content; secure cookies only over TLS.

### Secrets / config / logging
- [ ] No secret, key, password, or connection string committed. `.env` git-ignored;
      only `.env.example` with dummies.
- [ ] Config from env; safe failure if a required secret is missing.
- [ ] Logs contain **no PII and no secrets**; errors to users are generic, details logged.
- [ ] Audit log records the mutation (actor, action, entity, before/after) separately.

### DB / infra
- [ ] App connects as a **least-privilege** Postgres role (no superuser).
- [ ] `btree_gist` + `EXCLUDE` constraint present so double-booking is impossible at DB level.
- [ ] Migrations reversible; no destructive change without an explicit down + note.
- [ ] Backups scheduled, encrypted, off-site, and a **restore has been tested**.

### Dependencies
- [ ] `govulncheck ./...` clean.
- [ ] New dependencies are reputable, pinned, and actually needed.

### GDPR
- [ ] Only necessary personal data stored (minimization).
- [ ] Erasure/anonymization path exists for a driver's personal data.
- [ ] Data + backups stay in the EU.

## Output format

```
SECURITY REVIEW — <area>
BLOCKERS: <n>
- [BLOCKER] <finding> — <file:line> — <fix>
- [WARN]    <finding> — <file:line> — <fix>
- [NOTE]    <finding>
Verdict: APPROVE / CHANGES REQUIRED
```
