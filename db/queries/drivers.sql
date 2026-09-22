-- name: ListDrivers :many
SELECT id, full_name, phone, license_number, license_expiry, hourly_rate, pay_type,
       is_active, anonymized_at, created_at, updated_at
FROM drivers
ORDER BY full_name;

-- name: ListActiveDrivers :many
-- Candidates for an assignment.
SELECT id, full_name, phone, license_number, license_expiry, hourly_rate, pay_type,
       is_active, anonymized_at, created_at, updated_at
FROM drivers
WHERE is_active
ORDER BY full_name;

-- name: GetDriver :one
SELECT id, full_name, phone, license_number, license_expiry, hourly_rate, pay_type,
       is_active, anonymized_at, created_at, updated_at
FROM drivers
WHERE id = $1;

-- name: CreateDriver :one
-- payroll-seam: hourly_rate and pay_type are accepted and stored but no code reads them
-- in the MVP. Payroll later = worked hours (trips.actual_*) x rate, no migration needed.
INSERT INTO drivers (full_name, phone, license_number, license_expiry, hourly_rate, pay_type)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, full_name, phone, license_number, license_expiry, hourly_rate, pay_type,
          is_active, anonymized_at, created_at, updated_at;

-- name: UpdateDriver :one
UPDATE drivers
SET full_name = $2, phone = $3, license_number = $4, license_expiry = $5,
    hourly_rate = $6, pay_type = $7, is_active = $8
WHERE id = $1 AND anonymized_at IS NULL
RETURNING id, full_name, phone, license_number, license_expiry, hourly_rate, pay_type,
          is_active, anonymized_at, created_at, updated_at;

-- name: AnonymizeDriver :one
-- GDPR erasure. The row survives so assignments, worked time and the audit trail keep
-- their referential integrity (payroll-seam); only the identifying fields are dropped.
-- Idempotent: a second call matches no row and returns no result.
UPDATE drivers
SET full_name = $2, phone = NULL, license_number = NULL, license_expiry = NULL,
    is_active = false, anonymized_at = now()
WHERE id = $1 AND anonymized_at IS NULL
RETURNING id, full_name, phone, license_number, license_expiry, hourly_rate, pay_type,
          is_active, anonymized_at, created_at, updated_at;

-- name: GetDriverLoginUserID :one
-- The login account tied to a driver, if any; used to revoke sessions on erasure.
SELECT id FROM users WHERE driver_id = $1;
