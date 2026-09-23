# Project layout

How a PostgreSQL project that follows the operational conventions assumed by this skill is laid out on disk, and the rules that keep it healthy across many contributors.

## Top-level directory tree

```
<project-root>/
├── code/                              # repeatable migrations (Flyway-style R__ files)
│   └── <schema>/                      # one subfolder per DB schema
│       └── R__<object_name>.sql       # one file per function/procedure/view/type
├── versions/                          # versioned migrations grouped by release
│   └── <X>.<Y>.<Z>/                   # e.g. 0.0.1, 1.2.0
│       └── V<X>_<Y>_<Z>_<YYYYMMDDHHmmSS>__<description>.sql
├── modules/                           # external DB submodules (git submodules)
│   └── <module-name>/
├── submodules.yaml                    # module load order for the migration runner
├── testing/
│   ├── init.sql                       # roles, tablespaces (idempotent)
│   ├── plpgunit.sql                   # plpgunit framework
│   ├── run_tests.sql                  # test runner entrypoint
│   ├── lint-pgSQL.sql                 # plpgsql_check query
│   ├── tests/                         # unit test functions
│   │   ├── init.<topic>.sql
│   │   └── unit.<target_function>.sql
│   └── db-init.sh, db-test.sh, …      # shell wrappers used by Dockerfiles
├── docker/
│   ├── Dockerfile.dev                 # PostgreSQL server + extensions
│   ├── Dockerfile.init                # role/tablespace bootstrap
│   ├── Dockerfile.migrations          # runs versioned + repeatable migrations
│   ├── Dockerfile.test_data           # loads test data after migrations
│   ├── docker-compose.dev.yaml        # main dev stack
│   ├── docker-compose.admin.yaml      # optional pgAdmin overlay
│   └── .env                           # local env (gitignored)
├── DB_CONFIGURATION.md                # human-maintained operations reference
├── DOCUMENTATION.md                   # AUTO-GENERATED — never hand-edit
└── README.md                          # quick-start
```

A few rules are worth restating explicitly because they bite when ignored.

## Schema-per-subfolder under `code/`

Repeatable migrations live under `code/<schema>/R__<object>.sql`. One file per DB object (function, procedure, view, type). To create procedure `get_data` in schema `test`, the file is `code/test/R__get_data.sql` and its body starts with:

```sql
create or replace
```

This keeps every DB object replaceable in place — when the file changes, the runner detects the checksum drift and re-applies it. No diff-against-current-DB, no manual `DROP`.

## Migration file naming

Two distinct conventions; do not mix them.

- **Versioned** (run once, in order, tracked by checksum): `V<X>_<Y>_<Z>_<YYYYMMDDHHmmSS>__<description>.sql`
  - `<X>`, `<Y>`, `<Z>` — semantic version numbers, mirrored in the parent folder name.
  - `<YYYYMMDDHHmmSS>` — UTC timestamp at script creation. **Never two scripts within the same second** — bump by 1 if you must.
  - `<description>` — lowercase, words separated by `_`, English.
  - Example: `versions/0.0.1/V0_0_1_20220117125300__add_new_column_to_table.sql`
- **Repeatable** (re-applied on checksum change, alphabetical execution order): `R__<object_name>.sql`
  - The runner sorts repeatables alphabetically, so prefix letters (e.g. `R__x_grant_privileges.sql`, `R__z_util_grants.sql`) are the canonical way to force grants and util-only objects to run last.

## Immutability rules

These are enforced by a pre-commit / CI guard. Violating them rejects the PR.

- **`versions/` is immutable after merge.** Once a versioned file has been applied by any environment, its checksum is recorded in `public.changelog`. Modifying it later causes a checksum mismatch and the migration runner refuses to proceed. Fixes ship as a **new** versioned file in the same version folder.
- **`DOCUMENTATION.md` is bot-only.** It is regenerated from SQL comments on push to the main development branch. Any hand-edit is rejected.

The relevant check looks for both modifications:

```sh
modified_files=$(git diff --diff-filter=M FETCH_HEAD versions/)
# Modifications to existing files in versions/ folder are not allowed

git diff --name-only FETCH_HEAD | grep -q "DOCUMENTATION.md"
# Manual changes to DOCUMENTATION.md are not allowed. This file can only be modified by the automated bot.
```

## File-encoding rules

All text files committed to the project (`.sh`, `.sql`, `.env`, `.conf`, `.yml`, `.yaml`) must be:

- **UTF-8** without a Byte Order Mark.
- **LF** line endings (no CRLF).

The pre-commit guard checks both:

```sh
CR_CHAR=$(printf '\r')
if [ "$(head -c 3 "$file" | od -An -tx1 -N3 | tr -d '[:space:]')" = "efbbbf" ]; then
  # BOM detected → reject
fi
if LC_ALL=C grep -q "${CR_CHAR}\$" "$file"; then
  # CRLF detected → reject
fi
```

A BOM in a shell script breaks the shebang. CRLF in a `.sql` file confuses some psql-on-Linux setups when the script is piped through stdin. Editor recipes:

- **VS Code** — `File → Save with Encoding → UTF-8`. End-of-line picker in the status bar → `LF`.
- **Notepad++** — `Encoding → Encode in UTF-8 without BOM`. `Edit → EOL Conversion → Unix (LF)`.
- **Git** — set `* text=auto eol=lf` in `.gitattributes` to prevent platform drift.

## Submodule mechanism

External database modules (shared utility schemas, auth, audit, etc.) are pulled in as git submodules under `modules/<name>/`. The migration runner does not auto-discover them; the file `submodules.yaml` at the project root declares which to load:

```yaml
- database-util
- some-other-module
```

The list order is informational; the runner walks each module's own `code/` and `versions/` according to the same rules. To bump a module version, change the submodule's tracked tag in three places (the submodule pointer itself, the README's setup instructions, and the CI pipeline file), then commit.

## Default DB name consistency

The database name appears in at least four places that must stay in lockstep, or grants and connections silently target the wrong database:

| Location | What | Token |
|---|---|---|
| CI/CD pipeline YAML | deployment variables | `postgres_db`, `database_url` |
| `docker/.env` | local dev | `POSTGRES_DB` |
| `code/R__x_grant_privileges_user_roles.sql` | hard-coded in the grant block | `DATABASE` literal in `GRANT CONNECT ON DATABASE … TO …` |
| `.vscode/tasks.json` | dev task variables | `DATABASE` |

When forking the project for a new database name, search and replace all four. The "grants applied to the wrong DB" bug is annoying because the migration still succeeds — the privilege role just ends up unable to connect.

## See also

- [migrations.md](./migrations.md) — versioned vs repeatable, idempotent patterns, fix-up workflow.
- [roles-and-grants.md](./roles-and-grants.md) — login roles vs `NOINHERIT` privilege roles, tablespace ownership.
- [deployment.md](./deployment.md) — service chain, env vars, healthchecks.
- [testing.md](./testing.md) — plpgunit conventions and `plpgsql_check` linting.
