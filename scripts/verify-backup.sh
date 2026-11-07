#!/usr/bin/env bash
# Proves a backup can be restored: decrypts it, restores it into a throwaway Postgres
# container and prints row counts. Nothing touches the real database.
#
# Usage: scripts/verify-backup.sh <backup.dump.age> <age identity file>
set -euo pipefail
export MSYS_NO_PATHCONV=1

BACKUP="${1:?usage: verify-backup.sh <backup.dump.age> <age identity file>}"
IDENTITY="${2:?usage: verify-backup.sh <backup.dump.age> <age identity file>}"
PGIMAGE="postgres:17.11-alpine@sha256:3540c0362c809df084ffac0ae69fe794515f9de92692f02178bcdce0ef0302ac"
PG="bus-fleet-restore-$$"

hostpath() {
  if command -v cygpath >/dev/null 2>&1; then cygpath -m "$1"; else printf '%s' "$1"; fi
}
cleanup() { docker rm -f "$PG" >/dev/null 2>&1 || true; }
trap cleanup EXIT

echo "==> starting throwaway postgres"
docker run -d --name "$PG" -e POSTGRES_PASSWORD=verify -e POSTGRES_DB=fleet \
  -v "$(hostpath "$(cd "$(dirname "$BACKUP")" && pwd)"):/in:ro" \
  -v "$(hostpath "$(cd "$(dirname "$IDENTITY")" && pwd)"):/key:ro" \
  "$PGIMAGE" >/dev/null
for _ in $(seq 1 60); do
  docker exec "$PG" pg_isready -h 127.0.0.1 -U postgres -d fleet >/dev/null 2>&1 && break
  sleep 1
done

echo "==> decrypting and restoring $(basename "$BACKUP")"
docker exec "$PG" sh -c "apk add --no-cache age >/dev/null && \
  age -d -i '/key/$(basename "$IDENTITY")' '/in/$(basename "$BACKUP")' > /tmp/fleet.dump && \
  pg_restore -U postgres -d fleet --no-owner --no-privileges --exit-on-error /tmp/fleet.dump"

echo "==> row counts"
docker exec "$PG" psql -U postgres -d fleet -At -c "
  SELECT 'trips', count(*) FROM trips UNION ALL
  SELECT 'assignments', count(*) FROM assignments UNION ALL
  SELECT 'drivers', count(*) FROM drivers UNION ALL
  SELECT 'buses', count(*) FROM buses UNION ALL
  SELECT 'users', count(*) FROM users UNION ALL
  SELECT 'audit_log', count(*) FROM audit_log;"
echo "OK - backup restored successfully."
