-- Smoke test for the schema invariants. Run against a throwaway database
-- (see scripts/verify-schema.sh). Fails loudly via RAISE if an invariant is missing.
-- These same cases get proper Go table-driven tests in step 3.
\set ON_ERROR_STOP on
BEGIN;

CREATE FUNCTION pg_temp.must_fail(sql text, what text) RETURNS void
LANGUAGE plpgsql AS $$
BEGIN
    BEGIN
        EXECUTE sql;
    EXCEPTION WHEN others THEN
        RAISE NOTICE 'ok   (rejected: %) [%]', what, SQLSTATE;
        RETURN;
    END;
    RAISE EXCEPTION 'INVARIANT MISSING: % was accepted', what;
END;
$$;

INSERT INTO buses (plate, model, seats) VALUES ('AA-1111', 'Setra S515', 55), ('BB-2222', 'MAN Lions', 49);
INSERT INTO drivers (full_name, phone, license_number) VALUES ('Driver One', '+37120000001', 'LV-1'), ('Driver Two', '+37120000002', 'LV-2');

INSERT INTO trips (id, origin, destination, scheduled_start, scheduled_end) OVERRIDING SYSTEM VALUE VALUES
    (1, 'Riga', 'Liepaja', '2026-10-01 08:00+00', '2026-10-01 12:00+00'),
    (2, 'Riga', 'Ventspils','2026-10-01 10:00+00', '2026-10-01 14:00+00'),  -- overlaps trip 1
    (3, 'Liepaja','Riga',   '2026-10-01 12:00+00', '2026-10-01 16:00+00'),  -- back-to-back with 1
    (4, 'Riga', 'Jelgava',  '2026-10-01 09:00+00', '2026-10-01 11:00+00');  -- overlaps, to be cancelled
-- Explicit ids above don't advance the identity sequence; move it past them so later
-- inserts fail on the constraint under test, not on a duplicate key.
SELECT setval(pg_get_serial_sequence('trips', 'id'), 100);

-- Helper: assignments copy the trip's window; the composite FK guarantees the copy matches.
CREATE FUNCTION pg_temp.assign(p_trip bigint, p_bus bigint, p_driver bigint) RETURNS void
LANGUAGE sql AS $$
    INSERT INTO assignments (trip_id, bus_id, driver_id, scheduled_start, scheduled_end, trip_status)
    SELECT t.id, p_bus, p_driver, t.scheduled_start, t.scheduled_end, t.status FROM trips t WHERE t.id = p_trip;
$$;

SELECT pg_temp.assign(1, 1, 1);

-- 1. Same bus, overlapping window -> rejected.
SELECT pg_temp.must_fail($$SELECT pg_temp.assign(2, 1, 2)$$, 'bus double-booked');
-- 2. Same driver, overlapping window -> rejected.
SELECT pg_temp.must_fail($$SELECT pg_temp.assign(2, 2, 1)$$, 'driver double-booked');
-- 3. Different bus + driver, overlapping window -> allowed.
SELECT pg_temp.assign(2, 2, 2);
-- 4. Back-to-back ([) boundary): trip 1 ends 12:00, trip 3 starts 12:00 -> allowed.
SELECT pg_temp.assign(3, 1, 1);
-- 5. The denormalised window cannot drift from the trip (composite FK).
SELECT pg_temp.must_fail($$
    INSERT INTO assignments (trip_id, bus_id, driver_id, scheduled_start, scheduled_end, trip_status)
    VALUES (4, 1, 1, '2026-10-02 09:00+00', '2026-10-02 11:00+00', 'planned')$$,
    'assignment window not matching its trip');
