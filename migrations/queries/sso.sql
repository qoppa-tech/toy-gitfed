-- name: CreateSSO :one
INSERT INTO sso_providers (sso_id, user_id, name, provider, username, activated_at, created_at)
VALUES ($1, $2, $3, $4, $5, now(), now())
RETURNING *;

-- name: GetSSOByUserID :many
SELECT * FROM sso_providers
WHERE user_id = $1;

-- name: GetSSOByProviderAndUsername :one
SELECT * FROM sso_providers
WHERE provider = $1 AND username = $2;

-- name: DeleteSSO :exec
DELETE FROM sso_providers
WHERE sso_id = $1;
