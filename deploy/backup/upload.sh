#!/bin/sh
# Off-site upload hook, called with the path of each finished, encrypted backup.
#
# TODO: copy "$1" to EU-located object storage (GDPR: data must stay in the EU), e.g.
#   rclone copyto "$1" "eu-remote:bus-fleet-backups/$(basename "$1")"
# and configure the bucket's lifecycle to delete objects after BACKUP_KEEP_DAYS.
# The backup container currently sits on the internal network only; uploading will
# also need an egress network in docker-compose.yml.
set -eu
echo "backup: off-site upload not configured yet; $(basename "$1") kept locally only"
