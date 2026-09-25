# Deployment

The dev and CI stacks for a PostgreSQL project that follows these conventions are a four-service docker-compose chain: a PostgreSQL server, a one-shot bootstrap container, a migration container, and a test-data container. They run in strict dependency order so that each step sees a fully prepared database. Production differs only in how the same artefacts are scheduled (CI pipeline, Kubernetes Job, etc.) — the contracts (env vars, exit codes, ordering) are identical.

## Local dev — one command

Bring the stack up:

```sh
docker compose -f ./docker/docker-compose.dev.yaml -p <project>_db up --build -d
```

Bring it up with the optional pgAdmin web UI overlay:

```sh
docker compose -f ./docker/docker-compose.dev.yaml -f ./docker/docker-compose.admin.yaml -p <project>_db up
```

After this, the database is reachable at:

- **From the host:** `localhost:${DB_PORT}` (default `5432`, often overridden in `.env` to e.g. `5445` to avoid conflicts with a system PostgreSQL).
- **From other containers in the same compose project:** `postgresql:5432` (the internal service name).
- **pgAdmin (overlay only):** `http://localhost:${PGADMIN_PORT:-5050}/`.

## Service chain

The four services are ordered by `depends_on` conditions:

```yaml
services:
  postgresql:              # PostgreSQL server (long-running)
    ports: "${DB_PORT:-5432}:5432"
    healthcheck: pg_isready

  db-init:                 # one-shot — creates roles, tablespaces, extensions
    depends_on:
      postgresql:
        condition: service_healthy

  db-migrations:           # one-shot — runs versioned + repeatable migrations
    depends_on:
      db-init:
        condition: service_completed_successfully

  db-test-data:            # one-shot — loads test data + runs plpgunit
    depends_on:
      db-migrations:
        condition: service_completed_successfully
```

Discussion:

- **`service_healthy`** waits for `pg_isready` to succeed inside the `postgresql` container — important because the server takes a few seconds to accept connections after the process starts, and `db-init` will get `connection refused` if it tries too early.
- **`service_completed_successfully`** waits for the previous container to exit with code 0. This pattern is what makes each one-shot a hard gate: if migrations fail, test data does not load.
- **The three one-shots all share the same image base** but copy different artefacts (init scripts, migration scripts, test data) and run different entrypoints.

The full set of shared environment variables (centralised via a YAML anchor in the compose file):

| Variable | Default | Used by | Purpose |
|---|---|---|---|
| `POSTGRES_USER` | `postgres` | postgresql + all bootstrap containers | Superuser for the cluster |
| `POSTGRES_PASSWORD` | `test` | all | Cluster superuser password (dev default only) |
| `POSTGRES_DB` | `template` | all | Default database name. **Must equal** the value hard-coded in `R__x_grant_privileges_user_roles.sql` and the CI variables. |
| `DB_HOST` | `postgresql` | bootstrap containers | Service name to connect to |
| `DB_PORT` | `5432` | bootstrap containers | Internal port |
| `LOG_LEVEL` | `all` | postgresql `Dockerfile.dev` | `none`, `ddl`, `mod`, or `all` — passed as `log_statement=...` |
| `CLEAN_TEST_SCHEMA` | `True` | `db-test-data` | If `False`, the `unit_tests` / `assert` schemas are kept after the test run for inspection |
| `PGADMIN_DEFAULT_EMAIL` | `test@test.zzdats.lv` | pgadmin overlay | Login email |
| `PGADMIN_DEFAULT_PASSWORD` | `test` | pgadmin overlay | Login password |
| `PGADMIN_PORT` | `5050` | pgadmin overlay | Host-side port |
| `PGADMIN_THEME` | `dark` | pgadmin overlay | UI theme |

## The PostgreSQL server image

The dev image inherits from the official `postgres` image and adds debugging + linting extensions plus tablespace directories. The relevant `Dockerfile` excerpt:

```dockerfile
FROM postgres:18

ENV USE_PGXS=1
ENV PGDATA=/var/lib/postgresql/data/pgdata

RUN mkdir -p /data/main && \
	mkdir -p /data/index && \
	mkdir -p /data/archive && \
	mkdir -p /data/log && \
	mkdir -p /data/lx/index && \
	mkdir -p /data/lx/main && \
	mkdir -p /data/lx/archive && \
	chown -R postgres:postgres /data

ENTRYPOINT docker-entrypoint.sh postgres \
	-c shared_preload_libraries=plugin_debugger \
	-c log_statement=${LOG_LEVEL} \
	-p 5432
```

Discussion:

- **`/data/...` directories** are the on-disk locations for the tablespaces created later by `testing/init.sql`. They must exist and be owned by `postgres` before PostgreSQL starts.
- **`shared_preload_libraries=plugin_debugger`** loads `pldebugger`, which is what makes PL/pgSQL step-debugging available via pgAdmin. Changing this list requires a server restart, hence why it's set at the entrypoint rather than at runtime.
- **`log_statement=${LOG_LEVEL}`** controls how verbose the server log is. `all` is fine for dev; in production prefer `ddl` (DDL only) or `none` to avoid logging sensitive parameter values.
- **`USE_PGXS=1`** is set so the extension build process inside the image uses PostgreSQL's standard build system rather than expecting an in-tree build.

