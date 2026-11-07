-- 0011: GDPR redaction of personal data inside audit_log snapshots.
--
-- The app role may only INSERT/SELECT audit_log (see 0005), so a compromised app cannot
-- rewrite history. Erasure still has to remove a driver's / client's / user's personal
-- data from the before/after snapshots. This function is the single, narrow exception:
--   * SECURITY DEFINER, owned by the schema owner, callable by fleet_app only;
--   * it can only strip a fixed list of personal-data keys, for one entity row at a time;
--   * every other column (action, actor, timestamps, ids) is left untouched, so the
--     trail still shows who did what and when — just not to whom, by name.
CREATE FUNCTION redact_audit_pii(p_entity text, p_entity_id bigint)
RETURNS bigint
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = public, pg_temp
AS $$
DECLARE
    pii_keys text[];
    n        bigint;
BEGIN
    pii_keys := CASE p_entity
        WHEN 'driver' THEN ARRAY['full_name', 'phone', 'license_number', 'license_expiry', 'notes']
        WHEN 'client' THEN ARRAY['name', 'contact_person', 'phone', 'email', 'address', 'notes']
        WHEN 'user'   THEN ARRAY['email']
    END;
    IF pii_keys IS NULL OR p_entity_id IS NULL THEN
        RAISE EXCEPTION 'redact_audit_pii: unsupported entity %', p_entity
            USING ERRCODE = '22023'; -- invalid_parameter_value
    END IF;

    UPDATE audit_log
    SET before = CASE WHEN before IS NULL THEN NULL
                      ELSE (before - pii_keys) || '{"redacted": true}'::jsonb END,
        after  = CASE WHEN after IS NULL THEN NULL
                      ELSE (after - pii_keys) || '{"redacted": true}'::jsonb END
    WHERE entity = p_entity
      AND entity_id = p_entity_id
      AND (before ?| pii_keys OR after ?| pii_keys);
    GET DIAGNOSTICS n = ROW_COUNT;
    RETURN n;
END;
$$;

REVOKE ALL ON FUNCTION redact_audit_pii(text, bigint) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION redact_audit_pii(text, bigint) TO fleet_app;
