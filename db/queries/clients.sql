-- name: ListClients :many
SELECT id, name, contact_person, phone, email, address, notes, is_active, anonymized_at, created_at, updated_at
FROM clients
ORDER BY name;

-- name: ListActiveClients :many
-- Candidates for a trip's client picker.
SELECT id, name, contact_person, phone, email, address, notes, is_active, anonymized_at, created_at, updated_at
FROM clients
WHERE is_active
ORDER BY name;

-- name: GetClient :one
SELECT id, name, contact_person, phone, email, address, notes, is_active, anonymized_at, created_at, updated_at
FROM clients
WHERE id = $1;

-- name: CreateClient :one
INSERT INTO clients (name, contact_person, phone, email, address, notes)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, name, contact_person, phone, email, address, notes, is_active, anonymized_at, created_at, updated_at;

-- name: UpdateClient :one
UPDATE clients
SET name = $2, contact_person = $3, phone = $4, email = $5, address = $6, notes = $7, is_active = $8
WHERE id = $1 AND anonymized_at IS NULL
RETURNING id, name, contact_person, phone, email, address, notes, is_active, anonymized_at, created_at, updated_at;

-- name: AnonymizeClient :one
-- GDPR erasure. The row survives so trips keep their referent; every identifying field
-- (including free-text notes, which can themselves be personal data) is dropped.
-- Idempotent: a second call matches no row and returns no result.
UPDATE clients
SET name = $2, contact_person = NULL, phone = NULL, email = NULL, address = NULL,
    notes = NULL, is_active = false, anonymized_at = now()
WHERE id = $1 AND anonymized_at IS NULL
RETURNING id, name, contact_person, phone, email, address, notes, is_active, anonymized_at, created_at, updated_at;