Extensions baked into the image at build time: `pgulid` (ULID generation), `pldebugger` (step debugger), `plpgsql_check` (static analysis), and `pgcrypto` (built into the base image). They are *installed* into the image but only *enabled* in a target database by the init script below.

## Bootstrap container (`db-init`)

Runs `testing/init.sql` (idempotent role + tablespace creation) and then enables the extensions in the target database:

```sh
psql -h $DB_HOST -p $DB_PORT -U $POSTGRES_USER -d $POSTGRES_DB -v ON_ERROR_STOP=$ON_ERROR_STOP -f testing/init.sql

psql -h $DB_HOST -p $DB_PORT -U $POSTGRES_USER -d $POSTGRES_DB -v ON_ERROR_STOP=$ON_ERROR_STOP -c 'create extension if not exists pgcrypto;'
psql -h $DB_HOST -p $DB_PORT -U $POSTGRES_USER -d $POSTGRES_DB -v ON_ERROR_STOP=$ON_ERROR_STOP -c 'create extension if not exists pgulid;'
psql -h $DB_HOST -p $DB_PORT -U $POSTGRES_USER -d $POSTGRES_DB -v ON_ERROR_STOP=$ON_ERROR_STOP -c 'create extension if not exists plpgsql_check;'
```

Discussion:

- **`-v ON_ERROR_STOP=ON`** is non-negotiable. Without it, a failed `CREATE TABLESPACE` would print an error but psql would still exit 0, the container would succeed, and the next stage would proceed against a half-bootstrapped database.
- **`create extension if not exists`** is the idempotent form. `CREATE EXTENSION` without the guard fails if the extension is already present, which would break re-runs against a persistent volume.
- **Why extensions are created here, not in a migration:** `CREATE EXTENSION` typically requires superuser. The bootstrap container connects as `POSTGRES_USER` (the cluster superuser). The migration runner connects as a least-privileged user and cannot create extensions, so extensions must be in place before it runs.

The bootstrap container also reads `POSTGRES_HOST` as a fallback if `DB_HOST` is unset, which is useful when sharing the script across compose stacks that name the DB service differently.

## Migration container (`db-migrations`)

Runs all versioned and repeatable migrations against the database:

```sh
./DbMigration \
    --conn "Server=${DB_HOST};Database=${POSTGRES_DB};User Id=${POSTGRES_USER};Password=${POSTGRES_PASSWORD};Port=${DB_PORT:5432}" \
    --path ./ --config submodules.yaml --module-dir modules --script-dir versions,code
```

Discussion:

