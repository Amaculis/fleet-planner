-- name: InsertAuditEntry :one
-- Written inside the same transaction as the mutation it describes, so a mutation can
-- never be committed without its audit row. The app role has INSERT/SELECT only.
INSERT INTO audit_log (actor_user_id, action, entity, entity_id, before, after, ip, request_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id, created_at;

-- name: ListAuditEntriesForEntity :many
SELECT id, actor_user_id, action, entity, entity_id, before, after, ip, request_id, created_at
FROM audit_log
WHERE entity = $1 AND entity_id = $2
ORDER BY created_at DESC
LIMIT $3;

-- name: RedactAuditPII :one
-- GDPR erasure: strips personal-data keys from one entity's snapshots (see 0011).
SELECT redact_audit_pii(@entity::text, @entity_id::bigint)::bigint AS redacted;

-- name: PurgeOldAuditLog :one
-- Retention: deletes entries older than the fixed period in 0012.
SELECT purge_old_audit_log()::bigint AS purged;
