# Roles, grants, and tablespaces

A PostgreSQL project that follows these conventions runs with **two layers of users**: per-connection *login roles* that authenticate, and non-login *privilege roles* that bundle CRUD permissions. Login roles get permissions only by being granted membership in the privilege roles. This keeps least-privilege enforceable session-by-session and makes auditing tractable — you can answer "who can write to this schema?" by listing members of the write role.

## Layer 1 — login roles

A typical project ships with one application login role plus optionally a secondary login role used for utility schemas or testing. They are created idempotently in the initialization SQL:

```sql
do $$ begin
  if not exists (select from pg_roles where rolname = 'template') then
    create role template with login nosuperuser inherit nocreatedb nocreaterole noreplication password 'test';
  end if;
end $$;

do $$ begin
  if not exists (select from pg_roles where rolname = 'lx') then
    create role lx with login nosuperuser inherit nocreatedb nocreaterole noreplication password 'test';
  end if;
end $$;
```

Discussion of every modifier:

- `login` — the role can authenticate. Without it, the role is a "group role" and `SET ROLE` is the only way in.
- `nosuperuser` — explicit. Application roles should *never* be superuser; that bypasses RLS, every grant, and most safety checks.
- `inherit` — when this login role is granted a non-login privilege role, the privileges become active automatically for the session, no `SET ROLE` needed. (On the privilege roles themselves we use `NOINHERIT` — see below — to force explicit role activation when one privilege role is granted into another.)
- `nocreatedb` / `nocreaterole` — explicit. Application roles do not bootstrap databases or other roles.
- `noreplication` — explicit. Replication slots are an operational concern, not an application one.
- `password 'test'` — only acceptable in the throwaway dev cluster. In any other environment the password comes from a secret store, never from a checked-in SQL file. Use `ALTER ROLE … PASSWORD …` post-creation if needed.

The `if not exists` guard is what makes the file safe to re-run during a `docker compose up --build` rebuild against an empty cluster.

## Layer 2 — privilege roles (`user_read_role`, `user_write_role`)

Non-login roles that carry the CRUD permissions. Created in a versioned migration so it happens exactly once per cluster:

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

`NOINHERIT` here matters for a specific reason: if `user_write_role` is granted to *another* privilege role (rare but possible during reorganisations), members of that other role do **not** silently inherit write privileges. They have to `SET ROLE user_write_role` to use them. This is the explicit least-privilege boundary. Login roles set `INHERIT` so that the *normal* path — login role granted into `user_read_role` — does not need a per-statement `SET ROLE`.

## Schema-discovery grant pattern (repeatable)

Grants are a *repeatable* migration so they re-assert themselves on every deploy. The schema-discovery pattern walks `information_schema.schemata`, skips system schemas, and applies the same grant template to each:

```sql
-- Grant privileges for user_read_role;
DO $$
    DECLARE
        myschema RECORD;
    BEGIN
GRANT CONNECT ON DATABASE template TO user_read_role;
        FOR myschema IN (SELECT schema_name FROM information_schema.schemata WHERE schema_name NOT LIKE 'pg_%' AND schema_name <> 'information_schema')
        LOOP
            EXECUTE format ('GRANT USAGE ON SCHEMA %I TO user_read_role', myschema.schema_name);
            EXECUTE format ('GRANT SELECT ON ALL TABLES IN SCHEMA %I TO user_read_role', myschema.schema_name);
        END LOOP;
    END;
    $$ LANGUAGE plpgsql;

-- Grant privileges for user_write_role;
DO $$
    DECLARE
        myschema RECORD;
    BEGIN
GRANT CONNECT ON DATABASE template TO user_write_role;
        FOR myschema IN (SELECT schema_name FROM information_schema.schemata WHERE schema_name NOT LIKE 'pg_%' AND schema_name <> 'information_schema' and schema_name NOT LIKE 'public')
        LOOP
            EXECUTE format ('GRANT USAGE ON SCHEMA %I TO user_write_role', myschema.schema_name);
            EXECUTE format ('GRANT USAGE ON SCHEMA public TO user_write_role', myschema.schema_name);
            EXECUTE format ('GRANT INSERT, UPDATE, DELETE, SELECT ON ALL TABLES IN SCHEMA %I TO user_write_role', myschema.schema_name);
            EXECUTE format ('GRANT INSERT, UPDATE, DELETE, SELECT ON ALL TABLES IN SCHEMA public TO user_write_role', myschema.schema_name);
            EXECUTE format ('GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA %I TO user_write_role', myschema.schema_name);
            EXECUTE format ('GRANT ALL ON ALL FUNCTIONS IN SCHEMA %I TO user_write_role', myschema.schema_name);
            EXECUTE format ('GRANT ALL ON ALL PROCEDURES IN SCHEMA  %I TO user_write_role', myschema.schema_name);
        END LOOP;
    END;
    $$ LANGUAGE plpgsql;
```

Discussion:

- **Why repeatable:** when a future migration adds a new schema, this file's checksum doesn't change — but on the next deploy the runner detects that the file's dynamic `for` loop should be re-evaluated against the new schema list. So the canonical pattern is to keep this in `code/` and let the runner re-apply it any time the schema set drifts. (If you want bulletproof "added in the same release" grants, also add explicit `GRANT … ON SCHEMA new_schema` in the same versioned migration that creates the schema.)
- **Why `%I`:** `format('… %I …', identifier)` quotes the identifier safely. `%s` would inject the raw text and is exploitable if a schema name ever contained a quote.
- **System schema exclusion:** `WHERE schema_name NOT LIKE 'pg_%' AND schema_name <> 'information_schema'` — handles both real schemas (`pg_catalog`, `pg_toast`) and the SQL-standard view layer.
- **Public is special for write:** the write loop excludes `public` from the discovery filter (`schema_name NOT LIKE 'public'`) then explicitly grants on it inside the loop. This is a workaround for projects where `public` is intentionally separated from project schemas but writes still need to land there.
- **What `ALL TABLES IN SCHEMA … TO role` does *not* cover:** newly-created tables added after this grant runs. The grant only applies to tables that exist when the loop iterates. For tables created mid-cycle, either:
  - rely on the next deploy re-running this file, or
  - add an `ALTER DEFAULT PRIVILEGES IN SCHEMA <s> GRANT … TO <role>;` somewhere once, which makes future tables inherit the grant automatically.

