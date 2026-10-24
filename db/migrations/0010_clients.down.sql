ALTER TABLE trip_series DROP COLUMN client_id;
DROP INDEX trips_client_id_idx;
ALTER TABLE trips DROP COLUMN invoice_number, DROP COLUMN client_id;
DROP TABLE clients;
