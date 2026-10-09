-- name: CreateTripSeries :one
INSERT INTO trip_series (origin, destination, days_of_week, first_start, first_end, ends_on, payment_status, notes)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id, origin, destination, days_of_week, first_start, first_end, ends_on, payment_status, notes,
          created_at, updated_at;
