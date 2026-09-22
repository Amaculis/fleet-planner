#!/usr/bin/env bash
# Verifies the migrations against a throwaway Postgres container:
#   1. up   (all migrations, as the schema owner)
#   2. smoke test of every schema invariant (db/testdata/constraints_smoke.sql)
#   3. down (all the way to an empty schema)
#   4. up   again - proves the down migrations genuinely reverse the up ones
# Needs only Docker. The container and its data are deleted at the end.
#
# Usage: scripts/verify-schema.sh
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
CONTAINER="bus-fleet-schema-check-$$"
PGIMAGE="postgres:17.6-alpine"
OWNER_PW="verify-owner"
APP_PW="verify-app"

cleanup() { docker rm -f "$CONTAINER" >/dev/null 2>&1 || true; }
trap cleanup EXIT

# Git Bash / MSYS rewrites POSIX paths before Docker sees them; cygpath -m gives the
# Windows form Docker Desktop expects. A no-op elsewhere.
hostpath() {
  if command -v cygpath >/dev/null 2>&1; then cygpath -m "$1"; else printf '%s' "$1"; fi
}
export MSYS_NO_PATHCONV=1

echo "==> starting $PGIMAGE"
docker run -d --name "$CONTAINER" \
  -e POSTGRES_USER=postgres -e POSTGRES_PASSWORD=verify \
  -e POSTGRES_DB=postgres \
  -e FLEET_OWNER_PASSWORD="$OWNER_PW" -e FLEET_APP_PASSWORD="$APP_PW" \
  -v "$(hostpath "$ROOT/deploy/postgres/init"):/docker-entrypoint-initdb.d:ro" \
  "$PGIMAGE" >/dev/null

echo -n "==> waiting for postgres"
for _ in $(seq 1 60); do
  if docker exec "$CONTAINER" pg_isready -U postgres -d fleet >/dev/null 2>&1; then break; fi
  echo -n .; sleep 1
done
echo

# Run SQL as fleet_owner (the role that owns the schema), exactly like the migrate service.
psql_owner() { docker exec -i -e PGPASSWORD="$OWNER_PW" "$CONTAINER" \
  psql -v ON_ERROR_STOP=1 -U fleet_owner -d fleet -q "$@"; }

apply() { # apply <file>
  echo "    - $(basename "$1")"
  psql_owner -f - < "$1"
}

up_all() {
  for f in "$ROOT"/db/migrations/*.up.sql; do apply "$f"; done
}
down_all() {
  for f in $(ls -r "$ROOT"/db/migrations/*.down.sql); do apply "$f"; done
}

echo "==> up"
up_all

echo "==> smoke test (invariants)"
psql_owner -f - < "$ROOT/db/testdata/constraints_smoke.sql"

echo "==> checking least-privilege grants for fleet_app"
psql_owner -c "
SELECT table_name, string_agg(privilege_type, ',' ORDER BY privilege_type) AS granted
FROM information_schema.role_table_grants
WHERE grantee = 'fleet_app' GROUP BY table_name ORDER BY table_name;"
psql_owner -c "
DO \$\$
BEGIN
  IF EXISTS (SELECT 1 FROM information_schema.role_table_grants
             WHERE grantee = 'fleet_app' AND table_name = 'audit_log'
               AND privilege_type IN ('UPDATE','DELETE')) THEN
    RAISE EXCEPTION 'audit_log is not append-only for fleet_app';
  END IF;
END \$\$;"

echo "==> down"
down_all

echo "==> up again"
up_all

echo "==> leftover objects after a full down/up cycle (expect the same schema, no error)"
psql_owner -c "\dt"

echo "OK - migrations apply, reverse and re-apply cleanly; invariants hold."
