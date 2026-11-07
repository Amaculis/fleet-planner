# Database backups

Status: **template**. Encrypted local backups and rotation work; the off-site
destination (EU object storage) is still to be configured in `upload.sh`.

## How it works

The `backup` service in `docker-compose.yml` runs `backup.sh`:

1. `pg_dump --format=custom` of the `fleet` database.
2. Encrypted with [age](https://age-encryption.org) to a **public** key
   (`BACKUP_AGE_RECIPIENT`). The server cannot decrypt its own backups.
3. Written to the `backups` volume as `fleet-<UTC time>.dump.age`.
4. Files older than `BACKUP_KEEP_DAYS` (default 30) are deleted. Keep this short:
   personal data erased in the app must also age out of backups.
5. `upload.sh` is called with the new file (off-site hook, not configured yet).

Default schedule: every 24 h (`BACKUP_INTERVAL_SECONDS`).

## One-time setup

```sh
age-keygen -o fleet-backup.key      # prints the public key "age1..."
```

- Put the public key in `.env` as `BACKUP_AGE_RECIPIENT=age1...`.
- Store `fleet-backup.key` **offline** (password manager / safe), never on the server.
  Without it no backup can be restored.

## Restore test (do it regularly)

```sh
scripts/verify-backup.sh /path/to/fleet-<time>.dump.age /path/to/fleet-backup.key
```

It restores into a throwaway Postgres container, applies nothing else, and prints row
counts of the main tables. A backup you have never restored is not a backup.

## Real restore (outline)

1. Stop the app: `docker compose stop app`.
2. `age -d -i fleet-backup.key fleet-<time>.dump.age > fleet.dump`
3. `docker compose exec -T db pg_restore -U fleet_owner -d fleet --clean --if-exists < fleet.dump`
4. Start the app and check the dashboard.

## Still to do

- Off-site destination in the EU (`upload.sh`) and an egress network for it.
- Alerting when a backup fails (currently only the container log says `FAILED`).
