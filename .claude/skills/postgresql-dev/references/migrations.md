# Migrations

Two distinct file kinds — **versioned** and **repeatable** — sit side by side and are applied by the same migration runner against `public.changelog`. Pick the right one when you ship a change; mixing them up causes either checksum conflicts (versioned edited after merge) or silent no-ops (a one-shot data backfill written as a repeatable that the runner skips).

## Versioned migrations — `V…__…__…sql`

- **Filename:** `V<X>_<Y>_<Z>_<YYYYMMDDHHmmSS>__<short_description>.sql`
  - `<X>_<Y>_<Z>` matches the parent folder `versions/<X>.<Y>.<Z>/`.
  - `<YYYYMMDDHHmmSS>` is a UTC timestamp (e.g. `20230413135700`).
  - `<short_description>` is lowercase English with `_` between words.
- **Run once.** The runner records the file's SHA checksum in `public.changelog` (date, time, checksum) the first time it applies. If the file changes after that, the next run **fails** with a checksum mismatch — by design, to stop drift.
- **Use for:** schema changes (`CREATE TABLE`, `ALTER TABLE`), constraint additions, classifier-data seeds, one-shot backfills, role creation.
- **One file per logical change.** If a release needs four DDL changes, that's four versioned files in the same `versions/<X.Y.Z>/` folder, each with a distinct timestamp.
- **Never two scripts within the same second.** Bump the timestamp by `+1s` if you create two files in quick succession.

## Repeatable migrations — `R__….sql`

- **Filename:** `R__<object_name>.sql`. One file per DB object (function, procedure, view, type, trigger).
- **Body starts with `create or replace`.** The runner re-applies the file whenever its checksum changes, and `create or replace` is what makes that safe.
- **Alphabetical execution order.** A `R__a_…` file runs before `R__z_…`. Use this to force ordering — privilege grants that need every schema to exist conventionally live in `R__x_grant_privileges_user_roles.sql` and `R__z_util_grants.sql`.
- **Use for:** stored functions, procedures, views, types, triggers, grants that should re-assert themselves every deploy.
- **Do not use for:** anything that is not idempotent. A repeatable that runs `INSERT INTO classifier VALUES (…)` will produce duplicates on the second deploy.

## Why role creation is versioned, not repeatable

A role is created exactly once per cluster — it lives outside the database, not inside it — so `\gexec`-based idempotence is the only safe pattern, and that pattern only makes sense as a one-shot. Pattern:

```sql
do
$$
begin
  if not exists (SELECT * FROM pg_roles where rolname = 'user_write_role') then
     CREATE ROLE user_write_role NOINHERIT;
  end if;
end
$$;
```

Discussion:

- `NOINHERIT` on a privilege role forces members to `SET ROLE` explicitly before they can use its privileges. The opposite default (`INHERIT`) silently merges every granted role's privileges into the session — handy but blurs least-privilege boundaries.
- The `pg_roles` guard makes the file safe to re-run during a fresh DB rebuild. A bare `CREATE ROLE` would error on the second run because the migration runner reports any non-zero result as a failure.
- Even though the body is conditional, the file is **versioned**. The role only needs to be created once per cluster; a repeatable would also work but would re-check `pg_roles` on every deploy for no gain.

The companion pattern with login attributes is similar:

```sql
do $$ begin
  if not exists (select from pg_roles where rolname = 'template') then
    create role template with login nosuperuser inherit nocreatedb nocreaterole noreplication password 'test';
  end if;
end $$;
```

Discussion: this is for an *application login role* — it has `login` and `inherit`, and is allowed to receive memberships in the `NOINHERIT` privilege roles above. Each capability flag is opt-in: no superuser, no DB creation, no role creation, no replication. The `'test'` password is the default for the throwaway dev cluster only; production secrets come from the orchestrator.

## Idempotent tablespace creation with `\gexec`

Tablespaces, like roles, are cluster-global. The `\gexec` pattern is the cleanest psql-only idiom for conditional DDL on cluster objects:

```sql
select 'create tablespace template_index owner template location ''/data/index'''
where not exists (select from pg_tablespace where spcname = 'template_index')\gexec
```

Discussion:

