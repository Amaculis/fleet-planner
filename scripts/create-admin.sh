#!/usr/bin/env bash
# Creates the first admin account from ADMIN_EMAIL / ADMIN_PASSWORD in .env.
#
# DEV-ONLY — DO NOT USE THIS PATTERN IN PRODUCTION.
# Storing an admin password in a file (even a gitignored one) is a convenience for a
# single local dev machine, not a production practice. In production, run
# `create-admin` directly with the password typed or piped in at that moment:
#
#   docker compose run --rm -e ADMIN_EMAIL=you@company.com -e ADMIN_PASSWORD='...' app create-admin
#
# and never write the password to a file, a backup, or shell history.
#
# Usage: scripts/create-admin.sh
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

if [ ! -f .env ]; then
  echo "error: .env not found (copy .env.example to .env first)" >&2
  exit 1
fi

# Only ADMIN_EMAIL / ADMIN_PASSWORD are read from .env here; every other variable in
# it is picked up by `docker compose` itself via its own .env handling.
ADMIN_EMAIL="$(grep -E '^ADMIN_EMAIL=' .env | tail -1 | cut -d= -f2-)"
ADMIN_PASSWORD="$(grep -E '^ADMIN_PASSWORD=' .env | tail -1 | cut -d= -f2-)"

if [ -z "$ADMIN_EMAIL" ] || [ -z "$ADMIN_PASSWORD" ]; then
  echo "error: set ADMIN_EMAIL and ADMIN_PASSWORD in .env first (dev-only — see the comment above them)" >&2
  exit 1
fi

docker compose run --rm -e ADMIN_EMAIL="$ADMIN_EMAIL" -e ADMIN_PASSWORD="$ADMIN_PASSWORD" app create-admin