## Per-function explicit grants

For utility schemas, the convention is per-function grants rather than `GRANT EXECUTE ON ALL FUNCTIONS`:

```sql
grant usage on schema util to template;

grant execute on function util.to_date (character varying) to template;
```

Discussion:

- `grant usage on schema` is necessary even if every function inside is individually granted — without `USAGE` on the containing schema, the user cannot resolve the qualified name to invoke a function.
- Per-function granting (with the explicit signature `(character varying)`) prevents accidentally exposing future helpers that ship in the same utility schema. It does mean each new utility function needs its own `grant execute` line — that's intentional friction.
- A file like this is repeatable (`R__z_util_grants.sql`) and is forced to run last via the `z` prefix so that all schemas and all functions exist by the time the grants are applied.

## Adding a new application user — checklist

1. **Document it.** Add a one-line entry under "Database users" in the project `README.md` so contributors know the role exists.
2. **Create idempotently.** Add a `do $$ … if not exists … create role … end $$;` block to `testing/init.sql` mirroring the patterns above.
3. **Grant memberships.** In a versioned migration: `GRANT user_read_role TO <new_user>;` (and/or `user_write_role`).
4. **Utility access.** If the user needs utility-schema functions, add a dedicated `code/R__z_<user>_grants.sql` so the runner re-asserts the grants every deploy and the `z` prefix forces it to run after schema creation.
5. **Production secret.** Provision a non-default password in the orchestrator / secret store, not in SQL.

## Tablespaces

Tablespaces are cluster-level physical locations for table and index files. A common layout reserves separate spaces for main data, indexes, archives, and logs — plus a secondary owner for a sibling application:

```sql
select 'create tablespace lx_index owner lx location ''/data/lx/index'''
where not exists (select from pg_tablespace where spcname = 'lx_index')\gexec

select 'create tablespace template_main owner template location ''/data/main'''
where not exists (select from pg_tablespace where spcname = 'template_main')\gexec

select 'create tablespace template_index owner template location ''/data/index'''
where not exists (select from pg_tablespace where spcname = 'template_index')\gexec

select 'create tablespace template_archive owner template location ''/data/archive'''
where not exists (select from pg_tablespace where spcname = 'template_archive')\gexec

select 'create tablespace template_log owner template location ''/data/log'''
where not exists (select from pg_tablespace where spcname = 'template_log')\gexec
```

Discussion:

- **`\gexec` is psql-only.** The migration runner connects through psql, which is why this idiom is used here rather than a `DO $$` block. See [migrations.md](./migrations.md#idempotent-tablespace-creation-with-gexec).
- **Owner matters.** A tablespace's owner can create new objects in it. Splitting `template_*` (owned by `template`) from `lx_index` (owned by `lx`) lets two co-existing applications manage their physical files independently without granting each other `CREATE` on the same space.
- **The filesystem paths must already exist** before this script runs. That's why the `Dockerfile.dev` (or its production equivalent) pre-creates `/data/main`, `/data/index`, `/data/archive`, `/data/log`, `/data/lx/index`, `/data/lx/main`, `/data/lx/archive` with `chown -R postgres:postgres /data`.
- **When the split actually buys you something:** on multi-spindle storage, separating index I/O from data I/O reduces contention and lets backups exclude index tablespaces (indexes can be rebuilt). On a single SSD it's mostly organisational and good for restore-time exclusion lists.

### Per-object placement

To put an index in a dedicated tablespace at creation time:

```sql
CREATE INDEX orders_user_idx ON orders (user_id) TABLESPACE template_index;
```

To move an existing index later:

```sql
ALTER INDEX orders_user_idx SET TABLESPACE template_index;
```

`ALTER INDEX … SET TABLESPACE` acquires an `ACCESS EXCLUSIVE` lock for the duration of the move — not appropriate during business hours on a busy table. For tables, the analogous statement is `ALTER TABLE … SET TABLESPACE …`, with the same locking caveat.

## `security definer` and procedure-mediated access

When the application API is exposed exclusively through stored procedures, a least-privileged login role can be granted *only* `EXECUTE` on the procedures themselves — not direct `SELECT` / `INSERT` / `UPDATE` on the underlying tables. The procedure is then declared `security definer` (and given an explicit `set search_path = …`), so it runs as its owner — a role with full table access — regardless of which login role called it. This is the cleanest production layout: the read/write privilege roles described above are not strictly required when every table access flows through a `security definer` procedure. See [procedures.md](./procedures.md#procedure-signature) for the full contract.

## See also

- [migrations.md](./migrations.md) — when role/tablespace setup is versioned vs repeatable.
- [project-layout.md](./project-layout.md) — where `testing/init.sql` and the `R__x_…` / `R__z_…` files live.
- [deployment.md](./deployment.md) — how the bootstrap container actually runs the init SQL and creates the filesystem paths.
- [procedures.md](./procedures.md) — `security definer` procedures, the application-mediated access model.
