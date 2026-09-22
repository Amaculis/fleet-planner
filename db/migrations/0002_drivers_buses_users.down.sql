-- DESTRUCTIVE: drops all driver, bus and user data. Only reversible on an empty/dev DB.
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS buses;
DROP TABLE IF EXISTS drivers;
DROP TYPE IF EXISTS pay_type;
DROP TYPE IF EXISTS bus_status;
DROP TYPE IF EXISTS user_role;
