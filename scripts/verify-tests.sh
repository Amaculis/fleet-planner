#!/usr/bin/env bash
# Runs the whole Go test suite, including the integration tests in internal/inttest,
# against a throwaway PostgreSQL instance with the migrations applied.
#
# Needs only Docker. The database and containers are removed at the end.
#
# Usage: scripts/verify-tests.sh [extra go test flags]
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
export MSYS_NO_PATHCONV=1

hostpath() {
  if command -v cygpath >/dev/null 2>&1; then cygpath -m "$1"; else printf '%s' "$1"; fi
}

NET="bus-fleet-test-$$"
PG="bus-fleet-testdb-$$"
PGIMAGE="postgres:17.6-alpine"
GOIMAGE="golang:1.26-alpine"
OWNER_PW="verify-owner"
APP_PW="verify-app"

cleanup() {
  docker rm -f "$PG" >/dev/null 2>&1 || true
  docker network rm "$NET" >/dev/null 2>&1 || true
}
trap cleanup EXIT

echo "==> starting postgres"
docker network create "$NET" >/dev/null
docker run -d --name "$PG" --network "$NET" \
  -e POSTGRES_USER=postgres -e POSTGRES_PASSWORD=verify -e POSTGRES_DB=postgres \
  -e FLEET_OWNER_PASSWORD="$OWNER_PW" -e FLEET_APP_PASSWORD="$APP_PW" \
  -v "$(hostpath "$ROOT/deploy/postgres/init"):/docker-entrypoint-initdb.d:ro" \
  "$PGIMAGE" >/dev/null

echo -n "==> waiting for postgres"
for _ in $(seq 1 60); do
  if docker exec "$PG" pg_isready -U postgres -d fleet >/dev/null 2>&1; then break; fi
  echo -n .; sleep 1
done
echo

echo "==> applying migrations"
for f in "$ROOT"/db/migrations/*.up.sql; do
  echo "    - $(basename "$f")"
  docker exec -i -e PGPASSWORD="$OWNER_PW" "$PG" \
    psql -v ON_ERROR_STOP=1 -U fleet_owner -d fleet -q -f - < "$f"
done

# The tests connect as the application role, so they exercise the same least-privilege
# grants production runs with: no DDL, and an append-only audit log.
# TRUNCATE between tests needs table ownership, so the suite connects as the owner —
# the RBAC and conflict behaviour under test does not depend on the DB role.
echo "==> go test ./..."
docker run --rm --network "$NET" \
  -v "$(hostpath "$ROOT"):/src" -v bus-fleet-gomod:/go/pkg/mod -w /src \
  -e TEST_DATABASE_URL="postgres://fleet_owner:$OWNER_PW@$PG:5432/fleet?sslmode=disable" \
  "$GOIMAGE" go test ./... "$@"

echo "OK - unit and integration tests pass against a real database."
