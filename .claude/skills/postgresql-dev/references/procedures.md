# JSON-in / JSON-out procedures

A common pattern in this organisation is to expose **all** database operations to Go application code through a thin layer of stored procedures with a single shape: each procedure takes a JSON input parameter and returns a JSON output parameter. The Go side (the `jsondb` library — Go import path `git.zzdats.lv/lx/go-jsondb`, package `jsondb`) does no `SELECT` / `INSERT` / `UPDATE` of its own; every call goes through `Exec(ctx, "schema.proc", params, &result)` which translates to `CALL "schema"."proc"($1, $2)` with the marshalled `params` as `$1` and `'{}'` as `$2`.

This file documents the database-side conventions that contract demands.

## Procedure signature

Every procedure exposed to the Go layer follows this signature exactly:

```sql
create or replace procedure <schema>.<name>(in pi_data json, inout po_data json)
language plpgsql
as $$
begin
  -- body
end;
$$;
```

Discussion:

- **`in pi_data json`** — the marshalled request payload. May be `{}` if the caller passed no parameters. Even when the body needs no input (e.g. a pure read of a singleton), the parameter must still be declared — the Go caller always passes two arguments.
- **`inout po_data json`** — the response payload. Must be assigned before the procedure returns, otherwise the caller receives a NULL JSON and fails to unmarshal it.
- **`language plpgsql`** — the framework only invokes PL/pgSQL procedures (other languages work in principle, but the surrounding conventions — `RAISE EXCEPTION`, `result_error`, `plpgsql_check` linting — assume PL/pgSQL).
- **`security definer`** is often appropriate. It makes the procedure run with the **owner's** privileges, not the caller's, which is the standard way to expose well-defined operations to a least-privileged login role without granting that login direct table access. Pair it with `set search_path = ...` in the procedure body or at definition time to prevent search-path-shadowing attacks.

Example skeleton with `security definer`:

```sql
create or replace procedure public.test1(in pi_data json, inout po_data json)
security definer
language plpgsql
as
$$
DECLARE
  out_data json;
begin

  out_data := json_build_object('property', 'value', 'input', pi_data);
  po_data:= json_build_object('success', true, 'data', out_data);
end;
$$;
```

## Procedure body structure

Every procedure that writes to the database must wrap its body in a `begin/exception` block. Without this wrapper, an unexpected error surfaces as a generic driver error on the Go side rather than a structured `ExecError`, and the caller cannot inspect `.Code` or `.Message`.

```sql
create or replace procedure public.my_proc(in pi_data json, inout po_data json)
language plpgsql
security definer
set search_path = public, pg_temp
as $$
declare
  -- local variable declarations
begin
  -- procedure body

  po_data := result_success(json_build_object('result', 'value'));
exception
  when sqlstate 'P0001' then
    raise;
  when others then
    raise exception '%', json_build_object(
      'code',  'entity:error',
      'error', sqlerrm,
      'data',  json_build_object('sqlstate', sqlstate)
    ) using errcode = 'P0001';
end;
$$;
```

Discussion:

- **`when sqlstate 'P0001' then raise;`** — a P0001 was already raised by an inner Pattern B call or by this procedure's own validation logic. Re-raise it unchanged. Swallowing it (writing the caught message to `po_data` and returning) would silently commit whatever writes already ran and deliver a corrupted result to the caller.
- **`when others then raise exception '%', json_build_object(...) using errcode = 'P0001';`** — any unexpected error (constraint violation not caught by a specific handler, unexpected NULL, division by zero) is converted into a structured P0001. This prevents raw Postgres error text from reaching the Go side, which would bypass the `ExecError` type and force the caller to string-match against Postgres internals. Include `sqlerrm` and `sqlstate` in the `data` field for observability.
- **`po_data` must be assigned before the normal-exit path.** The `exception` block fires only on errors. If the procedure returns normally without assigning `po_data`, the caller receives the initial `{}` placeholder and fails to unmarshal it.
- **This wrapper does not replace specific exception handlers.** When a known failure mode is expected (e.g. `unique_violation`), add a dedicated handler *before* `when others` to return a meaningful, domain-specific code instead of the generic `entity:error`.

