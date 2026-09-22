-- name: ListBuses :many
SELECT id, plate, model, seats, status, insurance_expiry, inspection_expiry, created_at, updated_at
FROM buses
ORDER BY plate;

-- name: ListActiveBuses :many
-- Candidates for an assignment: a bus in maintenance or retired is not offered.
SELECT id, plate, model, seats, status, insurance_expiry, inspection_expiry, created_at, updated_at
FROM buses
WHERE status = 'active'
ORDER BY plate;

-- name: GetBus :one
SELECT id, plate, model, seats, status, insurance_expiry, inspection_expiry, created_at, updated_at
FROM buses
WHERE id = $1;

-- name: CreateBus :one
INSERT INTO buses (plate, model, seats, status, insurance_expiry, inspection_expiry)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, plate, model, seats, status, insurance_expiry, inspection_expiry, created_at, updated_at;

-- name: UpdateBus :one
UPDATE buses
SET plate = $2, model = $3, seats = $4, status = $5, insurance_expiry = $6, inspection_expiry = $7
WHERE id = $1
RETURNING id, plate, model, seats, status, insurance_expiry, inspection_expiry, created_at, updated_at;

-- name: DeleteBus :execrows
-- The FK from assignments is ON DELETE RESTRICT, so a bus that has ever been assigned
-- cannot be deleted; retire it instead (status = 'retired').
DELETE FROM buses WHERE id = $1;
