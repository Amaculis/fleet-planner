-- 0005: append-only audit log of every mutation. Kept separate from app logs.
CREATE TABLE audit_log (
    id             bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    -- NULL only for system actions (seed/CLI). Users are never hard-deleted.
    actor_user_id  bigint      NULL REFERENCES users (id) ON DELETE RESTRICT,
    action         text        NOT NULL CHECK (action ~ '^[a-z][a-z_]{1,39}$'),   -- create, update, delete, login, logout, anonymize, ...
    entity         text        NOT NULL CHECK (entity ~ '^[a-z][a-z_]{1,39}$'),   -- trip, assignment, bus, driver, user, session
    entity_id      bigint      NULL,
    before         jsonb       NULL,
    after          jsonb       NULL,
    -- ip is personal data (GDPR): kept for security forensics, subject to retention.
    ip             inet        NULL,
    request_id     text        NULL CHECK (length(request_id) <= 64),
    created_at     timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX audit_log_entity_idx  ON audit_log (entity, entity_id, created_at DESC);
CREATE INDEX audit_log_actor_idx   ON audit_log (actor_user_id, created_at DESC);
CREATE INDEX audit_log_created_idx ON audit_log (created_at DESC);

-- Append-only for the app: no UPDATE, no DELETE. A compromised app cannot rewrite
-- history. GDPR redaction of snapshot PII (driver erasure) will be a narrowly-scoped
-- SECURITY DEFINER function owned by the schema owner, added with the erasure feature.
GRANT SELECT, INSERT ON audit_log TO fleet_app;