- **`--script-dir versions,code`** is the order: every file under `versions/` is applied first (in version-then-timestamp order), then every file under `code/` (alphabetical, repeatable). Reversing the order would cause repeatable grants to run before the schemas they grant on exist.
- **`--config submodules.yaml --module-dir modules`** tells the runner to also walk each declared module's own `versions/` and `code/` and interleave them with the project's own. Module migrations run as part of the same transactional unit and are tracked in `public.changelog` the same way.
- **Connection string** is .NET-style (`Server=…;Database=…;User Id=…`) because the runner is a .NET binary; the order of fields doesn't matter but every field must be present.
- **`public.changelog`** is created by the runner on first contact with the database. Each successful migration writes a row with the file name, applied-at timestamp, and a checksum. Editing a previously-applied versioned file changes the checksum, and the next run fails — see [migrations.md](./migrations.md#checksum-conflict-recovery).

## Test-data container (`db-test-data`)

Loads any test data and runs the plpgunit test suite. The relevant flow:

```sh
run_tests() {
  # Import plpgunit framework
  psql -h $DB_HOST -p $DB_PORT -U $POSTGRES_USER -d $POSTGRES_DB -v ON_ERROR_STOP=ON -f testing/plpgunit.sql || return 1
  # Import test scripts
  find testing/tests -type f -iname '*.sql' -print0 | sort -zn | while IFS= read -r -d '' file; do
    psql -h $DB_HOST -p $DB_PORT -U $POSTGRES_USER -d $POSTGRES_DB -v ON_ERROR_STOP=ON -f "$file" || return 1
  done || return 1
  # Run tests
  psql -h $DB_HOST -p $DB_PORT -U $POSTGRES_USER -d $POSTGRES_DB -v ON_ERROR_STOP=ON -f testing/run_tests.sql || return 1
}

clean_test_data() {
  if [ "$CLEAN_TEST_SCHEMA" != "False" ]; then
    psql -h $DB_HOST -p $DB_PORT -U $POSTGRES_USER -d $POSTGRES_DB -v ON_ERROR_STOP=OFF -c 'drop schema if exists unit_tests cascade; drop schema if exists assert cascade;'
  fi
}
```

Discussion:

- **Test discovery is just `find … *.sql | sort`.** Naming therefore matters: see [testing.md](./testing.md#test-file-conventions) for the `init.<topic>.sql` / `unit.<target>.sql` pattern.
- **`CLEAN_TEST_SCHEMA=False`** is the right setting when a test is failing locally and you want to introspect `unit_tests.tests` and `unit_tests.test_details` after the container exits.
- **Cleanup uses `ON_ERROR_STOP=OFF`** because a fresh run may not have created the schemas yet, and we'd rather "schema didn't exist" be a no-op than a fatal error.

## Health probe — `wait-for-postgres.sh`

When a *non-compose* consumer (e.g. an application container that doesn't speak the compose dependency chain) needs to gate on DB readiness, the canonical script is:

```sh
RETRIES=10
export PGPASSWORD=$POSTGRES_PASSWORD

if [ -z "$DB_PORT" ]; then
    DB_PORT=5432
fi

until psql -h $POSTGRES_HOST -U $POSTGRES_USER -d $POSTGRES_DB -p $DB_PORT -c "select 1" > /dev/null 2>&1 || [ $RETRIES -eq 0 ]; do
  echo "Waiting for postgres server, $((RETRIES--)) remaining attempts..."
  sleep 3
done
```

Ten retries × three seconds = 30s worst case. Calibrate to your server's actual start-up time; this is a sensible default.

## pgAdmin overlay

The optional `docker-compose.admin.yaml` adds a `pgadmin` service (`dpage/pgadmin4`) with `PGADMIN_CONFIG_SERVER_MODE=False` (desktop-style single-user). The container's entrypoint script auto-discovers the database from the same env vars the rest of the stack uses:

- It writes a `servers.json` declaring a connection named `local db` to `${DB_HOST}:${DB_PORT}` as `${POSTGRES_USER}` against `${POSTGRES_DB}`.
- It writes a `.pgpass` file (mode `600`) so the saved password actually works on first run (pgAdmin's per-server password encryption otherwise fails until the user sets a master password).
- On every start, a small Python script updates the SQLite preferences DB with `PGADMIN_THEME` — because pgAdmin only reads the JSON preferences file on first launch.

URL: `http://localhost:${PGADMIN_PORT:-5050}/`. Default login is the env-var pair.

## pldebugger via pgAdmin

Step-debugging PL/pgSQL functions:

1. Connect to the database in pgAdmin as a `SUPERUSER` (typically `postgres`). The `pldebugger` extension uses backend-level instrumentation that non-superusers cannot drive.
2. Tools → Debugger to open the debugger pane.
3. Set breakpoints on the relevant lines.
4. Right-click the function in the tree → Debug → supply parameters → step through.

If the Debugger menu is greyed out, `shared_preload_libraries=plugin_debugger` is missing from the server config — see the `Dockerfile.dev` excerpt above.

## Forward-only deployments

Production migrations are **forward-only**. To "roll back" an already-applied versioned migration, ship a new versioned migration that reverses the effect — never rewrite history in `versions/`. The reason is operational: in a multi-environment pipeline (dev → staging → prod), every environment has its own `public.changelog`. Editing a file that has already shipped to staging means staging fails its next deploy with a checksum conflict, and the only "fix" is to manipulate the changelog row directly — exactly the kind of un-audited change the changelog exists to prevent.

For a real rollback after a bad release, the playbook is:

1. Open a hot-fix branch from the last good commit.
2. Author a new versioned migration whose body inverts the bad change.
3. Deploy through the normal pipeline.
4. Post-mortem the original change in the team channel, not by deleting it.

## CI pipeline shape

A typical CI run for a database project performs, in order:

1. **Restricted-file guard.** `check-restricted-files.sh` runs first and rejects the build if anyone touched a file under `versions/` (immutable after merge), edited `DOCUMENTATION.md` (bot-only), or committed CRLF / UTF-8-BOM.
2. **Build + migrate.** Spin up the same four-service compose stack against a clean volume; migrations must complete cleanly.
3. **Lint.** Run `plpgsql_check` over every PL/pgSQL function — see [testing.md](./testing.md#linting-with-plpgsql_check). Exit code 1 fails the build; exit code 2 (warnings only) is non-fatal.
4. **Test.** Run plpgunit — `select unit_tests.begin_psql();` raises an exception on any test failure, exiting psql with non-zero.
5. **Docs regen.** A bot regenerates `DOCUMENTATION.md` from SQL comments and pushes the update.

On a green build, the same migration container image is what production deploys; the only difference is the connection string and the secret-sourced password.

## Application-side env vars

The Go application that talks to this database has its own set of `POSTGRES_*` environment variables (SSL mode, pool sizing, log level, slow-query threshold, debug-arg flags). They live alongside — but are distinct from — the server-side vars in the table above; they configure the *client* connection pool, not the *server*. The full list is documented in [procedures.md](./procedures.md#application-side-configuration).

## See also

- [project-layout.md](./project-layout.md) — directory tree the compose stack expects.
- [migrations.md](./migrations.md) — what the migration runner does with the files.
- [roles-and-grants.md](./roles-and-grants.md) — what `db-init` provisions.
- [testing.md](./testing.md) — how `db-test-data` runs plpgunit and the linter.
- [procedures.md](./procedures.md) — JSON procedure contract and the application-side env vars.
