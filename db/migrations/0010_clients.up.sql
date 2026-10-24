-- 0010: clients (who a trip is for) and a hand-typed bill number per trip.
--
-- clients hold personal data of private customers (GDPR). They are never hard-deleted:
-- erasure anonymizes in place, exactly like drivers, so trips keep their referent.
-- trips.invoice_number is deliberately free text and NOT unique — one bill can cover
-- several trips — and it is never copied by a series (each occurrence has its own bill).

CREATE TABLE clients (
    id             bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name           text        NOT NULL CHECK (length(btrim(name)) BETWEEN 1 AND 200),
    contact_person text        NULL CHECK (length(btrim(contact_person)) BETWEEN 1 AND 200),
    phone          text        NULL CHECK (phone ~ '^\+?[0-9 ()-]{5,32}$'),
    email          text        NULL CHECK (length(btrim(email)) BETWEEN 3 AND 254),
    address        text        NULL CHECK (length(btrim(address)) BETWEEN 1 AND 500),
    notes          text        NULL CHECK (length(notes) <= 2000),
    is_active      boolean     NOT NULL DEFAULT true,
    anonymized_at  timestamptz NULL,
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now(),

    -- An anonymized client carries no direct identifiers and cannot be active.
    CONSTRAINT clients_anonymized_has_no_pii CHECK (
        anonymized_at IS NULL
        OR (contact_person IS NULL AND phone IS NULL AND email IS NULL
            AND address IS NULL AND notes IS NULL AND NOT is_active)
    )
);
CREATE TRIGGER clients_set_updated_at BEFORE UPDATE ON clients
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- RESTRICT: a client with trips can never be hard-deleted (and the app has no delete).
ALTER TABLE trips
    ADD COLUMN client_id bigint NULL REFERENCES clients (id) ON DELETE RESTRICT,
    ADD COLUMN invoice_number text NULL CHECK (length(btrim(invoice_number)) BETWEEN 1 AND 50);
CREATE INDEX trips_client_id_idx ON trips (client_id);

-- A series carries the client so every generated occurrence gets it.
ALTER TABLE trip_series
    ADD COLUMN client_id bigint NULL REFERENCES clients (id) ON DELETE RESTRICT;

GRANT SELECT, INSERT, UPDATE ON clients TO fleet_app;
