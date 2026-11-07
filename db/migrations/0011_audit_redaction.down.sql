-- Removes the function only; snapshots already redacted stay redacted (erasure is
-- irreversible by design).
DROP FUNCTION IF EXISTS redact_audit_pii(text, bigint);
