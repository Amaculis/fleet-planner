-- 0008: free-text dispatcher notes on a driver (e.g. preferences, restrictions).

ALTER TABLE drivers
    ADD COLUMN notes text NULL CHECK (length(notes) <= 2000);
