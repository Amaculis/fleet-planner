-- 0009: recurring trips. A trip_series is the weekly pattern a planner describes once
-- ("every Monday 8-10am until <date>"); concrete trips are generated eagerly for the
-- whole range and linked back via trips.series_id — no background job, no open-ended
-- recurrence. Editing/cancelling "this and future" bulk-operates on the generated rows;
-- it never touches this table's own fields.

CREATE TABLE trip_series (
    id             bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    origin         text        NOT NULL CHECK (length(btrim(origin)) BETWEEN 1 AND 200),
    destination    text        NOT NULL CHECK (length(btrim(destination)) BETWEEN 1 AND 200),
    -- ISO weekday numbers (1=Monday .. 7=Sunday), at least one, no duplicates/out-of-range.
    days_of_week   integer[]   NOT NULL,
    -- Anchors the series: its date is the first occurrence's date, its time-of-day and
    -- (first_end - first_start) duration are reused for every later occurrence.
    first_start    timestamptz NOT NULL,
    first_end      timestamptz NOT NULL,
    -- Exclusive cutoff: no occurrence starts at or after this instant. Capped to one year
    -- out so a series can never silently generate an unbounded number of trips.
    ends_on        timestamptz NOT NULL,
    payment_status payment_status NOT NULL DEFAULT 'unpaid',
    notes          text        NULL CHECK (length(notes) <= 2000),
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT trip_series_range CHECK (first_end > first_start),
    -- Duplicates aren't rejected here (a CHECK can't use a subquery to count distinct
    -- elements) — the service layer de-dupes before insert (see validateTripSeries).
    CONSTRAINT trip_series_days_valid CHECK (
        array_length(days_of_week, 1) > 0
        AND days_of_week <@ ARRAY[1,2,3,4,5,6,7]
    ),
    CONSTRAINT trip_series_horizon CHECK (
        ends_on > first_start AND ends_on <= first_start + INTERVAL '1 year'
    )
);
CREATE TRIGGER trip_series_set_updated_at BEFORE UPDATE ON trip_series
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ON DELETE SET NULL: a series row is never deleted by this app, but if it ever were,
-- the trips it generated should keep existing as plain trips rather than disappear.
ALTER TABLE trips ADD COLUMN series_id bigint NULL REFERENCES trip_series (id) ON DELETE SET NULL;
CREATE INDEX trips_series_id_idx ON trips (series_id);

GRANT SELECT, INSERT, UPDATE, DELETE ON trip_series TO fleet_app;
