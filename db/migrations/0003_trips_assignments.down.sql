-- DESTRUCTIVE: drops all trips and assignments (incl. worked time). Dev/empty DB only.
DROP TABLE IF EXISTS assignments;
DROP FUNCTION IF EXISTS assignments_lock_completed();
DROP TABLE IF EXISTS trips;
DROP TYPE IF EXISTS trip_status;
