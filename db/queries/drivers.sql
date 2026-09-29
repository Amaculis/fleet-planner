-- Column lists put notes last, matching its physical position from the ALTER TABLE
-- that added it (see 0008_driver_notes.up.sql) — the repo layer converts these row
-- types to sqlcgen.Driver via a plain Go type conversion, which needs identical field
-- order, not just identical field sets (see trips.sql's own copy of this note).

-- name: ListDrivers :many
SELECT id, full_name, phone, license_number, license_expiry, hourly_rate, pay_type,
       is_active, anonymized_at, created_at, updated_at, notes
FROM drivers
ORDER BY full_name;

-- name: ListActiveDrivers :many
-- Candidates for an assignment.
SELECT id, full_name, phone, license_number, license_expiry, hourly_rate, pay_type,
       is_active, anonymized_at, created_at, updated_at, notes
FROM drivers
WHERE is_active
ORDER BY full_name;

-- name: GetDriver :one
SELECT id, full_name, phone, license_number, license_expiry, hourly_rate, pay_type,
       is_active, anonymized_at, created_at, updated_at, notes
FROM drivers
WHERE id = $1;

-- name: CreateDriver :one
-- payroll-seam: hourly_rate and pay_type are accepted and stored but no code reads them
-- in the MVP. Payroll later = worked hours (trips.actual_*) x rate, no migration needed.
INSERT INTO drivers (full_name, phone, license_number, license_expiry, hourly_rate, pay_type, notes)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, full_name, phone, license_number, license_expiry, hourly_rate, pay_type,
          is_active, anonymized_at, created_at, updated_at, notes;

-- name: UpdateDriver :one
UPDATE drivers
SET full_name = $2, phone = $3, license_number = $4, license_expiry = $5,
    hourly_rate = $6, pay_type = $7, is_active = $8, notes = $9
WHERE id = $1 AND anonymized_at IS NULL
RETURNING id, full_name, phone, license_number, license_expiry, hourly_rate, pay_type,
          is_active, anonymized_at, created_at, updated_at, notes;

-- name: AnonymizeDriver :one
-- GDPR erasure. The row survives so assignments, worked time and the audit trail keep
-- their referential integrity (payroll-seam); only the identifying fields are dropped.
-- notes is cleared too — a dispatcher's free-text note about a driver can itself be
-- personal data ("lives near the depot", "prefers not to drive nights due to...").
-- Idempotent: a second call matches no row and returns no result.
UPDATE drivers
SET full_name = $2, phone = NULL, license_number = NULL, license_expiry = NULL,
    is_active = false, anonymized_at = now(), notes = NULL
WHERE id = $1 AND anonymized_at IS NULL
RETURNING id, full_name, phone, license_number, license_expiry, hourly_rate, pay_type,
          is_active, anonymized_at, created_at, updated_at, notes;

-- name: GetDriverLoginUserID :one
-- The login account tied to a driver, if any; used to revoke sessions on erasure.
SELECT id FROM users WHERE driver_id = $1;