## Return shape — `result_success` / `result_error`

The Go side unmarshals the returned JSON against a fixed shape:

```json
{ "success": true, "data": { ... } }
```

- `success: true` plus a `data` object → caller's result struct is populated from `data`.
- `success: false` plus a `code` / `error` pair (no `data`) → caller receives a typed `ExecError` with `.Code` and `.Message`.

Two helper functions, conventionally shipped as a shared submodule, build these shapes so you never hand-craft the wrapper JSON:

| Helper | Returns |
|---|---|
| `result_success(pi_data json)` | `{"success": true, "data": ...}` |
| `result_error(pi_code text, pi_error text)` | `{"code": ..., "error": ...}` — no `data` field |

Idiomatic usage:

```sql
-- success
po_data := result_success(json_build_object('id', 1, 'name', 'example'));

-- error without rollback (e.g. validation)
po_data := result_error('user:invalid', 'Invalid input');

-- error with rollback (e.g. after DB changes)
raise exception '%', result_error('user:not_found', 'User not found') using errcode = 'P0001';
```

If your project does not include the helpers, build the same shape inline (as in the `test1` example above) — the Go side only cares about the JSON structure, not which function produced it.

## Two error patterns — when to use which

### Pattern A: return error (no rollback)

Assign `result_error(...)` directly to `po_data` and `return`. The procedure exits normally, the wrapping transaction commits, and the Go caller receives an `ExecError`. **Use this only when no data has been modified yet** — typically for input validation that happens before the first `INSERT` / `UPDATE` / `DELETE`:

```sql
create or replace procedure public.my_proc(in pi_data json, inout po_data json)
language plpgsql as $$
begin
  -- validate first, before touching any data
  if (pi_data->>'id') is null then
    po_data := result_error('user:invalid', 'Missing id');
    return;
  end if;

  -- safe to modify data here
  insert into ...;

  po_data := result_success(json_build_object('id', 1));
end;
$$;
```

### Pattern B: raise exception (with rollback)

`raise exception '%', result_error(...) using errcode = 'P0001'` — the `P0001` errcode is the agreed signal that the exception message is itself the structured error JSON. The framework catches it, parses the message, and converts it back into an `ExecError`. **Use this whenever data has already been modified** and you need the changes rolled back:

```sql
  insert into orders ...;

  -- structured error - rolls back, returns ExecError to caller
  raise exception '%', result_error('order:conflict', 'Duplicate order') using errcode = 'P0001';
```

You can also include a `data` field in the raised JSON; the Go side will populate the caller's result struct from it (even though the call returned an error):

```sql
  raise exception '%', json_build_object(
    'code', 'order:conflict',
    'error', 'Duplicate order',
    'data', json_build_object('conflictingId', 42)
  ) using errcode = 'P0001';
```

### Anything else is a generic error

Any other `raise exception` — plain text without `P0001`, or `P0001` with a non-JSON message — still rolls the transaction back, but the Go caller receives a generic wrapped error rather than a typed `ExecError`. This is the right behaviour for programming bugs ("this should never happen") that should not be papered over as user-facing errors:

```sql
  raise exception 'Duplicate order';  -- generic error, rolls back, no Code field on the Go side
```

### Decision rule

| Situation | Pattern |
|---|---|
| Validating inputs before any write | A: `po_data := result_error(...); return;` |
| A precondition fails *after* one or more writes | B: `raise exception '%', result_error(...) using errcode = 'P0001';` |
| The operation succeeded | `po_data := result_success(...);` |
| Internal invariant violated, not a user-facing concern | plain `raise exception 'message';` |

## Error code naming

The `code` argument to `result_error` follows a `<domain>:<reason>` convention with a stable, machine-readable suffix. Two reasons:

1. Humans grepping logs can filter by domain (`user:*`, `order:*`).
2. **The suffix `:not_found`** is special. The Go side's `ExecError.StatusCode()` returns HTTP 404 when the code ends in `:not_found` and HTTP 422 otherwise:

