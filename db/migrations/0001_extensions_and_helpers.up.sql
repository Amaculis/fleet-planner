-- 0001: extensions and shared helper functions used by every later migration.

-- btree_gist lets a GiST index combine scalar equality (bus_id WITH =) with range
-- overlap (tstzrange WITH &&). Required for the no-double-booking EXCLUDE constraints.
-- It is a "trusted" extension (PG13+), so the non-superuser schema owner can create it.
CREATE EXTENSION IF NOT EXISTS btree_gist;

-- Maintains updated_at on every mutable table. Chosen over setting it in the repository
-- layer so it cannot be forgotten by a hand-written query.
CREATE FUNCTION set_updated_at() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    NEW.updated_at := now();
    RETURN NEW;
END;
$$;
