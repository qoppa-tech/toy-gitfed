-- name: CreateSession :one
INSERT INTO sessions (session_id, user_id, refresh_token, created_at)
VALUES ($1, $2, $3, now())
RETURNING *;

-- name: GetSessionByRefreshToken :one
SELECT * FROM sessions
WHERE refresh_token = $1;

-- name: GetSessionByID :one
SELECT * FROM sessions
WHERE session_id = $1;

-- name: DeleteSession :exec
DELETE FROM sessions
WHERE session_id = $1;

-- name: DeleteSessionsByUserID :exec
DELETE FROM sessions
WHERE user_id = $1;
