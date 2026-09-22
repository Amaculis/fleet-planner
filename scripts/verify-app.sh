#!/usr/bin/env bash
# Boots the real stack (db + migrations + app) and runs scripts/smoke-auth.sh against
# it, then tears everything down. Needs only Docker.
#
# The app runs with APP_ENV=development for this check so cookies are issued without
# the Secure attribute over plain HTTP; production keeps Secure + __Host- prefixes.
#
# Usage: scripts/verify-app.sh
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
export MSYS_NO_PATHCONV=1

hostpath() {
  if command -v cygpath >/dev/null 2>&1; then cygpath -m "$1"; else printf '%s' "$1"; fi
}

SMOKE_NAME="fleet-smoke-$$"
ADMIN_EMAIL="smoke-admin@example.lv"
ADMIN_PASSWORD="correct-horse-battery-staple"

cleanup() {
  docker rm -f "$SMOKE_NAME" >/dev/null 2>&1 || true
  docker compose down -v >/dev/null 2>&1 || true
}
trap cleanup EXIT

echo "==> building the app image"
docker compose build app

echo "==> starting the database and applying migrations"
docker compose up -d db
docker compose up migrate

echo "==> creating the smoke-test admin"
docker compose run --rm \
  -e ADMIN_EMAIL="$ADMIN_EMAIL" -e ADMIN_PASSWORD="$ADMIN_PASSWORD" \
  app create-admin

echo "==> starting the app"
docker compose run -d --name "$SMOKE_NAME" \
  -e APP_ENV=development -e APP_BASE_URL=http://localhost:8080 app
sleep 3

APP_IP="$(docker inspect -f '{{(index .NetworkSettings.Networks "bus-fleet_frontend").IPAddress}}' "$SMOKE_NAME")"
echo "==> app at $APP_IP"

echo "==> running the auth smoke test"
# libcurl will not store cookies for a single-label host, so the app is addressed
# through a dotted name pinned with --resolve (via .curlrc).
docker run --rm --network bus-fleet_frontend \
  -v "$(hostpath "$ROOT/scripts"):/work:ro" \
  -e APP_IP="$APP_IP" \
  -e ADMIN_EMAIL="$ADMIN_EMAIL" -e ADMIN_PASSWORD="$ADMIN_PASSWORD" \
  --entrypoint sh curlimages/curl:latest -c \
  'printf "resolve = \"app.test:8080:%s\"\n" "$APP_IP" > /tmp/.curlrc; export CURL_HOME=/tmp; sh /work/smoke-auth.sh'

echo "==> checking the audit trail"
docker compose exec -T db psql -U postgres -d fleet -c \
  "SELECT action, entity, entity_id, ip IS NOT NULL AS has_ip, request_id IS NOT NULL AS has_request_id
     FROM audit_log ORDER BY id;"

echo "OK - the stack boots, migrations apply and the auth pipeline behaves."
