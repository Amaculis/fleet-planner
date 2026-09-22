#!/bin/sh
# Runs once, as the Postgres superuser, when the data volume is first initialised.
# Creates two non-superuser roles and the application database:
#   fleet_owner - owns the schema; used ONLY by the one-shot `migrate` service.
#   fleet_app   - used by the running app; per-table DML grants come from the migrations.
# The superuser password is never given to the app or migrate containers.
set -eu

: "${FLEET_OWNER_PASSWORD:?FLEET_OWNER_PASSWORD is required}"
: "${FLEET_APP_PASSWORD:?FLEET_APP_PASSWORD is required}"

# Passwords are passed as psql variables and quoted with :'var' -- never interpolated
# into the SQL text by the shell.
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname postgres \
     -v owner_pw="$FLEET_OWNER_PASSWORD" -v app_pw="$FLEET_APP_PASSWORD" <<'SQL'
CREATE ROLE fleet_owner LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS PASSWORD :'owner_pw';
CREATE ROLE fleet_app   LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS PASSWORD :'app_pw';

CREATE DATABASE fleet OWNER fleet_owner ENCODING 'UTF8' TEMPLATE template0;
REVOKE ALL ON DATABASE fleet FROM PUBLIC;
GRANT CONNECT ON DATABASE fleet TO fleet_app;

\connect fleet
-- PG15+: the public schema is owned by pg_database_owner (= fleet_owner) and PUBLIC has
-- no CREATE. Stated explicitly so it holds regardless of version defaults.
REVOKE ALL ON SCHEMA public FROM PUBLIC;
GRANT USAGE ON SCHEMA public TO fleet_app;

-- search_path hardening: the app cannot shadow objects via another schema.
ALTER ROLE fleet_app   SET search_path = public;
ALTER ROLE fleet_owner SET search_path = public;

-- A runaway app query can never hold locks indefinitely.
ALTER ROLE fleet_app SET statement_timeout = '15s';
ALTER ROLE fleet_app SET idle_in_transaction_session_timeout = '30s';
SQL