```sql
-- 404 - via return (no rollback)
po_data := result_error('order:not_found', 'Order not found');
return;

-- 404 - via raise (with rollback)
raise exception '%', result_error('order:not_found', 'Order not found') using errcode = 'P0001';

-- 422 - via raise (with rollback)
raise exception '%', result_error('order:invalid_state', 'Order already shipped') using errcode = 'P0001';
```

Reserve `:not_found` strictly for "the requested entity does not exist in the database." Do not use it for "the user is forbidden from seeing this resource" — that's an authorisation concern, not a 404, and reusing the suffix muddies the HTTP semantics.

Other common suffixes (not framework-enforced, just convention):

| Suffix | Meaning | Likely HTTP |
|---|---|---|
| `:not_found` | Entity does not exist | 404 |
| `:invalid` | Input failed validation | 422 |
| `:conflict` | Uniqueness or state conflict | 422 |
| `:forbidden` | Authorisation denied | 422 (framework default — wrap at the HTTP layer if you need 403) |
| `:invalid_state` | Operation not allowed in the current entity state | 422 |

## Transactions — automatic vs manual

The Go side has two modes:

- **Automatic.** `store.Exec(ctx, "schema.proc", params, &result)` without a surrounding `Begin` wraps the single procedure call in its own transaction. Any error — query failure, JSON unmarshal failure, `success: false`, raised exception — triggers an automatic rollback.
- **Manual.** `tx, _ := store.Begin(ctx)` followed by one or more `Exec(tx, …)` calls and a final `tx.Commit()` (with a deferred `tx.Rollback()` as a safety net). When a transaction is already on the context, `Exec` does **not** rollback on its own — that's the caller's responsibility.

The procedure body itself does not know which mode it's running under, and shouldn't care. It should:

- Never `commit` or `rollback` inside the procedure (procedures can — but in this convention they shouldn't, because it would break the framework's automatic rollback).
- Use Pattern B (`raise exception`) whenever it has performed writes that would be visibly wrong if the rest of the manual transaction continued.

## Reading inputs from `pi_data`

The `pi_data json` parameter is unmarshalled via the standard JSON operators:

```sql
-- text value
v_email := pi_data->>'email';

-- nested object
v_address := pi_data->'address';

-- typed cast
v_user_id := (pi_data->>'user_id')::bigint;

-- with NULL handling
v_count := coalesce((pi_data->>'count')::int, 0);

-- array iteration
for v_item in select * from json_array_elements(pi_data->'items')
loop
  ...
end loop;
```

`json_populate_record` and `json_to_record` are handy when the input shape maps cleanly to a row type, but the `->>` / `->` operators are typically clear enough.

### `pi_data` may be unused

A read-only procedure or one whose only effect is "do the singleton thing" may not need any data from `pi_data` — but the parameter declaration is still mandatory because the Go caller always passes two arguments. The static-analysis linter (`plpgsql_check`) explicitly allowlists the "unused parameter `pi_data`" warning for exactly this reason. Other unused parameters still get flagged — `pi_data` is the only one that gets a pass.

## Calling convention — what the Go side actually sends

For a method string `"public.create_user"` and params `{"email": "..."}`, the Go side executes:

```sql
CALL "public"."create_user"($1, $2)
-- $1 = '{"email":"..."}'
-- $2 = '{}'
```

So:

- The schema name is hardcoded in the call string — there is no implicit schema lookup. If you write `Exec(ctx, "create_user", …)` (no schema), the library defaults to `public`.
- Identifiers are double-quoted, which means **the procedure's name must be the literal string** the Go side passes. Avoid mixed-case names (PostgreSQL would lowercase them when defined unquoted, but the Go call would look for the mixed-case spelling).
- `$2` is always the literal `{}`. The procedure body must reassign `po_data`; the initial `{}` is just a placeholder so that pgx has a typed JSON to bind.

## Idempotent migration form

