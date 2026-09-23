---
name: postgresql-dev
description: Design, evolve, deploy, and test PostgreSQL databases. Covers schema design (data types, indexing, constraints, JSONB, partitioning), plus project conventions for migrations, roles, tablespaces, dev/CI docker-compose stacks, plpgunit testing, plpgsql_check linting, and JSON-in/JSON-out stored procedures used as the application API surface.
---

# PostgreSQL

## Use this skill when

- Designing a schema for PostgreSQL
- Selecting data types and constraints
- Planning indexes, partitions, or RLS policies
- Reviewing tables for scale and maintainability
- Authoring a versioned or repeatable migration
- Writing a stored procedure that follows the JSON-in (`pi_data`) / JSON-out (`po_data`) contract used as the application API
- Wiring up the dev/CI docker-compose stack, roles, tablespaces, or extensions
- Writing plpgunit tests or running `plpgsql_check`

## Do not use this skill when

- You are targeting a non-PostgreSQL database
- You only need query tuning without schema changes
- You require a DB-agnostic modeling guide

## Instructions

1. Capture entities, access patterns, and scale targets (rows, QPS, retention).
2. Choose data types and constraints that enforce invariants.
3. Decide where the change lives: new DB object → repeatable file `code/<schema>/R__<object>.sql`; new table / column / constraint / data backfill → versioned file `versions/<X.Y.Z>/V<X>_<Y>_<Z>_<UTC_timestamp>__<desc>.sql`.
4. For application-facing operations, design a stored procedure with the canonical signature `(in pi_data json, inout po_data json)`; return `result_success(...)` or `result_error(code, msg)`; raise with `errcode = 'P0001'` when a rollback is needed.
5. Pick the role(s) that need access and the tablespace(s) the table/index should land on.
6. Add indexes for real query paths (FK columns, frequent filters/sorts, join keys) and validate with `EXPLAIN`.
7. Plan partitioning or RLS where required by scale or access control.
8. Write a plpgunit test for any new function/procedure; run `plpgsql_check` for static analysis.
9. Apply via the `postgresql → db-init → db-migrations → db-test-data` compose chain locally before opening a PR.
10. **Translate all `COMMENT ON` statements to Latvian.** After all development work for a task is complete, review every `COMMENT ON` statement written during that work — they may have been drafted in English for speed — and translate each one to Latvian per the **SQL Comments** section. This step is non-optional.

## Safety

- Avoid destructive DDL on production without backups and a rollback plan.
- Use migrations and staging validation before applying schema changes.
- Production migrations are forward-only: never edit a versioned file after merge; ship a new versioned file that inverts the effect.

## Project conventions

A PostgreSQL project that follows the operational conventions assumed by this skill lays out files like this:

- `code/<schema>/R__<object>.sql` — **repeatable** migrations, one file per DB object (function, procedure, view, type, trigger). Body starts with `create or replace`. Re-applied automatically when its checksum changes; executed in alphabetical order (prefix with `R__x_…` / `R__z_…` to force grants and util-only files to run last).
- `versions/<X>.<Y>.<Z>/V<X>_<Y>_<Z>_<YYYYMMDDHHmmSS>__<description>.sql` — **versioned** migrations, run once and only once, tracked by checksum in `public.changelog`. After merge they are immutable; fixes ship as new files in the same version folder. Never two scripts within the same second.
- `modules/<module-name>/` — external DB submodules (git submodules), declared in `submodules.yaml`.
- `testing/` — `init.sql` (idempotent role/tablespace setup), `plpgunit.sql` (test framework), `run_tests.sql`, `lint-pgSQL.sql`, and `tests/` (`init.<topic>.sql` for fixtures, `unit.<target>.sql` per tested function).
- `docker/` — `Dockerfile.dev` (server + extensions), `Dockerfile.init`, `Dockerfile.migrations`, `Dockerfile.test_data`, plus `docker-compose.dev.yaml` (main stack) and optional `docker-compose.admin.yaml` (pgAdmin overlay).
- `DOCUMENTATION.md` is auto-generated from SQL comments — never hand-edit.

