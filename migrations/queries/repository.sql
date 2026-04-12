-- name: CreateRepository :one
INSERT INTO git_repository (id, name, description, is_private, owner_id, default_ref, is_deleted, head, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, false, '', now(), now())
RETURNING *;

-- name: GetRepositoryByID :one
SELECT * FROM git_repository WHERE id = $1;

-- name: GetRepositoryByRepositoryName :many
SELECT * FROM git_repository WHERE name = $1;

-- name: GetRepositoryByOwnerId :many
SELECT * FROM git_repository WHERE owner_id = $1;

-- name: SoftDeleteRepository :one
UPDATE git_repository
SET is_deleted = true, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: UpdateRepository :exec
UPDATE git_repository
SET updated_at= now(), name= $2, description= $3, is_private= $4, owner_id= $5, default_ref= $6
WHERE id= $1;
