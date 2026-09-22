-- 0002: core master data — drivers, buses, users.

CREATE TYPE user_role  AS ENUM ('admin', 'dispatcher', 'driver');
CREATE TYPE bus_status AS ENUM ('active', 'maintenance', 'retired');
-- payroll-seam: how a driver is paid. Unused in MVP.
CREATE TYPE pay_type   AS ENUM ('hourly', 'per_trip', 'monthly');

-- ---------------------------------------------------------------------------
-- drivers — personal data (GDPR). Drivers are never hard-deleted: erasure is done by
-- anonymizing in place (anonymized_at set, PII nulled) so assignments, worked time and
-- the audit trail stay referentially intact.
-- ---------------------------------------------------------------------------
CREATE TABLE drivers (
    id              bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    full_name       text        NOT NULL CHECK (length(btrim(full_name)) BETWEEN 1 AND 200),
    phone           text        NULL     CHECK (phone ~ '^\+?[0-9 ()-]{5,32}$'),
    license_number  text        NULL     CHECK (length(btrim(license_number)) BETWEEN 1 AND 64),
    -- documents seam: expiry reminders come later; no logic reads this in MVP.
    license_expiry  date        NULL,

    -- payroll-seam: nullable and unused in MVP. Payroll later =
    --   sum(actual_end - actual_start) over completed trips assigned to the driver × rate.
    -- Rates are assumed EUR. If rate *history* is needed, add a driver_pay_rates
    -- (driver_id, valid_from, rate) table additively; these columns stay as "current".
    hourly_rate     numeric(10,2) NULL   CHECK (hourly_rate >= 0),
    pay_type        pay_type    NULL,

    is_active       boolean     NOT NULL DEFAULT true,
    anonymized_at   timestamptz NULL,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),

    -- An anonymized driver carries no direct identifiers and cannot be active.
    CONSTRAINT drivers_anonymized_has_no_pii CHECK (
        anonymized_at IS NULL
        OR (phone IS NULL AND license_number IS NULL AND license_expiry IS NULL AND NOT is_active)
    )
);
-- NULLs don't collide, so anonymized drivers (license_number NULL) don't conflict.
CREATE UNIQUE INDEX drivers_license_number_key ON drivers (license_number);
CREATE TRIGGER drivers_set_updated_at BEFORE UPDATE ON drivers
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------------------------------------------------------------------------
-- buses
-- ---------------------------------------------------------------------------
CREATE TABLE buses (
    id                bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    -- Stored normalized (upper-case, no spaces) by the app; the CHECK enforces it.
    plate             text        NOT NULL CHECK (plate ~ '^[A-Z0-9-]{2,16}$'),
    model             text        NOT NULL CHECK (length(btrim(model)) BETWEEN 1 AND 120),
    seats             smallint    NOT NULL CHECK (seats BETWEEN 1 AND 120),
    status            bus_status  NOT NULL DEFAULT 'active',
    -- documents seam: no reminder logic in MVP.
    insurance_expiry  date        NULL,
    inspection_expiry date        NULL,
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT buses_plate_key UNIQUE (plate)
);
CREATE TRIGGER buses_set_updated_at BEFORE UPDATE ON buses
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------------------------------------------------------------------------
-- users — login accounts. Deactivated (is_active = false), never hard-deleted, because
-- audit_log.actor_user_id references them.
-- ---------------------------------------------------------------------------
CREATE TABLE users (
    id             bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    -- App lower-cases before insert; CHECK guarantees it so the UNIQUE is case-insensitive.
    email          text        NOT NULL CHECK (email = lower(email) AND email ~ '^[^@\s]+@[^@\s]+$' AND length(email) <= 254),
    -- argon2id PHC string: $argon2id$v=19$m=...,t=...,p=...$salt$hash
    password_hash  text        NOT NULL CHECK (password_hash LIKE '$argon2id$%'),
    role           user_role   NOT NULL,
    -- Links a driver-role login to its driver record. RBAC resolves "my trips" from
    -- this server-side value, never from a client-supplied id.
    driver_id      bigint      NULL REFERENCES drivers (id) ON DELETE RESTRICT,
    -- UI language preference; NULL = fall back to Accept-Language, then 'en'.
    locale         text        NULL CHECK (locale IN ('en', 'lv', 'ru')),
    is_active      boolean     NOT NULL DEFAULT true,
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT users_email_key     UNIQUE (email),
    CONSTRAINT users_driver_id_key UNIQUE (driver_id),
    -- driver_id is set if and only if the role is 'driver'.
    CONSTRAINT users_driver_link CHECK ((role = 'driver') = (driver_id IS NOT NULL))
);
CREATE TRIGGER users_set_updated_at BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Least privilege: the app may read/write rows but never alter schema. No DELETE on
-- drivers or users — both are deactivated / anonymized instead.
GRANT SELECT, INSERT, UPDATE         ON drivers TO fleet_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON buses   TO fleet_app;
GRANT SELECT, INSERT, UPDATE         ON users   TO fleet_app;
