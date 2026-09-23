# Testing and linting

PL/pgSQL is tested with **plpgunit** (an in-database xUnit-style framework) and statically analysed with **plpgsql_check** (a PostgreSQL extension that catches compilation, type, and semantic errors at lint time). Both run in the same `db-test-data` container that the deployment chain spins up after migrations; both are gated in CI so that the build fails on a real regression.

## Framework primer

`testing/plpgunit.sql` defines two schemas:

- **`assert`** — assertion functions used inside tests. Examples: `assert.is_equal(have, want)`, `assert.is_null(value)`, `assert.is_true(boolean)`. Each returns a tuple `(message text, result boolean)`.
- **`unit_tests`** — the engine: bookkeeping tables (`unit_tests.tests`, `unit_tests.test_details`, `unit_tests.dependencies`), the discovery query that finds tests, the runner (`unit_tests.begin`, `unit_tests.begin_psql`), and the dependency machinery (`unit_tests.add_dependency`).

A **test** is a PL/pgSQL function that:

- Lives in any schema (by convention, `unit_tests`).
- Returns the domain `public.test_result` — a `text` domain. The runner discovers tests by querying `pg_proc` for functions with this return type.
- Returns an **empty string** (or `assert.ok(...)`) on success, and a **descriptive non-empty string** on failure.

The framework's `test_result` domain looks like:

```sql
CREATE DOMAIN public.test_result AS text;
```

## Test file conventions

All test SQL lives under `testing/tests/`. The test-data container discovers them with `find … -iname '*.sql' | sort` — file order is alphabetical, so naming matters.

- **`init.<topic>.sql`** — fixture-setup functions used by other tests. Run first because `init.` sorts before `unit.`.
- **`unit.<target_function>.sql`** — one file per function under test, named after the target. Example: `unit.get_global_constant.sql` tests `public.get_global_constant`.

Each file follows a `drop function if exists … ; create or replace function …` pattern so re-runs are clean.

## Example — anatomy of a unit test

```sql
drop function if exists unit_tests.get_global_constant();

create or replace function unit_tests.get_global_constant()
returns test_result as $$
declare
  message test_result;
  result boolean;
  v_constant_key varchar = 'test_constant';
  v_constant_value varchar = 'test_value';
  v_result varchar;
begin
  -- Setup: Insert a test constant
  insert into global_constants ("key", "value")
  values (v_constant_key, v_constant_value)
  on conflict ("key") do update set "value" = excluded."value";

  -- Test 1: Retrieve existing constant
  v_result := public.get_global_constant(v_constant_key);
  select * into message, result from assert.is_equal(v_result, v_constant_value);
  if not result then
      return 'Test 1 failed: get_global_constant should return the correct value. ' ||
            'Expected: ' || v_constant_value || ', Got: ' || COALESCE(v_result::text, 'NULL');
  end if;

  -- Test 2: Retrieve non-existing constant
  v_result := public.get_global_constant('non_existing_constant');
  select * into message, result from assert.is_null(v_result);
  if not result then
      return 'Test 2 failed: get_global_constant should return NULL for a non-existing constant. ' ||
            'Got: ' || COALESCE(v_result::text, 'NOT NULL');
  end if;

  -- Clean up
  delete from global_constants where "key" = v_constant_key;

  select * from assert.ok('All get_global_constant tests passed') into message, result;
  return message;
end;
$$ language plpgsql;
```

Walk-through:

- **`drop function if exists … ; create or replace …`** — the runner imports each file with `psql -f`. The drop-then-replace is harmless on a fresh DB and necessary when the function's *signature* changes (a plain `CREATE OR REPLACE` would create a new overload rather than replacing the old one). Always drop with the exact signature, otherwise the drop matches nothing and you keep the stale function.
- **Idempotent fixture insert** — `on conflict ("key") do update set "value" = excluded.""value""` makes the setup safe to re-run without manual cleanup between attempts.
- **`select * into message, result from assert.is_equal(...)`** — the assert functions return `(message text, result boolean)`. The test must split that tuple into local variables; `result` drives control flow, `message` is for logging.
- **Early-return on failure** — building a useful failure string with `||` concatenation is the convention; include actual vs expected, and `COALESCE(value::text, 'NULL')` so a NULL doesn't render as an empty string.
- **Cleanup before returning success** — explicitly delete fixture rows so a subsequent test doesn't inherit them. Note: the test runner *already* wraps the whole suite in a transaction that rolls back, so this is belt-and-braces; useful when running tests interactively without that wrapper.
- **`assert.ok('…')`** on the success path returns an empty `test_result` text, which is the convention for "pass". The captured `message` is then returned.

## Available assertions

From the `assert` schema:

