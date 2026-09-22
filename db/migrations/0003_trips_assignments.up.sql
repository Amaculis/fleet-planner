-- 0003: trips, assignments, and the no-double-booking invariant.

CREATE TYPE trip_status AS ENUM ('planned', 'in_progress', 'completed', 'cancelled');

-- ---------------------------------------------------------------------------
-- trips
-- ---------------------------------------------------------------------------
CREATE TABLE trips (
    id               bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    origin           text        NOT NULL CHECK (length(btrim(origin)) BETWEEN 1 AND 200),
    destination      text        NOT NULL CHECK (length(btrim(destination)) BETWEEN 1 AND 200),
    scheduled_start  timestamptz NOT NULL,
    scheduled_end    timestamptz NOT NULL,
    -- payroll-seam: actual worked time, recorded by the driver. Payroll sums
    -- (actual_end - actual_start) for completed trips per assigned driver.
    actual_start     timestamptz NULL,
    actual_end       timestamptz NULL,
    status           trip_status NOT NULL DEFAULT 'planned',
    notes            text        NULL CHECK (length(notes) <= 2000),
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT trips_scheduled_range CHECK (scheduled_end > scheduled_start),
    -- An end needs a start, and must come after it.
    CONSTRAINT trips_actual_range CHECK (
        actual_end IS NULL OR (actual_start IS NOT NULL AND actual_end > actual_start)
    ),
    -- A trip that hasn't started can't have actual times.
    CONSTRAINT trips_actual_needs_status CHECK (
        status NOT IN ('planned') OR (actual_start IS NULL AND actual_end IS NULL)
    ),
    -- Target of the composite FK from assignments (see below). Redundant with the PK
    -- for uniqueness, but Postgres requires a unique constraint to reference.
    CONSTRAINT trips_booking_key UNIQUE (id, scheduled_start, scheduled_end, status)
);
CREATE INDEX trips_scheduled_start_idx ON trips (scheduled_start);
CREATE TRIGGER trips_set_updated_at BEFORE UPDATE ON trips
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------------------------------------------------------------------------
-- assignments — one bus + one driver per trip.
--
-- The EXCLUDE constraints below need each assignment's time window and trip status on
-- the same row. Those columns are copies of the trip's, kept correct *declaratively*
-- by a composite foreign key with ON UPDATE CASCADE:
--   * on insert the copy must match the trip exactly, or the FK rejects it;
--   * when a trip is rescheduled or its status changes, Postgres cascades the new
--     values here, and the EXCLUDE constraints re-check — so rescheduling a trip into
--     a clash fails too, not only creating an assignment.
-- No trigger or app code can let the copy drift.
-- ---------------------------------------------------------------------------
CREATE TABLE assignments (
    id               bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    trip_id          bigint      NOT NULL,
    bus_id           bigint      NOT NULL REFERENCES buses   (id) ON DELETE RESTRICT,
    -- payroll-seam: this row is what ties a driver to a trip's worked time.
    -- RESTRICT + no hard-delete of drivers means the linkage is never lost.
    driver_id        bigint      NOT NULL REFERENCES drivers (id) ON DELETE RESTRICT,
    scheduled_start  timestamptz NOT NULL,
    scheduled_end    timestamptz NOT NULL,
    trip_status      trip_status NOT NULL,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT assignments_trip_id_key UNIQUE (trip_id),
    CONSTRAINT assignments_trip_fk
        FOREIGN KEY (trip_id, scheduled_start, scheduled_end, trip_status)
        REFERENCES trips (id, scheduled_start, scheduled_end, status)
        ON UPDATE CASCADE ON DELETE RESTRICT,

    -- The invariant. Half-open '[)' so back-to-back trips (one ends 10:00, next starts
    -- 10:00) are allowed. Cancelled trips don't hold the bus/driver.
    -- The service layer checks the same thing first for a friendly error; these are
    -- the backstop that makes double-booking impossible even with a bug or a race.
    CONSTRAINT assignments_no_bus_overlap EXCLUDE USING gist (
        bus_id WITH =,
        tstzrange(scheduled_start, scheduled_end, '[)') WITH &&
    ) WHERE (trip_status <> 'cancelled'),
    CONSTRAINT assignments_no_driver_overlap EXCLUDE USING gist (
        driver_id WITH =,
        tstzrange(scheduled_start, scheduled_end, '[)') WITH &&
    ) WHERE (trip_status <> 'cancelled')
);
CREATE INDEX assignments_bus_id_idx    ON assignments (bus_id);
CREATE INDEX assignments_driver_id_idx ON assignments (driver_id);
CREATE TRIGGER assignments_set_updated_at BEFORE UPDATE ON assignments
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- payroll-seam: once a trip is completed, who drove it (and which bus) is a fact that
-- payroll depends on. Block deleting or re-pointing that assignment. To correct a
-- mistake, first move the trip back out of 'completed' (audited), then fix it.
-- Compares values rather than using "UPDATE OF col", because the FK cascade rewrites
-- trip_id in its SET list even when the value is unchanged.
CREATE FUNCTION assignments_lock_completed() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF OLD.trip_status = 'completed' AND (
        TG_OP = 'DELETE'
        OR (NEW.trip_id, NEW.bus_id, NEW.driver_id)
           IS DISTINCT FROM (OLD.trip_id, OLD.bus_id, OLD.driver_id)
    ) THEN
        RAISE EXCEPTION 'assignment % belongs to a completed trip and is locked', OLD.id
            USING ERRCODE = 'check_violation', CONSTRAINT = 'assignments_completed_locked';
    END IF;
    IF TG_OP = 'DELETE' THEN
        RETURN OLD;
    END IF;
    RETURN NEW;
END;
$$;
CREATE TRIGGER assignments_lock_completed BEFORE UPDATE OR DELETE ON assignments
    FOR EACH ROW EXECUTE FUNCTION assignments_lock_completed();

-- Trips are cancelled rather than deleted once they matter; DELETE is allowed so a
-- dispatcher can remove a mistaken planned trip (the FK blocks it while assigned).
GRANT SELECT, INSERT, UPDATE, DELETE ON trips       TO fleet_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON assignments TO fleet_app;
