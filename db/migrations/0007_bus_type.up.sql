-- 0007: bus type (route category), for planning which vehicle suits which trip.

CREATE TYPE bus_type AS ENUM ('tourist', 'international', 'suburban');

ALTER TABLE buses
    ADD COLUMN type bus_type NOT NULL DEFAULT 'tourist';
