-- name: CreateSession :one
INSERT INTO sessions (token_hash, user_id, csrf_secret, expires_at)
VALUES ($1, $2, $3, $4)
RETURNING token_hash, user_id, csrf_secret, created_at, last_seen_at, expires_at;

-- name: GetAndTouchSession :one
-- Resolves a session and refreshes its idle clock in one round trip. Expiry is enforced
-- here, in SQL, not only in Go:
--   s.expires_at  > now()  -> absolute expiry
--   s.last_seen_at > $2    -> idle expiry ($2 = now() - idle timeout)
--   u.is_active            -> deactivating a user kills their live sessions immediately
UPDATE sessions s
SET last_seen_at = now()
FROM users u
WHERE s.token_hash = $1
  AND u.id = s.user_id
  AND s.expires_at > now()
  AND s.last_seen_at > $2
  AND u.is_active
RETURNING s.token_hash, s.user_id, s.csrf_secret, s.created_at, s.last_seen_at, s.expires_at,
          u.email, u.role, u.driver_id, u.locale;

-- name: DeleteSession :exec
DELETE FROM sessions WHERE token_hash = $1;

-- name: DeleteSessionsForUser :exec
-- Used on password change, deactivation and "log out everywhere".
DELETE FROM sessions WHERE user_id = $1;

-- name: DeleteExpiredSessions :execrows
-- Background purge; $1 = now() - idle timeout.
DELETE FROM sessions WHERE expires_at <= now() OR last_seen_at <= $1;