A procedure is a *repeatable* migration. The file lives at `code/<schema>/R__<proc_name>.sql` and the body opens with `create or replace`:

```sql
-- code/public/R__create_user.sql
create or replace procedure public.create_user(in pi_data json, inout po_data json)
language plpgsql
security definer
set search_path = public, pg_temp
as $$
declare
  v_email text;
  v_id bigint;
begin
  v_email := pi_data->>'email';

  if v_email is null or v_email !~ '^[^@]+@[^@]+$' then
    po_data := result_error('user:invalid', 'Email is required and must be well-formed');
    return;
  end if;

  insert into users(email)
  values (v_email)
  returning user_id into v_id;

  po_data := result_success(json_build_object('user_id', v_id));
exception
  when unique_violation then
    raise exception '%', result_error('user:conflict', 'Email already registered') using errcode = 'P0001';
end;
$$;
```

Walk-through:

- **Validation first, return-error pattern.** No writes have happened, so a plain `po_data := result_error(...); return;` is correct.
- **The `insert ... returning ... into` form** captures the generated ID without a follow-up `select`.
- **`when unique_violation`** translates a low-level pg error into a structured `user:conflict` response. The `raise exception '%' ... using errcode = 'P0001'` form rolls back the insert and returns the structured `ExecError` to the Go caller.
- **`security definer` + explicit `set search_path`** — the procedure runs as its owner (typically a role with full table access) regardless of which login role called it. Setting `search_path` at definition time pins the resolution so a malicious caller cannot shadow `users` with their own table.

## Testing JSON procedures with plpgunit

A unit test for the procedure above looks like:

```sql
drop function if exists unit_tests.create_user();

create or replace function unit_tests.create_user()
returns test_result as $$
declare
  message test_result;
  result boolean;
  v_out json;
begin
  -- happy path
  call public.create_user('{"email": "alice@example.com"}', v_out);
  select * into message, result from assert.is_true((v_out->>'success')::boolean);
  if not result then return 'happy path should succeed, got: ' || v_out::text; end if;

  -- validation: missing email -> return error, no rollback
  call public.create_user('{}', v_out);
  select * into message, result from assert.is_false((v_out->>'success')::boolean);
  if not result then return 'missing email should fail, got: ' || v_out::text; end if;
  select * into message, result from assert.is_equal(v_out->>'code', 'user:invalid');
  if not result then return 'expected code user:invalid, got: ' || (v_out->>'code'); end if;

  -- conflict: duplicate email -> raise exception, rollback
  begin
    call public.create_user('{"email": "alice@example.com"}', v_out);
    return 'duplicate email should have raised, but returned: ' || v_out::text;
  exception when sqlstate 'P0001' then
    -- expected; the message is the result_error JSON
    null;
  end;

  -- cleanup
  delete from users where email = 'alice@example.com';

  select * from assert.ok('all create_user paths exercised') into message, result;
  return message;
end;
$$ language plpgsql;
```

Discussion:

- **Calling a procedure from a function** uses the `CALL` statement; the `INOUT` parameter is bound to a local variable.
- **Pattern A (return error)** is checked by inspecting `v_out` directly.
- **Pattern B (raise exception)** is checked by wrapping the call in a `begin ... exception when sqlstate 'P0001'` block. The `null;` body says "this is the expected path"; any *other* exception or a missing exception fails the test.
- **Cleanup** uses the email-based delete because the test ran inside the plpgunit transaction wrapper — the row will be rolled back anyway, but the explicit cleanup keeps the test useful when run outside the wrapper.

## Application-side configuration

When the Go layer reads its database configuration, it consumes a fixed set of environment variables. Database operators and DevOps tooling need to know these because they often appear in the same `.env` / orchestration files as the server-side variables documented in [deployment.md](./deployment.md).

