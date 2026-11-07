#!/bin/sh
# Scheduled, encrypted logical backups of the fleet database.
#
#   pg_dump (custom format) | age -r <public key>  ->  /backups/fleet-<UTC time>.dump.age
#
# Only the age *public* key lives on the server, so a stolen backup volume (or server)
# does not expose the data. The private key is kept offline and is needed only to
# restore (see README.md and scripts/verify-backup.sh).
set -eu
set -o pipefail

INTERVAL="${BACKUP_INTERVAL_SECONDS:-86400}"
KEEP_DAYS="${BACKUP_KEEP_DAYS:-30}"
DIR=/backups

log() { printf '%s backup: %s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$*"; }

run_once() {
  ts="$(date -u +%Y%m%dT%H%M%SZ)"
  final="$DIR/fleet-$ts.dump.age"
  tmp="$final.partial"
  # A partial file never looks like a finished backup: written under .partial and
  # renamed only once pg_dump and age have both succeeded.
  if pg_dump --format=custom --no-owner --no-privileges | age -r "$BACKUP_AGE_RECIPIENT" -o "$tmp"; then
    mv "$tmp" "$final"
    log "wrote $(basename "$final") ($(wc -c <"$final") bytes)"
  else
    rm -f "$tmp"
    log "FAILED"
    return 1
  fi

  # Local rotation. Off-site copies follow their own retention (configure it there
  # with the same KEEP_DAYS so erased personal data ages out everywhere).
  find "$DIR" -name 'fleet-*.dump.age' -type f -mtime +"$KEEP_DAYS" -print -delete |
    while read -r f; do log "rotated out $(basename "$f")"; done

  /usr/local/bin/upload.sh "$final"
}

# Never write an unencrypted dump. Without a key, say so loudly and keep saying it.
if [ -z "${BACKUP_AGE_RECIPIENT:-}" ]; then
  while true; do
    log "NOT CONFIGURED: set BACKUP_AGE_RECIPIENT (age public key) - no backups are being taken"
    sleep 3600
  done
fi

log "starting: every ${INTERVAL}s, keep ${KEEP_DAYS} days"
while true; do
  run_once || true
  sleep "$INTERVAL"
done