Service startup chain: `postgresql → db-init → db-migrations → db-test-data`. Each one-shot waits for the previous to finish successfully. `db-init` creates roles, tablespaces, and enables extensions; `db-migrations` runs `versions/` then `code/`; `db-test-data` loads any test data and runs plpgunit + `plpgsql_check`.

Application access is mediated by stored procedures with a fixed shape: every procedure takes `pi_data json` and an `inout po_data json`, returning `{"success": true, "data": ...}` (via `result_success(...)`) on success or `{"code": "...", "error": "..."}` (via `result_error(...)`) on failure. Errors that need a rollback are raised with `errcode = 'P0001'`; errors caught before any write are assigned directly to `po_data` and returned. Code suffixes are meaningful: `:not_found` maps to HTTP 404, anything else to HTTP 422.

All text files (`.sh`, `.sql`, `.env`, `.conf`, `.yml`, `.yaml`) must be UTF-8 without BOM and LF-terminated. Pre-commit hooks reject CRLF and the UTF-8 BOM.

Pointers to detailed references:

- **Directory tree, file-encoding, immutability rules, submodules.yaml** — [references/project-layout.md](./references/project-layout.md)
- **Versioned vs repeatable, idempotent role/tablespace patterns, fix-up workflow, checksum recovery** — [references/migrations.md](./references/migrations.md)
- **Login roles vs NOINHERIT privilege roles, schema-discovery grants, per-function grants, tablespace strategy** — [references/roles-and-grants.md](./references/roles-and-grants.md)
- **Compose chain, env vars, pgAdmin, pldebugger, CI hooks, forward-only deployment** — [references/deployment.md](./references/deployment.md)
- **plpgunit framework, test file naming, dependencies, `plpgsql_check` linting and exit codes** — [references/testing.md](./references/testing.md)
- **JSON-in/JSON-out procedure contract, `result_success`/`result_error`, error patterns (return vs raise `P0001`), code-suffix HTTP mapping, application-side env vars** — [references/procedures.md](./references/procedures.md)

## SQL Style

- **Keywords**: lowercase — `select`, `insert`, `update`, `delete`, `create`, `alter`, `drop`, `where`, `from`, `join`, `on`, `and`, `or`, `not`, `null`, `true`, `false`.
- **Identifiers**: `snake_case` for tables, columns, functions, procedures, schemas. Never use quoted mixed-case names.
- **Indentation**: 2 spaces. Each column in a select list, each join, and each where predicate on its own line when there are more than 2–3 items.
- **Column lists**: always explicit — never `select *` in stored objects (functions, procedures, views).
- **Continuation operators**: `and` / `or` at the *start* of the continuation line, not the end.
- **CTEs over nested subqueries**: prefer `with cte_name as (…) select … from cte_name` to deeply nested inline subqueries.
- **ASCII-only punctuation**: no Unicode dashes, typographic quote marks, or non-breaking spaces in SQL source.
- **Column qualification**: always qualify column names with a table alias in any multi-table query.

Prefer the standard SQL forms over PostgreSQL-specific shortcuts:

| Use | Instead of |
|---|---|
| `current_timestamp` | `now()` |
| `coalesce(x, y)` | `nvl(x, y)` |
| `position(sub in str)` | `strpos(str, sub)` |
| `extract(field from value)` | `date_part(‘field’, value)` |

## SQL Comments

### Inline comments (`--`, `/* */`)

Add an inline comment only when the *why* is non-obvious — a hidden constraint, a subtle invariant, a workaround for a known bug. Write in **English**. Never narrate what the code does; the identifiers already say that.

### `COMMENT ON` — always in Latvian

Every named database object must carry a `COMMENT ON` statement. Write the comment text in **Latvian**. `DOCUMENTATION.md` is generated from these comments; untranslated English text will appear verbatim in that document.

Objects that must be commented: tables, views, materialized views, every column, functions, procedures, triggers, indexes, constraints, types, domains, enums, sequences, schemas.

Format — place `is` on the same line as the object clause, with the string on a new line indented 2 spaces:

```sql
comment on table orders is
  ‘Pasūtījumi, ko veikuši lietotāji.’;

comment on column orders.status is
  ‘Pasūtījuma statuss: PENDING, PAID vai CANCELED.’;

comment on procedure public.create_user is
  ‘Izveido jaunu lietotāju, validējot e-pasta adreses formātu.’;
```

