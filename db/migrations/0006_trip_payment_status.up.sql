-- 0006: client payment status on trips.

CREATE TYPE payment_status AS ENUM ('unpaid', 'reserved', 'advance_paid', 'paid');

ALTER TABLE trips
    ADD COLUMN payment_status payment_status NOT NULL DEFAULT 'unpaid';