- `\gexec` is a psql meta-command (`\` commands are not part of SQL). It takes each row of the previous query result and runs it as SQL. When the `where not exists (...)` clause is satisfied the outer `select` returns zero rows, so `\gexec` runs nothing.
- This is not portable. Drivers that speak the wire protocol directly (libpq, psycopg, pgx) cannot execute backslash commands. Conditional cluster-DDL through those clients has to be done with a `DO $$` block plus dynamic `EXECUTE`.
- The double-single-quote (`''`) is the SQL string-escape for the location path inside the quoted command.

## Workflow — adding a change

| Change type | File kind | Where it lives |
|---|---|---|
| New / changed function, procedure, view, type, trigger | repeatable | `code/<schema>/R__<object>.sql` starting with `create or replace` |
| New table, new column, new constraint, index creation, classifier-data seed, one-shot data backfill | versioned | `versions/<X.Y.Z>/V…__….sql` with the current UTC timestamp |
| Role / tablespace / extension setup | versioned (idempotent body) | `versions/<X.Y.Z>/V…__….sql` |
| Grant pattern that should re-assert on every deploy | repeatable | `code/R__<x|z>_<topic>_grants.sql` |
| Bug in a versioned file already merged & deployed | **new** versioned file | `versions/<same X.Y.Z>/V…__fix_….sql` — never edit the original |

Picking the version folder: stay within the active `<X.Y.Z>` until it's released; afterwards open a new folder and continue.

## Volatile defaults rewrite the table

When a versioned `ALTER TABLE` adds a `NOT NULL` column with a **volatile** default (`current_timestamp`, `gen_random_uuid()`, `nextval(...)`), PostgreSQL rewrites every existing row to fill it in. That holds an `ACCESS EXCLUSIVE` lock for the duration. Non-volatile defaults (constants, immutable expressions) take an instant metadata-only path.

Safe two-step for large tables:

```sql
-- step 1 (cheap, no rewrite): add the column with a non-volatile default or no default
alter table big_table add column created_at timestamptz;

-- step 2 (background backfill, then make it required)
update big_table set created_at = current_timestamp where created_at is null;  -- batch this if needed
alter table big_table alter column created_at set not null;
alter table big_table alter column created_at set default current_timestamp;   -- safe: only new rows
```

Each step is its own versioned migration with its own timestamp.

## `CREATE INDEX CONCURRENTLY` caveat

`CONCURRENTLY` cannot run inside an explicit transaction. If the migration runner wraps every script in `BEGIN…COMMIT` (most do), a script containing `CREATE INDEX CONCURRENTLY` fails with `CREATE INDEX CONCURRENTLY cannot run inside a transaction block`.

Options, in order of preference:

1. Use a plain `CREATE INDEX` if the table can tolerate the write lock for the build duration.
2. Apply the index out-of-band (manual `psql` session on a maintenance window) and record it as a placeholder no-op migration so the team knows where the index came from.
3. Configure the runner to skip transaction-wrapping for files matching a specific naming convention, if supported.

## Checksum-conflict recovery

If the runner refuses to start with `migration … has changed checksum`:

- **Production:** never resolve by editing the file. Roll forward with a new versioned file whose body reverses or extends the original, and accept that the prior file's logical effect is now "the original + the correction".
- **Dev only:** if the file is unreleased and you are sure nobody else has applied it, you can wipe the row from `public.changelog` and let the runner re-apply. Do **not** make this a habit; it trains the wrong reflex.

## Rollback = roll forward

There is no built-in `down` migration. To undo an applied change, write a new versioned file that issues the inverse statement (`DROP COLUMN`, `DROP TABLE`, an `UPDATE` that restores rows). This is intentional: in a multi-environment pipeline, "rollback" via destructive edits to history is far more dangerous than an explicit, auditable forward fix.

## Procedures are repeatables

Stored procedures that follow the JSON-in/JSON-out application API contract (`create or replace procedure …(in pi_data json, inout po_data json)`) are repeatable migrations like any other function or view: one file per procedure at `code/<schema>/R__<proc_name>.sql`, body starts with `create or replace`. The contract, error-handling patterns, and HTTP-status mapping are documented separately — see [procedures.md](./procedures.md).

## See also

- [project-layout.md](./project-layout.md) — immutability rules and the changelog table.
- [roles-and-grants.md](./roles-and-grants.md) — role/tablespace idempotent patterns.
- [deployment.md](./deployment.md) — how the runner is actually invoked and where `public.changelog` lives.
- [procedures.md](./procedures.md) — JSON procedure contract used as the application API.
