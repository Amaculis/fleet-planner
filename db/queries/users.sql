-- name: GetUserByEmail :one
-- Login lookup. The handler always runs an argon2id verification afterwards, even when
-- no row comes back, so response time does not reveal whether the email exists.
SELECT id, email, password_hash, role, driver_id, locale, is_active, created_at, updated_at
FROM users
WHERE email = $1;

-- name: GetUserByID :one
SELECT id, email, password_hash, role, driver_id, locale, is_active, created_at, updated_at
FROM users
WHERE id = $1;

-- name: SetUserLocale :exec
UPDATE users SET locale = $2 WHERE id = $1;

-- name: CreateUser :one
-- Used by the create-admin bootstrap command and, from step 3, by admin user
-- management. driver_id must be non-null exactly when role = 'driver' (DB-enforced).
INSERT INTO users (email, password_hash, role, driver_id, locale)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, email, password_hash, role, driver_id, locale, is_active, created_at, updated_at;

-- name: CountUsers :one
SELECT count(*) FROM users;

-- name: ListUsers :many
SELECT u.id, u.email, u.password_hash, u.role, u.driver_id, u.locale, u.is_active,
       u.created_at, u.updated_at, d.full_name AS driver_name
FROM users u
LEFT JOIN drivers d ON d.id = u.driver_id
ORDER BY u.email;

-- name: SetUserActive :one
-- Users are deactivated, never deleted: audit_log.actor_user_id references them.
-- The service deletes their sessions in the same transaction.
UPDATE users SET is_active = $2
WHERE id = $1
RETURNING id, email, password_hash, role, driver_id, locale, is_active, created_at, updated_at;

-- name: SetUserPassword :one
UPDATE users SET password_hash = $2
WHERE id = $1
RETURNING id, email, password_hash, role, driver_id, locale, is_active, created_at, updated_at;
