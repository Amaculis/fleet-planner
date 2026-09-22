-- 0004: server-side sessions (opaque tokens).
--
-- The cookie holds a random 32-byte token; only its SHA-256 is stored, so a DB read
-- (backup leak, SQL bug) does not yield usable session cookies.
CREATE TABLE sessions (
    token_hash    bytea       PRIMARY KEY CHECK (length(token_hash) = 32),
    user_id       bigint      NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    -- Per-session CSRF secret; the synchronizer token is derived from it.
    csrf_secret   bytea       NOT NULL CHECK (length(csrf_secret) = 32),
    created_at    timestamptz NOT NULL DEFAULT now(),
    last_seen_at  timestamptz NOT NULL DEFAULT now(),  -- idle expiry is measured from this
    expires_at    timestamptz NOT NULL,                -- absolute expiry, fixed at login
    CONSTRAINT sessions_expiry CHECK (expires_at > created_at)
);
-- "Log out everywhere" / deactivating a user deletes by user_id.
CREATE INDEX sessions_user_id_idx    ON sessions (user_id);
-- Periodic purge of expired rows.
CREATE INDEX sessions_expires_at_idx ON sessions (expires_at);

GRANT SELECT, INSERT, UPDATE, DELETE ON sessions TO fleet_app;