| Function | Use for |
|---|---|
| `assert.fail(message text)` | Raise a `WARNING` and return a failure message |
| `assert.pass(message text)` | Raise a `NOTICE`, return success |
| `assert.ok(message text)` | Identical to `pass` — slightly different log level convention |
| `assert.is_equal(have, want)` | Equality of two values of the same type |
| `assert.are_equal(VARIADIC anyarray)` | All elements of an array are equal |
| `assert.is_not_equal(have, dont_want)` | Inequality |
| `assert.are_not_equal(VARIADIC anyarray)` | No two array elements are equal |
| `assert.is_null(value)` | Value is NULL |
| `assert.is_not_null(value)` | Value is not NULL |
| `assert.is_true(boolean)` | Boolean is true |
| `assert.is_false(boolean)` | Boolean is false |
| `assert.is_greater_than(x, y)` | `x > y` |
| `assert.is_less_than(x, y)` | `x < y` |
| `assert.function_exists(function_name text)` | A function with the given fully-qualified signature exists |
| `assert.if_functions_compile(VARIADIC _schema_name text[])` | All functions in the given schema(s) compile cleanly |
| `assert.if_views_compile(VARIADIC _schema_name text[])` | All views in the given schema(s) compile cleanly |

The `if_functions_compile` / `if_views_compile` assertions are particularly useful as a smoke test: a single test function that calls them on every project schema acts as a cheap canary.

## Test dependencies

Declare ordering between tests so one test does not run before its prerequisite (e.g. a test of `get_global_constant` should not run before `init_settings` has populated the constants table):

```sql
select unit_tests.add_dependency('unit_tests.get_global_constant', 'unit_tests.init_settings');
```

The runner builds a dependency graph and:

- Refuses to run if a cycle is detected (it reports the cycle).
- Executes tests in topological order.
- **Skips** any test whose dependency failed, and reports it in the skipped count rather than re-running it speculatively.

Without explicit dependencies, tests run in alphabetical order of function name, which is usually fine but brittle if a fixture rename changes ordering.

## Runner contract

The test-data container's final psql call runs:

```sql
begin transaction;
select unit_tests.begin_psql();
rollback transaction;
```

Discussion:

