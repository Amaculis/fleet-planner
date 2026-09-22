---
name: db-migrations
description: Conventions for PostgreSQL schema migrations (golang-migrate) in the bus fleet system. Use whenever creating, editing, or reviewing any schema change, or adding sqlc queries that depend on the schema.
---

# Database migrations

Schema is boring on purpose: versioned, reversible, forward-only in production, never
edited after it ships. The data is real (a running business) — a bad migration loses money.

## Rules

1. **Every migration is a pair:** `NNNN_description.up.sql` and `NNNN_description.down.sql`,
   zero-padded sequential number (`0001_init.up.sql`). Tool: `golang-migrate`.
2. **Never edit a migration that has already run** anywhere real. Fix forward with a new one.
3. **`down` must genuinely reverse `up`.** If a change is truly irreversible (e.g. a data
   drop), say so explicitly in a comment and make `down` restore the structure at least.
4. **One logical change per migration.** Don't mix an index add with a column rename.
5. **Additive-first for safety.** To change a column: add new → backfill → switch reads/writes
   → drop old, across separate migrations, so a rollback is always possible.
6. **Destructive steps are called out.** Any `DROP`/`DELETE`/`ALTER … TYPE` that can lose data
   gets a `-- DESTRUCTIVE:` comment and is never bundled with unrelated changes.
7. After any schema change, regenerate types: **`sqlc generate`**, and update affected queries.
8. Migrations run in CI against a throwaway DB (up then down then up) before merge.

## Project-specific must-haves

- Enable extensions early: `CREATE EXTENSION IF NOT EXISTS btree_gist;`
- Enforce the no-double-booking invariant **in the schema**, e.g.:

  ```sql
  -- assignments cannot overlap in time for the same bus (and same for driver)
  ALTER TABLE assignments
    ADD CONSTRAINT no_bus_overlap
    EXCLUDE USING gist (
      bus_id WITH =,
      tstzrange(scheduled_start, scheduled_end, '[)') WITH &&
    ) WHERE (status <> 'cancelled');
  ```

  (Denormalize the trip's time onto the assignment, or use a generated/joined approach —
  whichever keeps the constraint enforceable. Mirror it for `driver_id`.)
- Use enums or `CHECK` constraints for `role`, `trips.status` — never free-text status.
- `created_at`/`updated_at timestamptz` on every table; `updated_at` maintained by trigger
  or in the repository layer, consistently.
- Foreign keys with explicit `ON DELETE` behavior chosen deliberately (usually `RESTRICT`
  for business data; anonymize rather than cascade-delete drivers — GDPR + audit).
- Timestamps are `timestamptz`, stored/queried in UTC.

## Review checklist

- [ ] Up and down present; down tested (up → down → up is clean).
- [ ] No edit to an already-shipped migration.
- [ ] Single logical change.
- [ ] Destructive steps commented and isolated.
- [ ] Constraints/enums used to make illegal states impossible.
- [ ] `sqlc generate` re-run; queries compile.
- [ ] FKs have deliberate `ON DELETE`.