### Mandatory final step — translate all `COMMENT ON` statements to Latvian

**After finishing all PostgreSQL development work for a task**, systematically go through every `COMMENT ON` statement written during that work. Comments may have been drafted in English for convenience during development — every one of them must be translated to Latvian before the work is considered done. Apply the format rules above. This step is required because `DOCUMENTATION.md` is auto-generated from these comments and must remain in Latvian.

## Core Rules

- Define a **PRIMARY KEY** for reference tables (users, orders, etc.). Not always needed for time-series/event/log data. When used, prefer `BIGINT GENERATED ALWAYS AS IDENTITY`; use `UUID` only when global uniqueness/opacity is needed.
- **Normalize first (to 3NF)** to eliminate data redundancy and update anomalies; denormalize **only** for measured, high-ROI reads where join performance is proven problematic. Premature denormalization creates maintenance burden.
- Add **NOT NULL** everywhere it’s semantically required; use **DEFAULT**s for common values.
- Create **indexes for access paths you actually query**: PK/unique (auto), **FK columns (manual!)**, frequent filters/sorts, and join keys.
- Prefer **TIMESTAMPTZ** for event time; **NUMERIC** for money; **TEXT** for strings; **BIGINT** for integer values, **DOUBLE PRECISION** for floats (or `NUMERIC` for exact decimal arithmetic).
- **Audit / mutability / soft-delete columns** (last-modified timestamp, active flag, deleted-at timestamp, etc.) follow the existing project's conventions — column names, types, and whether they exist at all vary by codebase (e.g. `date_modified` + `active boolean` in one project, `updated_at` + `deleted_at TIMESTAMPTZ` in another). Match what the surrounding tables do; do not introduce a new convention.

## PostgreSQL “Gotchas”

