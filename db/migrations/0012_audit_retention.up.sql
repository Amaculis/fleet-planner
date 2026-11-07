-- 0012: retention for audit_log (GDPR storage limitation). Entries older than two years
-- are deleted; they hold IP addresses and personal-data snapshots.
--
-- Same pattern as 0011: the app role still has no DELETE on audit_log. It may only call
-- this function, and the retention period is fixed here, not passed in — so even a
-- compromised app cannot use it to wipe recent history.
CREATE FUNCTION purge_old_audit_log()
RETURNS bigint
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = public, pg_temp
AS $$
DECLARE
    n bigint;
BEGIN
    DELETE FROM audit_log WHERE created_at < now() - interval '2 years';
    GET DIAGNOSTICS n = ROW_COUNT;
    RETURN n;
END;
$$;

REVOKE ALL ON FUNCTION purge_old_audit_log() FROM PUBLIC;
GRANT EXECUTE ON FUNCTION purge_old_audit_log() TO fleet_app;
