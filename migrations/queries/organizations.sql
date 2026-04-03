-- name: CreateOrganization :one
INSERT INTO organizations (organization_id, organization_name, organization_description)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetOrganizationByID :one
SELECT * FROM organizations
WHERE organization_id = $1;

-- name: AddUserToOrganization :exec
INSERT INTO organization_users (organization_id, user_id)
VALUES ($1, $2);

-- name: RemoveUserFromOrganization :exec
DELETE FROM organization_users
WHERE organization_id = $1 AND user_id = $2;

-- name: GetOrganizationsByUserID :many
SELECT o.* FROM organizations o
JOIN organization_users ou ON o.organization_id = ou.organization_id
WHERE ou.user_id = $1;

-- name: GetUsersByOrganizationID :many
SELECT u.* FROM users u
JOIN organization_users ou ON u.user_id = ou.user_id
WHERE ou.organization_id = $1 AND u.is_deleted = FALSE;
