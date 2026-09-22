-- name: ListTripsInRange :many
-- Planning list and (step 4) the timeline: trips whose window touches [$1, $2).
SELECT id, origin, destination, scheduled_start, scheduled_end, actual_start, actual_end,
       status, notes, created_at, updated_at
FROM trips
WHERE tstzrange(scheduled_start, scheduled_end, '[)') && tstzrange(@range_start, @range_end, '[)')
ORDER BY scheduled_start, id;

-- name: GetTrip :one
SELECT id, origin, destination, scheduled_start, scheduled_end, actual_start, actual_end,
       status, notes, created_at, updated_at
FROM trips
WHERE id = $1;

-- name: CreateTrip :one
INSERT INTO trips (origin, destination, scheduled_start, scheduled_end, notes)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, origin, destination, scheduled_start, scheduled_end, actual_start, actual_end,
          status, notes, created_at, updated_at;

-- name: UpdateTrip :one
-- Rescheduling an assigned trip cascades the new window onto its assignment, where the
-- EXCLUDE constraints re-check it — so moving a trip into a clash fails here.
UPDATE trips
SET origin = $2, destination = $3, scheduled_start = $4, scheduled_end = $5, notes = $6
WHERE id = $1
RETURNING id, origin, destination, scheduled_start, scheduled_end, actual_start, actual_end,
          status, notes, created_at, updated_at;

-- name: SetTripStatus :one
UPDATE trips
SET status = $2
WHERE id = $1
RETURNING id, origin, destination, scheduled_start, scheduled_end, actual_start, actual_end,
          status, notes, created_at, updated_at;

-- name: DeleteTrip :execrows
-- Blocked by the assignments FK while the trip is assigned; cancel it instead.
DELETE FROM trips WHERE id = $1;

-- name: ListTripsForDriver :many
-- A driver's own trips. Ownership lives in the WHERE clause: the driver id comes from
-- the session, never from the request, and no query returns another driver's trips.
SELECT t.id, t.origin, t.destination, t.scheduled_start, t.scheduled_end,
       t.actual_start, t.actual_end, t.status, t.notes, t.created_at, t.updated_at,
       b.plate AS bus_plate
FROM trips t
JOIN assignments a ON a.trip_id = t.id
JOIN buses b ON b.id = a.bus_id
WHERE a.driver_id = @driver_id
  AND t.status <> 'cancelled'
  AND t.scheduled_end >= @since
ORDER BY t.scheduled_start;

-- name: StartTripAsDriver :one
-- payroll-seam: actual_start is the worked-time clock payroll will sum.
-- The driver id is matched in SQL, so a forged trip id cannot touch another driver's
-- trip: a mismatch returns no row, which the service maps to "not found".
UPDATE trips
SET status = 'in_progress', actual_start = now()
FROM assignments a
WHERE trips.id = @trip_id
  AND a.trip_id = trips.id
  AND a.driver_id = @driver_id
  AND trips.status = 'planned'
RETURNING trips.id, trips.origin, trips.destination, trips.scheduled_start, trips.scheduled_end,
          trips.actual_start, trips.actual_end, trips.status, trips.notes,
          trips.created_at, trips.updated_at;

-- name: FinishTripAsDriver :one
-- payroll-seam: actual_end closes the worked-time window.
UPDATE trips
SET status = 'completed', actual_end = now()
FROM assignments a
WHERE trips.id = @trip_id
  AND a.trip_id = trips.id
  AND a.driver_id = @driver_id
  AND trips.status = 'in_progress'
RETURNING trips.id, trips.origin, trips.destination, trips.scheduled_start, trips.scheduled_end,
          trips.actual_start, trips.actual_end, trips.status, trips.notes,
          trips.created_at, trips.updated_at;

-- name: GetTripForDriver :one
-- Ownership in the WHERE clause: a trip id that is not the caller's returns no row.
SELECT t.id, t.origin, t.destination, t.scheduled_start, t.scheduled_end,
       t.actual_start, t.actual_end, t.status, t.notes, t.created_at, t.updated_at
FROM trips t
JOIN assignments a ON a.trip_id = t.id
WHERE t.id = @trip_id AND a.driver_id = @driver_id;