-- 6. One assignment per trip.
SELECT pg_temp.must_fail($$SELECT pg_temp.assign(1, 2, 2)$$, 'second assignment for the same trip');
-- 7. A cancelled trip releases its bus and driver.
UPDATE trips SET status = 'cancelled' WHERE id = 4;
SELECT pg_temp.assign(4, 1, 1);          -- overlaps trip 1, but cancelled -> allowed
SELECT pg_temp.must_fail($$UPDATE trips SET status = 'planned' WHERE id = 4$$,
    'un-cancelling a trip back into a clash');
-- 8. Rescheduling an assigned trip into a clash is rejected (FK cascade re-checks EXCLUDE).
SELECT pg_temp.must_fail($$UPDATE trips SET scheduled_start = '2026-10-01 11:00+00' WHERE id = 3$$,
    'rescheduling a trip into a clash');
-- 9. payroll-seam: a completed trip's assignment is locked.
UPDATE trips SET status = 'in_progress', actual_start = '2026-10-01 08:05+00' WHERE id = 1;
UPDATE trips SET status = 'completed',   actual_end   = '2026-10-01 12:10+00' WHERE id = 1;
SELECT pg_temp.must_fail($$DELETE FROM assignments WHERE trip_id = 1$$, 'deleting a completed trip''s assignment');
SELECT pg_temp.must_fail($$UPDATE assignments SET driver_id = 2 WHERE trip_id = 1$$, 'repointing a completed assignment');
SELECT pg_temp.must_fail($$DELETE FROM trips WHERE id = 1$$, 'deleting an assigned trip');
SELECT pg_temp.must_fail($$DELETE FROM drivers WHERE id = 1$$, 'deleting a driver with assignments');

-- 10. Field-level invariants.
SELECT pg_temp.must_fail($$INSERT INTO trips (origin, destination, scheduled_start, scheduled_end)
    VALUES ('A','B','2026-10-01 12:00+00','2026-10-01 08:00+00')$$, 'trip ending before it starts');
SELECT pg_temp.must_fail($$UPDATE trips SET actual_end = '2026-10-01 07:00+00' WHERE id = 1$$,
    'actual_end before actual_start');
SELECT pg_temp.must_fail($$INSERT INTO trips (origin, destination, scheduled_start, scheduled_end, actual_start)
    VALUES ('A','B','2026-10-01 08:00+00','2026-10-01 09:00+00','2026-10-01 08:00+00')$$,
    'actual times on a planned trip');
SELECT pg_temp.must_fail($$INSERT INTO buses (plate, model, seats) VALUES ('aa-9999','X',10)$$, 'unnormalised plate');
SELECT pg_temp.must_fail($$INSERT INTO buses (plate, model, seats) VALUES ('AA-1111','X',10)$$, 'duplicate plate');
SELECT pg_temp.must_fail($$INSERT INTO users (email, password_hash, role)
    VALUES ('Admin@Example.com','$argon2id$v=19$x','admin')$$, 'non-lowercased email');
SELECT pg_temp.must_fail($$INSERT INTO users (email, password_hash, role)
    VALUES ('a@example.com','plaintext','admin')$$, 'password not an argon2id hash');
SELECT pg_temp.must_fail($$INSERT INTO users (email, password_hash, role)
    VALUES ('b@example.com','$argon2id$v=19$x','driver')$$, 'driver login without driver_id');
SELECT pg_temp.must_fail($$INSERT INTO users (email, password_hash, role, driver_id)
    VALUES ('c@example.com','$argon2id$v=19$x','admin', 1)$$, 'non-driver login with driver_id');
SELECT pg_temp.must_fail($$UPDATE drivers SET anonymized_at = now() WHERE id = 1$$,
    'anonymised driver still holding PII');

-- 11. GDPR erasure path works while keeping the payroll/audit linkage.
UPDATE drivers SET full_name = 'Erased driver #1', phone = NULL, license_number = NULL,
                   license_expiry = NULL, is_active = false, anonymized_at = now()
WHERE id = 1;
SELECT count(*) AS assignments_kept_for_driver_1 FROM assignments WHERE driver_id = 1;

ROLLBACK;
