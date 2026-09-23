#!/usr/bin/env bash
# Runs the Playwright e2e suite (portal/e2e) against the already-running stack,
# through Caddy (TLS) — hitting the app container directly over plain HTTP defeats
# the __Host-prefixed session/CSRF cookies (Secure-only), which the browser then
# silently drops, turning every login into a false-positive CSRF failure. See
# playwright.config.mjs for the full story; that's where this was actually found.
#
# Needs the stack already up: docker compose up -d
#
# Usage: scripts/verify-e2e.sh [extra playwright test flags]
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
export MSYS_NO_PATHCONV=1

hostpath() {
  if command -v cygpath >/dev/null 2>&1; then cygpath -m "$1"; else printf '%s' "$1"; fi
}

if ! docker ps --filter "name=bus-fleet-app-1" --filter "status=running" -q | grep -q .; then
  echo "bus-fleet-app-1 is not running. Start the stack first: docker compose up -d" >&2
  exit 1
fi

if [ ! -f .env ]; then
  echo ".env not found — the e2e suite needs ADMIN_EMAIL/ADMIN_PASSWORD to log in" >&2
  exit 1
fi
set -a
source .env
set +a

PLAYWRIGHT_IMAGE="mcr.microsoft.com/playwright:v1.49.1-jammy"

# node_modules is shadowed by a named volume, not the bind mount: `npm install`
# inside this Linux container would otherwise write Linux-symlinked node_modules
# straight onto the Windows host through the bind mount — the same broken-symlink
# problem the Dockerfile build hit earlier (see git history). The named volume keeps
# that entirely inside Docker.
#
# Two separate docker run steps, not one: bus-fleet_frontend is deliberately
# `internal: true` (see docker-compose.yml's own header comment) — no internet
# egress, by design, the same reason app/db can't reach the internet either. `npm
# install` needs the real internet (npmjs.org); `npx playwright test` needs to reach
# `app` by its compose service name. No single network offers both, so install runs
# on the default network first, and only the test run itself attaches to
# bus-fleet_frontend, once node_modules already exists in the shared volume.
echo "==> installing @playwright/test"
docker run --rm \
  -v "$(hostpath "$ROOT/portal"):/work" \
  -v bus-fleet-e2e-node-modules:/work/node_modules \
  -v bus-fleet-e2e-npm-cache:/root/.npm \
  -w /work \
  "$PLAYWRIGHT_IMAGE" \
  npm install --no-audit --no-fund

: "${APP_DOMAIN:?APP_DOMAIN must be set in .env — it's what Caddy's site block and TLS SNI match on}"

echo "==> running e2e suite against https://$APP_DOMAIN (via caddy)"
docker run --rm --network bus-fleet_frontend \
  -v "$(hostpath "$ROOT/portal"):/work" \
  -v bus-fleet-e2e-node-modules:/work/node_modules \
  -w /work \
  -e "E2E_DOMAIN=$APP_DOMAIN" \
  -e "E2E_ADMIN_EMAIL=$ADMIN_EMAIL" \
  -e "E2E_ADMIN_PASSWORD=$ADMIN_PASSWORD" \
  "$PLAYWRIGHT_IMAGE" \
  npx playwright test "$@"

echo "OK - e2e suite passed against the live app container."
