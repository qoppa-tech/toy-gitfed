-- name: CreateUser :one
INSERT INTO users (user_id, name, username, password, email, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, now(), now())
RETURNING *;

-- name: GetUserByID :one
SELECT * FROM users
WHERE user_id = $1 AND is_deleted = FALSE;

-- name: GetUserByEmail :one
SELECT * FROM users
WHERE email = $1 AND is_deleted = FALSE;

-- name: GetUserByUsername :one
SELECT * FROM users
WHERE username = $1 AND is_deleted = FALSE;

-- name: VerifyUser :exec
UPDATE users
SET is_verified = TRUE, updated_at = now()
WHERE user_id = $1;

-- name: SoftDeleteUser :exec
UPDATE users
SET is_deleted = TRUE, deleted_at = now(), updated_at = now()
WHERE user_id = $1;

-- name: UpdateUserPassword :exec
UPDATE users
SET password = $2, updated_at = now()
WHERE user_id = $1;