- **Identifiers**: unquoted → lowercased. Avoid quoted/mixed-case names. Convention: use `snake_case` for table/column names.
- **Unique + NULLs**: UNIQUE allows multiple NULLs. Use `UNIQUE (...) NULLS NOT DISTINCT` (PG15+) to restrict to one NULL.
- **FK indexes**: PostgreSQL **does not** auto-index FK columns. Add them.
- **No silent coercions**: length/precision overflows error out (no truncation). Example: inserting 999 into `NUMERIC(2,0)` fails with error, unlike some databases that silently truncate or round.
- **Sequences/identity have gaps** (normal; don't "fix"). Rollbacks, crashes, and concurrent transactions create gaps in ID sequences (1, 2, 5, 6...). This is expected behavior—don't try to make IDs consecutive.
- **Heap storage**: no clustered PK by default (unlike SQL Server/MySQL InnoDB); `CLUSTER` is one-off reorganization, not maintained on subsequent inserts. Row order on disk is insertion order unless explicitly clustered.
- **MVCC**: updates/deletes leave dead tuples; vacuum handles them—design to avoid hot wide-row churn.

## Data Types

- **IDs**: `BIGINT GENERATED ALWAYS AS IDENTITY` preferred (`GENERATED BY DEFAULT` also fine); `UUID` when merging/federating/used in a distributed system or for opaque IDs. Generate with `uuidv7()` (preferred if using PG18+) or `gen_random_uuid()` (if using an older PG version).
- **Integers**: prefer `BIGINT` unless storage space is critical; `INTEGER` for smaller ranges; avoid `SMALLINT` unless constrained.
- **Floats**: prefer `DOUBLE PRECISION` over `REAL` unless storage space is critical. Use `NUMERIC` for exact decimal arithmetic.
- **Strings**: prefer `TEXT`; if length limits needed, use `CHECK (LENGTH(col) <= n)` instead of `VARCHAR(n)`; avoid `CHAR(n)`. Use `BYTEA` for binary data. Large strings/binary (>2KB default threshold) automatically stored in TOAST with compression. TOAST storage: `PLAIN` (no TOAST), `EXTENDED` (compress + out-of-line), `EXTERNAL` (out-of-line, no compress), `MAIN` (compress, keep in-line if possible). Default `EXTENDED` usually optimal. Control with `ALTER TABLE tbl ALTER COLUMN col SET STORAGE strategy` and `ALTER TABLE tbl SET (toast_tuple_target = 4096)` for threshold. Case-insensitive: for locale/accent handling use non-deterministic collations; for plain ASCII use expression indexes on `LOWER(col)` (preferred unless column needs case-insensitive PK/FK/UNIQUE) or `CITEXT`.
- **Money**: `NUMERIC(p,s)` (never float).
- **Time**: `TIMESTAMPTZ` for timestamps; `DATE` for date-only; `INTERVAL` for durations. Use `current_timestamp` for transaction start time, `clock_timestamp()` for current wall-clock time. (See "Do not use" list below for `TIMESTAMP`/`TIMETZ` rules.)
- **Booleans**: `BOOLEAN` with `NOT NULL` constraint unless tri-state values are required.
- **Enums**: `CREATE TYPE ... AS ENUM` for small, stable sets (e.g. US states, days of week). For business-logic-driven and evolving values (e.g. order statuses) → use TEXT (or INT) + CHECK or lookup table.
- **Arrays**: `TEXT[]`, `INTEGER[]`, etc. Use for ordered lists where you query elements. Index with **GIN** for containment (`@>`, `<@`) and overlap (`&&`) queries. Access: `arr[1]` (1-indexed), `arr[1:3]` (slicing). Good for tags, categories; avoid for relations—use junction tables instead. Literal syntax: `'{val1,val2}'` or `ARRAY[val1,val2]`.
- **Range types**: `daterange`, `numrange`, `tstzrange` for intervals. Support overlap (`&&`), containment (`@>`), operators. Index with **GiST**. Good for scheduling, versioning, numeric ranges. Pick a bounds scheme and use it consistently; prefer `[)` (inclusive/exclusive) by default.
- **Network types**: `INET` for IP addresses, `CIDR` for network ranges, `MACADDR` for MAC addresses. Support network operators (`<<`, `>>`, `&&`).
- **Geometric types**: `POINT`, `LINE`, `POLYGON`, `CIRCLE` for 2D spatial data. Index with **GiST**. Consider **PostGIS** for advanced spatial features.
- **Text search**: `TSVECTOR` for full-text search documents, `TSQUERY` for search queries. Index `tsvector` with **GIN**. Always specify language: `to_tsvector('english', col)` and `to_tsquery('english', 'query')`. Never use single-argument versions. This applies to both index expressions and queries.
- **Domain types**: `CREATE DOMAIN email AS TEXT CHECK (VALUE ~ '^[^@]+@[^@]+$')` for reusable custom types with validation. Enforces constraints across tables.
- **Composite types**: `CREATE TYPE address AS (street TEXT, city TEXT, zip TEXT)` for structured data within columns. Access with `(col).field` syntax.
- **JSONB**: preferred over JSON; index with **GIN**. Use only for optional/semi-structured attrs. ONLY use JSON if the original ordering of the contents MUST be preserved.
- **Vector types**: `vector` type by `pgvector` for vector similarity search for embeddings.


### Do not use the following data types
- DO NOT use `timestamp` (without time zone); DO use `timestamptz` instead.
- DO NOT use `char(n)` or `varchar(n)`; DO use `text` instead.
- DO NOT use `money` type; DO use `numeric` instead.
- DO NOT use `timetz` type; DO use `timestamptz` instead.
- DO NOT use `timestamptz(0)` or any other precision specification; DO use `timestamptz` instead
- DO NOT use `serial` type; DO use `generated always as identity` instead.


## Table Types

- **Regular**: default; fully durable, logged.
- **TEMPORARY**: session-scoped, auto-dropped, not logged. Faster for scratch work.
- **UNLOGGED**: persistent but not crash-safe. Faster writes; good for caches/staging.

## Row-Level Security

Enable with `ALTER TABLE tbl ENABLE ROW LEVEL SECURITY`. Create policies: `CREATE POLICY user_access ON orders FOR SELECT TO app_users USING (user_id = current_user_id())`. Built-in user-based access control at the row level.

## Constraints

- **PK**: implicit UNIQUE + NOT NULL; creates a B-tree index.
- **FK**: specify `ON DELETE/UPDATE` action (`CASCADE`, `RESTRICT`, `SET NULL`, `SET DEFAULT`). Add explicit index on referencing column—speeds up joins and prevents locking issues on parent deletes/updates. Use `DEFERRABLE INITIALLY DEFERRED` for circular FK dependencies checked at transaction end.
- **UNIQUE**: creates a B-tree index; allows multiple NULLs unless `NULLS NOT DISTINCT` (PG15+). Standard behavior: `(1, NULL)` and `(1, NULL)` are allowed. With `NULLS NOT DISTINCT`: only one `(1, NULL)` allowed. Prefer `NULLS NOT DISTINCT` unless you specifically need duplicate NULLs.
- **CHECK**: row-local constraints; NULL values pass the check (three-valued logic). Example: `CHECK (price > 0)` allows NULL prices. Combine with `NOT NULL` to enforce: `price NUMERIC NOT NULL CHECK (price > 0)`.
- **EXCLUDE**: prevents overlapping values using operators. `EXCLUDE USING gist (room_id WITH =, booking_period WITH &&)` prevents double-booking rooms. Requires appropriate index type (often GiST).

## Indexing

- **B-tree**: default for equality/range queries (`=`, `<`, `>`, `BETWEEN`, `ORDER BY`)
- **Composite**: order matters—index used if equality on leftmost prefix (`WHERE a = ? AND b > ?` uses index on `(a,b)`, but `WHERE b = ?` does not). Put most selective/frequently filtered columns first.
- **Covering**: `CREATE INDEX ON tbl (id) INCLUDE (name, email)` - includes non-key columns for index-only scans without visiting table.
- **Partial**: for hot subsets (`WHERE status = 'active'` → `CREATE INDEX ON tbl (user_id) WHERE status = 'active'`). Any query with `status = 'active'` can use this index.
- **Expression**: for computed search keys (`CREATE INDEX ON tbl (LOWER(email))`). Expression must match exactly in WHERE clause: `WHERE LOWER(email) = 'user@example.com'`.
- **GIN**: JSONB containment/existence, arrays (`@>`, `?`), full-text search (`@@`)
- **GiST**: ranges, geometry, exclusion constraints
- **BRIN**: very large, naturally ordered data (time-series)—minimal storage overhead. Effective when row order on disk correlates with indexed column (insertion order or after `CLUSTER`).

## Partitioning

- Use for very large tables (>100M rows) where queries consistently filter on partition key (often time/date).
- Alternate use: use for tables where data maintenance tasks dictates e.g. data pruned or bulk replaced periodically
- **RANGE**: common for time-series (`PARTITION BY RANGE (created_at)`). Create partitions: `CREATE TABLE logs_2024_01 PARTITION OF logs FOR VALUES FROM ('2024-01-01') TO ('2024-02-01')`. **TimescaleDB** automates time-based or ID-based partitioning with retention policies and compression.
- **LIST**: for discrete values (`PARTITION BY LIST (region)`). Example: `FOR VALUES IN ('us-east', 'us-west')`.
- **HASH**: for even distribution when no natural key (`PARTITION BY HASH (user_id)`). Creates N partitions with modulus.
- **Constraint exclusion**: requires `CHECK` constraints on partitions for query planner to prune. Auto-created for declarative partitioning (PG10+).
- Prefer declarative partitioning or hypertables. Do NOT use table inheritance.
- **Limitations**: no global UNIQUE constraints—include partition key in PK/UNIQUE. FKs from partitioned tables not supported; use triggers.

## Special Considerations

### Update-Heavy Tables

- **Separate hot/cold columns**—put frequently updated columns in separate table to minimize bloat.
- **Use `fillfactor=90`** to leave space for HOT updates that avoid index maintenance.
- **Avoid updating indexed columns**—prevents beneficial HOT updates.
- **Partition by update patterns**—separate frequently updated rows in a different partition from stable data.

### Insert-Heavy Workloads

- **Minimize indexes**—only create what you query; every index slows inserts.
- **Use `COPY` or multi-row `INSERT`** instead of single-row inserts.
- **UNLOGGED tables** for rebuildable staging data—much faster writes.
- **Defer index creation** for bulk loads—>drop index, load data, recreate indexes.
- **Partition by time/hash** to distribute load. **TimescaleDB** automates partitioning and compression of insert-heavy data.
- **Use a natural key for primary key** such as a (timestamp, device_id) if enforcing global uniqueness is important many insert-heavy tables don't need a primary key at all.
- If you do need a surrogate key, **Prefer `BIGINT GENERATED ALWAYS AS IDENTITY` over `UUID`**.

### Upsert-Friendly Design

- **Requires UNIQUE index** on conflict target columns—`ON CONFLICT (col1, col2)` needs exact matching unique index (partial indexes don't work).
- **Use `EXCLUDED.column`** to reference would-be-inserted values; only update columns that actually changed to reduce write overhead.
- **`DO NOTHING` faster** than `DO UPDATE` when no actual update needed.

### Safe Schema Evolution

- **Transactional DDL**: most DDL operations can run in transactions and be rolled back—`BEGIN; ALTER TABLE...; ROLLBACK;` for safe testing.
- **Concurrent index creation**: `CREATE INDEX CONCURRENTLY` avoids blocking writes but can't run in transactions.
- **Volatile defaults cause rewrites**: adding `NOT NULL` columns with volatile defaults (e.g., `current_timestamp`, `gen_random_uuid()`) rewrites entire table. Non-volatile defaults are fast.
- **Drop constraints before columns**: `ALTER TABLE DROP CONSTRAINT` then `DROP COLUMN` to avoid dependency issues.
- **Function signature changes**: `CREATE OR REPLACE` with different arguments creates overloads, not replacements. DROP old version if no overload desired.

### Re-executability

DDL in a versioned migration may be re-applied against a fresh database during a rebuild. Make every statement safe to re-run:

- **Tables**: `create table if not exists …`
- **Columns**: `alter table … add column if not exists …`
- **Indexes**: `create index if not exists …`
- **Triggers**: `drop trigger if exists … on …; create trigger …` (or `create or replace trigger …` on PG14+).
- **Constraints**: guard with a `pg_constraint` check before adding:

```sql
do $$ begin
  if not exists (
    select 1 from pg_constraint
    where conname = 'orders_status_check'
      and conrelid = 'orders'::regclass
  ) then
    alter table orders add constraint orders_status_check
      check (status in ('PENDING', 'PAID', 'CANCELED'));
  end if;
end $$;
```

### Performance

- **Partial indexes on filtered data**: `create index on orders (user_id) where deleted_at is null` — far smaller than a full index when most rows are soft-deleted or in a terminal state.
- **Sargable predicates**: avoid wrapping an indexed column in a function in `WHERE` — `where lower(email) = $1` cannot use an index on `email`. Create an expression index `create index on users (lower(email))` and ensure the query matches it exactly.
- **`exists` over `in` for large subqueries**: `where exists (select 1 from … where …)` short-circuits on first match; `in (select …)` materialises the full inner result.
- **`returning` instead of a follow-up `select`**: `insert into … returning id` captures generated values without an extra round-trip.
- **Avoid `select *` in stored objects**: explicit column lists prevent breakage when the table grows and keep query plans stable.

## Generated Columns

- `... GENERATED ALWAYS AS (<expr>) STORED` for computed, indexable fields. PG18+ adds `VIRTUAL` columns (computed on read, not stored).

## Functions and Procedures

- **Prefer SQL over PL/pgSQL** for simple, read-only logic. A SQL function is inlinable by the query planner; PL/pgSQL is opaque to the planner and forces an extra function-call boundary.
- **Declare volatility correctly**: `STABLE` for functions that read data but do not modify it and return the same result within a single transaction; `IMMUTABLE` for pure functions of their inputs (no DB reads, no side effects). The planner can fold `IMMUTABLE` calls at planning time and cache them per query.
- **Application-facing operations** use the JSON-in/JSON-out procedure contract: `(in pi_data json, inout po_data json)`. Every such procedure must be wrapped in a `begin/exception` block — see [references/procedures.md](./references/procedures.md) for the full contract and the mandatory exception handler structure.

## Extensions

A common dev install enables `pgcrypto` (crypto primitives), `pgulid` (ULID generation, k-sortable opaque IDs), and `plpgsql_check` (static analysis for PL/pgSQL) out of the box via the bootstrap container. `pldebugger` is preloaded by the server image for pgAdmin step-debugging. The list below covers the broader ecosystem.

- **`pgcrypto`**: `crypt()` for password hashing.
- **`uuid-ossp`**: alternative UUID functions; prefer `pgcrypto` for new projects.
- **`pg_trgm`**: fuzzy text search with `%` operator, `similarity()` function. Index with GIN for `LIKE '%pattern%'` acceleration.
- **`citext`**: case-insensitive text type. Prefer expression indexes on `LOWER(col)` unless you need case-insensitive constraints.
- **`btree_gin`/`btree_gist`**: enable mixed-type indexes (e.g., GIN index on both JSONB and text columns).
- **`hstore`**: key-value pairs; mostly superseded by JSONB but useful for simple string mappings.
- **`timescaledb`**: essential for time-series—automated partitioning, retention, compression, continuous aggregates.
- **`postgis`**: comprehensive geospatial support beyond basic geometric types—essential for location-based applications.
- **`pgvector`**: vector similarity search for embeddings.
- **`pgaudit`**: audit logging for all database activity.

## JSONB Guidance

- Prefer `JSONB` with **GIN** index.
- Default: `CREATE INDEX ON tbl USING GIN (jsonb_col);` → accelerates:
  - **Containment** `jsonb_col @> '{"k":"v"}'`
  - **Key existence** `jsonb_col ? 'k'`, **any/all keys** `?\|`, `?&`
  - **Path containment** on nested docs
  - **Disjunction** `jsonb_col @> ANY(ARRAY['{"status":"active"}', '{"status":"pending"}'])`
- Heavy `@>` workloads: consider opclass `jsonb_path_ops` for smaller/faster containment-only indexes:
  - `CREATE INDEX ON tbl USING GIN (jsonb_col jsonb_path_ops);`
  - **Trade-off**: loses support for key existence (`?`, `?|`, `?&`) queries—only supports containment (`@>`)
- Equality/range on a specific scalar field: extract and index with B-tree (generated column or expression):
  - `ALTER TABLE tbl ADD COLUMN price INT GENERATED ALWAYS AS ((jsonb_col->>'price')::INT) STORED;`
  - `CREATE INDEX ON tbl (price);`
  - Prefer queries like `WHERE price BETWEEN 100 AND 500` (uses B-tree) over `WHERE (jsonb_col->>'price')::INT BETWEEN 100 AND 500` without index.
- Arrays inside JSONB: use GIN + `@>` for containment (e.g., tags). Consider `jsonb_path_ops` if only doing containment.
- Keep core relations in tables; use JSONB for optional/variable attributes.
- Use constraints to limit allowed JSONB values in a column e.g. `config JSONB NOT NULL CHECK(jsonb_typeof(config) = 'object')`


## Examples

### Users

```sql
CREATE TABLE users (
  user_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  email TEXT NOT NULL UNIQUE,
  name TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT current_timestamp
);
CREATE UNIQUE INDEX ON users (LOWER(email));
CREATE INDEX ON users (created_at);
```

### Orders

```sql
CREATE TABLE orders (
  order_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(user_id),
  status TEXT NOT NULL DEFAULT 'PENDING' CHECK (status IN ('PENDING','PAID','CANCELED')),
  total NUMERIC(10,2) NOT NULL CHECK (total > 0),
  created_at TIMESTAMPTZ NOT NULL DEFAULT current_timestamp
);
CREATE INDEX ON orders (user_id) TABLESPACE template_index;
CREATE INDEX ON orders (created_at) TABLESPACE template_index;
```

The `TABLESPACE template_index` clause places the index files on a dedicated tablespace — useful when index I/O is isolated from data I/O on multi-spindle storage and/or when backups exclude rebuildable index spaces. Omit the clause to use the default tablespace.

### JSONB

```sql
CREATE TABLE profiles (
  user_id BIGINT PRIMARY KEY REFERENCES users(user_id),
  attrs JSONB NOT NULL DEFAULT '{}',
  theme TEXT GENERATED ALWAYS AS (attrs->>'theme') STORED
);
CREATE INDEX profiles_attrs_gin ON profiles USING GIN (attrs);
```