| Variable | Purpose | Default |
|---|---|---|
| `POSTGRES_HOST` | Hostname or IP | — (required) |
| `POSTGRES_PORT` | Port | `5432` |
| `POSTGRES_USER` | Login role | — (required) |
| `POSTGRES_PASSWORD` | Login password | — (required) |
| `POSTGRES_DB` | Database name | — (required) |
| `POSTGRES_SSLMODE` | One of `disable` / `allow` / `prefer` / `require` / `verify-ca` / `verify-full` | `prefer` |
| `POSTGRES_POOL_MIN_CONNS` | Minimum pool size | `0` |
| `POSTGRES_POOL_MAX_CONNS` | Maximum pool size | `10` |
| `POSTGRES_POOL_IDLE_TIME` | Max idle time per connection | `30m` |
| `POSTGRES_POOL_LIFE_TIME` | Max lifetime per connection | `1h` |
| `POSTGRES_LOGLEVEL` | One of `debug` / `info` / `warn` / `error` / `fatal` / `panic` / `dpanic` | `warn` |
| `POSTGRES_SLOW_QUERY_THRESHOLD` | Logs queries slower than this (e.g. `500ms`). `0` disables | `3s` |
| `POSTGRES_DEBUG_QUERY_ARGS` | Log truncated query args at debug/error level. **May log sensitive data.** | `false` |
| `POSTGRES_DEBUG_SLOW_QUERY_ARGS` | Log truncated query args for slow queries at warn level. **May log sensitive data.** | `false` |
| `POSTGRES_LOG_BYTES_LIMIT` | Max bytes per logged arg (max `1000000`; needs one of the debug-args flags) | `512` |

Operational notes:

- **`POSTGRES_USER` here is the application login role**, not the cluster superuser. The compose stack's bootstrap container uses the superuser to provision; the application reads with a least-privileged login (typically granted into `user_read_role` and/or `user_write_role`). Keep these two roles distinct in production.
- **`POSTGRES_DEBUG_QUERY_ARGS=true` is a production-risk setting.** Enabling it persists serialised JSON inputs to logs — including emails, names, tokens, anything that lands in `pi_data`. Use only in development, behind feature flags, or in incident-response windows with a clear scrub plan.
- **`POSTGRES_SLOW_QUERY_THRESHOLD=0` disables the slow-query log entirely.** Useful for benchmarking under load; not appropriate for production unless slow queries are observed by another mechanism.

## Checklist for adding a new procedure

1. **Pick the file path.** `code/<schema>/R__<procedure_name>.sql` (lowercase, snake_case).
2. **Start with the canonical header.** `create or replace procedure <schema>.<name>(in pi_data json, inout po_data json) language plpgsql security definer set search_path = <schema>, pg_temp as $$`.
3. **Declare locals.** Pull values out of `pi_data` with `->>` / `->`; cast as needed.
4. **Validate first.** Any input check that can fire *before* a write uses Pattern A (`po_data := result_error(...); return;`).
5. **Perform writes.** Use `insert ... returning ... into` to capture generated keys.
6. **Translate low-level errors.** Wrap predictable failures (`unique_violation`, `foreign_key_violation`, `check_violation`) in `exception` blocks that re-raise as Pattern B with a meaningful `code`.
7. **Assign `po_data`.** On success, `po_data := result_success(json_build_object(...));`.
8. **Write a plpgunit test.** One success case, every error code, every meaningful branch. File at `testing/tests/unit.<procedure_name>.sql`.
9. **Run the linter.** `plpgsql_check` will catch undeclared variables, wrong-arity calls, and type mismatches; the `unused parameter "pi_data"` warning is already allowlisted.
10. **Commit as a repeatable file.** No versioned migration needed — repeatables re-apply automatically on every deploy whose `R__` checksum changes.

## See also

- [migrations.md](./migrations.md) — why procedures live in `code/<schema>/R__…sql` and start with `create or replace`.
- [roles-and-grants.md](./roles-and-grants.md) — why `security definer` interacts with the read/write privilege roles.
- [testing.md](./testing.md) — plpgunit conventions; the `pi_data` lint allowlist; the `CALL …` form for invoking procedures inside tests.
- [deployment.md](./deployment.md) — server-side env vars; the application's pool settings overlap this table.