- **`begin_psql`** is the CI-flavour wrapper around `unit_tests.begin`. On any failure, it raises an exception, which (combined with psql's `-v ON_ERROR_STOP=ON`) causes psql to exit non-zero, which fails the container, which fails the compose-up.
- **Transactional wrapper.** Every change made by every test — fixture inserts, schema mutations, function creates — is rolled back at the end. State never leaks between test runs even if a test forgot its cleanup block.
- **Verbosity** (passed as the first argument to `begin` or `begin_psql`): `0` = debug5 (loudest), through `5` = log, `7` = warning, `9` = fatal (default), to `11` (capped to 9). Default is sensible; lower it to 5 (`log`) when investigating a failure locally.

## `CLEAN_TEST_SCHEMA` env var

After the test run completes (success or failure), the container's wrapper calls:

```sh
clean_test_data() {
  if [ "$CLEAN_TEST_SCHEMA" != "False" ]; then
    psql … -c 'drop schema if exists unit_tests cascade; drop schema if exists assert cascade;'
  fi
}
```

Set `CLEAN_TEST_SCHEMA=False` in `.env` when you want to introspect `unit_tests.tests` and `unit_tests.test_details` after a failed run — they're rolled back at the end of the test transaction but the schemas themselves survive, populated by the import step.

## Linting with `plpgsql_check`

`plpgsql_check` walks `pg_proc` and statically analyses every PL/pgSQL function and trigger. It catches:

- Compilation errors (undefined identifiers, wrong number of `INTO` targets).
- Type mismatches.
- Unused parameters and unused local variables.
- Unreachable code.

The query that runs the lint pass:

```sql
select
  (pcf).functionid::regprocedure,
  (pcf).lineno,
  (pcf).statement,
  (pcf).sqlstate,
  (pcf).message,
  (pcf).level
from (
  select plpgsql_check_function_tb (pg_proc.oid, coalesce(pg_trigger.tgrelid, 0)) as pcf
  from pg_proc
  left join pg_trigger on (pg_trigger.tgfoid = pg_proc.oid)
  where
    prolang = (select lang.oid from pg_language lang where lang.lanname = 'plpgsql')
    and pronamespace <> (select nsp.oid from pg_namespace nsp where nsp.nspname = 'pg_catalog')
    and (
      pg_proc.prorettype <> (select typ.oid from pg_type typ where typ.typname = 'trigger')
      or pg_trigger.tgfoid is not null
    )
  offset 0
) ss
where (pcf).message not like 'unused parameter "pi_data"'
  and not (pcf).functionid::regprocedure::text = 'generate_ulid()'
order by (pcf).functionid::regprocedure::text, (pcf).lineno;
```

Discussion:

- **What it covers:** every PL/pgSQL function outside `pg_catalog`. The `prorettype <> trigger` / `pg_trigger.tgfoid is not null` join makes sure trigger functions are checked *with* the relation they're attached to, which is what `plpgsql_check_function_tb` needs for accurate analysis.
- **`pi_data` allowlist.** Functions that act as callbacks for an external library sometimes need a parameter named `pi_data` that the function body legitimately ignores. The allowlist suppresses that one specific "unused parameter" warning rather than the entire unused-parameter category.
- **`generate_ulid()` allowlist.** This function comes from the `pgulid` extension and triggers a false positive that is not worth tracking down upstream.
- **`offset 0`** is a planner hint that prevents the planner from pushing the outer `where` filters into the lateral subquery, which would make the cost-estimate go haywire on a large `pg_proc`.

The shell wrapper that turns the lint output into a CI exit code:

```sh
if [ "$(grep -c 'error' /tmp/output.txt)" -ge 1 ]; then
  echo "FAILED!!! Errors found:"
  cat /tmp/output.txt
  exit 1
elif [ "$(grep -c 'warning' /tmp/output.txt)" -ge 1 ]; then
  echo "WARNING: Warnings found (no errors):"
  cat /tmp/output.txt
  exit 2
else
  exit 0
fi
```

Exit codes:

- **`0`** — clean. No errors, no warnings.
- **`1`** — errors found. Build fails.
- **`2`** — warnings only. Build is advisory: pipelines treat this as either soft-fail or pass-with-warning depending on policy. A pragmatic default is to fail PR builds on errors only but surface warnings as PR review comments.

## Local invocation cheat-sheet

With the dev stack already up (`docker compose -f ./docker/docker-compose.dev.yaml -p <project>_db up --build -d`):

```sh
# run all unit tests
docker exec -e PGPASSWORD=$POSTGRES_PASSWORD <project>_db-postgresql-1 \
  psql -U $POSTGRES_USER -d $POSTGRES_DB -c "begin transaction; select unit_tests.begin_psql(); rollback transaction;"

# run the linter and surface errors/warnings
docker exec -e PGPASSWORD=$POSTGRES_PASSWORD <project>_db-postgresql-1 \
  psql -U $POSTGRES_USER -d $POSTGRES_DB -v ON_ERROR_STOP=ON -c "\x on" -f /tmp/lint-pgSQL.sql

# inspect the most recent test run results (requires CLEAN_TEST_SCHEMA=False)
docker exec -e PGPASSWORD=$POSTGRES_PASSWORD <project>_db-postgresql-1 \
  psql -U $POSTGRES_USER -d $POSTGRES_DB -c "select function_name, status, message from unit_tests.test_details order by id desc limit 20;"
```

Substitute `<project>` with whatever `-p` value was passed to `docker compose up`.

## Test naming and coverage

### File and function names

- **File**: `testing/tests/unit.<target_function>.sql` — one file per function or procedure under test.
- **Test function**: `unit_tests.<target_function>()` — the function name mirrors the target. For complex objects with many scenarios, consider splitting into multiple test functions named `unit_tests.<target_function>__<scenario>()`, one scenario per function (e.g. `unit_tests.create_user__duplicate_email`, `unit_tests.create_user__missing_field`).

### Coverage requirements

A complete test covers at minimum:

1. **Happy path** — normal inputs produce the expected result.
2. **Null / missing inputs** — required fields absent, optional fields absent.
3. **Empty sets** — queries against empty tables or empty arrays return sensible results (not an error).
4. **Boundary values** — minimum/maximum allowed values, off-by-one edges for numeric checks.
5. **Error paths** — every distinct `result_error` code the procedure can return; every raised P0001.

When performance matters, add a timing assertion via `clock_timestamp()` around the critical call and fail the test if execution exceeds an agreed threshold. Document the threshold and the rationale for it in a comment.

## Writing a new test — checklist

1. Pick the right filename: `testing/tests/unit.<target_function>.sql`. For multiple scenario functions, keep them all in that file unless the file grows unwieldy.
2. Open with `drop function if exists unit_tests.<target_function>(); create or replace function unit_tests.<target_function>() returns test_result as $$ … $$ language plpgsql;`.
3. Set up fixtures idempotently (`on conflict … do update`). Don't rely on table state from a previous test.
4. Use the `select * into message, result from assert.<fn>(…); if not result then return '…descriptive failure…'; end if;` pattern for every assertion.
5. Cover at minimum: happy path, null/missing inputs, empty sets, boundary values, and every distinct error code (see **Test naming and coverage** above).
6. End with `select * from assert.ok('…'); return message;` on the success path.
7. If the test depends on a fixture from another test file, register the dependency with `unit_tests.add_dependency(...)` at the bottom of the file.

## Testing JSON procedures

When the function under test is a JSON-in/JSON-out application procedure (signature `(in pi_data json, inout po_data json)`), the plpgunit test calls it with `CALL public.<proc>('{...}', v_out)`, then inspects the resulting `v_out` JSON for `success`, `code`, and `data` fields. Pattern B errors (`raise exception … using errcode = 'P0001'`) are asserted by wrapping the `CALL` in `begin … exception when sqlstate 'P0001' then …`. A worked example is in [procedures.md](./procedures.md#testing-json-procedures-with-plpgunit).

## See also

- [deployment.md](./deployment.md) — how the test-data container is wired into the compose chain.
- [project-layout.md](./project-layout.md) — directory locations for `testing/` and `testing/tests/`.
- [migrations.md](./migrations.md) — repeatable vs versioned conventions that the test suite verifies.
- [procedures.md](./procedures.md) — the JSON procedure contract (explains why `pi_data` is on the linter's allowlist).
