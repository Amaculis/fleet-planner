-- name: CreateAssignment :one
-- The time window and status are copied straight from the trip row, so the composite FK
-- (trip_id, scheduled_start, scheduled_end, trip_status) always matches and the EXCLUDE
-- constraints see the real window. The app never supplies those values itself.
INSERT INTO assignments (trip_id, bus_id, driver_id, scheduled_start, scheduled_end, trip_status)
SELECT t.id, @bus_id, @driver_id, t.scheduled_start, t.scheduled_end, t.status
FROM trips t
WHERE t.id = @trip_id
RETURNING id, trip_id, bus_id, driver_id, scheduled_start, scheduled_end, trip_status,
          created_at, updated_at;

-- name: GetAssignmentForTrip :one
SELECT id, trip_id, bus_id, driver_id, scheduled_start, scheduled_end, trip_status,
       created_at, updated_at
FROM assignments
WHERE trip_id = $1;

-- name: DeleteAssignmentForTrip :execrows
-- Refused by the assignments_lock_completed trigger once the trip is completed:
-- payroll must not lose who drove it.
DELETE FROM assignments WHERE trip_id = $1;

-- name: FindBusConflicts :many
-- Service-layer pre-check, purely so the user gets a readable message. The EXCLUDE
-- constraint is what actually guarantees the invariant under concurrency.
SELECT a.trip_id, a.bus_id, a.driver_id, a.scheduled_start, a.scheduled_end,
       t.origin, t.destination
FROM assignments a
JOIN trips t ON t.id = a.trip_id
WHERE a.bus_id = @bus_id
  AND a.trip_status <> 'cancelled'
  AND a.trip_id <> @exclude_trip_id
  AND tstzrange(a.scheduled_start, a.scheduled_end, '[)')
      && tstzrange(@window_start, @window_end, '[)')
ORDER BY a.scheduled_start
LIMIT 5;

-- name: FindDriverConflicts :many
SELECT a.trip_id, a.bus_id, a.driver_id, a.scheduled_start, a.scheduled_end,
       t.origin, t.destination
FROM assignments a
JOIN trips t ON t.id = a.trip_id
WHERE a.driver_id = @driver_id
  AND a.trip_status <> 'cancelled'
  AND a.trip_id <> @exclude_trip_id
  AND tstzrange(a.scheduled_start, a.scheduled_end, '[)')
      && tstzrange(@window_start, @window_end, '[)')
ORDER BY a.scheduled_start
LIMIT 5;

-- name: ListAssignmentsInRange :many
-- Feeds the planning list and (step 4) the timeline.
SELECT a.id, a.trip_id, a.bus_id, a.driver_id, a.scheduled_start, a.scheduled_end,
       a.trip_status, b.plate AS bus_plate, d.full_name AS driver_name,
       t.origin, t.destination
FROM assignments a
JOIN buses b ON b.id = a.bus_id
JOIN drivers d ON d.id = a.driver_id
JOIN trips t ON t.id = a.trip_id
WHERE tstzrange(a.scheduled_start, a.scheduled_end, '[)')
      && tstzrange(@range_start, @range_end, '[)')
ORDER BY b.plate, a.scheduled_start;
