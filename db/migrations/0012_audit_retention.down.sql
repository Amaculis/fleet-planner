-- Removes the function only; rows it already purged are gone (retention is one-way).
DROP FUNCTION IF EXISTS purge_old_audit_log();
